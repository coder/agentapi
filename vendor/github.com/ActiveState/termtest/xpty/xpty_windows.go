// Copyright 2020 ActiveState Software. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file

// +build windows

package xpty

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

	conpty "github.com/ActiveState/termtest/conpty"
)

type impl struct {
	*conpty.ConPty
}

func open(cols, rows uint16) (*impl, error) {
	c, err := conpty.New(int16(cols), int16(rows))
	if err != nil {
		return nil, err
	}
	return &impl{c}, nil
}

func (p *impl) terminalOutPipe() io.Reader {
	return p.OutPipe()
}

func (p *impl) terminalInPipe() io.Writer {
	return p.InPipe()
}

func (p *impl) close() error {
	return p.Close()
}

func (p *impl) tty() *os.File {
	return nil
}

func (p *impl) terminalOutFd() uintptr {
	return p.OutFd()
}

func (p *impl) resize(cols, rows uint16) error {
	return p.ConPty.Resize(cols, rows)
}

func (p *impl) startProcessInTerminal(c *exec.Cmd) (err error) {
	var argv []string
	if len(c.Args) > 0 {
		argv = c.Args
	} else {
		argv = []string{c.Path}
	}

	var envv []string
	if c.Env != nil {
		envv = c.Env
	} else {
		envv = os.Environ()
	}
	pid, _, err := p.Spawn(c.Path, argv, &syscall.ProcAttr{
		Dir: c.Dir,
		Env: envv,
	})
	if err != nil {
		return fmt.Errorf("Failed to spawn process in terminal: %w", err)
	}

	// Let's pray that this always works.  Unfortunately we cannot create our process from a process handle.
	c.Process, err = os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("Failed to create an os.Process struct: %w", err)
	}

	// runtime.SetFinalizer(h, )

	return nil
}
