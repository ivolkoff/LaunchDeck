# Contributing to LaunchDeck

Thanks for your interest. LaunchDeck is a small, focused macOS TUI for
`launchctl`. This document covers how to build, test, and extend it.

## Prerequisites

- macOS (the app shells out to the real `launchctl`).
- Go 1.24+.

## Build, run, test

```bash
go build -o launchdeck ./cmd/launchdeck   # build
./launchdeck                              # run
go test ./...                             # all tests
gofmt -l .                                # must print nothing
go vet ./...                              # must be clean
```

CI (`.github/workflows/ci.yml`) runs `gofmt`, `go vet`, `go build`, and
`go test` on `macos-latest` for every push and pull request. Keep it green.

## Architecture

The code separates a pure decision core from the terminal rendering so the UI
is swappable and the logic is testable without a TTY.

- **`internal/launchctl`** — the launchctl client and parsers. Everything that
  shells out to `launchctl` and turns its text output into typed values lives
  here.
- **`internal/app`** — the pure application seam. `Reduce(Msg, State) → State`
  and `Derive(State) → ViewModel` are pure functions: no I/O, no time, no
  globals. This is where behavior is defined and where most tests live.
- **`internal/ui/bubbletea`** — the terminal adapter, built on
  [Bubble Tea](https://github.com/charmbracelet/bubbletea),
  [Lip Gloss](https://github.com/charmbracelet/lipgloss), and
  [bubblezone](https://github.com/lrstanley/bubblezone). It renders the
  `ViewModel` and translates key/mouse events into `app.Msg`s. Because the seam
  is pure, this layer stays thin and could be replaced by another renderer.
- **`internal/i18n`** — the in-binary EN/RU message catalog (`T`/`Tf`) and the
  startup language detection.
- **`internal/config`** and **`internal/session`** — the optional
  `~/.config/launchdeck/config.json` and the restored UI session.
- **`cmd/launchdeck`** — the entry point: flag parsing (`--help`/`--version`),
  language selection, and starting the TUI.

## Testing the TUI

The pure seam (`internal/app`) is covered by ordinary unit tests. The renderer
is verified by driving a real `Model` through `Update`/`View` at fixed terminal
sizes and asserting on the rendered grid — see
`internal/ui/bubbletea/render_test.go`. The load-bearing guarantee is that the
frame never overflows the terminal (`TestViewNeverOverflows`,
`TestClampFrameHardBounds`); keep those green.

## Adding or improving a translation

All user-facing TUI strings go through `i18n.T`/`i18n.Tf`, keyed by a
dot-namespaced string. Translations live in one place:
`internal/i18n/catalog.go`, each entry holding the English and Russian text.

- To fix or improve a string, edit its `en`/`ru` values in the catalog.
- The **English value must stay byte-for-byte identical to what the code
  expects** — the existing tests assert English output at the default language.
- Every entry must have a non-empty `en` and `ru`; `TestCatalogComplete` fails
  otherwise.
- CLI `--help`/`--version` output is intentionally English-only.

Adding a third language would mean extending the catalog entry shape and
`i18n.parse`/`Detect`; open an issue first so we can agree on the approach.

## Commits and pull requests

- Use [Conventional Commits](https://www.conventionalcommits.org/) — `feat:`,
  `fix:`, `docs:`, `ci:`, `chore:`, etc. Release notes are generated from them.
- Keep changes focused; every changed line should trace to the stated goal.
- Run `go test ./...`, `gofmt -l .`, and `go vet ./...` before pushing.

## Releases

Releases are cut by pushing a `vX.Y.Z` tag. GoReleaser
(`.github/workflows/release.yml`) then builds the macOS binaries and publishes a
GitHub Release. Maintainers only.

## License

By contributing you agree that your contributions are licensed under the
project's [GPL-3.0-or-later](LICENSE) license.
