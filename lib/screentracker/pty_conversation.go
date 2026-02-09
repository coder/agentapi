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

	// outboundQueue holds messages waiting to be sent to the agent
	outboundQueue chan outboundMessage
	// stableSignal is used by the snapshot loop to signal the send loop
	// when the agent is stable and there are items in the outbound queue.
	stableSignal chan struct{}
	// toolCallMessageSet keeps track of the tool calls that have been detected & logged in the current agent message
	toolCallMessageSet map[string]bool
	// initialPromptReady is closed when ReadyForInitialPrompt returns true.
	// This is checked by a separate goroutine to avoid calling ReadyForInitialPrompt on every tick.
	initialPromptReady chan struct{}
	// started is set when Start() is called, enabling the send loop
	started bool
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
		outboundQueue:      make(chan outboundMessage, 1),
		stableSignal:       make(chan struct{}, 1),
		toolCallMessageSet: make(map[string]bool),
		initialPromptReady: make(chan struct{}),
	}
	// If we have an initial prompt, enqueue it
	if len(cfg.InitialPrompt) > 0 {
		c.outboundQueue <- outboundMessage{parts: cfg.InitialPrompt, errCh: nil}
	}
	if c.cfg.OnSnapshot == nil {
		c.cfg.OnSnapshot = func(ConversationStatus, []ConversationMessage, string) {}
	}
	if c.cfg.ReadyForInitialPrompt == nil {
		c.cfg.ReadyForInitialPrompt = func(string) bool { return true }
	}
	return c
}

func (c *PTYConversation) Start(ctx context.Context) {
	c.lock.Lock()
	c.started = true
	c.lock.Unlock()

	// Initial prompt readiness loop - polls ReadyForInitialPrompt until it returns true,
	// then sets initialPromptReady and exits. This avoids calling ReadyForInitialPrompt
	// on every snapshot tick.
	go func() {
		ticker := c.cfg.Clock.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				screen := c.cfg.AgentIO.ReadScreen()
				if c.cfg.ReadyForInitialPrompt(screen) {
					close(c.initialPromptReady)
					return
				}
			}
		}
	}()

	// Snapshot loop
	go func() {
		ticker := c.cfg.Clock.NewTicker(c.cfg.SnapshotInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.lock.Lock()
				screen := c.cfg.AgentIO.ReadScreen()
				c.snapshotLocked(screen)
				status := c.statusLocked()
				messages := c.messagesLocked()

				// Signal send loop if agent is ready and queue has items.
				// We check readiness independently of statusLocked() because
				// statusLocked() returns "changing" when queue has items.
				isReady := false
				select {
				case <-c.initialPromptReady:
					isReady = true
				default:
				}
				if len(c.outboundQueue) > 0 && c.isScreenStableLocked() && isReady {
					select {
					case c.stableSignal <- struct{}{}:
					default:
						// Signal already pending
					}
				}
				c.lock.Unlock()

				c.cfg.OnSnapshot(status, messages, screen)
			}
		}
	}()

	// Send loop - primary call site for sendLocked() in production
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.stableSignal:
				select {
				case <-ctx.Done():
					return
				case msg := <-c.outboundQueue:
					c.lock.Lock()
					err := c.sendLocked(msg.parts...)
					c.lock.Unlock()
					if msg.errCh != nil {
						msg.errCh <- err
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
	// Validate message content before enqueueing
	var sb strings.Builder
	for _, part := range messageParts {
		sb.WriteString(part.String())
	}
	message := sb.String()
	if message != msgfmt.TrimWhitespace(message) {
		return ErrMessageValidationWhitespace
	}
	if message == "" {
		return ErrMessageValidationEmpty
	}

	c.lock.Lock()
	if !c.cfg.SkipSendMessageStatusCheck && c.statusLocked() != ConversationStatusStable {
		c.lock.Unlock()
		return ErrMessageValidationChanging
	}
	// If Start() hasn't been called, send directly (for tests)
	if !c.started {
		err := c.sendLocked(messageParts...)
		c.lock.Unlock()
		return err
	}
	c.lock.Unlock()

	errCh := make(chan error, 1)
	c.outboundQueue <- outboundMessage{parts: messageParts, errCh: errCh}
	return <-errCh
}

// sendLocked sends a message to the agent. Caller MUST hold c.lock.
// Validation is done by the caller (Send() validates, initial prompt is trusted).
func (c *PTYConversation) sendLocked(messageParts ...MessagePart) error {
	var sb strings.Builder
	for _, part := range messageParts {
		sb.WriteString(part.String())
	}
	message := sb.String()

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
	if len(c.outboundQueue) > 0 {
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
