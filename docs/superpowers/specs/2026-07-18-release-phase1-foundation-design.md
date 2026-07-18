# LaunchDeck Release — Phase 1: Foundation (Design Spec)

**Date:** 2026-07-18
**Status:** Approved (brainstorming), pending implementation plan

## Context

LaunchDeck is a working macOS `launchctl` TUI (see
`2026-07-16-launchdeck-design.md`). This spec is the first of a six-phase
open-source release effort. The release roadmap, in dependency order:

1. **Foundation** (this spec) — license, repo hygiene, CLI flags, panic recovery.
2. **CI** — GitHub Actions: build / test / vet / gofmt / lint on PR.
3. **i18n** — locale layer (EN + RU), auto-detected from `$LANG` with a config override.
4. **Distribution** — goreleaser + GitHub Releases binaries, `go install`.
5. **Docs** — README (EN polish + RU), screenshot/GIF, CHANGELOG, CONTRIBUTING.
6. **Polish** — remaining nice-to-haves (asciinema, security scan).

Each phase is its own spec → plan → implementation cycle. Cross-cutting
decisions already made: **License = GPL-3.0-or-later**; **Distribution = GitHub Releases
+ `go install`** (no Homebrew); **i18n = auto from `$LANG` + config override,
EN + RU**.

## Goals (Phase 1)

Make the repository legally and operationally publishable:

1. A GPL-3.0-or-later `LICENSE` so the code can be used and forked under known terms.
2. Repo hygiene: IDE cruft and the built binary out of version control.
3. `--version` and `--help` so the binary is inspectable without launching the TUI.
4. Panic recovery so a crash prints a clear, reportable message and exits cleanly.

## Non-Goals

- CI, i18n, packaging, docs polish — later phases. **Exception:** the README
  **License section** (§1) is a Phase-1 deliverable; all other README work stays
  in Phase 5.
- Per-file SPDX headers across the whole tree (only `main.go` gets one — YAGNI).
- A CLI framework (cobra/urfave). Two flags need only the stdlib `flag` package.
- Homebrew, code signing, notarization.

## Design

### 1. License (GPL-3.0-or-later)

