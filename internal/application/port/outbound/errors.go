package outbound

import "errors"

// Provider-independent failures that adapters map their transport errors to.
var (
	ErrNotFound            = errors.New("not found")
	ErrUnsupportedLanguage = errors.New("unsupported language")
	ErrRateLimited         = errors.New("rate limited")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrProviderUnavailable = errors.New("provider unavailable")
)
