package inbound

import (
	"context"

	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

type SaveWordInput struct {
	Term           string
	SourceLanguage vocab.LanguageCode
	TargetLanguage vocab.LanguageCode

	// Context and SourceURL describe where the word was encountered.
	// When both are empty no occurrence is recorded.
	Context   string
	SourceURL string
}

type SaveWordResult struct {
	Item *vocab.VocabularyItem
	// Created is false when the word was already in the collection.
	Created bool
	// Occurrence is the newly recorded occurrence, or nil when none was added
	// (no context supplied, or the same context was already recorded).
	Occurrence *vocab.Occurrence
}

// SaveWordService is the "save this word" capability shared by all interfaces:
// get-or-create the item and record where it was seen.
type SaveWordService interface {
	SaveWord(ctx context.Context, input SaveWordInput) (*SaveWordResult, error)
}
