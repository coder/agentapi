package msgfmt

import (
	"strings"
)

const WhiteSpaceChars = " \t\n\r\f\v"

func TrimWhitespace(msg string) string {
	return strings.Trim(msg, WhiteSpaceChars)
}

// IndexSubslice returns the index of the first instance of sub in s,
// or -1 if sub is not present in s.
// It's not the optimal algorithm - KMP would be better - but I don't
// want to implement anything more complex. If I can find a library
// that implements a faster algorithm, I'll use it.
func IndexSubslice[T comparable](s, sub []T) int {
	if len(sub) == 0 {
		return 0
	}
	if len(sub) > len(s) {
		return -1
	}

	for i := 0; i <= len(s)-len(sub); i++ {
		matched := true
		for j := 0; j < len(sub); j++ {
			if s[i+j] != sub[j] {
				matched = false
				break
			}
		}
		if matched {
			return i
		}
	}
	return -1
}

// Normalize the string to remove any whitespace.
// Remember in which line each rune is located.
// Return the runes, the lines, and the rune to line location mapping.
func normalizeAndGetRuneLineMapping(msgRaw string) ([]rune, []string, []int) {
	msgLines := strings.Split(msgRaw, "\n")
	msgRuneLineLocations := []int{}
	runes := []rune{}
	for lineIdx, line := range msgLines {
		for _, r := range line {
			if !strings.ContainsRune(WhiteSpaceChars, r) {
				runes = append(runes, r)
				msgRuneLineLocations = append(msgRuneLineLocations, lineIdx)
			}
		}
	}
	return runes, msgLines, msgRuneLineLocations
}

// Find where the user input starts in the message
func findUserInputStartIdx(msg []rune, msgRuneLineLocations []int, userInput []rune, userInputLineLocations []int) int {
	// We take up to 6 runes from the first line of the user input
	// and search for it in the message. 6 is arbitrary.
	// We only look at the first line to avoid running into user input
	// being broken up by UI elements.
	maxUserInputPrefixLen := 6
	userInputPrefixLen := -1
	for i, lineIdx := range userInputLineLocations {
		if lineIdx > 0 {
			break
		}
		if i >= maxUserInputPrefixLen {
			break
		}
		userInputPrefixLen = i + 1
	}
	if userInputPrefixLen == -1 {
		return -1
	}
	userInputPrefix := userInput[:userInputPrefixLen]

	// We'll only search the first 5 lines or 25 runes of the message,
	// whichever has more runes. This number is arbitrary. The intuition
	// is that user input is echoed back at the start of the message. The first
	// line or two may contain some UI elements.
	msgPrefixLen := 0
	for i, lineIdx := range msgRuneLineLocations {
		if lineIdx > 5 {
			break
		}
		msgPrefixLen = i + 1
	}
	defaultRunesFromMsg := 25
	if msgPrefixLen < defaultRunesFromMsg {
		msgPrefixLen = defaultRunesFromMsg
	}
	if msgPrefixLen > len(msg) {
		msgPrefixLen = len(msg)
	}
	msgPrefix := msg[:msgPrefixLen]

	return IndexSubslice(msgPrefix, userInputPrefix)
}

// Find the next match between the message and the user input.
// We're assuming that user input likely won't be truncated much,
// but it's likely some characters will be missing (e.g. OpenAI Codex strips
// "```" and instead formats enclosed text as a code block).
// We're going to see if any of the next 5 runes in the message
// match any of the next 5 runes in the user input.
func findNextMatch(knownMsgMatchIdx int, knownUserInputMatchIdx int, msg []rune, userInput []rune) (int, int) {
	for i := range 5 {
		for j := range 5 {
			userInputIdx := knownUserInputMatchIdx + i + 1
			msgIdx := knownMsgMatchIdx + j + 1

			if userInputIdx >= len(userInput) || msgIdx >= len(msg) {
				return -1, -1
			}
			if userInput[userInputIdx] == msg[msgIdx] {
				return msgIdx, userInputIdx
			}
		}
	}
	return -1, -1
}

