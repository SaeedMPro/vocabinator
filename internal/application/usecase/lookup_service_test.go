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
		{Term: "robust", Phonetic: "/rəˈbʌst/", Senses: []outbound.WordSense{
			{PartOfSpeech: "adjective", Definitions: []string{"Strong.", "Resilient."}, Examples: []string{"A robust design."}, Synonyms: []string{"sturdy"}},
		}},
		{Term: "robust", Phonetic: "ignored", Senses: []outbound.WordSense{
			{PartOfSpeech: "noun", Definitions: []string{"Other."}},
		}},
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
	if r.Term != "Robust" || r.Phonetic != "/rəˈbʌst/" || r.Translation != "fa:Robust" || len(r.Warnings) != 0 {
		t.Errorf("unexpected result %+v", r)
	}
	if len(r.Senses) != 2 || r.Senses[0].Definitions[1].Translation != "fa:Resilient." || r.Senses[1].Definitions[0].Translation != "fa:Other." {
		t.Errorf("unexpected senses %+v", r.Senses)
	}
	if tr.calls != 1 {
		t.Errorf("expected one batched translation call, got %d", tr.calls)
	}
}

func TestLookupWordLimitsTranslatedDefinitions(t *testing.T) {
	var defs []string
	for i := range 20 {
		defs = append(defs, fmt.Sprintf("def %d", i))
	}
	d := &fakeDict{entries: []outbound.DictionaryEntry{{Senses: []outbound.WordSense{{Definitions: defs}}}}}
	tr := &fakeTranslator{}

	r, err := lookup(t, d, tr, enFa)
	if err != nil {
		t.Fatal(err)
	}
	if len(tr.gotText) != 1+maxTranslatedDefinitions {
		t.Errorf("expected %d texts, got %d", 1+maxTranslatedDefinitions, len(tr.gotText))
	}
	if len(r.Senses[0].Definitions) != 20 || r.Senses[0].Definitions[maxTranslatedDefinitions].Translation != "" {
		t.Error("all definitions must be returned, only the first few translated")
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
			if len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0], "translation unavailable") || r.Translation != "" || len(r.Senses) != 2 {
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
	if err != nil || tr.calls != 0 || r.Translation != "" {
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
