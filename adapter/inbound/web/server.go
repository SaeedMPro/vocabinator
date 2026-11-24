// Package web is the local browser interface. It renders HTML and JSON from
// the same inbound ports the CLI uses and holds no business logic.
//
// Security model: the server must be bound to a loopback address (see
// ListenLoopback). Every request needs a per-run secret token (delivered once
// through the URL printed by `vocab serve`, then kept in an HttpOnly
// SameSite=Strict cookie). Host headers are restricted to loopback names to
// defeat DNS rebinding, state-changing requests must be same-origin, and a
// strict Content-Security-Policy forbids inline script. Third-party API keys
// never reach this package.
package web

import (
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/SaeedMPro/vocabinator/internal/application/port/inbound"
	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

const cookieName = "vocab_session"

// Settings is read-only information shown on the settings page. It must never
// contain secrets.
type Settings struct {
	DatabasePath        string
	TranslationProvider string
}

type Deps struct {
	Vocabulary inbound.VocabularyService
	SaveWord   inbound.SaveWordService
	Lookup     inbound.LookupService

	DefaultSource vocab.LanguageCode
	DefaultTarget vocab.LanguageCode
	Settings      Settings

	// Token is the per-run secret; use NewToken.
	Token  string
	Logger *slog.Logger
}

type Server struct {
	deps  Deps
	pages map[string]*template.Template
	mux   *http.ServeMux
}

// NewToken returns a random 256-bit token.
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func NewServer(deps Deps) (*Server, error) {
	if deps.Vocabulary == nil || deps.SaveWord == nil || deps.Lookup == nil {
		return nil, errors.New("web: all services are required")
	}
	if len(deps.Token) < 32 {
		return nil, errors.New("web: token must be at least 32 characters")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}

	pages, err := loadTemplates()
	if err != nil {
		return nil, err
	}

	s := &Server{deps: deps, pages: pages, mux: http.NewServeMux()}
	s.routes()
	return s, nil
}

func loadTemplates() (map[string]*template.Template, error) {
	funcs := template.FuncMap{
		"formatTime": func(t time.Time) string { return t.Local().Format("02/01/2006 15:04") },
		"join":       strings.Join,
	}

	pages := map[string]*template.Template{}
	for _, name := range []string{"dashboard", "words", "word", "settings", "error"} {
		t, err := template.New("").Funcs(funcs).ParseFS(templateFS, "templates/layout.html", "templates/"+name+".html")
		if err != nil {
			return nil, fmt.Errorf("parse template %s: %w", name, err)
		}
		pages[name] = t
	}
	return pages, nil
}

func (s *Server) routes() {
	static, _ := fs.Sub(staticFS, "static")
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))

	s.mux.HandleFunc("GET /{$}", s.dashboard)
	s.mux.HandleFunc("GET /words", s.listWords)
	s.mux.HandleFunc("POST /words", s.saveWord)
	s.mux.HandleFunc("GET /words/{id}", s.showWord)
	s.mux.HandleFunc("POST /words/{id}/delete", s.deleteWord)
	s.mux.HandleFunc("POST /words/{id}/occurrences", s.addOccurrence)
	s.mux.HandleFunc("POST /words/{id}/occurrences/{occurrenceID}/delete", s.deleteOccurrence)
	s.mux.HandleFunc("GET /settings", s.settings)
	s.mux.HandleFunc("GET /api/lookup", s.apiLookup)
}

// Handler returns the fully protected handler.
func (s *Server) Handler() http.Handler {
	return s.securityHeaders(s.hostCheck(s.authenticate(s.sameOrigin(s.mux))))
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; connect-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// hostCheck rejects requests whose Host is not a loopback name, which blocks
// DNS-rebinding attacks from web pages on other origins.
func (s *Server) hostCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackHostname(hostname(r.Host)) {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// authenticate requires the session cookie. A request carrying the correct
// ?token= sets the cookie and is redirected to the same URL without it.
func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(cookieName); err == nil && s.validToken(c.Value) {
			next.ServeHTTP(w, r)
			return
		}

		if r.Method == http.MethodGet && s.validToken(r.URL.Query().Get("token")) {
			http.SetCookie(w, &http.Cookie{
				Name: cookieName, Value: s.deps.Token, Path: "/",
				HttpOnly: true, SameSite: http.SameSiteStrictMode,
			})
			q := r.URL.Query()
			q.Del("token")
			target := url.URL{Path: r.URL.Path, RawQuery: q.Encode()}
			http.Redirect(w, r, target.String(), http.StatusSeeOther)
			return
		}

		http.Error(w, "unauthorised: open the URL printed by `vocab serve`", http.StatusUnauthorized)
	})
}

func (s *Server) validToken(candidate string) bool {
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(s.deps.Token)) == 1
}

// sameOrigin blocks cross-site state-changing requests (CSRF) in addition to
// the SameSite=Strict cookie.
func (s *Server) sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Host != r.Host {
				http.Error(w, "cross-origin request blocked", http.StatusForbidden)
				return
			}
		} else if site := r.Header.Get("Sec-Fetch-Site"); site != "same-origin" && site != "none" {
			http.Error(w, "cross-origin request blocked", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func hostname(hostport string) string {
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		return h
	}
	return strings.Trim(hostport, "[]")
}

func isLoopbackHostname(h string) bool {
	if strings.EqualFold(h, "localhost") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}
