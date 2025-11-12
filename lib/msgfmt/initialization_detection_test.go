package msgfmt

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsAgentReadyForInitialPrompt(t *testing.T) {
	dir := "testdata/initialization"
	agentTypes := []AgentType{AgentTypeClaude, AgentTypeGoose, AgentTypeAider, AgentTypeGemini, AgentTypeCopilot, AgentTypeAmp, AgentTypeCodex, AgentTypeCursor, AgentTypeAuggie, AgentTypeAmazonQ, AgentTypeOpencode, AgentTypeCustom}
	for _, agentType := range agentTypes {
		t.Run(string(agentType), func(t *testing.T) {
			t.Run("ready", func(t *testing.T) {
				cases, err := testdataDir.ReadDir(path.Join(dir, string(agentType), "ready"))
				if err != nil {
					t.Skipf("failed to read ready cases for agent type %s: %s", agentType, err)
				}
				if len(cases) == 0 {
					t.Skipf("no ready cases found for agent type %s", agentType)
				}
				for _, c := range cases {
					if c.IsDir() {
						continue
					}
					t.Run(c.Name(), func(t *testing.T) {
						msg, err := testdataDir.ReadFile(path.Join(dir, string(agentType), "ready", c.Name()))
						assert.NoError(t, err)
						assert.True(t, IsAgentReadyForInitialPrompt(agentType, string(msg)), "Expected agent to be ready for message:\n%s", string(msg))
					})
				}
			})

			t.Run("not_ready", func(t *testing.T) {
				cases, err := testdataDir.ReadDir(path.Join(dir, string(agentType), "not_ready"))
				if err != nil {
					t.Skipf("failed to read not_ready cases for agent type %s: %s", agentType, err)
				}
				if len(cases) == 0 {
					t.Skipf("no not_ready cases found for agent type %s", agentType)
				}
				for _, c := range cases {
					if c.IsDir() {
						continue
					}
					t.Run(c.Name(), func(t *testing.T) {
						msg, err := testdataDir.ReadFile(path.Join(dir, string(agentType), "not_ready", c.Name()))
						assert.NoError(t, err)
						assert.False(t, IsAgentReadyForInitialPrompt(agentType, string(msg)), "Expected agent to not be ready for message:\n%s", string(msg))
					})
				}
			})
		})
	}
}