- Add `LICENSE` at the repo root containing the full, verbatim GPL-3.0 text
  (the canonical text from https://www.gnu.org/licenses/gpl-3.0.txt). The
  `gpl-3.0.txt` body is byte-identical for the "only" and "or-later" grants; the
  grant is expressed by the notice that accompanies it (below), which uses the
  **or-later** wording.
- The rights-holder is the GitHub owner **volkoffskij**. Add one SPDX identifier
  line plus a copyright notice to `cmd/launchdeck/main.go` (first lines, above
  `package main`):
  `// SPDX-License-Identifier: GPL-3.0-or-later`
  `// Copyright (C) 2026 volkoffskij`
- A README already exists at the repo root (`README.md`); Phase 1 adds **only** a
  short **License** section (no other README changes — those are Phase 5):
  "Copyright (C) 2026 volkoffskij. GPL-3.0-or-later — see `LICENSE`."

### 2. Repo hygiene

- `.gitignore` gains two entries: `.idea/` and `/launchdeck` (the compiled
  binary, which currently sits in the working tree and must never be committed).
- `git rm -r --cached --ignore-unmatch .idea` untracks any JetBrains project
  files while leaving them on disk. Verified against the current repo: nothing
  under `.idea/` is tracked and the `launchdeck` binary is not tracked either, so
  this is expected to be a no-op — `--ignore-unmatch` keeps it from exiting
  non-zero and aborting the step. The binary needs no `git rm` (already
  untracked); only the `.gitignore` entry is required. Confirm afterward that
  `git ls-files` lists no `.idea/` paths and no `launchdeck` binary.

### 3. CLI flags (`cmd/launchdeck/main.go`, stdlib `flag`)

- **Version string (pure helper `versionString()`):** package-level
  `var version = "dev"`, overridable at build time via
  `-ldflags "-X main.version=<tag>"` (goreleaser sets it in Phase 4). The pure
  helper `versionString()` assembles the version line from the `version` var plus
  injected build-info values. When `version == "dev"`, it enriches the line from
  `runtime/debug.ReadBuildInfo()` in this order:
  1. If `info.Main.Version` is non-empty and not `"(devel)"` (the module-proxy
     channel: `go install <module>@<ver>`, enabled in Phase 4), use it as the
     version — output `launchdeck <Main.Version>`.
  2. Else if the `vcs.revision` build setting is present and non-empty, truncate
     it length-safely to at most 12 chars (`rev[:min(len(rev), 12)]`, so a
     revision shorter than 12 chars cannot panic with slice-out-of-range) and
     append `-dirty` when `vcs.modified == "true"` — output
     `launchdeck dev (<rev12>[-dirty])`.
  3. Else (no build info, or build info present but no `vcs.revision` and no
     usable `Main.Version`) — output `launchdeck dev`.
- **`--version` / `-v`:** print the version line to stdout; then `run()` returns
  (no `os.Exit`) and `main` maps that normal return to exit 0.
- **`--help` / `-h`:** print to stdout:
  - one-line description,
  - `Usage: launchdeck [flags]` and the flag list,
  - the config file paths (`~/.config/launchdeck/session.json`,
    `~/.config/launchdeck/theme.json`),
  - a pointer: "Press ? inside the app for the full keymap."
  Then `run()` returns (no `os.Exit`) and `main` maps that normal return to
  exit 0.
- **Precedence & unexpected args:** `--version` and `--help` are independent bool
  flags, so both can be set at once. Precedence: **`--help` wins** — if `--help`
  is set (with or without `--version`), print the help text and return exit 0.
  Unexpected positional args (e.g. `launchdeck foo`) are **ignored, not an
  error**: they land in `fs.Args()`, the info-flag guards do not fire, and the
  normal no-flag path (darwin/launchctl guards → TUI) proceeds. This
  deliberately-tolerant behavior is pinned by the guard-seam test rows below.
- **Flag definitions (stdlib-trap avoidance):** `-v`/`--version` and
  `-h`/`--help` are *explicitly* registered bool flags. Because `-h`/`--help`
  are defined, stdlib's auto-help `ErrHelp`/exit-2 path never fires. Parse with a
  `flag.FlagSet` in `flag.ContinueOnError` mode (not `ExitOnError`) so `run()`
  owns every exit code. The help text is built by a pure `helpText() string`
  helper (so its content is unit-testable) and **hand-written to stdout** — not
  `flag.PrintDefaults()`, which writes to stderr and enumerates the
  `-h/-help/-v/-version` variants with default-value noise.
- **Flag parsing runs first** in `run()`, before the `runtime.GOOS == "darwin"`
  and `launchctl`-in-PATH checks, so `--version`/`--help` succeed on any OS and
  without launchctl. When `--version`/`--help` is set, `run()` prints and
  **returns before reaching the guards** — this early return is the testable seam
  (a unit test can drive `run()` with those flags and assert the guards are not
  reached). The darwin/launchctl guards, and then `p.Run` (TUI startup), run only
  on the normal no-flag path.

### 4. Panic recovery (`main.go`)

- Put the guards and TUI startup in a `run() (code int)` helper that `main`
  calls as `os.Exit(run())`; the deferred `recover()` lives in `run()` (not bare
  in `main`). This keeps the recover on the same goroutine as the startup path
  and makes `run()` — not `os.Exit` — own the exit code, so a unit test can
  drive a panicking `run()` and assert the returned code. A normal return yields
  code 0 (covers the `--version`/`--help` and happy paths; `main` maps it to
  exit 0); the deferred recover sets the named `code` return to 1 on a non-nil
  recovered value. On a non-nil recovered value the recover:
  - writes the two lines from `crashMessage(<value>, <resolved version>)` to
    stderr — `<version> crashed: <value>` followed by
    `please report: https://github.com/volkoffskij/launchdeck/issues` — where
    `<version>` is the exact string §3's `versionString()` builds (e.g.
    `launchdeck v1.2.3`, which **already carries** the `launchdeck ` prefix, so
    the rendered line reads `launchdeck v1.2.3 crashed: <value>` — the prefix is
    **not** repeated by the template), so a pasted report identifies which build
    crashed,
  - sets the returned `code` to 1.
