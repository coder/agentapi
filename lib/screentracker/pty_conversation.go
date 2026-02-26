package screentracker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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

type AgentState struct {
	Version           int                   `json:"version"`
	Messages          []ConversationMessage `json:"messages"`
	InitialPrompt     string                `json:"initial_prompt"`
	InitialPromptSent bool                  `json:"initial_prompt_sent"`
}

// LoadStateStatus represents the state of loading persisted conversation state.
type LoadStateStatus int

const (
	// LoadStatePending indicates state loading has not been attempted yet.
	LoadStatePending LoadStateStatus = iota
	// LoadStateSucceeded indicates state was successfully loaded.
	LoadStateSucceeded
	// LoadStateFailed indicates state loading was attempted but failed.
	LoadStateFailed
)

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

// outboundMessage wraps a message to be sent with its error channel
type outboundMessage struct {
	parts []MessagePart
	errCh chan error
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
	// ReadyForInitialPrompt detects whether the agent has initialized and is ready to accept the initial prompt
	ReadyForInitialPrompt func(message string) bool
	// FormatToolCall removes the coder report_task tool call from the agent message and also returns the array of removed tool calls
	FormatToolCall func(message string) (string, []string)
	// InitialPrompt is the initial prompt to send to the agent once ready
	InitialPrompt          []MessagePart
	Logger                 *slog.Logger
	StatePersistenceConfig StatePersistenceConfig
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
	cfg     PTYConversationConfig
	emitter Emitter
	// How many stable snapshots are required to consider the screen stable
	stableSnapshotsThreshold    int
	snapshotBuffer              *RingBuffer[screenSnapshot]
	messages                    []ConversationMessage
	screenBeforeLastUserMessage string
	lock                        sync.Mutex

	// outboundQueue holds messages waiting to be sent to the agent.
	// Buffer size is 1. Callers are expected to be serialized (the HTTP
	// layer holds s.mu, and Send blocks until the message is processed),
	// so ordering is preserved.
	outboundQueue chan outboundMessage
	// sendingMessage is true while the send loop is processing a message.
	// Set under lock in the snapshot loop when signaling, cleared under
	// lock in the send loop after sendMessage returns.
	sendingMessage bool
	// stableSignal is used by the snapshot loop to signal the send loop
	// when the agent is stable and there are items in the outbound queue.
	stableSignal chan struct{}
	// toolCallMessageSet keeps track of the tool calls that have been detected & logged in the current agent message
	toolCallMessageSet map[string]bool
	// dirty tracks whether the conversation state has changed since the last save
	dirty bool
	// userSentMessageAfterLoadState tracks if the user has sent their first message after we load the state
	userSentMessageAfterLoadState bool
	// loadStateStatus tracks the status of loading conversation state from file.
	loadStateStatus LoadStateStatus
	// initialPromptReady is set to true when ReadyForInitialPrompt returns true.
	// Checked inline in the snapshot loop on each tick.
	initialPromptReady bool
	// initialPromptSent is set to true when the initial prompt has been enqueued to the outbound queue.
	initialPromptSent bool
}

var _ Conversation = &PTYConversation{}

type noopEmitter struct{}

func (noopEmitter) EmitMessages([]ConversationMessage) {}
func (noopEmitter) EmitStatus(ConversationStatus)      {}
func (noopEmitter) EmitScreen(string)                  {}
func (noopEmitter) EmitError(_ string, _ ErrorLevel)   {}

