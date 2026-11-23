package usecase

import "errors"

// Sentinel errors returned (wrapped) by use cases. Domain validation errors are
// wrapped together with ErrInvalidInput so callers can use errors.Is on either.
var (
	ErrItemNotFound       = errors.New("item not found")
	ErrOccurrenceNotFound = errors.New("occurrence not found")
	ErrItemExists         = errors.New("item already exists")
	ErrInvalidInput       = errors.New("invalid input")
	ErrInvalidFilter      = errors.New("invalid filter")
	ErrWordNotFound       = errors.New("word not found")
)
