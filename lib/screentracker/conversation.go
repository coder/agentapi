package screentracker

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/coder/agentapi/lib/msgfmt"
	"github.com/coder/agentapi/lib/util"
	"github.com/danielgtaylor/huma/v2"
	"golang.org/x/xerrors"
)

type screenSnapshot struct {
	timestamp time.Time
	screen    string
}

type AgentIO interface {
	Write(data []byte) (int, error)
	ReadScreen() string
}

type ConversationConfig struct {
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
}

type ConversationRole string

const (
	ConversationRoleUser  ConversationRole = "user"
	ConversationRoleAgent ConversationRole = "agent"
)

var ConversationRoleValues = []ConversationRole{
	ConversationRoleUser,
	ConversationRoleAgent,
}

func (c ConversationRole) Schema(r huma.Registry) *huma.Schema {
	return util.OpenAPISchema(r, "ConversationRole", ConversationRoleValues)
}

type ConversationMessage struct {
	Id      int
	Message string
	Role    ConversationRole
	Time    time.Time
}

type Conversation struct {
	cfg ConversationConfig
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
}

type ConversationStatus string

const (
	ConversationStatusChanging     ConversationStatus = "changing"
	ConversationStatusStable       ConversationStatus = "stable"
	ConversationStatusInitializing ConversationStatus = "initializing"
)

func getStableSnapshotsThreshold(cfg ConversationConfig) int {
	length := cfg.ScreenStabilityLength.Milliseconds()
	interval := cfg.SnapshotInterval.Milliseconds()
	threshold := int(length / interval)
	if length%interval != 0 {
		threshold++
	}
	return threshold + 1
}

func NewConversation(ctx context.Context, cfg ConversationConfig, initialPrompt string) *Conversation {
	threshold := getStableSnapshotsThreshold(cfg)
	c := &Conversation{
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
		InitialPrompt:     initialPrompt,
		InitialPromptSent: len(initialPrompt) == 0,
	}
	return c
}

func (c *Conversation) StartSnapshotLoop(ctx context.Context) {
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
				c.addSnapshotInner(screen)
				c.lock.Unlock()
			}
		}
	}()
}

func FindNewMessage(oldScreen, newScreen string, agentType msgfmt.AgentType) string {
	oldLines := strings.Split(oldScreen, "\n")
	newLines := strings.Split(newScreen, "\n")
	oldLinesMap := make(map[string]bool)

	// -1 indicates no header
	dynamicHeaderEnd := -1

	// Skip header lines for Opencode agent type to avoid false positives
	// The header contains dynamic content (token count, context percentage, cost)
	// that changes between screens, causing line comparison mismatches:
	//
	// ┃  # Getting Started with Claude CLI                                   ┃
	// ┃  /share to create a shareable link                 12.6K/6% ($0.05)  ┃
	if len(newLines) >= 2 && agentType == msgfmt.AgentTypeOpencode {
		dynamicHeaderEnd = 2
	}

	for _, line := range oldLines {
		oldLinesMap[line] = true
	}
	firstNonMatchingLine := len(newLines)
	for i, line := range newLines[dynamicHeaderEnd+1:] {
		if !oldLinesMap[line] {
			firstNonMatchingLine = i
			break
		}
	}
	newSectionLines := newLines[firstNonMatchingLine:]

	// remove leading and trailing lines which are empty or have only whitespace
	startLine := 0
	endLine := len(newSectionLines) - 1
	for i := 0; i < len(newSectionLines); i++ {
		if strings.TrimSpace(newSectionLines[i]) != "" {
			startLine = i
			break
		}
	}
	for i := len(newSectionLines) - 1; i >= 0; i-- {
		if strings.TrimSpace(newSectionLines[i]) != "" {
			endLine = i
			break
		}
	}
	return strings.Join(newSectionLines[startLine:endLine+1], "\n")
}

func (c *Conversation) lastMessage(role ConversationRole) ConversationMessage {
	for i := len(c.messages) - 1; i >= 0; i-- {
		if c.messages[i].Role == role {
			return c.messages[i]
		}
	}
	return ConversationMessage{}
}

// This function assumes that the caller holds the lock
func (c *Conversation) updateLastAgentMessage(screen string, timestamp time.Time) {
	agentMessage := FindNewMessage(c.screenBeforeLastUserMessage, screen, c.cfg.AgentType)
	lastUserMessage := c.lastMessage(ConversationRoleUser)
	if c.cfg.FormatMessage != nil {
		agentMessage = c.cfg.FormatMessage(agentMessage, lastUserMessage.Message)
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
	} else {
		c.messages[len(c.messages)-1] = conversationMessage
	}
	c.messages[len(c.messages)-1].Id = len(c.messages) - 1
}

// assumes the caller holds the lock
func (c *Conversation) addSnapshotInner(screen string) {
	snapshot := screenSnapshot{
		timestamp: c.cfg.GetTime(),
		screen:    screen,
	}
	c.snapshotBuffer.Add(snapshot)
	c.updateLastAgentMessage(screen, snapshot.timestamp)
}

func (c *Conversation) AddSnapshot(screen string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.addSnapshotInner(screen)
}

type MessagePart interface {
	Do(writer AgentIO) error
	String() string
}

type MessagePartText struct {
	Content string
	Alias   string
	Hidden  bool
}

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

func PartsToString(parts ...MessagePart) string {
	var sb strings.Builder
	for _, part := range parts {
		sb.WriteString(part.String())
	}
	return sb.String()
}

func ExecuteParts(writer AgentIO, parts ...MessagePart) error {
	for _, part := range parts {
		if err := part.Do(writer); err != nil {
			return xerrors.Errorf("failed to write message part: %w", err)
		}
	}
	return nil
}

func (c *Conversation) writeMessageWithConfirmation(ctx context.Context, messageParts ...MessagePart) error {
	if c.cfg.SkipWritingMessage {
		return nil
	}
	screenBeforeMessage := c.cfg.AgentIO.ReadScreen()
	if err := ExecuteParts(c.cfg.AgentIO, messageParts...); err != nil {
		return xerrors.Errorf("failed to write message: %w", err)
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

var MessageValidationErrorWhitespace = xerrors.New("message must be trimmed of leading and trailing whitespace")
var MessageValidationErrorEmpty = xerrors.New("message must not be empty")
var MessageValidationErrorChanging = xerrors.New("message can only be sent when the agent is waiting for user input")

func (c *Conversation) SendMessage(messageParts ...MessagePart) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.cfg.SkipSendMessageStatusCheck && c.statusInner() != ConversationStatusStable {
		return MessageValidationErrorChanging
	}

	message := PartsToString(messageParts...)
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
	c.updateLastAgentMessage(screenBeforeMessage, now)

	if err := c.writeMessageWithConfirmation(context.Background(), messageParts...); err != nil {
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

// Assumes that the caller holds the lock
func (c *Conversation) statusInner() ConversationStatus {
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
	return ConversationStatusStable
}

func (c *Conversation) Status() ConversationStatus {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.statusInner()
}

func (c *Conversation) Messages() []ConversationMessage {
	c.lock.Lock()
	defer c.lock.Unlock()

	result := make([]ConversationMessage, len(c.messages))
	copy(result, c.messages)
	return result
}

func (c *Conversation) Screen() string {
	c.lock.Lock()
	defer c.lock.Unlock()

	snapshots := c.snapshotBuffer.GetAll()
	if len(snapshots) == 0 {
		return ""
	}
	return snapshots[len(snapshots)-1].screen
}
