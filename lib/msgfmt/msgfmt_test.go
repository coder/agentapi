package msgfmt

import (
	"embed"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeAndGetRuneLineMapping(t *testing.T) {
	msg := "Hello, World!\n \nTest.\n"
	normalizedMsg, lines, runeLineLocations := normalizeAndGetRuneLineMapping(msg)
	assert.Equal(t, normalizedMsg, []rune("Hello,World!Test."))
	assert.Equal(t, []string{"Hello, World!", " ", "Test.", ""}, lines)
	assert.Equal(t, []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 2, 2, 2, 2}, runeLineLocations)

	nonAscii := "😄😄😄😄😄🎉🎉🎉🎉🎉🌮"
	normalizedNonAscii, lines, runeLineLocations := normalizeAndGetRuneLineMapping(nonAscii)
	assert.Equal(t, len([]rune(nonAscii)), len(runeLineLocations))
	assert.Equal(t, normalizedNonAscii, []rune("😄😄😄😄😄🎉🎉🎉🎉🎉🌮"))
	assert.Equal(t, []string{"😄😄😄😄😄🎉🎉🎉🎉🎉🌮"}, lines)
	assert.Equal(t, []int{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, runeLineLocations)

	nonAscii2 := "╭───"
	normalizedNonAscii2, lines, runeLineLocations := normalizeAndGetRuneLineMapping(nonAscii2)
	assert.Equal(t, len([]rune(nonAscii2)), len(runeLineLocations))
	assert.Equal(t, normalizedNonAscii2, []rune("╭───"))
	assert.Equal(t, []string{"╭───"}, lines)
	assert.Equal(t, []int{0, 0, 0, 0}, runeLineLocations)
}

func TestFindUserInputStartIdx(t *testing.T) {
	t.Run("single-line-msg", func(t *testing.T) {
		prefix := "Hello, World!"
		userInput := "How are you doing?"
		suffix := "Good Good"
		msg := prefix + userInput + suffix
		userInputStartIdx := findUserInputStartIdx([]rune(msg), make([]int, len(msg)), []rune(userInput), make([]int, len(userInput)))
		assert.Equal(t, len(prefix), userInputStartIdx)
	})
	t.Run("truncated-user-input", func(t *testing.T) {
		prefix := "Hello, World!"
		userInput := "How are you doing?"
		suffix := "Good Good"
		// Only the first 6 runes of the user input are considered
		msg := prefix + "How ar" + suffix
		userInputStartIdx := findUserInputStartIdx([]rune(msg), make([]int, len(msg)), []rune(userInput), make([]int, len(userInput)))
		assert.Equal(t, len(prefix), userInputStartIdx)
	})
	t.Run("short-message", func(t *testing.T) {
		prefix := "hey"
		userInput := "ho"
		msg := prefix + userInput
		userInputStartIdx := findUserInputStartIdx([]rune(msg), make([]int, len(msg)), []rune(userInput), make([]int, len(userInput)))
		assert.Equal(t, len(prefix), userInputStartIdx)
	})
	t.Run("empty-message", func(t *testing.T) {
		userInput := "How are you doing?"
		msg := ""
		userInputStartIdx := findUserInputStartIdx([]rune(msg), make([]int, len(msg)), []rune(userInput), make([]int, len(userInput)))
		assert.Equal(t, -1, userInputStartIdx)
	})
	t.Run("multi-line-msg", func(t *testing.T) {
		t.Run("long-first-line", func(t *testing.T) {
			firstLineBuilder := strings.Builder{}
			for range 100 {
				firstLineBuilder.WriteRune('-')
			}
			firstLine := firstLineBuilder.String()
			prefix := firstLine
			userInput := "How are you doing?"
			suffix := "Good Good"
			msg := prefix + userInput + suffix
			msgRuneLineMapping := make([]int, len(msg))
			for i := 100; i < len(msg); i++ {
				msgRuneLineMapping[i] = 1
			}
			userInputStartIdx := findUserInputStartIdx([]rune(msg), msgRuneLineMapping, []rune(userInput), make([]int, len(userInput)))
			assert.Equal(t, len(prefix), userInputStartIdx)
		})
		t.Run("short-leading-lines", func(t *testing.T) {
			prefixLines := []string{"a", "b", "c", "d", "e", "f"}
			prefix := strings.Join(prefixLines, "")
			userInput := "How are you doing?"
			suffix := "Good Good"
			msg := prefix + userInput + suffix
			msgRuneLineMapping := make([]int, len(msg))
			for i := range len(prefix) {
				msgRuneLineMapping[i] = i
			}
			for i := len(prefix); i < len(prefix)+len(userInput); i++ {
				msgRuneLineMapping[i] = len(prefixLines)
			}
			for i := len(prefix) + len(userInput); i < len(msg); i++ {
				msgRuneLineMapping[i] = len(prefixLines) + 1
			}
			userInputStartIdx := findUserInputStartIdx([]rune(msg), msgRuneLineMapping, []rune(userInput), make([]int, len(userInput)))
			assert.Equal(t, len(prefix), userInputStartIdx)
		})
	})
	t.Run("multi-line-user-input", func(t *testing.T) {
		t.Run("short-first-line", func(t *testing.T) {
			userInputLines := []string{"abc", "def"}
			userInput := strings.Join(userInputLines, "")
			prefix := "Hello, World!"
			suffix := "Good Good"
			// only the first line of input is considered
			msg := prefix + "abcxxx" + suffix
			userInputRuneLineMapping := []int{0, 0, 0, 1, 1, 1}
			userInputStartIdx := findUserInputStartIdx([]rune(msg), make([]int, len(msg)), []rune(userInput), userInputRuneLineMapping)
			assert.Equal(t, len(prefix), userInputStartIdx)
		})
	})
}

func TestFindUserInputEndIdx(t *testing.T) {
	t.Run("accurate-msg", func(t *testing.T) {
		prefix := "Hello, World!"
		userInput := "How are you doing?"
		suffix := "Good Good"
		msg := prefix + userInput + suffix
		userInputEndIdx := findUserInputEndIdx(len(prefix), []rune(msg), []rune(userInput))
		assert.Equal(t, suffix, msg[userInputEndIdx+1:])
	})
	t.Run("truncated-input", func(t *testing.T) {
		prefix := "Hello, World!"
		userInput := "How are you doing?"
		suffix := "...------"
		truncatedUserInput := userInput[:7]
		msg := prefix + truncatedUserInput + suffix
		userInputEndIdx := findUserInputEndIdx(len(prefix), []rune(msg), []rune(userInput))
		assert.Equal(t, len(prefix)+len(truncatedUserInput)-1, userInputEndIdx)
		assert.Equal(t, suffix, msg[userInputEndIdx+1:])
	})
	t.Run("truncated-input-suffix-matching-chars", func(t *testing.T) {
		t.Skip("this test doesn't work by design. TODO: improve the algorithm to handle this case")
		// The way the algorithm works is that if it can't find a rune-by-rune match,
		// it will look ahead in the message for the next matching rune up to 5 runes.
		// In this case, there's a matching rune (whitespace) in the non-user-supplied suffix,
		// which makes the algorithm choose the wrong end index. We could improve this
		// by looking for a couple of consecutive matching runes, but we have to also
		// handle the case where these matching runes could be broken up by UI elements.
		// I think we could store a running dictionary of non-matching runes and ignore
		// them when looking for the next matching rune. The idea being that these
		// non-matching runes are likely to be part of the UI elements.
		prefix := "Hello, World!"
		userInput := "How are you doing?"
		suffix := "Good Good"
		truncatedUserInput := userInput[:7]
		msg := prefix + truncatedUserInput + suffix
		userInputEndIdx := findUserInputEndIdx(len(prefix), []rune(msg), []rune(userInput))
		assert.Equal(t, len(prefix)+len(truncatedUserInput)-1, userInputEndIdx)
		assert.Equal(t, suffix, msg[userInputEndIdx+1:])
	})
	t.Run("broken-up-input", func(t *testing.T) {
		// This test simulates UI elements breaking up the user input.
		prefix := "Hello, World!"
		userInputParts := []string{"How ", "are ", "you ", "doing?"}
		userInput := strings.Join(userInputParts, "*|*")
		suffix := "...------"
		msg := prefix + userInput + suffix
		userInputEndIdx := findUserInputEndIdx(len(prefix), []rune(msg), []rune(userInput))
		assert.Equal(t, suffix, msg[userInputEndIdx+1:])
	})
	t.Run("no-user-input-in-message", func(t *testing.T) {
		msg := "Hello,World!"
		userInput := "/init"
		userInputEndIdx := findUserInputEndIdx(len(msg), []rune(msg), []rune(userInput))
		// returns the same index as userInputStartIdx
		assert.Equal(t, len(msg), userInputEndIdx)
	})
}

//go:embed testdata
var testdataDir embed.FS

func TestRemoveUserInput(t *testing.T) {
	dir := "testdata/remove-user-input"
	cases, err := testdataDir.ReadDir(dir)
	assert.NoError(t, err)
	for _, c := range cases {
		t.Run(c.Name(), func(t *testing.T) {
			msg, err := testdataDir.ReadFile(path.Join(dir, c.Name(), "msg.txt"))
			assert.NoError(t, err)
			userInput, err := testdataDir.ReadFile(path.Join(dir, c.Name(), "user.txt"))
			assert.NoError(t, err)
			expected, err := testdataDir.ReadFile(path.Join(dir, c.Name(), "expected.txt"))
			assert.NoError(t, err)
			assert.Equal(t, string(expected), RemoveUserInput(string(msg), string(userInput)))
		})
	}
}

func TestTrimEmptyLines(t *testing.T) {
	cases := []struct {
		input    []string
		expected []string
	}{
		{
			input:    []string{"", "", "Hello, World!", "Hello, World!"},
			expected: []string{"Hello, World!", "Hello, World!"},
		},
		{
			input:    []string{""},
			expected: []string{""},
		},
		{
			input:    []string{"", "Hello, World!", "", "1", ""},
			expected: []string{"Hello, World!", "", "1"},
		},
	}
	for _, c := range cases {
		assert.Equal(t, strings.Join(c.expected, "\n"), trimEmptyLines(strings.Join(c.input, "\n")))
	}
}

func TestFormatAgentMessage(t *testing.T) {
	dir := "testdata/format"
	agentTypes := []AgentType{AgentTypeClaude, AgentTypeGoose, AgentTypeAider, AgentTypeGemini, AgentTypeCodex, AgentTypeCustom}
	for _, agentType := range agentTypes {
		t.Run(string(agentType), func(t *testing.T) {
			cases, err := testdataDir.ReadDir(path.Join(dir, string(agentType)))
			if err != nil {
				t.Skipf("failed to read cases for agent type %s: %s", agentType, err)
			}
			for _, c := range cases {
				t.Run(c.Name(), func(t *testing.T) {
					msg, err := testdataDir.ReadFile(path.Join(dir, string(agentType), c.Name(), "msg.txt"))
					assert.NoError(t, err)
					userInput, err := testdataDir.ReadFile(path.Join(dir, string(agentType), c.Name(), "user.txt"))
					assert.NoError(t, err)
					expected, err := testdataDir.ReadFile(path.Join(dir, string(agentType), c.Name(), "expected.txt"))
					assert.NoError(t, err)
					assert.Equal(t, string(expected), FormatAgentMessage(agentType, string(msg), string(userInput)))
				})
			}
		})
	}
}
