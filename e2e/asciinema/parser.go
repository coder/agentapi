// Package asciinema provides utilities for parsing asciinema recordings
// and converting them to echo agent scripts.
package asciinema

import (
	"encoding/json"
	"fmt"
	"os"
)

// ScriptEntry represents an echo agent script entry
type ScriptEntry struct {
	ExpectMessage   string `json:"expectMessage"`
	ThinkDurationMS int64  `json:"thinkDurationMS"`
	ResponseMessage string `json:"responseMessage"`
}

// Recording represents an asciinema v2 recording file format
type Recording struct {
	Version float64 `json:"version"`
	Width   int     `json:"width"`
	Height int     `json:"height"`
	Lines  []Event  `json:"lines"`
}

// Event represents a single asciinema event
type Event struct {
	Time float64       `json:"time"`
	Type string        `json:"type"`
	Data interface{}  `json:"data"`
}

// ParseRecording parses an asciinema recording file
func ParseRecording(path string) (*Recording, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open recording: %w", err)
	}
	defer f.Close()

	var rec Recording
	if err := json.NewDecoder(f).Decode(&rec); err != nil {
		return nil, fmt.Errorf("decode recording: %w", err)
	}

	return &rec, nil
}

// ToEchoScript converts an asciinema recording to echo agent script format
func ToEchoScript(rec *Recording) ([]ScriptEntry, error) {
	var entries []ScriptEntry
	var lastTime float64

	for _, event := range rec.Lines {
		// Only process output events
		if event.Type != "o" {
			continue
		}

		// Get the output content
		var frames []interface{}
		switch d := event.Data.(type) {
		case []interface{}:
			frames = d
		default:
			continue
		}

		// Calculate think duration based on time delta
		thinkDuration := int64((event.Time - lastTime) * 1000)
		if thinkDuration < 0 {
			thinkDuration = 0
		}

		// Combine frames into response
		var output string
		for _, frame := range frames {
			switch f := frame.(type) {
			case string:
				output += f + "\n"
			case []interface{}:
				for _, line := range f {
					if s, ok := line.(string); ok {
						output += s + "\n"
					}
				}
			}
		}

		if output != "" {
			entries = append(entries, ScriptEntry{
				ThinkDurationMS: thinkDuration,
				ResponseMessage: output,
			})
		}

		lastTime = event.Time
	}

	return entries, nil
}

// WriteEchoScript writes the converted script to a JSON file
func WriteEchoScript(entries []ScriptEntry, path string) error {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// LoadAndConvert loads an asciinema recording and converts to echo script
func LoadAndConvert(asciinemaPath, echoPath string) error {
	rec, err := ParseRecording(asciinemaPath)
	if err != nil {
		return err
	}

	entries, err := ToEchoScript(rec)
	if err != nil {
		return err
	}

	return WriteEchoScript(entries, echoPath)
}
