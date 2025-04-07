package screentracker

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"
	"golang.org/x/xerrors"
)

type screenSnapshot struct {
	timestamp time.Time
	screen    string
}

type ConversationConfig struct {
	// GetScreen returns the current screen snapshot
	GetScreen func() string
	// SendMessage sends a message to the conversation
	SendMessage func(message string) error
	// GetTime returns the current time
	GetTime func() time.Time
	// How often to take a snapshot for the stability check
	SnapshotInterval time.Duration
	// How long the screen should not change to be considered stable
	ScreenStabilityLength time.Duration
}

type ConversationRole string

const (
	ConversationRoleUser  ConversationRole = "user"
	ConversationRoleAgent ConversationRole = "agent"
)

type ConversationMessage struct {
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

func NewConversation(ctx context.Context, cfg ConversationConfig) *Conversation {
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
				c.AddSnapshot(c.cfg.GetScreen())
			}
		}
	}()
}

func findNewMessage(oldScreen, newScreen string) string {
	dmp := diffmatchpatch.New()
	commonOverlapLength := dmp.DiffCommonOverlap(oldScreen, newScreen)
	newText := newScreen[commonOverlapLength:]
	lines := strings.Split(newText, "\n")

	// remove leading and trailing lines which are empty or have only whitespace
	startLine := 0
	endLine := len(lines) - 1
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "" {
			startLine = i
			break
		}
	}
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			endLine = i
			break
		}
	}
	return strings.Join(lines[startLine:endLine+1], "\n")
}

// This function assumes that the caller holds the lock
func (c *Conversation) updateLastAgentMessage(screen string, timestamp time.Time) {
	agentMessage := findNewMessage(c.screenBeforeLastUserMessage, screen)
	shouldCreateNewMessage := len(c.messages) == 0 || c.messages[len(c.messages)-1].Role == ConversationRoleUser
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
}

func (c *Conversation) AddSnapshot(screen string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	snapshot := screenSnapshot{
		timestamp: c.cfg.GetTime(),
		screen:    screen,
	}
	c.snapshotBuffer.Add(snapshot)
	c.updateLastAgentMessage(screen, snapshot.timestamp)
}

func (c *Conversation) SendMessage(message string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	screenBeforeMessage := c.cfg.GetScreen()
	now := c.cfg.GetTime()
	c.updateLastAgentMessage(screenBeforeMessage, now)
	if err := c.cfg.SendMessage(message); err != nil {
		return xerrors.Errorf("failed to send message: %w", err)
	}
	c.screenBeforeLastUserMessage = screenBeforeMessage
	c.messages = append(c.messages, ConversationMessage{
		Message: message,
		Role:    ConversationRoleUser,
		Time:    now,
	})
	return nil
}

func (c *Conversation) Status() ConversationStatus {
	c.lock.Lock()
	defer c.lock.Unlock()

	// sanity checks
	if c.snapshotBuffer.Capacity() != c.stableSnapshotsThreshold {
		panic(fmt.Sprintf("snapshot buffer capacity %d is not equal to snapshot threshold %d. can't check stability", c.snapshotBuffer.Capacity(), c.stableSnapshotsThreshold))
	}
	if c.stableSnapshotsThreshold == 0 {
		panic("stable snapshots threshold is 0. can't check stability")
	}

	snapshots := c.snapshotBuffer.GetAll()
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

func (c *Conversation) Messages() []ConversationMessage {
	c.lock.Lock()
	defer c.lock.Unlock()

	result := make([]ConversationMessage, len(c.messages))
	copy(result, c.messages)
	return result
}
