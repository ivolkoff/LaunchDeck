# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project
follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html). Per-release
binaries and auto-generated notes also live on the
[Releases](https://github.com/ivolkoff/launchdeck/releases) page.

## [Unreleased]

## [0.1.0] - 2026-07-18

First public release.

### Added
- macOS terminal UI for `launchctl`: browse GUI (`gui/$UID`) and system
  (`system`) services in a sidebar, inspect Metadata / Logs / Raw `print`
  output, and run start / restart / stop / enable / disable / load / unload
  actions.
- Mouse support (off by default so a plain drag selects text like a text
  editor; press `m` to capture the mouse for clicks, wheel, and dragging the
  divider).
- Live regex filter on the sidebar (`f` or `/`), a sort key/direction toggle,
  and a domain-scope toggle (user ↔ user+system).
- Session restore: selection, filter, scope, sort, scroll, active tab, and
  sidebar width persist to `~/.config/launchdeck/session.json`.
- Live-updating Logs tab, colour-coded log streams, and an editor-style
  line-number gutter for the Logs and Raw tabs.
- Configurable colours via `~/.config/launchdeck/theme.json`.
- English and Russian localization, auto-selected from the environment
  (`$LC_ALL`/`$LANG`/`$LANGUAGE`) with a `~/.config/launchdeck/config.json`
  `lang` override.
- `sudo` retry flow for privileged system-domain actions.
- `--version` and `--help`; panic recovery on the main goroutine.
- Distribution via `go install` and prebuilt macOS (Intel and Apple Silicon)
  archives on GitHub Releases.

[Unreleased]: https://github.com/ivolkoff/launchdeck/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/ivolkoff/launchdeck/releases/tag/v0.1.0
