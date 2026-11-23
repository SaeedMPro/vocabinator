CREATE TABLE vocabulary_items (
    id              TEXT PRIMARY KEY,
    term            TEXT NOT NULL,
    normalized_term TEXT NOT NULL,
    source_language TEXT NOT NULL,
    target_language TEXT NOT NULL,
    created_at      INTEGER NOT NULL, -- unix nanoseconds, UTC
    UNIQUE (normalized_term, source_language, target_language)
) STRICT;

CREATE INDEX idx_vocabulary_items_created_at ON vocabulary_items (created_at);
CREATE INDEX idx_vocabulary_items_languages ON vocabulary_items (source_language, target_language);

CREATE TABLE occurrences (
    id                 TEXT PRIMARY KEY,
    vocabulary_item_id TEXT NOT NULL REFERENCES vocabulary_items (id) ON DELETE CASCADE,
    context            TEXT NOT NULL,
    source_url         TEXT NOT NULL,
    captured_at        INTEGER NOT NULL -- unix nanoseconds, UTC
) STRICT;

CREATE INDEX idx_occurrences_item ON occurrences (vocabulary_item_id, captured_at);
