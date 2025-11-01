package termexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/agentapi/lib/logctx"
	"golang.org/x/xerrors"
)

const DefaultTmuxSessionName = "wingman-agents"

type Process struct {
	sessionName string
	paneID      string
	windowID    string
	bufferName  string
	width       uint16
	height      uint16
	writeLock   sync.Mutex
}

type StartProcessConfig struct {
	Program        string
	Args           []string
	TerminalWidth  uint16
	TerminalHeight uint16
	SessionName    string
}

func StartProcess(ctx context.Context, args StartProcessConfig) (*Process, error) {
	logger := logctx.From(ctx)
	sessionName := args.SessionName
	if sessionName == "" {
		sessionName = DefaultTmuxSessionName
	}
	width := args.TerminalWidth
	height := args.TerminalHeight
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 24
	}

	sessionExists, err := tmuxSessionExists(ctx, sessionName)
	if err != nil {
		return nil, err
	}

	var paneID string
	var windowID string

	if !sessionExists {
		tmuxArgs := []string{"new-session", "-d", "-s", sessionName, "-x", strconv.Itoa(int(width)), "-y", strconv.Itoa(int(height)), "-P", "-F", "#{pane_id},#{window_id}", "--", "env", "TERM=vt100", args.Program}
		if len(args.Args) > 0 {
			tmuxArgs = append(tmuxArgs, args.Args...)
		}
		cmd := exec.CommandContext(ctx, "tmux", tmuxArgs...)
		output, startErr := cmd.CombinedOutput()
		if startErr != nil {
			return nil, xerrors.Errorf("failed to start tmux session: %w: %s", startErr, output)
		}
		paneID, windowID, err = parsePaneWindowIDs(output)
		if err != nil {
			return nil, err
		}
		logger.Info("tmux session created", "session", sessionName)
		if err := configureSessionForClients(sessionName); err != nil {
			logger.Warn("failed to configure tmux session options", "session", sessionName, "error", err)
		}
		logger.Info("tmux window created", "session", sessionName, "window", windowID, "pane", paneID)
	} else {
		tmuxArgs := []string{"new-window", "-d", "-t", sessionName, "-P", "-F", "#{pane_id},#{window_id}", "--", "env", "TERM=vt100", args.Program}
		if len(args.Args) > 0 {
			tmuxArgs = append(tmuxArgs, args.Args...)
		}
		cmd := exec.CommandContext(ctx, "tmux", tmuxArgs...)
		output, startErr := cmd.CombinedOutput()
		if startErr != nil {
			return nil, xerrors.Errorf("failed to create tmux window: %w: %s", startErr, output)
		}
		paneID, windowID, err = parsePaneWindowIDs(output)
		if err != nil {
			return nil, err
		}
		if err := configureSessionForClients(sessionName); err != nil {
			logger.Warn("failed to configure tmux session options", "session", sessionName, "error", err)
		}
		resizeWidth := width
		resizeHeight := height
		if clientWidth, clientHeight, clientErr := clientSize(sessionName); clientErr == nil {
			if clientWidth > 0 {
				resizeWidth = clientWidth
			}
			if clientHeight > 0 {
				resizeHeight = clientHeight
			}
		} else if !errors.Is(clientErr, errPaneNotFound) {
			logger.Warn("failed to look up tmux client size", "session", sessionName, "error", clientErr)
		}
		if sessionWidth, sessionHeight, sizeErr := sessionSize(sessionName); sizeErr == nil {
			if sessionWidth > 0 {
				resizeWidth = sessionWidth
			}
			if sessionHeight > 0 {
				resizeHeight = sessionHeight
			}
		} else {
			logger.Warn("failed to look up tmux session size", "session", sessionName, "error", sizeErr)
		}
		if resizeErr := resizeWindow(windowID, resizeWidth, resizeHeight); resizeErr != nil && !errors.Is(resizeErr, errPaneNotFound) {
			return nil, resizeErr
		}
		logger.Info("tmux window created", "session", sessionName, "window", windowID, "pane", paneID)
	}

	process := &Process{
		sessionName: sessionName,
		paneID:      paneID,
		windowID:    windowID,
		bufferName:  fmt.Sprintf("%s_buffer_%s", sessionName, strings.TrimPrefix(paneID, "%")),
		width:       width,
		height:      height,
	}
	return process, nil
}

