//go:build integration

package mymemory

import (
	"context"
	"net"
	"net/url"
	"testing"
	"time"
)

// Manual check against the real service. It uses a few characters of the
// anonymous daily quota and sends no email.
//
//	go test -tags integration -run Integration -v ./adapter/outbound/mymemory
//
// Run it on the network you care about: success elsewhere says nothing about
// availability from your own connection. The log shows where a failure occurs
// (DNS, TLS/connection, provider response).
func TestIntegrationRealProvider(t *testing.T) {
	u, _ := url.Parse(DefaultBaseURL)

	start := time.Now()
	addrs, err := net.DefaultResolver.LookupHost(context.Background(), u.Hostname())
	if err != nil {
		t.Fatalf("DNS lookup of %s failed: %v", u.Hostname(), err)
	}
	t.Logf("DNS ok: %s -> %v (%v)", u.Hostname(), addrs, time.Since(start))

	p, err := New(Config{})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start = time.Now()
	out, err := p.Translate(ctx, []string{"We need a robust architecture."}, "en", "fa")
	if err != nil {
		t.Fatalf("translation failed after %v: %v", time.Since(start), err)
	}
	t.Logf("translation ok in %v: %q", time.Since(start), out[0])

	if out[0] == "" {
		t.Error("empty translation")
	}
}
