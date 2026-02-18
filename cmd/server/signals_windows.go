//go:build windows

package server

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/coder/agentapi/lib/httpapi"
	"github.com/coder/agentapi/lib/termexec"
)

// handleSignals sets up signal handlers for Windows.
// Only handles SIGTERM and SIGINT (SIGHUP and SIGUSR1 don't exist on Windows).
func handleSignals(ctx context.Context, logger *slog.Logger, srv *httpapi.Server, process *termexec.Process, pidFile string) {
	// Handle shutdown signals (SIGTERM, SIGINT only on Windows)
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		defer signal.Stop(shutdownCh)
		sig := <-shutdownCh
		performGracefulShutdown(sig, logger, srv, process, pidFile)
	}()
}
