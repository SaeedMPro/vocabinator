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
