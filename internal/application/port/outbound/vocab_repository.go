package outbound

import (
	"context"
	"time"

	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

type VocabItemFilter struct {
	Term           string
	SourceLanguage *vocab.LanguageCode
	TargetLanguage *vocab.LanguageCode

	Limit  int
	Offset int
}

type VocabularyRepository interface {
	FindByID(
		ctx context.Context,
		id vocab.ID,
	) (*vocab.VocabularyItem, bool, error)

	FindByKey(
		ctx context.Context,
		key vocab.VocabularyKey,
	) (*vocab.VocabularyItem, bool, error)

	List(
		ctx context.Context,
		filter VocabItemFilter,
	) ([]*vocab.VocabularyItem, error)

	Upsert(
		ctx context.Context,
		item *vocab.VocabularyItem,
	) error

	Delete(
		ctx context.Context,
		id vocab.ID,
	) error
}

type VocabOccurrenceFilter struct {
	Context    string
	SourceURL  string
	CapturedAt time.Time
}

type VocabOccurrenceRepository interface {
	FindByID(
		ctx context.Context,
		id vocab.ID,
	) (*vocab.Occurrence, bool, error)

	List(
		ctx context.Context,
		filter VocabOccurrenceFilter,
	) ([]*vocab.Occurrence, error)

	Upsert(
		ctx context.Context,
		occurrence *vocab.Occurrence,
	) error
}
