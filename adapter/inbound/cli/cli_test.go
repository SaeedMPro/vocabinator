package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SaeedMPro/vocabinator/adapter/inbound/cli"
	"github.com/SaeedMPro/vocabinator/bootstrap"
	"github.com/SaeedMPro/vocabinator/internal/application/port/outbound"
	"github.com/SaeedMPro/vocabinator/internal/application/usecase"
	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

type fakeDict struct{}

func (fakeDict) Lookup(_ context.Context, term string, _ vocab.LanguageCode) ([]outbound.DictionaryEntry, error) {
	if strings.EqualFold(term, "zzzz") {
		return nil, outbound.ErrNotFound
	}
	return []outbound.DictionaryEntry{{Term: term, Phonetic: "/x/", Senses: []outbound.WordSense{{
		PartOfSpeech: "adjective", Definitions: []string{"Strong."}, Examples: []string{"A robust design."}, Synonyms: []string{"sturdy"},
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

type fakeSelection struct {
	text    string
	err     error
	sources []outbound.SelectionSource
}

func (f *fakeSelection) Capture(_ context.Context, src outbound.SelectionSource) (string, error) {
	f.sources = append(f.sources, src)
	return f.text, f.err
}

type notification struct{ title, body string }

// harness builds a CLI over a real SQLite database in a temp dir.
type harness struct {
	t         *testing.T
	cfg       bootstrap.Config
	selection *fakeSelection
	notes     []notification
	notifyErr error
	stderr    string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	return &harness{t: t, selection: &fakeSelection{}, cfg: bootstrap.Config{
		DatabasePath:   filepath.Join(t.TempDir(), "vocab.db"),
		SourceLanguage: vocab.LanguageCodeEnglish,
		TargetLanguage: vocab.LanguageCodePersian,
	}}
}

func (h *harness) run(args ...string) (stdout string, err error) {
	h.t.Helper()
	var out, errOut bytes.Buffer

	root := cli.NewRootCmd(cli.Env{
		Stdout:  &out,
		Stderr:  &errOut,
		Version: "v1.2.3",
		Notify: func(_ context.Context, title, body string) error {
			h.notes = append(h.notes, notification{title, body})
			return h.notifyErr
		},
		Open: func(ctx context.Context) (cli.Services, io.Closer, error) {
			app, err := bootstrap.Build(ctx, h.cfg, bootstrap.WithDictionary(fakeDict{}), bootstrap.WithTranslator(fakeTranslator{}), bootstrap.WithSelectionProvider(h.selection))
			if err != nil {
				return cli.Services{}, nil, err
			}
			return cli.Services{
				Vocabulary: app.Vocabulary, SaveWord: app.SaveWord, Lookup: app.Lookup, Selection: app.Selection,
				DefaultSource: h.cfg.SourceLanguage, DefaultTarget: h.cfg.TargetLanguage,
			}, app, nil
		},
	})
	root.SetArgs(args)

	err = root.ExecuteContext(context.Background())
	h.stderr = errOut.String()
	return out.String(), err
}

func (h *harness) mustRun(args ...string) string {
	h.t.Helper()
	out, err := h.run(args...)
	if err != nil {
		h.t.Fatalf("vocab %v: %v", args, err)
	}
	return out
}

func TestLookupText(t *testing.T) {
	out := newHarness(t).mustRun("lookup", "robust")

	for _, want := range []string{"robust  /x/", "adjective", "1. Strong.", "→ fa:Strong.", "Example: A robust design.", "Synonyms: sturdy"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestLookupDoesNotSave(t *testing.T) {
	h := newHarness(t)
	h.mustRun("lookup", "robust")

	if out := h.mustRun("list"); !strings.Contains(out, "No vocabulary saved yet.") {
		t.Errorf("lookup must not save: %s", out)
	}
}

func TestLookupJSON(t *testing.T) {
	out := newHarness(t).mustRun("lookup", "robust", "--format", "json", "--source", "EN", "--target", "fa")

	var got struct {
		Term   string
		Senses []struct {
			Definitions []struct{ Text, Translation string }
		}
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if got.Term != "robust" || got.Senses[0].Definitions[0].Translation != "fa:Strong." {
		t.Errorf("unexpected JSON: %+v", got)
	}
}

func TestMultiWordTermJoinsArguments(t *testing.T) {
	out := newHarness(t).mustRun("lookup", "ice", "cream")
	if !strings.HasPrefix(out, "ice cream") {
		t.Errorf("unexpected output: %s", out)
	}
}

func TestSaveListShowRemoveFlow(t *testing.T) {
	h := newHarness(t)

	out := h.mustRun("save", "Robust", "--context", "We need a robust architecture.", "--url", "https://example.com/a")
	if !strings.Contains(out, `Saved "Robust"`) || !strings.Contains(out, "Context recorded") {
		t.Errorf("unexpected save output: %s", out)
	}

	if out := h.mustRun("save", "robust", "--context", "Go provides robust concurrency."); !strings.Contains(out, "already saved; added a new context") {
		t.Errorf("unexpected second save output: %s", out)
	}
	if out := h.mustRun("save", "robust", "--context", "Go provides robust concurrency."); !strings.Contains(out, "already saved.") {
		t.Errorf("repeat save must be idempotent: %s", out)
	}

	list := h.mustRun("list")
	if !strings.Contains(list, "Robust") || !strings.Contains(list, "en → fa") {
		t.Errorf("unexpected list: %s", list)
	}

	show := h.mustRun("show", "ROBUST")
	for _, want := range []string{"Contexts (2)", "We need a robust architecture.", "https://example.com/a", "Go provides robust concurrency."} {
		if !strings.Contains(show, want) {
			t.Errorf("show missing %q:\n%s", want, show)
		}
	}

	if out := h.mustRun("remove", "robust"); !strings.Contains(out, `Removed "Robust"`) {
		t.Errorf("unexpected remove output: %s", out)
	}
	if _, err := h.run("show", "robust"); !errors.Is(err, usecase.ErrItemNotFound) {
		t.Errorf("expected ErrItemNotFound after removal, got %v", err)
	}
}

func TestLanguagePairsAreSeparate(t *testing.T) {
	h := newHarness(t)
	h.mustRun("save", "robust")
	h.mustRun("save", "robust", "--target", "en")

	var items []struct{ TargetLanguage string }
	if err := json.Unmarshal([]byte(h.mustRun("list", "--format", "json")), &items); err != nil || len(items) != 2 {
		t.Fatalf("expected 2 items, got %d (err %v)", len(items), err)
	}

	var filtered []struct{ TargetLanguage string }
	if err := json.Unmarshal([]byte(h.mustRun("list", "--target", "en", "--format", "json")), &filtered); err != nil || len(filtered) != 1 || filtered[0].TargetLanguage != "en" {
		t.Errorf("filter failed: %+v (err %v)", filtered, err)
	}
}

func TestListSearchAndLimit(t *testing.T) {
	h := newHarness(t)
	for _, w := range []string{"robust", "robustness", "ephemeral"} {
		h.mustRun("save", w)
	}

	if out := h.mustRun("list", "--search", "robust"); strings.Contains(out, "ephemeral") || !strings.Contains(out, "robustness") {
		t.Errorf("unexpected search output: %s", out)
	}

	var items []map[string]any
	if err := json.Unmarshal([]byte(h.mustRun("list", "--limit", "2", "--format", "json")), &items); err != nil || len(items) != 2 {
		t.Errorf("expected 2 items, got %d (err %v)", len(items), err)
	}
}

func TestErrors(t *testing.T) {
	h := newHarness(t)

	cases := map[string][]string{
		"unknown word":    {"lookup", "zzzz"},
		"bad language":    {"lookup", "x", "--source", "xx"},
		"bad format":      {"lookup", "x", "--format", "yaml"},
		"missing word":    {"lookup"},
		"remove unsaved":  {"remove", "nothing"},
		"bad list filter": {"list", "--target", "zz"},
	}
	for name, args := range cases {
		if _, err := h.run(args...); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}

	if _, err := h.run("lookup", "zzzz"); !errors.Is(err, usecase.ErrWordNotFound) {
		t.Errorf("expected ErrWordNotFound, got %v", err)
	}
}

func TestOpenFailureIsReported(t *testing.T) {
	boom := errors.New("cannot open")
	root := cli.NewRootCmd(cli.Env{
		Stdout: io.Discard, Stderr: io.Discard,
		Open: func(context.Context) (cli.Services, io.Closer, error) { return cli.Services{}, nil, boom },
	})
	root.SetArgs([]string{"list"})

	if err := root.Execute(); !errors.Is(err, boom) {
		t.Errorf("expected open error, got %v", err)
	}
}

func TestInvalidFlagsNeverOpenServices(t *testing.T) {
	opened := false
	root := cli.NewRootCmd(cli.Env{
		Stdout: io.Discard, Stderr: io.Discard,
		Open: func(context.Context) (cli.Services, io.Closer, error) {
			opened = true
			return cli.Services{}, nil, errors.New("x")
		},
	})
	root.SetArgs([]string{"list", "--format", "xml"})

	if err := root.Execute(); err == nil || opened {
		t.Errorf("err=%v opened=%v", err, opened)
	}
}

func TestServeCommand(t *testing.T) {
	// Not registered without a Serve hook.
	root := cli.NewRootCmd(cli.Env{Stdout: io.Discard, Stderr: io.Discard})
	root.SetArgs([]string{"serve"})
	if err := root.Execute(); err == nil {
		t.Error("serve must not exist without a Serve hook")
	}

	var gotAddr string
	root = cli.NewRootCmd(cli.Env{
		Stdout: io.Discard, Stderr: io.Discard,
		Open: func(context.Context) (cli.Services, io.Closer, error) {
			t.Error("serve must not open services itself")
			return cli.Services{}, nil, errors.New("x")
		},
		Serve: func(_ context.Context, addr string, _ io.Writer) error { gotAddr = addr; return nil },
	})

	root.SetArgs([]string{"serve"})
	if err := root.Execute(); err != nil || gotAddr != cli.DefaultServeAddr {
		t.Errorf("default addr: %q err=%v", gotAddr, err)
	}
	root.SetArgs([]string{"serve", "--addr", "localhost:9999"})
	if err := root.Execute(); err != nil || gotAddr != "localhost:9999" {
		t.Errorf("custom addr: %q err=%v", gotAddr, err)
	}
}

func TestVersionFlag(t *testing.T) {
	if out := newHarness(t).mustRun("--version"); !strings.Contains(out, "v1.2.3") {
		t.Errorf("unexpected version output %q", out)
	}
}

func TestLookupSelection(t *testing.T) {
	h := newHarness(t)
	h.selection.text = "  “Robust,” \n"

	out := h.mustRun("lookup", "--selection")
	if !strings.HasPrefix(out, "Robust") || !strings.Contains(out, "→ fa:Strong.") {
		t.Errorf("unexpected output: %s", out)
	}
	if len(h.selection.sources) != 1 || h.selection.sources[0] != outbound.SelectionPrimary {
		t.Errorf("--selection must read the primary selection, got %v", h.selection.sources)
	}
	if len(h.notes) != 0 {
		t.Error("no notification without --notify")
	}
	if list := h.mustRun("list"); !strings.Contains(list, "No vocabulary saved yet.") {
		t.Error("lookup must not save")
	}
}

func TestLookupClipboardUsesClipboardSource(t *testing.T) {
	h := newHarness(t)
	h.selection.text = "robust"
	h.mustRun("lookup", "--clipboard")

	if len(h.selection.sources) != 1 || h.selection.sources[0] != outbound.SelectionClipboard {
		t.Errorf("got sources %v", h.selection.sources)
	}
}

func TestSaveSelection(t *testing.T) {
	h := newHarness(t)
	h.selection.text = "robust."

	if out := h.mustRun("save", "--selection", "--context", "A robust design."); !strings.Contains(out, `Saved "robust"`) {
		t.Errorf("unexpected output: %s", out)
	}
	if show := h.mustRun("show", "robust"); !strings.Contains(show, "A robust design.") {
		t.Errorf("context not stored: %s", show)
	}
}

func TestLookupNotify(t *testing.T) {
	h := newHarness(t)
	h.selection.text = "robust"
	h.mustRun("lookup", "--selection", "--notify")

	if len(h.notes) != 1 {
		t.Fatalf("expected 1 notification, got %v", h.notes)
	}
	n := h.notes[0]
	if !strings.HasPrefix(n.title, "robust") || !strings.Contains(n.body, "adjective: Strong.") || !strings.Contains(n.body, "→ fa:Strong.") {
		t.Errorf("unexpected notification %+v", n)
	}
}

func TestSaveNotify(t *testing.T) {
	h := newHarness(t)
	h.selection.text = "robust"
	h.mustRun("save", "--selection", "--notify")
	h.mustRun("save", "--selection", "--notify")

	if len(h.notes) != 2 || !strings.Contains(h.notes[0].body, "Saved") || !strings.Contains(h.notes[1].body, "already saved") {
		t.Errorf("unexpected notifications %v", h.notes)
	}
}

func TestErrorsAreNotifiedWhenRequested(t *testing.T) {
	h := newHarness(t)
	h.selection.text = "   "

	_, err := h.run("lookup", "--selection", "--notify")
	if !errors.Is(err, usecase.ErrNoSelection) {
		t.Fatalf("expected ErrNoSelection, got %v", err)
	}
	if len(h.notes) != 1 || !strings.Contains(h.notes[0].body, "no text is selected") {
		t.Errorf("error must be notified: %v", h.notes)
	}

	// Without --notify nothing is shown.
	h.notes = nil
	h.run("lookup", "--selection")
	if len(h.notes) != 0 {
		t.Error("unexpected notification")
	}
}

func TestSelectionErrors(t *testing.T) {
	h := newHarness(t)

	h.selection.err = outbound.ErrSelectionUnavailable
	if _, err := h.run("lookup", "--selection"); !errors.Is(err, outbound.ErrSelectionUnavailable) {
		t.Errorf("provider error must propagate, got %v", err)
	}

	h.selection.err = nil
	h.selection.text = strings.Repeat("word ", 40)
	if _, err := h.run("lookup", "--selection"); !errors.Is(err, usecase.ErrInvalidInput) || !errors.Is(err, vocab.ErrSelectionTooLong) {
		t.Errorf("long selection: got %v", err)
	}
}

func TestNotificationFailureDoesNotFailCommand(t *testing.T) {
	h := newHarness(t)
	h.selection.text = "robust"
	h.notifyErr = errors.New("no notification daemon")

	out, err := h.run("lookup", "--selection", "--notify")
	if err != nil || !strings.Contains(out, "robust") {
		t.Fatalf("lookup must succeed: %v", err)
	}
	if !strings.Contains(h.stderr, "could not show notification") {
		t.Errorf("expected a stderr warning, got %q", h.stderr)
	}
}

func TestInputFlagValidation(t *testing.T) {
	h := newHarness(t)
	cases := map[string][]string{
		"word and selection":      {"lookup", "robust", "--selection"},
		"selection and clipboard": {"lookup", "--selection", "--clipboard"},
		"nothing at all":          {"lookup"},
		"save word and clipboard": {"save", "robust", "--clipboard"},
		"save nothing":            {"save"},
	}
	for name, args := range cases {
		if _, err := h.run(args...); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if len(h.selection.sources) != 0 {
		t.Error("invalid flag combinations must not touch the selection")
	}
}

func TestNotifyWithoutNotifierWarns(t *testing.T) {
	var errOut bytes.Buffer
	root := cli.NewRootCmd(cli.Env{
		Stdout: io.Discard, Stderr: &errOut,
		Open: func(ctx context.Context) (cli.Services, io.Closer, error) {
			cfg := bootstrap.Config{DatabasePath: filepath.Join(t.TempDir(), "v.db"), SourceLanguage: "en", TargetLanguage: "fa"}
			app, err := bootstrap.Build(ctx, cfg, bootstrap.WithDictionary(fakeDict{}), bootstrap.WithTranslator(fakeTranslator{}))
			if err != nil {
				return cli.Services{}, nil, err
			}
			return cli.Services{Vocabulary: app.Vocabulary, SaveWord: app.SaveWord, Lookup: app.Lookup, Selection: app.Selection, DefaultSource: "en", DefaultTarget: "fa"}, app, nil
		},
	})
	root.SetArgs([]string{"lookup", "robust", "--notify"})
	if err := root.Execute(); err != nil || !strings.Contains(errOut.String(), "notifications are not available") {
		t.Errorf("err=%v stderr=%q", err, errOut.String())
	}
}
