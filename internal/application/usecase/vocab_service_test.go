package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/SaeedMPro/vocabinator/internal/application/port/inbound"
	"github.com/SaeedMPro/vocabinator/internal/application/port/outbound"
	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

// fakes

type fakeRepo struct {
	items   map[vocab.ID]*vocab.VocabularyItem
	saveErr error
	findErr error
	saves   int
}

func newFakeRepo() *fakeRepo { return &fakeRepo{items: map[vocab.ID]*vocab.VocabularyItem{}} }

func (r *fakeRepo) FindByID(_ context.Context, id vocab.ID) (*vocab.VocabularyItem, bool, error) {
	if r.findErr != nil {
		return nil, false, r.findErr
	}
	item, ok := r.items[id]
	return item, ok, nil
}

func (r *fakeRepo) FindByKey(_ context.Context, key vocab.VocabularyKey) (*vocab.VocabularyItem, bool, error) {
	if r.findErr != nil {
		return nil, false, r.findErr
	}
	for _, item := range r.items {
		if item.Key() == key {
			return item, true, nil
		}
	}
	return nil, false, nil
}

func (r *fakeRepo) List(_ context.Context, f outbound.VocabItemFilter) ([]*vocab.VocabularyItem, error) {
	var out []*vocab.VocabularyItem
	for _, item := range r.items {
		if f.Term != "" && !strings.Contains(item.NormalizedTerm(), f.Term) {
			continue
		}
		if f.SourceLanguage != nil && item.SourceLanguage() != *f.SourceLanguage {
			continue
		}
		if f.TargetLanguage != nil && item.TargetLanguage() != *f.TargetLanguage {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *fakeRepo) Save(_ context.Context, item *vocab.VocabularyItem) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.saves++
	r.items[item.ID()] = item
	return nil
}

func (r *fakeRepo) Delete(_ context.Context, id vocab.ID) error {
	delete(r.items, id)
	return nil
}

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

type fakeIDs struct{ n int }

func (g *fakeIDs) NewID() vocab.ID {
	g.n++
	return vocab.ID(fmt.Sprintf("id-%d", g.n))
}

func newService(t *testing.T) (inbound.VocabularyService, *fakeRepo, *fakeClock) {
	t.Helper()
	repo := newFakeRepo()
	clock := &fakeClock{now: time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)}
	return NewVocabularyService(repo, clock, &fakeIDs{}), repo, clock
}

