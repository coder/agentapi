package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: cast_agent <file.cast>")
		os.Exit(1)
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		<-sigCh
		os.Exit(0)
	}()

	f, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open cast file: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = f.Close() }()

	stdinReader := bufio.NewReader(os.Stdin)
	fileScanner := bufio.NewScanner(f)

	// Skip the header line.
	fileScanner.Scan()

	var inputBuf strings.Builder

	for fileScanner.Scan() {
		line := fileScanner.Text()
		var event [3]json.RawMessage
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse event: %v\n", err)
			os.Exit(1)
		}

		var eventType string
		if err := json.Unmarshal(event[1], &eventType); err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse event type: %v\n", err)
			os.Exit(1)
		}

		var data string
		if err := json.Unmarshal(event[2], &data); err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse event data: %v\n", err)
			os.Exit(1)
		}

		switch eventType {
		case "o":
			fmt.Print(data)
		case "i":
			switch data {
			case "\r":
				// Consume the Enter keystroke, then reset the accumulated buffer.
				if _, err := stdinReader.ReadByte(); err != nil && err != io.EOF {
					fmt.Fprintf(os.Stderr, "failed to read stdin: %v\n", err)
					os.Exit(1)
				}
				inputBuf.Reset()
			case "\x03":
				// Ctrl-C marks end of interactive session; block indefinitely.
				// This ensures the agent displays the last reply and remains stable
				// rather than continuing to replay exit sequences.
				<-make(chan struct{})
			default:
				// Block until agentapi writes this input, then validate byte-by-byte.
				expected := []byte(data)
				for _, exp := range expected {
					b, err := stdinReader.ReadByte()
					if err != nil && err != io.EOF {
						fmt.Fprintf(os.Stderr, "failed to read stdin: %v\n", err)
						os.Exit(1)
					}
					if b != exp {
						fmt.Fprintf(os.Stderr, "input mismatch: expected 0x%02x, got 0x%02x\n", exp, b)
						os.Exit(1)
					}
				}
				inputBuf.WriteString(data)
			}
		}
	}

	if err := fileScanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "error reading cast file: %v\n", err)
		os.Exit(1)
	}

	// Block until interrupted.
	<-make(chan struct{})
}