- **Scope:** `recover()` only catches panics that propagate on the main
  goroutine. Bubble Tea runs `tea.Cmd` functions on separate goroutines, so a
  panic inside a dispatched command unwinds that goroutine and never reaches this
  recover. Command/goroutine-level panic recovery is **out of scope for Phase 1**
  — this requirement covers main-goroutine panics only.
- Terminal restoration (leaving alt-screen, showing the cursor, disabling mouse
  reporting) is handled by Bubble Tea's own panic handling, which restores the
  terminal before re-panicking; our `recover()` only formats the message and
  sets the exit code. (If a manual smoke test shows the terminal left in a bad
  state after a forced panic, revisit — but do not pre-build terminal-reset
  escape output speculatively.)
- Factor the message formatting into a small pure helper,
  `crashMessage(v any, version string) string`, so it is unit-testable without
  triggering a real panic or touching os.Exit. It renders `v` with
  `fmt.Sprintf("%v", v)` and collapses any newlines in the rendered value to
  single spaces, so the message is **always exactly two lines** even when the
  panic value is non-string or multi-line (a wrapped/multi-line error). It emits
  only the two lines above (panic value + resolved version + issues URL); a
  `debug.Stack()` dump is
  **excluded** for Phase 1 — the reportable datum is the two-line message plus
  the build version, and keeping `crashMessage` pure/deterministic makes it
  exactly assertable; add a stack dump later only if triage proves the version +
  value insufficient.

## Error Handling

- `flag` parse error (unknown flag) → the `FlagSet` is created with
  `fs.SetOutput(os.Stderr)` and a **custom `fs.Usage`** that prints a single-line
  `Usage: launchdeck [flags] (run --help for details)` hint — **not**
  `flag.PrintDefaults()`, so the `-h/-help/-v/-version` default-value noise §3
  rejects is never emitted on this path either. On an unknown flag, `Parse` writes
  its own error message plus that one-line hint to stderr and returns the error to
  `run()`, which returns exit code **2** (stdlib convention for CLI usage errors)
  that `main` passes to `os.Exit`.
- All existing startup errors (non-darwin, missing launchctl, program run error)
  keep their current messages and exit codes.

## Testing

- **Unit:** `crashMessage(v, version)` — asserts the formatted string is exactly
  two lines. Rows (each asserts the full exact string):
  - `crashMessage("boom", "launchdeck v1.2.3")` → exactly
    `launchdeck v1.2.3 crashed: boom` then a newline then
    `please report: https://github.com/volkoffskij/launchdeck/issues`
  - non-string value: `crashMessage(42, "launchdeck dev")` →
    `launchdeck dev crashed: 42` + the report line (value rendered via `%v`).
  - multi-line value: a panic value whose `%v` contains `\n` → the newlines are
    collapsed to single spaces so the output stays exactly two lines.
- **Unit/smoke:** version formatting — a helper `versionString()` that assembles
  the version line from the `version` var + injected build-info values is
  table-tested. Because truncation and the `-dirty` trigger are pinned, each row
  asserts an exact string. Rows:
  - release build (`version="v1.2.3"`) → `launchdeck v1.2.3`
  - dev + `Main.Version="v1.2.3"` (module-proxy install) → `launchdeck v1.2.3`
  - dev + `vcs.revision` (40-char SHA), clean → `launchdeck dev (<first 12>)`
  - dev + `vcs.revision`, `vcs.modified="true"` → `launchdeck dev (<first 12>-dirty)`
  - dev + short `vcs.revision` (e.g. `"abc"`), clean → `launchdeck dev (abc)` (no slice panic)
  - dev + build info present but no `vcs.revision` and `Main.Version` empty/`(devel)` → `launchdeck dev`
  - dev + no build info → `launchdeck dev`
