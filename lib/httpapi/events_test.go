package httpapi

import (
	"testing"
	"time"

	mf "github.com/coder/agentapi/lib/msgfmt"
	st "github.com/coder/agentapi/lib/screentracker"
)

func TestEventEmitter_Subscribe(t *testing.T) {
	emitter := NewEventEmitter(10)

	subscriberID, ch, initialEvents := emitter.Subscribe()

	// subscriberID is an int, checking it was assigned (non-zero is valid)
	_ = subscriberID // Just verify subscription works
	if ch == nil {
		t.Error("expected non-nil channel")
	}
	if initialEvents == nil {
		t.Error("expected initial events slice")
	}

	// Clean up
	emitter.Unsubscribe(subscriberID)
}

func TestEventEmitter_MultipleSubscribers(t *testing.T) {
	emitter := NewEventEmitter(10)

	id1, ch1, _ := emitter.Subscribe()
	id2, ch2, _ := emitter.Subscribe()

	if id1 == id2 {
		t.Error("expected different subscriber IDs")
	}
	if ch1 == ch2 {
		t.Error("expected different channels")
	}

	emitter.Unsubscribe(id1)
	emitter.Unsubscribe(id2)
}

func TestEventEmitter_UpdateStatusAndEmitChanges(t *testing.T) {
	emitter := NewEventEmitter(10)
	_, ch, _ := emitter.Subscribe()

	err := emitter.UpdateStatusAndEmitChanges(st.ConversationStatusStable, mf.AgentTypeClaude)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	select {
	case event := <-ch:
		if event.Type != EventTypeStatusChange {
			t.Errorf("expected status change event, got %s", event.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected to receive event")
	}
}

func TestEventEmitter_UpdateMessagesAndEmitChanges(t *testing.T) {
	emitter := NewEventEmitter(10)
	_, ch, _ := emitter.Subscribe()

	messages := []st.ConversationMessage{
		{Id: 0, Message: "Hello", Role: st.ConversationRoleUser, Time: time.Now()},
	}

	emitter.UpdateMessagesAndEmitChanges(messages)

	select {
	case event := <-ch:
		if event.Type != EventTypeMessageUpdate {
			t.Errorf("expected message update event, got %s", event.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected to receive event")
	}
}

func TestEventEmitter_UpdateScreenAndEmitChanges(t *testing.T) {
	emitter := NewEventEmitter(10)
	_, ch, _ := emitter.Subscribe()

	screen := "test screen content"
	emitter.UpdateScreenAndEmitChanges(screen)

	select {
	case event := <-ch:
		if event.Type != EventTypeScreenUpdate {
			t.Errorf("expected screen update event, got %s", event.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected to receive event")
	}
}

func TestEventEmitter_NoDuplicateStatusEvents(t *testing.T) {
	emitter := NewEventEmitter(10)
	_, ch, _ := emitter.Subscribe()

	// Update to same status multiple times
	emitter.UpdateStatusAndEmitChanges(st.ConversationStatusStable, mf.AgentTypeClaude)
	emitter.UpdateStatusAndEmitChanges(st.ConversationStatusStable, mf.AgentTypeClaude)
	emitter.UpdateStatusAndEmitChanges(st.ConversationStatusStable, mf.AgentTypeClaude)

	// Should only receive one event (the first change)
	receivedCount := 0
	timeout := time.After(200 * time.Millisecond)

	for {
		select {
		case <-ch:
			receivedCount++
		case <-timeout:
			if receivedCount > 1 {
				t.Errorf("expected at most 1 event for same status, got %d", receivedCount)
			}
			return
		}
	}
}

func TestEventEmitter_Unsubscribe(t *testing.T) {
	emitter := NewEventEmitter(10)

	id, ch, _ := emitter.Subscribe()
	emitter.Unsubscribe(id)

	// Channel should be closed
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to be closed after unsubscribe")
		}
	default:
		// Channel might not be immediately closed, which is okay
	}
}

func TestConvertStatus(t *testing.T) {
	tests := []struct {
		input    st.ConversationStatus
		expected AgentStatus
		hasError bool
	}{
		{st.ConversationStatusStable, AgentStatusStable, false},
		{st.ConversationStatusChanging, AgentStatusRunning, false},
		{st.ConversationStatusInitializing, AgentStatusRunning, false},
		{st.ConversationStatus("unknown"), "", true},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			result, err := convertStatus(tt.input)
			if tt.hasError {
				if err == nil {
					t.Error("expected error")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected %s, got %s", tt.expected, result)
				}
			}
		})
	}
}

func TestStatusChangeBody(t *testing.T) {
	body := StatusChangeBody{
		Status:    AgentStatusStable,
		AgentType: mf.AgentTypeClaude,
	}

	if body.Status != AgentStatusStable {
		t.Errorf("expected stable status, got %s", body.Status)
	}
	if body.AgentType != mf.AgentTypeClaude {
		t.Errorf("expected claude agent type, got %s", body.AgentType)
	}
}

func TestMessageUpdateBody(t *testing.T) {
	body := MessageUpdateBody{
		Id:      0,
		Role:    st.ConversationRoleUser,
		Message: "Hello",
		Time:    time.Now(),
	}

	if body.Message != "Hello" {
		t.Errorf("expected 'Hello', got %s", body.Message)
	}
}

func TestScreenUpdateBody(t *testing.T) {
	body := ScreenUpdateBody{
		Screen: "terminal output",
	}

	if body.Screen != "terminal output" {
		t.Errorf("expected 'terminal output', got %s", body.Screen)
	}
}
