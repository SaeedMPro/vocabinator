package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/SaeedMPro/vocabinator/internal/application/port/inbound"
	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

type saveWordService struct {
	vocabulary inbound.VocabularyService
}

var _ inbound.SaveWordService = (*saveWordService)(nil)

func NewSaveWordService(vocabulary inbound.VocabularyService) inbound.SaveWordService {
	return &saveWordService{vocabulary: vocabulary}
}

func (s *saveWordService) SaveWord(ctx context.Context, input inbound.SaveWordInput) (*inbound.SaveWordResult, error) {
	key := inbound.VocabItemKey{Term: input.Term, SourceLanguage: input.SourceLanguage, TargetLanguage: input.TargetLanguage}
	result := &inbound.SaveWordResult{}

	item, err := s.vocabulary.CreateVocabItem(ctx, inbound.CreateVocabItemInput(key))
	switch {
	case err == nil:
		result.Created = true
	case errors.Is(err, ErrItemExists):
		if item == nil { // lost a creation race: the winner's item is in the store
			if item, err = s.vocabulary.FindVocabItem(ctx, key); err != nil {
				return nil, err
			}
		}
	default:
		return nil, err
	}

	text := strings.TrimSpace(input.Context)
	sourceURL := strings.TrimSpace(input.SourceURL)

	if (text != "" || sourceURL != "") && !hasOccurrence(item, text, sourceURL) {
		occurrence, err := s.vocabulary.AddVocabOccurrence(ctx, item.ID(), inbound.AddVocabOccurrenceInput{
			Context: text, SourceURL: sourceURL,
		})
		if err != nil {
			return nil, fmt.Errorf("record occurrence: %w", err)
		}
		result.Occurrence = occurrence

		// The aggregate changed; return its current state.
		if item, err = s.vocabulary.GetVocabItem(ctx, item.ID()); err != nil {
			return nil, err
		}
	}

	result.Item = item
	return result, nil
}

// hasOccurrence reports whether the item already records this exact
// context and URL, which makes repeated saves idempotent.
func hasOccurrence(item *vocab.VocabularyItem, text, sourceURL string) bool {
	for _, o := range item.Occurrences() {
		if o.Context() == text && o.SourceURL() == sourceURL {
			return true
		}
	}
	return false
}
