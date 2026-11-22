package outbound

import (
    "context"

    "github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

type DictionaryProvider interface {
    Lookup(
        ctx context.Context,
        term string,
        language vocab.LanguageCode,
    ) ([]DictionaryEntry, error)
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