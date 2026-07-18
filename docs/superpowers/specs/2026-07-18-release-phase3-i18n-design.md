# Release Phase 3 (i18n) — Design Spec

**Goal:** Localize the LaunchDeck TUI into English and Russian, auto-selecting
the language from the environment at startup with a config-file override.

**Status:** design approved 2026-07-18 (config variant A: dedicated
`config.json`; light 1-round subagent spec review).

## Scope

**In scope — every user-facing string rendered inside the TUI:**
- App-layer text produced by the pure seam (`internal/app`): `StatusMsg`
  values, detail `ErrMsg`/`LogNote`, list placeholders, the action-button
  labels, the prompt strings, and the rendered `RunState`/`EnableState` words.
- UI-layer chrome (`internal/ui/bubbletea`): header title, help overlay, the
  "terminal too small" notice, detail tab names, metadata field labels,
  "Select a service"/"Loading detail…", the text-select-mode banner.

**Out of scope (stays English):**
- CLI plumbing printed before the TUI starts: `--help` text, `--version`
  string, and the two startup errors ("macOS only", "launchctl not found in
  PATH"). These run before the config is loaded and `--help` is conventionally
  English.
- No runtime language switching (language is fixed once at startup).
- No external catalog files (translations compile into the binary).
- No plural/gender machinery — no current string interpolates a count, so it
  is not needed (YAGNI). If a count string is ever added, revisit.

## Architecture

Language is selected **once at startup** and never changes during a run.
That single fact drives the whole design: a package-global current language
read through translation helpers is safe (no reactive re-render needed), so
the pure `internal/app` seam keeps calling small helpers instead of threading
a translator parameter through every function. This is a deliberate,
bounded exception to the seam's purity, justified by the fixed-at-startup
lifetime.

### New package `internal/i18n`

```go
package i18n

type Lang int

const (
    En Lang = iota // default; also the zero value
    Ru
)

// SetLang sets the process-wide language. Call once at startup, before the
// UI or any T/Tf call that must be localized. Not safe for concurrent use
// with T/Tf; production code does not call it after startup, though tests may
// set it sequentially (resetting to En via t.Cleanup between cases).
func SetLang(l Lang)

// T returns the message for key in the current language. "Missing in the
// current language" means the selected language's field in the catalog entry
// is the empty string; an empty target-language value triggers the English
// fallback. Unknown key or such an empty value falls back to the English
// entry, then to the key itself (so a missing translation degrades visibly,
// never panics). A deliberately-blank Russian string is therefore not
// representable here; add an explicit presence flag if that is ever needed.
func T(key string) string

// Tf is T + fmt.Sprintf with the current language's format string.
func Tf(key string, args ...any) string

// Detect resolves the language. Precedence: a valid cfgLang wins; else the
// first of $LC_ALL, $LANG, $LANGUAGE that yields a known language; else En.
// getenv is injected for testability (pass os.Getenv in production).
func Detect(getenv func(string) string, cfgLang string) Lang
```

**Catalog:** one in-code map keyed by message key, each entry holding the
English and Russian strings:

```go
var catalog = map[string]struct{ en, ru string }{
    "action.ok":     {"%s ok", "%s ок"},
    "detail.select": {"Select a service", "Выберите сервис"},
    // …one entry per key…
}
```

The **English value of every entry is byte-for-byte the string in the code
today.** This is the invariant that keeps the existing test suite green:
`reduce_test.go`/`derive_test.go` run with no language set (default `En`), so
`T`/`Tf` reproduce the current English output exactly.

**Key naming:** dot-namespaced by area — `status.*` (StatusMsg), `detail.*`
(detail panel + metadata labels), `list.*` (placeholders), `prompt.*`
(prompts), `action.*` (button labels + action-result formats), `help.*` (help
overlay lines), `header.*`, `runstate.*`, `enablestate.*`. The plan enumerates
the full key→{en,ru} table; the mechanism here does not depend on the exact
list.

**Action-result format args:** the `%s` in every action-result format
(`action.ok` = `%s ok`, `action.timeout` = `%s timed out`, `action.sudo` =
`%s needs sudo…`, `action.failed` = `%s failed: …`) is the localized action
word, not the raw `msg.Action.String()` enum. Each `Action` value maps to a
word-level key (e.g. `action.restart` = `{"restart", "перезапуск"}`), and
callers pass `Tf("action.ok", T("action.<verb>"))` so Russian yields
`перезапуск ок`, not the mixed-language `restart ок`. The plan enumerates the
full `Action`→key mapping.

### Language detection

`Detect(getenv, cfgLang)`:
1. If `cfgLang` parses to a known language (`"ru"`/`"en"`, case-insensitive,
   first two letters), use it.
2. Else, in order, read `LC_ALL`, `LANG`, `LANGUAGE`; the first whose value
   parses to a known language wins (e.g. `ru_RU.UTF-8` → `Ru`).
3. Else `En`.

Parsing is three-valued — `parse(s) → (Lang, ok)`: lowercase the value, take
the leading run of ASCII letters, and match its first two characters. It is
`ok` only when those are exactly `"ru"` (→ `Ru`) or `"en"` (→ `En`); empty
input or anything else is not `ok`. A `cfgLang` is honored only when its parse
is `ok` (step 1); in the env loop (step 2) any value that is not `ok` is
skipped and the next var tried; `En` (step 3) applies only as the final
default. Only English and Russian are recognized; unknown or empty values
never win — they fall through to the next source, and detection ends at
English.

### Config file (variant A)

New package `internal/config`, mirroring `internal/ui/bubbletea/theme.go`:

```go
type Config struct {
    Lang string `json:"lang"` // "ru" | "en" | "" (absent → auto-detect)
}

func Path() (string, error)      // ~/.config/launchdeck/config.json
func Load(path string) Config    // missing/corrupt → zero Config (never errors)
```

A missing or malformed file yields an empty `Config` (`Lang == ""`), so
detection falls through to the environment. Documented in README alongside
`theme.json`.

### Startup wiring (`cmd/launchdeck/main.go`, `startTUI`)

After the platform guards, before `ui.New`:

```go
cfg := config.Config{}
if p, err := config.Path(); err == nil {
    cfg = config.Load(p)
}
i18n.SetLang(i18n.Detect(os.Getenv, cfg.Lang))
```

The rest of `startTUI` is unchanged. `run()` (flags/`--help`/`--version`) is
untouched — it runs before this and stays English.

## String migration

Replace each in-scope English literal with `i18n.T("key")` or
`i18n.Tf("key", args…)`:
- `internal/app/reduce.go` — `StatusMsg` assignments, `Detail.ErrMsg`.
- `internal/app/derive.go` — placeholders, `Buttons` labels, prompts,
  `LogNote`, `RunState`/`EnableState` words.
- `internal/ui/bubbletea/view.go` — header, help overlay, too-small notice.
- `internal/ui/bubbletea/detail.go` — tabs, metadata labels, "Select a
  service", "Loading detail…".
- `internal/ui/bubbletea/statusbar.go` — text-select banner, any labels.

Enum-only strings that are never shown to the user (`Mode` values
`"empty"`/`"ready"`/… used purely for switching, JSON tags, log stream
`"out"`/`"err"` tags) are **not** translated — they are internal keys.

## Layout

Cyrillic renders systematically wider than the English it replaces, and every
in-scope region is width-sensitive (header title, "terminal too small" notice,
detail tab names, column-aligned metadata labels, the action-button row, list
placeholders, `RunState`/`EnableState` words). Requirements:

- Width-bound regions must either truncate with an ellipsis or recompute their
  width from the active language's content — a fixed width sized for English
  must not clip or misalign Russian.
- The minimum-terminal-size threshold behind the "terminal too small" notice is
  derived from the longest active-language content, so a Russian session does
  not spuriously trip it at sizes that render English fine.
- A `SetLang(Ru)` render smoke test (UI layer, with
  `t.Cleanup(func(){ SetLang(En) })`) renders a representative small terminal
  size and asserts no overflow or column misalignment.

## Testing

- `i18n.Detect` — table test over the precedence rules: config override wins;
  each env var in order; `ru_RU.UTF-8` → `Ru`; unknown/empty → `En`.
- `i18n.T`/`Tf` — key present in both languages; missing-translation fallback
  to English; unknown-key fallback to the key; `Tf` formatting.
- `config.Load` — present file, absent file, corrupt file.
- Catalog completeness: iterate `catalog` and fail on any empty `ru` value;
  assert every key referenced by `T`/`Tf` in the code exists in `catalog` (a
  fixed key-set test). This catches typo'd/unmigrated keys (raw key shown) and
  silent half-English Russian output that the fallback would otherwise hide.
- Golden assertions for the currently-untested English chrome strings (header,
  help overlay, too-small notice, tabs, metadata labels) so the "reproduce
  English byte-for-byte" invariant is machine-checked, not merely asserted.
- Russian-render smoke checks: the app-layer assertion (`SetLang(Ru)` then
  assert one `derive`/`reduce` RU string) lives in an `internal/app` test, and
  every test that calls `SetLang(Ru)` must `t.Cleanup(func(){ SetLang(En) })`
  (or equivalent isolation) so the process-global language never leaks into a
  later English-default assertion. (The UI layout smoke test is in Layout.)
- The existing `internal/app` and `render_test` suites must stay green
  unchanged (English invariant).

## Non-goals / YAGNI

- No `golang.org/x/text/message`, `go-i18n`, or codegen.
- No per-string reload, no locale hot-swap, no plural rules.
- No translation of CLI `--help`/`--version`/startup errors.

## File plan

- Create `internal/i18n/i18n.go`, `internal/i18n/catalog.go`,
  `internal/i18n/i18n_test.go`.
- Create `internal/config/config.go`, `internal/config/config_test.go`.
- Modify `cmd/launchdeck/main.go` (startup wiring; `--help` text gains a
  `config.json` line documenting `lang`).
- Modify `internal/app/reduce.go`, `internal/app/derive.go`.
- Modify `internal/ui/bubbletea/view.go`, `detail.go`, `statusbar.go`.
- Add a new `internal/app` RU smoke test file (e.g. `i18n_render_test.go`, with
  `t.Cleanup(SetLang(En))`) and a UI-layer layout smoke test in
  `internal/ui/bubbletea`; existing `internal/app`/`render_test` cases stay
  unchanged.
- Update `README.md` (document `config.json` + `lang`, and auto-detection).
