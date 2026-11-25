package vocab

import (
	"errors"
	"strings"
	"testing"
)

func TestTermFromSelection(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"plain", "robust", "robust"},
		{"whitespace", "  \n robust\t ", "robust"},
		{"trailing punctuation", "robust,", "robust"},
		{"quotes", "“robust”", "robust"},
		{"brackets and dots", "(robust).", "robust"},
		{"keeps inner apostrophe", "don't", "don't"},
		{"keeps inner hyphen", "well-known.", "well-known"},
		{"phrase collapses whitespace", "ice \n  cream", "ice cream"},
		{"persian with punctuation", "«مقاوم»،", "مقاوم"},
		{"direction marks", "‏مقاوم‏", "مقاوم"},
		{"keeps zwnj inside", "می‌خواهم", "می‌خواهم"},
		{"case preserved", "Robust", "Robust"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := TermFromSelection(tc.in)
			if err != nil || got != tc.want {
				t.Errorf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestTermFromSelectionRejects(t *testing.T) {
	for _, in := range []string{"", "   \n", "...", "“”", "‏"} {
		if _, err := TermFromSelection(in); !errors.Is(err, ErrEmptyTerm) {
			t.Errorf("%q: expected ErrEmptyTerm, got %v", in, err)
		}
	}

	for name, in := range map[string]string{
		"too many words": "one two three four five",
		"too many runes": strings.Repeat("a", MaxTermRunes+1),
		"a paragraph":    strings.Repeat("The quick brown fox. ", 20),
	} {
		if _, err := TermFromSelection(in); !errors.Is(err, ErrSelectionTooLong) {
			t.Errorf("%s: expected ErrSelectionTooLong, got %v", name, err)
		}
	}

	if _, err := TermFromSelection("one two three four"); err != nil {
		t.Errorf("a %d-word phrase must be accepted: %v", MaxSelectionWords, err)
	}
}
