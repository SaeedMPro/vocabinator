package freedictionary

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/SaeedMPro/vocabinator/internal/application/port/outbound"
	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

const robustJSON = `[{
  "word": "robust",
  "phonetics": [
    {"audio": "no-text.mp3"},
    {"text": "/ɹəʊˈbʌst/", "audio": "https://example.com/uk.mp3"},
    {"text": "/ɹoʊˈbʌst/", "audio": ""},
    {"text": "/ɹəʊˈbʌst/", "audio": "https://example.com/duplicate.mp3"}
  ],
  "license": {"name": "CC BY-SA 3.0", "url": "https://creativecommons.org/licenses/by-sa/3.0"},
  "sourceUrls": ["https://en.wiktionary.org/wiki/robust"],
  "meanings": [{
    "partOfSpeech": "adjective",
    "definitions": [
      {"definition": "Strong and healthy.", "example": "A robust constitution.", "synonyms": ["sturdy", "Strong"], "antonyms": ["weak"]},
      {"definition": "Able to withstand stress.", "synonyms": ["strong", "resilient"], "antonyms": []},
      {"definition": ""}
    ],
    "synonyms": ["tough"],
    "antonyms": []
  }, {
    "partOfSpeech": "noun",
    "definitions": []
  }]
}]`

func newProvider(t *testing.T, h http.HandlerFunc) *Provider {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New(Config{BaseURL: srv.URL + "/"})
}

func TestLookupMapsResponse(t *testing.T) {
	var path string
	p := newProvider(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.EscapedPath()
		w.Write([]byte(robustJSON))
	})

	got, err := p.Lookup(context.Background(), "ice cream", vocab.LanguageCodeEnglish)
	if err != nil {
		t.Fatal(err)
	}
	if path != "/en/ice%20cream" {
		t.Errorf("unexpected path %q", path)
	}

	want := []outbound.DictionaryEntry{{
		Term: "robust",
		// No top-level "phonetic": the first textual pronunciation is used.
		Phonetic: "/ɹəʊˈbʌst/",
		Pronunciations: []outbound.Pronunciation{
			{Text: "/ɹəʊˈbʌst/", AudioURL: "https://example.com/uk.mp3"},
			{Text: "/ɹoʊˈbʌst/"},
		},
		Attribution: outbound.Attribution{
			SourceName: "Wiktionary", SourceURL: "https://en.wiktionary.org/wiki/robust",
			License: "CC BY-SA 3.0", LicenseURL: "https://creativecommons.org/licenses/by-sa/3.0",
		},
		Senses: []outbound.WordSense{{
			PartOfSpeech: "adjective",
			// Each example stays with its own definition.
			Definitions: []outbound.Definition{
				{Text: "Strong and healthy.", Example: "A robust constitution."},
				{Text: "Able to withstand stress."},
			},
			Synonyms: []string{"tough", "sturdy", "Strong", "resilient"},
			Antonyms: []string{"weak"},
		}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mismatch:\n got %+v\nwant %+v", got, want)
	}
}

func TestLookupErrors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		want   error
	}{
		{"not found", 404, outbound.ErrNotFound},
		{"rate limited", 429, outbound.ErrRateLimited},
		{"server error", 500, outbound.ErrProviderUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := newProvider(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				w.Write([]byte(`{"title":"No Definitions Found"}`))
			})
			p.client.BaseDelay = 0
			p.client.MaxAttempts = 1

			if _, err := p.Lookup(context.Background(), "x", vocab.LanguageCodeEnglish); !errors.Is(err, tc.want) {
				t.Errorf("expected %v, got %v", tc.want, err)
			}
		})
	}
}

func TestLookupUnsupportedLanguageMakesNoRequest(t *testing.T) {
	p := newProvider(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request expected")
	})
	if _, err := p.Lookup(context.Background(), "x", vocab.LanguageCodePersian); !errors.Is(err, outbound.ErrUnsupportedLanguage) {
		t.Errorf("expected ErrUnsupportedLanguage, got %v", err)
	}
}

func TestSourceName(t *testing.T) {
	for in, want := range map[string]string{
		"https://en.wiktionary.org/wiki/robust": "Wiktionary",
		"https://wiktionary.org/x":              "Wiktionary",
		"https://example.com/robust":            "example.com",
		"not a url":                             "",
		"":                                      "",
	} {
		if got := sourceName(in); got != want {
			t.Errorf("sourceName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExplicitPhoneticIsKeptAndListedFirst(t *testing.T) {
	p := newProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"word":"x","phonetic":"/main/","phonetics":[{"text":"/other/","audio":"a.mp3"}],"meanings":[{"partOfSpeech":"noun","definitions":[{"definition":"d"}]}]}]`))
	})
	got, err := p.Lookup(context.Background(), "x", vocab.LanguageCodeEnglish)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Phonetic != "/main/" || len(got[0].Pronunciations) != 2 || got[0].Pronunciations[0].Text != "/main/" {
		t.Errorf("unexpected %+v", got[0])
	}
	if got[0].Attribution != (outbound.Attribution{}) {
		t.Errorf("no attribution expected when the API gives none: %+v", got[0].Attribution)
	}
}

func TestBestSourceURLPrefersThePageForTheWord(t *testing.T) {
	urls := []string{"https://en.wiktionary.org/wiki/rin", "https://en.wiktionary.org/wiki/run"}
	if got := bestSourceURL(urls, "run"); got != "https://en.wiktionary.org/wiki/run" {
		t.Errorf("got %q", got)
	}
	if got := bestSourceURL(urls, "RUN"); got != "https://en.wiktionary.org/wiki/run" {
		t.Errorf("case-insensitive match failed: %q", got)
	}
	multi := []string{"https://en.wiktionary.org/wiki/ice", "https://en.wiktionary.org/wiki/ice_cream"}
	if got := bestSourceURL(multi, "ice cream"); got != multi[1] {
		t.Errorf("multi-word term should match its underscored page, got %q", got)
	}
	if got := bestSourceURL(urls, "walk"); got != urls[0] {
		t.Errorf("must fall back to the first URL, got %q", got)
	}
	if got := bestSourceURL(nil, "x"); got != "" {
		t.Errorf("got %q", got)
	}
}
