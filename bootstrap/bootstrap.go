// Package bootstrap is the composition root: it wires adapters to use cases.
package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/SaeedMPro/vocabinator/adapter/outbound/freedictionary"
	"github.com/SaeedMPro/vocabinator/adapter/outbound/googletranslate"
	"github.com/SaeedMPro/vocabinator/adapter/outbound/mymemory"
	"github.com/SaeedMPro/vocabinator/adapter/outbound/sqlite"
	"github.com/SaeedMPro/vocabinator/adapter/outbound/system"
	"github.com/SaeedMPro/vocabinator/adapter/platform/selection"
	"github.com/SaeedMPro/vocabinator/internal/application/port/inbound"
	"github.com/SaeedMPro/vocabinator/internal/application/port/outbound"
	"github.com/SaeedMPro/vocabinator/internal/application/usecase"
	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

// App exposes the inbound ports to interface adapters (CLI, web, ...).
type App struct {
	Config     Config
	Vocabulary inbound.VocabularyService
	SaveWord   inbound.SaveWordService
	Lookup     inbound.LookupService
	Selection  inbound.SelectionService

	db *sql.DB
}

// Close releases resources held by the app.
func (a *App) Close() error {
	if a.db == nil {
		return nil
	}
	return a.db.Close()
}

type options struct {
	dictionary outbound.DictionaryProvider
	translator outbound.TranslationProvider
	selection  outbound.SelectionProvider
}

// Option overrides a default adapter, mainly for tests.
type Option func(*options)

func WithDictionary(p outbound.DictionaryProvider) Option {
	return func(o *options) { o.dictionary = p }
}
func WithTranslator(p outbound.TranslationProvider) Option {
	return func(o *options) { o.translator = p }
}

func WithSelectionProvider(p outbound.SelectionProvider) Option {
	return func(o *options) { o.selection = p }
}

// Build opens the database and wires every use case.
func Build(ctx context.Context, cfg Config, opts ...Option) (*App, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	if o.dictionary == nil {
		o.dictionary = freedictionary.New(freedictionary.Config{})
	}
	if o.selection == nil {
		o.selection = selection.New()
	}
	if o.translator == nil {
		t, err := newTranslator(cfg)
		if err != nil {
			return nil, err
		}
		o.translator = t
	}

	db, err := sqlite.Open(ctx, cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("open database %s: %w", cfg.DatabasePath, err)
	}

	vocabulary := usecase.NewVocabularyService(sqlite.NewVocabularyRepository(db), system.Clock{}, system.UUIDGenerator{})

	return &App{
		Config:     cfg,
		Vocabulary: vocabulary,
		SaveWord:   usecase.NewSaveWordService(vocabulary),
		Lookup:     usecase.NewLookupService(o.dictionary, o.translator),
		Selection:  usecase.NewSelectionService(o.selection),
		db:         db,
	}, nil
}

// newTranslator builds the configured translation provider. MyMemory is the
// default because it needs no account, key or payment card.
func newTranslator(cfg Config) (outbound.TranslationProvider, error) {
	switch cfg.TranslationProvider {
	case ProviderNone:
		return disabledTranslator{}, nil
	case ProviderGoogle:
		t, err := googletranslate.New(googletranslate.Config{APIKey: cfg.GoogleAPIKey})
		if err != nil {
			return nil, fmt.Errorf("configure %s: set %s (%w)", ProviderGoogle, EnvGoogleAPIKey, err)
		}
		return t, nil
	default:
		t, err := mymemory.New(mymemory.Config{Email: cfg.MyMemoryEmail})
		if err != nil {
			return nil, fmt.Errorf("configure %s: %w", ProviderMyMemory, err)
		}
		return t, nil
	}
}

// TranslationLabel describes the active provider for display. It never
// includes credentials or the configured email.
func (c Config) TranslationLabel() string {
	switch c.TranslationProvider {
	case ProviderNone:
		return "Disabled"
	case ProviderGoogle:
		return "Google Cloud Translation"
	}
	if c.MyMemoryEmail != "" {
		return "MyMemory (extended quota)"
	}
	return "MyMemory (anonymous quota)"
}

// ErrTranslationDisabled explains why translations are missing.
var ErrTranslationDisabled = errors.New("translation is disabled (" + EnvTranslationProvider + "=none)")

// disabledTranslator lets lookups degrade gracefully when no key is set.
type disabledTranslator struct{}

func (disabledTranslator) Translate(context.Context, []string, vocab.LanguageCode, vocab.LanguageCode) ([]string, error) {
	return nil, ErrTranslationDisabled
}
