# Release Phase 3 (i18n) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Localize the LaunchDeck TUI into English and Russian, auto-selecting the language from the environment at startup with a `config.json` override.

**Architecture:** A new `internal/i18n` package holds an in-code EN/RU catalog and a process-global current language set once at startup; `T`/`Tf` helpers look up keys. A new `internal/config` package reads `~/.config/launchdeck/config.json`. Every user-facing string in the pure `internal/app` seam and the `internal/ui/bubbletea` layer is swapped for a `T`/`Tf` call. The English value of each catalog entry is byte-for-byte the current string, so the existing suite stays green.

**Tech Stack:** Go 1.24, stdlib only (`encoding/json`, `fmt`, `strings`, `unicode/utf8`). No new dependencies.

## Global Constraints

- **No new dependencies.** Catalog is in-code Go maps; no `x/text`, `go-i18n`, or codegen.
- **English invariant:** every catalog entry's `en` value is byte-for-byte the string in the code today. Tests run with the default language (`En`), so migrated code must reproduce current English output exactly.
- **Language is fixed once at startup;** production never calls `SetLang` after that. Tests that call `SetLang(Ru)` must `t.Cleanup(func(){ SetLang(En) })`.
- **Scope:** TUI chrome only. CLI `--help`/`--version` text and the pre-TUI startup errors ("macOS only", "launchctl not found in PATH") stay English.
- **Stable identifiers stay English:** zone-mark ids (`"tab:Metadata"`, `"btn:Start"`), `buttonKey` map keys, log-stream tags (`[out]`/`[err]`), and internal `Mode`/enum switch strings are NOT translated — only their rendered display is.
- **Module path is `github.com/volkoffskij/launchdeck`.**
- Commit messages: plain conventional commits, no AI attribution.

---

## File Structure

- **Create** `internal/i18n/i18n.go` — `Lang`, `SetLang`, `T`, `Tf`, `parse`, `Detect`.
- **Create** `internal/i18n/catalog.go` — the full EN/RU catalog map.
- **Create** `internal/i18n/i18n_test.go` — Detect table, T/Tf fallback, catalog completeness.
- **Create** `internal/config/config.go` — `Config`, `Path`, `Load` (mirrors `theme.go`).
- **Create** `internal/config/config_test.go` — present/absent/corrupt file.
- **Modify** `internal/app/reduce.go` — `StatusMsg`/`ErrMsg` literals → `T`/`Tf`, verb localization.
- **Modify** `internal/app/derive.go` — placeholders, prompts, `LogNote`, `RunState`/`EnableState` → `T`/`Tf`.
- **Create** `internal/app/i18n_render_test.go` — RU smoke over `Derive`/`Reduce`.
- **Modify** `internal/ui/bubbletea/view.go` — header, help overlay, too-small notice.
- **Modify** `internal/ui/bubbletea/detail.go` — "Select a service", "Loading detail…", "(gone)…", metadata labels (dynamic alignment), tab display names.
- **Modify** `internal/ui/bubbletea/statusbar.go` — text-select banner, button display labels.
- **Modify** `internal/ui/bubbletea/render_test.go` — add golden English chrome asserts + a `SetLang(Ru)` layout smoke.
- **Modify** `cmd/launchdeck/main.go` — startup wiring (config load + Detect + SetLang); `--help` gains a `config.json`/`lang` line.
- **Modify** `README.md` — document `config.json` `lang` and auto-detection.

---

## Task 1: `internal/i18n` package (mechanism + full catalog)

**Files:**
- Create: `internal/i18n/i18n.go`
- Create: `internal/i18n/catalog.go`
- Test: `internal/i18n/i18n_test.go`

**Interfaces:**
- Produces: `type Lang int` with `En`/`Ru`; `func SetLang(Lang)`; `func T(string) string`; `func Tf(string, ...any) string`; `func Detect(getenv func(string) string, cfgLang string) Lang`. Package-global `catalog map[string]struct{ en, ru string }`.

- [ ] **Step 1: Write the failing test** — `internal/i18n/i18n_test.go`:

