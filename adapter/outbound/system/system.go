// Package system provides the real-world Clock and IDGenerator adapters.
package system

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/SaeedMPro/vocabinator/internal/application/port/outbound"
	"github.com/SaeedMPro/vocabinator/internal/domain/vocab"
)

type Clock struct{}

func (Clock) Now() time.Time { return time.Now().UTC() }

// UUIDGenerator produces random (version 4) UUIDs.
type UUIDGenerator struct{}

func (UUIDGenerator) NewID() vocab.ID {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("system: crypto/rand failed: %v", err)) // documented to never fail on supported platforms
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return vocab.ID(fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]))
}

var (
	_ outbound.Clock       = Clock{}
	_ outbound.IDGenerator = UUIDGenerator{}
)
