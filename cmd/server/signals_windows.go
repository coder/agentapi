//go:build windows

package server

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/coder/agentapi/lib/httpapi"
)

// handleSignals sets up signal handlers for Windows.
// Only handles SIGTERM and SIGINT (SIGHUP and SIGUSR1 don't exist on Windows).
func handleSignals(ctx context.Context, cancel context.CancelFunc, logger *slog.Logger, srv *httpapi.Server) {
	// Handle shutdown signals (SIGTERM, SIGINT only on Windows)
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		defer signal.Stop(shutdownCh)
		sig := <-shutdownCh
		logger.Info("Received shutdown signal", "signal", sig)
		cancel()
	}()
}
