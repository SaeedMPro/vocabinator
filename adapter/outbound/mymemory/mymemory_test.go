package mymemory

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SaeedMPro/vocabinator/internal/application/port/outbound"
)

const secret = "my-private-sentence-xyz"

// fake serves MyMemory-shaped responses. handler receives the decoded query.
func fake(t *testing.T, handler func(q map[string]string) string) (*Provider, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := map[string]string{}
		for k := range r.URL.Query() {
			q[k] = r.URL.Query().Get(k)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, handler(q))
	}))
	t.Cleanup(srv.Close)

	p, err := New(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	p.client.BaseDelay = time.Millisecond
	return p, srv
}

func ok(text string) string {
	return fmt.Sprintf(`{"responseData":{"translatedText":%q,"match":0.85},"quotaFinished":false,"responseDetails":"","responseStatus":200}`, text)
}

func fail(status, details string) string {
	return fmt.Sprintf(`{"responseData":{"translatedText":%q},"quotaFinished":null,"responseDetails":%q,"responseStatus":%q}`, details, details, status)
}

func TestTranslateSuccessAndRequestShape(t *testing.T) {
	var got map[string]string
	p, _ := fake(t, func(q map[string]string) string { got = q; return ok("قوی و سالم؛ نیرومند.") })

	out, err := p.Translate(context.Background(), []string{"Strong and healthy; vigorous."}, "en", "fa")
	if err != nil || len(out) != 1 || out[0] != "قوی و سالم؛ نیرومند." {
		t.Fatalf("out=%v err=%v", out, err)
	}
	if got["q"] != "Strong and healthy; vigorous." || got["langpair"] != "en|fa" {
		t.Errorf("unexpected query %v", got)
	}
	if _, sent := got["de"]; sent {
		t.Error("no email must be sent unless configured")
	}
}

func TestEmailIsSentOnlyWhenConfigured(t *testing.T) {
	var got map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = map[string]string{"de": r.URL.Query().Get("de")}
		fmt.Fprint(w, ok("x"))
	}))
	defer srv.Close()

	p, err := New(Config{BaseURL: srv.URL, Email: " me@example.com "})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Translate(context.Background(), []string{"hi"}, "en", "fa"); err != nil || got["de"] != "me@example.com" {
		t.Errorf("err=%v got=%v", err, got)
	}
}

