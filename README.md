# LaunchDeck

**English** | [Русский](README.ru.md)

A macOS terminal UI (TUI) for `launchctl`: browse GUI and system services in a
sidebar, inspect metadata/logs/raw `print` output, and run start/restart/stop/
enable/disable/load/unload actions — with mouse support and session restore.

Single static Go binary, no background daemon.

## Install

With Go 1.24+ on macOS:

```bash
go install github.com/ivolkoff/launchdeck/cmd/launchdeck@latest
```

Or download a prebuilt macOS binary (Intel `amd64` or Apple Silicon `arm64`)
from the [Releases](https://github.com/ivolkoff/launchdeck/releases) page,
unpack the archive, and put `launchdeck` on your `PATH`.

## Build

From a clone:

```bash
go build -o launchdeck ./cmd/launchdeck
```

Requires macOS (`launchctl` on `PATH`) and Go 1.24+.

## Run

```bash
./launchdeck
```

Session state (selection, filter, domain scope, sort, list scroll, active tab,
sidebar width) is saved to `~/.config/launchdeck/session.json` and restored on
next launch. The **Logs** tab live-updates (re-reads the tail each ~2s poll,
newest first).

## Colors (theme)

Colors are configurable via `~/.config/launchdeck/theme.json`. The file is
optional — any omitted field falls back to the built-in palette, and a missing
or malformed file uses all defaults. Each value is a color lipgloss understands
(a 256-color index like `"42"` or a hex like `"#8be9fd"`):

```json
{
  "border": "240",
  "selected_fg": "231",
  "selected_bg": "62",
  "running": "42",
  "stopped": "244",
  "gone": "203",
  "tab_active_fg": "231",
  "tab_active_bg": "62",
  "gutter_fg": "250",
  "gutter_bg": "238",
  "accent": "213",
  "muted": "244"
}
```

## Language

The interface is available in English and Russian. The language is picked at
startup: `~/.config/launchdeck/config.json` wins if it sets one, otherwise it
is auto-detected from `$LC_ALL` / `$LANG` / `$LANGUAGE` (a `ru*` locale selects
Russian; anything else is English).

```json
{
  "lang": "ru"
}
```

`lang` accepts `"ru"` or `"en"`. Omit the file (or the field) to auto-detect.
The CLI `--help`/`--version` output stays English.

## Keymap

While a modal (filter input, load prompt, action picker, confirm, sudo-retry)
is open, only that modal's own keys are live; all keys below are global
otherwise.

| Key                       | Effect                                              |
|---------------------------|------------------------------------------------------|
| `↑`/`k`, `↓`/`j`          | focus=sidebar: move selection (list scrolls to keep it in view); focus=detail: scroll Logs/Raw |
| `Home` / `End`            | first / last visible row                            |
| `PgUp` / `PgDn`           | move selection ±one page (10 rows)                  |
| `Tab`                     | toggle focus sidebar ↔ detail                       |
| `1` / `2` / `3`           | detail tab: Metadata / Logs / Raw                   |
| `←` / `→`                 | cycle detail tab (prev/next)                        |
| `Ctrl-U` / `Ctrl-D`      | scroll the detail panel's Logs/Raw body (±10 lines)  |
| mouse wheel over detail   | scroll the detail panel's Logs/Raw body (±3 lines)  |
| `a`                       | open the action picker for the selected service     |
| `y`/`Enter`, `n`/`Esc`    | confirm / cancel whichever prompt is open (destructive confirm or sudo-retry) |
| `/` or `f`                | open the sidebar filter (regex, live as you type; `Esc` cancels) |
| `d`                       | toggle domain scope: user only ↔ user + system      |
| `s` / `S`                 | cycle sort key / toggle sort direction              |
| `L`                       | open the load (bootstrap) plist-path prompt         |
| `m`                       | capture / release the mouse (off by default → drag selects text) |
| `r`                       | manual refresh                                      |
| `q`, `Ctrl-C`             | save session and quit                               |

**Selecting and copying text** works out of the box: the mouse is not captured
by default, so a plain drag selects text and ⌘C copies it, just like a text
editor. Press `m` to **capture** the mouse for in-app clicks (rows, tabs,
buttons), wheel scrolling, and dragging the divider; press `m` again to release
it back to text selection.

Inside the action picker: `s` Start, `r` Restart, `k` Stop, `e` Enable,
`d` Disable, `u` Unload; `↑`/`↓` move the highlight, `Enter` picks it, `Esc`
cancels. The status bar shows these keys on each button (`a→[s Start]…`).

With the mouse captured (press `m`), you can click sidebar rows, detail tabs,
and the status-bar action buttons instead of using keys, scroll with the wheel,
and **drag the divider** between the sidebar and detail panel to resize them
(the panels won't shrink below a safe minimum).

## Manual sudo checklist

System-domain (`system/`) actions need root; user-domain (`gui/$UID`) actions
never do. This path always goes through the real `sudo` on your tty, so it's
verified by hand rather than by an automated test:

- [ ] Pick a throwaway `system` daemon, select it, open the action picker
      (`a`) and run an action (e.g. Start).
- [ ] Confirm the best-effort attempt fails with a **permission** failure
      (not a generic one) and the status bar offers **"Retry with sudo"**.
- [ ] Confirm the retry (`y`/`Enter`) suspends the TUI, `sudo` prompts for a
      password on the real terminal, and after a correct password the TUI
      resumes, the action succeeds, and the list/detail refresh.
- [ ] **Cancel path:** open "Retry with sudo" again and cancel it (`n`/`Esc`)
      — confirm `pendingSudo` clears, the status bar returns to normal, and no
      state is lost (selection/filter/scroll unchanged).
- [ ] **Wrong-password path:** retry with sudo and enter an incorrect
      password (or let `sudo` fail) — confirm `pendingSudo` clears, the
      failure's stderr is shown, and no further prompt appears.
- [ ] **Ctrl-C-at-prompt path:** retry with sudo and press Ctrl-C at the
      password prompt — confirm the TUI resumes cleanly (re-enters the
      alt-screen), the app does **not** quit, `pendingSudo` clears, and state
      is otherwise unchanged.

Note: only explicit **actions** offer a sudo retry. Inspecting a `system`
service you can't read shows a non-modal "requires sudo to inspect — run
launchdeck with sudo" in the detail panel; enumerating system services without
root simply omits them (run the whole app under `sudo` to see them).

## License

Copyright (C) 2026 volkoffskij.
GPL-3.0-or-later — see [`LICENSE`](LICENSE).
