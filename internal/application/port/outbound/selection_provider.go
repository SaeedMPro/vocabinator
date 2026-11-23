package outbound

import (
	"context"
	"errors"
)

// SelectionSource says which OS text buffer to read.
type SelectionSource int

const (
	// SelectionPrimary is the currently highlighted text (X11/Wayland primary
	// selection). It is not available on every platform.
	SelectionPrimary SelectionSource = iota
	// SelectionClipboard is the explicit clipboard (text the user copied).
	SelectionClipboard
)

// ErrSelectionUnavailable means the platform or its tooling cannot provide
// the requested source (missing tool, unsupported platform, no display).
var ErrSelectionUnavailable = errors.New("selection unavailable")

// SelectionProvider captures text from the desktop. Capture returns an empty
// string and no error when nothing is selected.
type SelectionProvider interface {
	Capture(ctx context.Context, source SelectionSource) (string, error)
}
