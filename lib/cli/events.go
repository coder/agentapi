package cli

import (
	"strings"
	"sync"

	mf "github.com/coder/agentapi/lib/cli/msgfmt"
	st "github.com/coder/agentapi/lib/cli/screentracker"
	"github.com/coder/agentapi/lib/types"
)

type EventType string

const (
	EventTypeMessageUpdate EventType = "message_update"
	EventTypeStatusChange  EventType = "status_change"
	EventTypeScreenUpdate  EventType = "screen_update"
)

type Event struct {
	Type    EventType
	Payload any
}

type EventEmitter struct {
	mu                  sync.Mutex
	messages            []types.ConversationMessage
	status              types.AgentStatus
	chans               map[int]chan Event
	chanIdx             int
	subscriptionBufSize int
	screen              string
}

// subscriptionBufSize is the size of the buffer for each subscription.
// Once the buffer is full, the channel will be closed.
// Listeners must actively drain the channel, so it's important to
// set this to a value that is large enough to handle the expected
// number of events.
func NewEventEmitter(subscriptionBufSize int) *EventEmitter {
	return &EventEmitter{
		mu:                  sync.Mutex{},
		messages:            make([]types.ConversationMessage, 0),
		status:              types.AgentStatusRunning,
		chans:               make(map[int]chan Event),
		chanIdx:             0,
		subscriptionBufSize: subscriptionBufSize,
	}
}

// Assumes the caller holds the lock.
func (e *EventEmitter) notifyChannels(eventType EventType, payload any) {
	chanIds := make([]int, 0, len(e.chans))
	for chanId := range e.chans {
		chanIds = append(chanIds, chanId)
	}
	for _, chanId := range chanIds {
		ch := e.chans[chanId]
		event := Event{
			Type:    eventType,
			Payload: payload,
		}

		select {
		case ch <- event:
		default:
			// If the channel is full, close it.
			// Listeners must actively drain the channel.
			e.unsubscribeInner(chanId)
		}
	}
}

// Assumes that only the last message can change or new messages can be added.
// If a new message is injected between existing messages (identified by Id), the behavior is undefined.
func (e *EventEmitter) UpdateMessagesAndEmitChanges(newMessages []types.ConversationMessage) {
	e.mu.Lock()
	defer e.mu.Unlock()

	maxLength := max(len(e.messages), len(newMessages))
	for i := range maxLength {
		var oldMsg types.ConversationMessage
		var newMsg types.ConversationMessage
		if i < len(e.messages) {
			oldMsg = e.messages[i]
		}
		if i < len(newMessages) {
			newMsg = newMessages[i]
		}
		if oldMsg != newMsg {
			e.notifyChannels(EventTypeMessageUpdate, types.MessageUpdateBody{
				Id:      newMessages[i].Id,
				Role:    newMessages[i].Role,
				Message: newMessages[i].Message,
				Time:    newMessages[i].Time,
			})
		}
	}

	e.messages = newMessages
}

func (e *EventEmitter) UpdateStatusAndEmitChanges(newStatus st.ConversationStatus) {
	e.mu.Lock()
	defer e.mu.Unlock()

	newAgentStatus := convertStatus(newStatus)
	if e.status == newAgentStatus {
		return
	}

	e.notifyChannels(EventTypeStatusChange, types.StatusChangeBody{Status: newAgentStatus})
	e.status = newAgentStatus
}

func (e *EventEmitter) UpdateScreenAndEmitChanges(newScreen string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.screen == newScreen {
		return
	}

	e.notifyChannels(EventTypeScreenUpdate, types.ScreenUpdateBody{Screen: strings.TrimRight(newScreen, mf.WhiteSpaceChars)})
	e.screen = newScreen
}

// Assumes the caller holds the lock.
func (e *EventEmitter) currentStateAsEvents() []Event {
	events := make([]Event, 0, len(e.messages)+2)
	for _, msg := range e.messages {
		events = append(events, Event{
			Type:    EventTypeMessageUpdate,
			Payload: types.MessageUpdateBody{Id: msg.Id, Role: msg.Role, Message: msg.Message, Time: msg.Time},
		})
	}
	events = append(events, Event{
		Type:    EventTypeStatusChange,
		Payload: types.StatusChangeBody{Status: e.status},
	})
	events = append(events, Event{
		Type:    EventTypeScreenUpdate,
		Payload: types.ScreenUpdateBody{Screen: strings.TrimRight(e.screen, mf.WhiteSpaceChars)},
	})
	return events
}

// Subscribe returns:
// - a subscription ID that can be used to unsubscribe.
// - a channel for receiving events.
// - a list of events that allow to recreate the state of the conversation right before the subscription was created.
func (e *EventEmitter) Subscribe() (int, <-chan Event, []Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	stateEvents := e.currentStateAsEvents()

	// Once a channel becomes full, it will be closed.
	ch := make(chan Event, e.subscriptionBufSize)
	e.chans[e.chanIdx] = ch
	e.chanIdx++
	return e.chanIdx - 1, ch, stateEvents
}

// Assumes the caller holds the lock.
func (e *EventEmitter) unsubscribeInner(chanId int) {
	close(e.chans[chanId])
	delete(e.chans, chanId)
}

func (e *EventEmitter) Unsubscribe(chanId int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.unsubscribeInner(chanId)
}
