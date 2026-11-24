package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/SaeedMPro/vocabinator/internal/application/port/inbound"
	"github.com/SaeedMPro/vocabinator/internal/application/port/outbound"
	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

type fakeDict struct {
	entries []outbound.DictionaryEntry
	err     error
	calls   int
	gotTerm string
	gotLang vocab.LanguageCode
}

func (d *fakeDict) Lookup(_ context.Context, term string, lang vocab.LanguageCode) ([]outbound.DictionaryEntry, error) {
	d.calls++
	d.gotTerm, d.gotLang = term, lang
	return d.entries, d.err
}

type fakeTranslator struct {
	err     error
	short   bool
	calls   int
	gotText []string
}

func (f *fakeTranslator) Translate(_ context.Context, texts []string, _, _ vocab.LanguageCode) ([]string, error) {
	f.calls++
	f.gotText = texts
	if f.err != nil {
		return nil, f.err
	}
	if f.short {
		return nil, nil
	}
	out := make([]string, len(texts))
	for i, t := range texts {
		out[i] = "fa:" + t
	}
	return out, nil
}

func robustEntries() []outbound.DictionaryEntry {
	return []outbound.DictionaryEntry{
		{
			Term: "robust", Phonetic: "/rəˈbʌst/",
			Pronunciations: []outbound.Pronunciation{{Text: "/rəˈbʌst/", AudioURL: "https://x/uk.mp3"}, {Text: "/roʊˈbʌst/"}},
			Attribution:    outbound.Attribution{SourceName: "Wiktionary", License: "CC BY-SA 3.0"},
			Senses: []outbound.WordSense{{
				PartOfSpeech: "adjective",
				Definitions:  []outbound.Definition{{Text: "Strong.", Example: "A robust design."}, {Text: "Resilient."}},
				Synonyms:     []string{"sturdy"},
			}},
		},
		{
			Term: "robust", Phonetic: "ignored",
			Pronunciations: []outbound.Pronunciation{{Text: "/roʊˈbʌst/"}, {Text: "/other/"}},
			Attribution:    outbound.Attribution{SourceName: "Other"},
			Senses:         []outbound.WordSense{{PartOfSpeech: "noun", Definitions: []outbound.Definition{{Text: "Other."}}}},
		},
	}
}

func lookup(t *testing.T, d *fakeDict, tr *fakeTranslator, in inbound.LookupWordInput) (*inbound.LookupResult, error) {
	t.Helper()
	return NewLookupService(d, tr).LookupWord(context.Background(), in)
}

var enFa = inbound.LookupWordInput{Term: "  Robust ", SourceLanguage: "EN", TargetLanguage: "fa"}

func TestLookupWordMergesAndTranslates(t *testing.T) {
	d, tr := &fakeDict{entries: robustEntries()}, &fakeTranslator{}

	r, err := lookup(t, d, tr, enFa)
	if err != nil {
		t.Fatal(err)
	}
	if d.gotTerm != "Robust" || d.gotLang != vocab.LanguageCodeEnglish {
		t.Errorf("dictionary got %q/%q", d.gotTerm, d.gotLang)
	}
	if r.Term != "Robust" || r.Phonetic != "/rəˈbʌst/" || len(r.Warnings) != 0 {
		t.Errorf("unexpected result %+v", r)
	}
	if len(r.Senses) != 2 || r.Senses[0].Definitions[1].Translation != "fa:Resilient." || r.Senses[1].Definitions[0].Translation != "fa:Other." {
		t.Errorf("unexpected senses %+v", r.Senses)
	}
	if tr.calls != 1 {
		t.Errorf("expected one batched translation call, got %d", tr.calls)
	}
	// Only definitions are translated. The bare headword is ambiguous and is
	// never sent to the translation provider.
	for _, sent := range tr.gotText {
		if sent == "Robust" || sent == "robust" {
			t.Errorf("headword must not be translated in isolation: %v", tr.gotText)
		}
	}
	if len(tr.gotText) != 3 || tr.gotText[0] != "Strong." {
		t.Errorf("expected the three definitions in order, got %v", tr.gotText)
	}
}

func TestLookupWordKeepsExamplesPronunciationsAndAttribution(t *testing.T) {
	r, err := lookup(t, &fakeDict{entries: robustEntries()}, &fakeTranslator{}, enFa)
	if err != nil {
		t.Fatal(err)
	}

	// The example stays attached to its own definition.
	if d := r.Senses[0].Definitions; d[0].Example != "A robust design." || d[1].Example != "" {
		t.Errorf("examples not attached to definitions: %+v", d)
	}
	// Pronunciations are merged without duplicates, keeping audio.
	if len(r.Pronunciations) != 3 || r.Pronunciations[0].AudioURL != "https://x/uk.mp3" || r.Pronunciations[2].Text != "/other/" {
		t.Errorf("unexpected pronunciations %+v", r.Pronunciations)
	}
	// Attribution comes from the first entry that has one.
	if r.Attribution.SourceName != "Wiktionary" || r.Attribution.License != "CC BY-SA 3.0" {
		t.Errorf("unexpected attribution %+v", r.Attribution)
	}
}

