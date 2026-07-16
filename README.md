# LaunchDeck

A macOS terminal UI (TUI) for `launchctl`: browse GUI and system services in a
sidebar, inspect metadata/logs/raw `print` output, and run start/restart/stop/
enable/disable/load/unload actions — with mouse support and session restore.

Single static Go binary, no background daemon.

## Build

```bash
go build -o launchdeck ./cmd/launchdeck
```

Requires macOS (`launchctl` on `PATH`) and Go 1.24+.

## Run

```bash
./launchdeck
```

Session state (selection, filter, domain scope, sort, list scroll, active tab)
is saved to `~/.config/launchdeck/session.json` and restored on next launch.

## Keymap

While a modal (filter input, load prompt, action picker, confirm, sudo-retry)
is open, only that modal's own keys are live; all keys below are global
otherwise.

| Key                       | Effect                                              |
|---------------------------|------------------------------------------------------|
| `↑`/`k`, `↓`/`j`          | focus=sidebar: move selection; focus=detail: scroll detail |
| `Home` / `End`            | first / last visible row                            |
| `PgUp` / `PgDn`           | move selection ±one page (10 rows)                  |
| `Tab`                     | toggle focus sidebar ↔ detail                       |
| `1` / `2` / `3`           | detail tab: Metadata / Logs / Raw                   |
| `←` / `→`                 | cycle detail tab (prev/next)                        |
| `Ctrl-U` / `Ctrl-D`      | scroll the focused panel (±10 lines)                 |
| mouse wheel               | scroll the focused panel (±3 lines)                 |
| `a`                       | open the action picker for the selected service     |
| `y`/`Enter`, `n`/`Esc`    | confirm / cancel whichever prompt is open (destructive confirm or sudo-retry) |
| `/`                       | open the text filter                                |
| `d`                       | cycle domain scope: user → system → all             |
| `s` / `S`                 | cycle sort key / toggle sort direction              |
| `L`                       | open the load (bootstrap) plist-path prompt         |
| `r`                       | manual refresh                                      |
| `q`, `Ctrl-C`             | save session and quit                               |

Inside the action picker: `s` Start, `r` Restart, `k` Stop, `e` Enable,
`d` Disable, `u` Unload; `↑`/`↓` move the highlight, `Enter` picks it, `Esc`
cancels. Mouse users can click sidebar rows, detail tabs, and the status-bar
action buttons instead of using keys.

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

Also worth a spot-check: the system-domain **enumeration** banner ("system
requires sudo to enumerate — Retry with sudo") and its own Retry/cancel path,
and the per-service **inspect** retry (a system row's detail shows "requires
sudo to inspect" → Retry with sudo → parses into the Metadata/Raw tabs).
