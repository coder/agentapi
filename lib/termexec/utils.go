package termexec

import (
	"strings"

	"github.com/coder/agentapi/lib/msgfmt"
)

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
	switch agentType {
	case msgfmt.AgentTypeOpencode:
		lines := strings.Split(screen, "\n")
		animatedContentEnd, continueRemoving := calcOpencodeAnimatedContent(lines)
		return strings.Join(lines[animatedContentEnd+1:], "\n"), continueRemoving
	default:
		return screen, false
	}
}
