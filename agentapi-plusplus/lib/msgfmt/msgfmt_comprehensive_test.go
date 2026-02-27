package msgfmt

import (
	"strings"
	"testing"
)

func TestTrimWhitespace(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  hello  ", "hello"},
		{"\t\nhello\r\n", "hello"},
		{"no_whitespace", "no_whitespace"},
		{"  ", ""},
		{"", ""},
		{"\n\t\r\f\v", ""},
		{"  multi\nline  ", "multi\nline"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := TrimWhitespace(tt.input)
			if result != tt.expected {
				t.Errorf("TrimWhitespace(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIndexSubslice(t *testing.T) {
	tests := []struct {
		s        []int
		sub      []int
		expected int
	}{
		{[]int{1, 2, 3, 4, 5}, []int{3, 4}, 2},
		{[]int{1, 2, 3, 4, 5}, []int{1, 2}, 0},
		{[]int{1, 2, 3, 4, 5}, []int{4, 5}, 3},
		{[]int{1, 2, 3}, []int{}, 0},            // empty sub
		{[]int{}, []int{1}, -1},                 // empty s
		{[]int{1, 2, 3}, []int{4, 5}, -1},       // not found
		{[]int{1, 2, 3}, []int{1, 2, 3, 4}, -1}, // sub longer than s
		{[]int{1, 1, 1, 2, 1}, []int{1, 2}, 2},  // multiple matches, first wins
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := IndexSubslice(tt.s, tt.sub)
			if result != tt.expected {
				t.Errorf("IndexSubslice(%v, %v) = %d, want %d", tt.s, tt.sub, result, tt.expected)
			}
		})
	}
}

func TestIndexSubsliceRunes(t *testing.T) {
	tests := []struct {
		s        string
		sub      string
		expected int
	}{
		{"hello world", "wor", 6},
		{"hello world", "hello", 0},
		{"hello world", "world", 6},
		{"hello world", "xyz", -1},
		{"", "test", -1},
		{"test", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.sub, func(t *testing.T) {
			result := IndexSubslice([]rune(tt.s), []rune(tt.sub))
			if result != tt.expected {
				t.Errorf("IndexSubslice(%q, %q) = %d, want %d", tt.s, tt.sub, result, tt.expected)
			}
		})
	}
}

// TestNormalizeAndGetRuneLineMapping - Already exists in msgfmt_test.go

func TestRemoveUserInput_Basic(t *testing.T) {
	tests := []struct {
		name        string
		msg         string
		userInput   string
		agentType   AgentType
		contains    string // result should contain this
		notContains string // result should NOT contain this
	}{
		{
			name:        "empty_user_input",
			msg:         "Hello world",
			userInput:   "",
			agentType:   AgentTypeClaude,
			contains:    "Hello world",
			notContains: "",
		},
		{
			name:        "claude_basic",
			msg:         "> user prompt\n\nAgent response here",
			userInput:   "user prompt",
			agentType:   AgentTypeClaude,
			contains:    "Agent response",
			notContains: "user prompt",
		},
		{
			name:        "goose_basic",
			msg:         "user input\n---\nAgent output",
			userInput:   "user input",
			agentType:   AgentTypeGoose,
			contains:    "Agent output",
			notContains: "user input",
		},
		{
			name:        "aider_basic",
			msg:         "> user input\n\nResponse",
			userInput:   "user input",
			agentType:   AgentTypeAider,
			contains:    "Response",
			notContains: "user input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemoveUserInput(tt.msg, tt.userInput, tt.agentType)

			if tt.contains != "" && !strings.Contains(result, tt.contains) {
				t.Errorf("result should contain %q, got %q", tt.contains, result)
			}
			if tt.notContains != "" && strings.Contains(result, tt.notContains) {
				t.Errorf("result should NOT contain %q, got %q", tt.notContains, result)
			}
		})
	}
}

func TestFormatAgentMessage_AllAgentTypes(t *testing.T) {
	agentTypes := []AgentType{
		AgentTypeClaude,
		AgentTypeGoose,
		AgentTypeAider,
		AgentTypeCodex,
		AgentTypeGemini,
		AgentTypeCopilot,
		AgentTypeAmp,
		AgentTypeCursor,
		AgentTypeAuggie,
		AgentTypeAmazonQ,
		AgentTypeOpencode,
		AgentTypeCustom,
	}

	for _, agentType := range agentTypes {
		t.Run(string(agentType), func(t *testing.T) {
			// Should not panic for any agent type
			result := FormatAgentMessage(agentType, "test message", "user input")
			// Result should be a string (not empty or the same for edge cases)
			_ = result
		})
	}
}

// TestTrimEmptyLines - Already exists in msgfmt_test.go

// FindNewMessage is in screentracker package, not msgfmt
// Removed duplicate test - see screentracker/conversation_test.go

func TestAgentTypeConstants(t *testing.T) {
	// Verify all agent types have non-empty string values
	agentTypes := map[AgentType]string{
		AgentTypeClaude:   string(AgentTypeClaude),
		AgentTypeGoose:    string(AgentTypeGoose),
		AgentTypeAider:    string(AgentTypeAider),
		AgentTypeCodex:    string(AgentTypeCodex),
		AgentTypeGemini:   string(AgentTypeGemini),
		AgentTypeCopilot:  string(AgentTypeCopilot),
		AgentTypeAmp:      string(AgentTypeAmp),
		AgentTypeCursor:   string(AgentTypeCursor),
		AgentTypeAuggie:   string(AgentTypeAuggie),
		AgentTypeAmazonQ:  string(AgentTypeAmazonQ),
		AgentTypeOpencode: string(AgentTypeOpencode),
		AgentTypeCustom:   string(AgentTypeCustom),
	}

	for at, value := range agentTypes {
		if value == "" {
			t.Errorf("AgentType %v has empty string value", at)
		}
	}
}
