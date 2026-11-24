# Vocabinator

Turn unknown words into knowledge.

Look a word up, read its definitions with Persian translations, save it with the context you met it in. Local first, one binary: `vocab`.

## What we use

- **Go**, a single static binary with no CGO
- **SQLite** (`modernc.org/sqlite`, pure Go) for local storage
- **Cobra** for the CLI, **net/http** and embedded HTML for the local web UI
- **Free Dictionary API** for definitions
- **MyMemory** for translation: no account, key or card ([details](docs/decisions/0002-translation-provider.md))
- Hexagonal architecture: domain, ports, use cases, adapters

## Install

Download the archive for your system from the
[Releases](https://github.com/SaeedMPro/vocabinator/releases) page
(`vocabinator_<version>_<os>_<arch>`), or build from source (below).
Check the download against `SHA256SUMS`.

### Linux

```bash
tar -xzf vocabinator_*_linux_amd64.tar.gz      # or linux_arm64
install -m 755 vocabinator_*/vocab ~/.local/bin/vocab
vocab --version
```

Optional, for lookup of highlighted text and notifications:
`xclip` or `xsel` (X11), `wl-clipboard` (Wayland), `libnotify` (`notify-send`).

### macOS

```bash
tar -xzf vocabinator_*_darwin_arm64.tar.gz     # or darwin_amd64 for Intel
sudo install -m 755 vocabinator_*/vocab /usr/local/bin/vocab
xattr -d com.apple.quarantine /usr/local/bin/vocab   # binary is not signed
vocab --version
```

### Windows

1. Unzip `vocabinator_<version>_windows_amd64.zip` (or `windows_arm64`).
2. Move `vocab.exe` to a folder on your `PATH`, for example `C:\Tools`.
3. In a new terminal: `vocab --version`.

### From source (any OS)

Needs Go (version in `go.mod`).

```bash
git clone https://github.com/SaeedMPro/vocabinator.git
cd vocabinator
go build -o vocab ./cmd/vocab        # vocab.exe on Windows
```

> Tested on Linux. The Windows and macOS builds compile and are tested in CI,
> but have not been run on real machines by the author yet.

## Use

```bash
vocab lookup robust                  # definitions, examples, Persian translations; nothing saved
vocab lookup run --all               # every definition (default: first few per part of speech)
vocab lookup run --format json       # complete data for scripts
vocab save robust --context "We need a robust architecture."
vocab list
vocab show robust
vocab remove robust
vocab serve                          # local web UI (prints a URL with a token)
```

Highlighted or copied text: `vocab lookup --selection --notify`,
`vocab lookup --clipboard`. Bind these to a key in your desktop settings,
see [docs/shortcuts.md](docs/shortcuts.md).

## Configure

Environment variables, all optional:

| Variable | Purpose | Default |
|---|---|---|
| `VOCAB_DB` | database path | per-user data directory |
| `VOCAB_SOURCE_LANGUAGE` / `VOCAB_TARGET_LANGUAGE` | default language pair | `en` / `fa` |
| `VOCAB_TRANSLATION_PROVIDER` | `mymemory`, `google` or `none` | `mymemory` |
| `VOCAB_MYMEMORY_EMAIL` | raises MyMemory's daily quota (sent to MyMemory) | unset |
| `VOCAB_GOOGLE_TRANSLATE_API_KEY` | only for `google` | unset |

Definitions you look up are sent to the translation provider. Translations are
a learning aid; short fragments can be wrong. Whether MyMemory is reachable from
your network is not guaranteed: check with
`go test -tags integration -run Integration -v ./adapter/outbound/mymemory`.

## Develop

```bash
go test ./...
go vet ./...
```

Releases: push a tag such as `v0.1.0`; the workflow tests on Linux, Windows and
macOS, builds every platform and publishes the release.

MIT licence.
