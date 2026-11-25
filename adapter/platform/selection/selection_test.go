package selection

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/SaeedMPro/vocabinator/internal/application/port/outbound"
)

type result struct {
	out string
	err error
}

// fakeRunner returns scripted results keyed by the full command line and
// records every call in order.
type fakeRunner struct {
	results map[string]result
	calls   []string
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	key := strings.Join(append([]string{name}, args...), " ")
	f.calls = append(f.calls, key)
	r, ok := f.results[key]
	if !ok {
		return nil, fmt.Errorf("run %s: %w", name, exec.ErrNotFound)
	}
	return []byte(r.out), r.err
}

func newProvider(goos string, env map[string]string, results map[string]result) (*Provider, *fakeRunner) {
	r := &fakeRunner{results: results}
	return &Provider{goos: goos, getenv: func(k string) string { return env[k] }, runner: r}, r
}

var errExit = errors.New("exit status 1")

func TestX11Primary(t *testing.T) {
	p, r := newProvider("linux", map[string]string{"DISPLAY": ":0"}, map[string]result{
		"xclip -o -selection primary": {out: "robust\n"},
	})
	got, err := p.Capture(context.Background(), outbound.SelectionPrimary)
	if err != nil || got != "robust" {
		t.Errorf("got %q, %v", got, err)
	}
	if len(r.calls) != 1 {
		t.Errorf("should stop at first success, calls=%v", r.calls)
	}
}

func TestX11FallsBackToXselWhenXclipMissing(t *testing.T) {
	p, r := newProvider("linux", map[string]string{"DISPLAY": ":0"}, map[string]result{
		"xsel --primary --output": {out: "robust"},
	})
	got, err := p.Capture(context.Background(), outbound.SelectionPrimary)
	if err != nil || got != "robust" || len(r.calls) != 2 {
		t.Errorf("got %q, %v, calls=%v", got, err, r.calls)
	}
}

func TestWaylandOrderAndXWaylandFallback(t *testing.T) {
	env := map[string]string{"XDG_SESSION_TYPE": "wayland", "WAYLAND_DISPLAY": "wayland-0", "DISPLAY": ":0"}

	p, r := newProvider("linux", env, map[string]result{
		"wl-paste --primary --no-newline": {err: errExit}, // compositor without data-control
		"xclip -o -selection primary":     {out: "robust"},
	})
	got, err := p.Capture(context.Background(), outbound.SelectionPrimary)
	if err != nil || got != "robust" {
		t.Fatalf("got %q, %v", got, err)
	}
	if r.calls[0] != "wl-paste --primary --no-newline" {
		t.Errorf("wl-paste must be tried first on Wayland, calls=%v", r.calls)
	}

	// Pure Wayland (no DISPLAY): X tools must never be tried.
	p, r = newProvider("linux", map[string]string{"WAYLAND_DISPLAY": "wayland-0"}, map[string]result{
		"wl-paste --primary --no-newline": {out: "robust"},
	})
	if got, err := p.Capture(context.Background(), outbound.SelectionPrimary); err != nil || got != "robust" {
		t.Errorf("got %q, %v", got, err)
	}
	for _, c := range r.calls {
		if strings.HasPrefix(c, "xclip") || strings.HasPrefix(c, "xsel") {
			t.Errorf("X tool called on pure Wayland: %v", r.calls)
		}
	}
}

func TestClipboardIsNeverAFallbackForPrimary(t *testing.T) {
	p, r := newProvider("linux", map[string]string{"DISPLAY": ":0"}, map[string]result{
		"xclip -o -selection primary":   {err: errExit},
		"xclip -o -selection clipboard": {out: "stale clipboard word"},
	})
	got, err := p.Capture(context.Background(), outbound.SelectionPrimary)
	if err != nil || got != "" {
		t.Errorf("expected empty selection, got %q, %v", got, err)
	}
	for _, c := range r.calls {
		if strings.Contains(c, "clipboard") {
			t.Errorf("clipboard read for a primary request: %v", r.calls)
		}
	}
}

