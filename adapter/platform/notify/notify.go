// Package notify shows short desktop notifications, used as the "small
// interface" for shortcut-triggered lookups where no terminal is visible.
//
//   - Linux: notify-send (libnotify).
//   - macOS: osascript, with title and body passed as arguments (never
//     interpolated into script text).
//   - Windows: not implemented; callers should fall back to other output.
package notify

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/SaeedMPro/vocabinator/adapter/platform/internal/proc"
)

// ErrUnavailable means notifications cannot be shown on this platform or the
// required tool is missing.
var ErrUnavailable = errors.New("notifications unavailable")

type Notifier struct {
	goos   string
	getenv func(string) string
	runner proc.Runner
}

func New() *Notifier {
	return &Notifier{goos: runtime.GOOS, getenv: os.Getenv, runner: proc.Exec{}}
}

// Notify shows a notification. title and body are plain text.
func (n *Notifier) Notify(ctx context.Context, title, body string) error {
	var name string
	var args []string

	switch n.goos {
	case "linux":
		if n.getenv("DISPLAY") == "" && n.getenv("WAYLAND_DISPLAY") == "" {
			return fmt.Errorf("%w: no graphical session", ErrUnavailable)
		}
		name = "notify-send"
		// Servers commonly interpret markup in the body, so escape it. "--"
		// stops text starting with "-" from being read as an option.
		args = []string{"--app-name=Vocabinator", "--expire-time=15000", "--", title, escapeMarkup(body)}
	case "darwin":
		name = "osascript"
		args = []string{
			"-e", "on run argv",
			"-e", "display notification (item 2 of argv) with title (item 1 of argv)",
			"-e", "end run",
			"--", title, body,
		}
	default:
		return fmt.Errorf("%w: not implemented on %s", ErrUnavailable, n.goos)
	}

	if _, err := n.runner.Run(ctx, name, args...); err != nil {
		if proc.IsNotInstalled(err) {
			return fmt.Errorf("%w: %s is not installed", ErrUnavailable, name)
		}
		return fmt.Errorf("show notification with %s: %w", name, err)
	}
	return nil
}

func escapeMarkup(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}
