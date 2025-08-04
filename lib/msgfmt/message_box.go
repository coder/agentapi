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
// Used by OpenAI Codex.
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

func removeCodexMessageBox(msg string) string {
	lines := strings.Split(msg, "\n")
	messageBoxEndIdx := -1
	messageBoxStartIdx := -1

	for i := len(lines) - 1; i >= 0; i-- {
		if messageBoxEndIdx == -1 {
			if strings.Contains(lines[i], "╰────────") && strings.Contains(lines[i], "───────╯") {
				messageBoxEndIdx = i
			}
		} else {
			// We reached the start of the message box (we don't want to show this line), also exit the loop
			if strings.Contains(lines[i], "╭") && strings.Contains(lines[i], "───────╮") {
				// We only want this to be i in case the top of the box is visible
				messageBoxStartIdx = i
				break
			}

			// We are in between the start and end of the message box, so remove the │ from the start and end of the line, let the trimEmptyLines handle the rest
			if strings.HasPrefix(lines[i], "│") {
				lines[i] = strings.TrimPrefix(lines[i], "│")
			}
			if strings.HasSuffix(lines[i], "│") {
				lines[i] = strings.TrimSuffix(lines[i], "│")
				lines[i] = strings.TrimRight(lines[i], " \t")
			}
		}
	}

	// If we didn't find messageBoxEndIdx, set it to the end of the lines
	if messageBoxEndIdx == -1 {
		messageBoxEndIdx = len(lines)
	}

	return strings.Join(lines[messageBoxStartIdx+1:messageBoxEndIdx], "\n")

}