func TestClipboardSourceReadsClipboard(t *testing.T) {
	p, _ := newProvider("linux", map[string]string{"DISPLAY": ":0"}, map[string]result{
		"xclip -o -selection clipboard": {out: "copied"},
	})
	if got, err := p.Capture(context.Background(), outbound.SelectionClipboard); err != nil || got != "copied" {
		t.Errorf("got %q, %v", got, err)
	}

	p, _ = newProvider("linux", map[string]string{"WAYLAND_DISPLAY": "w"}, map[string]result{
		"wl-paste --no-newline": {out: "copied"},
	})
	if got, err := p.Capture(context.Background(), outbound.SelectionClipboard); err != nil || got != "copied" {
		t.Errorf("wayland: got %q, %v", got, err)
	}
}

func TestNothingSelectedIsNotAnError(t *testing.T) {
	p, _ := newProvider("linux", map[string]string{"DISPLAY": ":0"}, map[string]result{
		"xclip -o -selection primary": {out: "  \n"},
	})
	if got, err := p.Capture(context.Background(), outbound.SelectionPrimary); err != nil || got != "" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestMissingToolsAreReportedWithHint(t *testing.T) {
	cases := map[string]struct {
		env  map[string]string
		hint string
	}{
		"x11":     {map[string]string{"DISPLAY": ":0"}, "install xclip or xsel"},
		"wayland": {map[string]string{"WAYLAND_DISPLAY": "w"}, "wl-clipboard"},
	}
	for name, tc := range cases {
		p, _ := newProvider("linux", tc.env, nil)
		_, err := p.Capture(context.Background(), outbound.SelectionPrimary)
		if !errors.Is(err, outbound.ErrSelectionUnavailable) || !strings.Contains(err.Error(), tc.hint) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

func TestNoGraphicalSession(t *testing.T) {
	p, r := newProvider("linux", nil, nil)
	if _, err := p.Capture(context.Background(), outbound.SelectionPrimary); !errors.Is(err, outbound.ErrSelectionUnavailable) || len(r.calls) != 0 {
		t.Errorf("err=%v calls=%v", err, r.calls)
	}
}

func TestMacAndWindowsOnlyOfferTheClipboard(t *testing.T) {
	for goos, cmd := range map[string]string{
		"darwin":  "pbpaste",
		"windows": "powershell -NoProfile -NonInteractive -Command Get-Clipboard -Raw",
	} {
		p, r := newProvider(goos, nil, map[string]result{cmd: {out: "copied\r\n"}})

		if _, err := p.Capture(context.Background(), outbound.SelectionPrimary); !errors.Is(err, outbound.ErrSelectionUnavailable) || !strings.Contains(err.Error(), "clipboard") || len(r.calls) != 0 {
			t.Errorf("%s primary: err=%v calls=%v", goos, err, r.calls)
		}
		if got, err := p.Capture(context.Background(), outbound.SelectionClipboard); err != nil || got != "copied" {
			t.Errorf("%s clipboard: %q, %v", goos, got, err)
		}
	}
}

func TestUnsupportedPlatform(t *testing.T) {
	p, _ := newProvider("plan9", nil, nil)
	if _, err := p.Capture(context.Background(), outbound.SelectionClipboard); !errors.Is(err, outbound.ErrSelectionUnavailable) {
		t.Errorf("got %v", err)
	}
}

func TestCancelledContextIsReturned(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p, _ := newProvider("linux", map[string]string{"DISPLAY": ":0"}, map[string]result{
		"xclip -o -selection primary": {err: errExit},
	})
	if _, err := p.Capture(ctx, outbound.SelectionPrimary); !errors.Is(err, context.Canceled) {
		t.Errorf("got %v", err)
	}
}
