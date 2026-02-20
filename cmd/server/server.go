package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/coder/agentapi/lib/screentracker"
	"github.com/mattn/go-isatty"
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
	AgentTypeClaude   AgentType = msgfmt.AgentTypeClaude
	AgentTypeGoose    AgentType = msgfmt.AgentTypeGoose
	AgentTypeAider    AgentType = msgfmt.AgentTypeAider
	AgentTypeCodex    AgentType = msgfmt.AgentTypeCodex
	AgentTypeGemini   AgentType = msgfmt.AgentTypeGemini
	AgentTypeCopilot  AgentType = msgfmt.AgentTypeCopilot
	AgentTypeAmp      AgentType = msgfmt.AgentTypeAmp
	AgentTypeCursor   AgentType = msgfmt.AgentTypeCursor
	AgentTypeAuggie   AgentType = msgfmt.AgentTypeAuggie
	AgentTypeAmazonQ  AgentType = msgfmt.AgentTypeAmazonQ
	AgentTypeOpencode AgentType = msgfmt.AgentTypeOpencode
	AgentTypeCustom   AgentType = msgfmt.AgentTypeCustom
)

// agentTypeAliases contains the mapping of possible input agent type strings to their canonical AgentType values
var agentTypeAliases = map[string]AgentType{
	"claude":       AgentTypeClaude,
	"goose":        AgentTypeGoose,
	"aider":        AgentTypeAider,
	"codex":        AgentTypeCodex,
	"gemini":       AgentTypeGemini,
	"copilot":      AgentTypeCopilot,
	"amp":          AgentTypeAmp,
	"auggie":       AgentTypeAuggie,
	"cursor":       AgentTypeCursor,
	"cursor-agent": AgentTypeCursor,
	"q":            AgentTypeAmazonQ,
	"amazonq":      AgentTypeAmazonQ,
	"opencode":     AgentTypeOpencode,
	"custom":       AgentTypeCustom,
}

