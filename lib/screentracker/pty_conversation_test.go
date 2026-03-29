package screentracker_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
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
	mu     sync.Mutex
	screen string
	// onWrite is called during Write to simulate the agent reacting to
	// terminal input (e.g., changing the screen), which unblocks
	// writeStabilize's polling loops.
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

type testEmitter struct{}

func (testEmitter) EmitMessages([]st.ConversationMessage) {}
func (testEmitter) EmitStatus(st.ConversationStatus)      {}
func (testEmitter) EmitScreen(string)                     {}
func (testEmitter) EmitError(_ string, _ st.ErrorLevel)   {}

// advanceFor is a shorthand for advanceUntil with a time-based condition.
func advanceFor(ctx context.Context, t *testing.T, mClock *quartz.Mock, total time.Duration) {
	t.Helper()
	target := mClock.Now().Add(total)
	advanceUntil(ctx, t, mClock, func() bool { return !mClock.Now().Before(target) })
}

// advanceUntil advances the mock clock one event at a time until done returns
// true. Because the snapshot TickerFunc is always pending and WaitFor reuses a
// single timer via Reset, there is always at least one event to advance.
func advanceUntil(ctx context.Context, t *testing.T, mClock *quartz.Mock, done func() bool) {
	t.Helper()
	for !done() {
		select {
		case <-ctx.Done():
			t.Fatal("context cancelled waiting for condition")
		default:
		}
		_, w := mClock.AdvanceNext()
		w.MustWait(ctx)
	}
}

// sendAndAdvance calls Send() in a goroutine and advances the mock clock until
// Send completes.
func sendAndAdvance(ctx context.Context, t *testing.T, c *st.PTYConversation, mClock *quartz.Mock, parts ...st.MessagePart) {
	t.Helper()
	errCh := make(chan error, 1)
	go func() {
		errCh <- c.Send(parts...)
	}()
	advanceUntil(ctx, t, mClock, func() bool {
		select {
		case err := <-errCh:
			require.NoError(t, err)
			return true
		default:
			return false
		}
	})
}

func assertMessages(t *testing.T, c *st.PTYConversation, expected []st.ConversationMessage) {
	t.Helper()
	actual := c.Messages()
	for i := range actual {
		require.False(t, actual[i].Time.IsZero(), "message %d Time should be non-zero", i)
		actual[i].Time = time.Time{}
	}
	require.Equal(t, expected, actual)
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

		c := st.NewPTY(ctx, params.cfg, &testEmitter{})
		c.Start(ctx)

		assert.Equal(t, st.ConversationStatusInitializing, c.Status())

		for i, step := range params.steps {
			agent.setScreen(step.snapshot)
			advanceFor(ctx, t, mClock, params.cfg.SnapshotInterval)
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
	// testAgent. Tests that Send() messages must use sendAndAdvance.
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
			Clock:                 mClock,
			AgentIO:               agent,
			SnapshotInterval:      100 * time.Millisecond,
			ScreenStabilityLength: 200 * time.Millisecond,
			Logger:                slog.New(slog.NewTextHandler(io.Discard, nil)),
		}
		for _, opt := range opts {
			opt(&cfg)
		}
		if a, ok := cfg.AgentIO.(*testAgent); ok {
			agent = a
		}

		c := st.NewPTY(ctx, cfg, &testEmitter{})
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
		advanceFor(ctx, t, mClock, interval)
		msgs := c.Messages()
		assertMessages(t, c, []st.ConversationMessage{
			{Id: 0, Message: "1", Role: st.ConversationRoleAgent},
		})

		advanceFor(ctx, t, mClock, interval)
		assert.Equal(t, msgs, c.Messages())
	})

	t.Run("tracking messages", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)
		c, agent, mClock := newConversation(ctx, t)

		// Agent message is recorded when the first snapshot is taken.
		agent.setScreen("1")
		advanceFor(ctx, t, mClock, interval*threshold)
		assertMessages(t, c, []st.ConversationMessage{
			{Id: 0, Message: "1", Role: st.ConversationRoleAgent},
		})

		// Agent message is updated when the screen changes.
		agent.setScreen("2")
		advanceFor(ctx, t, mClock, interval)
		assertMessages(t, c, []st.ConversationMessage{
			{Id: 0, Message: "2", Role: st.ConversationRoleAgent},
		})

		// Fill to stable so Send can proceed (screen is "2").
		agent.setScreen("2")
		advanceFor(ctx, t, mClock, interval*threshold)

		// User message is recorded.
		sendAndAdvance(ctx, t, c, mClock, st.MessagePartText{Content: "3"})

		// After send, screen is dirty from writeStabilize. Set to "4" and stabilize.
		agent.setScreen("4")
		advanceFor(ctx, t, mClock, interval*threshold)
		assertMessages(t, c, []st.ConversationMessage{
			{Id: 0, Message: "2", Role: st.ConversationRoleAgent},
			{Id: 1, Message: "3", Role: st.ConversationRoleUser},
			{Id: 2, Message: "4", Role: st.ConversationRoleAgent},
		})

		// Agent message is updated when the screen changes before a user message.
		agent.setScreen("5")
		advanceFor(ctx, t, mClock, interval*threshold)
		sendAndAdvance(ctx, t, c, mClock, st.MessagePartText{Content: "6"})

		agent.setScreen("7")
		advanceFor(ctx, t, mClock, interval*threshold)
		assertMessages(t, c, []st.ConversationMessage{
			{Id: 0, Message: "2", Role: st.ConversationRoleAgent},
			{Id: 1, Message: "3", Role: st.ConversationRoleUser},
			{Id: 2, Message: "5", Role: st.ConversationRoleAgent},
			{Id: 3, Message: "6", Role: st.ConversationRoleUser},
			{Id: 4, Message: "7", Role: st.ConversationRoleAgent},
		})
		assert.Equal(t, st.ConversationStatusStable, c.Status())

		// Send another message.
		sendAndAdvance(ctx, t, c, mClock, st.MessagePartText{Content: "8"})

		// After filling to stable, messages and status are correct.
		agent.setScreen("7")
		advanceFor(ctx, t, mClock, interval*threshold)
		assert.Equal(t, st.ConversationStatusStable, c.Status())
	})

	t.Run("tracking messages overlap", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)
		c, agent, mClock := newConversation(ctx, t)

		// Common overlap between screens is removed after a user message.
		agent.setScreen("1")
		advanceFor(ctx, t, mClock, interval*threshold)
		sendAndAdvance(ctx, t, c, mClock, st.MessagePartText{Content: "2"})
		agent.setScreen("1\n3")
		advanceFor(ctx, t, mClock, interval*threshold)
		assertMessages(t, c, []st.ConversationMessage{
			{Id: 0, Message: "1", Role: st.ConversationRoleAgent},
			{Id: 1, Message: "2", Role: st.ConversationRoleUser},
			{Id: 2, Message: "3", Role: st.ConversationRoleAgent},
		})

		agent.setScreen("1\n3x")
		advanceFor(ctx, t, mClock, interval*threshold)
		sendAndAdvance(ctx, t, c, mClock, st.MessagePartText{Content: "4"})
		agent.setScreen("1\n3x\n5")
		advanceFor(ctx, t, mClock, interval*threshold)
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
		agent.setScreen("1")
		advanceFor(ctx, t, mClock, interval*threshold)
		sendAndAdvance(ctx, t, c, mClock, st.MessagePartText{Content: "2"})

		// After send, set screen to "x" and take snapshots for new agent message.
		agent.setScreen("x")
		advanceFor(ctx, t, mClock, interval*threshold)
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
		agent.setScreen("1")
		advanceFor(ctx, t, mClock, interval*threshold)
		assert.Equal(t, st.ConversationStatusStable, c.Status())

		// Now send should succeed.
		sendAndAdvance(ctx, t, c, mClock, st.MessagePartText{Content: "4"})

		// After send, screen is dirty. Set to "2" (different from "1") so status is changing.
		agent.setScreen("2")
		advanceFor(ctx, t, mClock, interval)
		assert.Equal(t, st.ConversationStatusChanging, c.Status())
		assert.ErrorIs(t, sendMsg("5"), st.ErrMessageValidationChanging)
	})

	t.Run("send-message-empty-message", func(t *testing.T) {
		c, _, _ := newConversation(context.Background(), t)
		assert.ErrorIs(t, c.Send(st.MessagePartText{Content: ""}), st.ErrMessageValidationEmpty)
	})
}

