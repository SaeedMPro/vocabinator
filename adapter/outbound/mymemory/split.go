package mymemory

import "strings"

// splitText cuts text into pieces of at most max runes, preferring sentence
// boundaries, then spaces, and only as a last resort a hard cut. Pieces are
// trimmed. Text that already fits (including empty text) is returned as a
// single piece, so callers keep one result slot per input.
func splitText(text string, max int) []string {
	runes := []rune(text)
	if len(runes) <= max {
		return []string{text}
	}

	var pieces []string
	for len(runes) > max {
		cut := breakPoint(runes[:max+1])
		piece := strings.TrimSpace(string(runes[:cut]))
		if piece != "" {
			pieces = append(pieces, piece)
		}
		runes = []rune(strings.TrimLeft(string(runes[cut:]), " \t\r\n"))
	}
	if rest := strings.TrimSpace(string(runes)); rest != "" {
		pieces = append(pieces, rest)
	}
	return pieces
}

// breakPoint returns where to cut window (which is max+1 runes long): after
// the last sentence end followed by whitespace, else at the last space, else
// at max.
func breakPoint(window []rune) int {
	max := len(window) - 1

	sentence, space := -1, -1
	for i := 0; i < max; i++ {
		switch window[i] {
		case ' ', '\t', '\n', '\r':
			space = i
			if i > 0 && strings.ContainsRune(".!?؟;", window[i-1]) {
				sentence = i
			}
		}
	}
	// A space right after the window's last rune also allows a clean cut.
	if window[max] == ' ' || window[max] == '\n' {
		space = max
	}

	switch {
	case sentence > 0:
		return sentence
	case space > 0:
		return space
	}
	return max
}
