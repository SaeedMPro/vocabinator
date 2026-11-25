package outbound

import (
	"time"

	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

// Clock supplies the current time so use cases are deterministic in tests.
type Clock interface {
	Now() time.Time
}

// IDGenerator supplies new unique identifiers.
type IDGenerator interface {
	NewID() vocab.ID
}