func TestStatePersistence(t *testing.T) {
	t.Run("SaveState creates file with correct structure", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)

		// Create temp directory for state file
		tmpDir := t.TempDir()
		stateFile := tmpDir + "/state.json"

		mClock := quartz.NewMock(t)
		agent := &testAgent{screen: "initial"}
		cfg := st.PTYConversationConfig{
			Clock:                 mClock,
			SnapshotInterval:      100 * time.Millisecond,
			ScreenStabilityLength: 200 * time.Millisecond,
			AgentIO:               agent,
			Logger:                slog.New(slog.NewTextHandler(io.Discard, nil)),
			StatePersistenceConfig: st.StatePersistenceConfig{
				StateFile: stateFile,
				LoadState: false,
				SaveState: true,
			},
			InitialPrompt: []st.MessagePart{st.MessagePartText{Content: "test prompt"}},
		}

		c := st.NewPTY(ctx, cfg, &testEmitter{})
		c.Start(ctx)

		// Generate some conversation
		agent.setScreen("hello")
		advanceFor(ctx, t, mClock, 300*time.Millisecond)

		// Save state
		err := c.SaveState()
		require.NoError(t, err)

		// Read and verify the saved file
		data, err := os.ReadFile(stateFile)
		require.NoError(t, err)

		var agentState st.AgentState
		err = json.Unmarshal(data, &agentState)
		require.NoError(t, err)

		assert.Equal(t, 1, agentState.Version)
		assert.Equal(t, "test prompt", agentState.InitialPrompt)
		assert.NotEmpty(t, agentState.Messages)
	})

	t.Run("SaveState creates valid JSON", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)

		tmpDir := t.TempDir()
		stateFile := tmpDir + "/state.json"

		mClock := quartz.NewMock(t)
		fixedTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		mClock.Set(fixedTime)

		agent := &testAgent{screen: ""}
		writeCounter := 0
		agent.onWrite = func(data []byte) {
			writeCounter++
			// Change screen on each write so writeStabilize can detect changes
			agent.screen = fmt.Sprintf("__write_%d", writeCounter)
		}

		cfg := st.PTYConversationConfig{
			Clock:                 mClock,
			SnapshotInterval:      100 * time.Millisecond,
			ScreenStabilityLength: 200 * time.Millisecond,
			AgentIO:               agent,
			Logger:                slog.New(slog.NewTextHandler(io.Discard, nil)),
			StatePersistenceConfig: st.StatePersistenceConfig{
				StateFile: stateFile,
				LoadState: false,
				SaveState: true,
			},
			InitialPrompt: []st.MessagePart{st.MessagePartText{Content: "test prompt"}},
			ReadyForInitialPrompt: func(message string) bool {
				return message == "Hello! Ready to help."
			},
		}

		c := st.NewPTY(ctx, cfg, &testEmitter{})
		c.Start(ctx)

		// Step 1: Agent shows initial greeting
		agent.setScreen("Hello! Ready to help.")
		advanceFor(ctx, t, mClock, 300*time.Millisecond)

		// Step 2: Wait for initial prompt to be sent (uses advanceUntil like TestInitialPromptReadiness)
		advanceUntil(ctx, t, mClock, func() bool {
			return len(c.Messages()) >= 2 // greeting + user prompt
		})

		// Step 3: Agent shows response
		agent.setScreen("Response to test prompt")
		advanceFor(ctx, t, mClock, 300*time.Millisecond)

		// Save state - this creates state.json
		err := c.SaveState()
		require.NoError(t, err)

		// Read the saved state.json
		actualData, err := os.ReadFile(stateFile)
		require.NoError(t, err)

		// Read the expected golden file
		expectedData, err := os.ReadFile("testdata/expected_saved_state.json")
		require.NoError(t, err)

		// Parse both JSON files
		var actualState, expectedState st.AgentState
		err = json.Unmarshal(actualData, &actualState)
		require.NoError(t, err)
		err = json.Unmarshal(expectedData, &expectedState)
		require.NoError(t, err)

		// Compare the state files field by field
		assert.Equal(t, expectedState.Version, actualState.Version, "version should match")
		assert.Equal(t, expectedState.InitialPrompt, actualState.InitialPrompt, "initial_prompt should match")
		assert.Equal(t, expectedState.InitialPromptSent, actualState.InitialPromptSent, "initial_prompt_sent should match")
		assert.Equal(t, len(expectedState.Messages), len(actualState.Messages), "message count should match")

		// Compare each message
		for i := range expectedState.Messages {
			if i >= len(actualState.Messages) {
				break
			}
			assert.Equal(t, expectedState.Messages[i].Id, actualState.Messages[i].Id, "message %d: id should match", i)
			assert.Equal(t, expectedState.Messages[i].Message, actualState.Messages[i].Message, "message %d: message should match", i)
			assert.Equal(t, expectedState.Messages[i].Role, actualState.Messages[i].Role, "message %d: role should match", i)
		}
	})

	t.Run("SaveState skips when not configured", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)

		tmpDir := t.TempDir()
		stateFile := tmpDir + "/state.json"

		mClock := quartz.NewMock(t)
		agent := &testAgent{screen: "initial"}
		cfg := st.PTYConversationConfig{
			Clock:                 mClock,
			SnapshotInterval:      100 * time.Millisecond,
			ScreenStabilityLength: 200 * time.Millisecond,
			AgentIO:               agent,
			Logger:                slog.New(slog.NewTextHandler(io.Discard, nil)),
			StatePersistenceConfig: st.StatePersistenceConfig{
				StateFile: stateFile,
				LoadState: false,
				SaveState: false,
			},
		}

		c := st.NewPTY(ctx, cfg, &testEmitter{})
		c.Start(ctx)

		err := c.SaveState()
		require.NoError(t, err)

		// File should not be created
		_, err = os.Stat(stateFile)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("SaveState honors dirty flag", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)

		tmpDir := t.TempDir()
		stateFile := tmpDir + "/state.json"

		mClock := quartz.NewMock(t)
		agent := &testAgent{screen: "initial"}
		cfg := st.PTYConversationConfig{
			Clock:                 mClock,
			SnapshotInterval:      100 * time.Millisecond,
			ScreenStabilityLength: 200 * time.Millisecond,
			AgentIO:               agent,
			Logger:                slog.New(slog.NewTextHandler(io.Discard, nil)),
			StatePersistenceConfig: st.StatePersistenceConfig{
				StateFile: stateFile,
				LoadState: false,
				SaveState: true,
			},
		}

		c := st.NewPTY(ctx, cfg, &testEmitter{})
		c.Start(ctx)

		// Generate conversation and save
		agent.setScreen("hello")
		advanceFor(ctx, t, mClock, 300*time.Millisecond)
		err := c.SaveState()
		require.NoError(t, err)

		// Get file modification time
		info1, err := os.Stat(stateFile)
		require.NoError(t, err)
		modTime1 := info1.ModTime()

		// Save again without changes - file should not be modified
		err = c.SaveState()
		require.NoError(t, err)

		info2, err := os.Stat(stateFile)
		require.NoError(t, err)
		modTime2 := info2.ModTime()

		// File modification time should be the same (dirty flag prevents save)
		assert.Equal(t, modTime1, modTime2)
	})

	t.Run("SaveState creates directory if not exists", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)

		tmpDir := t.TempDir()
		stateFile := tmpDir + "/nested/deep/state.json"

		mClock := quartz.NewMock(t)
		agent := &testAgent{screen: "initial"}
		cfg := st.PTYConversationConfig{
			Clock:                 mClock,
			SnapshotInterval:      100 * time.Millisecond,
			ScreenStabilityLength: 200 * time.Millisecond,
			AgentIO:               agent,
			Logger:                slog.New(slog.NewTextHandler(io.Discard, nil)),
			StatePersistenceConfig: st.StatePersistenceConfig{
				StateFile: stateFile,
				LoadState: false,
				SaveState: true,
			},
		}

		c := st.NewPTY(ctx, cfg, &testEmitter{})
		c.Start(ctx)

		agent.setScreen("hello")
		advanceFor(ctx, t, mClock, 300*time.Millisecond)

		err := c.SaveState()
		require.NoError(t, err)

		// Verify file and directory were created
		_, err = os.Stat(stateFile)
		assert.NoError(t, err)
	})

	t.Run("LoadState restores conversation from file", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)

		tmpDir := t.TempDir()
		stateFile := tmpDir + "/state.json"

		// Create a state file with test data
		testState := st.AgentState{
			Version:       1,
			InitialPrompt: "restored prompt",
			Messages: []st.ConversationMessage{
				{Id: 0, Message: "agent message 1", Role: st.ConversationRoleAgent, Time: time.Now()},
				{Id: 1, Message: "user message 1", Role: st.ConversationRoleUser, Time: time.Now()},
				{Id: 2, Message: "agent message 2", Role: st.ConversationRoleAgent, Time: time.Now()},
			},
		}
		data, err := json.MarshalIndent(testState, "", " ")
		require.NoError(t, err)
		err = os.WriteFile(stateFile, data, 0o644)
		require.NoError(t, err)

		// Create conversation with LoadState enabled
		mClock := quartz.NewMock(t)
		agent := &testAgent{screen: "ready"}
		cfg := st.PTYConversationConfig{
			Clock:                 mClock,
			SnapshotInterval:      100 * time.Millisecond,
			ScreenStabilityLength: 200 * time.Millisecond,
			AgentIO:               agent,
			Logger:                slog.New(slog.NewTextHandler(io.Discard, nil)),
			FormatMessage: func(message string, userInput string) string {
				return message
			},
			ReadyForInitialPrompt: func(message string) bool {
				return message == "ready"
			},
			StatePersistenceConfig: st.StatePersistenceConfig{
				StateFile: stateFile,
				LoadState: true,
				SaveState: false,
			},
		}

		c := st.NewPTY(ctx, cfg, &testEmitter{})
		c.Start(ctx)

		// Advance until agent is ready and state is loaded
		advanceFor(ctx, t, mClock, 300*time.Millisecond)

		// Verify messages were restored
		messages := c.Messages()
		assert.Len(t, messages, 3)
		assert.Equal(t, "agent message 1", messages[0].Message)
		assert.Equal(t, "user message 1", messages[1].Message)
		// The last agent message may have adjustments from adjustScreenAfterStateLoad
		assert.Contains(t, messages[2].Message, "agent message 2")
	})

	t.Run("LoadState handles missing file gracefully", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)

		tmpDir := t.TempDir()
		stateFile := tmpDir + "/nonexistent.json"

		mClock := quartz.NewMock(t)
		agent := &testAgent{screen: "ready"}
		cfg := st.PTYConversationConfig{
			Clock:                 mClock,
			SnapshotInterval:      100 * time.Millisecond,
			ScreenStabilityLength: 200 * time.Millisecond,
			AgentIO:               agent,
			Logger:                slog.New(slog.NewTextHandler(io.Discard, nil)),
			FormatMessage: func(message string, userInput string) string {
				return message
			},
			ReadyForInitialPrompt: func(message string) bool {
				return message == "ready"
			},
			StatePersistenceConfig: st.StatePersistenceConfig{
				StateFile: stateFile,
				LoadState: true,
				SaveState: false,
			},
		}

		// Should not panic or error
		c := st.NewPTY(ctx, cfg, &testEmitter{})
		c.Start(ctx)

		advanceFor(ctx, t, mClock, 300*time.Millisecond)

		// Should have default initial message
		messages := c.Messages()
		assert.Len(t, messages, 1)
	})

	t.Run("LoadState handles empty file gracefully", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)

		tmpDir := t.TempDir()
		stateFile := tmpDir + "/empty.json"

		// Create empty file
		err := os.WriteFile(stateFile, []byte(""), 0o644)
		require.NoError(t, err)

		mClock := quartz.NewMock(t)
		agent := &testAgent{screen: "ready"}
		cfg := st.PTYConversationConfig{
			Clock:                 mClock,
			SnapshotInterval:      100 * time.Millisecond,
			ScreenStabilityLength: 200 * time.Millisecond,
			AgentIO:               agent,
			Logger:                slog.New(slog.NewTextHandler(io.Discard, nil)),
			FormatMessage: func(message string, userInput string) string {
				return message
			},
			ReadyForInitialPrompt: func(message string) bool {
				return message == "ready"
			},
			StatePersistenceConfig: st.StatePersistenceConfig{
				StateFile: stateFile,
				LoadState: true,
				SaveState: false,
			},
		}

		// Should not panic or error
		c := st.NewPTY(ctx, cfg, &testEmitter{})
		c.Start(ctx)

		advanceFor(ctx, t, mClock, 300*time.Millisecond)

		// Should have default initial message
		messages := c.Messages()
		assert.Len(t, messages, 1)
	})

	t.Run("LoadState handles corrupted JSON gracefully", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)

		tmpDir := t.TempDir()
		stateFile := tmpDir + "/corrupted.json"

		// Create corrupted JSON file
		err := os.WriteFile(stateFile, []byte("{invalid json}"), 0o644)
		require.NoError(t, err)

		mClock := quartz.NewMock(t)
		agent := &testAgent{screen: "ready"}
		cfg := st.PTYConversationConfig{
			Clock:                 mClock,
			SnapshotInterval:      100 * time.Millisecond,
			ScreenStabilityLength: 200 * time.Millisecond,
			AgentIO:               agent,
			Logger:                slog.New(slog.NewTextHandler(io.Discard, nil)),
			FormatMessage: func(message string, userInput string) string {
				return message
			},
			ReadyForInitialPrompt: func(message string) bool {
				return message == "ready"
			},
			StatePersistenceConfig: st.StatePersistenceConfig{
				StateFile: stateFile,
				LoadState: true,
				SaveState: false,
			},
		}

		// Should not panic - logs warning and continues
		c := st.NewPTY(ctx, cfg, &testEmitter{})
		c.Start(ctx)

		advanceFor(ctx, t, mClock, 300*time.Millisecond)

		// Should have default initial message
		messages := c.Messages()
		assert.Len(t, messages, 1)
	})

	t.Run("LoadState rejects unsupported version", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)

		tmpDir := t.TempDir()
		stateFile := tmpDir + "/unsupported_version.json"

		// Create state file with unsupported version
		unsupportedState := map[string]interface{}{
			"version":             999, // Unsupported version
			"messages":            []interface{}{},
			"initial_prompt":      "",
			"initial_prompt_sent": false,
		}
		stateBytes, err := json.Marshal(unsupportedState)
		require.NoError(t, err)
		err = os.WriteFile(stateFile, stateBytes, 0o644)
		require.NoError(t, err)

		mClock := quartz.NewMock(t)
		agent := &testAgent{screen: "ready"}
		cfg := st.PTYConversationConfig{
			Clock:                 mClock,
			SnapshotInterval:      100 * time.Millisecond,
			ScreenStabilityLength: 200 * time.Millisecond,
			AgentIO:               agent,
			Logger:                slog.New(slog.NewTextHandler(io.Discard, nil)),
			FormatMessage: func(message string, userInput string) string {
				return message
			},
			ReadyForInitialPrompt: func(message string) bool {
				return message == "ready"
			},
			StatePersistenceConfig: st.StatePersistenceConfig{
				StateFile: stateFile,
				LoadState: true,
				SaveState: false,
			},
		}

		// Should not panic - logs error and continues with empty state
		c := st.NewPTY(ctx, cfg, &testEmitter{})
		c.Start(ctx)

		advanceFor(ctx, t, mClock, 300*time.Millisecond)

		// Should have default initial message (version error causes fallback to empty state)
		messages := c.Messages()
		assert.Len(t, messages, 1)
	})

	t.Run("LoadState_last_message_is_user_role", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)

		tmpDir := t.TempDir()
		stateFile := tmpDir + "/state.json"

		// Create a state file where the last message is a user message.
		// Without the role check in updateLastAgentMessageLocked, the
		// user message content would be used as the new agent message.
		testState := st.AgentState{
			Version:           1,
			InitialPromptSent: true,
			Messages: []st.ConversationMessage{
				{Id: 0, Message: "agent greeting", Role: st.ConversationRoleAgent, Time: time.Now()},
				{Id: 1, Message: "user question", Role: st.ConversationRoleUser, Time: time.Now()},
			},
		}
		data, err := json.MarshalIndent(testState, "", " ")
		require.NoError(t, err)
		err = os.WriteFile(stateFile, data, 0o644)
		require.NoError(t, err)

		mClock := quartz.NewMock(t)
		agent := &testAgent{screen: "ready"}
		cfg := st.PTYConversationConfig{
			Clock:                 mClock,
			SnapshotInterval:      100 * time.Millisecond,
			ScreenStabilityLength: 200 * time.Millisecond,
			AgentIO:               agent,
			Logger:                slog.New(slog.NewTextHandler(io.Discard, nil)),
			FormatMessage: func(message string, userInput string) string {
				return message
			},
			ReadyForInitialPrompt: func(message string) bool {
				return message == "ready"
			},
			StatePersistenceConfig: st.StatePersistenceConfig{
				StateFile: stateFile,
				LoadState: true,
				SaveState: false,
			},
		}

		c := st.NewPTY(ctx, cfg, &testEmitter{})
		c.Start(ctx)

		// Advance past stability so state loads and a new agent message
		// is created from the current screen content.
		advanceFor(ctx, t, mClock, 300*time.Millisecond)

		messages := c.Messages()
		require.True(t, len(messages) >= 3, "expected at least 3 messages, got %d", len(messages))
		// The new agent message should derive from screen content ("ready"),
		// NOT from the last loaded message ("user question").
		lastMsg := messages[len(messages)-1]
		assert.Equal(t, st.ConversationRoleAgent, lastMsg.Role)
		assert.NotEqual(t, "user question", lastMsg.Message,
			"agent message must not contain the user message content")
		assert.Contains(t, lastMsg.Message, "ready")
	})

	t.Run("LoadState_preserves_unsent_initial_prompt_status", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)

		tmpDir := t.TempDir()
		stateFile := tmpDir + "/state.json"

		// Create state where the initial prompt was NOT sent (e.g. previous crash).
		testState := st.AgentState{
			Version:           1,
			InitialPrompt:     "test prompt",
			InitialPromptSent: false,
			Messages: []st.ConversationMessage{
				{Id: 0, Message: "agent greeting", Role: st.ConversationRoleAgent, Time: time.Now()},
			},
		}
		data, err := json.MarshalIndent(testState, "", " ")
		require.NoError(t, err)
		err = os.WriteFile(stateFile, data, 0o644)
		require.NoError(t, err)

		writeCounter := 0
		agent := &testAgent{screen: "ready"}
		agent.onWrite = func(data []byte) {
			writeCounter++
			agent.screen = fmt.Sprintf("__write_%d", writeCounter)
		}

		mClock := quartz.NewMock(t)
		cfg := st.PTYConversationConfig{
			Clock:                 mClock,
			SnapshotInterval:      100 * time.Millisecond,
			ScreenStabilityLength: 200 * time.Millisecond,
			AgentIO:               agent,
			Logger:                slog.New(slog.NewTextHandler(io.Discard, nil)),
			ReadyForInitialPrompt: func(message string) bool {
				return message == "ready"
			},
			StatePersistenceConfig: st.StatePersistenceConfig{
				StateFile: stateFile,
				LoadState: true,
				SaveState: false,
			},
			// Same initial prompt as saved state.
			InitialPrompt: []st.MessagePart{st.MessagePartText{Content: "test prompt"}},
		}

		c := st.NewPTY(ctx, cfg, &testEmitter{})
		c.Start(ctx)

		// Advance until we see a user message with the initial prompt.
		advanceUntil(ctx, t, mClock, func() bool {
			for _, m := range c.Messages() {
				if m.Role == st.ConversationRoleUser && m.Message == "test prompt" {
					return true
				}
			}
			return false
		})

		// Verify the initial prompt was sent as a user message.
		found := false
		for _, m := range c.Messages() {
			if m.Role == st.ConversationRoleUser && m.Message == "test prompt" {
				found = true
				break
			}
		}
		assert.True(t, found, "initial prompt should have been sent since InitialPromptSent was false in saved state")
	})
}

