package bootstrap

import (
	"strings"
	"testing"

	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := LoadConfig(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SourceLanguage != vocab.LanguageCodeEnglish || cfg.TargetLanguage != vocab.LanguageCodePersian || cfg.GoogleAPIKey != "" {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	if !strings.HasSuffix(cfg.DatabasePath, "vocabinator.db") {
		t.Errorf("unexpected database path %q", cfg.DatabasePath)
	}
}

func TestLoadConfigOverrides(t *testing.T) {
	cfg, err := LoadConfig(env(map[string]string{
		EnvDatabasePath: " /tmp/x.db ", EnvSourceLanguage: "FA", EnvTargetLanguage: "en", EnvGoogleAPIKey: " k ",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabasePath != "/tmp/x.db" || cfg.SourceLanguage != "fa" || cfg.TargetLanguage != "en" || cfg.GoogleAPIKey != "k" {
		t.Errorf("unexpected config: %+v", cfg)
	}
}

func TestLoadConfigRejectsBadLanguage(t *testing.T) {
	_, err := LoadConfig(env(map[string]string{EnvSourceLanguage: "zz"}))
	if err == nil || !strings.Contains(err.Error(), EnvSourceLanguage) {
		t.Errorf("expected error naming %s, got %v", EnvSourceLanguage, err)
	}
}

func TestTranslationProviderDefaultsToMyMemory(t *testing.T) {
	cfg, err := LoadConfig(env(nil))
	if err != nil || cfg.TranslationProvider != ProviderMyMemory || cfg.MyMemoryEmail != "" {
		t.Errorf("cfg=%+v err=%v", cfg, err)
	}
	if got := cfg.TranslationLabel(); got != "MyMemory (anonymous quota)" {
		t.Errorf("label %q", got)
	}
}

func TestTranslationProviderSelection(t *testing.T) {
	for in, want := range map[string]string{" MyMemory ": ProviderMyMemory, "GOOGLE": ProviderGoogle, "none": ProviderNone} {
		cfg, err := LoadConfig(env(map[string]string{EnvTranslationProvider: in}))
		if err != nil || cfg.TranslationProvider != want {
			t.Errorf("%q: cfg=%+v err=%v", in, cfg, err)
		}
	}
	if _, err := LoadConfig(env(map[string]string{EnvTranslationProvider: "deepl"})); err == nil || !strings.Contains(err.Error(), EnvTranslationProvider) {
		t.Errorf("expected an error naming %s, got %v", EnvTranslationProvider, err)
	}
}

func TestTranslationLabelNeverLeaksSecrets(t *testing.T) {
	cfg, _ := LoadConfig(env(map[string]string{
		EnvTranslationProvider: "mymemory", EnvMyMemoryEmail: "private@example.com", EnvGoogleAPIKey: "SECRET-KEY",
	}))
	label := cfg.TranslationLabel()
	if strings.Contains(label, "private@example.com") || strings.Contains(label, "SECRET-KEY") || label != "MyMemory (extended quota)" {
		t.Errorf("unexpected label %q", label)
	}
	cfg.TranslationProvider = ProviderGoogle
	if label := cfg.TranslationLabel(); strings.Contains(label, "SECRET-KEY") {
		t.Errorf("label leaks key: %q", label)
	}
}
