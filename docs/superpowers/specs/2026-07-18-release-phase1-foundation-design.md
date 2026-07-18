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
decisions already made: **License = GPL-3.0**; **Distribution = GitHub Releases
+ `go install`** (no Homebrew); **i18n = auto from `$LANG` + config override,
EN + RU**.

## Goals (Phase 1)

Make the repository legally and operationally publishable:

1. A GPL-3.0 `LICENSE` so the code can be used and forked under known terms.
2. Repo hygiene: IDE cruft and the built binary out of version control.
3. `--version` and `--help` so the binary is inspectable without launching the TUI.
4. Panic recovery so a crash prints a clear, reportable message and exits cleanly.

## Non-Goals

- CI, i18n, packaging, docs polish — later phases.
- Per-file SPDX headers across the whole tree (only `main.go` gets one — YAGNI).
- A CLI framework (cobra/urfave). Two flags need only the stdlib `flag` package.
- Homebrew, code signing, notarization.

## Design

### 1. License (GPL-3.0)

- Add `LICENSE` at the repo root containing the full, verbatim GPL-3.0 text
  (the canonical text from https://www.gnu.org/licenses/gpl-3.0.txt).
- Add one SPDX identifier line to `cmd/launchdeck/main.go`:
  `// SPDX-License-Identifier: GPL-3.0-or-later` (first line, above `package main`).
- README gets a short **License** section: "GPL-3.0-or-later — see `LICENSE`."

### 2. Repo hygiene

- `.gitignore` gains two entries: `.idea/` and `/launchdeck` (the compiled
  binary, which currently sits in the working tree and must never be committed).
- `git rm -r --cached .idea` removes the already-tracked JetBrains project files
  from version control while leaving them on disk. Confirm afterward that
  `git ls-files` lists no `.idea/` paths and no `launchdeck` binary.

### 3. CLI flags (`cmd/launchdeck/main.go`, stdlib `flag`)

- **Version string:** package-level `var version = "dev"`, overridable at build
  time via `-ldflags "-X main.version=<tag>"` (goreleaser sets it in Phase 4).
  When `version == "dev"`, enrich it from `runtime/debug.ReadBuildInfo()` — use
  the `vcs.revision` build setting (short) and `vcs.modified` flag if present,
  so a plain `go build`/`go install` still reports a useful commit. Format:
  `launchdeck <version>` for a release build, or `launchdeck dev (<rev>[-dirty])`
  for a source build with VCS info, or `launchdeck dev` when no build info.
- **`--version` / `-v`:** print the version line to stdout, exit 0.
- **`--help` / `-h`:** print to stdout:
  - one-line description,
  - `Usage: launchdeck [flags]` and the flag list,
  - the config file paths (`~/.config/launchdeck/session.json`,
    `~/.config/launchdeck/theme.json`),
  - a pointer: "Press ? inside the app for the full keymap."
  Exit 0.
- **Flag parsing runs first** in `main`, before the `runtime.GOOS == "darwin"`
  and `launchctl`-in-PATH checks, so `--version`/`--help` succeed on any OS and
  without launchctl. The darwin/launchctl guards run only when actually starting
  the TUI.

### 4. Panic recovery (`main.go`)

- Wrap the TUI startup path in a deferred `recover()` in `main` (or a `run()`
  helper `main` calls). On a non-nil recovered value:
  - write to stderr: `launchdeck crashed: <value>` followed by
    `please report: https://github.com/volkoffskij/launchdeck/issues`,
  - exit with status 1.
- Terminal restoration (leaving alt-screen, showing the cursor, disabling mouse
  reporting) is handled by Bubble Tea's own panic handling, which restores the
  terminal before re-panicking; our `recover()` only formats the message and
  sets the exit code. (If a manual smoke test shows the terminal left in a bad
  state after a forced panic, revisit — but do not pre-build terminal-reset
  escape output speculatively.)
- Factor the message formatting into a small pure helper,
  `crashMessage(v any) string`, so it is unit-testable without triggering a real
  panic or touching os.Exit.

## Error Handling

- `flag` parse error (unknown flag) → `flag` prints usage to stderr and the
  program exits non-zero (stdlib default); acceptable.
- All existing startup errors (non-darwin, missing launchctl, program run error)
  keep their current messages and exit codes.

## Testing

- **Unit:** `crashMessage(v)` — asserts the formatted string contains the panic
  value and the issues URL.
- **Unit/smoke:** version formatting — a helper `versionString()` that assembles
  the version line from the `version` var + injected build-info values is
  table-tested (release build, dev+rev, dev+rev+dirty, dev bare).
- **Manual (binary):**
  - `launchdeck --version` prints the version line, exit 0.
  - `launchdeck --help` prints usage + config paths + the `?` hint, exit 0.
  - both work with a non-existent `launchctl` (parsing precedes the guard) —
    verify e.g. by temporarily shadowing PATH, or trust the code order + a unit
    test that the guard is not reached for these flags.
  - `git ls-files | grep -E '\.idea/|^launchdeck$'` returns nothing.
  - `LICENSE` exists and begins with the GPL-3.0 header.

## Success Criteria

1. `LICENSE` (GPL-3.0) present; README links it; `main.go` carries the SPDX line.
2. `.idea/` and the `launchdeck` binary are git-ignored and untracked.
3. `launchdeck --version` and `launchdeck --help` work and exit 0, independent of
   OS and launchctl availability.
4. A panic prints `launchdeck crashed: … / please report: …` to stderr and exits 1.
