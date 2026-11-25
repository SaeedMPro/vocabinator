// Package selection captures selected or copied text from the desktop by
// calling standard platform tools. There is no portable API, so each platform
// is handled separately and honestly:
//
//   - Linux X11: primary selection via xclip or xsel.
//   - Linux Wayland: primary selection via wl-paste (needs a compositor that
//     exposes the data-control protocol; GNOME does not, see docs/shortcuts.md),
//     then xclip/xsel for XWayland applications.
//   - macOS and Windows: no primary selection exists, so only the clipboard is
//     offered (pbpaste, PowerShell Get-Clipboard). Capturing the highlighted
//     text there would require synthesising a copy keystroke, which needs
//     macOS Accessibility permission and is not implemented.
//
// The clipboard is never used as a silent fallback for the primary selection:
// stale clipboard content would look up the wrong word.
package selection

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/SaeedMPro/vocabinator/adapter/platform/internal/proc"
	"github.com/SaeedMPro/vocabinator/internal/application/port/outbound"
)

type command struct {
	name string
	args []string
}

func (c command) String() string { return strings.Join(append([]string{c.name}, c.args...), " ") }

// Provider implements outbound.SelectionProvider.
type Provider struct {
	goos   string
	getenv func(string) string
	runner proc.Runner
}

var _ outbound.SelectionProvider = (*Provider)(nil)

// New returns a Provider for the running platform.
func New() *Provider {
	return &Provider{goos: runtime.GOOS, getenv: os.Getenv, runner: proc.Exec{}}
}

func (p *Provider) Capture(ctx context.Context, source outbound.SelectionSource) (string, error) {
	candidates, err := p.candidates(source)
	if err != nil {
		return "", err
	}

	var missing []string
	ran := false

	for _, c := range candidates {
		out, err := p.runner.Run(ctx, c.name, c.args...)
		switch {
		case err == nil:
			ran = true
			if text := strings.TrimSpace(string(out)); text != "" {
				return text, nil
			}
		case proc.IsNotInstalled(err):
			missing = append(missing, c.name)
		case ctx.Err() != nil:
			return "", ctx.Err()
		default:
			// The tool exists but failed: typically "nothing selected".
			ran = true
		}
	}

	if ran {
		return "", nil
	}
	return "", fmt.Errorf("%w: none of the required tools is installed (%s); %s",
		outbound.ErrSelectionUnavailable, strings.Join(dedupe(missing), ", "), p.installHint())
}

func (p *Provider) candidates(source outbound.SelectionSource) ([]command, error) {
	switch p.goos {
	case "linux":
		return p.linuxCandidates(source)
	case "darwin":
		if source == outbound.SelectionClipboard {
			return []command{{name: "pbpaste"}}, nil
		}
		return nil, noPrimary("macOS")
	case "windows":
		if source == outbound.SelectionClipboard {
			return []command{{name: "powershell", args: []string{"-NoProfile", "-NonInteractive", "-Command", "Get-Clipboard -Raw"}}}, nil
		}
		return nil, noPrimary("Windows")
	default:
		return nil, fmt.Errorf("%w: unsupported platform %q", outbound.ErrSelectionUnavailable, p.goos)
	}
}

func noPrimary(platform string) error {
	return fmt.Errorf("%w: %s has no primary selection; copy the text (Ctrl or Cmd+C) and use the clipboard option instead",
		outbound.ErrSelectionUnavailable, platform)
}

func (p *Provider) linuxCandidates(source outbound.SelectionSource) ([]command, error) {
	wayland := p.getenv("XDG_SESSION_TYPE") == "wayland" || p.getenv("WAYLAND_DISPLAY") != ""
	x11 := p.getenv("DISPLAY") != ""

	if !wayland && !x11 {
		return nil, fmt.Errorf("%w: no graphical session found (neither WAYLAND_DISPLAY nor DISPLAY is set)", outbound.ErrSelectionUnavailable)
	}

	primary := source == outbound.SelectionPrimary
	var cs []command

	if wayland {
		if primary {
			cs = append(cs, command{"wl-paste", []string{"--primary", "--no-newline"}})
		} else {
			cs = append(cs, command{"wl-paste", []string{"--no-newline"}})
		}
	}
	// X11 sessions, and XWayland clients under Wayland.
	if x11 {
		if primary {
			cs = append(cs,
				command{"xclip", []string{"-o", "-selection", "primary"}},
				command{"xsel", []string{"--primary", "--output"}})
		} else {
			cs = append(cs,
				command{"xclip", []string{"-o", "-selection", "clipboard"}},
				command{"xsel", []string{"--clipboard", "--output"}})
		}
	}
	return cs, nil
}

func (p *Provider) installHint() string {
	if p.goos != "linux" {
		return "see docs/shortcuts.md"
	}
	if p.getenv("XDG_SESSION_TYPE") == "wayland" || p.getenv("WAYLAND_DISPLAY") != "" {
		return "install wl-clipboard (wl-paste); XWayland apps also work with xclip"
	}
	return "install xclip or xsel"
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
