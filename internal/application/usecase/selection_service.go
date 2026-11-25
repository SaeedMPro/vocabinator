package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/SaeedMPro/vocabinator/internal/application/port/inbound"
	"github.com/SaeedMPro/vocabinator/internal/application/port/outbound"
	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

type selectionService struct {
	provider outbound.SelectionProvider
}

var _ inbound.SelectionService = (*selectionService)(nil)

func NewSelectionService(provider outbound.SelectionProvider) inbound.SelectionService {
	return &selectionService{provider: provider}
}

func (s *selectionService) CaptureTerm(ctx context.Context, source inbound.SelectionSource) (string, error) {
	var src outbound.SelectionSource
	switch source {
	case inbound.SelectionPrimary:
		src = outbound.SelectionPrimary
	case inbound.SelectionClipboard:
		src = outbound.SelectionClipboard
	default:
		return "", fmt.Errorf("%w: unknown selection source %d", ErrInvalidInput, source)
	}

	raw, err := s.provider.Capture(ctx, src)
	if err != nil {
		return "", fmt.Errorf("capture selection: %w", err)
	}

	term, err := vocab.TermFromSelection(raw)
	switch {
	case errors.Is(err, vocab.ErrEmptyTerm):
		return "", ErrNoSelection
	case err != nil:
		return "", invalidInput(err)
	}
	return term, nil
}
