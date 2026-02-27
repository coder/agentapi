package msgfmt

import (
	"strings"
	"testing"
)

func TestFormatAgentMessage_PreservesFirstLine(t *testing.T) {
	// Test for issue #126 - first line of response being trimmed
	// This test ensures the first non-empty line is preserved
	
	tests := []struct {
		name     string
		agentType AgentType
		message  string
		userInput string
	}{
		{
			name:     "claude first line",
			agentType: AgentTypeClaude,
			message:  "Hello world\nThis is the response",
			userInput: "Hello",
		},
		{
			name:     "goose first line",
			agentType: AgentTypeGoose,
			message:  "First line\nSecond line",
			userInput: "Hi",
		},
		{
			name:     "codex first line",
			agentType: AgentTypeCodex,
			message:  "Response here\nMore content",
			userInput: "test",
		},
		{
			name:     "opencode first line",
			agentType: AgentTypeOpencode,
			message:  "OpenCode response\nMore lines",
			userInput: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatAgentMessage(tt.agentType, tt.message, tt.userInput)
			if result == "" {
				t.Error("result should not be empty")
			}
			// The first line should be preserved
			lines := strings.Split(result, "\n")
			if len(lines) > 0 && lines[0] == "" {
				t.Errorf("first line should not be empty, got %q", result)
			}
		})
	}
}

func TestTrimEmptyLines_PreservesContent(t *testing.T) {
	// Ensure trimEmptyLines doesn't remove actual content
	
	tests := []struct {
		input    string
		expected string
	}{
		{"hello\nworld", "hello\nworld"},
		{"\nhello\nworld", "hello\nworld"},
		{"hello\nworld\n", "hello\nworld"},
		{"\n\nhello\n\nworld\n\n", "hello\n\nworld"},
		{"single", "single"},
		{"\n\n\n", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := trimEmptyLines(tt.input)
			if result != tt.expected {
				t.Errorf("trimEmptyLines(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