```go
package i18n

import "testing"

func TestParseThreeValued(t *testing.T) {
	cases := []struct {
		in   string
		want Lang
		ok   bool
	}{
		{"ru", Ru, true},
		{"RU", Ru, true},
		{"ru_RU.UTF-8", Ru, true},
		{"russian", Ru, true},
		{"en", En, true},
		{"en_US.UTF-8", En, true},
		{"", En, false},
		{"fr", En, false},
		{"xx", En, false},
		{"e", En, false},
	}
	for _, c := range cases {
		got, ok := parse(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("parse(%q) = (%v,%v), want (%v,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestDetectPrecedence(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	// config override wins over env
	if got := Detect(env(map[string]string{"LANG": "en_US"}), "ru"); got != Ru {
		t.Errorf("cfg override: got %v want Ru", got)
	}
	// invalid config falls through to env
	if got := Detect(env(map[string]string{"LANG": "ru_RU"}), "fr"); got != Ru {
		t.Errorf("cfg invalid → env: got %v want Ru", got)
	}
	// LC_ALL beats LANG
	if got := Detect(env(map[string]string{"LC_ALL": "ru_RU", "LANG": "en_US"}), ""); got != Ru {
		t.Errorf("LC_ALL precedence: got %v want Ru", got)
	}
	// unset LC_ALL, LANG=ru wins (the critical regression: empty must not win as En)
	if got := Detect(env(map[string]string{"LANG": "ru_RU"}), ""); got != Ru {
		t.Errorf("empty LC_ALL then LANG=ru: got %v want Ru", got)
	}
	// nothing set → En
	if got := Detect(env(map[string]string{}), ""); got != En {
		t.Errorf("default: got %v want En", got)
	}
	// unknown everywhere → En
	if got := Detect(env(map[string]string{"LC_ALL": "fr", "LANG": "de"}), "xx"); got != En {
		t.Errorf("unknown → En: got %v want En", got)
	}
}

func TestTFallback(t *testing.T) {
	SetLang(En)
	t.Cleanup(func() { SetLang(En) })
	// unknown key → the key itself
	if got := T("no.such.key"); got != "no.such.key" {
		t.Errorf("unknown key: got %q", got)
	}
	// known key, English
	if got := T("list.empty"); got != "No matching services" {
		t.Errorf("en list.empty: got %q", got)
	}
	// Russian
	SetLang(Ru)
	if got := T("list.empty"); got != "Нет подходящих сервисов" {
		t.Errorf("ru list.empty: got %q", got)
	}
	// Tf formatting
	SetLang(En)
	if got := Tf("status.ok", "restart"); got != "restart ok" {
		t.Errorf("Tf en: got %q", got)
	}
}

// TestCatalogComplete guards the two silent-failure modes: an empty en (would
// show the raw key) or empty ru (would show half-English Russian UI).
func TestCatalogComplete(t *testing.T) {
	for k, e := range catalog {
		if e.en == "" {
			t.Errorf("catalog[%q].en is empty", k)
		}
		if e.ru == "" {
			t.Errorf("catalog[%q].ru is empty", k)
		}
	}
}
```

- [ ] **Step 2: Run it to verify it fails** — `go test ./internal/i18n/` → FAIL (package/symbols undefined).

- [ ] **Step 3: Write `internal/i18n/i18n.go`:**

```go
// Package i18n holds LaunchDeck's in-binary EN/RU message catalog and the
// process-global current language. The language is chosen once at startup
// (see Detect) and read through T/Tf; it does not change during a run.
package i18n

import (
	"fmt"
	"strings"
)

type Lang int

const (
	En Lang = iota // default; also the zero value
	Ru
)

var current Lang // En (zero value) until SetLang

// SetLang sets the process-wide language. Production calls it once at startup,
// before any localized T/Tf call. Not safe for concurrent use with T/Tf.
func SetLang(l Lang) { current = l }

// T returns the message for key in the current language. A missing entry (or an
// empty value for the current language) falls back to English, then to the key
// itself, so a gap degrades visibly and never panics.
func T(key string) string {
	e, ok := catalog[key]
	if !ok {
		return key
	}
	if current == Ru && e.ru != "" {
		return e.ru
	}
	return e.en
}

// Tf is T followed by fmt.Sprintf with the current language's format string.
func Tf(key string, args ...any) string {
	return fmt.Sprintf(T(key), args...)
}

// parse maps a locale-ish value to a language. ok is true only when the leading
// run of ASCII letters begins with exactly "ru" or "en" (case-insensitive);
// empty input or anything else is not ok.
func parse(s string) (Lang, bool) {
	i := 0
	for i < len(s) {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			i++
			continue
		}
		break
	}
	head := strings.ToLower(s[:i])
	switch {
	case strings.HasPrefix(head, "ru"):
		return Ru, true
	case strings.HasPrefix(head, "en"):
		return En, true
	}
	return En, false
}

// Detect resolves the language. Precedence: a valid cfgLang wins; else the first
// of LC_ALL, LANG, LANGUAGE that parses; else En. getenv is injected so tests
// can supply an environment (pass os.Getenv in production).
func Detect(getenv func(string) string, cfgLang string) Lang {
	if l, ok := parse(cfgLang); ok {
		return l
	}
	for _, k := range []string{"LC_ALL", "LANG", "LANGUAGE"} {
		if l, ok := parse(getenv(k)); ok {
			return l
		}
	}
	return En
}
```

