package web_test

import (
	"net/http"
	"net/http/cookiejar"
	"testing"
)

func newJar(t *testing.T) http.CookieJar {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return jar
}
