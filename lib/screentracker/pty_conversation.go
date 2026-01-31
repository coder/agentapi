package screentracker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/coder/agentapi/lib/msgfmt"
	"github.com/coder/agentapi/lib/util"
	"golang.org/x/xerrors"
)

// A screenSnapshot represents a snapshot of the PTY at a specific time.
type screenSnapshot struct {
	timestamp time.Time
	screen    string
}

type MessagePartText struct {
	Content string
	Alias   string
	Hidden  bool
}

var _ MessagePart = &MessagePartText{}

func (p MessagePartText) Do(writer AgentIO) error {
	_, err := writer.Write([]byte(p.Content))
	return err
}

func (p MessagePartText) String() string {
	if p.Hidden {
		return ""
	}
	if p.Alias != "" {
		return p.Alias
	}
	return p.Content
}

// PTYConversationConfig is the configuration for a PTYConversation.
type PTYConversationConfig struct {
	AgentType msgfmt.AgentType
	AgentIO   AgentIO
	// GetTime returns the current time
	GetTime func() time.Time
	// How often to take a snapshot for the stability check
	SnapshotInterval time.Duration
	// How long the screen should not change to be considered stable
	ScreenStabilityLength time.Duration
	// Function to format the messages received from the agent
	// userInput is the last user message
	FormatMessage func(message string, userInput string) string
	// SkipWritingMessage skips the writing of a message to the agent.
	// This is used in tests
	SkipWritingMessage bool
	// SkipSendMessageStatusCheck skips the check for whether the message can be sent.
	// This is used in tests
	SkipSendMessageStatusCheck bool
	// ReadyForInitialPrompt detects whether the agent has initialized and is ready to accept the initial prompt
	ReadyForInitialPrompt func(message string) bool
	// FormatToolCall removes the coder report_task tool call from the agent message and also returns the array of removed tool calls
	FormatToolCall func(message string) (string, []string)
	Logger         *slog.Logger
}

func (cfg PTYConversationConfig) getStableSnapshotsThreshold() int {
	length := cfg.ScreenStabilityLength.Milliseconds()
	interval := cfg.SnapshotInterval.Milliseconds()
	threshold := int(length / interval)
	if length%interval != 0 {
		threshold++
	}
	return threshold + 1
}

// PTYConversation is a conversation that uses a pseudo-terminal (PTY) for communication.
// It uses a combination of polling and diffs to detect changes in the screen.
type PTYConversation struct {
	cfg PTYConversationConfig
	// How many stable snapshots are required to consider the screen stable
	stableSnapshotsThreshold    int
	snapshotBuffer              *RingBuffer[screenSnapshot]
	messages                    []ConversationMessage
	screenBeforeLastUserMessage string
	lock                        sync.Mutex

	// InitialPrompt is the initial prompt passed to the agent
	InitialPrompt string
	// InitialPromptSent keeps track if the InitialPrompt has been successfully sent to the agents
	InitialPromptSent bool
	// ReadyForInitialPrompt keeps track if the agent is ready to accept the initial prompt
	ReadyForInitialPrompt bool
	// toolCallMessageSet keeps track of the tool calls that have been detected & logged in the current agent message
	toolCallMessageSet map[string]bool
	// dirty tracks whether the conversation state has changed since the last save
	dirty bool
	// firstStableSnapshot is the conversation history rolled out by the agent in case of a resume (given that the agent supports it)
	firstStableSnapshot string
	// userSentMessageAfterLoadState tracks if the user has sent their first message after we load the state
	userSentMessageAfterLoadState bool
}

var _ Conversation = &PTYConversation{}

func NewPTY(ctx context.Context, cfg PTYConversationConfig, initialPrompt string) *PTYConversation {
	threshold := cfg.getStableSnapshotsThreshold()
	c := &PTYConversation{
		cfg:                      cfg,
		stableSnapshotsThreshold: threshold,
		snapshotBuffer:           NewRingBuffer[screenSnapshot](threshold),
		messages: []ConversationMessage{
			{
				Message: "",
				Role:    ConversationRoleAgent,
				Time:    cfg.GetTime(),
			},
		},
		InitialPrompt:      initialPrompt,
		InitialPromptSent:  len(initialPrompt) == 0,
		toolCallMessageSet: make(map[string]bool),
	}
	return c
}

