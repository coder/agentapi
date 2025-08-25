package httpapi

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/coder/agentapi/lib/logctx"
	mf "github.com/coder/agentapi/lib/msgfmt"
	"github.com/coder/agentapi/lib/termexec"
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
