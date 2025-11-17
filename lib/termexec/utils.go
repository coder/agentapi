package termexec

import (
	"strings"

	"github.com/coder/agentapi/lib/msgfmt"
)

func calcAmpDynamicHeader(newLines []string) (int, bool) {
	dynamicHeaderEnd := -1
	firstTextEncountered := false
	continueRemoving := true

	// search for the first 3 consecutive empty lines after the first text encountered.
	if len(newLines) > 3 {
		for i := 0; i < len(newLines)-3; i++ {
			if !firstTextEncountered && len(strings.Trim(newLines[i], " \n")) != 0 {
				if strings.HasPrefix(strings.TrimSpace(newLines[i]), "┃") {
					continueRemoving = false
				}
				firstTextEncountered = true
			}
			if firstTextEncountered && len(strings.Trim(newLines[i], " \n")) == 0 && len(strings.Trim(newLines[i+1], " \n")) == 0 &&
				len(strings.Trim(newLines[i+2], " \n")) == 0 {
				dynamicHeaderEnd = i
				break

			}
		}
	}
	return dynamicHeaderEnd, continueRemoving
}

func calcOpencodeDynamicHeader(newLines []string) (int, bool) {
	// Skip header lines for Opencode agent type to avoid false positives
	// The header contains dynamic content (token count, context percentage, cost)
	// that changes between screens, causing line comparison mismatches:
	//
	// ┃  # Getting Started with Claude CLI                                   ┃
	// ┃  /share to create a shareable link                 12.6K/6% ($0.05)  ┃
	if len(newLines) >= 2 {
		return 2, true
	}
	return -1, true
}

func removeDynamicHeader(screen string, agentType msgfmt.AgentType) (string, bool) {
	lines := strings.Split(screen, "\n")
	dynamicHeaderEnd := -1
	continueRemoving := true
	if agentType == msgfmt.AgentTypeAmp {
		dynamicHeaderEnd, continueRemoving = calcAmpDynamicHeader(lines)
	} else if agentType == msgfmt.AgentTypeOpencode {
		dynamicHeaderEnd, continueRemoving = calcOpencodeDynamicHeader(lines)
	} else {
		continueRemoving = false
	}
	return strings.Join(lines[dynamicHeaderEnd+1:], "\n"), continueRemoving
}