func (c *PTYConversation) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(c.cfg.SnapshotInterval):
				// It's important that we hold the lock while reading the screen.
				// There's a race condition that occurs without it:
				// 1. The screen is read
				// 2. Independently, SendMessage is called and takes the lock.
				// 3. AddSnapshot is called and waits on the lock.
				// 4. SendMessage modifies the terminal state, releases the lock
				// 5. AddSnapshot adds a snapshot from a stale screen
				c.lock.Lock()
				screen := c.cfg.AgentIO.ReadScreen()
				c.snapshotLocked(screen)
				c.lock.Unlock()
			}
		}
	}()
}

func (c *PTYConversation) lastMessage(role ConversationRole) ConversationMessage {
	for i := len(c.messages) - 1; i >= 0; i-- {
		if c.messages[i].Role == role {
			return c.messages[i]
		}
	}
	return ConversationMessage{}
}

// caller MUST hold c.lock
func (c *PTYConversation) updateLastAgentMessageLocked(screen string, timestamp time.Time) {
	agentMessage := screenDiff(c.screenBeforeLastUserMessage, screen, c.cfg.AgentType)
	lastUserMessage := c.lastMessage(ConversationRoleUser)
	var toolCalls []string
	if c.cfg.FormatMessage != nil {
		agentMessage = c.cfg.FormatMessage(agentMessage, lastUserMessage.Message)
	}
	agentMessage = c.skipInitialSnapshot(agentMessage)
	if c.cfg.FormatToolCall != nil {
		agentMessage, toolCalls = c.cfg.FormatToolCall(agentMessage)
	}
	for _, toolCall := range toolCalls {
		if c.toolCallMessageSet[toolCall] == false {
			c.toolCallMessageSet[toolCall] = true
			c.cfg.Logger.Info("Tool call detected", "toolCall", toolCall)
		}
	}
	shouldCreateNewMessage := len(c.messages) == 0 || c.messages[len(c.messages)-1].Role == ConversationRoleUser
	lastAgentMessage := c.lastMessage(ConversationRoleAgent)
	if lastAgentMessage.Message == agentMessage {
		return
	}
	conversationMessage := ConversationMessage{
		Message: agentMessage,
		Role:    ConversationRoleAgent,
		Time:    timestamp,
	}
	if shouldCreateNewMessage {
		c.messages = append(c.messages, conversationMessage)

		// Cleanup
		c.toolCallMessageSet = make(map[string]bool)

	} else {
		c.messages[len(c.messages)-1] = conversationMessage
	}
	c.messages[len(c.messages)-1].Id = len(c.messages) - 1
	c.dirty = true
}

func (c *PTYConversation) Snapshot(screen string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.snapshotLocked(screen)
}

// caller MUST hold c.lock
func (c *PTYConversation) snapshotLocked(screen string) {
	snapshot := screenSnapshot{
		timestamp: c.cfg.GetTime(),
		screen:    screen,
	}
	c.snapshotBuffer.Add(snapshot)
	c.updateLastAgentMessageLocked(screen, snapshot.timestamp)
}

func (c *PTYConversation) Send(messageParts ...MessagePart) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.cfg.SkipSendMessageStatusCheck && c.statusLocked() != ConversationStatusStable {
		return MessageValidationErrorChanging
	}

	var sb strings.Builder
	for _, part := range messageParts {
		sb.WriteString(part.String())
	}
	message := sb.String()
	if message != msgfmt.TrimWhitespace(message) {
		// msgfmt formatting functions assume this
		return MessageValidationErrorWhitespace
	}
	if message == "" {
		// writeMessageWithConfirmation requires a non-empty message
		return MessageValidationErrorEmpty
	}

	screenBeforeMessage := c.cfg.AgentIO.ReadScreen()
	now := c.cfg.GetTime()
	c.updateLastAgentMessageLocked(screenBeforeMessage, now)

	if err := c.writeStabilize(context.Background(), messageParts...); err != nil {
		return xerrors.Errorf("failed to send message: %w", err)
	}

	c.screenBeforeLastUserMessage = screenBeforeMessage
	c.messages = append(c.messages, ConversationMessage{
		Id:      len(c.messages),
		Message: message,
		Role:    ConversationRoleUser,
		Time:    now,
	})
	c.dirty = true
	c.userSentMessageAfterLoadState = true

	return nil
}

