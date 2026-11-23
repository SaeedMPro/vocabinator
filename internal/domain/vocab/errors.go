package vocab

import "errors"

var (
	ErrInvalidID              = errors.New("invalid ID")
	ErrEmptyTerm              = errors.New("term cannot be empty")
	ErrInvalidLanguageCode    = errors.New("invalid language code")
	ErrInvalidTimestamp       = errors.New("invalid timestamp")
	ErrOccurrenceItemMismatch = errors.New("occurrence belongs to another vocabulary item")
	ErrDuplicateOccurrence    = errors.New("occurrence already exists")
	ErrOccurrenceNotFound     = errors.New("occurrence not found")
)
