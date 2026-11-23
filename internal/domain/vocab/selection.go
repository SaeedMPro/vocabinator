package vocab

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// MaxSelectionWords is the longest phrase accepted from a text selection.
	MaxSelectionWords = 4
	// MaxTermRunes is the longest accepted term.
	MaxTermRunes = 100
)

// TermFromSelection identifies the term in raw selected text: whitespace is
// collapsed and surrounding punctuation, quotes, symbols and invisible
// direction marks are trimmed, while inner apostrophes and hyphens are kept.
// Long selections are rejected rather than guessed at.
func TermFromSelection(text string) (string, error) {
	term := strings.Join(strings.Fields(text), " ")
	term = strings.TrimFunc(term, isEdgeNoise)

	if term == "" {
		return "", ErrEmptyTerm
	}
	if len(strings.Fields(term)) > MaxSelectionWords || utf8.RuneCountInString(term) > MaxTermRunes {
		return "", ErrSelectionTooLong
	}
	return term, nil
}

func isEdgeNoise(r rune) bool {
	return unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) || unicode.Is(unicode.Cf, r)
}
