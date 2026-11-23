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
	Term     string
	Phonetic string

	// Translation is the isolated term translation. It is a hint only: words
	// are ambiguous, so prefer the per-definition translations.
	Translation string

	Senses []SenseResult

	// Warnings describe degraded results, e.g. translation being unavailable.
	Warnings []string
}

type SenseResult struct {
	PartOfSpeech string
	Definitions  []DefinitionResult
	Examples     []string
	Synonyms     []string
	Antonyms     []string
}

type DefinitionResult struct {
	Text string
	// Translation is empty when the definition was not translated.
	Translation string
}

// LookupService provides dictionary lookup, separate from vocabulary CRUD.
type LookupService interface {
	LookupWord(ctx context.Context, input LookupWordInput) (*LookupResult, error)
}
