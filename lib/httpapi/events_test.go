package httpapi

import (
	"fmt"
	"testing"
	"time"

	mf "github.com/coder/agentapi/lib/msgfmt"
	st "github.com/coder/agentapi/lib/screentracker"
	"github.com/stretchr/testify/assert"
)

func TestEventEmitter(t *testing.T) {
	t.Run("single-subscription", func(t *testing.T) {
		emitter := NewEventEmitter(10)
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

		emitter.UpdateStatusAndEmitChanges(st.ConversationStatusStable, mf.AgentTypeAider)
		newEvent = <-ch
		assert.Equal(t, Event{
			Type:    EventTypeStatusChange,
			Payload: StatusChangeBody{Status: AgentStatusStable, AgentType: mf.AgentTypeAider},
		}, newEvent)
	})

	t.Run("multiple-subscriptions", func(t *testing.T) {
		emitter := NewEventEmitter(10)
		channels := make([]<-chan Event, 0, 10)
		for i := 0; i < 10; i++ {
			_, ch, _ := emitter.Subscribe()
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
		_, ch, _ := emitter.Subscribe()
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
}
