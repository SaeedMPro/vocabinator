package outbound

import (
	"context"

	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

// DictionaryProvider retrieves provider-independent dictionary information.
// Lookup returns ErrNotFound when the term is unknown and
// ErrUnsupportedLanguage when the provider does not cover the language.
type DictionaryProvider interface {
	Lookup(ctx context.Context, term string, language vocab.LanguageCode) ([]DictionaryEntry, error)
}

type DictionaryEntry struct {
	Term string
	// Phonetic is the primary pronunciation text; Pronunciations lists all of
	// them, including variants (for example UK and US).
	Phonetic       string
	Pronunciations []Pronunciation
	Senses         []WordSense

	// Attribution says where the content comes from and under what licence.
	// Display it wherever the content is shown.
	Attribution Attribution
}

type Pronunciation struct {
	Text     string
	AudioURL string // may be empty
}

type Attribution struct {
	SourceName string // for example "Wiktionary"
	SourceURL  string
	License    string // for example "CC BY-SA 3.0"
	LicenseURL string
}

type WordSense struct {
	PartOfSpeech string
	Definitions  []Definition
	Synonyms     []string
	Antonyms     []string
}

type Definition struct {
	Text    string
	Example string // may be empty
}
