package cast_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadScript(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "script.txt")
		err := os.WriteFile(f, []byte("agent\tHello!\nuser\tHi there\nagent\tHow can I help?\n"), 0o600)
		require.NoError(t, err)

		entries := loadScript(t, f)
		require.Len(t, entries, 3)
		require.Equal(t, scriptEntry{Role: "agent", Message: "Hello!"}, entries[0])
		require.Equal(t, scriptEntry{Role: "user", Message: "Hi there"}, entries[1])
		require.Equal(t, scriptEntry{Role: "agent", Message: "How can I help?"}, entries[2])
	})

	t.Run("empty file", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "empty.txt")
		err := os.WriteFile(f, []byte(""), 0o600)
		require.NoError(t, err)

		entries := loadScript(t, f)
		require.Empty(t, entries)
	})

	t.Run("blank lines are skipped", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "blanks.txt")
		err := os.WriteFile(f, []byte("\nagent\tGreeting\n\nuser\tQuestion\n\n"), 0o600)
		require.NoError(t, err)

		entries := loadScript(t, f)
		require.Len(t, entries, 2)
		require.Equal(t, "agent", entries[0].Role)
		require.Equal(t, "user", entries[1].Role)
	})

	t.Run("tab in message body is preserved", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "tabmsg.txt")
		// The message itself contains a tab character after the first one.
		err := os.WriteFile(f, []byte("user\thello\tworld\n"), 0o600)
		require.NoError(t, err)

		entries := loadScript(t, f)
		require.Len(t, entries, 1)
		require.Equal(t, "user", entries[0].Role)
		require.Equal(t, "hello\tworld", entries[0].Message)
	})
}
