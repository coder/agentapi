package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/spf13/cobra"

	"github.com/coder/openagent/lib/httpapi"
	"github.com/coder/openagent/lib/logctx"
)

var (
	agentTypeVar string
	port         int
)

type AgentType = httpapi.AgentType

const (
	AgentTypeClaude AgentType = httpapi.AgentTypeClaude
	AgentTypeGoose  AgentType = httpapi.AgentTypeGoose
	AgentTypeAider  AgentType = httpapi.AgentTypeAider
	AgentTypeCustom AgentType = httpapi.AgentTypeCustom
)

func parseAgentType(firstArg string, agentTypeVar string) (AgentType, error) {
	var agentType AgentType
	switch agentTypeVar {
	case string(AgentTypeClaude):
		agentType = AgentTypeClaude
	case string(AgentTypeGoose):
		agentType = AgentTypeGoose
	case string(AgentTypeAider):
		agentType = AgentTypeAider
	case string(AgentTypeCustom):
		agentType = AgentTypeCustom
	case "":
		// do nothing
	default:
		return "", fmt.Errorf("invalid agent type: %s", agentTypeVar)
	}
	if agentType != "" {
		return agentType, nil
	}

	switch firstArg {
	case string(AgentTypeClaude):
		agentType = AgentTypeClaude
	case string(AgentTypeGoose):
		agentType = AgentTypeGoose
	case string(AgentTypeAider):
		agentType = AgentTypeAider
	default:
		agentType = AgentTypeCustom
	}
	return agentType, nil
}

var ServerCmd = &cobra.Command{
	Use:   "server [agent]",
	Short: "Run the server",
	Long:  `Run the server with the specified agent (claude, goose, aider, custom)`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
		ctx := logctx.WithLogger(context.Background(), logger)
		argsToPass := cmd.Flags().Args()
		agent := argsToPass[0]
		agentType, err := parseAgentType(agent, agentTypeVar)
		if err != nil {
			logger.Error("Failed to parse agent type", "error", err)
			os.Exit(1)
		}
		process, cleanup, err := httpapi.SetupProcess(ctx, agent, argsToPass[1:]...)
		if err != nil {
			logger.Error("Failed to setup process", "error", err)
			os.Exit(1)
		}
		defer cleanup()
		srv := httpapi.NewServer(ctx, agentType, process, port)
		logger.Info("Starting server on port", "port", port)
		go func() {
			if err := process.Wait(); err != nil {
				logger.Error("Process exited with error", "error", err)
			}
			if err := srv.Stop(ctx); err != nil {
				logger.Error("Failed to stop server", "error", err)
			}
		}()
		if err := srv.Start(); err != nil && err != context.Canceled && err != http.ErrServerClosed {
			logger.Error("Failed to start server", "error", err)
			os.Exit(1)
		}
	},
}

func init() {
	ServerCmd.Flags().StringVarP(&agentTypeVar, "type", "t", "", "Override the agent type (one of: claude, goose, aider, custom)")
	ServerCmd.Flags().IntVarP(&port, "port", "p", 3284, "Port to run the server on")
}
