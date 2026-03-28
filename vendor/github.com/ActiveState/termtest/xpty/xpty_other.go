// Copyright 2020 ActiveState Software. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file

// +build darwin dragonfly linux netbsd openbsd solaris

package xpty

import (
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
)

type impl struct {
	ptm    *os.File
	pts    *os.File
	rwPipe *readWritePipe
}

func open(cols uint16, rows uint16) (*impl, error) {
	ptm, pts, err := pty.Open()
	if err != nil {
		return nil, err
	}
	err = pty.Setsize(ptm, &pty.Winsize{Cols: cols, Rows: rows})
	if err != nil {
		return nil, err
	}
	return &impl{ptm: ptm, pts: pts}, nil
}

func (p *impl) terminalOutPipe() io.Reader {
	return p.ptm
}

func (p *impl) terminalInPipe() io.Writer {
	return p.ptm
}

func (p *impl) resize(cols uint16, rows uint16) error {
	return pty.Setsize(p.ptm, &pty.Winsize{Cols: cols, Rows: rows})
}

func (p *impl) close() error {
	p.pts.Close()
	p.ptm.Close()
	return nil
}

func (p *impl) tty() *os.File {
	return p.pts
}

func (p *impl) terminalOutFd() uintptr {
	return p.ptm.Fd()
}

func (p *impl) startProcessInTerminal(cmd *exec.Cmd) error {
	cmd.Stdin = p.pts
	cmd.Stdout = p.pts
	cmd.Stderr = p.pts
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setctty = true
	cmd.SysProcAttr.Setsid = true
	return cmd.Start()
}
