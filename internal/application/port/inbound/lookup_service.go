package inbound

import (
	"context"

	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

type LookupWordInput struct {
	Term           string
	SourceLanguage vocab.LanguageCode
	TargetLanguage vocab.LanguageCode
}

// LookupResult is what every interface shows for a quick lookup. It never
// persists anything.
type LookupResult struct {
	Term string
	// Phonetic is the primary pronunciation; Pronunciations has all variants.
	Phonetic       string
	Pronunciations []PronunciationResult

	Senses []SenseResult

	// Attribution credits the dictionary content. Show it with the result.
	Attribution AttributionResult

	// Warnings describe degraded results, e.g. translation being unavailable.
	Warnings []string
}

type PronunciationResult struct {
	Text     string
	AudioURL string // empty when there is no recording
}

type AttributionResult struct {
	SourceName string
	SourceURL  string
	License    string
	LicenseURL string
}

type SenseResult struct {
	PartOfSpeech string
	Definitions  []DefinitionResult
	Synonyms     []string
	Antonyms     []string
}

type DefinitionResult struct {
	Text string
	// Example is a usage example for this definition, when the dictionary has one.
	Example string
	// Translation is empty when the definition was not translated.
	Translation string
}

// LookupService provides dictionary lookup, separate from vocabulary CRUD.
type LookupService interface {
	LookupWord(ctx context.Context, input LookupWordInput) (*LookupResult, error)
}
