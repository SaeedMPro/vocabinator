package vocab

import (
	"strings"
	"time"
)

// VocabularyKey is the natural identity of a vocabulary item:
// normalized term + source language + target language.
type VocabularyKey struct {
	NormalizedTerm string
	SourceLanguage LanguageCode
	TargetLanguage LanguageCode
}

// NewVocabularyKey validates and normalizes the parts of a vocabulary identity.
func NewVocabularyKey(term string, source, target LanguageCode) (VocabularyKey, error) {
	normalized := normalizeTerm(term)
	if normalized == "" {
		return VocabularyKey{}, ErrEmptyTerm
	}

	source, err := NewLanguageCode(string(source))
	if err != nil {
		return VocabularyKey{}, err
	}

	target, err = NewLanguageCode(string(target))
	if err != nil {
		return VocabularyKey{}, err
	}

	return VocabularyKey{
		NormalizedTerm: normalized,
		SourceLanguage: source,
		TargetLanguage: target,
	}, nil
}

// VocabularyItem is the aggregate root. Occurrences belong to it and can only
// be changed through its methods.
type VocabularyItem struct {
	id             ID
	term           string
	normalizedTerm string

	sourceLanguage LanguageCode
	targetLanguage LanguageCode

	occurrences []Occurrence

	createdAt time.Time
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

	if id == "" {
		return nil, ErrInvalidID
	}

	key, err := NewVocabularyKey(term, sourceLanguage, targetLanguage)
	if err != nil {
		return nil, err
	}

	if createdAt.IsZero() {
		return nil, ErrInvalidTimestamp
	}

	return &VocabularyItem{
		id:             id,
		term:           term,
		normalizedTerm: key.NormalizedTerm,
		sourceLanguage: key.SourceLanguage,
		targetLanguage: key.TargetLanguage,
		occurrences:    make([]Occurrence, 0),
		createdAt:      createdAt.UTC(),
	}, nil
}

// RestoreVocabularyItem rebuilds an aggregate from persisted state. It runs the
// same invariants as creation and AddOccurrence, so corrupt data is rejected.
func RestoreVocabularyItem(
	id ID,
	term string,
	sourceLanguage LanguageCode,
	targetLanguage LanguageCode,
	createdAt time.Time,
	occurrences []Occurrence,
) (*VocabularyItem, error) {
	item, err := NewVocabularyItem(id, term, sourceLanguage, targetLanguage, createdAt)
	if err != nil {
		return nil, err
	}

	for _, o := range occurrences {
		if err := item.AddOccurrence(o); err != nil {
			return nil, err
		}
	}

	return item, nil
}

func normalizeTerm(term string) string {
	return strings.ToLower(strings.Join(strings.Fields(term), " "))
}

// Getters

func (v VocabularyItem) ID() ID                       { return v.id }
func (v VocabularyItem) Term() string                 { return v.term }
func (v VocabularyItem) NormalizedTerm() string       { return v.normalizedTerm }
func (v VocabularyItem) SourceLanguage() LanguageCode { return v.sourceLanguage }
func (v VocabularyItem) TargetLanguage() LanguageCode { return v.targetLanguage }
func (v VocabularyItem) CreatedAt() time.Time         { return v.createdAt }

func (v VocabularyItem) Key() VocabularyKey {
	return VocabularyKey{
		NormalizedTerm: v.normalizedTerm,
		SourceLanguage: v.sourceLanguage,
		TargetLanguage: v.targetLanguage,
	}
}

// Occurrences returns a copy, so callers cannot mutate aggregate state.
func (v VocabularyItem) Occurrences() []Occurrence {
	result := make([]Occurrence, len(v.occurrences))
	copy(result, v.occurrences)
	return result
}

// AddOccurrence attaches an occurrence to the item.
func (v *VocabularyItem) AddOccurrence(o Occurrence) error {
	if o.id == "" {
		return ErrInvalidID
	}

	if o.vocabularyItemID != v.id {
		return ErrOccurrenceItemMismatch
	}

	for _, existing := range v.occurrences {
		if existing.id == o.id {
			return ErrDuplicateOccurrence
		}
	}

	v.occurrences = append(v.occurrences, o)
	return nil
}

// RemoveOccurrence removes the occurrence with the given ID.
func (v *VocabularyItem) RemoveOccurrence(id ID) error {
	for i, existing := range v.occurrences {
		if existing.id == id {
			v.occurrences = append(v.occurrences[:i], v.occurrences[i+1:]...)
			return nil
		}
	}

	return ErrOccurrenceNotFound
}
