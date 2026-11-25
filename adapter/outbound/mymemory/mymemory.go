// Package mymemory adapts the MyMemory translation API
// (https://mymemory.translated.net) to the TranslationProvider port.
//
// Provider behaviour this adapter is built around (observed against the live
// API on 24/11/2026; see docs/decisions/0002-translation-provider.md):
//
//   - Failures are reported with HTTP 200 and a body whose responseStatus is
//     a string ("403"), while success carries an integer (200).
//   - A query may be at most 500 characters, so long text is split at
//     sentence boundaries and translated in pieces.
//   - Anonymous use has a small daily quota that is tracked per IP address.
//     A valid email in the "de" parameter raises it (figures from third-party
//     summaries of the provider's documentation: 5,000 and 50,000 characters
//     per day; unverified against the official page).
//   - Isolated words translate poorly. Callers should translate definitions
//     and sentences, not bare headwords.
//
// User text is sent to the provider. It is never logged, and it is scrubbed
// from error messages.
package mymemory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SaeedMPro/vocabinator/adapter/outbound/internal/httpjson"
	"github.com/SaeedMPro/vocabinator/internal/application/port/outbound"
	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

const (
	DefaultBaseURL = "https://api.mymemory.translated.net/get"

	// maxChunkRunes stays below the provider's 500 character query limit.
	maxChunkRunes = 450

	// defaultConcurrency bounds parallel requests: enough to keep a quick
	// lookup fast, low enough to be a polite client.
	defaultConcurrency = 3
)

type Config struct {
	BaseURL string // default DefaultBaseURL
	// Email is optional. When set it is sent as the "de" parameter, which the
	// provider uses to raise the daily quota. It is personal data: only set
	// it deliberately.
	Email       string
	Timeout     time.Duration // per request; default 10s
	Concurrency int           // parallel requests; default 3
}

type Provider struct {
	baseURL     string
	email       string
	concurrency int
	client      *httpjson.Client
}

var _ outbound.TranslationProvider = (*Provider)(nil)

func New(cfg Config) (*Provider, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if u, err := url.Parse(cfg.BaseURL); err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, fmt.Errorf("mymemory: invalid base URL %q", cfg.BaseURL)
	}

	email := strings.TrimSpace(cfg.Email)
	if email != "" {
		if _, err := mail.ParseAddress(email); err != nil {
			return nil, errors.New("mymemory: the configured email address is not valid")
		}
	}

	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = defaultConcurrency
	}

	client := httpjson.New(cfg.Timeout, "vocabinator")
	// Quota exhaustion is reported in the body, not by status code, so retries
	// only ever cover transport failures and 5xx. Keep them to one extra try.
	client.MaxAttempts = 2

	return &Provider{baseURL: cfg.BaseURL, email: email, concurrency: cfg.Concurrency, client: client}, nil
}

type response struct {
	ResponseData struct {
		TranslatedText string `json:"translatedText"`
	} `json:"responseData"`
	QuotaFinished   *bool           `json:"quotaFinished"`
	ResponseDetails string          `json:"responseDetails"`
	ResponseStatus  json.RawMessage `json:"responseStatus"`
}

// Translate returns one translation per text, in order. Empty texts are
// returned unchanged without a request. The first failure aborts the rest.
func (p *Provider) Translate(ctx context.Context, texts []string, source, target vocab.LanguageCode) ([]string, error) {
	if source == target {
		return nil, fmt.Errorf("%w: source and target language are both %q", outbound.ErrUnsupportedLanguage, source)
	}
	langpair := string(source) + "|" + string(target)

	type piece struct {
		text  int
		chunk int
		body  string
	}

	results := make([][]string, len(texts))
	var pieces []piece
	for i, t := range texts {
		chunks := splitText(strings.TrimSpace(t), maxChunkRunes)
		results[i] = make([]string, len(chunks))
		for j, c := range chunks {
			if strings.TrimSpace(c) == "" {
				continue // nothing to translate; the result stays ""
			}
			pieces = append(pieces, piece{text: i, chunk: j, body: c})
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
		sem      = make(chan struct{}, p.concurrency)
	)

	for _, pc := range pieces {
		sem <- struct{}{}
		if ctx.Err() != nil {
			<-sem
			break
		}
		wg.Add(1)
		go func(pc piece) {
			defer wg.Done()
			defer func() { <-sem }()

			out, err := p.translateChunk(ctx, pc.body, langpair)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = err
					cancel()
				}
				return
			}
			results[pc.text][pc.chunk] = out
		}(pc)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	out := make([]string, len(texts))
	for i, chunks := range results {
		out[i] = strings.Join(chunks, " ")
	}
	return out, nil
}

