package screentracker_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/coder/quartz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	st "github.com/coder/agentapi/lib/screentracker"
)

const testTimeout = 10 * time.Second

// testAgent is a goroutine-safe mock implementation of AgentIO.
type testAgent struct {
	mu      sync.Mutex
	screen  string
	onWrite func(data []byte)
}

func (a *testAgent) ReadScreen() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.screen
}

func (a *testAgent) Write(data []byte) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.onWrite != nil {
		a.onWrite(data)
	}
	return len(data), nil
}

func (a *testAgent) setScreen(s string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.screen = s
}

// advancePast advances the mock clock by total, stepping through each
// intermediate event so that TickerFunc callbacks run to completion.
func advancePast(ctx context.Context, t *testing.T, mClock *quartz.Mock, total time.Duration) {
	t.Helper()
	target := mClock.Now().Add(total)
	for mClock.Now().Before(target) {
		remaining := target.Sub(mClock.Now())
		d, ok := mClock.Peek()
		if !ok || d > remaining {
			mClock.Advance(remaining).MustWait(ctx)
			return
		}
		mClock.Advance(d).MustWait(ctx)
	}
}

// fillToStable sets the screen and advances the clock enough times to fill the
// snapshot buffer, making status reach "stable".
func fillToStable(ctx context.Context, t *testing.T, agent *testAgent, mClock *quartz.Mock, screen string, interval time.Duration, threshold int) {
	t.Helper()
	agent.setScreen(screen)
	for i := 0; i < threshold; i++ {
		advancePast(ctx, t, mClock, interval)
	}
}

// sendWithClockDrive calls Send() in a goroutine and advances the mock clock
// until Send completes. This drives the snapshot loop (which signals
// stableSignal) and writeStabilize (which creates mock timers).
func sendWithClockDrive(ctx context.Context, t *testing.T, c *st.PTYConversation, mClock *quartz.Mock, parts ...st.MessagePart) error {
	t.Helper()
	errCh := make(chan error, 1)
	go func() {
		errCh <- c.Send(parts...)
	}()
	for {
		select {
		case err := <-errCh:
			return err
		default:
		}
		_, w := mClock.AdvanceNext()
		w.MustWait(ctx)
	}
}

func assertMessages(t *testing.T, c *st.PTYConversation, expected []st.ConversationMessage) {
	t.Helper()
	actual := c.Messages()
	require.Len(t, actual, len(expected))
	for i := range expected {
		assert.Equal(t, expected[i].Id, actual[i].Id, "message %d Id", i)
		assert.Equal(t, expected[i].Message, actual[i].Message, "message %d Message", i)
		assert.Equal(t, expected[i].Role, actual[i].Role, "message %d Role", i)
		if expected[i].Time.IsZero() {
			assert.False(t, actual[i].Time.IsZero(), "message %d Time should be non-zero", i)
		} else {
			assert.Equal(t, expected[i].Time, actual[i].Time, "message %d Time", i)
		}
	}
}

type statusTestStep struct {
	snapshot string
	status   st.ConversationStatus
}
type statusTestParams struct {
	cfg   st.PTYConversationConfig
	steps []statusTestStep
}

func statusTest(t *testing.T, params statusTestParams) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	t.Cleanup(cancel)
	t.Run(fmt.Sprintf("interval-%s,stability_length-%s", params.cfg.SnapshotInterval, params.cfg.ScreenStabilityLength), func(t *testing.T) {
		mClock := quartz.NewMock(t)
		params.cfg.Clock = mClock
		agent := &testAgent{}
		if params.cfg.AgentIO != nil {
			if a, ok := params.cfg.AgentIO.(*testAgent); ok {
				agent = a
			}
		}
		params.cfg.AgentIO = agent
		params.cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))

		c := st.NewPTY(ctx, params.cfg)
		c.Start(ctx)

		assert.Equal(t, st.ConversationStatusInitializing, c.Status())

		for i, step := range params.steps {
			agent.setScreen(step.snapshot)
			advancePast(ctx, t, mClock, params.cfg.SnapshotInterval)
			assert.Equal(t, step.status, c.Status(), "step %d", i)
		}
	})
}

