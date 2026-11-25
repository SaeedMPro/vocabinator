package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SaeedMPro/vocabinator/internal/application/port/inbound"
	"github.com/SaeedMPro/vocabinator/internal/application/port/outbound"
	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

type fakeSelection struct {
	text   string
	err    error
	source outbound.SelectionSource
}

func (f *fakeSelection) Capture(_ context.Context, src outbound.SelectionSource) (string, error) {
	f.source = src
	return f.text, f.err
}

func TestCaptureTerm(t *testing.T) {
	f := &fakeSelection{text: "  “Robust,” \n"}
	term, err := NewSelectionService(f).CaptureTerm(context.Background(), inbound.SelectionPrimary)
	if err != nil || term != "Robust" || f.source != outbound.SelectionPrimary {
		t.Errorf("term=%q err=%v source=%v", term, err, f.source)
	}

	if _, err := NewSelectionService(f).CaptureTerm(context.Background(), inbound.SelectionClipboard); err != nil || f.source != outbound.SelectionClipboard {
		t.Errorf("clipboard source not forwarded: err=%v source=%v", err, f.source)
	}
}

func TestCaptureTermErrors(t *testing.T) {
	ctx := context.Background()

	if _, err := NewSelectionService(&fakeSelection{text: "  "}).CaptureTerm(ctx, inbound.SelectionPrimary); !errors.Is(err, ErrNoSelection) {
		t.Errorf("empty: expected ErrNoSelection, got %v", err)
	}

	_, err := NewSelectionService(&fakeSelection{text: strings.Repeat("word ", 30)}).CaptureTerm(ctx, inbound.SelectionPrimary)
	if !errors.Is(err, ErrInvalidInput) || !errors.Is(err, vocab.ErrSelectionTooLong) {
		t.Errorf("long: got %v", err)
	}

	_, err = NewSelectionService(&fakeSelection{err: outbound.ErrSelectionUnavailable}).CaptureTerm(ctx, inbound.SelectionPrimary)
	if !errors.Is(err, outbound.ErrSelectionUnavailable) {
		t.Errorf("provider error must propagate, got %v", err)
	}

	if _, err := NewSelectionService(&fakeSelection{text: "x"}).CaptureTerm(ctx, inbound.SelectionSource(99)); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("unknown source: got %v", err)
	}
}
