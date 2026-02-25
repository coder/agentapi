package asciinema

import (
	"os"
	"testing"
)

func TestParseRecording(t *testing.T) {
	testRec := `{
		"version": 2,
		"width": 80,
		"height": 24,
		"lines": [
			[0.0, "o", [["Hello world"]],
			[0.5, "o", [["Thinking..."]],
			[1.0, "o", [["Response here"]]
		]
	}`

	tmp, err := os.CreateTemp("", "test-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(testRec); err != nil {
		t.Fatal(err)
	}
	tmp.Close()

	rec, err := ParseRecording(tmp.Name())
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if rec.Version != 2 {
		t.Errorf("expected version 2, got %v", rec.Version)
	}

	if len(rec.Lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(rec.Lines))
	}
}

func TestToEchoScript(t *testing.T) {
	rec := &Recording{
		Version: 2,
		Lines: []Event{
			{Time: 0.0, Type: "o", Data: []interface{}{[]interface{}{"Hello"}}},
			{Time: 0.5, Type: "o", Data: []interface{}{[]interface{}{"World"}}},
			{Time: 1.0, Type: "i", Data: []interface{}{"x"}},
		},
	}

	entries, err := ToEchoScript(rec)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}
