package bootstrap

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SaeedMPro/vocabinator/adapter/outbound/googletranslate"
	"github.com/SaeedMPro/vocabinator/adapter/outbound/mymemory"
	"github.com/SaeedMPro/vocabinator/internal/application/port/outbound"
)

func TestNewTranslatorSelectsAdapter(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want func(outbound.TranslationProvider) bool
	}{
		{"default is mymemory", Config{TranslationProvider: ProviderMyMemory}, func(p outbound.TranslationProvider) bool { _, ok := p.(*mymemory.Provider); return ok }},
		{"empty falls back to mymemory", Config{}, func(p outbound.TranslationProvider) bool { _, ok := p.(*mymemory.Provider); return ok }},
		{"google when selected", Config{TranslationProvider: ProviderGoogle, GoogleAPIKey: "k"}, func(p outbound.TranslationProvider) bool { _, ok := p.(*googletranslate.Provider); return ok }},
		{"none disables", Config{TranslationProvider: ProviderNone}, func(p outbound.TranslationProvider) bool { _, ok := p.(disabledTranslator); return ok }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := newTranslator(tc.cfg)
			if err != nil || !tc.want(p) {
				t.Errorf("got %T, %v", p, err)
			}
		})
	}
}

func TestGoogleRequiresAKey(t *testing.T) {
	_, err := newTranslator(Config{TranslationProvider: ProviderGoogle})
	if err == nil || !strings.Contains(err.Error(), EnvGoogleAPIKey) {
		t.Errorf("expected an error naming %s, got %v", EnvGoogleAPIKey, err)
	}
}

func TestInvalidMyMemoryEmailIsRejected(t *testing.T) {
	_, err := newTranslator(Config{TranslationProvider: ProviderMyMemory, MyMemoryEmail: "nope"})
	if err == nil || strings.Contains(err.Error(), "nope") {
		t.Errorf("expected an error that does not echo the value, got %v", err)
	}
}

func TestDisabledTranslatorExplainsItself(t *testing.T) {
	_, err := disabledTranslator{}.Translate(context.Background(), []string{"x"}, "en", "fa")
	if !errors.Is(err, ErrTranslationDisabled) {
		t.Errorf("got %v", err)
	}
}

func TestBuildWithDefaultsDoesNotNeedCredentials(t *testing.T) {
	app, err := Build(context.Background(), Config{
		DatabasePath: filepath.Join(t.TempDir(), "v.db"), SourceLanguage: "en", TargetLanguage: "fa",
		TranslationProvider: ProviderMyMemory,
	})
	if err != nil {
		t.Fatalf("default build must work with no account, key or card: %v", err)
	}
	app.Close()
}