// Find where the user input ends in the message. Returns the index of the last rune
// of the user input in the message.
func findUserInputEndIdx(userInputStartIdx int, msg []rune, userInput []rune) int {
	userInputIdx := 0
	msgIdx := userInputStartIdx
	for {
		m, u := findNextMatch(msgIdx, userInputIdx, msg, userInput)
		if m == -1 || u == -1 {
			break
		}
		msgIdx = m
		userInputIdx = u
	}
	return msgIdx
}

// RemoveUserInput removes the user input from the message.
// Goose, Aider, and Claude Code echo back the user's input to
// make it visible in the terminal. This function makes a best effort
// attempt to remove it.
// It assumes that the user input doesn't have any leading or trailing
// whitespace. Otherwise, the input may not be fully removed from the message.
// For instance, if there are any leading or trailing lines with only whitespace,
// and each line of the input in msgRaw is preceded by a character like `>`,
// these lines will not be removed.
func RemoveUserInput(msgRaw string, userInputRaw string) string {
	if userInputRaw == "" {
		return msgRaw
	}
	msg, msgLines, msgRuneLineLocations := normalizeAndGetRuneLineMapping(msgRaw)
	userInput, _, userInputLineLocations := normalizeAndGetRuneLineMapping(userInputRaw)
	userInputStartIdx := findUserInputStartIdx(msg, msgRuneLineLocations, userInput, userInputLineLocations)

	if userInputStartIdx == -1 {
		// The user input prefix was not found in the message prefix
		// Return the original message
		return msgRaw
	}

	userInputEndIdx := findUserInputEndIdx(userInputStartIdx, msg, userInput)

	// Return the original message starting with the first line
	// that doesn't contain the echoed user input.
	lastUserInputLineIdx := msgRuneLineLocations[userInputEndIdx]

	// In case of Gemini, the user input echoed back is wrapped in a rounded box, so we remove it.
	if lastUserInputLineIdx+1 < len(msgLines) && strings.Contains(msgLines[lastUserInputLineIdx+1], "╯") && strings.Contains(msgLines[lastUserInputLineIdx+1], "╰") {
		lastUserInputLineIdx += 1
	}

	return strings.Join(msgLines[lastUserInputLineIdx+1:], "\n")
}

func trimEmptyLines(message string) string {
	lines := strings.Split(message, "\n")
	firstIdx := 0
	for i := range lines {
		if strings.TrimSpace(lines[i]) != "" {
			break
		}
		firstIdx = i + 1
	}
	lines = lines[firstIdx:]
	lastIdx := len(lines) - 1
	for i := lastIdx; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			break
		}
		lastIdx = i - 1
	}
	lines = lines[:lastIdx+1]
	return strings.Join(lines, "\n")
}

type AgentType string

const (
	AgentTypeClaude AgentType = "claude"
	AgentTypeGoose  AgentType = "goose"
	AgentTypeAider  AgentType = "aider"
	AgentTypeCodex  AgentType = "codex"
	AgentTypeGemini AgentType = "gemini"
	AgentTypeCustom AgentType = "custom"
)

func formatGenericMessage(message string, userInput string) string {
	message = RemoveUserInput(message, userInput)
	message = removeMessageBox(message)
	message = trimEmptyLines(message)
	return message
}

func formatCodexMessage(message string, userInput string) string {
	message = RemoveUserInput(message, userInput)
	message = removeCodexInputBox(message)
	message = trimEmptyLines(message)
	return message
}

func FormatAgentMessage(agentType AgentType, message string, userInput string) string {
	switch agentType {
	case AgentTypeClaude:
		return formatGenericMessage(message, userInput)
	case AgentTypeGoose:
		return formatGenericMessage(message, userInput)
	case AgentTypeAider:
		return formatGenericMessage(message, userInput)
	case AgentTypeCodex:
		return formatCodexMessage(message, userInput)
	case AgentTypeGemini:
		return formatGenericMessage(message, userInput)
	case AgentTypeCustom:
		return formatGenericMessage(message, userInput)
	default:
		return message
	}
}