func TestNewValidatesConfig(t *testing.T) {
	for name, cfg := range map[string]Config{
		"bad email":  {Email: "not-an-email"},
		"bad url":    {BaseURL: "::nope"},
		"bad scheme": {BaseURL: "ftp://example.com"},
	} {
		if _, err := New(cfg); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if _, err := New(Config{}); err != nil {
		t.Errorf("defaults must be valid: %v", err)
	}
}

func TestMultipleTextsKeepOrderAndRunConcurrently(t *testing.T) {
	var inflight, peak atomic.Int32
	p, _ := fake(t, func(q map[string]string) string {
		n := inflight.Add(1)
		for {
			old := peak.Load()
			if n <= old || peak.CompareAndSwap(old, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		inflight.Add(-1)
		return ok("fa:" + q["q"])
	})

	texts := []string{"one", "two", "three", "four", "five", "six"}
	out, err := p.Translate(context.Background(), texts, "en", "fa")
	if err != nil {
		t.Fatal(err)
	}
	for i, tx := range texts {
		if out[i] != "fa:"+tx {
			t.Errorf("order broken at %d: %v", i, out)
		}
	}
	if peak.Load() > int32(defaultConcurrency) {
		t.Errorf("concurrency limit exceeded: %d", peak.Load())
	}
	if peak.Load() < 2 {
		t.Errorf("expected some parallelism, peak=%d", peak.Load())
	}
}

func TestEmptyTextsMakeNoRequest(t *testing.T) {
	var calls atomic.Int32
	p, _ := fake(t, func(map[string]string) string { calls.Add(1); return ok("x") })

	out, err := p.Translate(context.Background(), []string{"", "  \n", "hi"}, "en", "fa")
	if err != nil || out[0] != "" || out[1] != "" || out[2] != "x" || calls.Load() != 1 {
		t.Errorf("out=%q err=%v calls=%d", out, err, calls.Load())
	}

	if out, err := p.Translate(context.Background(), nil, "en", "fa"); err != nil || len(out) != 0 {
		t.Errorf("no texts: %v %v", out, err)
	}
}

func TestLongTextIsSplitAndRejoined(t *testing.T) {
	var mu sync.Mutex
	var lengths []int
	p, _ := fake(t, func(q map[string]string) string {
		mu.Lock()
		lengths = append(lengths, len([]rune(q["q"])))
		mu.Unlock()
		return ok("[" + q["q"][:3] + "]")
	})

	long := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 30) // ~1350 chars
	out, err := p.Translate(context.Background(), []string{long}, "en", "fa")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range lengths {
		if n > maxChunkRunes {
			t.Errorf("chunk of %d runes exceeds the limit", n)
		}
	}
	if len(lengths) < 3 || strings.Count(out[0], "[The]") != len(lengths) {
		t.Errorf("chunks=%d out=%q", len(lengths), out[0])
	}
}

func TestSplitText(t *testing.T) {
	if got := splitText("short", 10); len(got) != 1 || got[0] != "short" {
		t.Errorf("short: %v", got)
	}
	if got := splitText("", 10); len(got) != 1 || got[0] != "" {
		t.Errorf("empty: %v", got)
	}

	// Prefers sentence boundaries.
	got := splitText("First sentence. Second one here. Third.", 20)
	if len(got) < 2 || got[0] != "First sentence." {
		t.Errorf("sentence split: %q", got)
	}

	// Falls back to spaces, then to a hard cut.
	for _, in := range []string{strings.Repeat("word ", 40), strings.Repeat("x", 95), strings.Repeat("مقاوم ", 30)} {
		pieces := splitText(in, 20)
		var rejoined []string
		for _, p := range pieces {
			if n := len([]rune(p)); n > 20 || n == 0 {
				t.Errorf("bad piece length %d in %q", n, pieces)
			}
			rejoined = append(rejoined, p)
		}
		if strings.ReplaceAll(strings.Join(rejoined, ""), " ", "") != strings.ReplaceAll(in, " ", "") {
			t.Errorf("content lost splitting %q", in)
		}
	}
}

func TestResponseStatusAsStringOrInt(t *testing.T) {
	for _, st := range []string{`200`, `"200"`} {
		body := fmt.Sprintf(`{"responseData":{"translatedText":"ok"},"responseDetails":"","responseStatus":%s}`, st)
		p, _ := fake(t, func(map[string]string) string { return body })
		if out, err := p.Translate(context.Background(), []string{"x"}, "en", "fa"); err != nil || out[0] != "ok" {
			t.Errorf("status %s: %v %v", st, out, err)
		}
	}
}

func TestHTMLEntitiesAreDecoded(t *testing.T) {
	p, _ := fake(t, func(map[string]string) string { return ok("a &quot;b&quot; &amp; c &#39;d&#39;") })
	out, _ := p.Translate(context.Background(), []string{"x"}, "en", "fa")
	if out[0] != `a "b" & c 'd'` {
		t.Errorf("got %q", out[0])
	}
}

func TestProviderFailuresAreClassified(t *testing.T) {
	cases := []struct {
		name string
		body string
		want error
	}{
		{"quota status", fail("429", "MYMEMORY WARNING: YOU USED ALL AVAILABLE FREE TRANSLATIONS FOR TODAY"), outbound.ErrRateLimited},
		{"quota flag", `{"responseData":{"translatedText":"x"},"quotaFinished":true,"responseStatus":200}`, outbound.ErrRateLimited},
		{"quota warning in text", ok("MYMEMORY WARNING: YOU USED ALL AVAILABLE FREE TRANSLATIONS"), outbound.ErrRateLimited},
		{"invalid language", fail("403", "'XX' IS AN INVALID TARGET LANGUAGE . EXAMPLE: LANGPAIR=EN|IT"), outbound.ErrUnsupportedLanguage},
		{"same language", fail("403", "PLEASE SELECT TWO DISTINCT LANGUAGES"), outbound.ErrUnsupportedLanguage},
		{"length limit", fail("403", "QUERY LENGTH LIMIT EXCEEDED. MAX ALLOWED QUERY : 500 CHARS"), outbound.ErrProviderError},
		{"server status in body", fail("503", "try later"), outbound.ErrProviderUnavailable},
		{"empty translation", ok("   "), outbound.ErrInvalidResponse},
		{"missing status", `{"responseData":{"translatedText":"x"}}`, outbound.ErrInvalidResponse},
		{"garbage status", `{"responseData":{"translatedText":"x"},"responseStatus":"abc"}`, outbound.ErrInvalidResponse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, _ := fake(t, func(map[string]string) string { return tc.body })
			_, err := p.Translate(context.Background(), []string{"x"}, "en", "fa")
			if !errors.Is(err, tc.want) {
				t.Errorf("expected %v, got %v", tc.want, err)
			}
		})
	}
}

func TestSameLanguageIsRejectedWithoutARequest(t *testing.T) {
	p, _ := fake(t, func(map[string]string) string { t.Error("no request expected"); return "" })
	if _, err := p.Translate(context.Background(), []string{"x"}, "en", "en"); !errors.Is(err, outbound.ErrUnsupportedLanguage) {
		t.Errorf("got %v", err)
	}
}

func TestHTTPLevelFailures(t *testing.T) {
	for status, want := range map[int]error{
		429: outbound.ErrRateLimited,
		500: outbound.ErrProviderUnavailable,
		503: outbound.ErrProviderUnavailable,
		400: outbound.ErrProviderError,
		404: outbound.ErrProviderError,
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
		p, _ := New(Config{BaseURL: srv.URL})
		p.client.BaseDelay = time.Millisecond
		if _, err := p.Translate(context.Background(), []string{"x"}, "en", "fa"); !errors.Is(err, want) {
			t.Errorf("status %d: expected %v, got %v", status, want, err)
		}
		srv.Close()
	}
}

func TestRetriesAreBounded(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	p, _ := New(Config{BaseURL: srv.URL})
	p.client.BaseDelay = time.Millisecond
	p.Translate(context.Background(), []string{"x"}, "en", "fa")
	if calls.Load() != 2 {
		t.Errorf("expected exactly 2 attempts (1 retry), got %d", calls.Load())
	}
}

func TestQuotaExhaustionIsNotRetried(t *testing.T) {
	var calls atomic.Int32
	p, _ := fake(t, func(map[string]string) string {
		calls.Add(1)
		return fail("429", "MYMEMORY WARNING: YOU USED ALL AVAILABLE FREE TRANSLATIONS FOR TODAY")
	})
	p.Translate(context.Background(), []string{"x"}, "en", "fa")
	if calls.Load() != 1 {
		t.Errorf("quota exhaustion must not be retried, got %d calls", calls.Load())
	}
}

func TestFirstFailureStopsRemainingRequests(t *testing.T) {
	var calls atomic.Int32
	p, _ := fake(t, func(map[string]string) string {
		calls.Add(1)
		return fail("429", "MYMEMORY WARNING: YOU USED ALL AVAILABLE FREE TRANSLATIONS FOR TODAY")
	})
	p.concurrency = 1

	texts := make([]string, 20)
	for i := range texts {
		texts[i] = fmt.Sprint("t", i)
	}
	if _, err := p.Translate(context.Background(), texts, "en", "fa"); !errors.Is(err, outbound.ErrRateLimited) {
		t.Fatalf("got %v", err)
	}
	if calls.Load() > 2 {
		t.Errorf("remaining requests should be abandoned after the first failure, got %d calls", calls.Load())
	}
}

func TestInvalidJSON(t *testing.T) {
	for name, body := range map[string]string{"not json": `<html>oops</html>`, "truncated": `{"responseData":`, "wrong types": `{"responseData":"str","responseStatus":200}`} {
		p, _ := fake(t, func(map[string]string) string { return body })
		if _, err := p.Translate(context.Background(), []string{"x"}, "en", "fa"); !errors.Is(err, outbound.ErrInvalidResponse) {
			t.Errorf("%s: expected ErrInvalidResponse, got %v", name, err)
		}
	}
}

func TestNetworkFailureIsClassifiedAndDoesNotLeakText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // connection refused

	p, _ := New(Config{BaseURL: url})
	p.client.BaseDelay = time.Millisecond

	_, err := p.Translate(context.Background(), []string{secret}, "en", "fa")
	if !errors.Is(err, outbound.ErrNetwork) {
		t.Fatalf("expected ErrNetwork, got %v", err)
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "q=") {
		t.Errorf("error leaks the request URL or user text: %v", err)
	}
}

func TestContextCancellationAndTimeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-release }))
	defer srv.Close()
	defer close(release)

	p, _ := New(Config{BaseURL: srv.URL, Timeout: 5 * time.Second})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := p.Translate(ctx, []string{secret}, "en", "fa")
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 2*time.Second {
		t.Errorf("err=%v after %v", err, time.Since(start))
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("error leaks user text: %v", err)
	}

	ctx2, cancel2 := context.WithCancel(context.Background())
	cancel2()
	if _, err := p.Translate(ctx2, []string{"x"}, "en", "fa"); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}

	// The client's own timeout also produces a clean error.
	slow, _ := New(Config{BaseURL: srv.URL, Timeout: 30 * time.Millisecond})
	slow.client.MaxAttempts = 1
	if _, err := slow.Translate(context.Background(), []string{secret}, "en", "fa"); err == nil || strings.Contains(err.Error(), secret) {
		t.Errorf("timeout error: %v", err)
	}
}
