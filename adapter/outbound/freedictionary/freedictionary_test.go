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
  "phonetics": [{"audio": "x.mp3"}, {"text": "/roʊˈbʌst/", "audio": "y.mp3"}],
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
		Term:     "robust",
		Phonetic: "/roʊˈbʌst/",
		Senses: []outbound.WordSense{{
			PartOfSpeech: "adjective",
			Definitions:  []string{"Strong and healthy.", "Able to withstand stress."},
			Examples:     []string{"A robust constitution."},
			Synonyms:     []string{"tough", "sturdy", "Strong", "resilient"},
			Antonyms:     []string{"weak"},
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
