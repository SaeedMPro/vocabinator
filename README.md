# Vocabinator

Turn unknown words into knowledge.

A local-first vocabulary assistant: look a word up, save it with the context you
met it in, and study it later. Built in Go with a hexagonal architecture (see
`docs/decisions/`).

## Status

Milestones 1 to 5 are complete: domain, SQLite persistence, dictionary and
translation lookup, the `vocab` CLI and the local web UI. Selection shortcuts,
the daemon and daily review are not built yet.

## Build and test

```bash
go build -o vocab ./cmd/vocab
go test ./...
```

## Usage

```bash
vocab lookup robust                      # look up, never saves
vocab lookup robust --format json
vocab save robust --context "We need a robust architecture." --url https://example.com
vocab list [--search rob] [--source en] [--target fa] [--limit 20]
vocab show robust
vocab remove robust
vocab serve [--addr 127.0.0.1:8787]       # local web UI
```

Multi-word terms work without quotes (`vocab lookup ice cream`). Use
`--source` and `--target` to choose a language pair (`en` and `fa` are
supported). The same word with a different pair is a separate entry.

## Web UI

`vocab serve` starts a local dashboard (quick lookup, vocabulary list with
search and language filters, word details with contexts, read-only settings).
Open the URL it prints; it contains a per-run access token that is exchanged
for a session cookie.

Security: it only listens on loopback addresses (anything else is refused),
rejects non-loopback `Host` headers (DNS rebinding), requires the token cookie
(`HttpOnly`, `SameSite=Strict`) on every request, blocks cross-origin POSTs,
and serves a strict Content-Security-Policy with no inline script. API keys are
never sent to the browser. The daily review page arrives with the learning
milestone.

## Configuration

Environment variables:

| Variable | Purpose | Default |
|---|---|---|
| `VOCAB_DB` | SQLite database path | per-user data directory |
| `VOCAB_SOURCE_LANGUAGE` | default source language | `en` |
| `VOCAB_TARGET_LANGUAGE` | default target language | `fa` |
| `VOCAB_GOOGLE_TRANSLATE_API_KEY` | Google Cloud Translation key | unset |

Without a translation key, lookups still show definitions and print a note that
translation is unavailable. Keys are never stored in the database.

## Layout

```
cmd/vocab                 CLI entry point
bootstrap                 composition root and configuration
adapter/inbound/cli       Cobra commands (parse, call ports, render)
adapter/outbound/...      SQLite, dictionary, translation, clock and ID adapters
internal/application      ports and use cases
internal/domain/vocab     aggregate and invariants
```
