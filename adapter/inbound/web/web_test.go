package web_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/SaeedMPro/vocabinator/adapter/inbound/web"
	"github.com/SaeedMPro/vocabinator/bootstrap"
	"github.com/SaeedMPro/vocabinator/internal/application/port/outbound"
	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

type fakeDict struct{}

func (fakeDict) Lookup(_ context.Context, term string, _ vocab.LanguageCode) ([]outbound.DictionaryEntry, error) {
	if term == "zzzz" {
		return nil, outbound.ErrNotFound
	}
	return []outbound.DictionaryEntry{{Term: term, Phonetic: "/x/", Senses: []outbound.WordSense{{
		PartOfSpeech: "adjective", Definitions: []string{"Strong."},
	}}}}, nil
}

type fakeTranslator struct{}

func (fakeTranslator) Translate(_ context.Context, texts []string, _, _ vocab.LanguageCode) ([]string, error) {
	out := make([]string, len(texts))
	for i, t := range texts {
		out[i] = "fa:" + t
	}
	return out, nil
}

const token = "0123456789abcdef0123456789abcdef0123456789abcdef"

type env struct {
	t      *testing.T
	srv    *httptest.Server
	client *http.Client
}

// newEnv serves the real use cases over a temp SQLite database. The client is
// pre-authenticated unless noAuth is set.
func newEnv(t *testing.T, noAuth bool) *env {
	t.Helper()
	app, err := bootstrap.Build(context.Background(), bootstrap.Config{
		DatabasePath:   filepath.Join(t.TempDir(), "v.db"),
		SourceLanguage: vocab.LanguageCodeEnglish,
		TargetLanguage: vocab.LanguageCodePersian,
	}, bootstrap.WithDictionary(fakeDict{}), bootstrap.WithTranslator(fakeTranslator{}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.Close() })

	handler, err := web.NewServer(web.Deps{
		Vocabulary: app.Vocabulary, SaveWord: app.SaveWord, Lookup: app.Lookup,
		DefaultSource: "en", DefaultTarget: "fa",
		Settings: web.Settings{DatabasePath: "/data/vocab.db", TranslationProvider: "MyMemory (anonymous quota)"},
		Token:    token,
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(handler.Handler())
	t.Cleanup(srv.Close)

	e := &env{t: t, srv: srv, client: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	if !noAuth {
		e.client.Jar = newJar(t)
		resp := e.do("GET", "/?token="+token, nil, nil)
		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("login failed: %d", resp.StatusCode)
		}
	}
	return e
}

func (e *env) do(method, path string, form url.Values, hdr map[string]string) *http.Response {
	e.t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequest(method, e.srv.URL+path, body)
	if err != nil {
		e.t.Fatal(err)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for k, v := range hdr {
		if k == "Host" {
			req.Host = v
		} else {
			req.Header.Set(k, v)
		}
	}
	resp, err := e.client.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	return resp
}

func (e *env) get(path string) (int, string) {
	e.t.Helper()
	resp := e.do("GET", path, nil, nil)
	return resp.StatusCode, readBody(e.t, resp)
}

// post sends a same-origin form post, like a browser would.
func (e *env) post(path string, form url.Values) *http.Response {
	e.t.Helper()
	return e.do("POST", path, form, map[string]string{"Origin": e.srv.URL, "Sec-Fetch-Site": "same-origin"})
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

var idPattern = regexp.MustCompile(`/words/([0-9a-f-]{36})`)

func (e *env) save(term, context, link string) string {
	e.t.Helper()
	resp := e.post("/words", url.Values{"term": {term}, "source": {"en"}, "target": {"fa"}, "context": {context}, "url": {link}})
	if resp.StatusCode != http.StatusSeeOther {
		e.t.Fatalf("save %q: status %d: %s", term, resp.StatusCode, readBody(e.t, resp))
	}
	m := idPattern.FindStringSubmatch(resp.Header.Get("Location"))
	if m == nil {
		e.t.Fatalf("unexpected redirect %q", resp.Header.Get("Location"))
	}
	return m[1]
}

// Security

func TestUnauthenticatedRequestsAreRejected(t *testing.T) {
	e := newEnv(t, true)
	for _, path := range []string{"/", "/words", "/settings", "/api/lookup?term=a&source=en&target=fa", "/static/app.js", "/?token=wrong"} {
		if status, _ := e.get(path); status != http.StatusUnauthorized {
			t.Errorf("%s: expected 401, got %d", path, status)
		}
	}
}

func TestTokenLoginSetsStrictCookieAndStripsToken(t *testing.T) {
	e := newEnv(t, true)

	resp := e.do("GET", "/words?q=a&token="+token, nil, nil)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/words?q=a" {
		t.Errorf("token must be removed from redirect, got %q", loc)
	}

	cookies := resp.Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode || cookies[0].Path != "/" {
		t.Fatalf("unexpected cookies %+v", cookies)
	}

	req, _ := http.NewRequest("GET", e.srv.URL+"/", nil)
	req.AddCookie(cookies[0])
	if r, err := e.client.Do(req); err != nil || r.StatusCode != http.StatusOK {
		t.Errorf("cookie should authenticate: %v %v", r, err)
	}
}

func TestTokenInPostIsNotAccepted(t *testing.T) {
	e := newEnv(t, true)
	resp := e.do("POST", "/words?token="+token, url.Values{}, map[string]string{"Origin": e.srv.URL})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestForeignHostIsRejected(t *testing.T) {
	e := newEnv(t, false)
	for _, host := range []string{"evil.example", "evil.example:8787", "127.0.0.1.evil.example"} {
		resp := e.do("GET", "/", nil, map[string]string{"Host": host, "Cookie": "vocab_session=" + token})
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("host %q: expected 403, got %d", host, resp.StatusCode)
		}
	}
	for _, host := range []string{"localhost:8787", "[::1]:8787", "127.0.0.1:1"} {
		// Cookies are per host, so send the session cookie explicitly.
		hdr := map[string]string{"Host": host, "Cookie": "vocab_session=" + token}
		if resp := e.do("GET", "/", nil, hdr); resp.StatusCode != http.StatusOK {
			t.Errorf("host %q should be allowed, got %d", host, resp.StatusCode)
		}
	}
}

func TestCrossOriginPostsAreBlocked(t *testing.T) {
	e := newEnv(t, false)
	form := url.Values{"term": {"robust"}, "source": {"en"}, "target": {"fa"}}

	cases := map[string]map[string]string{
		"foreign origin":        {"Origin": "http://evil.example"},
		"cross-site fetch meta": {"Sec-Fetch-Site": "cross-site"},
		"no provenance headers": {},
	}
	for name, hdr := range cases {
		if resp := e.do("POST", "/words", form, hdr); resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s: expected 403, got %d", name, resp.StatusCode)
		}
	}

	if status, body := e.get("/words"); status != 200 || strings.Contains(body, "robust") {
		t.Error("blocked requests must not save anything")
	}
}

func TestSecurityHeaders(t *testing.T) {
	e := newEnv(t, false)
	h := e.do("GET", "/", nil, nil).Header

	if csp := h.Get("Content-Security-Policy"); !strings.Contains(csp, "script-src 'self'") || strings.Contains(csp, "unsafe-inline") || !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("weak CSP: %q", csp)
	}
	if h.Get("X-Content-Type-Options") != "nosniff" || h.Get("Referrer-Policy") != "no-referrer" || h.Get("Cache-Control") != "no-store" {
		t.Errorf("missing headers: %v", h)
	}
}

// Pages and flows

func TestStaticAssets(t *testing.T) {
	e := newEnv(t, false)
	for path, want := range map[string]string{"/static/app.js": "javascript", "/static/app.css": "text/css"} {
		resp := e.do("GET", path, nil, nil)
		if resp.StatusCode != 200 || !strings.Contains(resp.Header.Get("Content-Type"), want) {
			t.Errorf("%s: %d %q", path, resp.StatusCode, resp.Header.Get("Content-Type"))
		}
	}
}

func TestPagesRenderAndTemplatesHaveNoInlineScript(t *testing.T) {
	e := newEnv(t, false)
	id := e.save("robust", "We need a robust architecture.", "https://example.com/a")

	for _, path := range []string{"/", "/words", "/words/" + id, "/settings"} {
		status, body := e.get(path)
		if status != 200 || !strings.Contains(body, "<title>") {
			t.Errorf("%s: status %d", path, status)
		}
		if regexp.MustCompile(`(?i)<script(\s[^>]*)?>[^<]`).MatchString(body) || regexp.MustCompile(`(?i)\son\w+=`).MatchString(body) {
			t.Errorf("%s contains inline script or handler", path)
		}
	}
}

func TestDashboardStatsAndRecent(t *testing.T) {
	e := newEnv(t, false)
	_, body := e.get("/")
	if !strings.Contains(body, "Nothing saved yet") {
		t.Error("expected empty state")
	}

	e.save("robust", "one", "")
	e.save("ephemeral", "two", "")
	_, body = e.get("/")
	for _, want := range []string{"robust", "ephemeral", "words saved", "contexts recorded", "en → fa"} {
		if !strings.Contains(body, want) {
			t.Errorf("dashboard missing %q", want)
		}
	}
}

func TestSaveShowListSearchFilterDelete(t *testing.T) {
	e := newEnv(t, false)
	id := e.save("Robust", "We need a robust architecture.", "https://example.com/a")
	e.save("ephemeral", "", "")
	// same word again with a second context goes to the same item
	if again := e.save("robust", "Go provides robust concurrency.", ""); again != id {
		t.Errorf("expected the same item, got %s vs %s", again, id)
	}

	_, body := e.get("/words/" + id)
	for _, want := range []string{"Contexts (2)", "We need a robust architecture.", "Go provides robust concurrency.", `href="https://example.com/a"`} {
		if !strings.Contains(body, want) {
			t.Errorf("word page missing %q", want)
		}
	}

	if _, body := e.get("/words?q=ROB"); !strings.Contains(body, "Robust") || strings.Contains(body, "ephemeral") {
		t.Error("search failed")
	}
	if _, body := e.get("/words?target=en"); strings.Contains(body, "Robust") || !strings.Contains(body, "No words match") {
		t.Error("language filter failed")
	}

	if resp := e.post("/words/"+id+"/delete", url.Values{}); resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/words" {
		t.Errorf("delete: %d %q", resp.StatusCode, resp.Header.Get("Location"))
	}
	if status, _ := e.get("/words/" + id); status != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", status)
	}
}

