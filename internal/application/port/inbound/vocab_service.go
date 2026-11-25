package inbound

import (
	"context"
	"time"

	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

// VocabItemFilter narrows ListVocabItems. Term matches as a case-insensitive
// substring. A zero Limit means no limit.
type VocabItemFilter struct {
	Term           string
	SourceLanguage *vocab.LanguageCode
	TargetLanguage *vocab.LanguageCode

	Limit  int
	Offset int
}

// VocabOccurrenceFilter narrows ListVocabOccurrences. Context and SourceURL
// match as case-insensitive substrings; the time bounds are inclusive.
type VocabOccurrenceFilter struct {
	Context   string
	SourceURL string

	CapturedFrom *time.Time
	CapturedTo   *time.Time
}

// VocabItemKey identifies an item by its natural identity.
type VocabItemKey struct {
	Term           string
	SourceLanguage vocab.LanguageCode
	TargetLanguage vocab.LanguageCode
}

type CreateVocabItemInput struct {
	Term           string
	SourceLanguage vocab.LanguageCode
	TargetLanguage vocab.LanguageCode
}

type AddVocabOccurrenceInput struct {
	Context   string
	SourceURL string
}

// VocabularyService is the vocabulary-management capability exposed to all
// inbound adapters (CLI, web, shortcuts, workers).
type VocabularyService interface {
	CreateVocabItem(ctx context.Context, input CreateVocabItemInput) (*vocab.VocabularyItem, error)
	GetVocabItem(ctx context.Context, id vocab.ID) (*vocab.VocabularyItem, error)
	// FindVocabItem resolves an item by term and language pair, returning ErrItemNotFound if absent.
	FindVocabItem(ctx context.Context, key VocabItemKey) (*vocab.VocabularyItem, error)
	ListVocabItems(ctx context.Context, filter VocabItemFilter) ([]*vocab.VocabularyItem, error)
	DeleteVocabItem(ctx context.Context, id vocab.ID) error

	AddVocabOccurrence(ctx context.Context, itemID vocab.ID, input AddVocabOccurrenceInput) (*vocab.Occurrence, error)
	ListVocabOccurrences(ctx context.Context, itemID vocab.ID, filter VocabOccurrenceFilter) ([]vocab.Occurrence, error)
	DeleteVocabOccurrence(ctx context.Context, itemID vocab.ID, occurrenceID vocab.ID) error
}
