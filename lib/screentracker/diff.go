package screentracker

import (
	"strings"

	"github.com/coder/agentapi/lib/msgfmt"
)

// screenDiff compares two screen states and attempts to find latest message of the given agent type.
func screenDiff(oldScreen, newScreen string, agentType msgfmt.AgentType) string {
	oldLines := strings.Split(oldScreen, "\n")
	newLines := strings.Split(newScreen, "\n")
	oldLinesMap := make(map[string]bool)

	// -1 indicates no header
	dynamicHeaderEnd := -1

	// Skip header lines for Opencode agent type to avoid false positives
	// The header contains dynamic content (token count, context percentage, cost)
	// that changes between screens, causing line comparison mismatches:
	//
	// ┃  # Getting Started with Claude CLI                                   ┃
	// ┃  /share to create a shareable link                 12.6K/6% ($0.05)  ┃
	if len(newLines) >= 2 && agentType == msgfmt.AgentTypeOpencode {
		dynamicHeaderEnd = 2
	}

	for _, line := range oldLines {
		oldLinesMap[line] = true
	}
	firstNonMatchingLine := len(newLines)
	for i, line := range newLines[dynamicHeaderEnd+1:] {
		if !oldLinesMap[line] {
			firstNonMatchingLine = i
			break
		}
	}
	newSectionLines := newLines[firstNonMatchingLine:]

	// remove leading and trailing lines which are empty or have only whitespace
	startLine := 0
	endLine := len(newSectionLines) - 1
	for i := range newSectionLines {
		if strings.TrimSpace(newSectionLines[i]) != "" {
			startLine = i
			break
		}
	}
	for i := len(newSectionLines) - 1; i >= 0; i-- {
		if strings.TrimSpace(newSectionLines[i]) != "" {
			endLine = i
			break
		}
	}
	return strings.Join(newSectionLines[startLine:endLine+1], "\n")
}
