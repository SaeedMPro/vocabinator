package outbound

import (
	"context"
	"errors"

	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

// ErrConflict is returned by Save when another item already owns the same
// vocabulary key (normalized term + language pair).
var ErrConflict = errors.New("vocabulary item conflict")

// VocabItemFilter narrows List results. Term matches the normalized term as a
// substring. A zero Limit means no limit.
type VocabItemFilter struct {
	Term           string
	SourceLanguage *vocab.LanguageCode
	TargetLanguage *vocab.LanguageCode

	Limit  int
	Offset int
}

// VocabularyRepository persists the VocabularyItem aggregate, including its
// occurrences. Save must write the item and its occurrences atomically.
type VocabularyRepository interface {
	FindByID(ctx context.Context, id vocab.ID) (*vocab.VocabularyItem, bool, error)
	FindByKey(ctx context.Context, key vocab.VocabularyKey) (*vocab.VocabularyItem, bool, error)
	List(ctx context.Context, filter VocabItemFilter) ([]*vocab.VocabularyItem, error)

	// Save inserts or replaces the aggregate.
	Save(ctx context.Context, item *vocab.VocabularyItem) error

	// Delete removes the aggregate and its occurrences. Deleting a missing ID is not an error.
	Delete(ctx context.Context, id vocab.ID) error
}
