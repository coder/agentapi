package msgfmt

import "strings"

func IsAgentReadyForInitialPrompt(agentType AgentType, message string) bool {
	switch agentType {
	case AgentTypeClaude:
		return isGenericAgentReadyForInitialPrompt(message)
	case AgentTypeGoose:
		return isGenericAgentReadyForInitialPrompt(message)
	case AgentTypeAider:
		return isGenericAgentReadyForInitialPrompt(message)
	case AgentTypeCodex:
		return isCodexAgentReadyForInitialPrompt(message)
	case AgentTypeGemini:
		return isGenericAgentReadyForInitialPrompt(message)
	case AgentTypeCopilot:
		return isGenericAgentReadyForInitialPrompt(message)
	case AgentTypeAmp:
		return isAmpAgentReadyForInitialPrompt(message)
	case AgentTypeCursor:
		return isCursorAgentReadyForInitialPrompt(message)
	case AgentTypeAuggie:
		return isGenericAgentReadyForInitialPrompt(message)
	case AgentTypeAmazonQ:
		return isGenericAgentReadyForInitialPrompt(message)
	case AgentTypeOpencode:
		return isOpencodeAgentReadyForInitialPrompt(message)
	case AgentTypeCustom:
		return isGenericAgentReadyForInitialPrompt(message)
	default:
		return true
	}
}

func isGenericAgentReadyForInitialPrompt(message string) bool {
	message = trimEmptyLines(message)
	messageWithoutInputBox := removeMessageBox(message)
	return len(messageWithoutInputBox) != len(message)
}

func isCursorAgentReadyForInitialPrompt(message string) bool {
	if isGenericAgentReadyForInitialPrompt(message) {
		return true
	}
	normalized, _, _ := normalizeAndGetRuneLineMapping(message)
	return strings.Contains(message, "Cursor Agent") &&
		strings.Contains(string(normalized), "Plan,search,buildanything")
}

func isOpencodeAgentReadyForInitialPrompt(message string) bool {
	message = trimEmptyLines(message)
	messageWithoutInputBox := removeOpencodeMessageBox(message)
	return len(messageWithoutInputBox) != len(message)
}

func isCodexAgentReadyForInitialPrompt(message string) bool {
	message = trimEmptyLines(message)
	messageWithoutInputBox := removeCodexMessageBox(message)
	return len(messageWithoutInputBox) != len(message)
}

func isAmpAgentReadyForInitialPrompt(message string) bool {
	message = trimEmptyLines(message)
	messageWithoutInputBox := removeAmpMessageBox(message)
	return len(messageWithoutInputBox) != len(message)
}
