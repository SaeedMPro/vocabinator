package usecase

import (
	"errors"
	"fmt"
)

var (
	ErrItemNotFound  = errors.New("item not found")
	ErrItemExists    = errors.New("item already exists")
	ErrInvalidFilter = errors.New("invalid filter")
)

type UseCaseError struct {
	Code    string
	Message string
}

func (e *UseCaseError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
