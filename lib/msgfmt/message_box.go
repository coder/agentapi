package msgfmt

import (
	"strings"
)

// Usually something like
// ───────────────
// >
// ───────────────
// Used by Claude Code, Goose, and Aider.
func findGreaterThanMessageBox(lines []string) int {
	for i := len(lines) - 1; i >= max(len(lines)-6, 0); i-- {
		if strings.Contains(lines[i], ">") {
			if i > 0 && strings.Contains(lines[i-1], "───────────────") {
				return i - 1
			}
			return i
		}
	}
	return -1
}

// Usually something like
// ───────────────
// |
// ───────────────
func findGenericSlimMessageBox(lines []string) int {
	for i := len(lines) - 3; i >= max(len(lines)-9, 0); i-- {
		if strings.Contains(lines[i], "───────────────") &&
			(strings.Contains(lines[i+1], "|") || strings.Contains(lines[i+1], "│")) &&
			strings.Contains(lines[i+2], "───────────────") {
			return i
		}
	}
	return -1
}

func removeMessageBox(msg string) string {
	lines := strings.Split(msg, "\n")

	messageBoxStartIdx := findGreaterThanMessageBox(lines)
	if messageBoxStartIdx == -1 {
		messageBoxStartIdx = findGenericSlimMessageBox(lines)
	}

	if messageBoxStartIdx != -1 {
		lines = lines[:messageBoxStartIdx]
	}

	return strings.Join(lines, "\n")
}

func removeCodexInputBox(msg string) string {
	lines := strings.Split(msg, "\n")
	// Remove the input box, we need to match the exact pattern, because thinking follows the same pattern of ▌ followed by text
	if len(lines) >= 2 && strings.Contains(lines[len(lines)-2], "▌ Ask Codex to do anything") {
		idx := len(lines) - 2
		lines = append(lines[:idx], lines[idx+1:]...)
	}
	return strings.Join(lines, "\n")
}

func removeOpencodeMessageBox(msg string) string {
	lines := strings.Split(msg, "\n")
	//
	//  ┃
	//  ┃
	//  ┃
	//  ┃  Build  Anthropic Claude Sonnet 4
	//  ╹▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀
	//                                tab switch agent  ctrl+p commands
	//
	for i := len(lines) - 1; i >= 4; i-- {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "╹▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀") {
			lines = lines[:i-4]
			break
		}
	}
	return strings.Join(lines, "\n")
}

func removeAmpMessageBox(msg string) string {
	lines := strings.Split(msg, "\n")
	msgBoxEndFound := false
	msgBoxStartIdx := len(lines)
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if !msgBoxEndFound && strings.HasPrefix(line, "╰") && strings.HasSuffix(line, "╯") {
			msgBoxEndFound = true
		}
		if msgBoxEndFound && strings.HasPrefix(line, "╭") && strings.HasSuffix(line, "╮") {
			msgBoxStartIdx = i
			break
		}
	}
	formattedMsg := strings.Join(lines[:msgBoxStartIdx], "\n")
	if len(formattedMsg) == 0 {
		return "Welcome to Amp"
	}
	return formattedMsg
}

func removeClaudeReportTaskToolCall(msg string) string {
	// Remove all tool calls that start with `● coder - coder_report_task (MCP)` and end with `}`
	lines := strings.Split(msg, "\n")
	toolCallEndIdx := -1

	// Store all tool call start and end indices [[start, end], ...]
	var toolCallIdxs [][]int

	// Iterate backwards to find all occurrences
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "}" {
			toolCallEndIdx = i
		}
		if toolCallEndIdx != -1 && strings.HasPrefix(line, "● coder - coder_report_task (MCP)") {
			// Store [start, end] pair
			toolCallIdxs = append(toolCallIdxs, []int{i, toolCallEndIdx})

			// Reset to find the next tool call
			toolCallEndIdx = -1
		}
	}

	// If no tool calls found, return original message
	if len(toolCallIdxs) == 0 {
		return msg
	}

	// Remove tool calls from the message
	for _, idxPair := range toolCallIdxs {
		start, end := idxPair[0], idxPair[1]
		lines = append(lines[:start], lines[end+1:]...)
	}

	return strings.Join(lines, "\n")
}