func createRobust(t *testing.T, svc inbound.VocabularyService) *vocab.VocabularyItem {
	t.Helper()
	item, err := svc.CreateVocabItem(context.Background(), inbound.CreateVocabItemInput{
		Term: "Robust", SourceLanguage: vocab.LanguageCodeEnglish, TargetLanguage: vocab.LanguageCodePersian,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return item
}

// tests

func TestCreateVocabItem(t *testing.T) {
	svc, repo, clock := newService(t)
	item := createRobust(t, svc)

	if item.NormalizedTerm() != "robust" || !item.CreatedAt().Equal(clock.now) {
		t.Errorf("unexpected item: %+v", item)
	}
	if _, ok := repo.items[item.ID()]; !ok {
		t.Error("item was not saved")
	}
}

func TestCreateVocabItemDetectsExisting(t *testing.T) {
	svc, repo, _ := newService(t)
	first := createRobust(t, svc)

	existing, err := svc.CreateVocabItem(context.Background(), inbound.CreateVocabItemInput{
		Term: "  ROBUST ", SourceLanguage: "EN", TargetLanguage: "fa",
	})
	if !errors.Is(err, ErrItemExists) {
		t.Fatalf("expected ErrItemExists, got %v", err)
	}
	if existing == nil || existing.ID() != first.ID() {
		t.Error("expected the existing item to be returned alongside the error")
	}
	if len(repo.items) != 1 {
		t.Errorf("expected 1 item, got %d", len(repo.items))
	}
}

func TestCreateVocabItemSameTermOtherTargetIsSeparate(t *testing.T) {
	svc, repo, _ := newService(t)
	createRobust(t, svc)

	_, err := svc.CreateVocabItem(context.Background(), inbound.CreateVocabItemInput{
		Term: "robust", SourceLanguage: vocab.LanguageCodeEnglish, TargetLanguage: vocab.LanguageCodeEnglish,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.items) != 2 {
		t.Errorf("expected 2 items, got %d", len(repo.items))
	}
}

func TestCreateVocabItemInvalidInput(t *testing.T) {
	svc, _, _ := newService(t)

	cases := map[string]struct {
		input inbound.CreateVocabItemInput
		cause error
	}{
		"empty term":   {inbound.CreateVocabItemInput{Term: " ", SourceLanguage: "en", TargetLanguage: "fa"}, vocab.ErrEmptyTerm},
		"bad language": {inbound.CreateVocabItemInput{Term: "x", SourceLanguage: "zz", TargetLanguage: "fa"}, vocab.ErrInvalidLanguageCode},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := svc.CreateVocabItem(context.Background(), tc.input)
			if !errors.Is(err, ErrInvalidInput) || !errors.Is(err, tc.cause) {
				t.Errorf("expected ErrInvalidInput wrapping %v, got %v", tc.cause, err)
			}
		})
	}
}

func TestCreateVocabItemPropagatesRepositoryErrors(t *testing.T) {
	boom := errors.New("boom")

	svc, repo, _ := newService(t)
	repo.saveErr = boom
	_, err := svc.CreateVocabItem(context.Background(), inbound.CreateVocabItemInput{Term: "x", SourceLanguage: "en", TargetLanguage: "fa"})
	if !errors.Is(err, boom) {
		t.Errorf("save: expected boom, got %v", err)
	}

	svc, repo, _ = newService(t)
	repo.findErr = boom
	_, err = svc.CreateVocabItem(context.Background(), inbound.CreateVocabItemInput{Term: "x", SourceLanguage: "en", TargetLanguage: "fa"})
	if !errors.Is(err, boom) {
		t.Errorf("find: expected boom, got %v", err)
	}
}

func TestGetVocabItem(t *testing.T) {
	svc, _, _ := newService(t)
	item := createRobust(t, svc)

	got, err := svc.GetVocabItem(context.Background(), item.ID())
	if err != nil || got.ID() != item.ID() {
		t.Fatalf("unexpected result: %v, %v", got, err)
	}
	if _, err := svc.GetVocabItem(context.Background(), "missing"); !errors.Is(err, ErrItemNotFound) {
		t.Errorf("expected ErrItemNotFound, got %v", err)
	}
	if _, err := svc.GetVocabItem(context.Background(), " "); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestListVocabItemsFilters(t *testing.T) {
	svc, _, _ := newService(t)
	ctx := context.Background()
	for _, in := range []inbound.CreateVocabItemInput{
		{Term: "robust", SourceLanguage: "en", TargetLanguage: "fa"},
		{Term: "robustness", SourceLanguage: "en", TargetLanguage: "fa"},
		{Term: "ephemeral", SourceLanguage: "en", TargetLanguage: "fa"},
		{Term: "robust", SourceLanguage: "fa", TargetLanguage: "en"},
	} {
		if _, err := svc.CreateVocabItem(ctx, in); err != nil {
			t.Fatal(err)
		}
	}

	fa := vocab.LanguageCode("FA")
	cases := []struct {
		name   string
		filter inbound.VocabItemFilter
		want   int
	}{
		{"all", inbound.VocabItemFilter{}, 4},
		{"term", inbound.VocabItemFilter{Term: " ROBUST "}, 3},
		{"source language", inbound.VocabItemFilter{SourceLanguage: &fa}, 1},
		{"term and target", inbound.VocabItemFilter{Term: "robust", TargetLanguage: &fa}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := svc.ListVocabItems(ctx, tc.filter)
			if err != nil || len(got) != tc.want {
				t.Errorf("expected %d items, got %d (err %v)", tc.want, len(got), err)
			}
		})
	}
}

func TestListVocabItemsInvalidFilter(t *testing.T) {
	svc, _, _ := newService(t)
	bad := vocab.LanguageCode("zz")

	for name, f := range map[string]inbound.VocabItemFilter{
		"negative limit":  {Limit: -1},
		"negative offset": {Offset: -1},
		"bad language":    {SourceLanguage: &bad},
	} {
		if _, err := svc.ListVocabItems(context.Background(), f); !errors.Is(err, ErrInvalidFilter) {
			t.Errorf("%s: expected ErrInvalidFilter, got %v", name, err)
		}
	}
}

func TestDeleteVocabItem(t *testing.T) {
	svc, repo, _ := newService(t)
	item := createRobust(t, svc)

	if err := svc.DeleteVocabItem(context.Background(), item.ID()); err != nil {
		t.Fatal(err)
	}
	if len(repo.items) != 0 {
		t.Error("item was not deleted")
	}
	if err := svc.DeleteVocabItem(context.Background(), item.ID()); !errors.Is(err, ErrItemNotFound) {
		t.Errorf("expected ErrItemNotFound, got %v", err)
	}
}

func TestAddVocabOccurrence(t *testing.T) {
	svc, repo, clock := newService(t)
	ctx := context.Background()
	item := createRobust(t, svc)

	clock.now = clock.now.Add(time.Hour)
	o, err := svc.AddVocabOccurrence(ctx, item.ID(), inbound.AddVocabOccurrenceInput{
		Context: " We need a robust architecture. ", SourceURL: "https://example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if o.Context() != "We need a robust architecture." || !o.CapturedAt().Equal(clock.now) {
		t.Errorf("unexpected occurrence: %+v", o)
	}
	if got := repo.items[item.ID()].Occurrences(); len(got) != 1 {
		t.Errorf("expected 1 persisted occurrence, got %d", len(got))
	}

	if _, err := svc.AddVocabOccurrence(ctx, "missing", inbound.AddVocabOccurrenceInput{}); !errors.Is(err, ErrItemNotFound) {
		t.Errorf("expected ErrItemNotFound, got %v", err)
	}
}

func TestAddVocabOccurrenceSaveFailure(t *testing.T) {
	svc, repo, _ := newService(t)
	item := createRobust(t, svc)
	boom := errors.New("boom")
	repo.saveErr = boom

	if _, err := svc.AddVocabOccurrence(context.Background(), item.ID(), inbound.AddVocabOccurrenceInput{Context: "x"}); !errors.Is(err, boom) {
		t.Errorf("expected boom, got %v", err)
	}
}

func TestListVocabOccurrences(t *testing.T) {
	svc, _, clock := newService(t)
	ctx := context.Background()
	item := createRobust(t, svc)

	start := clock.now
	for i, in := range []inbound.AddVocabOccurrenceInput{
		{Context: "A robust architecture", SourceURL: "https://a.example"},
		{Context: "Go provides robust concurrency", SourceURL: "https://b.example"},
		{Context: "Something else", SourceURL: "https://a.example"},
	} {
		clock.now = start.Add(time.Duration(i) * time.Hour)
		if _, err := svc.AddVocabOccurrence(ctx, item.ID(), in); err != nil {
			t.Fatal(err)
		}
	}

	from, to := start.Add(30*time.Minute), start.Add(90*time.Minute)
	cases := []struct {
		name   string
		filter inbound.VocabOccurrenceFilter
		want   int
	}{
		{"all", inbound.VocabOccurrenceFilter{}, 3},
		{"context", inbound.VocabOccurrenceFilter{Context: "ROBUST"}, 2},
		{"url", inbound.VocabOccurrenceFilter{SourceURL: "a.example"}, 2},
		{"time window", inbound.VocabOccurrenceFilter{CapturedFrom: &from, CapturedTo: &to}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := svc.ListVocabOccurrences(ctx, item.ID(), tc.filter)
			if err != nil || len(got) != tc.want {
				t.Errorf("expected %d, got %d (err %v)", tc.want, len(got), err)
			}
		})
	}

	if _, err := svc.ListVocabOccurrences(ctx, item.ID(), inbound.VocabOccurrenceFilter{CapturedFrom: &to, CapturedTo: &from}); !errors.Is(err, ErrInvalidFilter) {
		t.Errorf("expected ErrInvalidFilter, got %v", err)
	}
	if _, err := svc.ListVocabOccurrences(ctx, "missing", inbound.VocabOccurrenceFilter{}); !errors.Is(err, ErrItemNotFound) {
		t.Errorf("expected ErrItemNotFound, got %v", err)
	}
}

func TestDeleteVocabOccurrence(t *testing.T) {
	svc, repo, _ := newService(t)
	ctx := context.Background()
	item := createRobust(t, svc)
	o, err := svc.AddVocabOccurrence(ctx, item.ID(), inbound.AddVocabOccurrenceInput{Context: "x"})
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.DeleteVocabOccurrence(ctx, item.ID(), o.ID()); err != nil {
		t.Fatal(err)
	}
	if got := repo.items[item.ID()].Occurrences(); len(got) != 0 {
		t.Errorf("expected no occurrences, got %d", len(got))
	}
	if err := svc.DeleteVocabOccurrence(ctx, item.ID(), o.ID()); !errors.Is(err, ErrOccurrenceNotFound) {
		t.Errorf("expected ErrOccurrenceNotFound, got %v", err)
	}
	if err := svc.DeleteVocabOccurrence(ctx, "missing", o.ID()); !errors.Is(err, ErrItemNotFound) {
		t.Errorf("expected ErrItemNotFound, got %v", err)
	}
}
