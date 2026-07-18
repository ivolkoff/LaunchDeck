# Release Phase 1 — Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the LaunchDeck repo legally and operationally publishable — GPL-3.0-or-later license, repo hygiene, `--version`/`--help`, and main-goroutine panic recovery.

**Architecture:** `cmd/launchdeck/main.go` is refactored so `main` becomes `os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, startTUI))`. `run` is a pure-ish, testable function: it defers a `recover()`, parses flags with a `flag.FlagSet` (ContinueOnError), handles `--version`/`--help` (returning before the darwin/launchctl guards), and otherwise calls the injected `start` (the real `startTUI`, which holds today's guards + TUI startup and returns an exit code). Three pure helpers — `buildVersion`, `helpText`, `crashMessage` — are table-tested.

**Tech Stack:** Go 1.24, stdlib `flag`, `runtime/debug` (build info). No new dependencies.

## Global Constraints

- **License:** GPL-3.0-or-later. Rights-holder: **volkoffskij**. Copyright year: **2026**.
- **`LICENSE`** must be byte-identical to the canonical https://www.gnu.org/licenses/gpl-3.0.txt.
- **SPDX + copyright** go only in `cmd/launchdeck/main.go` (first two lines, above `package main`): `// SPDX-License-Identifier: GPL-3.0-or-later` and `// Copyright (C) 2026 volkoffskij`. No per-file headers elsewhere.
- **Issues URL** (used verbatim in the crash message): `https://github.com/volkoffskij/launchdeck/issues`.
- **CLI:** stdlib `flag` only — no cobra/urfave. `-v`/`--version` and `-h`/`--help` are explicitly-registered bool flags. `--help` wins over `--version`. Unknown flag → exit code 2. `--version`/`--help` → stdout, exit 0. Never call `flag.PrintDefaults()` (dumps `-h/-v` noise).
- **Panic scope:** recover covers main-goroutine panics only; Bubble Tea command-goroutine panics are out of scope for Phase 1.
- **Version string format** (from `versionString()`, already prefixed `launchdeck `): release → `launchdeck <version>`; module install → `launchdeck <Main.Version>`; dev+VCS → `launchdeck dev (<rev12>[-dirty])`; else `launchdeck dev`.
- **Commits:** plain Conventional Commits, NO AI attribution.
- **README:** Phase 1 adds ONLY a License section; all other README work is Phase 5.

## File Structure

```
LICENSE                        (new)  canonical GPL-3.0 text
.gitignore                     (edit) add .idea/ and /launchdeck
README.md                      (edit) add a License section
cmd/launchdeck/main.go         (edit) SPDX header; run()/startTUI() refactor + helpers
cmd/launchdeck/main_test.go    (new)  unit/integration tests for the helpers + run()
```

Build order: **Task 1** legal + hygiene (independent), then **Tasks 2–4** the three pure helpers (independent of each other), then **Task 5** wires `run()`/`startTUI()`/`main()` using those helpers.

---

### Task 1: License + repo hygiene

**Files:**
- Create: `LICENSE`
- Modify: `.gitignore`
- Modify: `cmd/launchdeck/main.go` (top-of-file comment only)
- Modify: `README.md` (add License section)

**Interfaces:**
- Produces: nothing consumed by other tasks (the SPDX header lines at the top of `main.go` are preserved by Task 5).

- [ ] **Step 1: Fetch the canonical GPL-3.0 text into `LICENSE`**

Run:
```bash
cd /Users/ivanvolkov/WebDev/LaunchDeck
curl -fsSL https://www.gnu.org/licenses/gpl-3.0.txt -o LICENSE
```
Expected: `LICENSE` created. If `curl` is unavailable/offline, use `wget -qO LICENSE https://www.gnu.org/licenses/gpl-3.0.txt`.

- [ ] **Step 2: Verify `LICENSE` is the full GPL-3.0 text**

Run:
```bash
head -n 1 LICENSE
grep -c "GNU GENERAL PUBLIC LICENSE" LICENSE
wc -l LICENSE
```
Expected: first line contains `GNU GENERAL PUBLIC LICENSE`; the grep count is ≥ 1; `wc -l` reports ~674 lines (the full text, not a truncated stub). If it is short (a redirect/error page), re-fetch.

- [ ] **Step 3: Add ignore rules**

Append to `.gitignore` (create the lines if absent):
```
# IDE
.idea/

# built binary
/launchdeck
```

- [ ] **Step 4: Untrack IDE files (no-op-safe) and verify ignore rules**

Run:
```bash
git rm -r --cached --ignore-unmatch .idea
git ls-files | grep -E '\.idea/|^launchdeck$' || echo "clean: nothing tracked"
git check-ignore -q .idea/ && git check-ignore -q launchdeck && echo "ignored: ok"
```
Expected: the `git rm` is a no-op (nothing tracked) and does **not** error thanks to `--ignore-unmatch`; the grep prints `clean: nothing tracked`; the check-ignore prints `ignored: ok`.

- [ ] **Step 5: Add the SPDX + copyright header to `main.go`**

Insert these two lines as the very first lines of `cmd/launchdeck/main.go`, above `package main`:
```go
// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 volkoffskij

package main
```

- [ ] **Step 6: Add a License section to `README.md`**

Append to the end of `README.md`:
```markdown
## License

Copyright (C) 2026 volkoffskij.
GPL-3.0-or-later — see [`LICENSE`](LICENSE).
```

- [ ] **Step 7: Verify build still passes and the header/README checks hold**

Run:
```bash
go build ./... && echo BUILD_OK
head -n 2 cmd/launchdeck/main.go
grep -n 'LICENSE' README.md
```
Expected: `BUILD_OK`; the two `main.go` lines are the SPDX + copyright comments; the README grep matches the License link.

- [ ] **Step 8: Commit**

```bash
git add LICENSE .gitignore README.md cmd/launchdeck/main.go
git commit -m "chore: add GPL-3.0 license, ignore IDE files and the built binary"
```

---

### Task 2: `buildVersion` + `versionString`

**Files:**
- Modify: `cmd/launchdeck/main.go`
- Test: `cmd/launchdeck/main_test.go`

**Interfaces:**
- Produces: `var version = "dev"`; `func buildVersion(version, mainVer, rev string, modified, hasInfo bool) string`; `func versionString() string`. Task 5 calls `versionString()`.

- [ ] **Step 1: Write the failing test**

Create `cmd/launchdeck/main_test.go`:
```go
package main

import "testing"

func TestBuildVersion(t *testing.T) {
	sha := "0123456789abcdef0123456789abcdef01234567" // 40 chars
	cases := []struct {
		name                       string
		version, mainVer, rev      string
		modified, hasInfo          bool
		want                       string
	}{
		{"release", "v1.2.3", "", "", false, false, "launchdeck v1.2.3"},
		{"module install", "dev", "v1.2.3", "", false, true, "launchdeck v1.2.3"},
		{"dev vcs clean", "dev", "", sha, false, true, "launchdeck dev (0123456789ab)"},
		{"dev vcs dirty", "dev", "", sha, true, true, "launchdeck dev (0123456789ab-dirty)"},
		{"dev short rev", "dev", "", "abc", false, true, "launchdeck dev (abc)"},
		{"dev devel no rev", "dev", "(devel)", "", false, true, "launchdeck dev"},
		{"dev no build info", "dev", "", "", false, false, "launchdeck dev"},
	}
	for _, c := range cases {
		if got := buildVersion(c.version, c.mainVer, c.rev, c.modified, c.hasInfo); got != c.want {
			t.Errorf("%s: buildVersion = %q, want %q", c.name, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/launchdeck/ -run TestBuildVersion -v`
Expected: FAIL — `undefined: buildVersion`.

- [ ] **Step 3: Write minimal implementation**

Add to `cmd/launchdeck/main.go` (imports: add `"runtime/debug"`):
```go
// version is overridden at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// buildVersion assembles the version line from the ldflags-injected version plus
// build-info values. It is pure so it can be table-tested. The returned string is
// already prefixed with "launchdeck ".
func buildVersion(version, mainVer, rev string, modified, hasInfo bool) string {
	if version != "dev" {
		return "launchdeck " + version
	}
	if hasInfo {
		if mainVer != "" && mainVer != "(devel)" {
			return "launchdeck " + mainVer // module-proxy install: go install <module>@<ver>
		}
		if rev != "" {
			r := rev
			if len(r) > 12 {
				r = r[:12]
			}
			if modified {
				r += "-dirty"
			}
			return "launchdeck dev (" + r + ")"
		}
	}
	return "launchdeck dev"
}

// versionString reads the real build info and delegates to buildVersion.
func versionString() string {
	if version != "dev" {
		return buildVersion(version, "", "", false, false)
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return buildVersion(version, "", "", false, false)
	}
	var rev string
	var modified bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	return buildVersion(version, info.Main.Version, rev, modified, true)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/launchdeck/ -run TestBuildVersion -v`
Expected: PASS (all 7 rows).

- [ ] **Step 5: Commit**

```bash
git add cmd/launchdeck/main.go cmd/launchdeck/main_test.go
git commit -m "feat(cmd): version string from ldflags and build info"
```

---

### Task 3: `helpText`

**Files:**
- Modify: `cmd/launchdeck/main.go`
- Test: `cmd/launchdeck/main_test.go`

**Interfaces:**
- Produces: `func helpText() string`. Task 5 prints it on `--help`.

- [ ] **Step 1: Write the failing test**

Append to `cmd/launchdeck/main_test.go`:
```go
import "strings" // add to the existing import block

func TestHelpText(t *testing.T) {
	h := helpText()
	for _, want := range []string{
		"Usage: launchdeck [flags]",
		"~/.config/launchdeck/session.json",
		"~/.config/launchdeck/theme.json",
		"Press ?",
	} {
		if !strings.Contains(h, want) {
			t.Errorf("helpText missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/launchdeck/ -run TestHelpText -v`
Expected: FAIL — `undefined: helpText`.

- [ ] **Step 3: Write minimal implementation**

Add to `cmd/launchdeck/main.go`:
```go
// helpText is the hand-written --help output (not flag.PrintDefaults, which
// writes to stderr and dumps -h/-v default-value noise).
func helpText() string {
	return `launchdeck — a macOS launchctl services TUI

Usage: launchdeck [flags]

Flags:
  -h, --help     show this help and exit
  -v, --version  print the version and exit

Config files:
  ~/.config/launchdeck/session.json   restored UI session
  ~/.config/launchdeck/theme.json     colours and header toggle

Press ? inside the app for the full keymap.`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/launchdeck/ -run TestHelpText -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/launchdeck/main.go cmd/launchdeck/main_test.go
git commit -m "feat(cmd): --help text"
```

---

### Task 4: `crashMessage`

**Files:**
- Modify: `cmd/launchdeck/main.go`
- Test: `cmd/launchdeck/main_test.go`

**Interfaces:**
- Produces: `func crashMessage(v any, version string) string`. Task 5's recover writes it to stderr.

- [ ] **Step 1: Write the failing test**

Append to `cmd/launchdeck/main_test.go`:
```go
func TestCrashMessage(t *testing.T) {
	const url = "please report: https://github.com/volkoffskij/launchdeck/issues"

	got := crashMessage("boom", "launchdeck v1.2.3")
	want := "launchdeck v1.2.3 crashed: boom\n" + url
	if got != want {
		t.Errorf("string value:\n got %q\nwant %q", got, want)
	}

	got = crashMessage(42, "launchdeck dev")
	want = "launchdeck dev crashed: 42\n" + url
	if got != want {
		t.Errorf("non-string value:\n got %q\nwant %q", got, want)
	}

	// A multi-line panic value collapses to a single line — the message is
	// always exactly two lines.
	got = crashMessage("line1\nline2", "launchdeck dev")
	if lines := strings.Count(got, "\n"); lines != 1 {
		t.Errorf("multi-line value produced %d newlines, want exactly 1: %q", lines, got)
	}
	if !strings.Contains(got, "line1 line2") {
		t.Errorf("multi-line value not collapsed: %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/launchdeck/ -run TestCrashMessage -v`
Expected: FAIL — `undefined: crashMessage`.

- [ ] **Step 3: Write minimal implementation**

Add to `cmd/launchdeck/main.go` (imports: add `"fmt"` if not already present, and `"strings"`):
```go
// crashMessage formats the two-line crash report. It is pure and deterministic:
// the panic value is rendered with %v and any newlines collapsed to spaces, so
// the message is always exactly two lines regardless of the value's type.
func crashMessage(v any, version string) string {
	val := strings.ReplaceAll(fmt.Sprintf("%v", v), "\n", " ")
	return version + " crashed: " + val +
		"\nplease report: https://github.com/volkoffskij/launchdeck/issues"
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/launchdeck/ -run TestCrashMessage -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/launchdeck/main.go cmd/launchdeck/main_test.go
git commit -m "feat(cmd): crash message formatter"
```

---

### Task 5: `run()` / `startTUI()` wiring + flags + panic recovery

**Files:**
- Modify: `cmd/launchdeck/main.go`
- Test: `cmd/launchdeck/main_test.go`

**Interfaces:**
- Consumes: `versionString()` (Task 2), `helpText()` (Task 3), `crashMessage(any, string)` (Task 4).
- Produces: `func run(args []string, stdout, stderr io.Writer, start func() int) (code int)`; `func startTUI() int`; a `main` that calls `os.Exit(run(...))`.

- [ ] **Step 1: Write the failing tests**

Append to `cmd/launchdeck/main_test.go` (imports: add `"io"`, `"bytes"`):
```go
// noStart fails the test if the guards/TUI path is reached.
func noStart(t *testing.T) func() int {
	return func() int {
		t.Helper()
		t.Fatal("start (guards/TUI) reached — info flag should have returned first")
		return 0
	}
}

func TestRunVersionAndHelpReturnBeforeGuards(t *testing.T) {
	cases := [][]string{{"--version"}, {"-v"}, {"--help"}, {"-h"}, {"--version", "--help"}}
	for _, args := range cases {
		var out, errb bytes.Buffer
		code := run(args, &out, &errb, noStart(t))
		if code != 0 {
			t.Errorf("%v: code = %d, want 0", args, code)
		}
		if out.Len() == 0 {
			t.Errorf("%v: expected stdout output", args)
		}
	}
}

func TestRunHelpWinsOverVersion(t *testing.T) {
	var out bytes.Buffer
	run([]string{"--version", "--help"}, &out, io.Discard, noStart(t))
	if !strings.Contains(out.String(), "Usage: launchdeck [flags]") {
		t.Errorf("--version --help should print help, got %q", out.String())
	}
}

func TestRunUnknownFlag(t *testing.T) {
	var errb bytes.Buffer
	code := run([]string{"--nope"}, io.Discard, &errb, func() int { return 0 })
	if code != 2 {
		t.Errorf("unknown flag: code = %d, want 2", code)
	}
	// The one-line hint is shown, not flag.PrintDefaults()'s -h/-v dump.
	if strings.Contains(errb.String(), "-version") || strings.Contains(errb.String(), "default") {
		t.Errorf("stderr leaked flag default dump: %q", errb.String())
	}
}

func TestRunRecoversMainGoroutinePanic(t *testing.T) {
	var errb bytes.Buffer
	code := run(nil, io.Discard, &errb, func() int { panic("boom") })
	if code != 1 {
		t.Errorf("panic: code = %d, want 1", code)
	}
	if !strings.Contains(errb.String(), "crashed: boom") ||
		!strings.Contains(errb.String(), "please report:") {
		t.Errorf("crash message not written to stderr: %q", errb.String())
	}
}

func TestRunNormalPathCallsStart(t *testing.T) {
	called := false
	code := run(nil, io.Discard, io.Discard, func() int { called = true; return 7 })
	if !called {
		t.Error("normal path did not call start")
	}
	if code != 7 {
		t.Errorf("code = %d, want 7 (start's return propagated)", code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/launchdeck/ -run TestRun -v`
Expected: FAIL — `undefined: run`.

- [ ] **Step 3: Rewrite `main.go`'s bottom half**

Replace the current `func main() { ... }` in `cmd/launchdeck/main.go` with the following (keep the SPDX/copyright header from Task 1, the imports, and the helpers from Tasks 2–4). Update the import block to include `"flag"`, `"io"`; it already needs `"fmt"`, `"os"`, `"os/exec"`, `"runtime"`, `"runtime/debug"`, `"strings"`, `tea`, and the internal packages.

```go
func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, startTUI))
}

// run parses flags, handles --version/--help, and otherwise calls start (the
// guards + TUI). A deferred recover turns a main-goroutine panic into a clean
// two-line message on stderr and exit code 1. start is injected so tests can
// drive run without launching a TUI.
func run(args []string, stdout, stderr io.Writer, start func() int) (code int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(stderr, crashMessage(r, versionString()))
			code = 1
		}
	}()

	var showVersion, showHelp bool
	fs := flag.NewFlagSet("launchdeck", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: launchdeck [flags] (run --help for details)")
	}
	fs.BoolVar(&showVersion, "version", false, "print the version and exit")
	fs.BoolVar(&showVersion, "v", false, "print the version and exit")
	fs.BoolVar(&showHelp, "help", false, "show this help and exit")
	fs.BoolVar(&showHelp, "h", false, "show this help and exit")
	if err := fs.Parse(args); err != nil {
		return 2 // usage error; fs already wrote the message + one-line hint
	}

	if showHelp { // --help wins over --version
		fmt.Fprintln(stdout, helpText())
		return 0
	}
	if showVersion {
		fmt.Fprintln(stdout, versionString())
		return 0
	}
	return start()
}

// startTUI runs the platform guards and the TUI, returning an exit code. This is
// the former body of main(); it is reached only on the normal no-flag path.
func startTUI() int {
	if runtime.GOOS != "darwin" {
		fmt.Fprintln(os.Stderr, "launchdeck: macOS only")
		return 1
	}
	if _, err := exec.LookPath("launchctl"); err != nil {
		fmt.Fprintln(os.Stderr, "launchdeck: launchctl not found in PATH")
		return 1
	}

	uid := os.Getuid()
	var st app.AppState
	if p, err := session.Path(); err == nil {
		st = app.FromSession(session.Load(p), uid)
	} else {
		st = app.NewState(uid)
	}

	m := ui.New(st, launchctl.New())
	if p, err := ui.ThemePath(); err == nil {
		m = m.WithTheme(ui.LoadTheme(p))
	}
	prog := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := prog.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "launchdeck:", err)
		return 1
	}
	return 0
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/launchdeck/ -v`
Expected: PASS (all `TestRun*`, `TestBuildVersion`, `TestHelpText`, `TestCrashMessage`).

- [ ] **Step 5: Build + manual smoke**

Run:
```bash
go build -o launchdeck ./cmd/launchdeck
./launchdeck --version 1>/tmp/v.out 2>/tmp/v.err; echo "exit=$?"; cat /tmp/v.out; echo "stderr:"; cat /tmp/v.err
./launchdeck --help    1>/tmp/h.out 2>/tmp/h.err; echo "exit=$?"; test -s /tmp/h.out && echo "stdout non-empty"; test -s /tmp/h.err && echo "STDERR LEAK" || echo "stderr empty"
./launchdeck --nope; echo "badflag exit=$?"
go vet ./... && gofmt -l .
```
Expected: `--version` prints `launchdeck …` to stdout, `exit=0`, stderr empty; `--help` prints to stdout (`stdout non-empty`, `stderr empty`), `exit=0`; `--nope` → `badflag exit=2`; `go vet` clean; `gofmt -l` prints nothing.

- [ ] **Step 6: Full suite + commit**

Run: `go test ./...`
Expected: all packages PASS.
```bash
git add cmd/launchdeck/main.go cmd/launchdeck/main_test.go
git commit -m "feat(cmd): --version/--help flags and main-goroutine panic recovery"
```

---

## Self-Review Notes

- **Spec coverage:** §1 License → Task 1 (LICENSE byte-identical, SPDX+copyright, README section). §2 Repo hygiene → Task 1 (.gitignore + `--ignore-unmatch` no-op + check-ignore). §3 CLI flags → Tasks 2 (versionString), 3 (helpText), 5 (FlagSet ContinueOnError, `--help` precedence, guard seam, exit 0/2, no PrintDefaults). §4 Panic recovery → Tasks 4 (crashMessage) + 5 (run() defer/recover, code 1, main-goroutine scope). Error Handling (bad flag → 2) → Task 5. Testing rows → Tasks 2–5 tests + Task 5 manual smoke. Success Criteria 1–4 → Tasks 1 & 5.
- **Deferred by spec (not built here):** command-goroutine panic recovery; terminal-reset escape output (Bubble Tea handles restore); `debug.Stack()` dump. All explicitly out of scope in the spec.
- **Manual-only checks:** the forced-panic *binary* smoke test (terminal-restore verification) from the spec is covered functionally by `TestRunRecoversMainGoroutinePanic` (code + stderr); the terminal-restore observation is inherently interactive — run it by hand on a Mac if desired, but it is not a blocker since our recover only formats the message (Bubble Tea owns restore).
- **Header preservation:** Task 5 rewrites the bottom of `main.go`; the implementer must keep Task 1's SPDX/copyright comment lines at the top.
