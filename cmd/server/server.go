package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/xerrors"

	"github.com/coder/agentapi/lib/httpapi"
	"github.com/coder/agentapi/lib/logctx"
	"github.com/coder/agentapi/lib/msgfmt"
	"github.com/coder/agentapi/lib/termexec"
)

var (
	agentTypeVar string
	port         int
	printOpenAPI bool
)

type AgentType = msgfmt.AgentType

const (
	AgentTypeClaude AgentType = msgfmt.AgentTypeClaude
	AgentTypeGoose  AgentType = msgfmt.AgentTypeGoose
	AgentTypeAider  AgentType = msgfmt.AgentTypeAider
	AgentTypeCodex  AgentType = msgfmt.AgentTypeCodex
	AgentTypeCustom AgentType = msgfmt.AgentTypeCustom
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
	case string(AgentTypeCodex):
		agentType = AgentTypeCodex
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
	case string(AgentTypeCodex):
		agentType = AgentTypeCodex
	default:
		agentType = AgentTypeCustom
	}
	return agentType, nil
}

func runServer(ctx context.Context, logger *slog.Logger, argsToPass []string) error {
	agent := argsToPass[0]
	agentType, err := parseAgentType(agent, agentTypeVar)
	if err != nil {
		return xerrors.Errorf("failed to parse agent type: %w", err)
	}
	var process *termexec.Process
	if printOpenAPI {
		process = nil
	} else {
		process, err = httpapi.SetupProcess(ctx, agent, argsToPass[1:]...)
		if err != nil {
			return xerrors.Errorf("failed to setup process: %w", err)
		}
	}
	srv := httpapi.NewServer(ctx, agentType, process, port)
	if printOpenAPI {
		fmt.Println(srv.GetOpenAPI())
		return nil
	}
	logger.Info("Starting server on port", "port", port)
	processExitCh := make(chan error, 1)
	go func() {
		defer close(processExitCh)
		if err := process.Wait(); err != nil {
			if errors.Is(err, termexec.ErrNonZeroExitCode) {
				processExitCh <- xerrors.Errorf("========\n%s\n========\n: %w", strings.TrimSpace(process.ReadScreen()), err)
			} else {
				processExitCh <- xerrors.Errorf("failed to wait for process: %w", err)
			}
		}
		if err := srv.Stop(ctx); err != nil {
			logger.Error("Failed to stop server", "error", err)
		}
	}()
	if err := srv.Start(); err != nil && err != context.Canceled && err != http.ErrServerClosed {
		return xerrors.Errorf("failed to start server: %w", err)
	}
	select {
	case err := <-processExitCh:
		return xerrors.Errorf("agent exited with error: %w", err)
	default:
	}
	return nil
}

var ServerCmd = &cobra.Command{
	Use:   "server [agent]",
	Short: "Run the server",
	Long:  `Run the server with the specified agent (claude, goose, aider, codex)`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
		ctx := logctx.WithLogger(context.Background(), logger)
		if err := runServer(ctx, logger, cmd.Flags().Args()); err != nil {
			fmt.Fprintf(os.Stderr, "%+v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	ServerCmd.Flags().StringVarP(&agentTypeVar, "type", "t", "", "Override the agent type (one of: claude, goose, aider, custom)")
	ServerCmd.Flags().IntVarP(&port, "port", "p", 3284, "Port to run the server on")
	ServerCmd.Flags().BoolVarP(&printOpenAPI, "print-openapi", "P", false, "Print the OpenAPI schema to stdout and exit")
}
