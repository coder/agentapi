//go:build windows

package server

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	"github.com/coder/agentapi/lib/httpapi"
)

// handleSignals sets up signal handlers for Windows.
func handleSignals(ctx context.Context, cancel context.CancelFunc, logger *slog.Logger, srv *httpapi.Server) {
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt)
	go func() {
		defer signal.Stop(shutdownCh)
		sig := <-shutdownCh
		logger.Info("Received shutdown signal", "signal", sig)
		cancel()
	}()
}