func TestConversation(t *testing.T) {
	changing := st.ConversationStatusChanging
	stable := st.ConversationStatusStable
	initializing := st.ConversationStatusInitializing

	statusTest(t, statusTestParams{
		cfg: st.PTYConversationConfig{
			SnapshotInterval:      1 * time.Second,
			ScreenStabilityLength: 2 * time.Second,
			// stability threshold: 3
			AgentIO: &testAgent{
				screen: "1",
			},
		},
		steps: []statusTestStep{
			{snapshot: "1", status: initializing},
			{snapshot: "1", status: initializing},
			{snapshot: "1", status: stable},
			{snapshot: "1", status: stable},
			{snapshot: "2", status: changing},
		},
	})

	statusTest(t, statusTestParams{
		cfg: st.PTYConversationConfig{
			SnapshotInterval:      2 * time.Second,
			ScreenStabilityLength: 3 * time.Second,
			// stability threshold: 3
		},
		steps: []statusTestStep{
			{snapshot: "1", status: initializing},
			{snapshot: "1", status: initializing},
			{snapshot: "1", status: stable},
			{snapshot: "1", status: stable},
			{snapshot: "2", status: changing},
			{snapshot: "2", status: changing},
			{snapshot: "2", status: stable},
			{snapshot: "2", status: stable},
			{snapshot: "2", status: stable},
		},
	})

	statusTest(t, statusTestParams{
		cfg: st.PTYConversationConfig{
			SnapshotInterval:      6 * time.Second,
			ScreenStabilityLength: 14 * time.Second,
			// stability threshold: 4
		},
		steps: []statusTestStep{
			{snapshot: "1", status: initializing},
			{snapshot: "1", status: initializing},
			{snapshot: "1", status: initializing},
			{snapshot: "1", status: stable},
			{snapshot: "1", status: stable},
			{snapshot: "1", status: stable},
			{snapshot: "2", status: changing},
			{snapshot: "2", status: changing},
			{snapshot: "2", status: changing},
			{snapshot: "2", status: stable},
		},
	})
}