func (p *Process) Signal(sig os.Signal) error {
	// Sending signals is not supported directly when running inside tmux.
	return xerrors.Errorf("sending signals is not supported for tmux-managed processes")
}

func (p *Process) ReadScreen() string {
	height := p.height
	if height == 0 {
		height = 24
	}
	start := fmt.Sprintf("-%d", height)
	cmd := exec.Command("tmux", "capture-pane", "-t", p.paneID, "-p", "-S", start)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return string(output)
}

func (p *Process) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	p.writeLock.Lock()
	defer p.writeLock.Unlock()

	loadCmd := exec.Command("tmux", "load-buffer", "-b", p.bufferName, "-")
	loadCmd.Stdin = bytes.NewReader(data)
	if output, err := loadCmd.CombinedOutput(); err != nil {
		return 0, xerrors.Errorf("tmux load-buffer failed: %w: %s", err, output)
	}

	pasteCmd := exec.Command("tmux", "paste-buffer", "-t", p.paneID, "-b", p.bufferName, "-d")
	if output, err := pasteCmd.CombinedOutput(); err != nil {
		return 0, xerrors.Errorf("tmux paste-buffer failed: %w: %s", err, output)
	}
	return len(data), nil
}

func (p *Process) Close(logger *slog.Logger, timeout time.Duration) error {
	logger.Info("Closing process")
	if err := p.sendKeys("C-c"); err != nil {
		logger.Warn("failed to send interrupt to tmux session", "error", err)
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		dead, _, err := p.paneStatus()
		if err != nil && !errors.Is(err, errPaneNotFound) {
			return err
		}
		if dead || errors.Is(err, errPaneNotFound) {
			if err := p.killWindow(); err != nil && !errors.Is(err, errPaneNotFound) {
				return err
			}
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	if err := p.killWindow(); err != nil && !errors.Is(err, errPaneNotFound) {
		return err
	}
	return nil
}

var ErrNonZeroExitCode = xerrors.New("non-zero exit code")

func (p *Process) Wait() error {
	for {
		dead, status, err := p.paneStatus()
		if err != nil {
			if errors.Is(err, errPaneNotFound) {
				return nil
			}
			return err
		}
		if dead {
			if status != 0 {
				return ErrNonZeroExitCode
			}
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
}

var errPaneNotFound = errors.New("tmux pane not found")

func (p *Process) paneStatus() (bool, int, error) {
	cmd := exec.Command("tmux", "display-message", "-p", "-t", p.paneID, "#{pane_dead}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		if isTmuxTargetMissing(err, output) {
			return true, 0, errPaneNotFound
		}
		return false, 0, xerrors.Errorf("tmux display-message failed: %w: %s", err, output)
	}
	dead := strings.TrimSpace(string(output)) == "1"
	if !dead {
		return false, 0, nil
	}
	cmd = exec.Command("tmux", "display-message", "-p", "-t", p.paneID, "#{pane_dead_status}")
	output, err = cmd.CombinedOutput()
	if err != nil {
		if isTmuxTargetMissing(err, output) {
			return true, 0, errPaneNotFound
		}
		return true, 0, xerrors.Errorf("tmux display-message failed: %w: %s", err, output)
	}
	statusStr := strings.TrimSpace(string(output))
	if statusStr == "" {
		return true, 0, nil
	}
	status, err := strconv.Atoi(statusStr)
	if err != nil {
		return true, 0, xerrors.Errorf("failed to parse pane dead status: %w", err)
	}
	return true, status, nil
}

func (p *Process) killWindow() error {
	if p.windowID == "" {
		return nil
	}
	cmd := exec.Command("tmux", "kill-window", "-t", p.windowID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if isTmuxTargetMissing(err, output) {
			return errPaneNotFound
		}
		return xerrors.Errorf("tmux kill-window failed: %w: %s", err, output)
	}
	return nil
}

func (p *Process) sendKeys(keys string) error {
	cmd := exec.Command("tmux", "send-keys", "-t", p.paneID, keys)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if isTmuxTargetMissing(err, output) {
			return errPaneNotFound
		}
		return xerrors.Errorf("tmux send-keys failed: %w: %s", err, output)
	}
	return nil
}

func isTmuxTargetMissing(err error, output []byte) bool {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		out := strings.ToLower(string(output))
		if strings.Contains(out, "can't find") || strings.Contains(out, "no such") || strings.Contains(out, "not found") {
			return true
		}
	}
	return false
}

func tmuxSessionExists(ctx context.Context, sessionName string) (bool, error) {
	cmd := exec.CommandContext(ctx, "tmux", "has-session", "-t", sessionName)
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, xerrors.Errorf("failed to check tmux session: %w", err)
	}
	return true, nil
}

