// Copyright 2020 ActiveState Software. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file

package xpty

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/ActiveState/vt10x"
)

// Xpty reprents an abstract peudo-terminal for the Windows or *nix architecture
type Xpty struct {
	*impl  // os specific
	Term   *vt10x.VT
	State  *vt10x.State
	rwPipe *readWritePipe
	pp     *PassthroughPipe
}

// readWritePipe is a helper that we use to let the application communicate with a virtual terminal.
type readWritePipe struct {
	r *io.PipeReader
	w *io.PipeWriter
}

func newReadWritePipe() *readWritePipe {
	r, w := io.Pipe()
	return &readWritePipe{r, w}
}

// Read from the reader part of the pipe
func (rw *readWritePipe) Read(buf []byte) (int, error) {
	return rw.r.Read(buf)
}

// Write to the writer part of the pipe
func (rw *readWritePipe) Write(buf []byte) (int, error) {
	return rw.w.Write(buf)
}

// Close all parts of the pipe
func (rw *readWritePipe) Close() error {
	var errMessage string
	e := rw.r.Close()
	if e != nil {
		errMessage += fmt.Sprintf("failed to close read-part of pipe: %v ", e)
	}
	e = rw.w.Close()
	if e != nil {
		errMessage += fmt.Sprintf("failed to close write-part of pipe: %v ", e)
	}
	if len(errMessage) > 0 {
		return fmt.Errorf(errMessage)
	}
	return nil
}

func (p *Xpty) openVT(cols uint16, rows uint16) (err error) {

	/*
			 We are creating a communication pipe to handle DSR (device status report) and
			 (CPR) cursor position report queries.

			 If an application is sending these queries it is usually expecting a response
		     from the terminal emulator (like xterm). If the response is not send, the
		     application may hang forever waiting for it. The vt10x terminal emulator is able to handle it. If
		     we multiplex the ptm output to a vt10x terminal, the DSR/CPR requests are
		     intercepted and it can inject the responses in the read-write-pipe.

			 The read-part of the read-write-pipe continuously feeds into the ptm device that
			 forwards it to the application.

			      DSR/CPR req                        reply
			 app ------------->  pts/ptm -> vt10x.VT ------> rwPipe --> ptm/pts --> app

			 Note: This is a simplification from github.com/hinshun/vt10x (console.go)
	*/

	p.rwPipe = newReadWritePipe()

	// Note: the Term instance also closes the rwPipe
	p.Term, err = vt10x.Create(p.State, p.rwPipe)
	if err != nil {
		return err
	}
	p.Term.Resize(int(cols), int(rows))

	// connect the pipes as described above
	go func() {
		// this drains the rwPipe continuously.  If that didn't happen, we would block on write.
		io.Copy(p.impl.terminalInPipe(), p.rwPipe)
	}()

	// forward the terminal output to a passthrough pipe, such that we can read it rune-by-rune
	// and can control read timeouts
	br := bufio.NewReaderSize(p.impl.terminalOutPipe(), 100)
	p.pp = NewPassthroughPipe(br)
	return nil
}

// Resize resizes the underlying pseudo-terminal
func (p *Xpty) Resize(cols, rows uint16) error {
	p.Term.Resize(int(cols), int(rows))
	return p.impl.resize(cols, rows)
}

// New opens a pseudo-terminal of the given size
func New(cols uint16, rows uint16, recordHistory bool) (*Xpty, error) {
	xpImpl, err := open(cols, rows)
	if err != nil {
		return nil, err
	}
	xp := &Xpty{impl: xpImpl, Term: nil, State: &vt10x.State{RecordHistory: recordHistory}}
	err = xp.openVT(cols, rows)
	if err != nil {
		return nil, err
	}

	return xp, nil
}

// ReadRune reads a single rune from the terminal output pipe, and updates the terminal
func (p *Xpty) ReadRune() (rune, int, error) {
	c, sz, err := p.pp.ReadRune()
	if err != nil {
		return c, 0, err
	}
	// update the terminal
	p.Term.WriteRune(c)
	return c, sz, err
}

// SetReadDeadline sets a deadline for a successful read the next rune
func (p *Xpty) SetReadDeadline(d time.Time) {
	p.pp.SetReadDeadline(d)
}

// TerminalInPipe returns a writer that can be used to write user input to the pseudo terminal.
// On unix this is the /dev/ptm file
func (p *Xpty) TerminalInPipe() io.Writer {
	return p.impl.terminalInPipe()
}

// WriteTo writes the terminal output stream to a writer w
func (p *Xpty) WriteTo(w io.Writer) (int64, error) {
	var written int64
	for {
		c, sz, err := p.ReadRune()
		if err != nil {
			return written, err
		}
		written += int64(sz)
		_, err = w.Write([]byte(string(c)))
		if err != nil {
			return written, fmt.Errorf("failed writing to writer: %w", err)
		}
	}

}

// WaitTillDrained waits until the PassthroughPipe is blocked in the reading state.
// When this function returns, the PassthroughPipe should be blocked in the
// reading state waiting for more input.
func (p *Xpty) WaitTillDrained() {
	for {
		if p.pp.IsBlocked() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// CloseTTY closes just the terminal, giving you some time to finish reading from the
// pass-through pipe later.
// Call CloseReaders() when you are done reading all the data that is still buffered
// Consider this little dance to avoid losing any data:
//     go func() {
//          ...
//     		// command finishes
//          cmd.Wait()
//          // wait until the pass-through pipe has consumed all data
//          xp.WaitTillDrained()
//          xp.CloseTTY()
//     }()
//     xp.WriteTo(...)
//     // now close the passthrough pipe
//     xp.CloseReaders()
func (p *Xpty) CloseTTY() error {
	return p.impl.close()
}

// CloseReaders closes the passthrough pipe
func (p *Xpty) CloseReaders() error {
	err := p.pp.Close()
	if err != nil {
		return err
	}
	if p.Term == nil {
		return nil
	}
	return p.Term.Close()
}

// Close closes the abstracted pseudo-terminal
func (p *Xpty) Close() error {
	err := p.CloseTTY()
	if err != nil {
		return err
	}
	return p.CloseReaders()
}

// Tty returns the pseudo terminal files that an application can read from or write to
// This is only available on linux, and would return the "slave" /dev/pts file
func (p *Xpty) Tty() *os.File {
	return p.impl.tty()
}

// TerminalOutFd returns the file descriptor of the terminal
func (p *Xpty) TerminalOutFd() uintptr {
	return p.impl.terminalOutFd()
}

// StartProcessInTerminal executes the given command connected to the abstracted pseudo-terminal
func (p *Xpty) StartProcessInTerminal(cmd *exec.Cmd) error {
	return p.impl.startProcessInTerminal(cmd)
}
