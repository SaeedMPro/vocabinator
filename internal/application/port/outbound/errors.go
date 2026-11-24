package outbound

import "errors"

// Provider-independent failures that adapters map their transport errors to.
var (
	ErrNotFound            = errors.New("not found")
	ErrUnsupportedLanguage = errors.New("unsupported language")
	ErrRateLimited         = errors.New("rate limited")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrProviderUnavailable = errors.New("provider unavailable")

	// ErrNetwork means the provider could not be reached (DNS, TLS, connection,
	// timeout), as opposed to the provider answering with a failure.
	ErrNetwork = errors.New("network error")
	// ErrProviderError means the provider answered but rejected the request.
	ErrProviderError = errors.New("provider error")
	// ErrInvalidResponse means the provider's answer was malformed or unusable.
	ErrInvalidResponse = errors.New("invalid provider response")
)