func TestAddAndDeleteOccurrence(t *testing.T) {
	e := newEnv(t, false)
	id := e.save("robust", "first", "")

	if resp := e.post("/words/"+id+"/occurrences", url.Values{"context": {"second context"}, "url": {"https://example.com"}}); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("add: %d", resp.StatusCode)
	}
	_, body := e.get("/words/" + id)
	if !strings.Contains(body, "Contexts (2)") {
		t.Fatalf("expected 2 contexts:\n%s", body)
	}

	m := regexp.MustCompile(`/words/` + id + `/occurrences/([0-9a-f-]{36})/delete`).FindStringSubmatch(body)
	if m == nil {
		t.Fatal("no occurrence delete form")
	}
	if resp := e.post("/words/"+id+"/occurrences/"+m[1]+"/delete", url.Values{}); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("delete occurrence: %d", resp.StatusCode)
	}
	if _, body := e.get("/words/" + id); !strings.Contains(body, "Contexts (1)") {
		t.Error("expected 1 context after deletion")
	}
	if resp := e.post("/words/"+id+"/occurrences/missing/delete", url.Values{}); resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestStoredContentIsEscapedAndUnsafeLinksAreNotLinked(t *testing.T) {
	e := newEnv(t, false)
	id := e.save(`<script>alert(1)</script>`, `"><img src=x onerror=alert(1)>`, "javascript:alert(1)")

	_, body := e.get("/words/" + id)
	for _, bad := range []string{"<script>alert(1)", "<img src=x", `href="javascript:`} {
		if strings.Contains(body, bad) {
			t.Errorf("unescaped content %q in page", bad)
		}
	}
	if !strings.Contains(body, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Error("expected escaped term")
	}

	_, list := e.get("/words")
	if strings.Contains(list, "<script>alert") {
		t.Error("list page leaks unescaped term")
	}
}