- [ ] **Step 4: Write `internal/i18n/catalog.go`** with the full table. Every `en` is byte-for-byte the current code string.

```go
package i18n

// catalog maps a dot-namespaced key to its English and Russian text. The en
// value of every entry MUST be byte-for-byte the string in the code today (the
// English invariant that keeps the existing suite green). Format entries use
// fmt verbs consumed by Tf.
var catalog = map[string]struct{ en, ru string }{
	// --- status line (internal/app/reduce.go) ---
	"status.infer_domain": {"cannot infer domain (path is under neither LaunchAgents nor LaunchDaemons)", "не удалось определить домен (путь не в LaunchAgents и не в LaunchDaemons)"},
	"status.load":         {"load…", "загрузка…"},
	"status.timeout":      {"%s timed out", "%s: таймаут"},
	"status.ok":           {"%s ok", "%s: ок"},
	"status.load_perm":    {"load failed (permission denied) — system daemons must be loaded with elevated privileges", "загрузка не удалась (отказано в доступе) — системные демоны загружаются с повышенными привилегиями"},
	"status.needs_sudo":   {"%s needs sudo — Retry with sudo", "%s: нужен sudo — повторить с sudo"},
	"status.failed":       {"%s failed: %s", "%s: ошибка: %s"},
	"status.busy":         {"action already running", "действие уже выполняется"},
	"status.enum_root":    {"system services need root to enumerate (run launchdeck with sudo to see them)", "перечисление системных сервисов требует root (запустите launchdeck под sudo)"},
	"status.parse_fail":   {"failed to parse services", "не удалось разобрать список сервисов"},

	// --- detail error (internal/app/reduce.go) ---
	"detail.err_sudo": {"requires sudo to inspect — run launchdeck with sudo to view system services", "нужен sudo для просмотра — запустите launchdeck под sudo, чтобы видеть системные сервисы"},

	// --- localized action verbs (used inside status/prompt formats) ---
	"action.verb.start":   {"start", "запуск"},
	"action.verb.restart": {"restart", "перезапуск"},
	"action.verb.stop":    {"stop", "остановка"},
	"action.verb.enable":  {"enable", "включение"},
	"action.verb.disable": {"disable", "отключение"},
	"action.verb.unload":  {"unload", "выгрузка"},
	"action.verb.load":    {"load", "загрузка"},
	"action.verb.unknown": {"unknown", "неизвестно"},

	// --- prompts (internal/app/derive.go) ---
	"prompt.sudo":    {"Retry with sudo? (y/n)", "Повторить с sudo? (y/n)"},
	"prompt.confirm": {"%s %s? (y/n)", "%s %s? (y/n)"},
	"prompt.filter":  {"filter: ", "фильтр: "},
	"prompt.load":    {"load plist: ", "загрузить plist: "},
	"prompt.action":  {"action: %s (s/r/k/e/d/u, Enter, Esc)", "действие: %s (s/r/k/e/d/u, Enter, Esc)"},

	// --- list placeholders (internal/app/derive.go) ---
	"list.loading": {"Loading services…", "Загрузка сервисов…"},
	"list.empty":   {"No matching services", "Нет подходящих сервисов"},

	// --- run / enable state words (internal/app/derive.go) ---
	"runstate.running": {"running", "работает"},
	"runstate.stopped": {"stopped", "остановлен"},
	"enable.enabled":   {"enabled", "включён"},
	"enable.disabled":  {"disabled", "отключён"},

	// --- log note (internal/app/derive.go) ---
	"log.none": {"no log configured", "лог не настроен"},

	// --- action-button display labels (internal/ui/bubbletea/statusbar.go) ---
	"btn.Start":   {"Start", "Запуск"},
	"btn.Restart": {"Restart", "Перезапуск"},
	"btn.Stop":    {"Stop", "Стоп"},
	"btn.Enable":  {"Enable", "Вкл"},
	"btn.Disable": {"Disable", "Выкл"},
	"btn.Unload":  {"Unload", "Выгрузка"},

	// --- detail panel (internal/ui/bubbletea/detail.go) ---
	"detail.select":  {"Select a service", "Выберите сервис"},
	"detail.loading": {"Loading detail…", "Загрузка деталей…"},
	"detail.gone":    {"(gone) — service no longer present", "(удалён) — сервиса больше нет"},

	// --- metadata labels (colon + alignment added in code) ---
	"meta.label":   {"Label", "Метка"},
	"meta.domain":  {"Domain", "Домен"},
	"meta.pid":     {"PID", "PID"},
	"meta.exit":    {"Last exit", "Выход"},
	"meta.run":     {"Run", "Статус"},
	"meta.enable":  {"Enable", "Включён"},
	"meta.program": {"Program", "Программа"},
	"meta.plist":   {"Plist", "Plist"},

	// --- detail tab display names (zone id stays English) ---
	"tab.Metadata": {"Metadata", "Метаданные"},
	"tab.Logs":     {"Logs", "Логи"},
	"tab.Raw":      {"Raw", "Raw"},

	// --- header + too-small (internal/ui/bubbletea/view.go) ---
	"view.too_small": {"terminal too small (need ≥60×20)", "терминал слишком мал (нужно ≥60×20)"},
	"header.title":   {" LaunchDeck — launchctl services · ? help", " LaunchDeck — сервисы launchctl · ? справка"},

	// --- help overlay (internal/ui/bubbletea/view.go) ---
	"help.title":           {"LaunchDeck — help", "LaunchDeck — справка"},
	"help.nav":             {"Navigation", "Навигация"},
	"help.nav.move":        {"  ↑/k ↓/j      move selection (sidebar) · scroll (detail, by focus)", "  ↑/k ↓/j      выбор (меню) · прокрутка (детали, по фокусу)"},
	"help.nav.homeend":     {"  Home/End     first / last row", "  Home/End     первая / последняя строка"},
	"help.nav.page":        {"  PgUp/PgDn    page up / down", "  PgUp/PgDn    страница вверх / вниз"},
	"help.nav.tab":         {"  Tab          switch focus: sidebar ↔ detail", "  Tab          фокус: меню ↔ детали"},
	"help.nav.tabs":        {"  1/2/3  ←/→   detail tabs: Metadata / Logs / Raw", "  1/2/3  ←/→   вкладки: Метаданные / Логи / Raw"},
	"help.nav.scroll":      {"  Ctrl-U/D     scroll detail ±10   ·   mouse wheel ±3", "  Ctrl-U/D     прокрутка деталей ±10  ·  колесо ±3"},
	"help.actions":         {"Actions", "Действия"},
	"help.actions.suffix":  {" (on the selected service)", " (над выбранным сервисом)"},
	"help.actions.picker1": {"  a            action picker → s start · r restart · k stop", "  a            меню действий → s запуск · r перезапуск · k стоп"},
	"help.actions.picker2": {"               e enable · d disable · u unload", "               e включить · d отключить · u выгрузить"},
	"help.actions.confirm": {"  y/Enter n/Esc  confirm / cancel a prompt", "  y/Enter n/Esc  подтвердить / отменить"},
	"help.actions.load":    {"  L            load a plist (bootstrap)", "  L            загрузить plist (bootstrap)"},
	"help.view":            {"View", "Вид"},
	"help.view.filter":     {"  /            filter by name        d   user ↔ user+system", "  /            фильтр по имени       d   user ↔ user+system"},
	"help.view.sort":       {"  s / S        sort key / direction  r   refresh now", "  s / S        сортировка / порядок  r   обновить"},
	"help.view.mouse":      {"  m            capture mouse (click · wheel · divider) — off = drag selects text", "  m            захват мыши (клик · колесо · разделитель) — выкл = drag выделяет"},
	"help.view.help":       {"  ?            this help             q / Ctrl-C  quit (saves)", "  ?            эта справка           q / Ctrl-C  выход (сохр.)"},
	"help.mouse":           {"Mouse", "Мышь"},
	"help.mouse.desc":      {"  click rows/tabs/buttons · wheel scroll · drag the divider to resize", "  клики строк/вкладок/кнопок · колесо · тяните разделитель"},
	"help.footer":          {"press ? or Esc to close", "нажмите ? или Esc чтобы закрыть"},
}
```

