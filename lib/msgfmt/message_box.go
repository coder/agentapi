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
