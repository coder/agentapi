// Copyright 2020 ActiveState Software, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file

package xpty

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"time"
	"unicode"
)

type errPassthroughTimeout struct {
	error
}

func (errPassthroughTimeout) Timeout() bool { return true }

// PassthroughPipe pipes data from a io.Reader and allows setting a read
// deadline. If a timeout is reached the error is returned, otherwise the error
// from the provided io.Reader returned is passed through instead.
type PassthroughPipe struct {
	rdr      *bufio.Reader
	deadline time.Time
	ctx      context.Context
	cancel   context.CancelFunc
	lastRead int64
}

var maxTime = time.Unix(1<<60-1, 999999999)

// NewPassthroughPipe returns a new pipe for a io.Reader that passes through
// non-timeout errors.
func NewPassthroughPipe(r *bufio.Reader) *PassthroughPipe {
	ctx, cancel := context.WithCancel(context.Background())

	p := PassthroughPipe{
		rdr:      r,
		deadline: maxTime,
		ctx:      ctx,
		cancel:   cancel,
	}

	return &p
}

// IsBlocked returns true when the PassthroughPipe is (most likely) blocked reading ie., waiting for input
func (p *PassthroughPipe) IsBlocked() bool {
	lr := atomic.LoadInt64(&p.lastRead)
	return time.Duration(time.Now().UTC().UnixNano()-lr) > 100*time.Millisecond
}

// SetReadDeadline sets a deadline for a successful read
func (p *PassthroughPipe) SetReadDeadline(d time.Time) {
	p.deadline = d
}

// Close releases all resources allocated by the pipe
func (p *PassthroughPipe) Close() error {
	p.cancel()
	return nil
}

type runeResponse struct {
	rune rune
	size int
	err  error
}

// ReadRune reads from the PassthroughPipe and errors out if no data has been written to the pipe before the read deadline expired
// If read is called after the PassthroughPipe has been closed `0, io.EOF` is returned
func (p *PassthroughPipe) ReadRune() (rune, int, error) {
	cs := make(chan runeResponse)
	done := make(chan struct{})
	defer close(done)
	atomic.StoreInt64(&p.lastRead, time.Now().UTC().UnixNano())

	go func() {
		defer close(cs)

		if p.ctx.Err() != nil || p.deadline.Before(time.Now()) {
			return
		}

		var (
			r   rune
			sz  int
			err error
		)
		for {
			r, sz, err = p.rdr.ReadRune()

			if err != nil && r == unicode.ReplacementChar && sz == 1 {
				if p.rdr.Buffered() > 0 {
					err = fmt.Errorf("invalid utf8 sequence")
					break
				}
				continue
			}
			break
		}

		select {
		case <-done:
			return
		default:
			cs <- runeResponse{r, sz, err}
		}
	}()

	select {
	case c := <-cs:
		return c.rune, c.size, c.err

	case <-p.ctx.Done():
		return rune(0), 0, io.EOF

	case <-time.After(p.deadline.Sub(time.Now())):
		return rune(0), 0, &errPassthroughTimeout{errors.New("passthrough i/o timeout")}
	}
}