- [ ] **Step 5: Run the tests** — `go test ./internal/i18n/` → PASS (all four tests).

- [ ] **Step 6: Commit**

```bash
git add internal/i18n/
git commit -m "feat(i18n): add EN/RU catalog, T/Tf, and env language detection"
```

---

## Task 2: `internal/config` package

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces: `type Config struct{ Lang string }`; `func Path() (string, error)`; `func Load(path string) Config`.

- [ ] **Step 1: Write the failing test** — `internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPresent(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte(`{"lang":"ru"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Load(p).Lang; got != "ru" {
		t.Errorf("Lang = %q, want ru", got)
	}
}

func TestLoadAbsent(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nope.json")
	if got := Load(p).Lang; got != "" {
		t.Errorf("absent → Lang = %q, want empty", got)
	}
}

func TestLoadCorrupt(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte(`{ not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Load(p).Lang; got != "" {
		t.Errorf("corrupt → Lang = %q, want empty", got)
	}
}
```

- [ ] **Step 2: Run it to verify it fails** — `go test ./internal/config/` → FAIL (undefined).

- [ ] **Step 3: Write `internal/config/config.go`:**

```go
// Package config reads LaunchDeck's optional ~/.config/launchdeck/config.json.
// It mirrors the theme loader: a missing or malformed file yields a zero Config
// (never an error), so callers always get usable defaults.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds the non-visual settings. Lang is "ru" | "en" | "" (absent →
// auto-detect from the environment).
type Config struct {
	Lang string `json:"lang"`
}

// Path returns ~/.config/launchdeck/config.json.
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "launchdeck", "config.json"), nil
}

// Load reads the config file. A missing or corrupt file yields a zero Config
// (never an error).
func Load(path string) Config {
	var c Config
	data, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	_ = json.Unmarshal(data, &c) // parse error keeps the zero Config
	return c
}
```

- [ ] **Step 4: Run the tests** — `go test ./internal/config/` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): read ~/.config/launchdeck/config.json (lang)"
```

---

## Task 3: Migrate the pure app seam (`internal/app`)

Swap every in-scope English literal in `reduce.go` and `derive.go` for a `T`/`Tf` call, localizing the action verb where it is interpolated. The existing `reduce_test.go`/`derive_test.go` run at default `En` and must stay green unchanged.

**Files:**
- Modify: `internal/app/reduce.go`
- Modify: `internal/app/derive.go`
- Test: `internal/app/i18n_render_test.go` (new)

**Interfaces:**
- Consumes: `i18n.T`, `i18n.Tf` (Task 1).
- Produces: a package-local helper `verb(a launchctl.ActionKind) string` returning the localized verb.

- [ ] **Step 1: Write the failing RU smoke test** — `internal/app/i18n_render_test.go`:

```go
package app

import (
	"testing"

	"github.com/volkoffskij/launchdeck/internal/i18n"
	"github.com/volkoffskij/launchdeck/internal/launchctl"
)

func TestDeriveRussianPlaceholder(t *testing.T) {
	i18n.SetLang(i18n.Ru)
	t.Cleanup(func() { i18n.SetLang(i18n.En) })

	s := NewState(501) // FirstScanDone == false → loading placeholder
	if got := Derive(s).List.Placeholder; got != "Загрузка сервисов…" {
		t.Errorf("ru placeholder = %q", got)
	}
}

func TestReduceRussianActionOK(t *testing.T) {
	i18n.SetLang(i18n.Ru)
	t.Cleanup(func() { i18n.SetLang(i18n.En) })

	s := NewState(501)
	s.ActionRunning = true
	out := Reduce(ActionResult{
		Action:  launchctl.ActionRestart,
		Outcome: launchctl.ActionOutcome{Code: 0}, // OK() true
	}, s)
	if out.StatusMsg != "перезапуск: ок" {
		t.Errorf("ru action ok = %q, want %q", out.StatusMsg, "перезапуск: ок")
	}
}
```

> Note for the implementer: `NewState`, `ActionResult`, and `ActionOutcome.OK()` already exist — confirm the exact `ActionOutcome` zero-value that makes `OK()` true (Code 0) by reading `internal/launchctl`. Adjust the literal in the test if the constructor differs, but keep the RU assertion.

- [ ] **Step 2: Run it to verify it fails** — `go test ./internal/app/ -run Russian` → FAIL (still English).

- [ ] **Step 3: Add the verb helper** to `internal/app/reduce.go` (near the top, after imports):

```go
// verb returns the current-language word for an action (e.g. "restart" /
// "перезапуск"), used inside the localized status and prompt formats.
func verb(a launchctl.ActionKind) string {
	return i18n.T("action.verb." + a.String())
}
```

Add `"github.com/volkoffskij/launchdeck/internal/i18n"` to the `reduce.go` imports.

- [ ] **Step 4: Rewrite the `reduce.go` literals.** Apply exactly these replacements (left = current line, right = new expression):

| Current | Replacement |
|---|---|
| `s.StatusMsg = "cannot infer domain (path is under neither LaunchAgents nor LaunchDaemons)"` | `s.StatusMsg = i18n.T("status.infer_domain")` |
| `s.StatusMsg = "load…"` | `s.StatusMsg = i18n.T("status.load")` |
| `s.StatusMsg = msg.Action.String() + " timed out"` | `s.StatusMsg = i18n.Tf("status.timeout", verb(msg.Action))` |
| `s.StatusMsg = msg.Action.String() + " ok"` | `s.StatusMsg = i18n.Tf("status.ok", verb(msg.Action))` |
| `s.StatusMsg = "load failed (permission denied) — system daemons must be loaded with elevated privileges"` | `s.StatusMsg = i18n.T("status.load_perm")` |
| `s.StatusMsg = msg.Action.String() + " needs sudo — Retry with sudo"` | `s.StatusMsg = i18n.Tf("status.needs_sudo", verb(msg.Action))` |
| `s.StatusMsg = msg.Action.String() + " failed: " + msg.Outcome.Stderr` | `s.StatusMsg = i18n.Tf("status.failed", verb(msg.Action), msg.Outcome.Stderr)` |
| `s.Detail.ErrMsg = "requires sudo to inspect — run launchdeck with sudo to view system services"` | `s.Detail.ErrMsg = i18n.T("detail.err_sudo")` |
| `s.StatusMsg = "action already running"` | `s.StatusMsg = i18n.T("status.busy")` |
| `s.StatusMsg = "system services need root to enumerate (run launchdeck with sudo to see them)"` | `s.StatusMsg = i18n.T("status.enum_root")` |
| `s.StatusMsg = "failed to parse services"` | `s.StatusMsg = i18n.T("status.parse_fail")` |

- [ ] **Step 5: Rewrite the `derive.go` literals.** Add `"github.com/volkoffskij/launchdeck/internal/i18n"` to imports, then:

| Current | Replacement |
|---|---|
| `return ListVM{Placeholder: "Loading services…"}` | `return ListVM{Placeholder: i18n.T("list.loading")}` |
| `return ListVM{Placeholder: "No matching services"}` | `return ListVM{Placeholder: i18n.T("list.empty")}` |
| `d.RunState = "running"` | `d.RunState = i18n.T("runstate.running")` |
| `d.RunState = "stopped"` | `d.RunState = i18n.T("runstate.stopped")` |
| `return "enabled"` (in `enableStr`) | `return i18n.T("enable.enabled")` |
| `return "disabled"` (in `enableStr`) | `return i18n.T("enable.disabled")` |
| `return nil, "no log configured"` | `return nil, i18n.T("log.none")` |
| `st.Prompt = fmt.Sprintf("%s %s? (y/n)", s.PendingConfirm.Action, labelOf(s.PendingConfirm.Target))` | `st.Prompt = i18n.Tf("prompt.confirm", verb(s.PendingConfirm.Action), labelOf(s.PendingConfirm.Target))` |
| `st.Prompt = "Retry with sudo? (y/n)"` | `st.Prompt = i18n.T("prompt.sudo")` |
| `st.Prompt = "filter: " + s.FilterBuffer` | `st.Prompt = i18n.T("prompt.filter") + s.FilterBuffer` |
| `st.Prompt = "load plist: " + s.LoadPrompt.Buffer` | `st.Prompt = i18n.T("prompt.load") + s.LoadPrompt.Buffer` |
| `st.Prompt = "action: " + s.ActionPicker.HighlightedVerb.String() + " (s/r/k/e/d/u, Enter, Esc)"` | `st.Prompt = i18n.Tf("prompt.action", verb(s.ActionPicker.HighlightedVerb))` |

Leave unchanged: `Buttons: []string{"Start", ...}` (stable identifiers — localized at the statusbar in Task 4), the `enableStr` `"?"` default, `Mode`/`RunState` switch tags used internally, and the `"[" + l.Stream + "] "` log prefix.

> `s.PendingConfirm.Action` and `s.ActionPicker.HighlightedVerb` are `launchctl.ActionKind` (confirm the types by reading `state.go`); `verb(...)` takes an `ActionKind`. If either is already a string, wrap via the matching `ActionKind` or add a string overload — but do not change the state types.

- [ ] **Step 6: Run tests** — `go test ./internal/app/` → PASS. Existing English assertions still hold (default `En` reproduces the old strings byte-for-byte); the two new RU tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/app/
git commit -m "feat(i18n): localize app-seam status, prompts, and state words"
```

---

## Task 4: Migrate the UI layer (`internal/ui/bubbletea`)

Swap the chrome literals for `T`/`Tf`, make the metadata block align dynamically (Cyrillic labels differ in width), keep zone-ids/`buttonKey` English, and add golden-English + RU-layout tests.

**Files:**
- Modify: `internal/ui/bubbletea/view.go`
- Modify: `internal/ui/bubbletea/detail.go`
- Modify: `internal/ui/bubbletea/statusbar.go`
- Test: `internal/ui/bubbletea/render_test.go`

**Interfaces:**
- Consumes: `i18n.T`, `i18n.Tf`.

- [ ] **Step 1: Add golden-English + RU-layout tests** to `render_test.go`. (Import `strings`, `unicode/utf8`, and the `i18n` package if not already imported.)

```go
func TestHelpOverlayEnglishGolden(t *testing.T) {
	i18n.SetLang(i18n.En)
	t.Cleanup(func() { i18n.SetLang(i18n.En) })
	m := New(app.NewState(501), nil).WithSize(100, 40)
	m.helpOpen = true
	out := m.render()
	for _, want := range []string{
		"LaunchDeck — help",
		"Navigation",
		"switch focus: sidebar ↔ detail",
		"press ? or Esc to close",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("help overlay missing %q", want)
		}
	}
}

func TestRussianRenderNoOverflow(t *testing.T) {
	i18n.SetLang(i18n.Ru)
	t.Cleanup(func() { i18n.SetLang(i18n.En) })
	const w, h = 60, 20 // the documented minimum
	m := New(app.NewState(501), nil).WithSize(w, h)
	out := m.render()
	lines := strings.Split(out, "\n")
	if len(lines) > h {
		t.Fatalf("frame has %d lines, want ≤ %d", len(lines), h)
	}
	for i, l := range lines {
		if lipgloss.Width(l) > w {
			t.Errorf("line %d width %d > %d: %q", i, lipgloss.Width(l), w, l)
		}
	}
}
```

> Adjust the model constructor/size setter to whatever `render_test.go` already uses (e.g. an existing `newTestModel`/`WithSize` helper). The assertions — help-overlay English strings present, and no RU line exceeds the frame — are the contract; match the file's existing harness style. `New(...)`'s second arg is the launchctl client; pass whatever the existing tests pass (likely `nil` or a fake).

- [ ] **Step 2: Run them to verify they fail** — `go test ./internal/ui/bubbletea/ -run 'English|Russian'`. Golden may pass already (English unchanged); the RU-overflow test may fail if metadata padding or another region overflows. Proceed to implement so both pass.

- [ ] **Step 3: Migrate `view.go`.** Add the `i18n` import. Replace:

- Line ~20 (too-small): `"terminal too small (need ≥60×20)"` → `i18n.T("view.too_small")`.
- `renderHeader` body: `" LaunchDeck — launchctl services · ? help"` → `i18n.T("header.title")`.
- `renderHelp` `title`: `th.accent().Bold(true).Render("LaunchDeck — help")` → `...Render(i18n.T("help.title"))`.
- The `body := strings.Join([]string{ ... }, "\n")` slice becomes (keeping the `sec` styling and blank lines):

```go
body := strings.Join([]string{
	title,
	"",
	sec(i18n.T("help.nav")),
	i18n.T("help.nav.move"),
	i18n.T("help.nav.homeend"),
	i18n.T("help.nav.page"),
	i18n.T("help.nav.tab"),
	i18n.T("help.nav.tabs"),
	i18n.T("help.nav.scroll"),
	"",
	sec(i18n.T("help.actions")) + i18n.T("help.actions.suffix"),
	i18n.T("help.actions.picker1"),
	i18n.T("help.actions.picker2"),
	i18n.T("help.actions.confirm"),
	i18n.T("help.actions.load"),
	"",
	sec(i18n.T("help.view")),
	i18n.T("help.view.filter"),
	i18n.T("help.view.sort"),
	i18n.T("help.view.mouse"),
	i18n.T("help.view.help"),
	"",
	sec(i18n.T("help.mouse")) + i18n.T("help.mouse.desc"),
	"",
	th.muted().Render(i18n.T("help.footer")),
}, "\n")
```

- [ ] **Step 4: Migrate `detail.go`.** Add the `i18n` and `unicode/utf8` imports. Replace:

- Line ~33: `th.muted().Render("Select a service")` → `th.muted().Render(i18n.T("detail.select"))`.
- Line ~94: `body = "Loading detail…"` → `body = i18n.T("detail.loading")`.
- Line ~119: `body = "(gone) — service no longer present\n\n" + body` → `body = i18n.T("detail.gone") + "\n\n" + body`.
- Replace the hard-coded metadata block (lines ~98-107) with a dynamically aligned build:

```go
default:
	body = metaBlock([][2]string{
		{i18n.T("meta.label"), vm.Label},
		{i18n.T("meta.domain"), vm.Domain},
		{i18n.T("meta.pid"), vm.PID},
		{i18n.T("meta.exit"), vm.LastExit},
		{i18n.T("meta.run"), vm.RunState},
		{i18n.T("meta.enable"), vm.EnableState},
		{i18n.T("meta.program"), vm.Program},
		{i18n.T("meta.plist"), vm.Plist},
	})
}
```

Add the helper (rune-aware padding — Cyrillic runes are single width, so rune count equals display cells):

```go
// metaBlock renders "Label:  value" rows with the colon-suffixed labels padded
// to a common width so the values align, regardless of language. Two spaces
// separate the widest label from its value.
func metaBlock(rows [][2]string) string {
	max := 0
	for _, r := range rows {
		if n := utf8.RuneCountInString(r[0]); n > max {
			max = n
		}
	}
	lines := make([]string, len(rows))
	for i, r := range rows {
		pad := max - utf8.RuneCountInString(r[0])
		lines[i] = r[0] + ":" + strings.Repeat(" ", pad+2) + r[1]
	}
	return strings.Join(lines, "\n")
}
```

> This changes the exact English spacing (old: `"Label:     "` = colon + 5 spaces to a fixed 11-col field; new: labels padded to the widest label `"Last exit"` (9) + colon + 2 = value column at 12). The English metadata output shifts by a space or two. If `render_test.go` has an assertion pinning the old `"Label:     "` spacing, update that assertion to the new alignment (the values still align in a column — that is the contract, not the exact space count).

- Migrate `renderTabs` (line ~194) — keep the English zone id, localize the display:

```go
func (m Model) renderTabs(active app.Tab) string {
	names := []string{"Metadata", "Logs", "Raw"} // stable zone ids
	var out []string
	for i, n := range names {
		s := " " + i18n.T("tab."+n) + " "
		if app.Tab(i) == active {
			s = m.theme.tabActive().Render(s)
		} else {
			s = m.theme.muted().Render(s)
		}
		out = append(out, zone.Mark("tab:"+n, s))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, out...)
}
```

- [ ] **Step 5: Migrate `statusbar.go`.** Add the `i18n` import. Replace the
button label build (line ~46): keep the English `b` for `buttonKey[b]` and the
zone id, localize only the visible label:

```go
for _, b := range vm.Buttons {
	label := "[" + th.accent().Render(buttonKey[b]) + " " + i18n.T("btn."+b) + "]"
	btns = append(btns, zone.Mark("btn:"+b, label))
}
```

- [ ] **Step 6: Run the suite** — `go test ./internal/ui/bubbletea/` → PASS (update any old fixed-spacing metadata assertion per Step 4). Then `go test ./...` → all green.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/bubbletea/
git commit -m "feat(i18n): localize UI chrome, dynamic metadata alignment, RU layout test"
```

---

## Task 5: Startup wiring + docs

Select the language at startup and document the config file.

**Files:**
- Modify: `cmd/launchdeck/main.go`
- Modify: `README.md`
- Test: `cmd/launchdeck/main_test.go` (help-text assertion)

**Interfaces:**
- Consumes: `config.Path`, `config.Load`, `i18n.Detect`, `i18n.SetLang`.

- [ ] **Step 1: Update the help-text test** in `main_test.go`. Find `TestHelpText`; add an assertion that the help text now mentions the config file:

```go
if !strings.Contains(got, "config.json") {
	t.Errorf("help text should mention config.json")
}
```

- [ ] **Step 2: Run it to verify it fails** — `go test ./cmd/launchdeck/ -run TestHelpText` → FAIL.

- [ ] **Step 3: Wire startup** in `startTUI` (`cmd/launchdeck/main.go`), right after the `launchctl` LookPath guard and before `uid := os.Getuid()`:

```go
cfg := config.Config{}
if p, err := config.Path(); err == nil {
	cfg = config.Load(p)
}
i18n.SetLang(i18n.Detect(os.Getenv, cfg.Lang))
```

Add imports `"github.com/volkoffskij/launchdeck/internal/config"` and `"github.com/volkoffskij/launchdeck/internal/i18n"`.

- [ ] **Step 4: Add the `config.json` line to `helpText()`.** In the `helpText` function, in the block that lists `~/.config/launchdeck/session.json` and `theme.json`, add a matching line:

```
  ~/.config/launchdeck/config.json    language ("lang": "ru" | "en"; auto if absent)
```

Match the surrounding alignment of the existing two lines exactly.

- [ ] **Step 5: Run the tests** — `go test ./cmd/launchdeck/` → PASS. Then `go build ./cmd/launchdeck` → builds.

- [ ] **Step 6: Update `README.md`.** After the "Colors (theme)" section, add a short "Language" section:

```markdown
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
```

- [ ] **Step 7: Run the full suite + build** — `go test ./...` → all green; `gofmt -l .` → empty; `go vet ./...` → clean; `go build ./cmd/launchdeck` → ok.

- [ ] **Step 8: Commit**

```bash
git add cmd/launchdeck/main.go cmd/launchdeck/main_test.go README.md
git commit -m "feat(i18n): select language at startup from config + env; document it"
```

---

## Self-review notes

- **Spec coverage:** i18n package (T1), config (T2), app-seam migration incl. verb localization (T3), UI chrome + dynamic metadata alignment + RU layout test (T4), startup wiring + docs (T5). Layout requirement met by `metaBlock` dynamic alignment + `TestRussianRenderNoOverflow`. Completeness met by `TestCatalogComplete`. English invariant met by default-`En` + golden asserts.
- **Verb localization (review Fix 4):** `verb()` helper feeds every `status.*`/`prompt.*` format, so RU yields "перезапуск: ок", not "restart ок".
- **Global-state leakage (review Fix 6):** every `SetLang(Ru)` test uses `t.Cleanup(SetLang(En))`.
- **The `metaBlock` change shifts English metadata spacing** — the one place an existing assertion may need updating; called out in T4 Step 4.