// writeStabilize writes messageParts to the screen and waits for the screen to stabilize after the message is written.
func (c *PTYConversation) writeStabilize(ctx context.Context, messageParts ...MessagePart) error {
	if c.cfg.SkipWritingMessage {
		return nil
	}
	screenBeforeMessage := c.cfg.AgentIO.ReadScreen()
	for _, part := range messageParts {
		if err := part.Do(c.cfg.AgentIO); err != nil {
			return xerrors.Errorf("failed to write message part: %w", err)
		}
	}
	// wait for the screen to stabilize after the message is written
	if err := util.WaitFor(ctx, util.WaitTimeout{
		Timeout:     15 * time.Second,
		MinInterval: 50 * time.Millisecond,
		InitialWait: true,
	}, func() (bool, error) {
		screen := c.cfg.AgentIO.ReadScreen()
		if screen != screenBeforeMessage {
			time.Sleep(1 * time.Second)
			newScreen := c.cfg.AgentIO.ReadScreen()
			return newScreen == screen, nil
		}
		return false, nil
	}); err != nil {
		return xerrors.Errorf("failed to wait for screen to stabilize: %w", err)
	}

	// wait for the screen to change after the carriage return is written
	screenBeforeCarriageReturn := c.cfg.AgentIO.ReadScreen()
	lastCarriageReturnTime := time.Time{}
	if err := util.WaitFor(ctx, util.WaitTimeout{
		Timeout:     15 * time.Second,
		MinInterval: 25 * time.Millisecond,
	}, func() (bool, error) {
		// we don't want to spam additional carriage returns because the agent may process them
		// (aider does this), but we do want to retry sending one if nothing's
		// happening for a while
		if time.Since(lastCarriageReturnTime) >= 3*time.Second {
			lastCarriageReturnTime = time.Now()
			if _, err := c.cfg.AgentIO.Write([]byte("\r")); err != nil {
				return false, xerrors.Errorf("failed to write carriage return: %w", err)
			}
		}
		time.Sleep(25 * time.Millisecond)
		screen := c.cfg.AgentIO.ReadScreen()

		return screen != screenBeforeCarriageReturn, nil
	}); err != nil {
		return xerrors.Errorf("failed to wait for processing to start: %w", err)
	}

	return nil
}

func (c *PTYConversation) Status() ConversationStatus {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.statusLocked()
}

// caller MUST hold c.lock
func (c *PTYConversation) statusLocked() ConversationStatus {
	// sanity checks
	if c.snapshotBuffer.Capacity() != c.stableSnapshotsThreshold {
		panic(fmt.Sprintf("snapshot buffer capacity %d is not equal to snapshot threshold %d. can't check stability", c.snapshotBuffer.Capacity(), c.stableSnapshotsThreshold))
	}
	if c.stableSnapshotsThreshold == 0 {
		panic("stable snapshots threshold is 0. can't check stability")
	}

	snapshots := c.snapshotBuffer.GetAll()
	if len(c.messages) > 0 && c.messages[len(c.messages)-1].Role == ConversationRoleUser {
		// if the last message is a user message then the snapshot loop hasn't
		// been triggered since the last user message, and we should assume
		// the screen is changing
		return ConversationStatusChanging
	}

	if len(snapshots) != c.stableSnapshotsThreshold {
		return ConversationStatusInitializing
	}

	for i := 1; i < len(snapshots); i++ {
		if snapshots[0].screen != snapshots[i].screen {
			return ConversationStatusChanging
		}
	}

	if !c.InitialPromptSent && !c.ReadyForInitialPrompt {
		if len(snapshots) > 0 && c.cfg.ReadyForInitialPrompt(snapshots[len(snapshots)-1].screen) {
			c.ReadyForInitialPrompt = true
			return ConversationStatusStable
		}
		return ConversationStatusChanging
	}

	return ConversationStatusStable
}