- **Unit:** guard seam — drive `run()` with `--version`, with `--help`, and with
  **both `--version` and `--help` set at once** (expect `--help` output per the
  precedence rule), asserting each returns code 0 before the darwin/launchctl
  guards (e.g. via an injectable guard flag/counter that stays unreached),
  confirming these flags never touch the guards or `p.Run`.
- **Unit:** bad-flag path — drive `run()` with an unknown flag (e.g. `--nope`),
  assert it returns code **2**, and assert captured stderr does **not** contain
  the `-help`/`-version` default-value dump (confirming `flag.PrintDefaults()` was
  not used).
- **Unit:** `helpText()` — assert the returned string contains the
  `Usage: launchdeck [flags]` line, **both** config paths
  (`~/.config/launchdeck/session.json` and `~/.config/launchdeck/theme.json`),
  and the `Press ?` hint.
- **Integration:** drive `run()` on a path forced to panic on the main goroutine
  (a build-tagged or injectable debug trigger) with stderr captured; assert the
  returned code is 1 and captured stderr contains both the
  `<version> crashed:` line (with `<version>` = `versionString()`, e.g.
  `launchdeck v1.2.3 crashed:`) and the `please report:` line. This is the
  end-to-end check that the defer/recover wiring, stderr stream, and exit code
  actually work (Success Criterion #4), not just the pure helper.
- **Manual (binary):**
  - `launchdeck --version` prints the version line to stdout, exit 0.
  - `launchdeck --help` prints usage + config paths + the `?` hint to **stdout**
    (verify with `launchdeck --help 1>out 2>err`: `out` non-empty, `err` empty),
    exit 0.
  - both work with a non-existent `launchctl` (parsing precedes the guard) —
    verify e.g. by temporarily shadowing PATH (the unit guard-seam test above
    covers "guard not reached").
  - on macOS, `launchdeck` with no flags still launches the TUI (happy path
    unchanged by the `run()` restructure).
  - **Forced-panic smoke test:** with a temporary debug trigger, run the built
    binary so a real main-goroutine panic fires; confirm both crash lines print
    to stderr, exit status is 1, and the terminal is restored (out of alt-screen,
    cursor visible, mouse reporting off).
  - `git ls-files | grep -E '\.idea/|^launchdeck$'` returns nothing.
  - `git check-ignore -q .idea/ && git check-ignore -q launchdeck` — both succeed
    (exit 0), confirming the `.gitignore` rules actually match, independent of
    tracking state.
  - `LICENSE` is **byte-identical** to the canonical
    https://www.gnu.org/licenses/gpl-3.0.txt — verify by checksum/byte-compare
    (e.g. `diff LICENSE gpl-3.0.txt` or `shasum LICENSE` against the canonical
    hash), **not** just a header-prefix match, so a truncated `LICENSE` fails.
  - `head -n 5 cmd/launchdeck/main.go` shows, above `package main`, the exact
    lines `// SPDX-License-Identifier: GPL-3.0-or-later` and
    `// Copyright (C) 2026 volkoffskij`.
  - README (`README.md`) has a **License** section that references/links `LICENSE`
    and states the copyright holder — e.g. `grep -n 'LICENSE' README.md` matches
    and the License heading is present.

## Success Criteria

1. `LICENSE` (GPL-3.0-or-later) present; README links it and states the copyright
   holder; `main.go` carries the SPDX line and copyright notice.
2. `.idea/` and the `launchdeck` binary are git-ignored and untracked.
3. `launchdeck --version` and `launchdeck --help` print to stdout and exit 0,
   independent of OS and launchctl availability.
4. A panic **propagating to the main goroutine** prints
   `<version> crashed: … / please report: …` (where `<version>` is
   `versionString()`, e.g. `launchdeck v1.2.3`, so the build version is included)
   to stderr and exits 1 — exercised end-to-end by the integration test and the
   forced-panic smoke test, not just the pure `crashMessage` unit test. Panics on
   Bubble Tea command goroutines are out of scope for Phase 1.
