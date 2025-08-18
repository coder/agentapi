package cmd

import (
	"fmt"
	"os"

	"github.com/coder/agentapi/cmd/attach"
	"github.com/coder/agentapi/cmd/server"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "agentapi",
	Short:   "AgentAPI CLI",
	Long:    `AgentAPI - HTTP API for Claude Code, Goose, Aider, Gemini and Codex`,
	Version: "0.4.1",
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(server.CreateServerCmd())
	rootCmd.AddCommand(attach.AttachCmd)
}
