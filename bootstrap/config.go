package bootstrap

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/SaeedMPro/vocabinator/adapter/platform/appdirs"
	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

// Environment variables read by LoadConfig.
const (
	EnvDatabasePath   = "VOCAB_DB"
	EnvSourceLanguage = "VOCAB_SOURCE_LANGUAGE"
	EnvTargetLanguage = "VOCAB_TARGET_LANGUAGE"
	EnvGoogleAPIKey   = "VOCAB_GOOGLE_TRANSLATE_API_KEY"

	// EnvTranslationProvider selects the provider: mymemory (default),
	// google or none.
	EnvTranslationProvider = "VOCAB_TRANSLATION_PROVIDER"
	// EnvMyMemoryEmail optionally raises MyMemory's daily quota. The address
	// is sent to MyMemory with each request.
	EnvMyMemoryEmail = "VOCAB_MYMEMORY_EMAIL"
)

// Translation providers.
const (
	ProviderMyMemory = "mymemory"
	ProviderGoogle   = "google"
	ProviderNone     = "none"
)

// Config holds runtime settings. Secrets are only ever read from the
// environment (a protected config file arrives with the daemon work) and are
// never written to SQLite.
type Config struct {
	DatabasePath   string
	SourceLanguage vocab.LanguageCode
	TargetLanguage vocab.LanguageCode
	GoogleAPIKey   string

	TranslationProvider string
	MyMemoryEmail       string
}

// LoadConfig builds a Config from environment lookups, applying defaults.
func LoadConfig(getenv func(string) string) (Config, error) {
	cfg := Config{
		DatabasePath:   strings.TrimSpace(getenv(EnvDatabasePath)),
		SourceLanguage: vocab.LanguageCodeEnglish,
		TargetLanguage: vocab.LanguageCodePersian,
		GoogleAPIKey:   strings.TrimSpace(getenv(EnvGoogleAPIKey)),

		TranslationProvider: strings.ToLower(strings.TrimSpace(getenv(EnvTranslationProvider))),
		MyMemoryEmail:       strings.TrimSpace(getenv(EnvMyMemoryEmail)),
	}

	switch cfg.TranslationProvider {
	case "":
		cfg.TranslationProvider = ProviderMyMemory
	case ProviderMyMemory, ProviderGoogle, ProviderNone:
	default:
		return Config{}, fmt.Errorf("%s: unknown provider %q (use %s, %s or %s)",
			EnvTranslationProvider, cfg.TranslationProvider, ProviderMyMemory, ProviderGoogle, ProviderNone)
	}

	if cfg.DatabasePath == "" {
		dir, err := appdirs.DataDir()
		if err != nil {
			return Config{}, err
		}
		cfg.DatabasePath = filepath.Join(dir, "vocabinator.db")
	}

	for env, dst := range map[string]*vocab.LanguageCode{
		EnvSourceLanguage: &cfg.SourceLanguage,
		EnvTargetLanguage: &cfg.TargetLanguage,
	} {
		if v := strings.TrimSpace(getenv(env)); v != "" {
			code, err := vocab.NewLanguageCode(v)
			if err != nil {
				return Config{}, fmt.Errorf("%s: %w: %q", env, err, v)
			}
			*dst = code
		}
	}

	return cfg, nil
}
