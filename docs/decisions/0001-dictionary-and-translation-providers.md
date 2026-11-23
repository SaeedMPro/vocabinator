# 0001: Dictionary and translation providers

Status: accepted (Milestone 3). Research date: 23/11/2026.

Research used web search only; the sandbox could not call the live APIs. Wire
formats below are from provider documentation and public examples and are
covered by fake-server tests, not by live calls. Verify against the real
services before release (see "Open verification items").

## Dictionary

| Provider | Auth / cost | Languages | Content | Licensing and limits |
|---|---|---|---|---|
| [Free Dictionary API](https://dictionaryapi.dev) | none, free | English focus | definitions, examples, synonyms, antonyms, phonetics, audio | Community service. No overall licence or rate limit found; audio files carry per-file licences (e.g. BY-SA 3.0 from Wikimedia Commons). |
| [Merriam-Webster](https://dictionaryapi.com/info/terms-of-service) | key; free for non-commercial use, 1,000 queries/day/key, two reference APIs | English | rich senses, thesaurus | Licence bars automated or recorded queries unless approved in writing. No explicit caching statement found. |
| Wordnik, Oxford, Datamuse | not evaluated in depth | | | Oxford's API terms are non-commercial research; Datamuse remains a candidate for synonyms/related words in enrichment (Milestone 7). |

**Decision:** Free Dictionary API behind `outbound.DictionaryProvider`.
It needs no key, covers every field the contract needs, and is replaceable.
Merriam-Webster was rejected because its licence restricts automated queries
and caps usage.

**Persistence constraint:** no licence for the data was found. Lookups are
live and are not cached or stored by this milestone. Before enrichment
(Milestone 7) persists dictionary content, confirm the terms or switch to a
source with a clear licence. Audio URLs are deliberately not part of the
contract yet for the same reason.

## Translation

| Provider | Auth / cost | Persian |
|---|---|---|
| [Google Cloud Translation](https://cloud.google.com/translation/pricing) (Basic, v2) | API key; first 500,000 characters per month free, then about $20 per million characters (USD list price) | `fa` supported |
| [DeepL API](https://developers.deepl.com/docs/getting-started/supported-languages) | key; free and paid plans | Persian listed for the next-generation model; glossary and formality are not available for it. Confirm via `/v3/languages`. |

**Decision:** Google Cloud Translation Basic v2 as the first adapter. Persian
is supported, pricing is metered per character with a free allowance, and the
REST shape is simple. DeepL stays a drop-in alternative behind
`outbound.TranslationProvider`.

## Design consequences

- Translation is per definition, not per isolated word, to limit ambiguity.
  The term itself is also translated and presented as a hint.
- A lookup translates at most 8 definitions plus the term, in one batched
  request, to bound cost.
- Translation failure degrades to a warning; the dictionary result is still
  returned.
- API keys are sent in headers, never URLs, and never stored in SQLite.

## Open verification items

- Confirm both response shapes and the Google `X-goog-api-key` header against
  the live services.
- Confirm Free Dictionary API usage terms for stored content.
