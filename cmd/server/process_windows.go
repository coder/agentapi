//go:build windows

package server

// isProcessRunning checks if a process with the given PID is running.
// On Windows, Signal(0) is not supported, so this always returns false.
// PID file liveness detection is best-effort on this platform.
func isProcessRunning(_ int) bool {
	return false
}
