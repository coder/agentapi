package httpapi

import (
	"fmt"
	"testing"
	"time"

	st "github.com/coder/agentapi/lib/screentracker"
	"github.com/coder/quartz"
	"github.com/stretchr/testify/assert"
)

func TestEventEmitter(t *testing.T) {
	t.Run("single-subscription", func(t *testing.T) {
		emitter := NewEventEmitter(WithSubscriptionBufSize(10))
		_, ch, stateEvents := emitter.Subscribe()
		assert.Empty(t, ch)
		assert.Equal(t, []Event{
			{
				Type:    EventTypeStatusChange,
				Payload: StatusChangeBody{Status: AgentStatusRunning},
			},
			{
				Type:    EventTypeScreenUpdate,
				Payload: ScreenUpdateBody{Screen: ""},
			},
		}, stateEvents)

		now := time.Now()
		emitter.EmitMessages([]st.ConversationMessage{
			{Id: 1, Message: "Hello, world!", Role: st.ConversationRoleUser, Time: now},
		})
		newEvent := <-ch
		assert.Equal(t, Event{
			Type:    EventTypeMessageUpdate,
			Payload: MessageUpdateBody{Id: 1, Message: "Hello, world!", Role: st.ConversationRoleUser, Time: now},
		}, newEvent)

		emitter.EmitMessages([]st.ConversationMessage{
			{Id: 1, Message: "Hello, world! (updated)", Role: st.ConversationRoleUser, Time: now},
			{Id: 2, Message: "What's up?", Role: st.ConversationRoleAgent, Time: now},
		})
		newEvent = <-ch
		assert.Equal(t, Event{
			Type:    EventTypeMessageUpdate,
			Payload: MessageUpdateBody{Id: 1, Message: "Hello, world! (updated)", Role: st.ConversationRoleUser, Time: now},
		}, newEvent)

		newEvent = <-ch
		assert.Equal(t, Event{
			Type:    EventTypeMessageUpdate,
			Payload: MessageUpdateBody{Id: 2, Message: "What's up?", Role: st.ConversationRoleAgent, Time: now},
		}, newEvent)

		emitter.EmitStatus(st.ConversationStatusStable)
		newEvent = <-ch
		assert.Equal(t, Event{
			Type:    EventTypeStatusChange,
			Payload: StatusChangeBody{Status: AgentStatusStable, AgentType: ""},
		}, newEvent)
	})

	t.Run("multiple-subscriptions", func(t *testing.T) {
		emitter := NewEventEmitter(WithSubscriptionBufSize(10))
		channels := make([]<-chan Event, 0, 10)
		for i := 0; i < 10; i++ {
			_, ch, _ := emitter.Subscribe()
			channels = append(channels, ch)
		}
		now := time.Now()

		emitter.EmitMessages([]st.ConversationMessage{
			{Id: 1, Message: "Hello, world!", Role: st.ConversationRoleUser, Time: now},
		})
		for _, ch := range channels {
			newEvent := <-ch
			assert.Equal(t, Event{
				Type:    EventTypeMessageUpdate,
				Payload: MessageUpdateBody{Id: 1, Message: "Hello, world!", Role: st.ConversationRoleUser, Time: now},
			}, newEvent)
		}
	})

	t.Run("close-channel", func(t *testing.T) {
		emitter := NewEventEmitter(WithSubscriptionBufSize(1))
		_, ch, _ := emitter.Subscribe()
		for i := range 5 {
			emitter.EmitMessages([]st.ConversationMessage{
				{Id: i, Message: fmt.Sprintf("Hello, world! %d", i), Role: st.ConversationRoleUser, Time: time.Now()},
			})
		}
		_, ok := <-ch
		assert.True(t, ok)
		select {
		case _, ok := <-ch:
			assert.False(t, ok)
		default:
			t.Fatalf("read should not block")
		}
	})

	t.Run("clock-injection", func(t *testing.T) {
		mockClock := quartz.NewMock(t)
		fixedTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		mockClock.Set(fixedTime)

		emitter := NewEventEmitter(WithClock(mockClock), WithSubscriptionBufSize(10))
		_, ch, stateEvents := emitter.Subscribe()

		// Verify initial state events
		assert.Len(t, stateEvents, 2)

		// Emit an error and verify it uses the mock clock time
		emitter.EmitError("test error", st.ErrorLevelError)

		event := <-ch
		assert.Equal(t, EventTypeError, event.Type)
		errorBody, ok := event.Payload.(ErrorBody)
		assert.True(t, ok)
		assert.Equal(t, "test error", errorBody.Message)
		assert.Equal(t, st.ErrorLevelError, errorBody.Level)
		assert.Equal(t, fixedTime, errorBody.Time)

		// Advance the clock and emit another error
		newTime := fixedTime.Add(1 * time.Hour)
		mockClock.Set(newTime)
		emitter.EmitError("another error", st.ErrorLevelWarning)

		event = <-ch
		assert.Equal(t, EventTypeError, event.Type)
		errorBody, ok = event.Payload.(ErrorBody)
		assert.True(t, ok)
		assert.Equal(t, "another error", errorBody.Message)
		assert.Equal(t, st.ErrorLevelWarning, errorBody.Level)
		assert.Equal(t, newTime, errorBody.Time)
	})
}