func NewPTY(ctx context.Context, cfg PTYConversationConfig, emitter Emitter) *PTYConversation {
	if cfg.Clock == nil {
		cfg.Clock = quartz.NewReal()
	}
	if emitter == nil {
		emitter = noopEmitter{}
	}
	threshold := cfg.getStableSnapshotsThreshold()
	c := &PTYConversation{
		cfg:                      cfg,
		emitter:                  emitter,
		stableSnapshotsThreshold: threshold,
		snapshotBuffer:           NewRingBuffer[screenSnapshot](threshold),
		messages: []ConversationMessage{
			{
				Message: "",
				Role:    ConversationRoleAgent,
				Time:    cfg.Clock.Now(),
			},
		},
		outboundQueue:                 make(chan outboundMessage, 1),
		stableSignal:                  make(chan struct{}, 1),
		toolCallMessageSet:            make(map[string]bool),
		dirty:                         false,
		userSentMessageAfterLoadState: false,
		loadStateStatus:               LoadStatePending,
	}
	if c.cfg.ReadyForInitialPrompt == nil {
		c.cfg.ReadyForInitialPrompt = func(string) bool { return true }
	}
	return c
}

func (c *PTYConversation) Start(ctx context.Context) {
	// Snapshot loop
	c.cfg.Clock.TickerFunc(ctx, c.cfg.SnapshotInterval, func() error {
		c.lock.Lock()
		screen := c.cfg.AgentIO.ReadScreen()
		c.snapshotLocked(screen)
		status := c.statusLocked()
		messages := c.messagesLocked()

		// Signal send loop if agent is ready and queue has items.
		// We check readiness independently of statusLocked() because
		// statusLocked() returns "changing" when queue has items.
		if !c.initialPromptReady && c.cfg.ReadyForInitialPrompt(screen) {
			c.initialPromptReady = true
		}

		var loadErr string
		if c.initialPromptReady && c.loadStateStatus == LoadStatePending && c.cfg.StatePersistenceConfig.LoadState {
			if err := c.loadStateLocked(); err != nil {
				c.cfg.Logger.Error("Failed to load state", "error", err)
				loadErr = fmt.Sprintf("Failed to restore previous session: %v", err)
				c.loadStateStatus = LoadStateFailed
			} else {
				c.loadStateStatus = LoadStateSucceeded
			}
		}

		if c.initialPromptReady && len(c.cfg.InitialPrompt) > 0 && !c.initialPromptSent {
			// Safe to send under lock: the queue is guaranteed empty here because
			// statusLocked blocks Send until the snapshot buffer fills, which
			// cannot happen before this first enqueue completes.
			c.outboundQueue <- outboundMessage{parts: c.cfg.InitialPrompt, errCh: nil}
			c.initialPromptSent = true
			c.dirty = true
		}

		if c.initialPromptReady && len(c.outboundQueue) > 0 && c.isScreenStableLocked() {
			select {
			case c.stableSignal <- struct{}{}:
				c.sendingMessage = true
			default:
				// Signal already pending
			}
		}
		c.lock.Unlock()

		if loadErr != "" {
			c.emitter.EmitError(loadErr, ErrorLevelWarning)
		}
		c.emitter.EmitStatus(status)
		c.emitter.EmitMessages(messages)
		c.emitter.EmitScreen(screen)
		return nil
	}, "snapshot")

	// Send loop - primary call site for sendLocked() in production
	go func() {
		defer func() {
			// Drain outbound queue so Send() callers don't block forever.
			for {
				select {
				case msg := <-c.outboundQueue:
					if msg.errCh != nil {
						msg.errCh <- ctx.Err()
						close(msg.errCh)
					}
				default:
					return
				}
			}
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.stableSignal:
				select {
				case <-ctx.Done():
					return
				case msg := <-c.outboundQueue:
					err := c.sendMessage(ctx, msg.parts...)
					c.lock.Lock()
					c.sendingMessage = false
					c.lock.Unlock()
					if msg.errCh != nil {
						msg.errCh <- err
						// Close so the Send() caller's <-errCh never blocks
						// if it has already consumed the error value.
						close(msg.errCh)
					}
				default:
					c.cfg.Logger.Error("received stable signal but outbound queue is empty")
				}
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
	if c.loadStateStatus == LoadStateSucceeded && !c.userSentMessageAfterLoadState && len(c.messages) > 0 &&
		c.messages[len(c.messages)-1].Role == ConversationRoleAgent {
		agentMessage = c.messages[len(c.messages)-1].Message
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

	c.dirty = true
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
	// Validate message content before enqueueing
	message := buildStringFromMessageParts(messageParts)
	if message != msgfmt.TrimWhitespace(message) {
		return ErrMessageValidationWhitespace
	}
	if message == "" {
		return ErrMessageValidationEmpty
	}

	c.lock.Lock()
	if c.statusLocked() != ConversationStatusStable {
		c.lock.Unlock()
		return ErrMessageValidationChanging
	}
	c.lock.Unlock()

	errCh := make(chan error, 1)
	c.outboundQueue <- outboundMessage{parts: messageParts, errCh: errCh}
	return <-errCh
}

// sendMessage sends a message to the agent. It acquires and releases c.lock
// around the parts that access shared state, but releases it during
// writeStabilize to avoid blocking the snapshot loop.
func (c *PTYConversation) sendMessage(ctx context.Context, messageParts ...MessagePart) error {
	message := buildStringFromMessageParts(messageParts)

	c.lock.Lock()
	screenBeforeMessage := c.cfg.AgentIO.ReadScreen()
	now := c.cfg.Clock.Now()
	c.updateLastAgentMessageLocked(screenBeforeMessage, now)
	c.lock.Unlock()

	if err := c.writeStabilize(ctx, messageParts...); err != nil {
		return xerrors.Errorf("failed to send message: %w", err)
	}

	c.lock.Lock()
	// Re-apply the pre-send agent message from the screen captured before
	// the write. While the lock was released during writeStabilize, the
	// snapshot loop continued taking snapshots and calling
	// updateLastAgentMessageLocked with whatever was on screen at each
	// tick (typically echoed user input or intermediate terminal state).
	// Those updates corrupt the agent message for this turn. Restoring it
	// here ensures the conversation history is correct. The next line sets
	// screenBeforeLastUserMessage so the *next* agent message will be
	// diffed relative to the pre-send screen.
	c.updateLastAgentMessageLocked(screenBeforeMessage, now)
	c.screenBeforeLastUserMessage = screenBeforeMessage
	c.messages = append(c.messages, ConversationMessage{
		Id:      len(c.messages),
		Message: message,
		Role:    ConversationRoleUser,
		Time:    now,
	})
	c.userSentMessageAfterLoadState = true

	c.lock.Unlock()
	return nil
}

// writeStabilize writes messageParts to the screen and waits for the screen to stabilize after the message is written.
func (c *PTYConversation) writeStabilize(ctx context.Context, messageParts ...MessagePart) error {
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
		Clock:       c.cfg.Clock,
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
		Clock:       c.cfg.Clock,
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

// isScreenStableLocked returns true if the screen content has been stable
// for the required number of snapshots. Caller MUST hold c.lock.
func (c *PTYConversation) isScreenStableLocked() bool {
	snapshots := c.snapshotBuffer.GetAll()
	if len(snapshots) < c.stableSnapshotsThreshold {
		return false
	}
	for i := 1; i < len(snapshots); i++ {
		if snapshots[0].screen != snapshots[i].screen {
			return false
		}
	}
	return true
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

	if !c.isScreenStableLocked() {
		return ConversationStatusChanging
	}

	// Handle initial prompt readiness: report "changing" until the queue is drained
	// to avoid the status flipping "changing" -> "stable" -> "changing"
	if len(c.outboundQueue) > 0 || c.sendingMessage {
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

func (c *PTYConversation) SaveState() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	stateFile := c.cfg.StatePersistenceConfig.StateFile
	saveState := c.cfg.StatePersistenceConfig.SaveState

	if !saveState {
		c.cfg.Logger.Info("State persistence is disabled")
		return nil
	}

	// Skip if not dirty
	if !c.dirty {
		c.cfg.Logger.Info("Skipping state save: no changes since last save")
		return nil
	}

	conversation := c.messagesLocked()

	// Serialize initial prompt from message parts
	var initialPromptStr string
	if len(c.cfg.InitialPrompt) > 0 {
		initialPromptStr = buildStringFromMessageParts(c.cfg.InitialPrompt)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(stateFile)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return xerrors.Errorf("failed to create state directory: %w", err)
	}

	// Use atomic write: write to temp file, then rename to target path
	tempFile := stateFile + ".tmp"
	f, err := os.OpenFile(tempFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return xerrors.Errorf("failed to create temp state file: %w", err)
	}

	// Clean up temp file on error (before successful rename)
	var renamed bool
	defer func() {
		if !renamed {
			if removeErr := os.Remove(tempFile); removeErr != nil && !os.IsNotExist(removeErr) {
				c.cfg.Logger.Warn("Failed to clean up temp state file", "path", tempFile, "err", removeErr)
			}
		}
	}()

	// Encode directly to file to avoid loading entire JSON into memory
	encoder := json.NewEncoder(f)
	if err := encoder.Encode(AgentState{
		Version:           1,
		Messages:          conversation,
		InitialPrompt:     initialPromptStr,
		InitialPromptSent: c.initialPromptSent,
	}); err != nil {
		_ = f.Close()
		return xerrors.Errorf("failed to encode state: %w", err)
	}

	// Flush to disk before rename for crash safety
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return xerrors.Errorf("failed to sync state file: %w", err)
	}

	// Close file before rename
	if err := f.Close(); err != nil {
		return xerrors.Errorf("failed to close temp state file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, stateFile); err != nil {
		return xerrors.Errorf("failed to rename state file: %w", err)
	}
	renamed = true

	// Clear dirty flag after successful save
	c.dirty = false

	c.cfg.Logger.Info("State saved successfully", "path", stateFile)

	return nil
}

// loadStateLocked loads the state, this method assumes that caller holds the Lock
func (c *PTYConversation) loadStateLocked() error {
	stateFile := c.cfg.StatePersistenceConfig.StateFile
	loadState := c.cfg.StatePersistenceConfig.LoadState

	if !loadState || c.loadStateStatus != LoadStatePending {
		return nil
	}

	// Check if file exists
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		c.cfg.Logger.Info("No previous state to load (file does not exist)", "path", stateFile)
		return xerrors.Errorf("No previous state to load (file does not exist)")
	}

	// Open state file
	f, err := os.Open(stateFile)
	if err != nil {
		return xerrors.Errorf("failed to open state file: %w", err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			c.cfg.Logger.Warn("Failed to close state file", "path", stateFile, "err", closeErr)
		}
	}()

	var agentState AgentState
	decoder := json.NewDecoder(f)
	if err := decoder.Decode(&agentState); err != nil {
		return xerrors.Errorf("failed to unmarshal state (corrupted or invalid JSON): %w", err)
	}

	// Validate version
	if agentState.Version != 1 {
		return xerrors.Errorf("unsupported state file version %d (expected 1)", agentState.Version)
	}

	// Handle initial prompt restoration:
	// - If a new initial prompt was provided via flags, check if it differs from the saved one.
	//   If different, mark as not sent (will be sent). If same, preserve sent status.
	// - If no new prompt provided, restore the saved prompt and its sent status.
	c.initialPromptSent = agentState.InitialPromptSent
	if len(c.cfg.InitialPrompt) > 0 {
		isDifferent := buildStringFromMessageParts(c.cfg.InitialPrompt) != agentState.InitialPrompt
		if isDifferent {
			c.initialPromptSent = false
		}
		// If same prompt, keep agentState.InitialPromptSent
	} else if agentState.InitialPrompt != "" {
		c.cfg.InitialPrompt = []MessagePart{MessagePartText{
			Content: agentState.InitialPrompt,
			Alias:   "",
			Hidden:  false,
		}}
	}

	c.messages = agentState.Messages

	c.dirty = false

	c.cfg.Logger.Info("Successfully loaded state", "path", stateFile, "messages", len(c.messages))
	return nil
}
