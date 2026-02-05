//go:build windows

package httpapi

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/coder/agentapi/lib/termexec"
)

// HandleSignals sets up signal handlers for Windows.
// Only handles SIGTERM and SIGINT (SIGHUP and SIGUSR1 don't exist on Windows).
func (s *Server) HandleSignals(ctx context.Context, process *termexec.Process) {
	// Handle shutdown signals (SIGTERM, SIGINT only on Windows)
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		defer signal.Stop(shutdownCh)
		sig := <-shutdownCh
		s.logger.Info("Received shutdown signal, saving state before closing process", "signal", sig)

		s.saveAndCleanup(sig, process)
	}()
}
