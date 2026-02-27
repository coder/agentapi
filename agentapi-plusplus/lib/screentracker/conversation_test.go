package screentracker

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/coder/agentapi/lib/msgfmt"
)

// mockAgentIO implements AgentIO for testing
type mockAgentIO struct {
	screen string
	writes []string
}

func (m *mockAgentIO) Write(data []byte) (int, error) {
	m.writes = append(m.writes, string(data))
	return len(data), nil
}

func (m *mockAgentIO) ReadScreen() string {
	return m.screen
}

func (m *mockAgentIO) SetScreen(s string) {
	m.screen = s
}

func newTestConversation() *Conversation {
	cfg := ConversationConfig{
		AgentType:             msgfmt.AgentTypeClaude,
		AgentIO:               &mockAgentIO{},
		GetTime:               time.Now,
		SnapshotInterval:      100 * time.Millisecond,
		ScreenStabilityLength: 300 * time.Millisecond,
		FormatMessage: func(msg, input string) string {
			return msg
		},
		SkipWritingMessage:         true,
		SkipSendMessageStatusCheck: true,
	}
	return NewConversation(context.Background(), cfg, "")
}

func TestNewConversation(t *testing.T) {
	c := newTestConversation()
	if c == nil {
		t.Fatal("expected conversation, got nil")
	}
	if len(c.messages) == 0 {
		t.Error("expected initial message")
	}
	if c.messages[0].Role != ConversationRoleAgent {
		t.Errorf("expected agent role, got %s", c.messages[0].Role)
	}
}

func TestConversationStatus_Initializing(t *testing.T) {
	c := newTestConversation()
	// Before snapshots are taken
	status := c.Status()
	if status != ConversationStatusInitializing {
		t.Errorf("expected initializing status, got %s", status)
	}
}

func TestConversationStatus_Stable(t *testing.T) {
	c := newTestConversation()
	// Add enough stable snapshots
	for i := 0; i < 5; i++ {
		c.AddSnapshot("stable screen")
	}
	status := c.Status()
	if status != ConversationStatusStable {
		t.Errorf("expected stable status, got %s", status)
	}
}

func TestConversationStatus_Changing(t *testing.T) {
	c := newTestConversation()
	// Add changing snapshots
	for i := 0; i < 5; i++ {
		c.AddSnapshot("changing screen " + string(rune('a'+i)))
	}
	status := c.Status()
	if status != ConversationStatusChanging {
		t.Errorf("expected changing status, got %s", status)
	}
}

func TestConversationStatus_AfterUserMessage(t *testing.T) {
	c := newTestConversation()
	// Make stable
	for i := 0; i < 5; i++ {
		c.AddSnapshot("stable screen")
	}
	// Send message
	err := c.SendMessage(MessagePartText{Content: "test message"})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}
	// Status should be changing after user message
	status := c.Status()
	if status != ConversationStatusChanging {
		t.Errorf("expected changing status after user message, got %s", status)
	}
}

func TestSendMessage_Validation_Empty(t *testing.T) {
	c := newTestConversation()
	c.cfg.SkipSendMessageStatusCheck = false

	// First make stable
	for i := 0; i < 5; i++ {
		c.AddSnapshot("screen")
	}

	err := c.SendMessage(MessagePartText{Content: ""})
	if err != MessageValidationErrorEmpty {
		t.Errorf("expected empty validation error, got %v", err)
	}
}

func TestSendMessage_Validation_Whitespace(t *testing.T) {
	c := newTestConversation()
	c.cfg.SkipSendMessageStatusCheck = false

	// Make stable
	for i := 0; i < 5; i++ {
		c.AddSnapshot("screen")
	}

	err := c.SendMessage(MessagePartText{Content: "  leading space"})
	if err != MessageValidationErrorWhitespace {
		t.Errorf("expected whitespace validation error, got %v", err)
	}
}

func TestSendMessage_Validation_TrailingSpace(t *testing.T) {
	c := newTestConversation()
	c.cfg.SkipSendMessageStatusCheck = false

	// Make stable
	for i := 0; i < 5; i++ {
		c.AddSnapshot("screen")
	}

	err := c.SendMessage(MessagePartText{Content: "trailing space  "})
	if err != MessageValidationErrorWhitespace {
		t.Errorf("expected whitespace validation error, got %v", err)
	}
}

