package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/SaeedMPro/vocabinator/internal/application/port/inbound"
	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

const timeLayout = "02/01/2006 15:04"

func formatTime(t time.Time) string { return t.Local().Format(timeLayout) }

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// JSON views keep the machine-readable contract independent of internal types.

type lookupJSON struct {
	Term        string      `json:"term"`
	Phonetic    string      `json:"phonetic,omitempty"`
	Translation string      `json:"translation,omitempty"`
	Senses      []senseJSON `json:"senses"`
	Warnings    []string    `json:"warnings,omitempty"`
}

type senseJSON struct {
	PartOfSpeech string           `json:"partOfSpeech,omitempty"`
	Definitions  []definitionJSON `json:"definitions"`
	Examples     []string         `json:"examples,omitempty"`
	Synonyms     []string         `json:"synonyms,omitempty"`
	Antonyms     []string         `json:"antonyms,omitempty"`
}

type definitionJSON struct {
	Text        string `json:"text"`
	Translation string `json:"translation,omitempty"`
}

type itemJSON struct {
	ID             string           `json:"id"`
	Term           string           `json:"term"`
	SourceLanguage string           `json:"sourceLanguage"`
	TargetLanguage string           `json:"targetLanguage"`
	CreatedAt      time.Time        `json:"createdAt"`
	Occurrences    []occurrenceJSON `json:"occurrences"`
}

type occurrenceJSON struct {
	ID         string    `json:"id"`
	Context    string    `json:"context,omitempty"`
	SourceURL  string    `json:"sourceUrl,omitempty"`
	CapturedAt time.Time `json:"capturedAt"`
}

func toItemJSON(item *vocab.VocabularyItem) itemJSON {
	out := itemJSON{
		ID:             string(item.ID()),
		Term:           item.Term(),
		SourceLanguage: item.SourceLanguage().String(),
		TargetLanguage: item.TargetLanguage().String(),
		CreatedAt:      item.CreatedAt(),
		Occurrences:    []occurrenceJSON{},
	}
	for _, o := range item.Occurrences() {
		out.Occurrences = append(out.Occurrences, occurrenceJSON{
			ID: string(o.ID()), Context: o.Context(), SourceURL: o.SourceURL(), CapturedAt: o.CapturedAt(),
		})
	}
	return out
}

func renderLookup(w io.Writer, f format, r *inbound.LookupResult) error {
	if f == formatJSON {
		out := lookupJSON{Term: r.Term, Phonetic: r.Phonetic, Translation: r.Translation, Warnings: r.Warnings, Senses: []senseJSON{}}
		for _, s := range r.Senses {
			sj := senseJSON{PartOfSpeech: s.PartOfSpeech, Examples: s.Examples, Synonyms: s.Synonyms, Antonyms: s.Antonyms, Definitions: []definitionJSON{}}
			for _, d := range s.Definitions {
				sj.Definitions = append(sj.Definitions, definitionJSON{Text: d.Text, Translation: d.Translation})
			}
			out.Senses = append(out.Senses, sj)
		}
		return writeJSON(w, out)
	}

	head := r.Term
	if r.Phonetic != "" {
		head += "  " + r.Phonetic
	}
	fmt.Fprintln(w, head)
	if r.Translation != "" {
		fmt.Fprintf(w, "≈ %s\n", r.Translation)
	}

	for _, s := range r.Senses {
		fmt.Fprintf(w, "\n%s\n", orDefault(s.PartOfSpeech, "(unspecified)"))
		for i, d := range s.Definitions {
			fmt.Fprintf(w, "  %d. %s\n", i+1, d.Text)
			if d.Translation != "" {
				fmt.Fprintf(w, "     → %s\n", d.Translation)
			}
		}
		if len(s.Examples) > 0 {
			fmt.Fprintf(w, "  Example: %s\n", s.Examples[0])
		}
		if len(s.Synonyms) > 0 {
			fmt.Fprintf(w, "  Synonyms: %s\n", strings.Join(s.Synonyms, ", "))
		}
		if len(s.Antonyms) > 0 {
			fmt.Fprintf(w, "  Antonyms: %s\n", strings.Join(s.Antonyms, ", "))
		}
	}

	for _, warn := range r.Warnings {
		fmt.Fprintf(w, "\nNote: %s\n", warn)
	}
	return nil
}

func renderItem(w io.Writer, f format, item *vocab.VocabularyItem) error {
	if f == formatJSON {
		return writeJSON(w, toItemJSON(item))
	}

	fmt.Fprintf(w, "%s  (%s → %s)\n", item.Term(), item.SourceLanguage(), item.TargetLanguage())
	fmt.Fprintf(w, "Saved: %s\n", formatTime(item.CreatedAt()))

	occurrences := item.Occurrences()
	if len(occurrences) == 0 {
		fmt.Fprintln(w, "\nNo contexts recorded.")
		return nil
	}

	fmt.Fprintf(w, "\nContexts (%d):\n", len(occurrences))
	for _, o := range occurrences {
		fmt.Fprintf(w, "  [%s] %s\n", formatTime(o.CapturedAt()), orDefault(o.Context(), "(no context)"))
		if o.SourceURL() != "" {
			fmt.Fprintf(w, "    %s\n", o.SourceURL())
		}
	}
	return nil
}

func renderList(w io.Writer, f format, items []*vocab.VocabularyItem) error {
	if f == formatJSON {
		out := make([]itemJSON, 0, len(items))
		for _, item := range items {
			out = append(out, toItemJSON(item))
		}
		return writeJSON(w, out)
	}

	if len(items) == 0 {
		fmt.Fprintln(w, "No vocabulary saved yet.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TERM\tLANGUAGES\tCONTEXTS\tSAVED")
	for _, item := range items {
		fmt.Fprintf(tw, "%s\t%s → %s\t%d\t%s\n",
			item.Term(), item.SourceLanguage(), item.TargetLanguage(), len(item.Occurrences()), formatTime(item.CreatedAt()))
	}
	return tw.Flush()
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