func parsePaneWindowIDs(output []byte) (string, string, error) {
	parts := strings.Split(strings.TrimSpace(string(output)), ",")
	if len(parts) != 2 {
		return "", "", xerrors.Errorf("unexpected tmux output: %q", string(output))
	}
	return parts[0], parts[1], nil
}

func sessionSize(sessionName string) (uint16, uint16, error) {
	cmd := exec.Command("tmux", "display-message", "-p", "-t", sessionName, "#{session_width},#{session_height}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		if isTmuxTargetMissing(err, output) {
			return 0, 0, errPaneNotFound
		}
		return 0, 0, xerrors.Errorf("tmux display-message failed: %w: %s", err, output)
	}
	parts := strings.Split(strings.TrimSpace(string(output)), ",")
	if len(parts) != 2 {
		return 0, 0, xerrors.Errorf("unexpected tmux output: %q", string(output))
	}
	width, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, xerrors.Errorf("failed to parse session width %q: %w", parts[0], err)
	}
	height, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, xerrors.Errorf("failed to parse session height %q: %w", parts[1], err)
	}
	return uint16(width), uint16(height), nil
}

func clientSize(sessionName string) (uint16, uint16, error) {
	cmd := exec.Command("tmux", "list-clients", "-t", sessionName, "-F", "#{client_width},#{client_height}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		if isTmuxTargetMissing(err, output) {
			return 0, 0, errPaneNotFound
		}
		// When no clients are attached tmux exits with status 1 and the message "no clients".
		// Treat this as "no size available" rather than an error.
		out := strings.TrimSpace(string(output))
		if strings.Contains(out, "no clients") {
			return 0, 0, nil
		}
		return 0, 0, xerrors.Errorf("tmux list-clients failed: %w: %s", err, output)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) != 2 {
			continue
		}
		width, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return 0, 0, xerrors.Errorf("failed to parse client width %q: %w", parts[0], err)
		}
		height, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return 0, 0, xerrors.Errorf("failed to parse client height %q: %w", parts[1], err)
		}
		return uint16(width), uint16(height), nil
	}
	return 0, 0, nil
}

func configureSessionForClients(sessionName string) error {
	var errs []error
	if err := setTmuxOption(sessionName, "window-size", "latest"); err != nil {
		errs = append(errs, err)
	}
	if err := setTmuxOption(sessionName, "aggressive-resize", "on"); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func setTmuxOption(sessionName, option, value string) error {
	cmd := exec.Command("tmux", "set-option", "-t", sessionName, option, value)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if isTmuxTargetMissing(err, output) {
			return errPaneNotFound
		}
		return xerrors.Errorf("tmux set-option failed: %w: %s", err, output)
	}
	return nil
}

func resizeWindow(windowID string, width, height uint16) error {
	if windowID == "" {
		return nil
	}
	args := []string{"resize-window", "-t", windowID}
	if width > 0 {
		args = append(args, "-x", strconv.Itoa(int(width)))
	}
	if height > 0 {
		args = append(args, "-y", strconv.Itoa(int(height)))
	}
	if len(args) <= 3 {
		return nil
	}
	cmd := exec.Command("tmux", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if isTmuxTargetMissing(err, output) {
			return errPaneNotFound
		}
		return xerrors.Errorf("tmux resize-window failed: %w: %s", err, output)
	}
	return nil
}
