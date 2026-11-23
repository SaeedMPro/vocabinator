package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/SaeedMPro/vocabinator/internal/application/port/inbound"
	"github.com/SaeedMPro/vocabinator/internal/application/port/outbound"
	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

type vocabularyService struct {
	repository outbound.VocabularyRepository
	clock      outbound.Clock
	ids        outbound.IDGenerator
}

var _ inbound.VocabularyService = (*vocabularyService)(nil)

func NewVocabularyService(
	repository outbound.VocabularyRepository,
	clock outbound.Clock,
	ids outbound.IDGenerator,
) inbound.VocabularyService {
	return &vocabularyService{repository: repository, clock: clock, ids: ids}
}

func (s *vocabularyService) CreateVocabItem(ctx context.Context, input inbound.CreateVocabItemInput) (*vocab.VocabularyItem, error) {
	key, err := vocab.NewVocabularyKey(input.Term, input.SourceLanguage, input.TargetLanguage)
	if err != nil {
		return nil, invalidInput(err)
	}

	existing, found, err := s.repository.FindByKey(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("find vocabulary item by key: %w", err)
	}
	if found {
		return existing, fmt.Errorf("%w: %q (%s -> %s)", ErrItemExists, existing.Term(), key.SourceLanguage, key.TargetLanguage)
	}

	item, err := vocab.NewVocabularyItem(s.ids.NewID(), input.Term, key.SourceLanguage, key.TargetLanguage, s.clock.Now())
	if err != nil {
		return nil, invalidInput(err)
	}

	if err := s.repository.Save(ctx, item); err != nil {
		if errors.Is(err, outbound.ErrConflict) {
			// Lost a race with a concurrent create of the same key.
			return nil, fmt.Errorf("%w: %q: %w", ErrItemExists, item.Term(), err)
		}
		return nil, fmt.Errorf("save vocabulary item: %w", err)
	}

	return item, nil
}

func (s *vocabularyService) GetVocabItem(ctx context.Context, id vocab.ID) (*vocab.VocabularyItem, error) {
	return s.load(ctx, id)
}

func (s *vocabularyService) ListVocabItems(ctx context.Context, filter inbound.VocabItemFilter) ([]*vocab.VocabularyItem, error) {
	if filter.Limit < 0 || filter.Offset < 0 {
		return nil, fmt.Errorf("%w: limit and offset must not be negative", ErrInvalidFilter)
	}

	repoFilter := outbound.VocabItemFilter{
		Term:   strings.ToLower(strings.Join(strings.Fields(filter.Term), " ")),
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}

	var err error
	if repoFilter.SourceLanguage, err = validLanguage(filter.SourceLanguage); err != nil {
		return nil, fmt.Errorf("%w: source language: %w", ErrInvalidFilter, err)
	}
	if repoFilter.TargetLanguage, err = validLanguage(filter.TargetLanguage); err != nil {
		return nil, fmt.Errorf("%w: target language: %w", ErrInvalidFilter, err)
	}

	items, err := s.repository.List(ctx, repoFilter)
	if err != nil {
		return nil, fmt.Errorf("list vocabulary items: %w", err)
	}

	return items, nil
}

func (s *vocabularyService) DeleteVocabItem(ctx context.Context, id vocab.ID) error {
	if _, err := s.load(ctx, id); err != nil {
		return err
	}

	if err := s.repository.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete vocabulary item: %w", err)
	}

	return nil
}

func (s *vocabularyService) AddVocabOccurrence(ctx context.Context, itemID vocab.ID, input inbound.AddVocabOccurrenceInput) (*vocab.Occurrence, error) {
	item, err := s.load(ctx, itemID)
	if err != nil {
		return nil, err
	}

	occurrence, err := vocab.NewOccurrence(s.ids.NewID(), item.ID(), input.Context, input.SourceURL, s.clock.Now())
	if err != nil {
		return nil, invalidInput(err)
	}

	if err := item.AddOccurrence(occurrence); err != nil {
		return nil, invalidInput(err)
	}

	if err := s.repository.Save(ctx, item); err != nil {
		return nil, fmt.Errorf("save vocabulary item: %w", err)
	}

	return &occurrence, nil
}

func (s *vocabularyService) ListVocabOccurrences(ctx context.Context, itemID vocab.ID, filter inbound.VocabOccurrenceFilter) ([]vocab.Occurrence, error) {
	if filter.CapturedFrom != nil && filter.CapturedTo != nil && filter.CapturedFrom.After(*filter.CapturedTo) {
		return nil, fmt.Errorf("%w: captured-from is after captured-to", ErrInvalidFilter)
	}

	item, err := s.load(ctx, itemID)
	if err != nil {
		return nil, err
	}

	result := make([]vocab.Occurrence, 0)
	for _, o := range item.Occurrences() {
		if matchesOccurrence(o, filter) {
			result = append(result, o)
		}
	}

	return result, nil
}

func (s *vocabularyService) DeleteVocabOccurrence(ctx context.Context, itemID vocab.ID, occurrenceID vocab.ID) error {
	item, err := s.load(ctx, itemID)
	if err != nil {
		return err
	}

	if err := item.RemoveOccurrence(occurrenceID); err != nil {
		if errors.Is(err, vocab.ErrOccurrenceNotFound) {
			return fmt.Errorf("%w: %s", ErrOccurrenceNotFound, occurrenceID)
		}
		return invalidInput(err)
	}

	if err := s.repository.Save(ctx, item); err != nil {
		return fmt.Errorf("save vocabulary item: %w", err)
	}

	return nil
}

func (s *vocabularyService) load(ctx context.Context, id vocab.ID) (*vocab.VocabularyItem, error) {
	if strings.TrimSpace(string(id)) == "" {
		return nil, invalidInput(vocab.ErrInvalidID)
	}

	item, found, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find vocabulary item: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("%w: %s", ErrItemNotFound, id)
	}

	return item, nil
}

func invalidInput(err error) error {
	return fmt.Errorf("%w: %w", ErrInvalidInput, err)
}

func validLanguage(code *vocab.LanguageCode) (*vocab.LanguageCode, error) {
	if code == nil {
		return nil, nil
	}

	valid, err := vocab.NewLanguageCode(string(*code))
	if err != nil {
		return nil, err
	}

	return &valid, nil
}

func matchesOccurrence(o vocab.Occurrence, f inbound.VocabOccurrenceFilter) bool {
	if f.Context != "" && !strings.Contains(strings.ToLower(o.Context()), strings.ToLower(f.Context)) {
		return false
	}
	if f.SourceURL != "" && !strings.Contains(strings.ToLower(o.SourceURL()), strings.ToLower(f.SourceURL)) {
		return false
	}
	if f.CapturedFrom != nil && o.CapturedAt().Before(*f.CapturedFrom) {
		return false
	}
	if f.CapturedTo != nil && o.CapturedAt().After(*f.CapturedTo) {
		return false
	}
	return true
}