func TestMessages(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// newConversation creates a started conversation with a mock clock and
	// testAgent. Tests that Send() messages must use sendWithClockDrive.
	newConversation := func(ctx context.Context, t *testing.T, opts ...func(*st.PTYConversationConfig)) (*st.PTYConversation, *testAgent, *quartz.Mock) {
		t.Helper()

		writeCounter := 0
		agent := &testAgent{}
		// Default onWrite: each write produces a unique screen so that
		// writeStabilize can detect screen changes.
		agent.onWrite = func(data []byte) {
			writeCounter++
			agent.screen = fmt.Sprintf("__write_%d", writeCounter)
		}
		mClock := quartz.NewMock(t)
		mClock.Set(now)
		cfg := st.PTYConversationConfig{
			Clock:                      mClock,
			AgentIO:                    agent,
			SnapshotInterval:           100 * time.Millisecond,
			ScreenStabilityLength:      200 * time.Millisecond,
			Logger:                     slog.New(slog.NewTextHandler(io.Discard, nil)),
		}
		for _, opt := range opts {
			opt(&cfg)
		}
		if a, ok := cfg.AgentIO.(*testAgent); ok {
			agent = a
		}

		c := st.NewPTY(ctx, cfg)
		c.Start(ctx)

		return c, agent, mClock
	}

	// threshold = 3 (200ms / 100ms = 2, + 1 = 3)
	const threshold = 3
	const interval = 100 * time.Millisecond

	t.Run("messages are copied", func(t *testing.T) {
		c, _, _ := newConversation(context.Background(), t)
		messages := c.Messages()
		assertMessages(t, c, []st.ConversationMessage{
			{Id: 0, Message: "", Role: st.ConversationRoleAgent},
		})

		messages[0].Message = "modification"

		assertMessages(t, c, []st.ConversationMessage{
			{Id: 0, Message: "", Role: st.ConversationRoleAgent},
		})
	})

	t.Run("whitespace-padding", func(t *testing.T) {
		c, _, _ := newConversation(context.Background(), t)
		for _, msg := range []string{"123 ", " 123", "123\t\t", "\n123", "123\n\t", " \t123\n\t"} {
			err := c.Send(st.MessagePartText{Content: msg})
			assert.ErrorIs(t, err, st.ErrMessageValidationWhitespace)
		}
	})

	t.Run("no-change-no-message-update", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)
		c, agent, mClock := newConversation(ctx, t)

		agent.setScreen("1")
		advancePast(ctx, t, mClock, interval)
		msgs := c.Messages()
		assertMessages(t, c, []st.ConversationMessage{
			{Id: 0, Message: "1", Role: st.ConversationRoleAgent},
		})

		advancePast(ctx, t, mClock, interval)
		assert.Equal(t, msgs, c.Messages())
	})

	t.Run("tracking messages", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)
		c, agent, mClock := newConversation(ctx, t)

		// Agent message is recorded when the first snapshot is taken.
		fillToStable(ctx, t, agent, mClock, "1", interval, threshold)
		assertMessages(t, c, []st.ConversationMessage{
			{Id: 0, Message: "1", Role: st.ConversationRoleAgent},
		})

		// Agent message is updated when the screen changes.
		agent.setScreen("2")
		advancePast(ctx, t, mClock, interval)
		assertMessages(t, c, []st.ConversationMessage{
			{Id: 0, Message: "2", Role: st.ConversationRoleAgent},
		})

		// Fill to stable so Send can proceed (screen is "2").
		fillToStable(ctx, t, agent, mClock, "2", interval, threshold)

		// User message is recorded.
		require.NoError(t, sendWithClockDrive(ctx, t, c, mClock, st.MessagePartText{Content: "3"}))

		// After send, screen is dirty from writeStabilize. Set to "4" and stabilize.
		fillToStable(ctx, t, agent, mClock, "4", interval, threshold)
		assertMessages(t, c, []st.ConversationMessage{
			{Id: 0, Message: "2", Role: st.ConversationRoleAgent},
			{Id: 1, Message: "3", Role: st.ConversationRoleUser},
			{Id: 2, Message: "4", Role: st.ConversationRoleAgent},
		})

		// Agent message is updated when the screen changes before a user message.
		fillToStable(ctx, t, agent, mClock, "5", interval, threshold)
		require.NoError(t, sendWithClockDrive(ctx, t, c, mClock, st.MessagePartText{Content: "6"}))

		fillToStable(ctx, t, agent, mClock, "7", interval, threshold)
		assertMessages(t, c, []st.ConversationMessage{
			{Id: 0, Message: "2", Role: st.ConversationRoleAgent},
			{Id: 1, Message: "3", Role: st.ConversationRoleUser},
			{Id: 2, Message: "5", Role: st.ConversationRoleAgent},
			{Id: 3, Message: "6", Role: st.ConversationRoleUser},
			{Id: 4, Message: "7", Role: st.ConversationRoleAgent},
		})
		assert.Equal(t, st.ConversationStatusStable, c.Status())

		// Send another message.
		require.NoError(t, sendWithClockDrive(ctx, t, c, mClock, st.MessagePartText{Content: "8"}))

		// After filling to stable, messages and status are correct.
		fillToStable(ctx, t, agent, mClock, "7", interval, threshold)
		assert.Equal(t, st.ConversationStatusStable, c.Status())
	})

	t.Run("tracking messages overlap", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)
		c, agent, mClock := newConversation(ctx, t)

		// Common overlap between screens is removed after a user message.
		fillToStable(ctx, t, agent, mClock, "1", interval, threshold)
		require.NoError(t, sendWithClockDrive(ctx, t, c, mClock, st.MessagePartText{Content: "2"}))
		fillToStable(ctx, t, agent, mClock, "1\n3", interval, threshold)
		assertMessages(t, c, []st.ConversationMessage{
			{Id: 0, Message: "1", Role: st.ConversationRoleAgent},
			{Id: 1, Message: "2", Role: st.ConversationRoleUser},
			{Id: 2, Message: "3", Role: st.ConversationRoleAgent},
		})

		fillToStable(ctx, t, agent, mClock, "1\n3x", interval, threshold)
		require.NoError(t, sendWithClockDrive(ctx, t, c, mClock, st.MessagePartText{Content: "4"}))
		fillToStable(ctx, t, agent, mClock, "1\n3x\n5", interval, threshold)
		assertMessages(t, c, []st.ConversationMessage{
			{Id: 0, Message: "1", Role: st.ConversationRoleAgent},
			{Id: 1, Message: "2", Role: st.ConversationRoleUser},
			{Id: 2, Message: "3x", Role: st.ConversationRoleAgent},
			{Id: 3, Message: "4", Role: st.ConversationRoleUser},
			{Id: 4, Message: "5", Role: st.ConversationRoleAgent},
		})
	})

	t.Run("format-message", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)
		c, agent, mClock := newConversation(ctx, t, func(cfg *st.PTYConversationConfig) {
			cfg.FormatMessage = func(message string, userInput string) string {
				return message + " " + userInput
			}
		})

		// Fill to stable with screen "1", then send.
		fillToStable(ctx, t, agent, mClock, "1", interval, threshold)
		require.NoError(t, sendWithClockDrive(ctx, t, c, mClock, st.MessagePartText{Content: "2"}))

		// After send, set screen to "x" and take snapshots for new agent message.
		fillToStable(ctx, t, agent, mClock, "x", interval, threshold)
		assertMessages(t, c, []st.ConversationMessage{
			{Id: 0, Message: "1 ", Role: st.ConversationRoleAgent},
			{Id: 1, Message: "2", Role: st.ConversationRoleUser},
			{Id: 2, Message: "x 2", Role: st.ConversationRoleAgent},
		})
	})

	t.Run("format-message-initial", func(t *testing.T) {
		c, _, _ := newConversation(context.Background(), t, func(cfg *st.PTYConversationConfig) {
			cfg.FormatMessage = func(message string, userInput string) string {
				return "formatted"
			}
		})
		assertMessages(t, c, []st.ConversationMessage{
			{Id: 0, Message: "", Role: st.ConversationRoleAgent},
		})
	})

	t.Run("send-message-status-check", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)
		c, agent, mClock := newConversation(ctx, t)

		sendMsg := func(msg string) error {
			return c.Send(st.MessagePartText{Content: msg})
		}

		// Status is initializing, send should fail.
		assert.ErrorIs(t, sendMsg("1"), st.ErrMessageValidationChanging)

		// Fill to stable.
		fillToStable(ctx, t, agent, mClock, "1", interval, threshold)
		assert.Equal(t, st.ConversationStatusStable, c.Status())

		// Now send should succeed.
		require.NoError(t, sendWithClockDrive(ctx, t, c, mClock, st.MessagePartText{Content: "4"}))

		// After send, screen is dirty. Set to "2" (different from "1") so status is changing.
		agent.setScreen("2")
		advancePast(ctx, t, mClock, interval)
		assert.Equal(t, st.ConversationStatusChanging, c.Status())
		assert.ErrorIs(t, sendMsg("5"), st.ErrMessageValidationChanging)
	})

	t.Run("send-message-empty-message", func(t *testing.T) {
		c, _, _ := newConversation(context.Background(), t)
		assert.ErrorIs(t, c.Send(st.MessagePartText{Content: ""}), st.ErrMessageValidationEmpty)
	})
}

