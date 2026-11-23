// Package freedictionary adapts https://dictionaryapi.dev to the
// DictionaryProvider port. Only English is supported by this adapter.
package freedictionary

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/SaeedMPro/vocabinator/adapter/outbound/internal/httpjson"
	"github.com/SaeedMPro/vocabinator/internal/application/port/outbound"
	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

const DefaultBaseURL = "https://api.dictionaryapi.dev/api/v2/entries"

type Config struct {
	BaseURL string        // default DefaultBaseURL
	Timeout time.Duration // per request; default 8s
}

type Provider struct {
	baseURL string
	client  *httpjson.Client
}

var _ outbound.DictionaryProvider = (*Provider)(nil)

func New(cfg Config) *Provider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 8 * time.Second
	}
	return &Provider{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		client:  httpjson.New(cfg.Timeout, "vocabinator"),
	}
}

// Provider JSON; never leaves this package.
type entryDTO struct {
	Word      string `json:"word"`
	Phonetic  string `json:"phonetic"`
	Phonetics []struct {
		Text string `json:"text"`
	} `json:"phonetics"`
	Meanings []struct {
		PartOfSpeech string `json:"partOfSpeech"`
		Definitions  []struct {
			Definition string   `json:"definition"`
			Example    string   `json:"example"`
			Synonyms   []string `json:"synonyms"`
			Antonyms   []string `json:"antonyms"`
		} `json:"definitions"`
		Synonyms []string `json:"synonyms"`
		Antonyms []string `json:"antonyms"`
	} `json:"meanings"`
}

func (p *Provider) Lookup(ctx context.Context, term string, language vocab.LanguageCode) ([]outbound.DictionaryEntry, error) {
	if language != vocab.LanguageCodeEnglish {
		return nil, fmt.Errorf("%w: %s (freedictionary supports en)", outbound.ErrUnsupportedLanguage, language)
	}

	endpoint := fmt.Sprintf("%s/%s/%s", p.baseURL, language, url.PathEscape(term))

	var dtos []entryDTO
	if err := p.client.Do(ctx, http.MethodGet, endpoint, nil, nil, &dtos); err != nil {
		return nil, mapError(err)
	}

	entries := make([]outbound.DictionaryEntry, 0, len(dtos))
	for _, d := range dtos {
		entries = append(entries, toEntry(d))
	}
	return entries, nil
}

func mapError(err error) error {
	var se *httpjson.StatusError
	if errors.As(err, &se) {
		switch {
		case se.Status == http.StatusNotFound:
			return fmt.Errorf("%w: %w", outbound.ErrNotFound, err)
		case se.Status == http.StatusTooManyRequests:
			return fmt.Errorf("%w: %w", outbound.ErrRateLimited, err)
		case se.Status >= 500:
			return fmt.Errorf("%w: %w", outbound.ErrProviderUnavailable, err)
		}
	}
	return fmt.Errorf("freedictionary lookup: %w", err)
}

func toEntry(d entryDTO) outbound.DictionaryEntry {
	e := outbound.DictionaryEntry{Term: d.Word, Phonetic: d.Phonetic}
	if e.Phonetic == "" {
		for _, ph := range d.Phonetics {
			if ph.Text != "" {
				e.Phonetic = ph.Text
				break
			}
		}
	}

	for _, m := range d.Meanings {
		sense := outbound.WordSense{PartOfSpeech: m.PartOfSpeech}
		syn := newSet(m.Synonyms)
		ant := newSet(m.Antonyms)

		for _, def := range m.Definitions {
			if def.Definition != "" {
				sense.Definitions = append(sense.Definitions, def.Definition)
			}
			if def.Example != "" {
				sense.Examples = append(sense.Examples, def.Example)
			}
			syn.add(def.Synonyms)
			ant.add(def.Antonyms)
		}

		sense.Synonyms, sense.Antonyms = syn.list, ant.list
		if len(sense.Definitions) > 0 {
			e.Senses = append(e.Senses, sense)
		}
	}
	return e
}

// set is an insertion-ordered, de-duplicating string collection.
type set struct {
	seen map[string]bool
	list []string
}

func newSet(initial []string) *set {
	s := &set{seen: map[string]bool{}}
	s.add(initial)
	return s
}

func (s *set) add(values []string) {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" && !s.seen[strings.ToLower(v)] {
			s.seen[strings.ToLower(v)] = true
			s.list = append(s.list, v)
		}
	}
}
