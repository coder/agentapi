package screentracker_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	st "github.com/coder/agentapi/lib/screentracker"
)

type statusTestStep struct {
	snapshot string
	status   st.ConversationStatus
}
type statusTestParams struct {
	cfg   st.PTYConversationConfig
	steps []statusTestStep
}

type testAgent struct {
	st.AgentIO
	screen string
}

func (a *testAgent) ReadScreen() string {
	return a.screen
}

func (a *testAgent) Write(data []byte) (int, error) {
	return 0, nil
}

func statusTest(t *testing.T, params statusTestParams) {
	ctx := context.Background()
	t.Run(fmt.Sprintf("interval-%s,stability_length-%s", params.cfg.SnapshotInterval, params.cfg.ScreenStabilityLength), func(t *testing.T) {
		if params.cfg.GetTime == nil {
			params.cfg.GetTime = func() time.Time { return time.Now() }
		}
		c := st.NewPTY(ctx, params.cfg, "")
		assert.Equal(t, st.ConversationStatusInitializing, c.Status())

		for i, step := range params.steps {
			c.Snapshot(step.snapshot)
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
	now := time.Now()
	agentMsg := func(id int, msg string) st.ConversationMessage {
		return st.ConversationMessage{
			Id:      id,
			Message: msg,
			Role:    st.ConversationRoleAgent,
			Time:    now,
		}
	}
	userMsg := func(id int, msg string) st.ConversationMessage {
		return st.ConversationMessage{
			Id:      id,
			Message: msg,
			Role:    st.ConversationRoleUser,
			Time:    now,
		}
	}
	sendMsg := func(c *st.PTYConversation, msg string) error {
		return c.Send(st.MessagePartText{Content: msg})
	}
	newConversation := func(opts ...func(*st.PTYConversationConfig)) *st.PTYConversation {
		cfg := st.PTYConversationConfig{
			GetTime:                    func() time.Time { return now },
			SnapshotInterval:           1 * time.Second,
			ScreenStabilityLength:      2 * time.Second,
			SkipWritingMessage:         true,
			SkipSendMessageStatusCheck: true,
		}
		for _, opt := range opts {
			opt(&cfg)
		}
		return st.NewPTY(context.Background(), cfg, "")
	}

	t.Run("messages are copied", func(t *testing.T) {
		c := newConversation()
		messages := c.Messages()
		assert.Equal(t, []st.ConversationMessage{
			agentMsg(0, ""),
		}, messages)

		messages[0].Message = "modification"

		assert.Equal(t, []st.ConversationMessage{
			agentMsg(0, ""),
		}, c.Messages())
	})

	t.Run("whitespace-padding", func(t *testing.T) {
		c := newConversation()
		for _, msg := range []string{"123 ", " 123", "123\t\t", "\n123", "123\n\t", " \t123\n\t"} {
			err := c.Send(st.MessagePartText{Content: msg})
			assert.ErrorIs(t, err, st.ErrMessageValidationWhitespace)
		}
	})

	t.Run("no-change-no-message-update", func(t *testing.T) {
		nowWrapper := struct {
			time.Time
		}{
			Time: now,
		}
		c := newConversation(func(cfg *st.PTYConversationConfig) {
			cfg.GetTime = func() time.Time { return nowWrapper.Time }
		})

		c.Snapshot("1")
		msgs := c.Messages()
		assert.Equal(t, []st.ConversationMessage{
			agentMsg(0, "1"),
		}, msgs)
		nowWrapper.Time = nowWrapper.Add(1 * time.Second)
		c.Snapshot("1")
		assert.Equal(t, msgs, c.Messages())
	})

	t.Run("tracking messages", func(t *testing.T) {
		agent := &testAgent{}
		c := newConversation(func(cfg *st.PTYConversationConfig) {
			cfg.AgentIO = agent
		})
		// agent message is recorded when the first snapshot is added
		c.Snapshot("1")
		assert.Equal(t, []st.ConversationMessage{
			agentMsg(0, "1"),
		}, c.Messages())

		// agent message is updated when the screen changes
		c.Snapshot("2")
		assert.Equal(t, []st.ConversationMessage{
			agentMsg(0, "2"),
		}, c.Messages())

		// user message is recorded
		agent.screen = "2"
		assert.NoError(t, sendMsg(c, "3"))
		assert.Equal(t, []st.ConversationMessage{
			agentMsg(0, "2"),
			userMsg(1, "3"),
		}, c.Messages())

		// agent message is added after a user message
		c.Snapshot("4")
		assert.Equal(t, []st.ConversationMessage{
			agentMsg(0, "2"),
			userMsg(1, "3"),
			agentMsg(2, "4"),
		}, c.Messages())

		// agent message is updated when the screen changes before a user message
		agent.screen = "5"
		assert.NoError(t, sendMsg(c, "6"))
		assert.Equal(t, []st.ConversationMessage{
			agentMsg(0, "2"),
			userMsg(1, "3"),
			agentMsg(2, "5"),
			userMsg(3, "6"),
		}, c.Messages())

		// conversation status is changing right after a user message
		c.Snapshot("7")
		c.Snapshot("7")
		c.Snapshot("7")
		assert.Equal(t, st.ConversationStatusStable, c.Status())
		agent.screen = "7"
		assert.NoError(t, sendMsg(c, "8"))
		assert.Equal(t, []st.ConversationMessage{
			agentMsg(0, "2"),
			userMsg(1, "3"),
			agentMsg(2, "5"),
			userMsg(3, "6"),
			agentMsg(4, "7"),
			userMsg(5, "8"),
		}, c.Messages())
		assert.Equal(t, st.ConversationStatusChanging, c.Status())

		// conversation status is back to stable after a snapshot that
		// doesn't change the screen
		c.Snapshot("7")
		assert.Equal(t, st.ConversationStatusStable, c.Status())
	})

	t.Run("tracking messages overlap", func(t *testing.T) {
		agent := &testAgent{}
		c := newConversation(func(cfg *st.PTYConversationConfig) {
			cfg.AgentIO = agent
		})

		// common overlap between screens is removed after a user message
		c.Snapshot("1")
		agent.screen = "1"
		assert.NoError(t, sendMsg(c, "2"))
		c.Snapshot("1\n3")
		assert.Equal(t, []st.ConversationMessage{
			agentMsg(0, "1"),
			userMsg(1, "2"),
			agentMsg(2, "3"),
		}, c.Messages())

		agent.screen = "1\n3x"
		assert.NoError(t, sendMsg(c, "4"))
		c.Snapshot("1\n3x\n5")
		assert.Equal(t, []st.ConversationMessage{
			agentMsg(0, "1"),
			userMsg(1, "2"),
			agentMsg(2, "3x"),
			userMsg(3, "4"),
			agentMsg(4, "5"),
		}, c.Messages())
	})

	t.Run("format-message", func(t *testing.T) {
		agent := &testAgent{}
		c := newConversation(func(cfg *st.PTYConversationConfig) {
			cfg.AgentIO = agent
			cfg.FormatMessage = func(message string, userInput string) string {
				return message + " " + userInput
			}
		})
		agent.screen = "1"
		assert.NoError(t, sendMsg(c, "2"))
		assert.Equal(t, []st.ConversationMessage{
			agentMsg(0, "1 "),
			userMsg(1, "2"),
		}, c.Messages())
		agent.screen = "x"
		c.Snapshot("x")
		assert.Equal(t, []st.ConversationMessage{
			agentMsg(0, "1 "),
			userMsg(1, "2"),
			agentMsg(2, "x 2"),
		}, c.Messages())
	})

	t.Run("format-message", func(t *testing.T) {
		agent := &testAgent{}
		c := newConversation(func(cfg *st.PTYConversationConfig) {
			cfg.AgentIO = agent
			cfg.FormatMessage = func(message string, userInput string) string {
				return "formatted"
			}
		})
		assert.Equal(t, []st.ConversationMessage{
			{
				Id:      0,
				Message: "",
				Role:    st.ConversationRoleAgent,
				Time:    now,
			},
		}, c.Messages())
	})

	t.Run("send-message-status-check", func(t *testing.T) {
		c := newConversation(func(cfg *st.PTYConversationConfig) {
			cfg.SkipSendMessageStatusCheck = false
			cfg.SnapshotInterval = 1 * time.Second
			cfg.ScreenStabilityLength = 2 * time.Second
			cfg.AgentIO = &testAgent{}
		})
		assert.ErrorIs(t, sendMsg(c, "1"), st.ErrMessageValidationChanging)
		for range 3 {
			c.Snapshot("1")
		}
		assert.NoError(t, sendMsg(c, "4"))
		c.Snapshot("2")
		assert.ErrorIs(t, sendMsg(c, "5"), st.ErrMessageValidationChanging)
	})

	t.Run("send-message-empty-message", func(t *testing.T) {
		c := newConversation()
		assert.ErrorIs(t, sendMsg(c, ""), st.ErrMessageValidationEmpty)
	})
}

func TestInitialPromptReadiness(t *testing.T) {
	now := time.Now()

	t.Run("agent not ready - status remains changing", func(t *testing.T) {
		cfg := st.PTYConversationConfig{
			GetTime:               func() time.Time { return now },
			SnapshotInterval:      1 * time.Second,
			ScreenStabilityLength: 0,
			AgentIO:               &testAgent{screen: "loading..."},
			ReadyForInitialPrompt: func(message string) bool {
				return message == "ready"
			},
		}
		c := st.NewPTY(context.Background(), cfg, "initial prompt here")

		// Fill buffer with stable snapshots, but agent is not ready
		c.Snapshot("loading...")

		// Even though screen is stable, status should be changing because agent is not ready
		assert.Equal(t, st.ConversationStatusChanging, c.Status())
		assert.False(t, c.ReadyForInitialPrompt)
		assert.False(t, c.InitialPromptSent)
	})

	t.Run("agent becomes ready - status changes to stable", func(t *testing.T) {
		cfg := st.PTYConversationConfig{
			GetTime:               func() time.Time { return now },
			SnapshotInterval:      1 * time.Second,
			ScreenStabilityLength: 0,
			AgentIO:               &testAgent{screen: "loading..."},
			ReadyForInitialPrompt: func(message string) bool {
				return message == "ready"
			},
		}
		c := st.NewPTY(context.Background(), cfg, "initial prompt here")

		// Agent not ready initially
		c.Snapshot("loading...")
		assert.Equal(t, st.ConversationStatusChanging, c.Status())

		// Agent becomes ready
		c.Snapshot("ready")
		assert.Equal(t, st.ConversationStatusStable, c.Status())
		assert.True(t, c.ReadyForInitialPrompt)
		assert.False(t, c.InitialPromptSent)
	})

	t.Run("ready for initial prompt lifecycle: false -> true -> false", func(t *testing.T) {
		agent := &testAgent{screen: "loading..."}
		cfg := st.PTYConversationConfig{
			GetTime:               func() time.Time { return now },
			SnapshotInterval:      1 * time.Second,
			ScreenStabilityLength: 0,
			AgentIO:               agent,
			ReadyForInitialPrompt: func(message string) bool {
				return message == "ready"
			},
			SkipWritingMessage:         true,
			SkipSendMessageStatusCheck: true,
		}
		c := st.NewPTY(context.Background(), cfg, "initial prompt here")

		// Initial state: ReadyForInitialPrompt should be false
		c.Snapshot("loading...")
		assert.False(t, c.ReadyForInitialPrompt, "should start as false")
		assert.False(t, c.InitialPromptSent)
		assert.Equal(t, st.ConversationStatusChanging, c.Status())

		// Agent becomes ready: ReadyForInitialPrompt should become true
		agent.screen = "ready"
		c.Snapshot("ready")
		assert.Equal(t, st.ConversationStatusStable, c.Status())
		assert.True(t, c.ReadyForInitialPrompt, "should become true when ready")
		assert.False(t, c.InitialPromptSent)

		// Send the initial prompt
		assert.NoError(t, c.Send(st.MessagePartText{Content: "initial prompt here"}))

		// After sending initial prompt: ReadyForInitialPrompt should be set back to false
		// (simulating what happens in the actual server code)
		c.InitialPromptSent = true
		c.ReadyForInitialPrompt = false

		// Verify final state
		assert.False(t, c.ReadyForInitialPrompt, "should be false after initial prompt sent")
		assert.True(t, c.InitialPromptSent)
	})

	t.Run("no initial prompt - normal status logic applies", func(t *testing.T) {
		cfg := st.PTYConversationConfig{
			GetTime:               func() time.Time { return now },
			SnapshotInterval:      1 * time.Second,
			ScreenStabilityLength: 0,
			AgentIO:               &testAgent{screen: "loading..."},
			ReadyForInitialPrompt: func(message string) bool {
				return false // Agent never ready
			},
		}
		// Empty initial prompt means no need to wait for readiness
		c := st.NewPTY(context.Background(), cfg, "")

		c.Snapshot("loading...")

		// Status should be stable because no initial prompt to wait for
		assert.Equal(t, st.ConversationStatusStable, c.Status())
		assert.False(t, c.ReadyForInitialPrompt)
		assert.True(t, c.InitialPromptSent) // Set to true when initial prompt is empty
	})

	t.Run("initial prompt sent - normal status logic applies", func(t *testing.T) {
		agent := &testAgent{screen: "ready"}
		cfg := st.PTYConversationConfig{
			GetTime:               func() time.Time { return now },
			SnapshotInterval:      1 * time.Second,
			ScreenStabilityLength: 0,
			AgentIO:               agent,
			ReadyForInitialPrompt: func(message string) bool {
				return message == "ready"
			},
			SkipWritingMessage:         true,
			SkipSendMessageStatusCheck: true,
		}
		c := st.NewPTY(context.Background(), cfg, "initial prompt here")

		// First, agent becomes ready
		c.Snapshot("ready")
		assert.Equal(t, st.ConversationStatusStable, c.Status())
		assert.True(t, c.ReadyForInitialPrompt)
		assert.False(t, c.InitialPromptSent)

		// Send the initial prompt
		agent.screen = "processing..."
		assert.NoError(t, c.Send(st.MessagePartText{Content: "initial prompt here"}))

		// Mark initial prompt as sent (simulating what the server does)
		c.InitialPromptSent = true
		c.ReadyForInitialPrompt = false

		// Now test that status logic works normally after initial prompt is sent
		c.Snapshot("processing...")

		// Status should be stable because initial prompt was already sent
		// and the readiness check is bypassed
		assert.Equal(t, st.ConversationStatusStable, c.Status())
		assert.False(t, c.ReadyForInitialPrompt)
		assert.True(t, c.InitialPromptSent)
	})
}
