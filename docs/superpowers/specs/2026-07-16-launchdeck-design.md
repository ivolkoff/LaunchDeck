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
4. Persist UI session state so relaunching restores the view (selection, filters,
   sort, list scroll, active tab) — the "minimized browser tab" feel. (Log scroll is not
   restored; the log buffer is cleared on quit and re-tails to the bottom.)
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
    client.go    exec wrapper: ScanDomain(domain), Print(domain,label), action verbs
    parse.go     parse `launchctl print <domain>` + `print <domain>/<label>` → structs
    types.go     Service, ServiceDetail, Domain, ActionKind
  session/
    session.go   load/save ~/.config/launchdeck/session.json
  app/           ← framework-agnostic: state + intents + viewmodel (THE SEAM)
    state.go     AppState (services, selection, filters{domainScope,textPattern},
                 filterBuffer, filterEditing, sort, scroll{list,log}, focus,
                 activeTab, detail{loadState,metadata,raw}, logRing(cap 5000),
                 tailIdentity, statusMsg, loadPrompt,
                 actionPicker{open, highlightedVerb},
                 pendingSudo{kind∈{action|inspect|enumerate}, target},
                 pendingConfirm{action,target}, firstScanDone, selectionResolved)
    intent.go    Msg reduce ingests = user Intent | async data message.
                 User Intent: SelectService | MoveSelection | RunAction |
                   ConfirmAction | CancelAction | OpenActionPicker |
                   MoveActionPicker | PickAction | CancelActionPicker |
                   OpenFilter | SetFilterBuffer |
                   CommitFilter | CancelFilter | CycleDomainScope | SetFilter |
                   SetSort | SetTab | FocusPanel | Scroll | OpenLoadPrompt |
                   SetLoadBuffer | SubmitLoad | CancelLoad | ConfirmSudo |
                   CancelSudo | Refresh | Quit
                 Async data msg: ServicesLoaded | ServiceDetailLoaded{target} |
                   LogLinesAppended{tailTarget} | ActionResult
    reduce.go    reduce(Msg, State) → State   (pure over the given Msg; all
                 launchctl output enters State only via async data msgs)
    viewmodel.go ViewModel: flat data for rendering (ListVM | DetailVM | StatusVM)
    derive.go    State → ViewModel                    (pure)
  ui/            ← swappable render layer (the whole package = the adapter)
    bubbletea/   model / view / list / detail / statusbar / mouse / keys / cmds
