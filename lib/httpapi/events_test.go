package httpapi

import (
	"fmt"
	"testing"
	"time"

	st "github.com/coder/agentapi/lib/screentracker"
	"github.com/stretchr/testify/assert"
)

func TestEventEmitter(t *testing.T) {
	t.Run("single-subscription", func(t *testing.T) {
		emitter := NewEventEmitter(10)
		_, ch, stateEvents, err := emitter.Subscribe()
		assert.NoError(t, err)
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
		emitter.UpdateMessagesAndEmitChanges([]st.ConversationMessage{
			{Id: 1, Message: "Hello, world!", Role: st.ConversationRoleUser, Time: now},
		})
		newEvent := <-ch
		assert.Equal(t, Event{
			Type:    EventTypeMessageUpdate,
			Payload: MessageUpdateBody{Id: 1, Message: "Hello, world!", Role: st.ConversationRoleUser, Time: now},
		}, newEvent)

		emitter.UpdateMessagesAndEmitChanges([]st.ConversationMessage{
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

		emitter.UpdateStatusAndEmitChanges(st.ConversationStatusStable)
		newEvent = <-ch
		assert.Equal(t, Event{
			Type:    EventTypeStatusChange,
			Payload: StatusChangeBody{Status: AgentStatusStable},
		}, newEvent)
	})

	t.Run("multiple-subscriptions", func(t *testing.T) {
		emitter := NewEventEmitter(10)
		channels := make([]<-chan Event, 0, 10)
		for i := 0; i < 10; i++ {
			_, ch, _, err := emitter.Subscribe()
			assert.NoError(t, err)
			channels = append(channels, ch)
		}
		now := time.Now()

		emitter.UpdateMessagesAndEmitChanges([]st.ConversationMessage{
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
		emitter := NewEventEmitter(1)
		_, ch, _, err := emitter.Subscribe()
		assert.NoError(t, err)
		for i := range 5 {
			emitter.UpdateMessagesAndEmitChanges([]st.ConversationMessage{
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

	t.Run("subscriber-limit", func(t *testing.T) {
		emitter := NewEventEmitter(10)
		// Subscribe up to the limit
		for i := 0; i < MaxSSESubscribers; i++ {
			_, _, _, err := emitter.Subscribe()
			assert.NoError(t, err, "subscription %d should succeed", i)
		}
		// Next subscription should fail
		_, _, _, err := emitter.Subscribe()
		assert.Error(t, err)
		assert.IsType(t, &SubscriberLimitError{}, err)
	})
}
