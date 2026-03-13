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

	t.Run("error-cap", func(t *testing.T) {
		emitter := NewEventEmitter(WithSubscriptionBufSize(10))

		for i := range 150 {
			emitter.EmitError(fmt.Sprintf("error %d", i), st.ErrorLevelError)
		}

		_, _, stateEvents := emitter.Subscribe()

		var errorEvents []Event
		for _, ev := range stateEvents {
			if ev.Type == EventTypeError {
				errorEvents = append(errorEvents, ev)
			}
		}

		assert.Len(t, errorEvents, maxStoredErrors)

		// Errors should be the last 100: "error 50" through "error 149".
		for i, ev := range errorEvents {
			body, ok := ev.Payload.(ErrorBody)
			assert.True(t, ok)
			assert.Equal(t, fmt.Sprintf("error %d", i+50), body.Message)
		}
	})

	t.Run("error-events-in-initial-state", func(t *testing.T) {
		mockClock := quartz.NewMock(t)
		fixedTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		mockClock.Set(fixedTime)

		emitter := NewEventEmitter(WithClock(mockClock), WithSubscriptionBufSize(10))

		emitter.EmitError("err1", st.ErrorLevelError)
		mockClock.Set(fixedTime.Add(1 * time.Second))
		emitter.EmitError("err2", st.ErrorLevelWarning)
		mockClock.Set(fixedTime.Add(2 * time.Second))
		emitter.EmitError("err3", st.ErrorLevelError)

		_, _, stateEvents := emitter.Subscribe()

		var errorEvents []Event
		for _, ev := range stateEvents {
			if ev.Type == EventTypeError {
				errorEvents = append(errorEvents, ev)
			}
		}

		assert.Len(t, errorEvents, 3)

		expected := []ErrorBody{
			{Message: "err1", Level: st.ErrorLevelError, Time: fixedTime},
			{Message: "err2", Level: st.ErrorLevelWarning, Time: fixedTime.Add(1 * time.Second)},
			{Message: "err3", Level: st.ErrorLevelError, Time: fixedTime.Add(2 * time.Second)},
		}
		for i, ev := range errorEvents {
			body, ok := ev.Payload.(ErrorBody)
			assert.True(t, ok)
			assert.Equal(t, expected[i].Message, body.Message)
			assert.Equal(t, expected[i].Level, body.Level)
			assert.Equal(t, expected[i].Time, body.Time)
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