func TestLookupWordLimitsTranslatedDefinitions(t *testing.T) {
	var defs []outbound.Definition
	for i := range 20 {
		defs = append(defs, outbound.Definition{Text: fmt.Sprintf("def %d", i)})
	}
	d := &fakeDict{entries: []outbound.DictionaryEntry{{Senses: []outbound.WordSense{{Definitions: defs}}}}}
	tr := &fakeTranslator{}

	r, err := lookup(t, d, tr, enFa)
	if err != nil {
		t.Fatal(err)
	}
	if len(tr.gotText) != maxTranslatedDefinitions {
		t.Errorf("expected %d texts, got %d", maxTranslatedDefinitions, len(tr.gotText))
	}
	if len(r.Senses[0].Definitions) != 20 || r.Senses[0].Definitions[maxTranslatedDefinitions].Translation != "" {
		t.Error("all definitions must be returned, only the first few translated")
	}
}

func TestTranslationBudgetCoversEverySenseBeforeGoingDeeper(t *testing.T) {
	mk := func(pos string, n int) outbound.WordSense {
		s := outbound.WordSense{PartOfSpeech: pos}
		for i := 1; i <= n; i++ {
			s.Definitions = append(s.Definitions, outbound.Definition{Text: fmt.Sprintf("%s %d", pos, i)})
		}
		return s
	}
	d := &fakeDict{entries: []outbound.DictionaryEntry{{Senses: []outbound.WordSense{mk("verb", 9), mk("noun", 9), mk("adj", 1), mk("adv", 9)}}}}
	tr := &fakeTranslator{}

	r, err := lookup(t, d, tr, enFa)
	if err != nil {
		t.Fatal(err)
	}

	// Budget of 5: the first definition of all four senses, then the second of the first sense.
	want := []string{"verb 1", "noun 1", "adj 1", "adv 1", "verb 2"}
	if fmt.Sprint(tr.gotText) != fmt.Sprint(want) {
		t.Errorf("translated %v, want %v", tr.gotText, want)
	}
	for _, s := range r.Senses {
		if s.Definitions[0].Translation == "" {
			t.Errorf("sense %q has no translation for its first definition", s.PartOfSpeech)
		}
	}
	if r.Senses[0].Definitions[1].Translation == "" || r.Senses[1].Definitions[1].Translation != "" {
		t.Error("the fifth translation goes to the second verb definition only")
	}
}

func TestLookupWordTranslationFailureDegrades(t *testing.T) {
	for name, tr := range map[string]*fakeTranslator{
		"error":    {err: outbound.ErrRateLimited},
		"mismatch": {short: true},
	} {
		t.Run(name, func(t *testing.T) {
			r, err := lookup(t, &fakeDict{entries: robustEntries()}, tr, enFa)
			if err != nil {
				t.Fatalf("lookup must still succeed: %v", err)
			}
			if len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0], "translation unavailable") || len(r.Senses) != 2 {
				t.Errorf("unexpected result %+v", r)
			}
		})
	}
}

func TestLookupWordContextCancellationIsAnError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	tr := &cancelingTranslator{cancel: cancel}

	_, err := NewLookupService(&fakeDict{entries: robustEntries()}, tr).LookupWord(ctx, enFa)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

type cancelingTranslator struct{ cancel func() }

func (c *cancelingTranslator) Translate(ctx context.Context, _ []string, _, _ vocab.LanguageCode) ([]string, error) {
	c.cancel()
	return nil, ctx.Err()
}

func TestLookupWordSameLanguageSkipsTranslation(t *testing.T) {
	tr := &fakeTranslator{}
	in := inbound.LookupWordInput{Term: "robust", SourceLanguage: "en", TargetLanguage: "en"}

	r, err := lookup(t, &fakeDict{entries: robustEntries()}, tr, in)
	if err != nil || tr.calls != 0 || r.Senses[0].Definitions[0].Translation != "" {
		t.Errorf("err=%v calls=%d result=%+v", err, tr.calls, r)
	}
}

func TestLookupWordErrors(t *testing.T) {
	boom := errors.New("boom")

	t.Run("not found", func(t *testing.T) {
		_, err := lookup(t, &fakeDict{err: outbound.ErrNotFound}, &fakeTranslator{}, enFa)
		if !errors.Is(err, ErrWordNotFound) {
			t.Errorf("got %v", err)
		}
	})
	t.Run("empty entries", func(t *testing.T) {
		_, err := lookup(t, &fakeDict{}, &fakeTranslator{}, enFa)
		if !errors.Is(err, ErrWordNotFound) {
			t.Errorf("got %v", err)
		}
	})
	t.Run("provider error propagates", func(t *testing.T) {
		_, err := lookup(t, &fakeDict{err: boom}, &fakeTranslator{}, enFa)
		if !errors.Is(err, boom) || errors.Is(err, ErrWordNotFound) {
			t.Errorf("got %v", err)
		}
	})
	t.Run("unsupported language propagates", func(t *testing.T) {
		_, err := lookup(t, &fakeDict{err: outbound.ErrUnsupportedLanguage}, &fakeTranslator{}, enFa)
		if !errors.Is(err, outbound.ErrUnsupportedLanguage) {
			t.Errorf("got %v", err)
		}
	})
}

func TestLookupWordInvalidInputNeverCallsProviders(t *testing.T) {
	d := &fakeDict{entries: robustEntries()}
	for name, in := range map[string]inbound.LookupWordInput{
		"empty term":   {Term: " ", SourceLanguage: "en", TargetLanguage: "fa"},
		"bad language": {Term: "x", SourceLanguage: "zz", TargetLanguage: "fa"},
		"too long":     {Term: strings.Repeat("a", maxTermRunes+1), SourceLanguage: "en", TargetLanguage: "fa"},
	} {
		if _, err := lookup(t, d, &fakeTranslator{}, in); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: expected ErrInvalidInput, got %v", name, err)
		}
	}
	if d.calls != 0 {
		t.Error("dictionary must not be called for invalid input")
	}
}
