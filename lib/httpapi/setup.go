package httpapi

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/coder/agentapi/lib/logctx"
	mf "github.com/coder/agentapi/lib/msgfmt"
	st "github.com/coder/agentapi/lib/screentracker"
	"github.com/coder/agentapi/lib/termexec"
	"github.com/coder/agentapi/x/acpio"
)

// AgentIO is the interface for agent communication (re-exported from screentracker)
type AgentIO = st.AgentIO

type SetupProcessConfig struct {
	Program        string
	ProgramArgs    []string
	TerminalWidth  uint16
	TerminalHeight uint16
	AgentType      mf.AgentType
}

func SetupProcess(ctx context.Context, config SetupProcessConfig) (*termexec.Process, error) {
	logger := logctx.From(ctx)

	logger.Info(fmt.Sprintf("Running: %s %s", config.Program, strings.Join(config.ProgramArgs, " ")))

	process, err := termexec.StartProcess(ctx, termexec.StartProcessConfig{
		Program:        config.Program,
		Args:           config.ProgramArgs,
		TerminalWidth:  config.TerminalWidth,
		TerminalHeight: config.TerminalHeight,
	})
	if err != nil {
		logger.Error(fmt.Sprintf("Error starting process: %v", err))
		os.Exit(1)
	}

	// Hack for sourcegraph amp to stop the animation.
	if config.AgentType == mf.AgentTypeAmp {
		_, err = process.Write([]byte(" \b"))
		if err != nil {
			return nil, err
		}
	}

	// Handle SIGINT (Ctrl+C) and send it to the process
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalCh
		if err := process.Close(logger, 5*time.Second); err != nil {
			logger.Error("Error closing process", "error", err)
		}
	}()

	return process, nil
}

type SetupACPConfig struct {
	Program     string
	ProgramArgs []string
}

func SetupACP(ctx context.Context, config SetupACPConfig) (*acpio.ACPAgentIO, error) {
	logger := logctx.From(ctx)

	args := config.ProgramArgs
	logger.Info(fmt.Sprintf("Running (ACP): %s %s", config.Program, strings.Join(args, " ")))

	cmd := exec.CommandContext(ctx, config.Program, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start process: %w", err)
	}

	agentIO, err := acpio.NewWithPipes(ctx, stdin, stdout, logger)
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("failed to initialize ACP connection: %w", err)
	}

	go func() {
		<-ctx.Done()
		logger.Info("Context done, closing ACP agent")
		_ = stdin.Close()
		_ = stdout.Close()
		_ = cmd.Process.Kill()
	}()

	return agentIO, nil
}
