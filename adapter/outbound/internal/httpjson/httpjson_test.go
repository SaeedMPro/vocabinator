package httpjson

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func newClient() *Client {
	c := New(time.Second, "test-agent")
	c.BaseDelay = time.Millisecond
	return c
}

func TestDoSuccessAndHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "test-agent" || r.Header.Get("X-Test") != "1" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected headers: %v", r.Header)
		}
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	var out struct{ OK bool }
	err := newClient().Do(context.Background(), http.MethodPost, srv.URL, http.Header{"X-Test": {"1"}}, map[string]int{"a": 1}, &out)
	if err != nil || !out.OK {
		t.Fatalf("unexpected: %v %+v", err, out)
	}
}

func TestDoRetriesTransientStatuses(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	var out map[string]any
	if err := newClient().Do(context.Background(), http.MethodGet, srv.URL, nil, nil, &out); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 {
		t.Errorf("expected 3 calls, got %d", calls.Load())
	}
}

func TestDoGivesUpAfterMaxAttempts(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	err := newClient().Do(context.Background(), http.MethodGet, srv.URL, nil, nil, &struct{}{})
	var se *StatusError
	if !errors.As(err, &se) || se.Status != 503 || calls.Load() != 3 {
		t.Errorf("err=%v calls=%d", err, calls.Load())
	}
}

func TestDoDoesNotRetryClientErrorsOrBadJSON(t *testing.T) {
	for name, handler := range map[string]http.HandlerFunc{
		"404":      func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) },
		"bad json": func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{`)) },
	} {
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				handler(w, r)
			}))
			defer srv.Close()

			if err := newClient().Do(context.Background(), http.MethodGet, srv.URL, nil, nil, &struct{}{}); err == nil {
				t.Fatal("expected error")
			}
			if calls.Load() != 1 {
				t.Errorf("expected 1 call, got %d", calls.Load())
			}
		})
	}
}

func TestDoRetriesTransportErrorsAndHonoursContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // connection refused

	if err := newClient().Do(context.Background(), http.MethodGet, url, nil, nil, &struct{}{}); err == nil {
		t.Fatal("expected transport error")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := newClient().Do(ctx, http.MethodGet, url, nil, nil, &struct{}{}); err == nil {
		t.Fatal("expected context error")
	}
}

func TestDoTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()

	c := newClient()
	c.HTTP.Timeout = 20 * time.Millisecond
	c.MaxAttempts = 1
	if err := c.Do(context.Background(), http.MethodGet, srv.URL, nil, nil, &struct{}{}); err == nil {
		t.Fatal("expected timeout")
	}
}