func TestPagination(t *testing.T) {
	e := newEnv(t, false)
	for i := range 51 {
		e.save(fmt.Sprintf("word%02d", i), "", "")
	}

	_, p1 := e.get("/words")
	if !strings.Contains(p1, `rel="next"`) || strings.Contains(p1, `rel="prev"`) {
		t.Error("page 1 should only have next")
	}
	_, p2 := e.get("/words?page=2")
	if !strings.Contains(p2, `rel="prev"`) || strings.Contains(p2, `rel="next"`) || !strings.Contains(p2, "word00") {
		t.Error("page 2 should have the oldest word and only prev")
	}
}

func TestInvalidInputAndMissingResources(t *testing.T) {
	e := newEnv(t, false)

	if status, _ := e.get("/words?source=zz"); status != http.StatusBadRequest {
		t.Errorf("bad filter language: %d", status)
	}
	if status, _ := e.get("/words/does-not-exist"); status != http.StatusNotFound {
		t.Errorf("missing word: %d", status)
	}
	if status, _ := e.get("/nope"); status != http.StatusNotFound {
		t.Errorf("unknown route: %d", status)
	}
	for name, form := range map[string]url.Values{
		"empty term":   {"term": {" "}, "source": {"en"}, "target": {"fa"}},
		"no languages": {"term": {"x"}},
		"bad language": {"term": {"x"}, "source": {"zz"}, "target": {"fa"}},
	} {
		if resp := e.post("/words", form); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d", name, resp.StatusCode)
		}
	}
}

