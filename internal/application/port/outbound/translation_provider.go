package outbound

import (
	"context"

	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

// TranslationProvider translates texts between supported languages.
// Translate returns exactly one translation per input text, in order.
type TranslationProvider interface {
	Translate(
		ctx context.Context,
		texts []string,
		sourceLanguage vocab.LanguageCode,
		targetLanguage vocab.LanguageCode,
	) ([]string, error)
}
