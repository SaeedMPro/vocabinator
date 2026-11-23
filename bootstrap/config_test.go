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
