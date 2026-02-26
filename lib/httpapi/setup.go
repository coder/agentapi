package httpapi

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/coder/agentapi/lib/logctx"
	mf "github.com/coder/agentapi/lib/msgfmt"
	"github.com/coder/agentapi/lib/termexec"
	"github.com/coder/agentapi/x/acpio"
	"github.com/coder/quartz"
)

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
	return process, nil
}

type SetupACPConfig struct {
	Program     string
	ProgramArgs []string
	Clock       quartz.Clock
}

// SetupACPResult contains the result of setting up an ACP process.
type SetupACPResult struct {
	AgentIO *acpio.ACPAgentIO
	Wait    func() error  // Calls cmd.Wait() and returns exit error
	Done    chan struct{} // Close this when Wait() returns to clean up goroutine
}

func SetupACP(ctx context.Context, config SetupACPConfig) (*SetupACPResult, error) {
	logger := logctx.From(ctx)

	if config.Clock == nil {
		config.Clock = quartz.NewReal()
	}

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

	agentIO, err := acpio.NewWithPipes(ctx, stdin, stdout, logger, os.Getwd)
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("failed to initialize ACP connection: %w", err)
	}

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			logger.Info("Context done, closing ACP agent")
			// Try graceful shutdown first
			_ = cmd.Process.Signal(syscall.SIGTERM)
			// Then close pipes
			_ = stdin.Close()
			_ = stdout.Close()
			// Force kill after timeout
			config.Clock.AfterFunc(5*time.Second, func() {
				_ = cmd.Process.Kill()
			})
			return
		case <-done:
			// Process exited normally, nothing to clean up
			return
		}
	}()

	return &SetupACPResult{
		AgentIO: agentIO,
		Wait:    cmd.Wait,
		Done:    done,
	}, nil
}
