package vocab

import (
	"errors"
	"testing"
	"time"
)

func mustLanguageCode(t *testing.T, code string) LanguageCode {
	t.Helper()

	language, err := NewLanguageCode(code)
	if err != nil {
		t.Fatalf("failed to create language code: %v", err)
	}

	return language
}

func newTestItem(t *testing.T) *VocabularyItem {
	t.Helper()

	item, err := NewVocabularyItem(
		"item-1",
		"robust",
		mustLanguageCode(t, "en"),
		mustLanguageCode(t, "fa"),
		time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("failed to create vocabulary item: %v", err)
	}

	return item
}

func newTestOccurrence(t *testing.T, itemID ID, occurrenceID ID) Occurrence {
	t.Helper()

	occurrence, err := NewOccurrence(
		occurrenceID,
		itemID,
		"We need a robust architecture.",
		"https://example.com/article",
		time.Date(2026, 11, 22, 13, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("failed to create occurrence: %v", err)
	}

	return occurrence
}

// Constructor tests

func TestNewVocabularyItem(t *testing.T) {
	createdAt := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)

	item, err := NewVocabularyItem(
		"item-1",
		"  Robust  ",
		mustLanguageCode(t, "EN"),
		mustLanguageCode(t, "FA"),
		createdAt,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if item.ID() != "item-1" {
		t.Errorf("expected ID item-1, got %s", item.ID())
	}

	if item.Term() != "Robust" {
		t.Errorf("expected term Robust, got %s", item.Term())
	}

	if item.NormalizedTerm() != "robust" {
		t.Errorf("expected normalized term robust, got %s", item.NormalizedTerm())
	}

	if item.SourceLanguage() != "en" {
		t.Errorf("expected source language en, got %s", item.SourceLanguage())
	}

	if item.TargetLanguage() != "fa" {
		t.Errorf("expected target language fa, got %s", item.TargetLanguage())
	}

	if !item.CreatedAt().Equal(createdAt) {
		t.Errorf("unexpected CreatedAt: %v", item.CreatedAt())
	}
}

func TestNewVocabularyItemRejectsInvalidID(t *testing.T) {
	_, err := NewVocabularyItem(
		" ",
		"robust",
		mustLanguageCode(t, "en"),
		mustLanguageCode(t, "fa"),
		time.Now(),
	)

	if !errors.Is(err, ErrInvalidID) {
		t.Fatalf("expected ErrInvalidID, got %v", err)
	}
}

func TestNewVocabularyItemRejectsEmptyTerm(t *testing.T) {
	_, err := NewVocabularyItem(
		"item-1",
		"   ",
		mustLanguageCode(t, "en"),
		mustLanguageCode(t, "fa"),
		time.Now(),
	)

	if !errors.Is(err, ErrEmptyTerm) {
		t.Fatalf("expected ErrEmptyTerm, got %v", err)
	}
}

func TestNewVocabularyItemRejectsInvalidTimestamp(t *testing.T) {
	_, err := NewVocabularyItem(
		"item-1",
		"robust",
		mustLanguageCode(t, "en"),
		mustLanguageCode(t, "fa"),
		time.Time{},
	)

	if !errors.Is(err, ErrInvalidTimestamp) {
		t.Fatalf("expected ErrInvalidTimestamp, got %v", err)
	}
}

// Normalization tests

func TestNormalizeTerm(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "lowercase",
			input:    "Robust",
			expected: "robust",
		},
		{
			name:     "uppercase",
			input:    "ROBUST",
			expected: "robust",
		},
		{
			name:     "leading and trailing spaces",
			input:    "  robust  ",
			expected: "robust",
		},
		{
			name:     "multiple spaces",
			input:    "Hello    World",
			expected: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeTerm(tt.input)

			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// Vocabulary key tests

func TestVocabularyItemKey(t *testing.T) {
	item := newTestItem(t)

	key := item.Key()

	if key.NormalizedTerm != "robust" {
		t.Errorf("expected robust, got %s", key.NormalizedTerm)
	}

	if key.SourceLanguage != "en" {
		t.Errorf("expected en, got %s", key.SourceLanguage)
	}

	if key.TargetLanguage != "fa" {
		t.Errorf("expected fa, got %s", key.TargetLanguage)
	}
}

// Occurrence tests

func TestAddOccurrence(t *testing.T) {
	item := newTestItem(t)

	occurrence := newTestOccurrence(t, item.ID(), "occ-1")

	err := item.AddOccurrence(occurrence)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	occurrences := item.Occurrences()

	if len(occurrences) != 1 {
		t.Fatalf("expected 1 occurrence, got %d", len(occurrences))
	}

	if occurrences[0].ID() != "occ-1" {
		t.Errorf("expected occurrence ID occ-1, got %s", occurrences[0].ID())
	}
}

func TestAddOccurrenceRejectsDifferentVocabularyItem(t *testing.T) {
	item := newTestItem(t)

	occurrence := newTestOccurrence(t, "another-item", "occ-1")

	err := item.AddOccurrence(occurrence)

	if !errors.Is(err, ErrOccurrenceItemMismatch) {
		t.Fatalf("expected ErrOccurrenceItemMismatch, got %v", err)
	}

	if len(item.Occurrences()) != 0 {
		t.Error("occurrence should not have been added")
	}
}

func TestAddOccurrenceRejectsDuplicate(t *testing.T) {
	item := newTestItem(t)

	occurrence := newTestOccurrence(t, item.ID(), "occ-1")

	if err := item.AddOccurrence(occurrence); err != nil {
		t.Fatal(err)
	}

	err := item.AddOccurrence(occurrence)

	if !errors.Is(err, ErrDuplicateOccurrence) {
		t.Fatalf("expected ErrDuplicateOccurrence, got %v", err)
	}

	if len(item.Occurrences()) != 1 {
		t.Errorf("expected 1 occurrence, got %d", len(item.Occurrences()))
	}
}

func TestOccurrencesReturnsCopy(t *testing.T) {
	item := newTestItem(t)

	occurrence := newTestOccurrence(t, item.ID(), "occ-1")

	if err := item.AddOccurrence(occurrence); err != nil {
		t.Fatal(err)
	}

	occurrences := item.Occurrences()

	// Modify the returned slice.
	occurrences[0] = Occurrence{}

	// The original entity should remain unchanged.
	if item.Occurrences()[0].ID() != "occ-1" {
		t.Error("modifying returned occurrences changed the original entity")
	}
}