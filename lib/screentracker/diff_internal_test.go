package screentracker

import (
	"embed"
	"path"
	"testing"

	"github.com/coder/agentapi/lib/msgfmt"
	"github.com/stretchr/testify/assert"
)

//go:embed testdata
var testdataDir embed.FS

func TestScreenDiff(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		assert.Equal(t, "", screenDiff("123456", "123456", msgfmt.AgentTypeCustom))
		assert.Equal(t, "1234567", screenDiff("123456", "1234567", msgfmt.AgentTypeCustom))
		assert.Equal(t, "42", screenDiff("123", "123\n  \n \n \n42", msgfmt.AgentTypeCustom))
		assert.Equal(t, "12342", screenDiff("123", "12342\n   \n \n \n", msgfmt.AgentTypeCustom))
		assert.Equal(t, "42", screenDiff("123", "123\n  \n \n \n42\n   \n \n \n", msgfmt.AgentTypeCustom))
		assert.Equal(t, "42", screenDiff("89", "42", msgfmt.AgentTypeCustom))
	})

	dir := "testdata/diff"
	cases, err := testdataDir.ReadDir(dir)
	assert.NoError(t, err)
	for _, c := range cases {
		t.Run(c.Name(), func(t *testing.T) {
			before, err := testdataDir.ReadFile(path.Join(dir, c.Name(), "before.txt"))
			assert.NoError(t, err)
			after, err := testdataDir.ReadFile(path.Join(dir, c.Name(), "after.txt"))
			assert.NoError(t, err)
			expected, err := testdataDir.ReadFile(path.Join(dir, c.Name(), "expected.txt"))
			assert.NoError(t, err)
			assert.Equal(t, string(expected), screenDiff(string(before), string(after), msgfmt.AgentTypeCustom))
		})
	}
}