func TestOversizedFormIsRejected(t *testing.T) {
	e := newEnv(t, false)
	resp := e.post("/words", url.Values{"term": {"x"}, "source": {"en"}, "target": {"fa"}, "context": {strings.Repeat("a", 100<<10)}})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestLookupAPI(t *testing.T) {
	e := newEnv(t, false)

	status, body := e.get("/api/lookup?term=robust&source=en&target=fa")
	if status != 200 {
		t.Fatalf("status %d: %s", status, body)
	}
	var got struct {
		Term   string
		Senses []struct {
			Definitions []struct{ Text, Translation string }
		}
	}
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatal(err)
	}
	if got.Term != "robust" || got.Senses[0].Definitions[0].Translation != "fa:Strong." {
		t.Errorf("unexpected body %s", body)
	}

	// Lookup never saves.
	if _, list := e.get("/words"); !strings.Contains(list, "No words match") {
		t.Error("lookup must not save")
	}

	for path, want := range map[string]int{
		"/api/lookup?term=zzzz&source=en&target=fa": 404,
		"/api/lookup?term=x":                        400,
		"/api/lookup?term=x&source=zz&target=fa":    400,
		"/api/lookup?term=&source=en&target=fa":     400,
	} {
		status, body := e.get(path)
		var errBody struct{ Error string }
		if status != want || json.Unmarshal([]byte(body), &errBody) != nil || errBody.Error == "" {
			t.Errorf("%s: status %d body %s", path, status, body)
		}
	}
}

func TestSettingsPage(t *testing.T) {
	e := newEnv(t, false)
	_, body := e.get("/settings")
	for _, want := range []string{"/data/vocab.db", "MyMemory (anonymous quota)", "VOCAB_TRANSLATION_PROVIDER", "English"} {
		if !strings.Contains(body, want) {
			t.Errorf("settings missing %q", want)
		}
	}
}

// Wiring

func TestNewServerValidation(t *testing.T) {
	if _, err := web.NewServer(web.Deps{Token: token}); err == nil {
		t.Error("expected error for missing services")
	}
	app, err := bootstrap.Build(context.Background(), bootstrap.Config{DatabasePath: filepath.Join(t.TempDir(), "v.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if _, err := web.NewServer(web.Deps{Vocabulary: app.Vocabulary, SaveWord: app.SaveWord, Lookup: app.Lookup, Token: "short"}); err == nil {
		t.Error("expected error for short token")
	}
}

func TestNewTokenIsRandom(t *testing.T) {
	a, _ := web.NewToken()
	b, _ := web.NewToken()
	if len(a) != 64 || a == b {
		t.Errorf("unexpected tokens %q %q", a, b)
	}
}

func TestListenLoopback(t *testing.T) {
	for _, addr := range []string{"0.0.0.0:0", ":0", "[::]:0", "192.168.1.10:0", "example.com:80", "8787", "localhost"} {
		if ln, err := web.ListenLoopback(addr); err == nil {
			ln.Close()
			t.Errorf("%q should be refused", addr)
		}
	}
	for _, addr := range []string{"127.0.0.1:0", "localhost:0"} {
		ln, err := web.ListenLoopback(addr)
		if err != nil {
			t.Errorf("%q: %v", addr, err)
			continue
		}
		if !strings.HasPrefix(ln.Addr().String(), "127.0.0.1:") {
			t.Errorf("%q bound to %s", addr, ln.Addr())
		}
		ln.Close()
	}
}

func TestServeShutsDownOnCancel(t *testing.T) {
	ln, err := web.ListenLoopback("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- web.Serve(ctx, ln, http.NotFoundHandler()) }()

	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("unexpected error %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not shut down")
	}
}
