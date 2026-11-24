package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/SaeedMPro/vocabinator/internal/application/port/inbound"
	"github.com/SaeedMPro/vocabinator/internal/application/port/outbound"
	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

const (
	maxTermRunes = 100
	// maxTranslatedDefinitions bounds translation cost per lookup. Free
	// providers meter characters per day, so only the first definitions are
	// translated; all definitions are still returned.
	maxTranslatedDefinitions = 5
)

type lookupService struct {
	dictionary outbound.DictionaryProvider
	translator outbound.TranslationProvider
}

var _ inbound.LookupService = (*lookupService)(nil)

func NewLookupService(dictionary outbound.DictionaryProvider, translator outbound.TranslationProvider) inbound.LookupService {
	return &lookupService{dictionary: dictionary, translator: translator}
}

func (s *lookupService) LookupWord(ctx context.Context, input inbound.LookupWordInput) (*inbound.LookupResult, error) {
	key, err := vocab.NewVocabularyKey(input.Term, input.SourceLanguage, input.TargetLanguage)
	if err != nil {
		return nil, invalidInput(err)
	}
	term := strings.Join(strings.Fields(input.Term), " ")
	if utf8.RuneCountInString(term) > maxTermRunes {
		return nil, fmt.Errorf("%w: term longer than %d characters", ErrInvalidInput, maxTermRunes)
	}

	entries, err := s.dictionary.Lookup(ctx, term, key.SourceLanguage)
	if err != nil {
		if errors.Is(err, outbound.ErrNotFound) {
			return nil, fmt.Errorf("%w: %q: %w", ErrWordNotFound, term, err)
		}
		return nil, fmt.Errorf("look up dictionary entry: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrWordNotFound, term)
	}

	result := mergeEntries(term, entries)

	if key.SourceLanguage != key.TargetLanguage {
		if err := s.translate(ctx, result, key); err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			result.Warnings = append(result.Warnings, fmt.Sprintf("translation unavailable: %v", err))
		}
	}

	return result, nil
}

// translate fills each definition's Translation. Definitions are translated,
// never the bare headword: an isolated word is ambiguous ("run") and
// translates poorly, while a definition carries its own sense.
func (s *lookupService) translate(ctx context.Context, r *inbound.LookupResult, key vocab.VocabularyKey) error {
	var texts []string
	var targets []*inbound.DefinitionResult

	for i := range r.Senses {
		for j := range r.Senses[i].Definitions {
			if len(targets) == maxTranslatedDefinitions {
				break
			}
			d := &r.Senses[i].Definitions[j]
			texts = append(texts, d.Text)
			targets = append(targets, d)
		}
	}

	if len(texts) == 0 {
		return nil
	}

	out, err := s.translator.Translate(ctx, texts, key.SourceLanguage, key.TargetLanguage)
	if err != nil {
		return err
	}
	if len(out) != len(texts) {
		return fmt.Errorf("translator returned %d translations for %d texts", len(out), len(texts))
	}

	for i, d := range targets {
		d.Translation = out[i]
	}
	return nil
}

// mergeEntries combines homograph entries into one result, preserving order.
func mergeEntries(term string, entries []outbound.DictionaryEntry) *inbound.LookupResult {
	result := &inbound.LookupResult{Term: term}

	for _, e := range entries {
		if result.Phonetic == "" {
			result.Phonetic = e.Phonetic
		}
		for _, sense := range e.Senses {
			sr := inbound.SenseResult{
				PartOfSpeech: sense.PartOfSpeech,
				Examples:     sense.Examples,
				Synonyms:     sense.Synonyms,
				Antonyms:     sense.Antonyms,
			}
			for _, d := range sense.Definitions {
				sr.Definitions = append(sr.Definitions, inbound.DefinitionResult{Text: d})
			}
			result.Senses = append(result.Senses, sr)
		}
	}

	return result
}
