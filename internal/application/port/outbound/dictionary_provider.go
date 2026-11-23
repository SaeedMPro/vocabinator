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
	Term     string
	Phonetic string
	Senses   []WordSense
}

type WordSense struct {
	PartOfSpeech string
	Definitions  []string
	Examples     []string
	Synonyms     []string
	Antonyms     []string
}