```

Swapping to tview later means rewriting `ui/bubbletea/` → `ui/tview/` against the
same `core`. State, domain logic, and parsing (the bulk of the value) are untouched.

## Data Flow (MVU)

- A `tea.Cmd` polls `launchctl` every **2s** → `ServicesLoaded` msg → `reduce` →
  new State → `derive` → ViewModel → `View` renders. Manual refresh on `r`.
  - **Single in-flight poll:** a new poll is not scheduled while one is running,
    and each poll carries a 3s timeout — on timeout/error the prior list is kept
    and the status bar notes "refresh failed", no crash.
  - **List merge (in `reduce`):** `ServicesLoaded` replaces the list but preserves
    selection *by label* and keeps both scroll offsets stable (clamped to the new
    lengths). Selection reconciliation depends on `State.firstScanDone`:
    - **First scan** (`firstScanDone` false): reconcile the persisted selection
      against the filtered/visible set (persisted text filter + domain scope
      applied) — if the persisted label resolves *and is visible*, bind to it;
      otherwise select the first visible row, or clear selection (show "Select a
      service") if the visible set is empty even when the underlying scan is
      non-empty. Then set `firstScanDone`, and set `selectionResolved` when a row
      is bound.
    - **Later scans** (`firstScanDone` true): if an already-resolved selected label
      (`selectionResolved`) is absent from the new list, keep it shown as "(gone)"
      until the user moves selection; if it reappears, re-bind to it.
    - A landing `ServicesLoaded` whose list no longer contains a pending
      `pendingConfirm` target auto-cancels that confirm (see Actions).
  - **Never clobber pendingSudo:** while `State.pendingSudo` is set, a landing
    `ServicesLoaded` updates the list only and leaves `pendingSudo` untouched.
- **Detail fetch:** on `SelectService`, a `tea.Cmd` runs `launchctl print
  <domain>/<label>` (3s timeout, single in-flight per selection) → parses it into
  `State.detail.metadata` and stores the raw dump in `State.detail.raw`, emitting
  one `ServiceDetailLoaded{target}` carrying the `<domain>/<label>` it fetched.
  Until it lands `State.detail.loadState` is `loading`
  and the Metadata/Raw tabs show "Loading detail…"; on timeout/error `loadState`
  is `error` and the tabs show the failure (permission-denied → "requires sudo to
  inspect" + Retry with sudo, see Actions). Selecting a different service
  supersedes any in-flight fetch: `reduce` drops any `ServiceDetailLoaded` whose
  `target` ≠ the current selection (a late result for a non-current selection is
  dropped).
  - **Refresh of the open detail:** after each `ActionResult` for the selected
    service, and on each 2s poll tick, re-fetch `print <domain>/<label>` for the
    still-selected service (same 3s timeout, single in-flight, same
    `target`-supersede rule), so the Metadata/Raw tabs track PID/state changes
    within ~2s instead of showing a point-in-time snapshot. A "(gone)" selection
    does not re-fetch (see List merge / Detail Panel).
- Input (key/mouse) → `Update` translates to a user `Intent` → `reduce(Intent, State)`.
  Mouse: the bubblezone zone id encodes the intent's target (service / button / tab).
- Log tail: on select, a `tea.Cmd` reads the initial buffer of the service's log
  paths, then follows, emitting `LogLinesAppended{tailTarget}` msgs carrying the
  `<domain>/<label>` whose tail produced them; `reduce` drops any whose
  `tailTarget` ≠ the current selection.
  - **Initial buffer (exact rule):** applied **per path** before merging — read
    the **last 64 KB** of each path, then keep the **last 500 lines** of that read
    (whichever bound is hit first wins; a shorter file yields fewer). A single line
    longer than the 64 KB read window is truncated to that window with a trailing
    `…` (truncated) marker.
  - **Out vs err:** both `StandardOutPath` and `StandardErrorPath` feed one
    interleaved buffer, each line prefixed `[out]`/`[err]`, ordered by read time.
    If the two paths are equal, follow it once (tag `[out]`); if only one is set,
    follow just that one.
  - **Bound:** the followed buffer is a ring capped at **5000 lines**; older lines drop.
  - **Lifecycle:** selecting a different service (and quitting) cancels the prior
    tail, closes its files, and clears the buffer before the new tail starts.
  - **File states:** rotation/truncation → reopen from the start of the new file;
    deletion → show "log removed"; a set-but-unreadable (permission-denied) path →
    "log unreadable (permission denied)", no crash.

## Keymap

Keyboard is first-class: every action reachable by mouse is reachable by key. Each
key emits an Intent (see Architecture). While a modal is open (filter input, load
prompt, action picker, destructive confirm, sudo-retry) only that modal's own keys
(editing / navigation / commit / cancel) are live; all global keys are suppressed.
Mouse is likewise modal: while a modal/confirm/sudo-retry is open, non-modal mouse
clicks (rows, action buttons, tabs) and wheel-scroll are suppressed — only clicks
inside the open modal's own zones are live. (This is why the destructive confirm's
captured target can only be invalidated by the target vanishing via
`ServicesLoaded`, never by a stray click or key — see Actions.)

| Key                      | Intent                       | Effect                                 |
|--------------------------|------------------------------|----------------------------------------|
| `↑`/`k`, `↓`/`j`         | MoveSelection / Scroll(detail) | focus=sidebar: move row ∓1; focus=detail: scroll detail ∓1 line |
| `Home` / `End`           | MoveSelection(top/bottom)    | first / last visible row               |
| `PgUp` / `PgDn`          | MoveSelection(∓page)         | one viewport up / down                 |
| `Tab`                    | FocusPanel                   | toggle focus sidebar ↔ detail          |
| `1` / `2` / `3`          | SetTab                       | Metadata / Logs / Raw                  |
| `←` / `→`                | SetTab(prev/next)            | cycle detail tab                       |
| `Ctrl-U`/`Ctrl-D`, wheel | Scroll(panel, ∓)             | Ctrl-U/D: ∓half viewport height; one wheel notch: 3 lines |
| `a`                      | OpenActionPicker             | open the verb picker for the selection (see Actions) |
| `y`/`Enter`, `n`/`Esc`   | ConfirmAction/CancelAction or ConfirmSudo/CancelSudo | answer whichever prompt is open — dispatches by open modal (destructive confirm vs sudo-retry) |
| `/`                      | OpenFilter                   | open the text-filter input             |
| `d`                      | CycleDomainScope             | cycle domain scope user → system → all |
| `s` / `S`                | SetSort                      | cycle sort key / toggle direction      |
| `L`                      | OpenLoadPrompt               | open the load plist-path prompt        |
| `r`                      | Refresh                      | manual poll now                        |
| `q`                      | Quit                         | save session and exit                  |

`activeTab` is the detail sub-tab (Metadata/Logs/Raw); `focus` is which viewport
(sidebar or detail) the keyboard nav/scroll keys target. `Tab` toggles `focus`.
`↑`/`k`/`↓`/`j` follow focus: focus=sidebar → `MoveSelection`; focus=detail →
`Scroll(detail)` one line. `Home`/`End`/`PgUp`/`PgDn` always `MoveSelection` (the
sidebar). Mouse-wheel scroll always targets the zone under the cursor regardless of
`focus`.

## Layout & resize (WindowSizeMsg)

`tea.WindowSizeMsg` (initial and on every resize) drives layout:
- **Reflow:** recompute the three regions — sidebar (left), detail (right), status
  bar (bottom row) — from the new width/height; the sidebar width =
  `round(totalWidth * 0.33)` clamped to `[24, 48]` cols, the detail panel takes the
  remainder, and each viewport's height = total minus the status row. Sidebar row
  labels are ellipsized (`…`) to the sidebar width.
- **Min size:** below a minimum (60×20), replace the whole view with a "terminal
  too small (need ≥60×20)" message until it grows back; no crash.
- **Re-clamp:** list and log scroll offsets are meaningful only relative to viewport
  height, so on every resize re-clamp both into the new range (this also finalizes
  the restored list-scroll offset once the first size is known).

## Scope Filter

**Domain-aware enumeration.** The sidebar is built from two domain-scoped scans,
not flat `launchctl list`, so every `Service` carries a real `Domain`:

- `launchctl print gui/$UID` → the per-user GUI services.
- `launchctl print system`  → the system daemons.

Recognized `Domain` values are exactly **`gui/<uid>`** and **`system`**; every
`print`, log, and action target is built as `<domain>/<label>` from the service's
own `Domain` (see Actions). Inspecting `system` may require elevation; split the
two permission-denied cases (see Error Handling):
- **Enumeration denied** (`launchctl print system` itself is denied): there are no
  scanned system rows, so show a banner "system requires sudo to enumerate — Retry
  with sudo" and zero system rows. Retry sets `pendingSudo{kind: enumerate, target:
  system}` and runs the **captured-output** command `sudo launchctl print system`
  (stdout piped to us, password prompt on the real tty, as with the inspect retry);
  on exit-0 its parsed services are merged into the domain-scoped list (replacing
  the `system` rows, same List-merge selection/scroll rules) and the banner clears;
  on non-zero the banner is replaced by its stderr; a user cancel or Ctrl-C of the
  prompt clears the banner and leaves the list unchanged. Every outcome clears
  `pendingSudo`.
- **Per-service inspect denied** (a scanned row's `launchctl print system/<label>`
  is denied): the row still lists (label + domain from the scan) but its detail
  shows "requires sudo to inspect" and offers **Retry with sudo** for the inspect.

**Filter** (a view filter, no hard boundary; stored in the session):

- **Domain scope:** `user` (gui only) / `system` / `all`. Unknown persisted values
  fall back to `all`.
- **Text pattern:** case-insensitive substring match against the service **label**;
  an empty pattern matches all services.

**Filter interaction.** `/` opens the text-filter input: `State.filterEditing`
becomes true and `State.filterBuffer` is seeded from the current pattern. Typing
edits `filterBuffer` (SetFilterBuffer); `Ctrl-U` clears it to empty; `Enter`
commits (CommitFilter → emits `SetFilter` with the buffer, closes the input);
`Esc` cancels (CancelFilter → discards the buffer, restores the prior pattern).
`d` cycles the domain scope `user → system → all` (CycleDomainScope → emits
`SetFilter` with the new scope). `reduce` applies `SetFilter` to `State.filters`
(re-derived and persisted); the buffer is UI-only and never persisted.

**List states:** before the first scan lands, show a "Loading services…"
placeholder; when the filter matches zero services, show "No matching services".

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
- **Action picker (`a`):** the only keyboard route to run a verb on the selection.
  `a` (OpenActionPicker) sets `State.actionPicker{open: true, highlightedVerb}`
  (highlight defaults to Start) — a modal list of the six per-selection verbs, each
  with a single-key shortcut: **`s`** Start, **`r`** Restart, **`k`** Stop,
  **`e`** Enable, **`d`** Disable, **`u`** Unload. (Load is not offered — it has no
  row target; use `L`.) `↑`/`↓` (MoveActionPicker) move the highlight; `Enter` or a
  verb's shortcut key (PickAction) closes the picker and dispatches `RunAction` on
  that verb (destructive verbs then set `pendingConfirm`, below); `Esc`
  (CancelActionPicker) closes it with no action. The picker targets the row selected
  when it opened. (Mouse users use the status-bar action buttons instead.)
- **Destructive actions** (Stop/kill, Unload/bootout, Disable) require an in-TUI
  **confirm step** first: `RunAction` on a destructive verb sets
  `State.pendingConfirm{action, target}` — the target `<domain>/<label>` captured
  at prompt-open — and the status bar shows a yes/no prompt. `y`/`Enter`
  (ConfirmAction) runs the captured action; `n`/`Esc` (CancelAction) clears it.
  Start/Restart/Enable/Load run without confirm. A stray key or click therefore
  cannot kill or unload a service. If the selection changes or the captured target
  vanishes ("(gone)") while the confirm is open, `pendingConfirm` auto-cancels (a
  landing `ServicesLoaded` that no longer contains the captured target clears it).
- **Permission detection.** An attempt is a permission failure when it exits
  non-zero **and** its lower-cased stderr matches any of the phrases (substring)
  `operation not permitted`, `permission denied`, `not privileged`, `requires
  root`, or the errno codes `errno 1` (EPERM) / `errno 13` (EACCES) matched with a
  **word-boundary regex `\berrno (1|13)\b`** — the boundary is required so
  `errno 12` / `errno 19` / `errno 100` do **not** match. Any other non-zero exit
  is a **generic** failure: show the raw stderr in the status bar, no sudo offer. A
  `(exit code, stderr) → {permission|generic}` table test pins this.
- **Best-effort then sudo retry:** attempt the action as the current user. On a
  permission failure, set `State.pendingSudo{kind: action, target}`; the UI offers
  **"Retry with sudo"**. On confirm, `ConfirmSudo` dispatches on `pendingSudo.kind`
  (action / inspect / enumerate); for `action`, run via `tea.ExecProcess("sudo",
  "launchctl", ...)`: Bubble Tea releases the terminal (leaves alt-screen), `sudo`
  draws its own password prompt on the real tty, and the TUI resumes on completion,
  then refreshes.
- **Captured-output sudo (inspect retry):** the sudo *action* retry above is
  fire-and-forget (ExecProcess captures no stdout); the sudo *inspect* retry
  (`pendingSudo{kind: inspect, target}`) must
  parse stdout. For it, run `sudo launchctl print <domain>/<label>` with stdout
  piped to us while the password prompt stays on the real tty (sudo prompts on the
  tty, not stdout). On exit-0, parse the captured dump into `State.detail` and emit
  `ServiceDetailLoaded`; on non-zero, show its stderr in the detail panel and leave
  `loadState` = `error`. Both outcomes clear `pendingSudo`.
- **Sudo retry terminal outcomes** (each clears `pendingSudo`):
  - user cancels the "Retry with sudo" prompt → no-op, status bar cleared.
  - `sudo` exits non-zero (wrong password / not a sudoer) → show its stderr, no
    further prompt.
  - user Ctrl-C's the `sudo` password prompt → the TUI resumes cleanly (re-enters
    alt-screen), treated as a cancel.
- **Security:** the password is never handled or stored by our code — it flows only
  into the system `sudo`. No in-TUI password capture, no askpass helper.
- **Single in-flight action:** at most one action (or its sudo retry) runs at a
  time. While an action is pending (awaiting its `ActionResult`) or a
  `pendingSudo`/`pendingConfirm` is set, a new `RunAction`/`ConfirmAction` is
  ignored (the status bar notes "action already running"); at most one `pendingSudo`
  exists at any time. This mirrors the single-in-flight poll and detail-fetch guards
  and prevents racing `launchctl` processes / `ActionResult`s.
- **Action timeout:** every per-service verb (kickstart, kill TERM, bootstrap,
  bootout, enable, disable) carries a **10s** timeout. On timeout the process is
  killed and an `ActionResult{timedout}` lands → the status bar shows "<action>
  timed out"; the TUI never hangs and `pendingSudo`/`pendingConfirm` are untouched.
- **Signal disposition:** outside `tea.ExecProcess`, SIGINT/SIGTERM is owned by our
  handler → save session, then quit (see Persistence). During `ExecProcess` the
  child (`sudo`) owns the tty and receives SIGINT itself, so Ctrl-C at the password
  prompt cancels only that sudo retry and does **not** trigger the quit-save; our
  handler is suspended for the duration of `ExecProcess` and restored when it
  returns. (Covered in the manual sudo checklist.)

## Load flow

`load` targets a service that is *not yet* in any scan, so it has no sidebar row
and no `print`-derived plist path.

- **Intents / state:** `L` opens the prompt (OpenLoadPrompt → `State.loadPrompt`
  holds the in-progress path buffer and the current candidate list); typing edits
  the buffer (SetLoadBuffer); `Enter` submits (SubmitLoad); `Esc` cancels
  (CancelLoad → discards `loadPrompt`). Unlike RunAction, load has no selected-row
  target — its target is the entered path.
- **Browse UX:** the buffer is a text input pre-populated with
  `~/Library/LaunchAgents/`; as the user types it shows a filtered candidate list
  of `*.plist` files found under the four standard dirs (`~/Library/LaunchAgents`,
  `/Library/LaunchAgents`, `/Library/LaunchDaemons`, `/System/Library/LaunchDaemons`)
  whose path contains the typed substring; `↑`/`↓` move the highlight and `Tab`
  completes to it. A leading `~` in the buffer is expanded to `$HOME` before both the
  substring match against the (absolute) candidate paths and the
  exists-and-ends-in-`.plist` check. A path outside the candidate list is still
  accepted if it exists and ends in `.plist`.
- **Domain inference (exact rule):** inspect the chosen path's directory
  components — a path under a `LaunchAgents` directory → `gui/$UID`; under a
  `LaunchDaemons` directory → `system` (which takes the sudo retry path). If
  neither component is present, **reject** with a status-bar error "cannot infer
  domain (path is under neither LaunchAgents nor LaunchDaemons)" and keep the
  prompt open.
- **Run + errors:** on a valid target, `launchctl bootstrap <domain> <plist>` runs.
  On success the next 2s scan surfaces the newly-loaded service as a normal row.
  Failures route to the status bar: a missing/invalid plist or any non-zero
  `bootstrap` exit shows its stderr; a permission failure (system domain) takes the
  sudo retry path (see Actions).

## Detail Panel + Logs

With no selection (empty list, or a stale/cleared session selection) the panel
shows a neutral **"Select a service"** placeholder. When a service is selected the
panel is a **tabbed** view — three clickable tabs, switched by mouse or `SetTab`
(keys `1`/`2`/`3` or `←`/`→`):

- **Metadata** — parsed from `launchctl print <domain>/<label>` into `State.detail`
  on select (domain from the service's own `Domain`; fetched on select — see Data
  Flow, shows "Loading detail…" until it lands): label, domain, PID, last exit
  code, **runState** ∈ {running, stopped} (has-PID; shown for the selected row — the
  list-level `status` sort key comes from `ScanDomain`, see Sorting),
  **enableState** ∈ {enabled, disabled} (best-effort, from the disabled flag in the
  print dump; this flag does not update promptly after enable/disable — see Success
  Criteria 3), program + args, plist path.
- **Logs** — the live tail (see Data Flow for size bound, out/err interleaving,
  lifecycle, and file-state handling) in a scrollable viewport. Missing/empty path
  → "no log configured", not a crash.
- **Raw** — the full `launchctl print` output (`State.detail.raw`; same fetch).
- **"(gone)" selection** — when the selected service drops out of a later scan (List
  merge keeps it shown as "(gone)"), the detail panel freezes the last-known
  Metadata/Raw with a "(gone) — service no longer present" banner, the log tail is
  stopped and its files closed (the buffer stays visible, read-only), and no detail
  re-fetch runs. It stays frozen until the user moves selection (or the service
  reappears and re-binds, resuming a fresh fetch + tail).

The per-service **action buttons** (Start / Restart / Stop / Enable / Disable /
Unload, each a bubblezone) live in the status bar and act on the selected service.

## Sorting

The list is sorted by one key at a time. Keys: **label** (default), **status**
(runState: running-before-stopped), **PID**. The PID and status keys come from
per-service PID/runState that **`ScanDomain` parses for every row** out of the
`launchctl print <domain>` services table (not the on-select detail fetch, which
covers only the selected row), so every unselected row has data to sort by.
Direction toggles ascending/descending (label/PID default ascending, status default
running-first). The
**`s`** key cycles the sort key, **`S`** toggles direction (`SetSort` intent);
both persist in the session. Sort is applied in `derive` when building the
`ListVM`, so it is a pure function of State and testable.

**Determinism.** Every sort is total and stable:
- **Secondary key:** ties on the primary key break on **label** ascending
  (case-insensitive, then bytewise for equal-fold labels), giving one canonical
  order.
- **Null PID:** stopped services have no PID; under the PID key they sort **after**
  all PID-bearing services regardless of direction (direction orders only the
  non-null PIDs), then by the secondary key.
- **"(gone)" rows:** a "(gone)" selection has no fresh fields and sorts **last**
  (after stopped), then by label.
- **Label case:** label comparison is case-insensitive primary with a bytewise
  tie-break, so `Foo` and `foo` have a stable order.

## Persistence (session file)

`~/.config/launchdeck/session.json` holds: selected label, text filter, domain
scope, sort (key + direction, see Sorting), **list scroll** offset, and active
tab. On startup, read it and seed State. (Log scroll is not persisted — the log
buffer is cleared on quit and re-tails to the bottom on relaunch.)

- **Save cadence:** debounced **1s** whenever any persisted field (selection, text
  filter, domain scope, sort, list scroll, active tab) changes — whether from a user
  intent or an async data msg (e.g. a poll's `ServicesLoaded` that re-binds or
  first-scan-reconciles the persisted selection) — plus a
  guaranteed final save on every exit path — normal quit **and** `SIGINT`/`SIGTERM`
  handled by our own handler (which saves before Bubble Tea tears down). The one
  exception is while `tea.ExecProcess` runs a sudo retry: there the child owns
  SIGINT (a cancel of that retry only), our handler is suspended, and no quit-save
  fires (see Actions → Signal disposition).
- **Startup / load robustness** (untrusted file — a trust boundary):
  - missing file (first run) or corrupt/unreadable JSON → start from defaults, no crash.
  - persisted selected label that no longer resolves → handled once at the first
    scan (see Data Flow → List merge): select the first visible row, or clear
    selection if the list is empty.
  - list scroll offset past the current list length → clamp into range (re-clamped
    on resize against the current viewport height, see Layout & resize).
  - unknown domain scope, sort key, or active tab → ignore that field and use its
    default.
- **Save robustness** (write side): before saving, `mkdir -p ~/.config/launchdeck/`
  (it does not exist on first run); write atomically via a temp file in the same dir
  + `rename`, so an interrupted `SIGINT`/`SIGTERM` save never leaves half-written
  JSON; on any save error, note it in the status bar and continue rather than crash.

## Error Handling

- Non-macOS or `launchctl` not found → fatal at startup with a clear message.
- Action failure → generic failures show exact stderr in the status bar; permission
  failures (detection criteria under Actions) take the sudo retry path.
- Permission-denied enumerating (`launchctl print system` without root) → no system
  rows; show a "system requires sudo to enumerate — Retry with sudo" banner, no
  crash.
- Permission-denied on a per-service `print` (inspecting a `system` service without
  root) → the row still lists; detail shows "requires sudo to inspect" and offers
  Retry with sudo, no crash.
- Corrupt/missing session file → start from defaults (see Persistence).
- `launchctl print` output format varies by macOS version → on **detail** parse
  failure, fall back to showing the raw dump rather than crashing.
- **Scan parse failure** (a `print gui/$UID` or `print system` dump that cannot be
  parsed into `Service` structs) → keep the prior list, show a "failed to parse
  services" banner, never crash.

## Testing

**Tier 0 — unit (CI, no macOS needed):** `parse.go`, `reduce.go`, `derive.go`
against fixed captured output strings. Table tests for `Msg → State` and
`State → ViewModel`. Required cases:

- text-pattern filter (substring / case-insensitivity / empty-matches-all).
- permission detection: `(exit code, stderr) → {permission | generic}`, including
  negative cases `errno 12` / `errno 19` / `errno 100` that must classify as
  **generic** (not caught by the `\berrno (1|13)\b` boundary).
- feeding captured permission stderr → asserts `pendingSudo` set and the
  "Retry with sudo" ViewModel; and the sudo terminal outcomes (cancel / non-zero /
  interrupt) each clear `pendingSudo`.
- a `ServicesLoaded` refresh while `pendingSudo` is set does **not** clobber it,
  and preserves selection-by-label + scroll.
- `derive` ViewModels for the loading, no-matching-services, and no-selection states.
- a destructive `RunAction` sets `pendingConfirm`; a later `ServicesLoaded` whose
  list drops the captured target auto-cancels it, while one that keeps it leaves it
  pending; `ConfirmAction`/`CancelAction` resolve it.
- an `ActionResult` with a timeout outcome → status-bar "timed out" message, no
  state corruption.
- detail fetch: `ServiceDetailLoaded` populates `State.detail` and clears
  `loadState`; a timeout/error sets `loadState` = error; a late
  `ServiceDetailLoaded{target}` whose `target` ≠ current selection is dropped (keyed
  off the payload identity), as is a `LogLinesAppended{tailTarget}` for a superseded
  tail.
- sudo inspect retry: a captured exit-0 dump parses into `ServiceDetailLoaded`; a
  captured non-zero sets `loadState` = error with stderr; both clear `pendingSudo`.
- load domain inference: `.../LaunchAgents/x.plist` → `gui/$UID`;
  `.../LaunchDaemons/x.plist` → `system`; a path under neither is rejected with the
  infer-error status message.
- sort determinism: for each key × direction, a fixture with ties and a null-PID
  (stopped) service derives one fixed order (label secondary tie-break, null PID
  last).
- action picker: `OpenActionPicker` opens it; `PickAction` on a non-destructive verb
  dispatches `RunAction`, on a destructive verb sets `pendingConfirm`;
  `CancelActionPicker` closes with no action.
- single in-flight action: a second `RunAction`/`ConfirmAction` fired while one is
  pending (or `pendingSudo`/`pendingConfirm` set) is ignored — no second launchctl,
  state unchanged but for the status note.
- first scan with a persisted filter matching nothing (scan non-empty): selection
  clears (no-selection ViewModel), `firstScanDone` set.
- a "(gone)" selection derives frozen last-known Metadata/Raw with a "(gone)" banner
  and a stopped, read-only tail.
- a row/button/tab click (or wheel-scroll) while `pendingConfirm` is open changes
  nothing (mouse is modal-suppressed).
- malformed scan output → `reduce` keeps the prior list and sets a "failed to parse
  services" banner, no panic.
- reflow: a `WindowSizeMsg` derives sidebar width = round(w*0.33) clamped [24,48],
  detail = remainder, labels ellipsized.
- session save into a nonexistent `~/.config/launchdeck/` creates the dir (mkdir -p)
  and writes atomically; a save error is surfaced, not fatal.

**Tier 1 — read-only integration (safe, default on darwin):** run the real
domain-scoped scans (`launchctl print gui/$UID`, and `launchctl print system` when
readable) plus a `launchctl print <domain>/<label>` for one scanned service, feed
the output to the parser, assert it does not crash and produces sane structs
(non-empty label, recognized domain of `gui/<uid>` or `system`). Catches format
drift for the running macOS version. Guard: `runtime.GOOS == "darwin"`, else
`t.Skip`. Mutates nothing.

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

**Manual (SC3 sudo path):** the system-domain + real-`sudo` flow is never
automated. Verify by hand with a checklist: pick a throwaway `system` daemon → run
an action → confirm the permission failure is detected → confirm "Retry with sudo"
→ enter password → action succeeds and the list refreshes; separately confirm the
cancel and wrong-password paths clear `pendingSudo` cleanly; and confirm Ctrl-C at
the sudo password prompt cancels only the retry (TUI resumes, no quit, no state
loss).

## Success Criteria

1. Launch → "Loading services…" then a live service list; a case-insensitive
   substring filter on label + domain scope narrow it; a non-matching pattern shows
   "No matching services".
2. Select (click / keyboard) → detail tabs Metadata / Logs / Raw render; no
   selection shows "Select a service".
3. Each action is observable within ~2s via `print`: start/kickstart → PID appears;
   restart → PID changes; stop/kill → PID gone; enable/disable → command exits 0
   (enableState's `print` flag does not update promptly, so success is the exit
   code, not a PID/state change); load/bootstrap → new row appears; unload/bootout
   → row disappears. Works on user services; system services attempt, then offer
   "Retry with sudo"; destructive actions confirm first.
4. Quit or Ctrl-C + relaunch → restored to the same selection / filter / domain
   scope / sort / list scroll / active tab (logs re-tail to the bottom).
5. Mouse: click rows, action buttons, tabs; scroll list and logs.