func parseAgentType(firstArg string, agentTypeVar string) (AgentType, error) {
	// if the agent type is provided, use it
	if castedAgentType, ok := agentTypeAliases[agentTypeVar]; ok {
		return castedAgentType, nil
	}
	if agentTypeVar != "" {
		return AgentTypeCustom, fmt.Errorf("invalid agent type: %s", agentTypeVar)
	}
	// if the agent type is not provided, guess it from the first argument
	if castedFirstArg, ok := agentTypeAliases[firstArg]; ok {
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
	}

	// Read stdin if it's piped, to be used as initial prompt
	initialPrompt := viper.GetString(FlagInitialPrompt)
	if initialPrompt == "" {
		if !isatty.IsTerminal(os.Stdin.Fd()) {
			if stdinData, err := io.ReadAll(os.Stdin); err != nil {
				return xerrors.Errorf("failed to read stdin: %w", err)
			} else if len(stdinData) > 0 {
				initialPrompt = string(stdinData)
				logger.Info("Read initial prompt from stdin", "bytes", len(stdinData))
			}
		}
	}

	// Get the variables related to state management
	stateFile := viper.GetString(FlagStateFile)
	loadState := false
	saveState := false

	// Validate state file configuration
	if stateFile != "" {
		if !viper.IsSet(FlagLoadState) {
			loadState = true
		} else {
			loadState = viper.GetBool(FlagLoadState)
		}

		if !viper.IsSet(FlagSaveState) {
			saveState = true
		} else {
			saveState = viper.GetBool(FlagSaveState)
		}
	} else {
		if viper.IsSet(FlagLoadState) && viper.GetBool(FlagLoadState) {
			return xerrors.Errorf("--load-state requires --state-file to be set")
		}
		if viper.IsSet(FlagSaveState) && viper.GetBool(FlagSaveState) {
			return xerrors.Errorf("--save-state requires --state-file to be set")
		}
	}

	pidFile := viper.GetString(FlagPidFile)

	// Write PID file if configured
	if pidFile != "" {
		if err := writePIDFile(pidFile, logger); err != nil {
			return xerrors.Errorf("failed to write PID file: %w", err)
		}
		defer cleanupPIDFile(pidFile, logger)
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
			AgentType:      agentType,
		})
		if err != nil {
			return xerrors.Errorf("failed to setup process: %w", err)
		}
	}
	port := viper.GetInt(FlagPort)
	srv, err := httpapi.NewServer(ctx, httpapi.ServerConfig{
		AgentType:      agentType,
		Process:        process,
		Port:           port,
		ChatBasePath:   viper.GetString(FlagChatBasePath),
		AllowedHosts:   viper.GetStringSlice(FlagAllowedHosts),
		AllowedOrigins: viper.GetStringSlice(FlagAllowedOrigins),
		InitialPrompt:  initialPrompt,
		StatePersistenceConfig: screentracker.StatePersistenceConfig{
			StateFile: stateFile,
			LoadState: loadState,
			SaveState: saveState,
		},
	})

	if err != nil {
		return xerrors.Errorf("failed to create server: %w", err)
	}
	if printOpenAPI {
		fmt.Println(srv.GetOpenAPI())
		return nil
	}

	// Create a context for graceful shutdown
	gracefulCtx, gracefulCancel := context.WithCancel(ctx)
	defer gracefulCancel()

	// Setup signal handlers (they will call gracefulCancel)
	handleSignals(gracefulCtx, gracefulCancel, logger, srv)

	logger.Info("Starting server on port", "port", port)

	// Monitor process exit
	processExitCh := make(chan error, 1)
	go func() {
		defer close(processExitCh)
		defer gracefulCancel()
		if err := process.Wait(); err != nil {
			if errors.Is(err, termexec.ErrNonZeroExitCode) {
				processExitCh <- xerrors.Errorf("========\n%s\n========\n: %w", strings.TrimSpace(process.ReadScreen()), err)
			} else {
				processExitCh <- xerrors.Errorf("failed to wait for process: %w", err)
			}
		}
	}()

	// Start the server
	serverErrCh := make(chan error, 1)
	go func() {
		defer close(serverErrCh)
		if err := srv.Start(); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
		}
	}()

	select {
	case err := <-serverErrCh:
		if err != nil {
			return xerrors.Errorf("failed to start server: %w", err)
		}
	case <-gracefulCtx.Done():
	}

	if err := srv.SaveState("shutdown"); err != nil {
		logger.Error("Failed to save state during shutdown", "error", err)
	}

	// Stop the HTTP server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Stop(shutdownCtx); err != nil {
		logger.Error("Failed to stop HTTP server", "error", err)
	}

	select {
	case err := <-processExitCh:
		if err != nil {
			return xerrors.Errorf("agent exited with error: %w", err)
		}
	default:
		// Close the process
		if err := process.Close(logger, 5*time.Second); err != nil {
			logger.Error("Failed to close process cleanly", "error", err)
		}
	}
	return nil
}

var agentNames = (func() []string {
	names := make([]string, 0, len(agentTypeAliases))
	for agentType := range agentTypeAliases {
		names = append(names, agentType)
	}
	sort.Strings(names)
	return names
})()

// writePIDFile writes the current process ID to the specified file
func writePIDFile(pidFile string, logger *slog.Logger) error {
	pid := os.Getpid()
	pidContent := fmt.Sprintf("%d\n", pid)

	// Create directory if it doesn't exist
	dir := filepath.Dir(pidFile)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return xerrors.Errorf("failed to create PID file directory: %w", err)
	}

	// Write PID file
	if err := os.WriteFile(pidFile, []byte(pidContent), 0o600); err != nil {
		return xerrors.Errorf("failed to write PID file: %w", err)
	}

	logger.Info("Wrote PID file", "pidFile", pidFile, "pid", pid)
	return nil
}

// cleanupPIDFile removes the PID file if it exists
func cleanupPIDFile(pidFile string, logger *slog.Logger) {
	if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
		logger.Error("Failed to remove PID file", "pidFile", pidFile, "error", err)
	} else if err == nil {
		logger.Info("Removed PID file", "pidFile", pidFile)
	}
}

