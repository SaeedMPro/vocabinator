// Package googletranslate adapts Google Cloud Translation (Basic, v2 REST) to
// the TranslationProvider port.
package googletranslate

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/SaeedMPro/vocabinator/adapter/outbound/internal/httpjson"
	"github.com/SaeedMPro/vocabinator/internal/application/port/outbound"
	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

const (
	DefaultBaseURL = "https://translation.googleapis.com/language/translate/v2"
	// maxBatch is below the API's per-request text limit.
	maxBatch = 100
)

type Config struct {
	APIKey  string        // required; sent in a header, never in the URL
	BaseURL string        // default DefaultBaseURL
	Timeout time.Duration // per request; default 8s
}

type Provider struct {
	apiKey  string
	baseURL string
	client  *httpjson.Client
}

var _ outbound.TranslationProvider = (*Provider)(nil)

func New(cfg Config) (*Provider, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("%w: google translation API key is not configured", outbound.ErrUnauthorized)
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 8 * time.Second
	}
	return &Provider{
		apiKey:  cfg.APIKey,
		baseURL: cfg.BaseURL,
		client:  httpjson.New(cfg.Timeout, "vocabinator"),
	}, nil
}

type request struct {
	Q      []string `json:"q"`
	Source string   `json:"source"`
	Target string   `json:"target"`
	Format string   `json:"format"`
}

type response struct {
	Data struct {
		Translations []struct {
			TranslatedText string `json:"translatedText"`
		} `json:"translations"`
	} `json:"data"`
}

func (p *Provider) Translate(ctx context.Context, texts []string, source, target vocab.LanguageCode) ([]string, error) {
	result := make([]string, 0, len(texts))

	for start := 0; start < len(texts); start += maxBatch {
		batch := texts[start:min(start+maxBatch, len(texts))]

		var resp response
		header := http.Header{"X-Goog-Api-Key": []string{p.apiKey}}
		body := request{Q: batch, Source: string(source), Target: string(target), Format: "text"}

		if err := p.client.Do(ctx, http.MethodPost, p.baseURL, header, body, &resp); err != nil {
			return nil, mapError(err)
		}
		if len(resp.Data.Translations) != len(batch) {
			return nil, fmt.Errorf("google translate returned %d translations for %d texts", len(resp.Data.Translations), len(batch))
		}
		for _, t := range resp.Data.Translations {
			result = append(result, t.TranslatedText)
		}
	}

	return result, nil
}

func mapError(err error) error {
	var se *httpjson.StatusError
	if errors.As(err, &se) {
		switch {
		case se.Status == http.StatusUnauthorized || se.Status == http.StatusForbidden:
			return fmt.Errorf("%w: %w", outbound.ErrUnauthorized, err)
		case se.Status == http.StatusTooManyRequests:
			return fmt.Errorf("%w: %w", outbound.ErrRateLimited, err)
		case se.Status >= 500:
			return fmt.Errorf("%w: %w", outbound.ErrProviderUnavailable, err)
		}
	}
	return fmt.Errorf("google translate: %w", err)
}
