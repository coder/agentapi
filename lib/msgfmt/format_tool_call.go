package msgfmt

import (
	"strings"
)

func removeClaudeReportTaskToolCall(msg string) (string, []string) {
	msg = "\n" + msg // This handles the case where the message starts with a tool call

	// Remove all tool calls that start with `● coder - coder_report_task (MCP)`
	lines := strings.Split(msg, "\n")

	toolCallStartIdx := -1

	// Store all tool call start and end indices [[start, end], ...]
	var toolCallIdxs [][]int

	for i := 1; i < len(lines)-1; i++ {
		prevLine := strings.TrimSpace(lines[i-1])
		line := strings.TrimSpace(lines[i])
		nextLine := strings.TrimSpace(lines[i+1])

		if strings.Contains(line, "coder - coder_report_task (MCP)") {
			toolCallStartIdx = i
		} else if toolCallStartIdx != -1 && line == "\"message\": \"Thanks for reporting!\"" && nextLine == "}" && strings.HasSuffix(prevLine, "{") {
			// Store [start, end] pair
			toolCallIdxs = append(toolCallIdxs, []int{toolCallStartIdx, min(len(lines), i+2)})

			// Reset to find the next tool call
			toolCallStartIdx = -1
		}
	}

	// If no tool calls found, return original message
	if len(toolCallIdxs) == 0 {
		return strings.TrimLeft(msg, "\n"), []string{}
	}

	toolCallMessages := make([]string, 0)

	// Remove tool calls from the message
	for i := len(toolCallIdxs) - 1; i >= 0; i-- {
		idxPair := toolCallIdxs[i]
		start, end := idxPair[0], idxPair[1]

		toolCallMessages = append(toolCallMessages, strings.Join(lines[start:end], "\n"))

		lines = append(lines[:start], lines[end:]...)
	}
	return strings.TrimLeft(strings.Join(lines, "\n"), "\n"), toolCallMessages
}

func removeCodexReportTaskToolCall(msg string) (string, []string) {
	lines := strings.Split(msg, "\n")

	toolCallStartIdx := -1

	// Store all tool call start and end indices [[start, end], ...]
	var toolCallIdxs [][]int

	for idx := 0; idx < len(lines); {
		line := strings.TrimSpace(lines[idx])

		// Check for tool call start (requires looking at next line)
		if idx+1 < len(lines) {
			nextLine := strings.TrimSpace(lines[idx+1])
			if strings.Replace(line, " ", "", -1) == "•Called" && strings.Contains(nextLine, "Coder.coder_report_task") {
				toolCallStartIdx = idx
			}
		}

		// Check for tool call end
		if toolCallStartIdx != -1 && line == "{\"message\": \"Thanks for reporting!\"}" {
			// Store [start, end] pair
			// trim all the remaining empty lines after tool call
			for idx+1 < len(lines) {
				if strings.TrimSpace(lines[idx+1]) == "" {
					idx++
				} else {
					break
				}
			}
			toolCallIdxs = append(toolCallIdxs, []int{toolCallStartIdx, idx + 1})

			// Reset to find the next tool call
			toolCallStartIdx = -1
		}
		idx++
	}

	// If no tool calls found, return original message
	if len(toolCallIdxs) == 0 {
		return strings.TrimRight(msg, "\n"), []string{}
	}

	toolCallMessages := make([]string, 0)

	// Remove tool calls from the message
	for i := len(toolCallIdxs) - 1; i >= 0; i-- {
		idxPair := toolCallIdxs[i]
		start, end := idxPair[0], idxPair[1]

		toolCallMessages = append(toolCallMessages, strings.Join(lines[start:end], "\n"))

		lines = append(lines[:start], lines[end:]...)
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n"), toolCallMessages
}

func FormatToolCall(agentType AgentType, message string) (string, []string) {
	switch agentType {
	case AgentTypeClaude:
		return removeClaudeReportTaskToolCall(message)
	case AgentTypeGoose:
		return message, []string{}
	case AgentTypeAider:
		return message, []string{}
	case AgentTypeCodex:
		return removeCodexReportTaskToolCall(message)
	case AgentTypeGemini:
		return message, []string{}
	case AgentTypeCopilot:
		return message, []string{}
	case AgentTypeAmp:
		return message, []string{}
	case AgentTypeCursor:
		return message, []string{}
	case AgentTypeAuggie:
		return message, []string{}
	case AgentTypeAmazonQ:
		return message, []string{}
	case AgentTypeOpencode:
		return message, []string{}
	case AgentTypeCustom:
		return message, []string{}
	default:
		return message, []string{}
	}
}
