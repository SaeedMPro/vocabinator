package inbound

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

type VocabOccurrenceFilter struct {
    Context string
    SourceURL string

    CapturedFrom *time.Time
    CapturedTo   *time.Time
}

type VocabularyService interface {
    CreateVocabItem(
        ctx context.Context,
        input CreateVocabItemInput,
    ) (*vocab.VocabularyItem, error)

    GetVocabItem(
        ctx context.Context,
        id vocab.ID,
    ) (*vocab.VocabularyItem, error)

    ListVocabItems(
        ctx context.Context,
        filter VocabItemFilter,
    ) ([]*vocab.VocabularyItem, error)

    DeleteVocabItem(
        ctx context.Context,
        id vocab.ID,
    ) error

    AddVocabOccurrence(
        ctx context.Context,
        itemID vocab.ID,
        input AddVocabOccurrenceInput,
    ) (*vocab.Occurrence, error)

    ListVocabOccurrences(
        ctx context.Context,
        itemID vocab.ID,
        filter VocabOccurrenceFilter,
    ) ([]vocab.Occurrence, error)

    DeleteVocabOccurrence(
        ctx context.Context,
        itemID vocab.ID,
        occurrenceID vocab.ID,
    ) error
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