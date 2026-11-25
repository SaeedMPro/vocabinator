package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/SaeedMPro/vocabinator/internal/application/port/inbound"
	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

func newSaver(t *testing.T) (inbound.SaveWordService, inbound.VocabularyService, *fakeRepo) {
	t.Helper()
	vs, repo, _ := newService(t)
	return NewSaveWordService(vs), vs, repo
}

func saveInput(text, url string) inbound.SaveWordInput {
	return inbound.SaveWordInput{Term: "Robust", SourceLanguage: "en", TargetLanguage: "fa", Context: text, SourceURL: url}
}

func TestSaveWordCreatesItemWithOccurrence(t *testing.T) {
	saver, _, repo := newSaver(t)

	res, err := saver.SaveWord(context.Background(), saveInput("We need a robust architecture.", "https://example.com"))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Created || res.Occurrence == nil || len(res.Item.Occurrences()) != 1 || len(repo.items) != 1 {
		t.Errorf("unexpected result %+v", res)
	}
}

func TestSaveWordWithoutContextRecordsNoOccurrence(t *testing.T) {
	saver, _, _ := newSaver(t)

	res, err := saver.SaveWord(context.Background(), saveInput("  ", ""))
	if err != nil || !res.Created || res.Occurrence != nil || len(res.Item.Occurrences()) != 0 {
		t.Errorf("err=%v result=%+v", err, res)
	}
}

func TestSaveWordExistingItemGainsNewOccurrenceOnly(t *testing.T) {
	saver, _, repo := newSaver(t)
	ctx := context.Background()

	first, _ := saver.SaveWord(ctx, saveInput("one", ""))
	second, err := saver.SaveWord(ctx, saveInput("two", ""))
	if err != nil {
		t.Fatal(err)
	}
	if second.Created || second.Item.ID() != first.Item.ID() || len(second.Item.Occurrences()) != 2 || len(repo.items) != 1 {
		t.Errorf("unexpected result %+v", second)
	}

	// Saving the same context again is idempotent.
	again, err := saver.SaveWord(ctx, saveInput(" two ", ""))
	if err != nil || again.Occurrence != nil || len(again.Item.Occurrences()) != 2 {
		t.Errorf("err=%v result=%+v", err, again)
	}
}

func TestSaveWordInvalidInputAndErrors(t *testing.T) {
	saver, _, repo := newSaver(t)
	ctx := context.Background()

	if _, err := saver.SaveWord(ctx, inbound.SaveWordInput{Term: " ", SourceLanguage: "en", TargetLanguage: "fa"}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}

	boom := errors.New("boom")
	repo.saveErr = boom
	if _, err := saver.SaveWord(ctx, saveInput("x", "")); !errors.Is(err, boom) {
		t.Errorf("expected boom, got %v", err)
	}
}

// raceService simulates losing a creation race: Create reports the item exists
// but returns no item.
type raceService struct {
	inbound.VocabularyService
}

func (r raceService) CreateVocabItem(context.Context, inbound.CreateVocabItemInput) (*vocab.VocabularyItem, error) {
	return nil, ErrItemExists
}

func TestSaveWordRecoversFromCreationRace(t *testing.T) {
	vs, _, _ := newService(t)
	existing := createRobust(t, vs)

	res, err := NewSaveWordService(raceService{VocabularyService: vs}).SaveWord(context.Background(), saveInput("x", ""))
	if err != nil || res.Created || res.Item.ID() != existing.ID() || res.Occurrence == nil {
		t.Errorf("err=%v result=%+v", err, res)
	}
}
