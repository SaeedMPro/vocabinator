package usecase

import (
	"context"

	"github.com/SaeedMPro/vocabinator/internal/application/port/inbound"
	"github.com/SaeedMPro/vocabinator/internal/application/port/outbound"
	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

type vocabularyService struct {
	repository outbound.VocabularyRepository
	occurrenceRepository outbound.VocabOccurrenceRepository
}

var _ inbound.VocabularyService = (*vocabularyService)(nil)

func NewVocabularyService(
	repository outbound.VocabularyRepository,
	occurrenceRepository outbound.VocabOccurrenceRepository,
) inbound.VocabularyService {
	return &vocabularyService{
		repository: repository,
		occurrenceRepository: occurrenceRepository,
	}
}

func (s *vocabularyService) CreateVocabItem(ctx context.Context, input inbound.CreateVocabItemInput) (*vocab.VocabularyItem, error) {
	return nil, nil
}

func (s *vocabularyService) GetVocabItem(ctx context.Context, id vocab.ID) (*vocab.VocabularyItem, error) {
	return nil, nil
}

func (s *vocabularyService) ListVocabItems(ctx context.Context, filter inbound.VocabItemFilter) ([]*vocab.VocabularyItem, error) {
	return nil, nil
}

func (s *vocabularyService) DeleteVocabItem(ctx context.Context, id vocab.ID) error {
	return nil
}

func (s *vocabularyService) AddVocabOccurrence(ctx context.Context, itemID vocab.ID, input inbound.AddVocabOccurrenceInput) (*vocab.Occurrence, error) {
	return nil, nil
}

func (s *vocabularyService) ListVocabOccurrences(ctx context.Context, itemID vocab.ID, filter inbound.VocabOccurrenceFilter) ([]vocab.Occurrence, error) {
	return nil, nil
}

func (s *vocabularyService) DeleteVocabOccurrence(ctx context.Context, itemID vocab.ID, occurrenceID vocab.ID) error {
	return nil
}