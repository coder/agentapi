package server

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAgentType(t *testing.T) {
	tests := []struct {
		firstArg     string
		agentTypeVar string
		want         AgentType
	}{
		{
			firstArg:     "",
			agentTypeVar: "",
			want:         AgentTypeCustom,
		},
		{
			firstArg:     "claude",
			agentTypeVar: "",
			want:         AgentTypeClaude,
		},
		{
			firstArg:     "gemini",
			agentTypeVar: "",
			want:         AgentTypeGemini,
		},
		{
			firstArg:     "goose",
			agentTypeVar: "",
			want:         AgentTypeGoose,
		},
		{
			firstArg:     "aider",
			agentTypeVar: "",
			want:         AgentTypeAider,
		},
		{
			firstArg:     "whatever",
			agentTypeVar: "",
			want:         AgentTypeCustom,
		},
		{
			firstArg:     "claude",
			agentTypeVar: "goose",
			want:         AgentTypeGoose,
		},
		{
			firstArg:     "goose",
			agentTypeVar: "claude",
			want:         AgentTypeClaude,
		},
		{
			firstArg:     "claude",
			agentTypeVar: "gemini",
			want:         AgentTypeGemini,
		},
		{
			firstArg:     "aider",
			agentTypeVar: "claude",
			want:         AgentTypeClaude,
		},
		{
			firstArg:     "aider",
			agentTypeVar: "custom",
			want:         AgentTypeCustom,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%s-%s-%s", test.firstArg, test.agentTypeVar, test.want), func(t *testing.T) {
			got, err := parseAgentType(test.firstArg, test.agentTypeVar)
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}

	t.Run("invalid agent type", func(t *testing.T) {
		_, err := parseAgentType("claude", "invalid")
		require.Error(t, err)
	})
}
