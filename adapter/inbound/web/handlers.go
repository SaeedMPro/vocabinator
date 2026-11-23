package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/SaeedMPro/vocabinator/internal/application/port/inbound"
	"github.com/SaeedMPro/vocabinator/internal/application/port/outbound"
	"github.com/SaeedMPro/vocabinator/internal/application/usecase"
	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

const (
	pageSize        = 50
	maxFormBytes    = 64 << 10
	lookupTimeout   = 20 * time.Second
	recentWordCount = 5
)

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	items, err := s.deps.Vocabulary.ListVocabItems(r.Context(), inbound.VocabItemFilter{})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, "dashboard", page{Title: "Dashboard", Nav: "dashboard", Data: s.newDashboardView(items)})
}

func (s *Server) listWords(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	source, err := parseLanguage(q.Get("source"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	target, err := parseLanguage(q.Get("target"))
	if err != nil {
		s.fail(w, r, err)
		return
	}

	pageNo, _ := strconv.Atoi(q.Get("page"))
	pageNo = max(pageNo, 1)

	items, err := s.deps.Vocabulary.ListVocabItems(r.Context(), inbound.VocabItemFilter{
		Term: q.Get("q"), SourceLanguage: source, TargetLanguage: target,
		Limit: pageSize + 1, Offset: (pageNo - 1) * pageSize,
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}

	hasNext := len(items) > pageSize
	if hasNext {
		items = items[:pageSize]
	}

	view := wordsView{
		Query: q.Get("q"), Source: q.Get("source"), Target: q.Get("target"),
		Languages: languageOptions(), Words: toSummaries(items),
		Page: pageNo,
	}
	if pageNo > 1 {
		view.PrevURL = pageURL(q, pageNo-1)
	}
	if hasNext {
		view.NextURL = pageURL(q, pageNo+1)
	}
	s.render(w, "words", page{Title: "Vocabulary", Nav: "words", Data: view})
}

func (s *Server) showWord(w http.ResponseWriter, r *http.Request) {
	item, err := s.deps.Vocabulary.GetVocabItem(r.Context(), vocab.ID(r.PathValue("id")))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, "word", page{Title: item.Term(), Nav: "words", Data: toWordView(item)})
}

func (s *Server) saveWord(w http.ResponseWriter, r *http.Request) {
	if !s.parseForm(w, r) {
		return
	}
	source, err := parseLanguage(r.PostFormValue("source"))
	if err != nil || source == nil {
		s.fail(w, r, invalid("source language is required"))
		return
	}
	target, err := parseLanguage(r.PostFormValue("target"))
	if err != nil || target == nil {
		s.fail(w, r, invalid("target language is required"))
		return
	}

	res, err := s.deps.SaveWord.SaveWord(r.Context(), inbound.SaveWordInput{
		Term: r.PostFormValue("term"), SourceLanguage: *source, TargetLanguage: *target,
		Context: r.PostFormValue("context"), SourceURL: r.PostFormValue("url"),
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	http.Redirect(w, r, "/words/"+url.PathEscape(string(res.Item.ID())), http.StatusSeeOther)
}

func (s *Server) deleteWord(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Vocabulary.DeleteVocabItem(r.Context(), vocab.ID(r.PathValue("id"))); err != nil {
		s.fail(w, r, err)
		return
	}
	http.Redirect(w, r, "/words", http.StatusSeeOther)
}

func (s *Server) addOccurrence(w http.ResponseWriter, r *http.Request) {
	if !s.parseForm(w, r) {
		return
	}
	id := vocab.ID(r.PathValue("id"))
	if _, err := s.deps.Vocabulary.AddVocabOccurrence(r.Context(), id, inbound.AddVocabOccurrenceInput{
		Context: r.PostFormValue("context"), SourceURL: r.PostFormValue("url"),
	}); err != nil {
		s.fail(w, r, err)
		return
	}
	http.Redirect(w, r, "/words/"+url.PathEscape(string(id)), http.StatusSeeOther)
}

func (s *Server) deleteOccurrence(w http.ResponseWriter, r *http.Request) {
	id := vocab.ID(r.PathValue("id"))
	if err := s.deps.Vocabulary.DeleteVocabOccurrence(r.Context(), id, vocab.ID(r.PathValue("occurrenceID"))); err != nil {
		s.fail(w, r, err)
		return
	}
	http.Redirect(w, r, "/words/"+url.PathEscape(string(id)), http.StatusSeeOther)
}

func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	s.render(w, "settings", page{Title: "Settings", Nav: "settings", Data: settingsView{
		DatabasePath:          s.deps.Settings.DatabasePath,
		TranslationConfigured: s.deps.Settings.TranslationConfigured,
		Source:                s.deps.DefaultSource,
		Target:                s.deps.DefaultTarget,
	}})
}

func (s *Server) apiLookup(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	source, err := parseLanguage(q.Get("source"))
	if err != nil || source == nil {
		s.fail(w, r, invalid("source language is required"))
		return
	}
	target, err := parseLanguage(q.Get("target"))
	if err != nil || target == nil {
		s.fail(w, r, invalid("target language is required"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), lookupTimeout)
	defer cancel()

	result, err := s.deps.Lookup.LookupWord(ctx, inbound.LookupWordInput{
		Term: q.Get("term"), SourceLanguage: *source, TargetLanguage: *target,
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toLookupJSON(result))
}

// parseForm limits and parses a POST body.
func (s *Server) parseForm(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseForm(); err != nil {
		s.fail(w, r, invalid("invalid or oversized form"))
		return false
	}
	return true
}

type inputError string

func (e inputError) Error() string { return string(e) }

func invalid(msg string) error { return inputError(msg) }

func parseLanguage(v string) (*vocab.LanguageCode, error) {
	if strings.TrimSpace(v) == "" {
		return nil, nil
	}
	code, err := vocab.NewLanguageCode(v)
	if err != nil {
		return nil, err
	}
	return &code, nil
}

func pageURL(q url.Values, n int) string {
	next := url.Values{}
	for k, v := range q {
		next[k] = v
	}
	next.Set("page", strconv.Itoa(n))
	return "/words?" + next.Encode()
}

// fail maps application errors to an HTTP status and a safe message. Internal
// details are logged, never shown.
func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	status, msg := http.StatusInternalServerError, "Something went wrong."

	var ie inputError
	switch {
	case errors.As(err, &ie):
		status, msg = http.StatusBadRequest, ie.Error()
	case errors.Is(err, usecase.ErrItemNotFound), errors.Is(err, usecase.ErrOccurrenceNotFound):
		status, msg = http.StatusNotFound, "That word or context was not found."
	case errors.Is(err, usecase.ErrWordNotFound):
		status, msg = http.StatusNotFound, "No dictionary entry was found for that word."
	case errors.Is(err, usecase.ErrInvalidInput), errors.Is(err, usecase.ErrInvalidFilter), errors.Is(err, vocab.ErrInvalidLanguageCode):
		status, msg = http.StatusBadRequest, "That input is not valid: "+validationDetail(err)
	case errors.Is(err, outbound.ErrUnsupportedLanguage):
		status, msg = http.StatusUnprocessableEntity, "The dictionary does not support that language."
	case errors.Is(err, outbound.ErrRateLimited):
		status, msg = http.StatusTooManyRequests, "The provider is rate limiting requests. Try again shortly."
	case errors.Is(err, outbound.ErrProviderUnavailable):
		status, msg = http.StatusBadGateway, "The provider is currently unavailable."
	case errors.Is(err, context.DeadlineExceeded):
		status, msg = http.StatusGatewayTimeout, "The lookup timed out."
	case errors.Is(err, context.Canceled):
		return // client went away
	}

	if status >= 500 {
		s.deps.Logger.Error("request failed", "method", r.Method, "path", r.URL.Path, "error", err)
	}

	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeJSON(w, status, map[string]string{"error": msg})
		return
	}
	w.WriteHeader(status)
	s.render(w, "error", page{Title: http.StatusText(status), Data: errorView{Status: status, Message: msg}})
}

// validationDetail extracts the innermost domain message for validation errors.
func validationDetail(err error) string {
	for _, known := range []error{
		vocab.ErrEmptyTerm, vocab.ErrInvalidLanguageCode, vocab.ErrInvalidID,
		usecase.ErrInvalidFilter,
	} {
		if errors.Is(err, known) {
			return known.Error()
		}
	}
	return "please check the values"
}

type page struct {
	Title string
	Nav   string
	Data  any
}

// render writes the page; it buffers so a template error cannot produce a
// half-written response.
func (s *Server) render(w http.ResponseWriter, name string, p page) {
	var buf bytes.Buffer
	if err := s.pages[name].ExecuteTemplate(&buf, "layout", p); err != nil {
		s.deps.Logger.Error("render template", "template", name, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
