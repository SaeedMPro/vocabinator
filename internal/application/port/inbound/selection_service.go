package inbound

import "context"

// SelectionSource says where captured text comes from.
type SelectionSource int

const (
	// SelectionPrimary is the highlighted text. Not available on all platforms.
	SelectionPrimary SelectionSource = iota
	// SelectionClipboard is text the user explicitly copied.
	SelectionClipboard
)

// SelectionService turns text selected in any desktop application into a
// vocabulary term, for use by lookup and save from every interface.
type SelectionService interface {
	CaptureTerm(ctx context.Context, source SelectionSource) (string, error)
}
