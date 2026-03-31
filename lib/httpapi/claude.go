package httpapi

import (
	mf "github.com/coder/agentapi/lib/msgfmt"
	st "github.com/coder/agentapi/lib/screentracker"
)

// formatPaste wraps message in bracketed paste escape sequences.
// These sequences start with ESC (\x1b), which TUI selection
// widgets (e.g. Claude Code's numbered-choice prompt) interpret
// as "cancel". For selection prompts, callers should use
// MessageTypeRaw to send raw keystrokes directly instead.
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

	return parts
}

func FormatMessage(agentType mf.AgentType, message string) []st.MessagePart {
	message = mf.TrimWhitespace(message)
	// for now Claude Code formatting seems to also work for Goose and Aider
	// so we can use the same function for all three
	return formatClaudeCodeMessage(message)
}
