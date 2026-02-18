package server

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/coder/agentapi/lib/httpapi"
	"github.com/coder/agentapi/lib/termexec"
)

// performGracefulShutdown handles the common shutdown logic for all platforms.
// It saves state, stops the HTTP server, closes the process, and exits.
func performGracefulShutdown(sig os.Signal, logger *slog.Logger, srv *httpapi.Server, process *termexec.Process, pidFile string) {
	logger.Info("Received shutdown signal, initiating graceful shutdown", "signal", sig)

	// Save state
	if err := srv.SaveState(sig.String()); err != nil {
		logger.Error("Failed to save state during shutdown", "signal", sig, "error", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Stop(shutdownCtx); err != nil {
		logger.Error("Failed to stop HTTP server", "signal", sig, "error", err)
	}

	// Close the process
	if err := process.Close(logger, 5*time.Second); err != nil {
		logger.Error("Failed to close process cleanly", "signal", sig, "error", err)
	}

	// Clean up PID file before exit
	if pidFile != "" {
		cleanupPIDFile(pidFile, logger)
	}

	// Exit cleanly
	os.Exit(0)
}
