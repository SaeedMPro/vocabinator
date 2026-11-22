package vocab

import (
    "strings"
    "time"
)

type VocabularyKey struct {
    NormalizedTerm string
    SourceLanguage LanguageCode
    TargetLanguage LanguageCode
}

type VocabularyItem struct {
    id             ID
    term           string
    normalizedTerm string

    sourceLanguage LanguageCode
    targetLanguage LanguageCode

    occurrences []Occurrence

    createdAt time.Time
    updatedAt time.Time
}

func NewVocabularyItem(
    id ID,
    term string,
    sourceLanguage LanguageCode,
    targetLanguage LanguageCode,
    createdAt time.Time,
) (*VocabularyItem, error) {

	id = ID(strings.TrimSpace(string(id)))
	term = strings.TrimSpace(term)
	sourceLanguage = LanguageCode(strings.TrimSpace(string(sourceLanguage)))
	targetLanguage = LanguageCode(strings.TrimSpace(string(targetLanguage)))

    if id == "" {
        return nil, ErrInvalidID
    }

    if term == "" {
        return nil, ErrEmptyTerm
    }

    if createdAt.IsZero() {
        return nil, ErrInvalidTimestamp
    }

    now := createdAt.UTC()

    return &VocabularyItem{
        id:             id,
        term:           term,
        normalizedTerm: normalizeTerm(term),

        sourceLanguage: sourceLanguage,
        targetLanguage: targetLanguage,

        occurrences: make([]Occurrence, 0),

        createdAt: now,
        updatedAt: now,
    }, nil
}

func normalizeTerm(term string) string {
    return strings.ToLower(
        strings.Join(strings.Fields(term), " "),
    )
}

// Getters

func (v VocabularyItem) ID() ID {
    return v.id
}

func (v VocabularyItem) Term() string {
    return v.term
}

func (v VocabularyItem) NormalizedTerm() string {
    return v.normalizedTerm
}

func (v VocabularyItem) SourceLanguage() LanguageCode {
    return v.sourceLanguage
}

func (v VocabularyItem) TargetLanguage() LanguageCode {
    return v.targetLanguage
}

func (v VocabularyItem) CreatedAt() time.Time {
    return v.createdAt
}

func (v VocabularyItem) UpdatedAt() time.Time {
    return v.updatedAt
}

func (v VocabularyItem) Key() VocabularyKey {
    return VocabularyKey{
        NormalizedTerm: v.normalizedTerm,
        SourceLanguage: v.sourceLanguage,
        TargetLanguage: v.targetLanguage,
    }
}

func (v VocabularyItem) Occurrences() []Occurrence {
    result := make([]Occurrence, len(v.occurrences))
    copy(result, v.occurrences)
    return result
}

// AddOccurrence adds a new occurrence to the vocabulary item.

func (v *VocabularyItem) AddOccurrence(o Occurrence) error {
    if o.vocabularyItemID != v.id {
        return ErrOccurrenceItemMismatch
    }

    for _, existing := range v.occurrences {
        if existing.id == o.id {
            return ErrDuplicateOccurrence
        }
    }

    v.occurrences = append(v.occurrences, o)

    if o.capturedAt.After(v.updatedAt) {
        v.updatedAt = o.capturedAt
    }

    return nil
}