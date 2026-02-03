//go:build unix

package httpapi

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/coder/agentapi/lib/termexec"
)

// HandleSignals sets up signal handlers for:
// - SIGTERM, SIGINT, SIGHUP: save conversation state, then close the process
// - SIGUSR1: save conversation state without exiting
func (s *Server) HandleSignals(ctx context.Context, process *termexec.Process) {
	// Handle shutdown signals (SIGTERM, SIGINT, SIGHUP)
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		sig := <-shutdownCh
		s.logger.Info("Received shutdown signal, saving state before closing process", "signal", sig)

		s.saveAndCleanup(sig, process)
	}()

	// Handle SIGUSR1 for save without exit
	saveOnlyCh := make(chan os.Signal, 1)
	signal.Notify(saveOnlyCh, syscall.SIGUSR1)
	go func() {
		for {
			select {
			case <-saveOnlyCh:
				s.logger.Info("Received SIGUSR1, saving state without exiting")
				s.saveStateIfConfigured("SIGUSR1")
			case <-ctx.Done():
				return
			}
		}
	}()
}
