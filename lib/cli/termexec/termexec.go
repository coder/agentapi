package termexec

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/ActiveState/termtest/xpty"
	"github.com/coder/agentapi/lib/logctx"
	"github.com/coder/agentapi/lib/util"
	"golang.org/x/xerrors"
)

type Process struct {
	xp               *xpty.Xpty
	execCmd          *exec.Cmd
	screenUpdateLock sync.RWMutex
	lastScreenUpdate time.Time
}

type StartProcessConfig struct {
	Program        string
	Args           []string
	TerminalWidth  uint16
	TerminalHeight uint16
}

func StartProcess(ctx context.Context, args StartProcessConfig) (*Process, error) {
	logger := logctx.From(ctx)
	xp, err := xpty.New(args.TerminalWidth, args.TerminalHeight, false)
	if err != nil {
		return nil, err
	}
	execCmd := exec.Command(args.Program, args.Args...)
	// vt100 is the terminal type that the vt10x library emulates.
	// Setting this signals to the process that it should only use compatible
	// escape sequences.
	execCmd.Env = append(os.Environ(), "TERM=vt100")
	if err := xp.StartProcessInTerminal(execCmd); err != nil {
		return nil, err
	}

	process := &Process{xp: xp, execCmd: execCmd}

	go func() {
		// HACK: Working around xpty concurrency limitations
		//
		// Problem:
		// 1. We need to track when the terminal screen was last updated (for ReadScreen)
		// 2. xpty only updates terminal state through xp.ReadRune()
		// 3. xp.ReadRune() has a bug - it panics when SetReadDeadline is used
		// 4. Without deadlines, ReadRune blocks until the process outputs data
		//
		// Why this matters:
		// If we wrapped ReadRune + lastScreenUpdate in a mutex, this goroutine would
		// hold the lock while waiting for process output. Since ReadRune blocks indefinitely,
		// ReadScreen callers would be locked out until new output arrives. Even worse,
		// after output arrives, this goroutine could immediately reacquire the lock
		// for the next ReadRune call, potentially starving ReadScreen callers indefinitely.
		//
		// Solution:
		// Instead of using xp.ReadRune(), we directly use its internal components:
		// - pp.ReadRune() - handles the blocking read from the process
		// - xp.Term.WriteRune() - updates the terminal state
		//
		// This lets us apply the mutex only around the terminal update and timestamp,
		// keeping reads non-blocking while maintaining thread safety.
		//
		// Warning: This depends on xpty internals and may break if xpty changes.
		// A proper fix would require forking xpty or getting upstream changes.
		pp := util.GetUnexportedField(xp, "pp").(*xpty.PassthroughPipe)
		for {
			r, _, err := pp.ReadRune()
			if err != nil {
				if err != io.EOF {
					logger.Error("Error reading from pseudo terminal", "error", err)
				}
				// TODO: handle this error better. if this happens, the terminal
				// state will never be updated anymore and the process will appear
				// unresponsive.
				return
			}
			process.screenUpdateLock.Lock()
			// writing to the terminal updates its state. without it,
			// xp.State will always return an empty string
			xp.Term.WriteRune(r)
			process.lastScreenUpdate = time.Now()
			process.screenUpdateLock.Unlock()
		}
	}()

	return process, nil
}

func (p *Process) Signal(sig os.Signal) error {
	return p.execCmd.Process.Signal(sig)
}

// ReadScreen returns the contents of the terminal window.
// It waits for the terminal to be stable for 16ms before
// returning, or 48 ms since it's called, whichever is sooner.
//
// This logic acts as a kind of vsync. Agents regularly redraw
// parts of the screen. If we naively snapshotted the screen,
// we'd often capture it while it's being updated. This would
// result in a malformed agent message being returned to the
// user.
func (p *Process) ReadScreen() string {
	for range 3 {
		p.screenUpdateLock.RLock()
		if time.Since(p.lastScreenUpdate) >= 16*time.Millisecond {
			state := p.xp.State.String()
			p.screenUpdateLock.RUnlock()
			return state
		}
		p.screenUpdateLock.RUnlock()
		time.Sleep(16 * time.Millisecond)
	}
	return p.xp.State.String()
}

// Write sends input to the process via the pseudo terminal.
func (p *Process) Write(data []byte) (int, error) {
	return p.xp.TerminalInPipe().Write(data)
}

// Close closes the process using a SIGINT signal or forcefully killing it if the process
// does not exit after the timeout. It then closes the pseudo terminal.
func (p *Process) Close(logger *slog.Logger, timeout time.Duration) error {
	logger.Info("Closing process")
	if err := p.execCmd.Process.Signal(os.Interrupt); err != nil {
		return xerrors.Errorf("failed to send SIGINT to process: %w", err)
	}

	exited := make(chan error, 1)
	go func() {
		_, err := p.execCmd.Process.Wait()
		exited <- err
		close(exited)
	}()

	var exitErr error
	select {
	case <-time.After(timeout):
		if err := p.execCmd.Process.Kill(); err != nil {
			exitErr = xerrors.Errorf("failed to forcefully kill the process: %w", err)
		}
		// don't wait for the process to exit to avoid hanging indefinitely
		// if the process never exits
	case err := <-exited:
		var pathErr *os.SyscallError
		// ECHILD is expected if the process has already exited
		if err != nil && !(errors.As(err, &pathErr) && pathErr.Err == syscall.ECHILD) {
			exitErr = xerrors.Errorf("process exited with error: %w", err)
		}
	}
	if err := p.xp.Close(); err != nil {
		return xerrors.Errorf("failed to close pseudo terminal: %w, exitErr: %w", err, exitErr)
	}
	return exitErr
}

var ErrNonZeroExitCode = xerrors.New("non-zero exit code")

// Wait waits for the process to exit.
func (p *Process) Wait() error {
	state, err := p.execCmd.Process.Wait()
	if err != nil {
		return xerrors.Errorf("process exited with error: %w", err)
	}
	if state.ExitCode() != 0 {
		return ErrNonZeroExitCode
	}
	return nil
}