func (c *PTYConversation) Messages() []ConversationMessage {
	c.lock.Lock()
	defer c.lock.Unlock()

	result := make([]ConversationMessage, len(c.messages))
	copy(result, c.messages)
	return result
}

func (c *PTYConversation) String() string {
	c.lock.Lock()
	defer c.lock.Unlock()

	snapshots := c.snapshotBuffer.GetAll()
	if len(snapshots) == 0 {
		return ""
	}
	return snapshots[len(snapshots)-1].screen
}

func (c *PTYConversation) SaveState(conversation []ConversationMessage, stateFile string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	// Skip if state file is not configured
	if stateFile == "" {
		return nil
	}

	// Skip if not dirty
	if !c.dirty {
		return nil
	}

	// Use atomic write: write to temp file, then rename to target path
	data, err := json.MarshalIndent(AgentState{
		Version:           1,
		Messages:          conversation,
		InitialPrompt:     c.InitialPrompt,
		InitialPromptSent: c.InitialPromptSent,
	}, "", " ")
	if err != nil {
		return xerrors.Errorf("failed to marshal state: %w", err)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(stateFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return xerrors.Errorf("failed to create state directory: %w", err)
	}

	// Write to temp file
	tempFile := stateFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0o644); err != nil {
		return xerrors.Errorf("failed to write temp state file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, stateFile); err != nil {
		return xerrors.Errorf("failed to rename state file: %w", err)
	}

	// Clear dirty flag after successful save
	c.dirty = false
	return nil
}

func (c *PTYConversation) LoadState(stateFile string) ([]ConversationMessage, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	// Skip if state file is not configured
	if stateFile == "" {
		return nil, nil
	}

	// Check if file exists
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		c.cfg.Logger.Info("No previous state to load (file does not exist)", "path", stateFile)
		return nil, nil
	}

	// Read state file
	data, err := os.ReadFile(stateFile)
	if err != nil {
		c.cfg.Logger.Warn("Failed to load state file", "path", stateFile, "err", err)
		return nil, xerrors.Errorf("failed to read state file: %w", err)
	}

	if len(data) == 0 {
		c.cfg.Logger.Info("No previous state to load (file is empty)", "path", stateFile)
		return nil, nil
	}

	var agentState AgentState
	if err := json.Unmarshal(data, &agentState); err != nil {
		c.cfg.Logger.Warn("Failed to load state file (corrupted or invalid JSON)", "path", stateFile, "err", err)
		return nil, xerrors.Errorf("failed to unmarshal state (corrupted or invalid JSON): %w", err)
	}

	c.InitialPromptSent = agentState.InitialPromptSent
	c.InitialPrompt = agentState.InitialPrompt
	c.messages = agentState.Messages

	// Store the first stable snapshot for filtering later
	snapshots := c.snapshotBuffer.GetAll()
	if len(snapshots) > 0 {
		c.firstStableSnapshot = c.cfg.FormatMessage(strings.TrimSpace(snapshots[len(snapshots)-1].screen), "")
	}

	c.cfg.Logger.Info("Successfully loaded state", "path", stateFile, "messages", len(c.messages))
	return c.messages, nil
}

func (c *PTYConversation) skipInitialSnapshot(screen string) string {
	newScreen := strings.ReplaceAll(screen, c.firstStableSnapshot, "")

	// Before the first user message after loading state, return the last message from the loaded state.
	// This prevents computing incorrect diffs from the restored screen, as the agent's message should
	// remain stable until the user continues the conversation.
	if c.userSentMessageAfterLoadState == false {
		newScreen = "\n" + c.messages[len(c.messages)-1].Message
	}

	return newScreen
}