func TestInitialPromptReadiness(t *testing.T) {
	discardLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("agent not ready - status remains changing", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)
		mClock := quartz.NewMock(t)
		agent := &testAgent{screen: "loading..."}
		cfg := st.PTYConversationConfig{
			Clock:                 mClock,
			SnapshotInterval:      1 * time.Second,
			ScreenStabilityLength: 0,
			AgentIO:               agent,
			ReadyForInitialPrompt: func(message string) bool {
				return message == "ready"
			},
			InitialPrompt: []st.MessagePart{st.MessagePartText{Content: "initial prompt here"}},
			Logger:        discardLogger,
		}

		c := st.NewPTY(ctx, cfg)
		c.Start(ctx)

		// Take a snapshot with "loading...". Threshold is 1 (stability 0 / interval 1s = 0 + 1 = 1).
		advancePast(ctx, t, mClock, 1*time.Second)

		// Even though screen is stable, status should be changing because
		// the initial prompt is still in the outbound queue.
		assert.Equal(t, st.ConversationStatusChanging, c.Status())
	})

	t.Run("agent becomes ready - status stays changing until initial prompt sent", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)
		mClock := quartz.NewMock(t)
		agent := &testAgent{screen: "loading..."}
		cfg := st.PTYConversationConfig{
			Clock:                 mClock,
			SnapshotInterval:      1 * time.Second,
			ScreenStabilityLength: 0,
			AgentIO:               agent,
			ReadyForInitialPrompt: func(message string) bool {
				return message == "ready"
			},
			InitialPrompt: []st.MessagePart{st.MessagePartText{Content: "initial prompt here"}},
			Logger:        discardLogger,
		}

		c := st.NewPTY(ctx, cfg)
		c.Start(ctx)

		// Agent not ready initially.
		advancePast(ctx, t, mClock, 1*time.Second)
		assert.Equal(t, st.ConversationStatusChanging, c.Status())

		// Agent becomes ready, but status stays "changing" because the
		// initial prompt is still in the outbound queue.
		agent.setScreen("ready")
		advancePast(ctx, t, mClock, 1*time.Second)
		assert.Equal(t, st.ConversationStatusChanging, c.Status())
	})

	t.Run("initial prompt lifecycle - status stays changing until sent", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)
		mClock := quartz.NewMock(t)
		agent := &testAgent{screen: "loading..."}
		writeCounter := 0
		agent.onWrite = func(data []byte) {
			writeCounter++
			agent.screen = fmt.Sprintf("__write_%d", writeCounter)
		}
		cfg := st.PTYConversationConfig{
			Clock:                      mClock,
			SnapshotInterval:           1 * time.Second,
			ScreenStabilityLength:      0,
			AgentIO:                    agent,
			ReadyForInitialPrompt: func(message string) bool {
				return message == "ready"
			},
			InitialPrompt:             []st.MessagePart{st.MessagePartText{Content: "initial prompt here"}},
			Logger:                     discardLogger,
		}

		c := st.NewPTY(ctx, cfg)
		c.Start(ctx)

		// Status is "changing" while waiting for readiness.
		advancePast(ctx, t, mClock, 1*time.Second)
		assert.Equal(t, st.ConversationStatusChanging, c.Status())

		// Agent becomes ready. The readiness loop detects this, the snapshot
		// loop sees queue + stable + ready and signals the send loop.
		// writeStabilize runs with onWrite changing the screen, so it completes.
		agent.setScreen("ready")
		// Drive clock until the initial prompt is sent (queue drains).
		for i := 0; i < 500; i++ {
			_, ok := mClock.Peek()
			if !ok {
				time.Sleep(1 * time.Millisecond)
				continue
			}
			_, w := mClock.AdvanceNext()
			w.MustWait(ctx)
			// Check if the queue has been drained by checking status.
			// After the initial prompt is sent, last message is user, so status
			// is "changing". Then after more snapshots, it becomes stable.
			// We just need to advance until the send loop has processed the message.
			// A simple heuristic: check if Messages() shows a user message.
			msgs := c.Messages()
			if len(msgs) >= 2 {
				break
			}
		}

		// The initial prompt should have been sent. Set a clean screen and
		// advance enough ticks for the snapshot loop to record it as an
		// agent message and fill the stability buffer (threshold=1).
		agent.setScreen("response")
		advancePast(ctx, t, mClock, 2*time.Second)
		assert.Equal(t, st.ConversationStatusStable, c.Status())
	})

	t.Run("no initial prompt - normal status logic applies", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)
		mClock := quartz.NewMock(t)
		agent := &testAgent{screen: "loading..."}
		cfg := st.PTYConversationConfig{
			Clock:                 mClock,
			SnapshotInterval:      1 * time.Second,
			ScreenStabilityLength: 0,
			AgentIO:               agent,
			ReadyForInitialPrompt: func(message string) bool {
				return false
			},
			Logger: discardLogger,
		}

		c := st.NewPTY(ctx, cfg)
		c.Start(ctx)

		advancePast(ctx, t, mClock, 1*time.Second)

		// Status should be stable because no initial prompt to wait for.
		assert.Equal(t, st.ConversationStatusStable, c.Status())
	})

	t.Run("no initial prompt configured - normal status logic applies", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)
		mClock := quartz.NewMock(t)
		agent := &testAgent{screen: "ready"}
		cfg := st.PTYConversationConfig{
			Clock:                      mClock,
			SnapshotInterval:           1 * time.Second,
			ScreenStabilityLength:      2 * time.Second, // threshold = 3
			AgentIO:                    agent,
			Logger:                     discardLogger,
		}

		c := st.NewPTY(ctx, cfg)
		c.Start(ctx)

		// Fill buffer to reach stability with "ready" screen.
		fillToStable(ctx, t, agent, mClock, "ready", 1*time.Second, 3)
		assert.Equal(t, st.ConversationStatusStable, c.Status())

		// After screen changes, status becomes changing.
		agent.setScreen("processing...")
		advancePast(ctx, t, mClock, 1*time.Second)
		assert.Equal(t, st.ConversationStatusChanging, c.Status())

		// After screen is stable again (3 identical snapshots), status becomes stable.
		advancePast(ctx, t, mClock, 1*time.Second)
		advancePast(ctx, t, mClock, 1*time.Second)
		assert.Equal(t, st.ConversationStatusStable, c.Status())
	})
}
