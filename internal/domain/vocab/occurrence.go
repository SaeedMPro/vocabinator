package vocab

import (
    "strings"
    "time"
)

type ID string

type Occurrence struct {
    id               ID
    vocabularyItemID ID
    context          string
    sourceURL        string
    capturedAt       time.Time
}

func NewOccurrence(
    id ID,
    vocabularyItemID ID,
    context string,
    sourceURL string,
    capturedAt time.Time,
) (Occurrence, error) {

    id = ID(strings.TrimSpace(string(id)))
    vocabularyItemID = ID(strings.TrimSpace(string(vocabularyItemID)))

    if id == "" || vocabularyItemID == "" {
        return Occurrence{}, ErrInvalidID
    }

    if capturedAt.IsZero() {
        return Occurrence{}, ErrInvalidTimestamp
    }

    return Occurrence{
        id:               id,
        vocabularyItemID: vocabularyItemID,
        context:          strings.TrimSpace(context),
        sourceURL:        strings.TrimSpace(sourceURL),
        capturedAt:       capturedAt.UTC(),
    }, nil
}

func (o Occurrence) ID() ID {
    return o.id
}

func (o Occurrence) VocabularyItemID() ID {
    return o.vocabularyItemID
}

func (o Occurrence) Context() string {
    return o.context
}

func (o Occurrence) SourceURL() string {
    return o.sourceURL
}

func (o Occurrence) CapturedAt() time.Time {
    return o.capturedAt
}