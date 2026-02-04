package screentracker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/coder/agentapi/lib/msgfmt"
	"github.com/coder/agentapi/lib/util"
	"github.com/coder/quartz"
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
	// Clock provides time operations for the conversation
	Clock quartz.Clock
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
	// InitialPrompt is the initial prompt to send to the agent once ready
	InitialPrompt []MessagePart
	// OnSnapshot is called after each snapshot with current status, messages, and screen content
	OnSnapshot func(status ConversationStatus, messages []ConversationMessage, screen string)
	Logger     *slog.Logger
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
	// initialPromptSent keeps track if the InitialPrompt has been successfully sent to the agent
	initialPromptSent bool
	// initialPromptReady is closed when the agent is ready to receive the initial prompt
	initialPromptReady chan struct{}
	// toolCallMessageSet keeps track of the tool calls that have been detected & logged in the current agent message
	toolCallMessageSet map[string]bool
}

var _ Conversation = &PTYConversation{}

func NewPTY(ctx context.Context, cfg PTYConversationConfig) *PTYConversation {
	if cfg.Clock == nil {
		cfg.Clock = quartz.NewReal()
	}
	threshold := cfg.getStableSnapshotsThreshold()
	c := &PTYConversation{
		cfg:                      cfg,
		stableSnapshotsThreshold: threshold,
		snapshotBuffer:           NewRingBuffer[screenSnapshot](threshold),
		messages: []ConversationMessage{
			{
				Message: "",
				Role:    ConversationRoleAgent,
				Time:    cfg.Clock.Now(),
			},
		},
		initialPromptSent:  len(cfg.InitialPrompt) == 0,
		toolCallMessageSet: make(map[string]bool),
	}
	// Initialize the channel only if we have an initial prompt to send
	if len(cfg.InitialPrompt) > 0 {
		c.initialPromptReady = make(chan struct{})
	}
	return c
}

func (c *PTYConversation) Start(ctx context.Context) {
	go func() {
		ticker := c.cfg.Clock.NewTicker(c.cfg.SnapshotInterval)
		defer ticker.Stop()

		// Create a nil channel if no initial prompt - select will never receive from it
		initialPromptReady := c.initialPromptReady
		if initialPromptReady == nil {
			initialPromptReady = make(chan struct{})
			// Don't close it - we want it to block forever
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// It's important that we hold the lock while reading the screen.
				// There's a race condition that occurs without it:
				// 1. The screen is read
				// 2. Independently, Send is called and takes the lock.
				// 3. snapshotLocked is called and waits on the lock.
				// 4. Send modifies the terminal state, releases the lock
				// 5. snapshotLocked adds a snapshot from a stale screen
				c.lock.Lock()
				screen := c.cfg.AgentIO.ReadScreen()
				c.snapshotLocked(screen)
				status := c.statusLocked()
				messages := c.messagesLocked()
				c.lock.Unlock()

				// Call snapshot callback if configured
				if c.cfg.OnSnapshot != nil {
					c.cfg.OnSnapshot(status, messages, screen)
				}
			case <-initialPromptReady:
				// Agent is ready for initial prompt - send it
				c.lock.Lock()
				if !c.initialPromptSent && len(c.cfg.InitialPrompt) > 0 {
					if err := c.sendLocked(c.cfg.InitialPrompt...); err != nil {
						c.cfg.Logger.Error("failed to send initial prompt", "error", err)
					} else {
						c.initialPromptSent = true
					}
				}
				c.lock.Unlock()
				// Nil out to prevent this case from triggering again
				initialPromptReady = nil
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
}

// Snapshot writes the current screen snapshot to the snapshot buffer.
// ONLY TO BE USED FOR TESTING PURPOSES.
// TODO(Cian): This method can be removed by mocking AgentIO.
func (c *PTYConversation) Snapshot(screen string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.snapshotLocked(screen)
}

// caller MUST hold c.lock
func (c *PTYConversation) snapshotLocked(screen string) {
	snapshot := screenSnapshot{
		timestamp: c.cfg.Clock.Now(),
		screen:    screen,
	}
	c.snapshotBuffer.Add(snapshot)
	c.updateLastAgentMessageLocked(screen, snapshot.timestamp)
}

func (c *PTYConversation) Send(messageParts ...MessagePart) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.cfg.SkipSendMessageStatusCheck && c.statusLocked() != ConversationStatusStable {
		return ErrMessageValidationChanging
	}

	return c.sendLocked(messageParts...)
}

// sendLocked sends a message to the agent. Caller MUST hold c.lock.
func (c *PTYConversation) sendLocked(messageParts ...MessagePart) error {
	var sb strings.Builder
	for _, part := range messageParts {
		sb.WriteString(part.String())
	}
	message := sb.String()
	if message != msgfmt.TrimWhitespace(message) {
		// msgfmt formatting functions assume this
		return ErrMessageValidationWhitespace
	}
	if message == "" {
		// writeMessageWithConfirmation requires a non-empty message
		return ErrMessageValidationEmpty
	}

	screenBeforeMessage := c.cfg.AgentIO.ReadScreen()
	now := c.cfg.Clock.Now()
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
			select {
			case <-ctx.Done():
				return false, ctx.Err()
			case <-util.After(c.cfg.Clock, 1*time.Second):
			}
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
		if c.cfg.Clock.Since(lastCarriageReturnTime) >= 3*time.Second {
			lastCarriageReturnTime = c.cfg.Clock.Now()
			if _, err := c.cfg.AgentIO.Write([]byte("\r")); err != nil {
				return false, xerrors.Errorf("failed to write carriage return: %w", err)
			}
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-util.After(c.cfg.Clock, 25*time.Millisecond):
		}
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

	// Handle initial prompt readiness: report "changing" until the prompt is sent
	// to avoid the status flipping "changing" → "stable" → "changing"
	if !c.initialPromptSent {
		// Check if agent is ready for initial prompt and signal if so
		if c.initialPromptReady != nil && len(snapshots) > 0 && c.cfg.ReadyForInitialPrompt != nil && c.cfg.ReadyForInitialPrompt(snapshots[len(snapshots)-1].screen) {
			close(c.initialPromptReady)
			c.initialPromptReady = nil // Prevent double-close
		}
		// Keep returning "changing" until initial prompt is actually sent
		return ConversationStatusChanging
	}

	return ConversationStatusStable
}

func (c *PTYConversation) Messages() []ConversationMessage {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.messagesLocked()
}

// messagesLocked returns a copy of messages. Caller MUST hold c.lock.
func (c *PTYConversation) messagesLocked() []ConversationMessage {
	result := make([]ConversationMessage, len(c.messages))
	copy(result, c.messages)
	return result
}

func (c *PTYConversation) Text() string {
	c.lock.Lock()
	defer c.lock.Unlock()

	snapshots := c.snapshotBuffer.GetAll()
	if len(snapshots) == 0 {
		return ""
	}
	return snapshots[len(snapshots)-1].screen
}
