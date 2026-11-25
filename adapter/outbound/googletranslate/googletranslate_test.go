package googletranslate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SaeedMPro/vocabinator/internal/application/port/outbound"
)

func TestNewRequiresAPIKey(t *testing.T) {
	if _, err := New(Config{APIKey: "  "}); !errors.Is(err, outbound.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func newProvider(t *testing.T, h http.HandlerFunc) *Provider {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	p, err := New(Config{APIKey: "secret-key", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	p.client.BaseDelay = 0
	p.client.MaxAttempts = 1
	return p
}

func echoHandler(t *testing.T, batches *[]int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Goog-Api-Key") != "secret-key" {
			t.Error("api key header missing")
		}
		if strings.Contains(r.URL.String(), "secret-key") {
			t.Error("api key leaked into URL")
		}
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Source != "en" || req.Target != "fa" || req.Format != "text" {
			t.Errorf("unexpected request %+v", req)
		}
		*batches = append(*batches, len(req.Q))

		var resp response
		for _, q := range req.Q {
			resp.Data.Translations = append(resp.Data.Translations, struct {
				TranslatedText string `json:"translatedText"`
			}{"fa:" + q})
		}
		json.NewEncoder(w).Encode(resp)
	}
}

func TestTranslate(t *testing.T) {
	var batches []int
	p := newProvider(t, echoHandler(t, &batches))

	got, err := p.Translate(context.Background(), []string{"robust", "strong"}, "en", "fa")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "fa:robust" || got[1] != "fa:strong" {
		t.Errorf("unexpected: %v", got)
	}
}

func TestTranslateSplitsLargeBatches(t *testing.T) {
	var batches []int
	p := newProvider(t, echoHandler(t, &batches))

	texts := make([]string, 250)
	for i := range texts {
		texts[i] = fmt.Sprint(i)
	}
	got, err := p.Translate(context.Background(), texts, "en", "fa")
	if err != nil || len(got) != 250 || got[249] != "fa:249" {
		t.Fatalf("err=%v len=%d", err, len(got))
	}
	if fmt.Sprint(batches) != "[100 100 50]" {
		t.Errorf("unexpected batches %v", batches)
	}
}

func TestTranslateErrors(t *testing.T) {
	for status, want := range map[int]error{
		401: outbound.ErrUnauthorized,
		403: outbound.ErrUnauthorized,
		429: outbound.ErrRateLimited,
		503: outbound.ErrProviderUnavailable,
	} {
		p := newProvider(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) })
		if _, err := p.Translate(context.Background(), []string{"x"}, "en", "fa"); !errors.Is(err, want) {
			t.Errorf("status %d: expected %v, got %v", status, want, err)
		}
	}
}

func TestTranslateRejectsCountMismatch(t *testing.T) {
	p := newProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{"translations":[]}}`))
	})
	if _, err := p.Translate(context.Background(), []string{"x"}, "en", "fa"); err == nil {
		t.Error("expected mismatch error")
	}
}
