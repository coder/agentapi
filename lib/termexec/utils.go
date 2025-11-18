package termexec

import (
	"strings"

	"github.com/coder/agentapi/lib/msgfmt"
)

func calcAmpAnimatedContent(lines []string) (int, bool) {
	animatedContentEnd := -1
	firstTextEncountered := false
	continueRemoving := true

	// search for the first 3 consecutive empty lines after the first text encountered.
	if len(lines) > 3 {
		for i := 0; i < len(lines)-3; i++ {
			if !firstTextEncountered && len(strings.Trim(lines[i], " \n")) != 0 {
				if strings.HasPrefix(strings.TrimSpace(lines[i]), "┃") {
					continueRemoving = false
				}
				firstTextEncountered = true
			}
			if firstTextEncountered && len(strings.Trim(lines[i], " \n")) == 0 && len(strings.Trim(lines[i+1], " \n")) == 0 &&
				len(strings.Trim(lines[i+2], " \n")) == 0 {
				animatedContentEnd = i
				break

			}
		}
	}
	return animatedContentEnd, continueRemoving
}

func calcOpencodeAnimatedContent(lines []string) (int, bool) {
	// Skip header lines for Opencode agent type to avoid false positives
	// The header contains dynamic content (token count, context percentage, cost)
	// that changes between screens, causing line comparison mismatches:
	//
	// ┃  # Getting Started with Claude CLI                                   ┃
	// ┃  /share to create a shareable link                 12.6K/6% ($0.05)  ┃
	if len(lines) >= 2 {
		return 2, true
	}
	return -1, true
}

func removeAnimatedContent(screen string, agentType msgfmt.AgentType) (string, bool) {
	lines := strings.Split(screen, "\n")
	animatedContentEnd := -1
	var continueRemoving bool
	if agentType == msgfmt.AgentTypeAmp {
		animatedContentEnd, continueRemoving = calcAmpAnimatedContent(lines)
	} else if agentType == msgfmt.AgentTypeOpencode {
		animatedContentEnd, continueRemoving = calcOpencodeAnimatedContent(lines)
	} else {
		continueRemoving = false
	}
	return strings.Join(lines[animatedContentEnd+1:], "\n"), continueRemoving
}