func (p *Provider) translateChunk(ctx context.Context, text, langpair string) (string, error) {
	q := url.Values{}
	q.Set("q", text)
	q.Set("langpair", langpair)
	if p.email != "" {
		q.Set("de", p.email)
	}

	var resp response
	if err := p.client.Do(ctx, http.MethodGet, p.baseURL+"?"+q.Encode(), nil, nil, &resp); err != nil {
		return "", mapTransportError(err)
	}
	return interpret(resp)
}

// interpret validates a decoded response and maps provider-level failures.
func interpret(resp response) (string, error) {
	status, err := parseStatus(resp.ResponseStatus)
	if err != nil {
		return "", err
	}

	text := resp.ResponseData.TranslatedText
	details := strings.TrimSpace(resp.ResponseDetails)
	upper := strings.ToUpper(details + " " + text)

	quotaGone := status == http.StatusTooManyRequests ||
		(resp.QuotaFinished != nil && *resp.QuotaFinished) ||
		strings.HasPrefix(strings.ToUpper(strings.TrimSpace(text)), "MYMEMORY WARNING")
	if quotaGone {
		return "", fmt.Errorf("%w: daily translation quota used up (set an email to raise it, or try again later)", outbound.ErrRateLimited)
	}

	if status != http.StatusOK {
		switch {
		case strings.Contains(upper, "INVALID") && strings.Contains(upper, "LANGUAGE"),
			strings.Contains(upper, "DISTINCT LANGUAGES"):
			return "", fmt.Errorf("%w: %s", outbound.ErrUnsupportedLanguage, truncate(details, 160))
		case status >= 500:
			return "", fmt.Errorf("%w: provider status %d", outbound.ErrProviderUnavailable, status)
		}
		return "", fmt.Errorf("%w: status %d: %s", outbound.ErrProviderError, status, truncate(details, 160))
	}

	text = strings.TrimSpace(html.UnescapeString(text))
	if text == "" {
		return "", fmt.Errorf("%w: empty translation", outbound.ErrInvalidResponse)
	}
	return text, nil
}

// parseStatus accepts the integer form (200) and the string form ("403").
func parseStatus(raw json.RawMessage) (int, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, fmt.Errorf("%w: missing responseStatus", outbound.ErrInvalidResponse)
	}
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			return n, nil
		}
	}
	return 0, fmt.Errorf("%w: unreadable responseStatus", outbound.ErrInvalidResponse)
}

// mapTransportError classifies client errors. The request URL carries the
// user's text in its query string, so it is stripped from every message.
func mapTransportError(err error) error {
	var se *httpjson.StatusError
	switch {
	case errors.As(err, &se):
		switch {
		case se.Status == http.StatusTooManyRequests:
			return fmt.Errorf("%w: HTTP %d", outbound.ErrRateLimited, se.Status)
		case se.Status >= 500:
			return fmt.Errorf("%w: HTTP %d", outbound.ErrProviderUnavailable, se.Status)
		}
		return fmt.Errorf("%w: HTTP %d", outbound.ErrProviderError, se.Status)

	case httpjson.IsDecode(err):
		return fmt.Errorf("%w: body is not the expected JSON", outbound.ErrInvalidResponse)

	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return scrub(err)

	case httpjson.IsTransport(err):
		return fmt.Errorf("%w: %w", outbound.ErrNetwork, scrub(err))
	}
	return fmt.Errorf("mymemory: %w", scrub(err))
}

// scrub replaces *url.Error (which embeds the full URL) with its cause.
func scrub(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
