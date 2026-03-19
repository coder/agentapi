package msgfmt

import (
	"regexp"
	"strings"
)

var genericSlimMessageBoxPattern = regexp.MustCompile(`(?m)^.*─{15,}.*\n.*[|│❯].*\n(?:.*\n)?.*─{15,}.*`)

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
	// genericSlimMessageBoxPattern matches a message box pattern like:
	// ───────────────
	// | or │ or ❯
	//
	// (optional line(s))
	// ───────────────

	// Search within the last ~9 lines for the message box pattern
	startIdx := max(len(lines)-9, 0)
	searchText := strings.Join(lines[startIdx:], "\n")

	loc := genericSlimMessageBoxPattern.FindStringIndex(searchText)
	if loc == nil {
		return -1
	}

	// Count newlines before the match to find the line number
	linesBeforeMatch := strings.Count(searchText[:loc[0]], "\n")
	return startIdx + linesBeforeMatch
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
	if len(lines) >= 3 && strings.Contains(lines[len(lines)-3], "›") {
		idx := len(lines) - 3
		lines = append(lines[:idx], lines[idx+2])
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