func TestInitialPromptReadiness(t *testing.T) {
	discardLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("agent not ready - status is stable until agent becomes ready", func(t *testing.T) {
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

		c := st.NewPTY(ctx, cfg, &testEmitter{})
		c.Start(ctx)

		// Take a snapshot with "loading...". Threshold is 1 (stability 0 / interval 1s = 0 + 1 = 1).
		advanceFor(ctx, t, mClock, 1*time.Second)

		// Screen is stable and agent is not ready, so initial prompt hasn't been enqueued yet.
		// Status should be stable.
		assert.Equal(t, st.ConversationStatusStable, c.Status())
	})

	t.Run("agent becomes ready - prompt enqueued and status changes to changing", func(t *testing.T) {
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

		c := st.NewPTY(ctx, cfg, &testEmitter{})
		c.Start(ctx)

		// Agent not ready initially, status should be stable
		advanceFor(ctx, t, mClock, 1*time.Second)
		assert.Equal(t, st.ConversationStatusStable, c.Status())

		// Agent becomes ready, prompt gets enqueued, status becomes "changing"
		agent.setScreen("ready")
		advanceFor(ctx, t, mClock, 1*time.Second)
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

		c := st.NewPTY(ctx, cfg, &testEmitter{})
		c.Start(ctx)

		// Status is "stable" while waiting for readiness (prompt not yet enqueued).
		advanceFor(ctx, t, mClock, 1*time.Second)
		assert.Equal(t, st.ConversationStatusStable, c.Status())

		// Agent becomes ready. The snapshot loop detects this, enqueues the prompt,
		// then sees queue + stable + ready and signals the send loop.
		// writeStabilize runs with onWrite changing the screen, so it completes.
		agent.setScreen("ready")
		// Drive clock until the initial prompt is sent (queue drains).
		advanceUntil(ctx, t, mClock, func() bool {
			return len(c.Messages()) >= 2
		})

		// The initial prompt should have been sent. Set a clean screen and
		// advance enough ticks for the snapshot loop to record it as an
		// agent message and fill the stability buffer (threshold=1).
		agent.setScreen("response")
		advanceFor(ctx, t, mClock, 2*time.Second)
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

		c := st.NewPTY(ctx, cfg, &testEmitter{})
		c.Start(ctx)

		advanceFor(ctx, t, mClock, 1*time.Second)

		// Status should be stable because no initial prompt to wait for.
		assert.Equal(t, st.ConversationStatusStable, c.Status())
	})

	t.Run("no initial prompt configured - normal status logic applies", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)
		mClock := quartz.NewMock(t)
		agent := &testAgent{screen: "ready"}
		cfg := st.PTYConversationConfig{
			Clock:                 mClock,
			SnapshotInterval:      1 * time.Second,
			ScreenStabilityLength: 2 * time.Second, // threshold = 3
			AgentIO:               agent,
			Logger:                discardLogger,
		}

		c := st.NewPTY(ctx, cfg, &testEmitter{})
		c.Start(ctx)

		// Fill buffer to reach stability with "ready" screen.
		agent.setScreen("ready")
		advanceFor(ctx, t, mClock, 3*time.Second)
		assert.Equal(t, st.ConversationStatusStable, c.Status())

		// After screen changes, status becomes changing.
		agent.setScreen("processing...")
		advanceFor(ctx, t, mClock, 1*time.Second)
		assert.Equal(t, st.ConversationStatusChanging, c.Status())

		// After screen is stable again (3 identical snapshots), status becomes stable.
		advanceFor(ctx, t, mClock, 1*time.Second)
		advanceFor(ctx, t, mClock, 1*time.Second)
		assert.Equal(t, st.ConversationStatusStable, c.Status())
	})
}