type flagSpec struct {
	name         string
	shorthand    string
	defaultValue any
	usage        string
	flagType     string
}

const (
	FlagType           = "type"
	FlagPort           = "port"
	FlagPrintOpenAPI   = "print-openapi"
	FlagChatBasePath   = "chat-base-path"
	FlagTermWidth      = "term-width"
	FlagTermHeight     = "term-height"
	FlagAllowedHosts   = "allowed-hosts"
	FlagAllowedOrigins = "allowed-origins"
	FlagExit           = "exit"
	FlagInitialPrompt  = "initial-prompt"
	FlagStateFile      = "state-file"
	FlagLoadState      = "load-state"
	FlagSaveState      = "save-state"
	FlagPidFile        = "pid-file"
)

func CreateServerCmd() *cobra.Command {
	serverCmd := &cobra.Command{
		Use:   "server [agent]",
		Short: "Run the server",
		Long:  fmt.Sprintf("Run the server with the specified agent (one of: %s)", strings.Join(agentNames, ", ")),
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// The --exit flag is used for testing validation of flags in the test suite
			if viper.GetBool(FlagExit) {
				return
			}
			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
			if viper.GetBool(FlagPrintOpenAPI) {
				// We don't want log output here.
				logger = slog.New(logctx.DiscardHandler)
			}
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
		// localhost is the default host for the server. Port is ignored during matching.
		{FlagAllowedHosts, "a", []string{"localhost", "127.0.0.1", "[::1]"}, "HTTP allowed hosts (hostnames only, no ports). Use '*' for all, comma-separated list via flag, space-separated list via AGENTAPI_ALLOWED_HOSTS env var", "stringSlice"},
		// localhost:3284 is the default origin when you open the chat interface in your browser. localhost:3000 and 3001 are used during development.
		{FlagAllowedOrigins, "o", []string{"http://localhost:3284", "http://localhost:3000", "http://localhost:3001"}, "HTTP allowed origins. Use '*' for all, comma-separated list via flag, space-separated list via AGENTAPI_ALLOWED_ORIGINS env var", "stringSlice"},
		{FlagInitialPrompt, "I", "", "Initial prompt for the agent. Recommended only if the agent doesn't support initial prompt in interaction mode. Will be read from stdin if piped (e.g., echo 'prompt' | agentapi server -- my-agent)", "string"},
		{FlagStateFile, "s", "", "Path to file for saving/loading server state", "string"},
		{FlagLoadState, "", false, "Load state from state-file on startup (defaults to true when state-file is set)", "bool"},
		{FlagSaveState, "", false, "Save state to state-file on shutdown (defaults to true when state-file is set)", "bool"},
		{FlagPidFile, "", "", "Path to file where the server process ID will be written for shutdown scripts", "string"},
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
		case "stringSlice":
			serverCmd.Flags().StringSliceP(spec.name, spec.shorthand, spec.defaultValue.([]string), spec.usage)
		default:
			panic(fmt.Sprintf("unknown flag type: %s", spec.flagType))
		}
		if err := viper.BindPFlag(spec.name, serverCmd.Flags().Lookup(spec.name)); err != nil {
			panic(fmt.Sprintf("failed to bind flag %s: %v", spec.name, err))
		}
	}

	serverCmd.Flags().Bool(FlagExit, false, "Exit immediately after parsing arguments")
	if err := serverCmd.Flags().MarkHidden(FlagExit); err != nil {
		panic(fmt.Sprintf("failed to mark flag %s as hidden: %v", FlagExit, err))
	}
	if err := viper.BindPFlag(FlagExit, serverCmd.Flags().Lookup(FlagExit)); err != nil {
		panic(fmt.Sprintf("failed to bind flag %s: %v", FlagExit, err))
	}

	viper.SetEnvPrefix("AGENTAPI")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	return serverCmd
}
