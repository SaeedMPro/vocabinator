package outbound

import (
	"context"

	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

type TranslationProvider interface {
	Translate(
		ctx context.Context,
		text string,
		sourceLanguage vocab.LanguageCode,
		targetLanguage vocab.LanguageCode,
	) (string, error)
}
