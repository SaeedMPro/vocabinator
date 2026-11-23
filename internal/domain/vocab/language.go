package vocab

import "strings"

type LanguageCode string

const (
	LanguageCodeEnglish LanguageCode = "en"
	LanguageCodePersian LanguageCode = "fa"
)

func NewLanguageCode(value string) (LanguageCode, error) {
	normalized := LanguageCode(
		strings.ToLower(strings.TrimSpace(value)),
	)

	switch normalized {
	case LanguageCodeEnglish, LanguageCodePersian:
		return normalized, nil
	default:
		return "", ErrInvalidLanguageCode
	}
}

func (l LanguageCode) String() string {
	return string(l)
}

// SupportedLanguages lists every language code the domain accepts.
func SupportedLanguages() []LanguageCode {
	return []LanguageCode{LanguageCodeEnglish, LanguageCodePersian}
}

// DisplayName returns a human-readable English name for the language.
func (l LanguageCode) DisplayName() string {
	switch l {
	case LanguageCodeEnglish:
		return "English"
	case LanguageCodePersian:
		return "Persian"
	default:
		return string(l)
	}
}
