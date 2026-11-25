// Package httpjson is a small JSON-over-HTTP client with timeouts, bounded
// retries and Retry-After awareness, shared by the provider adapters.
package httpjson

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	maxResponseBytes = 4 << 20
	maxRetryDelay    = 5 * time.Second
)

// StatusError is returned for non-2xx responses.
type StatusError struct {
	Status int
	Body   string // truncated
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("unexpected HTTP status %d: %s", e.Status, e.Body)
}

type Client struct {
	HTTP        *http.Client
	UserAgent   string
	MaxAttempts int           // total attempts including the first; minimum 1
	BaseDelay   time.Duration // backoff base; doubled per retry
}

// New returns a client with a per-request timeout and sensible retry policy.
func New(timeout time.Duration, userAgent string) *Client {
	return &Client{
		HTTP:        &http.Client{Timeout: timeout},
		UserAgent:   userAgent,
		MaxAttempts: 3,
		BaseDelay:   250 * time.Millisecond,
	}
}

// Do sends the request (JSON-encoding body when non-nil) and decodes a 2xx
// response into out. 429, 5xx and transport errors are retried.
func (c *Client) Do(ctx context.Context, method, url string, header http.Header, body, out any) error {
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
	}

	attempts := max(c.MaxAttempts, 1)
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		retryAfter, err := c.once(ctx, method, url, header, payload, out)
		if err == nil {
			return nil
		}
		lastErr = err

		if attempt == attempts || !retryable(err) || ctx.Err() != nil {
			break
		}

		delay := retryAfter
		if delay <= 0 {
			delay = c.BaseDelay << (attempt - 1)
		}
		delay = min(delay, maxRetryDelay)

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return lastErr
}

func (c *Client) once(ctx context.Context, method, url string, header http.Header, payload []byte, out any) (time.Duration, error) {
	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, &transportError{err: err}
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return 0, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		snippet := string(data)
		if len(snippet) > 300 {
			snippet = snippet[:300]
		}
		return parseRetryAfter(resp.Header.Get("Retry-After")), &StatusError{Status: resp.StatusCode, Body: snippet}
	}

	if err := json.Unmarshal(data, out); err != nil {
		return 0, &DecodeError{err: err}
	}
	return 0, nil
}

// transportError marks failures to complete the HTTP exchange (retryable).
type transportError struct{ err error }

func (e *transportError) Error() string { return "send request: " + e.err.Error() }
func (e *transportError) Unwrap() error { return e.err }

// DecodeError marks a 2xx response whose body was not the expected JSON.
type DecodeError struct{ err error }

func (e *DecodeError) Error() string { return "decode response: " + e.err.Error() }
func (e *DecodeError) Unwrap() error { return e.err }

// IsTransport reports whether err is a failure to complete the HTTP exchange.
func IsTransport(err error) bool {
	var te *transportError
	return errors.As(err, &te)
}

// IsDecode reports whether err is a response decoding failure.
func IsDecode(err error) bool {
	var de *DecodeError
	return errors.As(err, &de)
}

func retryable(err error) bool {
	var se *StatusError
	if errors.As(err, &se) {
		return se.Status == http.StatusTooManyRequests || se.Status >= 500
	}
	var te *transportError
	return errors.As(err, &te)
}

func parseRetryAfter(v string) time.Duration {
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	return 0
}
