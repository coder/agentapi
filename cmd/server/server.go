package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/xerrors"

	"github.com/coder/agentapi/lib/httpapi"
	"github.com/coder/agentapi/lib/logctx"
	"github.com/coder/agentapi/lib/msgfmt"
	"github.com/coder/agentapi/lib/termexec"
)

type AgentType = msgfmt.AgentType

const (
	AgentTypeClaude AgentType = msgfmt.AgentTypeClaude
	AgentTypeGoose  AgentType = msgfmt.AgentTypeGoose
	AgentTypeAider  AgentType = msgfmt.AgentTypeAider
	AgentTypeCodex  AgentType = msgfmt.AgentTypeCodex
	AgentTypeGemini AgentType = msgfmt.AgentTypeGemini
	AgentTypeCustom AgentType = msgfmt.AgentTypeCustom
)

// exhaustiveness of this map is checked by the exhaustive linter
var agentTypeMap = map[AgentType]bool{
	AgentTypeClaude: true,
	AgentTypeGoose:  true,
	AgentTypeAider:  true,
	AgentTypeCodex:  true,
	AgentTypeGemini: true,
	AgentTypeCustom: true,
}

func parseAgentType(firstArg string, agentTypeVar string) (AgentType, error) {
	// if the agent type is provided, use it
	castedAgentType := AgentType(agentTypeVar)
	if _, ok := agentTypeMap[castedAgentType]; ok {
		return castedAgentType, nil
	}
	if agentTypeVar != "" {
		return AgentTypeCustom, fmt.Errorf("invalid agent type: %s", agentTypeVar)
	}
	// if the agent type is not provided, guess it from the first argument
	castedFirstArg := AgentType(firstArg)
	if _, ok := agentTypeMap[castedFirstArg]; ok {
		return castedFirstArg, nil
	}
	return AgentTypeCustom, nil
}

func runServer(ctx context.Context, logger *slog.Logger, argsToPass []string) error {
	agent := argsToPass[0]
	agentTypeValue := viper.GetString(FlagType)
	agentType, err := parseAgentType(agent, agentTypeValue)
	if err != nil {
		return xerrors.Errorf("failed to parse agent type: %w", err)
	}

	termWidth := viper.GetUint16(FlagTermWidth)
	termHeight := viper.GetUint16(FlagTermHeight)

	if termWidth < 10 {
		return xerrors.Errorf("term width must be at least 10")
	}
	if termHeight < 10 {
		return xerrors.Errorf("term height must be at least 10")
	} else if agentType == AgentTypeCodex && termHeight > 930 {
		logger.Warn(fmt.Sprintf("Term height is set to %d which may cause issues with Codex. Setting it to 930 instead.", termHeight))
		termHeight = 930 // codex has a bug where the TUI distorts the screen if the height is too large, see: https://github.com/openai/codex/issues/1608
	}

	printOpenAPI := viper.GetBool(FlagPrintOpenAPI)
	var process *termexec.Process
	if printOpenAPI {
		process = nil
	} else {
		process, err = httpapi.SetupProcess(ctx, httpapi.SetupProcessConfig{
			Program:        agent,
			ProgramArgs:    argsToPass[1:],
			TerminalWidth:  termWidth,
			TerminalHeight: termHeight,
		})
		if err != nil {
			return xerrors.Errorf("failed to setup process: %w", err)
		}
	}
	port := viper.GetInt(FlagPort)
	srv := httpapi.NewServer(ctx, httpapi.ServerConfig{
		AgentType:    agentType,
		Process:      process,
		Port:         port,
		ChatBasePath: viper.GetString(FlagChatBasePath),
	})
	if printOpenAPI {
		fmt.Println(srv.GetOpenAPI())
		return nil
	}
	srv.StartSnapshotLoop(ctx)
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

var agentNames = (func() []string {
	names := make([]string, 0, len(agentTypeMap))
	for agentType := range agentTypeMap {
		names = append(names, string(agentType))
	}
	sort.Strings(names)
	return names
})()

type flagSpec struct {
	name         string
	shorthand    string
	defaultValue any
	usage        string
	flagType     string
}

const (
	FlagType         = "type"
	FlagPort         = "port"
	FlagPrintOpenAPI = "print-openapi"
	FlagChatBasePath = "chat-base-path"
	FlagTermWidth    = "term-width"
	FlagTermHeight   = "term-height"
)

func CreateServerCmd() *cobra.Command {
	serverCmd := &cobra.Command{
		Use:   "server [agent]",
		Short: "Run the server",
		Long:  fmt.Sprintf("Run the server with the specified agent (one of: %s)", strings.Join(agentNames, ", ")),
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

	flagSpecs := []flagSpec{
		{FlagType, "t", "", fmt.Sprintf("Override the agent type (one of: %s, custom)", strings.Join(agentNames, ", ")), "string"},
		{FlagPort, "p", 3284, "Port to run the server on", "int"},
		{FlagPrintOpenAPI, "P", false, "Print the OpenAPI schema to stdout and exit", "bool"},
		{FlagChatBasePath, "c", "/chat", "Base path for assets and routes used in the static files of the chat interface", "string"},
		{FlagTermWidth, "W", uint16(80), "Width of the emulated terminal", "uint16"},
		{FlagTermHeight, "H", uint16(1000), "Height of the emulated terminal", "uint16"},
	}

	for _, spec := range flagSpecs {
		switch spec.flagType {
		case "string":
			serverCmd.Flags().StringP(spec.name, spec.shorthand, spec.defaultValue.(string), spec.usage)
		case "int":
			serverCmd.Flags().IntP(spec.name, spec.shorthand, spec.defaultValue.(int), spec.usage)
		case "bool":
			serverCmd.Flags().BoolP(spec.name, spec.shorthand, spec.defaultValue.(bool), spec.usage)
		case "uint16":
			serverCmd.Flags().Uint16P(spec.name, spec.shorthand, spec.defaultValue.(uint16), spec.usage)
		default:
			panic(fmt.Sprintf("unknown flag type: %s", spec.flagType))
		}
		if err := viper.BindPFlag(spec.name, serverCmd.Flags().Lookup(spec.name)); err != nil {
			panic(fmt.Sprintf("failed to bind flag %s: %v", spec.name, err))
		}
	}

	viper.SetEnvPrefix("AGENTAPI")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	return serverCmd
}
