package notify

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

type fakeRunner struct {
	err  error
	name string
	args []string
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.name, f.args = name, args
	return nil, f.err
}

func TestLinuxNotifySend(t *testing.T) {
	r := &fakeRunner{}
	n := &Notifier{goos: "linux", getenv: func(k string) string { return map[string]string{"DISPLAY": ":0"}[k] }, runner: r}

	if err := n.Notify(context.Background(), "-robust", "a < b & c > d"); err != nil {
		t.Fatal(err)
	}
	if r.name != "notify-send" {
		t.Errorf("name %q", r.name)
	}
	line := strings.Join(r.args, "|")
	if !strings.Contains(line, "--|-robust|a &lt; b &amp; c &gt; d") {
		t.Errorf("title must follow -- and body be escaped: %s", line)
	}
}

func TestMacUsesArgvNotScriptInterpolation(t *testing.T) {
	r := &fakeRunner{}
	n := &Notifier{goos: "darwin", getenv: func(string) string { return "" }, runner: r}

	evil := `x" & (do shell script "id") & "`
	if err := n.Notify(context.Background(), evil, evil); err != nil {
		t.Fatal(err)
	}
	for i, a := range r.args {
		if strings.HasPrefix(a, "display") || a == "on run argv" || a == "end run" {
			if strings.Contains(a, "do shell script") {
				t.Errorf("script text %d contains user data: %q", i, a)
			}
		}
	}
	if r.args[len(r.args)-1] != evil || r.args[len(r.args)-2] != evil {
		t.Errorf("title/body must be trailing argv: %v", r.args)
	}
}

func TestUnavailableCases(t *testing.T) {
	headless := &Notifier{goos: "linux", getenv: func(string) string { return "" }, runner: &fakeRunner{}}
	if err := headless.Notify(context.Background(), "t", "b"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("headless: %v", err)
	}

	win := &Notifier{goos: "windows", getenv: func(string) string { return "" }, runner: &fakeRunner{}}
	if err := win.Notify(context.Background(), "t", "b"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("windows: %v", err)
	}

	missing := &Notifier{goos: "linux", getenv: func(string) string { return ":0" }, runner: &fakeRunner{err: fmt.Errorf("x: %w", exec.ErrNotFound)}}
	if err := missing.Notify(context.Background(), "t", "b"); !errors.Is(err, ErrUnavailable) || !strings.Contains(err.Error(), "notify-send") {
		t.Errorf("missing tool: %v", err)
	}

	failing := &Notifier{goos: "linux", getenv: func(string) string { return ":0" }, runner: &fakeRunner{err: errors.New("boom")}}
	if err := failing.Notify(context.Background(), "t", "b"); err == nil || errors.Is(err, ErrUnavailable) {
		t.Errorf("a failing tool is an error, not 'unavailable': %v", err)
	}
}
