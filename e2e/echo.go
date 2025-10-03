package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"time"

	"github.com/acarl005/stripansi"
	st "github.com/coder/agentapi/lib/screentracker"
)

type ScriptEntry struct {
	ExpectMessage   string `json:"expectMessage"`
	ThinkDurationMS int64  `json:"thinkDurationMS"`
	ResponseMessage string `json:"responseMessage"`
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: echo <script.json>")
		os.Exit(1)
	}

	runEchoAgent(os.Args[1])
}

func loadScript(scriptPath string) ([]ScriptEntry, error) {
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read script file: %w", err)
	}

	var script []ScriptEntry
	if err := json.Unmarshal(data, &script); err != nil {
		return nil, fmt.Errorf("failed to parse script JSON: %w", err)
	}

	return script, nil
}

func runEchoAgent(scriptPath string) {
	script, err := loadScript(scriptPath)
	if err != nil {
		fmt.Printf("Error loading script: %v\n", err)
		os.Exit(1)
	}

	if len(script) == 0 {
		fmt.Println("Script is empty")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		for {
			select {
			case <-sigCh:
				cancel()
				fmt.Println("Exiting...")
				os.Exit(0)
			case <-ctx.Done():
				return
			}
		}
	}()

	var messages []st.ConversationMessage
	redrawTerminal(messages, false)

	scriptIndex := 0
	scanner := bufio.NewScanner(os.Stdin)

	for scriptIndex < len(script) {
		entry := script[scriptIndex]
		expectedMsg := strings.TrimSpace(entry.ExpectMessage)

		// Handle initial/follow-up messages (empty ExpectMessage)
		if expectedMsg == "" {
			// Show thinking state if there's a delay
			if entry.ThinkDurationMS > 0 {
				redrawTerminal(messages, true)
				spinnerCtx, spinnerCancel := context.WithCancel(ctx)
				go runSpinner(spinnerCtx)
				time.Sleep(time.Duration(entry.ThinkDurationMS) * time.Millisecond)
				if spinnerCancel != nil {
					spinnerCancel()
				}
			}

			messages = append(messages, st.ConversationMessage{
				Role:    st.ConversationRoleAgent,
				Message: entry.ResponseMessage,
				Time:    time.Now(),
			})
			redrawTerminal(messages, false)
			scriptIndex++
			continue
		}

		// Wait for user input for non-initial messages
		if !scanner.Scan() {
			break
		}

		input := scanner.Text()
		input = cleanTerminalInput(input)
		if input == "" {
			continue
		}

		if input != expectedMsg {
			fmt.Printf("Error: Expected message '%s' but received '%s'\n", expectedMsg, input)
			os.Exit(1)
		}

		messages = append(messages, st.ConversationMessage{
			Role:    st.ConversationRoleUser,
			Message: entry.ExpectMessage,
			Time:    time.Now(),
		})
		redrawTerminal(messages, false)

		// Show thinking state if there's a delay
		if entry.ThinkDurationMS > 0 {
			redrawTerminal(messages, true)
			spinnerCtx, spinnerCancel := context.WithCancel(ctx)
			go runSpinner(spinnerCtx)
			time.Sleep(time.Duration(entry.ThinkDurationMS) * time.Millisecond)
			if spinnerCancel != nil {
				spinnerCancel()
			}
		}

		messages = append(messages, st.ConversationMessage{
			Role:    st.ConversationRoleAgent,
			Message: entry.ResponseMessage,
			Time:    time.Now(),
		})
		redrawTerminal(messages, false)
		scriptIndex++
	}

	// Now just do nothing.
	<-make(chan struct{})
}

func redrawTerminal(messages []st.ConversationMessage, thinking bool) {
	fmt.Print("\033[2J\033[H") // Clear screen and move cursor to home

	// Show conversation history
	for _, msg := range messages {
		if msg.Role == st.ConversationRoleUser {
			fmt.Printf("> %s\n", msg.Message)
		} else {
			fmt.Printf("%s\n", msg.Message)
		}
	}

	if thinking {
		fmt.Print("Thinking... ")
	} else {
		fmt.Print("> ")
	}
}

func cleanTerminalInput(input string) string {
	// Strip ANSI escape sequences
	input = stripansi.Strip(input)

	// Remove bracketed paste mode sequences (^[[200~ and ^[[201~)
	bracketedPasteRe := regexp.MustCompile(`\x1b\[\d+~`)
	input = bracketedPasteRe.ReplaceAllString(input, "")

	// Remove backspace sequences (character followed by ^H)
	backspaceRe := regexp.MustCompile(`.\x08`)
	input = backspaceRe.ReplaceAllString(input, "")

	// Remove other common control characters
	input = strings.ReplaceAll(input, "\x08", "") // backspace
	input = strings.ReplaceAll(input, "\x7f", "") // delete
	input = strings.ReplaceAll(input, "\x1b", "") // escape (if any remain)

	return strings.TrimSpace(input)
}

func runSpinner(ctx context.Context) {
	spinnerChars := []string{"|", "/", "-", "\\"}
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	i := 0

	for {
		select {
		case <-ticker.C:
			fmt.Printf("\rThinking %s", spinnerChars[i%len(spinnerChars)])
			i++
		case <-ctx.Done():
			// Clear spinner on cancellation
			fmt.Print("\r" + strings.Repeat(" ", 20) + "\r")
			return
		}
	}
}
