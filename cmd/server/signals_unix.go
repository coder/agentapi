//go:build unix

package server

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/coder/agentapi/lib/httpapi"
)

// handleSignals sets up signal handlers for:
// - SIGTERM, SIGINT, SIGHUP: trigger graceful shutdown by canceling the context
// - SIGUSR1: save conversation state without exiting
func handleSignals(ctx context.Context, cancel context.CancelFunc, logger *slog.Logger, srv *httpapi.Server) {
	// Handle shutdown signals (SIGTERM, SIGINT, SIGHUP)
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT)
	go func() {
		defer signal.Stop(shutdownCh)
		sig := <-shutdownCh
		logger.Info("Received shutdown signal", "signal", sig)
		cancel()
	}()

	// Handle SIGUSR1 for save without exit
	saveOnlyCh := make(chan os.Signal, 1)
	signal.Notify(saveOnlyCh, syscall.SIGUSR1)
	go func() {
		defer signal.Stop(saveOnlyCh)
		for {
			select {
			case <-saveOnlyCh:
				logger.Info("Received SIGUSR1, saving state without exiting")
				if err := srv.SaveState("SIGUSR1"); err != nil {
					logger.Error("Failed to save state on SIGUSR1", "error", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}
