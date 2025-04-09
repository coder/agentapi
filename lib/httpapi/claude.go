package httpapi

import (
	"time"

	"github.com/coder/openagent/lib/msgfmt"
	st "github.com/coder/openagent/lib/screentracker"
)

type AgentType string

const (
	AgentTypeClaude AgentType = "claude"
	AgentTypeGoose  AgentType = "goose"
	AgentTypeAider  AgentType = "aider"
	AgentTypeCustom AgentType = "custom"
)

func formatPaste(message string) []st.MessagePart {
	return []st.MessagePart{
		// Bracketed paste mode start sequence
		st.MessagePartText{Content: "\x1b[200~", Hidden: true},
		st.MessagePartText{Content: message},
		// Bracketed paste mode end sequence
		st.MessagePartText{Content: "\x1b[201~", Hidden: true},
	}
}

func formatClaudeCodeMessage(message string) []st.MessagePart {
	parts := make([]st.MessagePart, 0)
	// janky hack: send a random character and then a backspace because otherwise
	// Claude Code echoes the startSeq back to the terminal.
	// This basically simulates a user typing and then removing the character.
	parts = append(parts, st.MessagePartText{Content: "x\b", Hidden: true})
	parts = append(parts, formatPaste(message)...)
	// wait because Claude Code doesn't recognize "\r" as a command
	// to process the input if it's sent right away
	parts = append(parts, st.MessagePartWait{Duration: 50 * time.Millisecond})
	parts = append(parts, st.MessagePartText{Content: "\r", Hidden: true})

	return parts
}

func FormatMessage(agentType AgentType, message string) []st.MessagePart {
	message = msgfmt.TrimWhitespace(message)
	// for now Claude Code formatting seems to also work for Goose and Aider
	// so we can use the same function for all three
	return formatClaudeCodeMessage(message)
}
