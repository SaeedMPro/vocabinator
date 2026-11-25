// Package proc runs external helper programs (never through a shell) with a
// timeout and a bounded output buffer.
package proc

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"
)

// MaxOutput caps captured stdout. Anything longer is truncated, which is fine
// for callers that only accept short text.
const MaxOutput = 64 << 10

// Runner executes a program and returns its stdout.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// Exec is the real Runner.
type Exec struct {
	Timeout time.Duration // default 3s
}

type cappedBuffer struct {
	buf bytes.Buffer
}

// Write discards bytes beyond MaxOutput but reports success so the child is
// not killed by a broken pipe while we drain it.
func (c *cappedBuffer) Write(p []byte) (int, error) {
	if room := MaxOutput - c.buf.Len(); room > 0 {
		c.buf.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}

func (e Exec) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	timeout := e.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var out cappedBuffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return out.buf.Bytes(), nil
}

// IsNotInstalled reports whether err means the program does not exist.
func IsNotInstalled(err error) bool {
	return errors.Is(err, exec.ErrNotFound)
}