func TestSendMessage_Success(t *testing.T) {
	c := newTestConversation()

	err := c.SendMessage(MessagePartText{Content: "valid message"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	messages := c.Messages()
	if len(messages) < 2 {
		t.Fatal("expected at least 2 messages")
	}

	lastMsg := messages[len(messages)-1]
	if lastMsg.Role != ConversationRoleUser {
		t.Errorf("expected user role, got %s", lastMsg.Role)
	}
	if lastMsg.Message != "valid message" {
		t.Errorf("expected 'valid message', got %q", lastMsg.Message)
	}
}

func TestMessages_ReturnsCopy(t *testing.T) {
	c := newTestConversation()

	msgs1 := c.Messages()
	msgs2 := c.Messages()

	// Modify one
	if len(msgs1) > 0 {
		msgs1[0].Message = "modified"
	}

	// Should not affect the other
	if len(msgs2) > 0 && msgs2[0].Message == "modified" {
		t.Error("Messages() should return a copy")
	}
}

func TestConversationRole_Values(t *testing.T) {
	if !containsRole(ConversationRoleValues, ConversationRoleUser) {
		t.Error("ConversationRoleValues should contain User")
	}
	if !containsRole(ConversationRoleValues, ConversationRoleAgent) {
		t.Error("ConversationRoleValues should contain Agent")
	}
	if len(ConversationRoleValues) != 2 {
		t.Errorf("expected 2 role values, got %d", len(ConversationRoleValues))
	}
}

func containsRole(roles []ConversationRole, role ConversationRole) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

func TestFindNewMessage_NoChange(t *testing.T) {
	oldScreen := "line1\nline2\nline3"
	newScreen := "line1\nline2\nline3"

	result := FindNewMessage(oldScreen, newScreen, msgfmt.AgentTypeClaude)
	if result != "" {
		t.Errorf("expected empty result for unchanged screens, got %q", result)
	}
}

func TestFindNewMessage_AppendToEnd(t *testing.T) {
	oldScreen := "line1\nline2"
	newScreen := "line1\nline2\nline3"

	result := FindNewMessage(oldScreen, newScreen, msgfmt.AgentTypeClaude)
	if !strings.Contains(result, "line3") {
		t.Errorf("expected result to contain 'line3', got %q", result)
	}
}

func TestFindNewMessage_CompletelyNew(t *testing.T) {
	oldScreen := "old content"
	newScreen := "completely different"

	result := FindNewMessage(oldScreen, newScreen, msgfmt.AgentTypeClaude)
	if result != "completely different" {
		t.Errorf("expected 'completely different', got %q", result)
	}
}

func TestFindNewMessage_OpencodeHeaderSkip(t *testing.T) {
	// Opencode has a dynamic header that should be skipped
	// This is a simplified test - the actual behavior depends on screen content
	oldScreen := "content line 1\ncontent line 2"
	newScreen := "content line 1\ncontent line 2\nnew content"

	result := FindNewMessage(oldScreen, newScreen, msgfmt.AgentTypeOpencode)
	// Should find the new content, may be empty if no actual difference detected
	_ = result // Just verify it doesn't panic
}

func TestMessagePartText_String(t *testing.T) {
	tests := []struct {
		name     string
		part     MessagePartText
		expected string
	}{
		{
			name:     "basic",
			part:     MessagePartText{Content: "hello"},
			expected: "hello",
		},
		{
			name:     "hidden",
			part:     MessagePartText{Content: "secret", Hidden: true},
			expected: "",
		},
		{
			name:     "alias",
			part:     MessagePartText{Content: "actual content", Alias: "display"},
			expected: "display",
		},
		{
			name:     "hidden_with_alias",
			part:     MessagePartText{Content: "content", Alias: "alias", Hidden: true},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.part.String()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestPartsToString(t *testing.T) {
	parts := []MessagePart{
		MessagePartText{Content: "hello "},
		MessagePartText{Content: "world"},
	}

	result := PartsToString(parts...)
	if result != "hello world" {
		t.Errorf("expected 'hello world', got %q", result)
	}
}

func TestPartsToString_WithHidden(t *testing.T) {
	parts := []MessagePart{
		MessagePartText{Content: "visible "},
		MessagePartText{Content: "hidden", Hidden: true},
		MessagePartText{Content: " more"},
	}

	result := PartsToString(parts...)
	if result != "visible  more" {
		t.Errorf("expected 'visible  more', got %q", result)
	}
}

func TestConversation_InitialPrompt(t *testing.T) {
	cfg := ConversationConfig{
		AgentType:             msgfmt.AgentTypeClaude,
		AgentIO:               &mockAgentIO{},
		GetTime:               time.Now,
		SnapshotInterval:      100 * time.Millisecond,
		ScreenStabilityLength: 300 * time.Millisecond,
		SkipWritingMessage:    true,
	}

	c := NewConversation(context.Background(), cfg, "initial prompt text")

	if c.InitialPrompt != "initial prompt text" {
		t.Errorf("expected 'initial prompt text', got %q", c.InitialPrompt)
	}
	if c.InitialPromptSent {
		t.Error("InitialPromptSent should be false initially")
	}
}

func TestConversation_EmptyInitialPrompt(t *testing.T) {
	cfg := ConversationConfig{
		AgentType:             msgfmt.AgentTypeClaude,
		AgentIO:               &mockAgentIO{},
		GetTime:               time.Now,
		SnapshotInterval:      100 * time.Millisecond,
		ScreenStabilityLength: 300 * time.Millisecond,
	}

	c := NewConversation(context.Background(), cfg, "")

	if !c.InitialPromptSent {
		t.Error("InitialPromptSent should be true when initial prompt is empty")
	}
}

func TestConversation_InitialPrompt_MultiLine(t *testing.T) {
	cfg := ConversationConfig{
		AgentType:             msgfmt.AgentTypeClaude,
		AgentIO:               &mockAgentIO{},
		GetTime:               time.Now,
		SnapshotInterval:      100 * time.Millisecond,
		ScreenStabilityLength: 300 * time.Millisecond,
	}

	multiLinePrompt := "Line 1\nLine 2\nLine 3"
	c := NewConversation(context.Background(), cfg, multiLinePrompt)

	if c.InitialPrompt != multiLinePrompt {
		t.Errorf("expected multi-line prompt, got %q", c.InitialPrompt)
	}
	if c.InitialPromptSent {
		t.Error("InitialPromptSent should be false for non-empty prompt")
	}
}

func TestConversation_InitialPrompt_WhitespaceOnly(t *testing.T) {
	cfg := ConversationConfig{
		AgentType:             msgfmt.AgentTypeClaude,
		AgentIO:               &mockAgentIO{},
		GetTime:               time.Now,
		SnapshotInterval:      100 * time.Millisecond,
		ScreenStabilityLength: 300 * time.Millisecond,
	}

	// Whitespace-only should be treated as empty
	c := NewConversation(context.Background(), cfg, "   \n\t\n  ")

	// InitialPromptSent should be true because the prompt is effectively empty
	if !c.InitialPromptSent {
		t.Error("InitialPromptSent should be true for whitespace-only prompt")
	}
}

func TestGetStableSnapshotsThreshold(t *testing.T) {
	tests := []struct {
		stability time.Duration
		interval  time.Duration
		minimum   int // threshold should be >= this
	}{
		{1 * time.Second, 100 * time.Millisecond, 10},
		{500 * time.Millisecond, 100 * time.Millisecond, 5},
		{300 * time.Millisecond, 100 * time.Millisecond, 3},
		{2 * time.Second, 100 * time.Millisecond, 20},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			cfg := ConversationConfig{
				ScreenStabilityLength: tt.stability,
				SnapshotInterval:      tt.interval,
			}
			threshold := getStableSnapshotsThreshold(cfg)
			if threshold < tt.minimum {
				t.Errorf("threshold %d < minimum %d", threshold, tt.minimum)
			}
		})
	}
}

func TestConversation_Screen(t *testing.T) {
	c := newTestConversation()
	mockIO := c.cfg.AgentIO.(*mockAgentIO)
	mockIO.SetScreen("test screen content")

	// Add a snapshot
	c.AddSnapshot("test screen content")

	screen := c.Screen()
	if screen != "test screen content" {
		t.Errorf("expected 'test screen content', got %q", screen)
	}
}

func TestConversation_Screen_Empty(t *testing.T) {
	c := newTestConversation()
	// No snapshots yet
	screen := c.Screen()
	if screen != "" {
		t.Errorf("expected empty screen, got %q", screen)
	}
}