func TestInitialPromptSent(t *testing.T) {
	discardLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("initialPromptSent is set when initial prompt is sent", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)

		tmpDir := t.TempDir()
		stateFile := tmpDir + "/state.json"

		mClock := quartz.NewMock(t)
		agent := &testAgent{screen: "loading..."}
		writeCounter := 0
		agent.onWrite = func(data []byte) {
			writeCounter++
			agent.screen = fmt.Sprintf("__write_%d", writeCounter)
		}

		cfg := st.PTYConversationConfig{
			Clock:                 mClock,
			SnapshotInterval:      1 * time.Second,
			ScreenStabilityLength: 0,
			AgentIO:               agent,
			ReadyForInitialPrompt: func(message string) bool {
				return message == "ready"
			},
			InitialPrompt: []st.MessagePart{st.MessagePartText{Content: "test prompt"}},
			Logger:        discardLogger,
			StatePersistenceConfig: st.StatePersistenceConfig{
				StateFile: stateFile,
				LoadState: false,
				SaveState: true,
			},
		}

		c := st.NewPTY(ctx, cfg, &testEmitter{})
		c.Start(ctx)

		// Agent becomes ready and initial prompt is sent
		agent.setScreen("ready")
		advanceUntil(ctx, t, mClock, func() bool {
			return len(c.Messages()) >= 2
		})

		// Save state and verify initialPromptSent is persisted
		agent.setScreen("response")
		advanceFor(ctx, t, mClock, 2*time.Second)

		err := c.SaveState()
		require.NoError(t, err)

		data, err := os.ReadFile(stateFile)
		require.NoError(t, err)

		var agentState st.AgentState
		err = json.Unmarshal(data, &agentState)
		require.NoError(t, err)

		assert.True(t, agentState.InitialPromptSent, "initialPromptSent should be true after initial prompt is sent")
		assert.Equal(t, "test prompt", agentState.InitialPrompt)
	})

	t.Run("initialPromptSent prevents re-sending prompt after state load", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)

		tmpDir := t.TempDir()
		stateFile := tmpDir + "/state.json"

		// Create a state file with initialPromptSent=true
		testState := st.AgentState{
			Version:           1,
			InitialPrompt:     "test prompt",
			InitialPromptSent: true,
			Messages: []st.ConversationMessage{
				{Id: 0, Message: "agent message", Role: st.ConversationRoleAgent, Time: time.Now()},
				{Id: 1, Message: "test prompt", Role: st.ConversationRoleUser, Time: time.Now()},
			},
		}
		data, err := json.MarshalIndent(testState, "", " ")
		require.NoError(t, err)
		err = os.WriteFile(stateFile, data, 0o644)
		require.NoError(t, err)

		// Create conversation with same initial prompt
		mClock := quartz.NewMock(t)
		agent := &testAgent{screen: "ready"}
		writeCount := 0
		agent.onWrite = func(data []byte) {
			writeCount++
			agent.screen = "after_write"
		}

		cfg := st.PTYConversationConfig{
			Clock:                 mClock,
			SnapshotInterval:      100 * time.Millisecond,
			ScreenStabilityLength: 200 * time.Millisecond,
			AgentIO:               agent,
			ReadyForInitialPrompt: func(message string) bool {
				return message == "ready"
			},
			InitialPrompt: []st.MessagePart{st.MessagePartText{Content: "test prompt"}},
			Logger:        discardLogger,
			StatePersistenceConfig: st.StatePersistenceConfig{
				StateFile: stateFile,
				LoadState: true,
				SaveState: false,
			},
		}

		c := st.NewPTY(ctx, cfg, &testEmitter{})
		c.Start(ctx)

		// Advance until ready and state is loaded
		advanceFor(ctx, t, mClock, 500*time.Millisecond)

		// Verify the prompt was NOT re-sent (no writes occurred)
		assert.Equal(t, 0, writeCount, "initial prompt should not be re-sent when already sent")

		// Messages should be restored from state (at minimum, the original 2)
		messages := c.Messages()
		assert.GreaterOrEqual(t, len(messages), 2, "messages should be restored from state")
		// Verify the first two messages match what we saved
		assert.Equal(t, "agent message", messages[0].Message)
		assert.Equal(t, st.ConversationRoleAgent, messages[0].Role)
		assert.Equal(t, "test prompt", messages[1].Message)
		assert.Equal(t, st.ConversationRoleUser, messages[1].Role)
	})

	t.Run("new initial prompt is sent if different from saved prompt", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)

		tmpDir := t.TempDir()
		stateFile := tmpDir + "/state.json"

		// Create a state file with old prompt
		testState := st.AgentState{
			Version:           1,
			InitialPrompt:     "old prompt",
			InitialPromptSent: true,
			Messages: []st.ConversationMessage{
				{Id: 0, Message: "agent message", Role: st.ConversationRoleAgent, Time: time.Now()},
			},
		}
		data, err := json.MarshalIndent(testState, "", " ")
		require.NoError(t, err)
		err = os.WriteFile(stateFile, data, 0o644)
		require.NoError(t, err)

		// Create conversation with different initial prompt
		mClock := quartz.NewMock(t)
		agent := &testAgent{screen: "loading..."}
		writeCounter := 0
		agent.onWrite = func(data []byte) {
			writeCounter++
			agent.screen = fmt.Sprintf("__write_%d", writeCounter)
		}

		cfg := st.PTYConversationConfig{
			Clock:                 mClock,
			SnapshotInterval:      1 * time.Second,
			ScreenStabilityLength: 0,
			AgentIO:               agent,
			ReadyForInitialPrompt: func(message string) bool {
				return message == "ready"
			},
			InitialPrompt: []st.MessagePart{st.MessagePartText{Content: "new prompt"}},
			Logger:        discardLogger,
			StatePersistenceConfig: st.StatePersistenceConfig{
				StateFile: stateFile,
				LoadState: true,
				SaveState: false,
			},
		}

		c := st.NewPTY(ctx, cfg, &testEmitter{})
		c.Start(ctx)

		// Agent becomes ready
		agent.setScreen("ready")

		// Advance until the new prompt is sent
		advanceUntil(ctx, t, mClock, func() bool {
			msgs := c.Messages()
			// Look for the new prompt in messages
			for _, msg := range msgs {
				if msg.Role == st.ConversationRoleUser && msg.Message == "new prompt" {
					return true
				}
			}
			return false
		})

		// Verify the new prompt was sent
		messages := c.Messages()
		found := false
		for _, msg := range messages {
			if msg.Role == st.ConversationRoleUser && msg.Message == "new prompt" {
				found = true
				break
			}
		}
		assert.True(t, found, "new prompt should be sent when different from saved prompt")
	})

	t.Run("initialPromptSent not set when no initial prompt configured", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)

		tmpDir := t.TempDir()
		stateFile := tmpDir + "/state.json"

		mClock := quartz.NewMock(t)
		agent := &testAgent{screen: "ready"}

		cfg := st.PTYConversationConfig{
			Clock:                 mClock,
			SnapshotInterval:      100 * time.Millisecond,
			ScreenStabilityLength: 200 * time.Millisecond,
			AgentIO:               agent,
			Logger:                discardLogger,
			StatePersistenceConfig: st.StatePersistenceConfig{
				StateFile: stateFile,
				LoadState: false,
				SaveState: true,
			},
		}

		c := st.NewPTY(ctx, cfg, &testEmitter{})
		c.Start(ctx)

		advanceFor(ctx, t, mClock, 300*time.Millisecond)

		err := c.SaveState()
		require.NoError(t, err)

		data, err := os.ReadFile(stateFile)
		require.NoError(t, err)

		var agentState st.AgentState
		err = json.Unmarshal(data, &agentState)
		require.NoError(t, err)

		assert.False(t, agentState.InitialPromptSent, "initialPromptSent should be false when no initial prompt configured")
	})

	t.Run("restored prompt used when no new prompt provided", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)

		tmpDir := t.TempDir()
		stateFile := tmpDir + "/state.json"

		// Create a state file with a prompt
		testState := st.AgentState{
			Version:           1,
			InitialPrompt:     "saved prompt",
			InitialPromptSent: false,
			Messages: []st.ConversationMessage{
				{Id: 0, Message: "agent message", Role: st.ConversationRoleAgent, Time: time.Now()},
			},
		}
		data, err := json.MarshalIndent(testState, "", " ")
		require.NoError(t, err)
		err = os.WriteFile(stateFile, data, 0o644)
		require.NoError(t, err)

		// Create conversation without providing an initial prompt
		mClock := quartz.NewMock(t)
		agent := &testAgent{screen: "loading..."}
		writeCounter := 0
		agent.onWrite = func(data []byte) {
			writeCounter++
			agent.screen = fmt.Sprintf("__write_%d", writeCounter)
		}

		cfg := st.PTYConversationConfig{
			Clock:                 mClock,
			SnapshotInterval:      1 * time.Second,
			ScreenStabilityLength: 0,
			AgentIO:               agent,
			ReadyForInitialPrompt: func(message string) bool {
				return message == "ready"
			},
			Logger: discardLogger,
			StatePersistenceConfig: st.StatePersistenceConfig{
				StateFile: stateFile,
				LoadState: true,
				SaveState: false,
			},
		}

		c := st.NewPTY(ctx, cfg, &testEmitter{})
		c.Start(ctx)

		// Agent becomes ready
		agent.setScreen("ready")

		// Advance until the saved prompt is sent
		advanceUntil(ctx, t, mClock, func() bool {
			msgs := c.Messages()
			for _, msg := range msgs {
				if msg.Role == st.ConversationRoleUser && msg.Message == "saved prompt" {
					return true
				}
			}
			return false
		})

		// Verify the saved prompt was sent
		messages := c.Messages()
		found := false
		for _, msg := range messages {
			if msg.Role == st.ConversationRoleUser && msg.Message == "saved prompt" {
				found = true
				break
			}
		}
		assert.True(t, found, "saved prompt should be sent when no new prompt provided")
	})

	t.Run("empty prompt from state is not restored", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		t.Cleanup(cancel)

		tmpDir := t.TempDir()
		stateFile := tmpDir + "/state.json"

		// Create state file with empty prompt
		emptyPromptState := st.AgentState{
			Version:           1,
			Messages:          []st.ConversationMessage{},
			InitialPrompt:     "", // Empty prompt
			InitialPromptSent: false,
		}
		stateBytes, err := json.Marshal(emptyPromptState)
		require.NoError(t, err)
		err = os.WriteFile(stateFile, stateBytes, 0o644)
		require.NoError(t, err)

		mClock := quartz.NewMock(t)
		agent := &testAgent{screen: "ready"}

		cfg := st.PTYConversationConfig{
			Clock:                 mClock,
			SnapshotInterval:      100 * time.Millisecond,
			ScreenStabilityLength: 200 * time.Millisecond,
			AgentIO:               agent,
			Logger:                discardLogger,
			FormatMessage: func(message string, userInput string) string {
				return message
			},
			ReadyForInitialPrompt: func(message string) bool {
				return message == "ready"
			},
			StatePersistenceConfig: st.StatePersistenceConfig{
				StateFile: stateFile,
				LoadState: true,
				SaveState: false,
			},
		}

		c := st.NewPTY(ctx, cfg, &testEmitter{})
		c.Start(ctx)

		// Agent becomes ready
		agent.setScreen("ready")

		// Advance time to ensure any prompt would be sent
		advanceFor(ctx, t, mClock, 500*time.Millisecond)

		// Verify no prompt was sent (should only have the initial screen message)
		messages := c.Messages()
		for _, msg := range messages {
			if msg.Role == st.ConversationRoleUser {
				t.Errorf("Unexpected user message sent: %q (empty prompt should not be restored)", msg.Message)
			}
		}
	})
}
