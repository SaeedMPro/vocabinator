# Shortcuts: lookup from any application

Select a word anywhere, press a key, see the meaning.

Vocabinator does **not** register global hotkeys itself. X11 needs native
libraries for that, and Wayland has no portable global-shortcut API. Instead,
bind a key in your desktop's own shortcut settings to one of these commands,
which work the same on every platform:

```bash
vocab lookup --selection --notify     # quick lookup, nothing saved
vocab save   --selection --notify     # save the word
```

`--notify` shows the result (or the error) as a desktop notification, because a
shortcut has no terminal to print to.

## What gets captured

| Option | Reads | Available on |
|---|---|---|
| `--selection` | the highlighted text (primary selection) | Linux |
| `--clipboard` | the text you last copied | Linux, macOS, Windows |

The clipboard is never used as a silent fallback for `--selection`: stale
clipboard text would look up the wrong word. Choose it explicitly.

The captured text is reduced to a term: whitespace is collapsed and surrounding
punctuation and quotes are trimmed (`“robust,”` becomes `robust`). Selections
longer than four words are rejected rather than guessed at.

**Context:** a selection contains only the word itself, so `save --selection`
cannot know the sentence around it. Pass `--context "..."` yourself when you
want it stored.

## Linux

Required tools (the first available is used):

| Session | Tools |
|---|---|
| X11 | `xclip` or `xsel` |
| Wayland | `wl-paste` (package `wl-clipboard`); `xclip` or `xsel` for XWayland applications |
| Notifications | `notify-send` (package `libnotify-bin` or `libnotify`) |

Wayland caveat: reading the primary selection with `wl-paste` needs a
compositor that exposes the data-control protocol. It is commonly reported to
work on wlroots compositors (sway, Hyprland) and KDE, but not on GNOME's
compositor; on GNOME, applications running through XWayland still work with
`xclip`. Test on your own setup: `wl-paste --primary` in a terminal after
highlighting some text.

Example bindings (adjust the key):

```
# sway / i3 config
bindsym $mod+d exec vocab lookup --selection --notify
bindsym $mod+Shift+d exec vocab save --selection --notify
```

GNOME and KDE: open the keyboard shortcut settings, add a custom shortcut and
use the command above.

## macOS and Windows

There is no primary selection on these systems, so use `--clipboard`: copy the
word (Cmd or Ctrl+C), then trigger `vocab lookup --clipboard --notify`.

- macOS notifications use `osascript`. Bind the command to a key with a
  Shortcuts or Automator "Run Shell Script" action.
- Windows notifications are not implemented yet; the command still prints the
  result and reports that notifications are unavailable.

Capturing the highlighted text directly on these systems would mean sending a
synthetic copy keystroke, which needs Accessibility permission on macOS. That
is not implemented.

## Verification status

Developed and tested on Linux. The selection and notification adapters are
unit tested with fake process runners on every platform, and the Linux argument
wiring was exercised end to end against a stub `xclip`. The real tools
(`xclip`, `xsel`, `wl-paste`, `pbpaste`, PowerShell, `osascript`) and the
Windows and macOS adapters have not been run on real systems yet.
