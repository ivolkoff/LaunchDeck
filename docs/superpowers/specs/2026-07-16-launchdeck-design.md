# LaunchDeck — Design Spec

**Date:** 2026-07-16
**Status:** Approved (brainstorming), pending implementation plan

## Summary

LaunchDeck is a macOS terminal UI (TUI) dashboard for controlling `launchctl`
services. A single screen shows a sidebar service list, a detail panel, and a
status bar. Keyboard and mouse are both first-class (click rows, buttons, tabs;
scroll). Ships as a single Go binary, no background daemon.

The design is modeled on [herdr](https://github.com/ogulcancelik/herdr)'s
architecture: application state and intents are cleanly separated from the pure
rendering layer, so the UI framework is swappable.

## Goals

1. See a live list of `launchctl` services, filterable by domain and text pattern.
2. Select a service (click or keyboard) and inspect its metadata, tail its logs,
   and view the raw `launchctl print` dump.
3. Run actions on services: start, restart, stop, enable, disable, load, unload.
4. Persist UI session state so relaunching restores the exact view (selection,
   filters, sort, scroll) — the "minimized browser tab" feel.
5. Mouse interaction: click rows, action buttons, tabs; scroll list and logs.

## Non-Goals (YAGNI)

- No background daemon / socket. (Considered herdr-style daemon; the user's need
  is "restore where I left off", which a session file satisfies. Nothing needs to
  run while the TUI is closed.)
- No crash history / event timeline persistence.
- No favorites, groups, notes, or watch/alert rules.
- No cross-framework `Renderer` interface. (herdr does not do this either; it
  commits to one renderer and separates state/actions from rendering.)

## Stack

- **Go** — single static binary, no runtime, trivial to shell out to `launchctl`.
- **Bubble Tea** (Charm) — MVU architecture; closest Go analog to herdr's
  ratatui (render whole view from state).
- **Lip Gloss** — styling.
- **Bubbles** — ready-made `list` / `viewport` widgets.
- **bubblezone** — mouse hit-testing (marks screen "zones" → maps a click to an
  element). Analogous to herdr's `app/input/mouse.rs`.

## Architecture — core/ui split

The seam is **State + Intent → ViewModel → pure render**, mirroring herdr's
`app/` (state, actions, input) vs `ui/` (pure render widgets). Core has zero TUI
dependencies. The entire `ui/` package is the swappable adapter — not a
per-widget interface.

```
cmd/launchdeck/main.go          wire core + ui, start Bubble Tea
internal/
  launchctl/   ← core, ZERO TUI deps
    client.go    exec wrapper: List(), Print(label), action verbs
    parse.go     parse `launchctl list` + `launchctl print` → structs
    types.go     Service, ServiceDetail, Domain, ActionKind
  session/
    session.go   load/save ~/.config/launchdeck/session.json
  app/           ← framework-agnostic: state + intents + viewmodel (THE SEAM)
    state.go     AppState (services, selection, filters, sort, scroll, activePanel)
    intent.go    Intent: SelectService | RunAction | SetFilter | ToggleRaw | Refresh | Quit ...
    reduce.go    reduce(Intent, State) → State        (pure)
    viewmodel.go ViewModel: flat data for rendering (ListVM | DetailVM | StatusVM)
    derive.go    State → ViewModel                    (pure)
  ui/            ← swappable render layer (the whole package = the adapter)
    bubbletea/   model / view / list / detail / statusbar / mouse / keys / cmds
```

Swapping to tview later means rewriting `ui/bubbletea/` → `ui/tview/` against the
same `core`. State, domain logic, and parsing (the bulk of the value) are untouched.

## Data Flow (MVU)

- A `tea.Cmd` polls `launchctl` every **2s** → message → `reduce` → new State →
  `derive` → ViewModel → `View` renders. Manual refresh on `r`.
- Input (key/mouse) → `Update` translates to an `app.Intent` → `reduce(Intent, State)`.
  Mouse: the bubblezone zone id encodes the intent's target (which service / button).
- Log tail: on select, a `tea.Cmd` tails `StandardOutPath` / `StandardErrorPath`
  (read + follow), streaming lines into the detail log buffer.

## Scope Filter

Flat `launchctl list` (plus `launchctl print` per service for detail). A filter by
domain (user / system / all) plus a text pattern is toggled in the UI and stored
in the session. No hard domain boundary — it is a view filter.

## Actions + sudo

Modern domain-scoped `launchctl` verbs:

| Action        | Command                                        |
|---------------|------------------------------------------------|
| Start         | `launchctl kickstart <domain>/<label>`         |
| Restart       | `launchctl kickstart -k <domain>/<label>`      |
| Stop / kill   | `launchctl kill TERM <domain>/<label>`         |
| Load          | `launchctl bootstrap <domain> <plist>`         |
| Unload        | `launchctl bootout <domain>/<label>`           |
| Enable        | `launchctl enable <domain>/<label>`            |
| Disable       | `launchctl disable <domain>/<label>`           |

- User domain (`gui/$UID`) actions need no sudo. System domain (`system/`) needs root.
- **Best-effort then sudo retry:** attempt the action as the current user. On a
  permission failure ("Operation not permitted"), set `State.pendingSudo`; the UI
  offers **"Retry with sudo"**. On confirm, run via `tea.ExecProcess("sudo",
  "launchctl", ...)`: Bubble Tea releases the terminal (leaves alt-screen), `sudo`
  draws its own password prompt on the real tty, and the TUI resumes on completion,
  then refreshes.
- **Security:** the password is never handled or stored by our code — it flows only
  into the system `sudo`. No in-TUI password capture, no askpass helper.

## Detail Panel + Logs

When a service is selected, the right panel shows:

- **Metadata** parsed from `launchctl print gui/$UID/<label>`: label, domain, PID,
  last exit code, state, program + args, plist path.
- **Live log tail** of `StandardOutPath` / `StandardErrorPath` in a scrollable
  viewport. Missing/empty path → "no log configured", not a crash.
- **Raw toggle** — the full `launchctl print` output.

## Persistence (session file)

On quit (and periodically), write `~/.config/launchdeck/session.json`: selected
label, filters, sort order, domain scope, scroll position, active panel. On
startup, read it and seed State. Relaunch lands the user where they left off.

## Error Handling

- Non-macOS or `launchctl` not found → fatal at startup with a clear message.
- Action failure → exact stderr in the status bar; permission failure → sudo retry path.
- `launchctl print` output format varies by macOS version → on parse failure, fall
  back to showing the raw dump rather than crashing.

## Testing

**Tier 0 — unit (CI, no macOS needed):** `parse.go`, `reduce.go`, `derive.go`
against fixed captured output strings. Table tests for `Intent → State` and
`State → ViewModel`.

**Tier 1 — read-only integration (safe, default on darwin):** run real
`launchctl list` and `launchctl print gui/$UID/<label>` on the current machine,
feed the output to the parser, assert it does not crash and produces sane structs
(non-empty label, recognized domain). Catches format drift for the running macOS
version. Guard: `runtime.GOOS == "darwin"`, else `t.Skip`. Mutates nothing.

**Tier 2 — mutating integration (opt-in, throwaway agent):** the test creates a
disposable LaunchAgent in a temp dir — minimal plist, label
`com.launchdeck.itest.<pid>`, program `/bin/sh -c 'while true; do sleep 1; done'`.
It exercises the real verbs end-to-end in the **user domain** (`gui/$UID`, no sudo):
`bootstrap` → `enable`/`disable` → `kickstart` → `kickstart -k` → `kill TERM` →
`bootout`, verifying results via `launchctl print` (PID appears / changes / disappears).
This is the one runnable check proving the action path works against live launchd.

Tier 2 guardrails (hard):
- Runs only when `runtime.GOOS == "darwin"` **AND** env `LAUNCHDECK_INTEGRATION=1`;
  otherwise skip. A plain `go test` never touches the system.
- Touches only its own `com.launchdeck.itest.*` label, only the user domain, never system.
- `t.Cleanup`: `bootout` + remove the plist — guaranteed cleanup even on failure.

## Success Criteria

1. Launch → live service list, filterable by domain and pattern.
2. Select (click / keyboard) → detail shows metadata + tailing logs + raw toggle.
3. start / restart / stop / enable / disable / load / unload work on user services;
   system services attempt then offer "Retry with sudo".
4. Quit + relaunch → restored to the same selection / filters / scroll.
5. Mouse: click rows, action buttons, tabs; scroll list and logs.
