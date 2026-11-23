package proc

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestExecRunsWithoutShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX printf")
	}
	// A shell would expand $HOME or run the substitution; we must not.
	out, err := Exec{}.Run(context.Background(), "printf", "%s", "$HOME; echo pwned")
	if err != nil || string(out) != "$HOME; echo pwned" {
		t.Errorf("out=%q err=%v", out, err)
	}
}

func TestExecNotInstalled(t *testing.T) {
	_, err := Exec{}.Run(context.Background(), "definitely-not-a-real-binary-xyz")
	if !IsNotInstalled(err) {
		t.Errorf("expected not-installed error, got %v", err)
	}
	if IsNotInstalled(nil) {
		t.Error("nil is not a not-installed error")
	}
}

func TestExecCapsOutputAndReportsExitFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX tools")
	}
	out, err := Exec{}.Run(context.Background(), "sh", "-c", "head -c 200000 /dev/zero")
	if err != nil || len(out) != MaxOutput {
		t.Errorf("len=%d err=%v", len(out), err)
	}

	if _, err := (Exec{}).Run(context.Background(), "sh", "-c", "exit 3"); err == nil || IsNotInstalled(err) {
		t.Errorf("expected plain exit error, got %v", err)
	}
}

func TestExecTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX sleep")
	}
	start := time.Now()
	_, err := Exec{Timeout: 50 * time.Millisecond}.Run(context.Background(), "sleep", "5")
	if err == nil || time.Since(start) > 2*time.Second {
		t.Errorf("expected quick timeout, got %v after %v", err, time.Since(start))
	}
	if strings.Contains(err.Error(), "not found") {
		t.Errorf("unexpected error %v", err)
	}
}
