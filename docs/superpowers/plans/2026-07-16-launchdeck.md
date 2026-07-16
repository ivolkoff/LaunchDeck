# LaunchDeck Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a macOS terminal-UI dashboard that lists, inspects, and controls `launchctl` services, with keyboard + mouse, and restores its UI session on relaunch.

**Architecture:** MVU (Bubble Tea). A framework-agnostic **core** (`internal/launchctl`, `internal/session`) and a pure **app seam** (`internal/app`: `reduce(Msg,State)→State` and `derive(State)→ViewModel`, both pure) carry all logic and are unit-tested with zero TUI deps. A thin, swappable **`internal/ui/bubbletea`** render layer renders the ViewModel and translates key/mouse input into intents. `launchctl` output enters `State` only through async data messages, keeping `reduce`/`derive` pure and table-testable.

**Tech Stack:** Go 1.23, [Bubble Tea](https://github.com/charmbracelet/bubbletea) (MVU), [Lip Gloss](https://github.com/charmbracelet/lipgloss) (styling), [Bubbles](https://github.com/charmbracelet/bubbles) (viewport), [bubblezone](https://github.com/lrstanley/bubblezone) (mouse hit-testing). Shells out to the system `launchctl`.

## Global Constraints

- **Platform:** macOS only. `runtime.GOOS != "darwin"` or a missing `launchctl` binary → fatal at startup with a clear message.
- **Go module:** `github.com/volkoffskij/launchdeck`, `go 1.23`.
- **Purity:** `reduce` and `derive` are pure functions of their inputs. All `launchctl`/filesystem output enters `State` only via async data messages (`ServicesLoaded`, `ServiceDetailLoaded`, `LogLinesAppended`, `ActionResult`). No I/O, clocks, or randomness inside `reduce`/`derive`.
- **Recognized domains:** exactly `gui/<uid>` and `system`. Every `print`/log/action target is `<domain>/<label>`.
- **No AI attribution** in commits. Conventional-commit messages.
- **Session file:** `~/.config/launchdeck/session.json`. Writes are atomic (temp file + rename) after `mkdir -p`.
- **Timeouts:** poll & detail fetch 3s; each per-service action 10s.
- **Bounds:** log ring 5000 lines; initial tail per path = last 64 KB then last 500 lines of that read.
- **Security:** never capture or store a sudo password; only `sudo` sees it.

---

## File Structure

```
go.mod
cmd/launchdeck/main.go              wire core + ui, signal handling, start Bubble Tea
internal/
  launchctl/
    types.go        Domain, RunState, EnableState, Service, ServiceDetail, ActionKind
    parse.go        parseDomainScan(), parseServiceDetail(), classifyFailure()
    parse_test.go
    client.go       Client{run}, ScanDomain, Print, Action, Bootstrap (injectable runner)
    client_test.go
    integration_test.go   Tier 1 (read-only) + Tier 2 (mutating, opt-in)
  session/
    session.go      Session struct, Load(), Save() (atomic), path helper
    session_test.go
  app/
    state.go        AppState + all nested structs + enums for focus/tab/scope/loadState
    intent.go       Msg interface + all Intent + async-msg types
    reduce.go       Reduce(Msg, State) → State
    reduce_test.go
    viewmodel.go    ViewModel, ListVM, DetailVM, StatusVM
    derive.go       Derive(State) → ViewModel
    derive_test.go
    filter.go       applyFilter(), applySort()  (pure helpers used by derive)
    filter_test.go
    permission.go   ClassifyFailure duplicate? no — lives in launchctl; app re-exports kind
  ui/
    bubbletea/
      model.go      tea.Model: holds *app.State + core clients; Init/Update/View
      keys.go       key → Intent (respecting modal suppression)
      mouse.go      bubblezone zone id → Intent
      cmds.go       tea.Cmd builders: pollCmd, detailCmd, logTailCmd, actionCmd, sudoCmd
      view.go       Derive(state) → strings; top-level layout + min-size gate
      list.go       render ListVM (sidebar)
      detail.go     render DetailVM (tabs: Metadata/Logs/Raw)
      statusbar.go  render StatusVM + action buttons + prompts
```

Build order is bottom-up: **Phase 1** core (`launchctl`), **Phase 2** app seam (`app`), **Phase 3** `session`, **Phase 4** `ui` + `main`. Phases 1–3 are fully unit-tested; Phase 4 is verified by smoke run + the manual checklist.

---

# Phase 1 — core: `internal/launchctl`

### Task 1: Module + domain types

**Files:**
- Create: `go.mod`
- Create: `internal/launchctl/types.go`
- Test: `internal/launchctl/types_test.go`

**Interfaces:**
- Produces: `Domain{Kind string; UID int}`, `Domain.String()`, `Domain.Target(label string) string`, `GUIDomain(uid int)`, `SystemDomain`, `RunState`, `EnableState`, `Service`, `ServiceDetail`, `ActionKind` + `ActionKind.String()`.

- [ ] **Step 1: Init the module**

Run:
```bash
cd /Users/ivanvolkov/WebDev/LaunchDeck
go mod init github.com/volkoffskij/launchdeck
go mod edit -go=1.23
```
Expected: `go.mod` created with module path and `go 1.23`.

- [ ] **Step 2: Write the failing test**

`internal/launchctl/types_test.go`:
```go
package launchctl

import "testing"

func TestDomainString(t *testing.T) {
	if got := GUIDomain(501).String(); got != "gui/501" {
		t.Fatalf("gui: got %q", got)
	}
	if got := SystemDomain().String(); got != "system" {
		t.Fatalf("system: got %q", got)
	}
}

func TestDomainTarget(t *testing.T) {
	if got := GUIDomain(501).Target("com.example.a"); got != "gui/501/com.example.a" {
		t.Fatalf("gui target: got %q", got)
	}
	if got := SystemDomain().Target("com.example.a"); got != "system/com.example.a" {
		t.Fatalf("system target: got %q", got)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/launchctl/ -run TestDomain -v`
Expected: FAIL — `undefined: GUIDomain`.

- [ ] **Step 4: Write minimal implementation**

`internal/launchctl/types.go`:
```go
// Package launchctl wraps the macOS launchctl CLI: enumeration, inspection,
// and lifecycle actions. It has zero TUI dependencies.
package launchctl

import "strconv"

// Domain is a launchd domain target. Kind is "gui" or "system".
type Domain struct {
	Kind string
	UID  int // meaningful only when Kind == "gui"
}

func GUIDomain(uid int) Domain { return Domain{Kind: "gui", UID: uid} }
func SystemDomain() Domain      { return Domain{Kind: "system"} }

func (d Domain) String() string {
	if d.Kind == "gui" {
		return "gui/" + strconv.Itoa(d.UID)
	}
	return "system"
}

// Target builds the "<domain>/<label>" specifier launchctl verbs take.
func (d Domain) Target(label string) string { return d.String() + "/" + label }

type RunState int

const (
	Stopped RunState = iota
	Running
)

type EnableState int

const (
	EnableUnknown EnableState = iota
	Enabled
	Disabled
)

// Service is one row from a domain scan.
type Service struct {
	Label    string
	Domain   Domain
	PID      int  // 0 when HasPID is false
	HasPID   bool
	LastExit int
}

func (s Service) RunState() RunState {
	if s.HasPID {
		return Running
	}
	return Stopped
}

// ServiceDetail is the parsed `launchctl print <domain>/<label>` dump.
type ServiceDetail struct {
	Service
	Program     string
	Args        []string
	PlistPath   string
	StdoutPath  string
	StderrPath  string
	EnableState EnableState
	Raw         string // the full dump, always populated
}

type ActionKind int

const (
	ActionStart ActionKind = iota // kickstart
	ActionRestart                 // kickstart -k
	ActionStop                    // kill TERM
	ActionEnable                  // enable
	ActionDisable                 // disable
	ActionUnload                  // bootout
	ActionLoad                    // bootstrap (uses a plist path, not a label target)
)

func (a ActionKind) String() string {
	switch a {
	case ActionStart:
		return "start"
	case ActionRestart:
		return "restart"
	case ActionStop:
		return "stop"
	case ActionEnable:
		return "enable"
	case ActionDisable:
		return "disable"
	case ActionUnload:
		return "unload"
	case ActionLoad:
		return "load"
	default:
		return "unknown"
	}
}

// Destructive reports whether the action needs an in-TUI confirm.
func (a ActionKind) Destructive() bool {
	return a == ActionStop || a == ActionDisable || a == ActionUnload
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/launchctl/ -run TestDomain -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod internal/launchctl/types.go internal/launchctl/types_test.go
git commit -m "feat(core): launchctl domain and service types"
```

---

### Task 2: Parse a domain scan (`launchctl print <domain>` services table)

**Files:**
- Create: `internal/launchctl/parse.go`
- Test: `internal/launchctl/parse_test.go`

**Interfaces:**
- Produces: `parseDomainScan(dump string, d Domain) ([]Service, error)`. Parses the `services = { ... }` block: each row is `<pid>\t<last-exit>\t<label>` where `pid`/`last-exit` may be `-`.

- [ ] **Step 1: Write the failing test**

`internal/launchctl/parse_test.go`:
```go
package launchctl

import "testing"

const guiScanFixture = `gui/501 = {
	type = User
	handle = 501
	active count = 42
	services = {
		12345	0	com.example.running
		-	0	com.example.stopped
		-	78	com.example.crashed
		-	-	com.apple.never.ran
	}
}
`

func TestParseDomainScan(t *testing.T) {
	svcs, err := parseDomainScan(guiScanFixture, GUIDomain(501))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(svcs) != 4 {
		t.Fatalf("want 4 services, got %d", len(svcs))
	}
	run := svcs[0]
	if run.Label != "com.example.running" || !run.HasPID || run.PID != 12345 || run.LastExit != 0 {
		t.Fatalf("running row wrong: %+v", run)
	}
	if run.Domain.String() != "gui/501" {
		t.Fatalf("domain not stamped: %v", run.Domain)
	}
	stopped := svcs[1]
	if stopped.HasPID || stopped.PID != 0 || stopped.LastExit != 0 {
		t.Fatalf("stopped row wrong: %+v", stopped)
	}
	crashed := svcs[2]
	if crashed.HasPID || crashed.LastExit != 78 {
		t.Fatalf("crashed row wrong: %+v", crashed)
	}
}

func TestParseDomainScanNoServicesBlock(t *testing.T) {
	if _, err := parseDomainScan("gui/501 = {\n}\n", GUIDomain(501)); err == nil {
		t.Fatal("expected error when services block absent")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/launchctl/ -run TestParseDomainScan -v`
Expected: FAIL — `undefined: parseDomainScan`.

- [ ] **Step 3: Write minimal implementation**

`internal/launchctl/parse.go`:
```go
package launchctl

import (
	"bufio"
	"errors"
	"strconv"
	"strings"
)

// parseDomainScan extracts the services table from a `launchctl print <domain>`
// dump. Each row is "<pid>\t<last-exit>\t<label>"; "-" means absent.
func parseDomainScan(dump string, d Domain) ([]Service, error) {
	start := strings.Index(dump, "services = {")
	if start < 0 {
		return nil, errors.New("launchctl print: no services block")
	}
	rest := dump[start+len("services = {"):]
	end := strings.Index(rest, "}")
	if end < 0 {
		return nil, errors.New("launchctl print: unterminated services block")
	}
	block := rest[:end]

	var out []Service
	sc := bufio.NewScanner(strings.NewReader(block))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pidTok, exitTok, label := fields[0], fields[1], fields[len(fields)-1]
		svc := Service{Label: label, Domain: d}
		if pidTok != "-" {
			if pid, err := strconv.Atoi(pidTok); err == nil {
				svc.PID, svc.HasPID = pid, true
			}
		}
		if exitTok != "-" {
			if code, err := strconv.Atoi(exitTok); err == nil {
				svc.LastExit = code
			}
		}
		out = append(out, svc)
	}
	return out, sc.Err()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/launchctl/ -run TestParseDomainScan -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/launchctl/parse.go internal/launchctl/parse_test.go
git commit -m "feat(core): parse launchctl domain scan services table"
```

---

### Task 3: Parse a service detail dump

**Files:**
- Modify: `internal/launchctl/parse.go`
- Test: `internal/launchctl/parse_test.go`

**Interfaces:**
- Produces: `parseServiceDetail(dump string, svc Service) ServiceDetail`. Best-effort key/value extraction; `Raw` is always the full dump; never errors (spec: degrade to raw).

- [ ] **Step 1: Write the failing test**

Append to `internal/launchctl/parse_test.go`:
```go
const detailFixture = `com.example.running = {
	active count = 1
	path = /Users/me/Library/LaunchAgents/com.example.running.plist
	state = running
	program = /usr/local/bin/agent
	arguments = {
		/usr/local/bin/agent
		--serve
	}
	pid = 12345
	last exit code = 0
	stdout path = /tmp/agent.out
	stderr path = /tmp/agent.err
	disabled = false
}
`

func TestParseServiceDetail(t *testing.T) {
	base := Service{Label: "com.example.running", Domain: GUIDomain(501), PID: 12345, HasPID: true}
	d := parseServiceDetail(detailFixture, base)
	if d.Program != "/usr/local/bin/agent" {
		t.Fatalf("program: %q", d.Program)
	}
	if len(d.Args) != 2 || d.Args[1] != "--serve" {
		t.Fatalf("args: %#v", d.Args)
	}
	if d.PlistPath != "/Users/me/Library/LaunchAgents/com.example.running.plist" {
		t.Fatalf("plist: %q", d.PlistPath)
	}
	if d.StdoutPath != "/tmp/agent.out" || d.StderrPath != "/tmp/agent.err" {
		t.Fatalf("log paths: %q %q", d.StdoutPath, d.StderrPath)
	}
	if d.EnableState != Enabled {
		t.Fatalf("enableState: %v", d.EnableState)
	}
	if d.Raw != detailFixture {
		t.Fatal("raw not preserved verbatim")
	}
}

func TestParseServiceDetailDisabled(t *testing.T) {
	d := parseServiceDetail("x = {\n\tdisabled = true\n}\n", Service{Label: "x"})
	if d.EnableState != Disabled {
		t.Fatalf("want Disabled, got %v", d.EnableState)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/launchctl/ -run TestParseServiceDetail -v`
Expected: FAIL — `undefined: parseServiceDetail`.

- [ ] **Step 3: Write minimal implementation**

Append to `internal/launchctl/parse.go`:
```go
// parseServiceDetail best-effort-parses a `launchctl print <domain>/<label>`
// dump. It never errors: Raw always holds the full dump so the UI can fall back
// to it when a field is missing or the format drifts.
func parseServiceDetail(dump string, svc Service) ServiceDetail {
	d := ServiceDetail{Service: svc, Raw: dump, EnableState: EnableUnknown}
	sc := bufio.NewScanner(strings.NewReader(dump))
	inArgs := false
	for sc.Scan() {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		if inArgs {
			if trimmed == "}" {
				inArgs = false
				continue
			}
			if trimmed != "" {
				d.Args = append(d.Args, trimmed)
			}
			continue
		}
		if trimmed == "arguments = {" {
			inArgs = true
			continue
		}
		key, val, ok := strings.Cut(trimmed, " = ")
		if !ok {
			continue
		}
		switch key {
		case "path":
			d.PlistPath = val
		case "program":
			d.Program = val
		case "stdout path":
			d.StdoutPath = val
		case "stderr path":
			d.StderrPath = val
		case "disabled":
			if val == "true" {
				d.EnableState = Disabled
			} else if val == "false" {
				d.EnableState = Enabled
			}
		}
	}
	return d
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/launchctl/ -run TestParseServiceDetail -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/launchctl/parse.go internal/launchctl/parse_test.go
git commit -m "feat(core): parse launchctl service detail dump"
```

---

### Task 4: Classify a failure as permission vs generic

**Files:**
- Modify: `internal/launchctl/parse.go`
- Test: `internal/launchctl/parse_test.go`

**Interfaces:**
- Produces: `FailureKind` (`FailureGeneric`, `FailurePermission`) and `ClassifyFailure(exitCode int, stderr string) FailureKind`. Permission when exit≠0 AND lower-cased stderr matches any of `operation not permitted`, `permission denied`, `not privileged`, `requires root`, or word-boundary `\berrno (1|13)\b`. `errno 12/19/100` must be generic.

- [ ] **Step 1: Write the failing test**

Append to `internal/launchctl/parse_test.go`:
```go
func TestClassifyFailure(t *testing.T) {
	cases := []struct {
		exit   int
		stderr string
		want   FailureKind
	}{
		{0, "anything", FailureGeneric},                       // exit 0 is never a failure
		{1, "Operation not permitted", FailurePermission},
		{1, "permission denied", FailurePermission},
		{1, "Bootstrap failed: 5: Input/output error", FailureGeneric},
		{1, "Could not find service (errno 1)", FailurePermission},
		{1, "failed (errno 13)", FailurePermission},
		{1, "failed (errno 12)", FailureGeneric},
		{1, "failed (errno 19)", FailureGeneric},
		{1, "failed (errno 100)", FailureGeneric},
		{1, "service is not privileged", FailurePermission},
	}
	for _, c := range cases {
		if got := ClassifyFailure(c.exit, c.stderr); got != c.want {
			t.Errorf("Classify(%d,%q) = %v, want %v", c.exit, c.stderr, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/launchctl/ -run TestClassifyFailure -v`
Expected: FAIL — `undefined: ClassifyFailure`.

- [ ] **Step 3: Write minimal implementation**

Append to `internal/launchctl/parse.go` (add `regexp` to imports):
```go
type FailureKind int

const (
	FailureGeneric FailureKind = iota
	FailurePermission
)

var errnoPermRe = regexp.MustCompile(`\berrno (1|13)\b`)

var permPhrases = []string{
	"operation not permitted",
	"permission denied",
	"not privileged",
	"requires root",
}

// ClassifyFailure decides whether a non-zero launchctl result is a permission
// failure (→ offer sudo retry) or a generic one (→ show stderr).
func ClassifyFailure(exitCode int, stderr string) FailureKind {
	if exitCode == 0 {
		return FailureGeneric
	}
	low := strings.ToLower(stderr)
	for _, p := range permPhrases {
		if strings.Contains(low, p) {
			return FailurePermission
		}
	}
	if errnoPermRe.MatchString(low) {
		return FailurePermission
	}
	return FailureGeneric
}
```
Update the import block in `parse.go` to include `"regexp"`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/launchctl/ -run TestClassifyFailure -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/launchctl/parse.go internal/launchctl/parse_test.go
git commit -m "feat(core): classify permission vs generic launchctl failures"
```

---

### Task 5: Client with an injectable command runner

**Files:**
- Create: `internal/launchctl/client.go`
- Test: `internal/launchctl/client_test.go`

**Interfaces:**
- Produces:
  - `type runFunc func(ctx context.Context, name string, args ...string) (stdout, stderr []byte, exitCode int, err error)`
  - `type Client struct { run runFunc }`
  - `func New() *Client` (real `exec` runner)
  - `func newWith(run runFunc) *Client` (tests)
  - `func (c *Client) ScanDomain(ctx, d Domain) ([]Service, error)`
  - `func (c *Client) Print(ctx, d Domain, label string) (ServiceDetail, error)`
  - `type ActionOutcome struct { Err error; ExitCode int; Stderr string; Kind FailureKind }`
  - `func (c *Client) Action(ctx, a ActionKind, d Domain, label string) ActionOutcome`
  - `func (c *Client) Bootstrap(ctx, d Domain, plistPath string) ActionOutcome`

- [ ] **Step 1: Write the failing test**

`internal/launchctl/client_test.go`:
```go
package launchctl

import (
	"context"
	"strings"
	"testing"
)

func TestClientScanDomain(t *testing.T) {
	c := newWith(func(_ context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		if name != "launchctl" || args[0] != "print" || args[1] != "gui/501" {
			t.Fatalf("unexpected argv: %s %v", name, args)
		}
		return []byte(guiScanFixture), nil, 0, nil
	})
	svcs, err := c.ScanDomain(context.Background(), GUIDomain(501))
	if err != nil || len(svcs) != 4 {
		t.Fatalf("scan: err=%v n=%d", err, len(svcs))
	}
}

func TestClientActionPermission(t *testing.T) {
	c := newWith(func(_ context.Context, _ string, _ ...string) ([]byte, []byte, int, error) {
		return nil, []byte("Operation not permitted"), 1, nil
	})
	out := c.Action(context.Background(), ActionStop, SystemDomain(), "com.x")
	if out.Kind != FailurePermission {
		t.Fatalf("want permission, got %v (stderr=%q)", out.Kind, out.Stderr)
	}
}

func TestClientActionArgv(t *testing.T) {
	var gotArgs []string
	c := newWith(func(_ context.Context, _ string, args ...string) ([]byte, []byte, int, error) {
		gotArgs = args
		return nil, nil, 0, nil
	})
	c.Action(context.Background(), ActionRestart, GUIDomain(501), "com.x")
	if strings.Join(gotArgs, " ") != "kickstart -k gui/501/com.x" {
		t.Fatalf("restart argv: %v", gotArgs)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/launchctl/ -run TestClient -v`
Expected: FAIL — `undefined: newWith`.

- [ ] **Step 3: Write minimal implementation**

`internal/launchctl/client.go`:
```go
package launchctl

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
)

type runFunc func(ctx context.Context, name string, args ...string) (stdout, stderr []byte, exitCode int, err error)

type Client struct{ run runFunc }

func New() *Client { return &Client{run: execRun} }

func newWith(run runFunc) *Client { return &Client{run: run} }

// execRun runs a command and returns stdout, stderr, its exit code, and only a
// non-nil err for spawn/timeout failures (a non-zero exit is reported via code).
func execRun(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
			err = nil
		}
	}
	return out.Bytes(), errb.Bytes(), code, err
}

func (c *Client) ScanDomain(ctx context.Context, d Domain) ([]Service, error) {
	stdout, stderr, code, err := c.run(ctx, "launchctl", "print", d.String())
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, &ScanError{Domain: d, ExitCode: code, Stderr: string(stderr),
			Kind: ClassifyFailure(code, string(stderr))}
	}
	return parseDomainScan(string(stdout), d)
}

// ScanError distinguishes a permission-denied enumeration (→ sudo enumerate) from
// a generic scan failure.
type ScanError struct {
	Domain   Domain
	ExitCode int
	Stderr   string
	Kind     FailureKind
}

func (e *ScanError) Error() string { return "scan " + e.Domain.String() + ": " + e.Stderr }

func (c *Client) Print(ctx context.Context, d Domain, label string) (ServiceDetail, error) {
	stdout, stderr, code, err := c.run(ctx, "launchctl", "print", d.Target(label))
	if err != nil {
		return ServiceDetail{}, err
	}
	if code != 0 {
		return ServiceDetail{}, &ScanError{Domain: d, ExitCode: code, Stderr: string(stderr),
			Kind: ClassifyFailure(code, string(stderr))}
	}
	return parseServiceDetail(string(stdout), Service{Label: label, Domain: d}), nil
}

type ActionOutcome struct {
	Err      error // spawn/timeout error (ctx cancelled → timed out)
	ExitCode int
	Stderr   string
	Kind     FailureKind
}

func (o ActionOutcome) OK() bool { return o.Err == nil && o.ExitCode == 0 }

func actionArgs(a ActionKind, target string) []string {
	switch a {
	case ActionStart:
		return []string{"kickstart", target}
	case ActionRestart:
		return []string{"kickstart", "-k", target}
	case ActionStop:
		return []string{"kill", "TERM", target}
	case ActionEnable:
		return []string{"enable", target}
	case ActionDisable:
		return []string{"disable", target}
	case ActionUnload:
		return []string{"bootout", target}
	default:
		return nil
	}
}

func (c *Client) Action(ctx context.Context, a ActionKind, d Domain, label string) ActionOutcome {
	args := actionArgs(a, d.Target(label))
	if args == nil {
		return ActionOutcome{Err: errors.New("action has no label form: " + a.String())}
	}
	_, stderr, code, err := c.run(ctx, "launchctl", args...)
	return ActionOutcome{Err: err, ExitCode: code, Stderr: string(stderr),
		Kind: ClassifyFailure(code, string(stderr))}
}

func (c *Client) Bootstrap(ctx context.Context, d Domain, plistPath string) ActionOutcome {
	_, stderr, code, err := c.run(ctx, "launchctl", "bootstrap", d.String(), plistPath)
	return ActionOutcome{Err: err, ExitCode: code, Stderr: string(stderr),
		Kind: ClassifyFailure(code, string(stderr))}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/launchctl/ -run TestClient -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/launchctl/client.go internal/launchctl/client_test.go
git commit -m "feat(core): launchctl client with injectable runner"
```

---

### Task 6: Integration tests (Tier 1 read-only, Tier 2 opt-in mutating)

**Files:**
- Create: `internal/launchctl/integration_test.go`

**Interfaces:**
- Consumes: `New()`, `ScanDomain`, `Print`, `Action`, `Bootstrap`, `GUIDomain`, `os.Getuid`, `os.Getpid`.

- [ ] **Step 1: Write the Tier 1 read-only test**

`internal/launchctl/integration_test.go`:
```go
package launchctl

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestIntegrationScanReadOnly(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c := New()
	svcs, err := c.ScanDomain(ctx, GUIDomain(os.Getuid()))
	if err != nil {
		t.Fatalf("gui scan: %v", err)
	}
	if len(svcs) == 0 {
		t.Skip("no gui services on this machine")
	}
	for _, s := range svcs {
		if s.Label == "" {
			t.Fatal("parsed a service with empty label")
		}
		if s.Domain.String() != GUIDomain(os.Getuid()).String() {
			t.Fatalf("domain not stamped: %v", s.Domain)
		}
	}
	// One detail fetch must parse without crashing.
	if _, err := c.Print(ctx, GUIDomain(os.Getuid()), svcs[0].Label); err != nil {
		t.Logf("print %s: %v (non-fatal — some services deny inspect)", svcs[0].Label, err)
	}
}
```

- [ ] **Step 2: Run it**

Run: `go test ./internal/launchctl/ -run TestIntegrationScanReadOnly -v`
Expected on macOS: PASS (or SKIP if no services). On other OS: SKIP.

- [ ] **Step 3: Write the Tier 2 mutating test**

Append to `internal/launchctl/integration_test.go`:
```go
func TestIntegrationActionRoundTrip(t *testing.T) {
	if runtime.GOOS != "darwin" || os.Getenv("LAUNCHDECK_INTEGRATION") != "1" {
		t.Skip("set LAUNCHDECK_INTEGRATION=1 on darwin to run")
	}
	uid := os.Getuid()
	dom := GUIDomain(uid)
	label := "com.launchdeck.itest." + itoa(os.Getpid())
	dir := t.TempDir()
	plist := filepath.Join(dir, label+".plist")
	body := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>` + label + `</string>
  <key>ProgramArguments</key><array><string>/bin/sh</string><string>-c</string><string>while true; do sleep 1; done</string></array>
</dict></plist>`
	if err := os.WriteFile(plist, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c := New()
	ctx := context.Background()

	t.Cleanup(func() {
		c.Action(ctx, ActionUnload, dom, label) // bootout, ignore error
		os.Remove(plist)
	})

	if out := c.Bootstrap(ctx, dom, plist); !out.OK() {
		t.Fatalf("bootstrap: %+v", out)
	}
	if out := c.Action(ctx, ActionStart, dom, label); !out.OK() {
		t.Fatalf("kickstart: %+v", out)
	}
	// Give launchd a moment, then confirm a PID via print.
	time.Sleep(500 * time.Millisecond)
	d, err := c.Print(ctx, dom, label)
	if err != nil {
		t.Fatalf("print after start: %v", err)
	}
	if !d.HasPID {
		t.Fatalf("expected a running PID after kickstart, got %+v", d.Service)
	}
	if out := c.Action(ctx, ActionStop, dom, label); !out.OK() {
		t.Fatalf("kill: %+v", out)
	}
	if out := c.Action(ctx, ActionUnload, dom, label); !out.OK() {
		t.Fatalf("bootout: %+v", out)
	}
}

func itoa(n int) string { // avoid importing strconv just for the label
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
```

- [ ] **Step 4: Run it opted-in (on a Mac only)**

Run: `LAUNCHDECK_INTEGRATION=1 go test ./internal/launchctl/ -run TestIntegrationActionRoundTrip -v`
Expected on macOS: PASS. Otherwise: SKIP.

- [ ] **Step 5: Commit**

```bash
git add internal/launchctl/integration_test.go
git commit -m "test(core): tier-1 read-only and tier-2 opt-in launchctl integration"
```

---

# Phase 2 — app seam: `internal/app`

> `reduce`/`derive` are pure. `launchctl` output enters only via async messages. This phase carries the bulk of the spec's behavior and is entirely table-tested.

### Task 7: State + enums

**Files:**
- Create: `internal/app/state.go`

**Interfaces:**
- Produces the `AppState` struct and its nested types. Names below are load-bearing — later tasks reference them exactly.

- [ ] **Step 1: Write `state.go`**

`internal/app/state.go`:
```go
package app

import "github.com/volkoffskij/launchdeck/internal/launchctl"

type Focus int

const (
	FocusSidebar Focus = iota
	FocusDetail
)

type Tab int

const (
	TabMetadata Tab = iota
	TabLogs
	TabRaw
)

type DomainScope int

const (
	ScopeUser DomainScope = iota
	ScopeSystem
	ScopeAll
)

type LoadState int

const (
	DetailIdle LoadState = iota
	DetailLoading
	DetailReady
	DetailError
)

type SudoKind int

const (
	SudoAction SudoKind = iota
	SudoInspect
	SudoEnumerate
)

type Filters struct {
	DomainScope DomainScope
	TextPattern string
}

type Scroll struct {
	List int
	Log  int
}

type LogLine struct {
	Stream string // "out" or "err"
	Text   string
}

type Detail struct {
	LoadState LoadState
	Metadata  launchctl.ServiceDetail
	Raw       string
	ErrMsg    string
}

type ActionPicker struct {
	Open            bool
	HighlightedVerb launchctl.ActionKind
}

type PendingConfirm struct {
	Active bool
	Action launchctl.ActionKind
	Target string // "<domain>/<label>" captured at prompt-open
}

type PendingSudo struct {
	Active bool
	Kind   SudoKind
	Target string
}

type LoadPrompt struct {
	Open       bool
	Buffer     string
	Candidates []string
	Highlight  int
}

// AppState is the whole application state. reduce is the only mutator.
type AppState struct {
	Services  []launchctl.Service // domain-scoped scan result (unfiltered, unsorted)
	Selected  string              // selected label ("" = none)
	Gone      bool                // selected label absent from the latest scan

	Filters      Filters
	FilterEditing bool
	FilterBuffer  string

	SortKey   SortKey
	SortDesc  bool

	Scroll   Scroll
	Focus    Focus
	ActiveTab Tab

	Detail       Detail
	LogRing      []LogLine // capped at logRingCap
	TailIdentity string    // "<domain>/<label>" the current tail follows

	StatusMsg string

	ActionPicker   ActionPicker
	PendingConfirm PendingConfirm
	PendingSudo    PendingSudo
	LoadPrompt     LoadPrompt
	ActionRunning  bool

	FirstScanDone     bool
	SelectionResolved bool

	UID int
}

const logRingCap = 5000

type SortKey int

const (
	SortLabel SortKey = iota
	SortStatus
	SortPID
)
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/app/`
Expected: builds (no test yet).

- [ ] **Step 3: Commit**

```bash
git add internal/app/state.go
git commit -m "feat(app): AppState and enums"
```

---

### Task 8: Messages (intents + async)

**Files:**
- Create: `internal/app/intent.go`

**Interfaces:**
- Produces: `Msg` marker interface and every intent/async type used by `reduce`. Later tasks construct these exactly.

- [ ] **Step 1: Write `intent.go`**

`internal/app/intent.go`:
```go
package app

import "github.com/volkoffskij/launchdeck/internal/launchctl"

// Msg is anything reduce ingests: a user Intent or an async data message.
type Msg interface{ isMsg() }

type base struct{}

func (base) isMsg() {}

// --- User intents ---

type SelectService struct{ base; Label string }
type MoveSelection struct{ base; Delta int; ToTop, ToBottom bool } // Delta ±1 or ±page
type FocusPanel struct{ base }                                     // toggle sidebar↔detail
type SetTab struct{ base; Tab Tab }
type ScrollMsg struct{ base; Panel Focus; Delta int }

type RunAction struct{ base; Action launchctl.ActionKind }
type ConfirmAction struct{ base }
type CancelAction struct{ base }

type OpenActionPicker struct{ base }
type MoveActionPicker struct{ base; Delta int }
type PickAction struct{ base; Action launchctl.ActionKind }
type CancelActionPicker struct{ base }

type OpenFilter struct{ base }
type SetFilterBuffer struct{ base; Buffer string }
type CommitFilter struct{ base }
type CancelFilter struct{ base }
type CycleDomainScope struct{ base }
type SetFilter struct{ base; Filters Filters } // internal, emitted by commit/cycle

type SetSort struct{ base; ToggleDir bool } // false = cycle key, true = toggle direction

type OpenLoadPrompt struct{ base }
type SetLoadBuffer struct{ base; Buffer string }
type SubmitLoad struct{ base }
type CancelLoad struct{ base }

type ConfirmSudo struct{ base }
type CancelSudo struct{ base }

type Refresh struct{ base }
type Quit struct{ base }

// --- Async data messages ---

type ServicesLoaded struct {
	base
	Services []launchctl.Service
	Err      *launchctl.ScanError // non-nil → keep prior list; permission → enumerate banner
}

type ServiceDetailLoaded struct {
	base
	Target string // "<domain>/<label>" the fetch was for
	Detail launchctl.ServiceDetail
	Err    *launchctl.ScanError
}

type LogLinesAppended struct {
	base
	TailTarget string
	Lines      []LogLine
	State      string // "", "removed", "unreadable"
}

type ActionResult struct {
	base
	Action   launchctl.ActionKind
	Target   string
	Outcome  launchctl.ActionOutcome
	TimedOut bool
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/app/`
Expected: builds.

- [ ] **Step 3: Commit**

```bash
git add internal/app/intent.go
git commit -m "feat(app): message and intent types"
```

---

### Task 9: `applyFilter` + `applySort` pure helpers

**Files:**
- Create: `internal/app/filter.go`
- Test: `internal/app/filter_test.go`

**Interfaces:**
- Produces: `applyFilter([]launchctl.Service, Filters, uid int) []launchctl.Service` and `applySort([]launchctl.Service, SortKey, desc bool) []launchctl.Service` (returns a sorted copy; total, stable order per spec Determinism).

- [ ] **Step 1: Write the failing test**

`internal/app/filter_test.go`:
```go
package app

import (
	"testing"

	"github.com/volkoffskij/launchdeck/internal/launchctl"
)

func svc(label string, dom launchctl.Domain, pid int) launchctl.Service {
	s := launchctl.Service{Label: label, Domain: dom}
	if pid > 0 {
		s.PID, s.HasPID = pid, true
	}
	return s
}

func TestApplyFilterTextCaseInsensitive(t *testing.T) {
	in := []launchctl.Service{
		svc("com.example.Foo", launchctl.GUIDomain(501), 0),
		svc("com.other.bar", launchctl.GUIDomain(501), 0),
	}
	out := applyFilter(in, Filters{DomainScope: ScopeAll, TextPattern: "foo"}, 501)
	if len(out) != 1 || out[0].Label != "com.example.Foo" {
		t.Fatalf("text filter: %#v", out)
	}
	// empty pattern → all
	if len(applyFilter(in, Filters{DomainScope: ScopeAll}, 501)) != 2 {
		t.Fatal("empty pattern should match all")
	}
}

func TestApplyFilterDomainScope(t *testing.T) {
	in := []launchctl.Service{
		svc("a", launchctl.GUIDomain(501), 0),
		svc("b", launchctl.SystemDomain(), 0),
	}
	if got := applyFilter(in, Filters{DomainScope: ScopeUser}, 501); len(got) != 1 || got[0].Label != "a" {
		t.Fatalf("user scope: %#v", got)
	}
	if got := applyFilter(in, Filters{DomainScope: ScopeSystem}, 501); len(got) != 1 || got[0].Label != "b" {
		t.Fatalf("system scope: %#v", got)
	}
}

func TestApplySortPIDNullsLast(t *testing.T) {
	in := []launchctl.Service{
		svc("z", launchctl.GUIDomain(501), 0), // stopped, null PID
		svc("a", launchctl.GUIDomain(501), 30),
		svc("b", launchctl.GUIDomain(501), 10),
	}
	out := applySort(in, SortPID, false)
	if out[0].Label != "b" || out[1].Label != "a" || out[2].Label != "z" {
		t.Fatalf("pid asc, null last: %v %v %v", out[0].Label, out[1].Label, out[2].Label)
	}
}

func TestApplySortLabelSecondaryTieBreak(t *testing.T) {
	// Two running services tie on status; secondary key = label ascending.
	in := []launchctl.Service{
		svc("Beta", launchctl.GUIDomain(501), 2),
		svc("alpha", launchctl.GUIDomain(501), 3),
	}
	out := applySort(in, SortStatus, false)
	if out[0].Label != "alpha" || out[1].Label != "Beta" {
		t.Fatalf("status tie → label secondary: %v", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run 'TestApplyFilter|TestApplySort' -v`
Expected: FAIL — `undefined: applyFilter`.

- [ ] **Step 3: Write minimal implementation**

`internal/app/filter.go`:
```go
package app

import (
	"sort"
	"strings"

	"github.com/volkoffskij/launchdeck/internal/launchctl"
)

func applyFilter(in []launchctl.Service, f Filters, uid int) []launchctl.Service {
	pat := strings.ToLower(f.TextPattern)
	out := make([]launchctl.Service, 0, len(in))
	for _, s := range in {
		switch f.DomainScope {
		case ScopeUser:
			if s.Domain.Kind != "gui" {
				continue
			}
		case ScopeSystem:
			if s.Domain.Kind != "system" {
				continue
			}
		}
		if pat != "" && !strings.Contains(strings.ToLower(s.Label), pat) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// labelLess is the canonical secondary order: case-insensitive, then bytewise.
func labelLess(a, b string) bool {
	la, lb := strings.ToLower(a), strings.ToLower(b)
	if la != lb {
		return la < lb
	}
	return a < b
}

func applySort(in []launchctl.Service, key SortKey, desc bool) []launchctl.Service {
	out := make([]launchctl.Service, len(in))
	copy(out, in)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		var less bool
		switch key {
		case SortLabel:
			if labelLess(a.Label, b.Label) != labelLess(b.Label, a.Label) {
				return labelLess(a.Label, b.Label) != desc
			}
			return false
		case SortStatus:
			if a.HasPID != b.HasPID {
				// running-before-stopped for ascending; direction flips it.
				less = a.HasPID && !b.HasPID
				if desc {
					return !less
				}
				return less
			}
		case SortPID:
			if a.HasPID != b.HasPID {
				return a.HasPID // null PIDs always last regardless of direction
			}
			if a.HasPID && a.PID != b.PID {
				if desc {
					return a.PID > b.PID
				}
				return a.PID < b.PID
			}
		}
		return labelLess(a.Label, b.Label) // secondary tie-break, always ascending
	})
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app/ -run 'TestApplyFilter|TestApplySort' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/filter.go internal/app/filter_test.go
git commit -m "feat(app): pure filter and deterministic sort helpers"
```

---

### Task 10: `reduce` — ServicesLoaded list-merge + first-scan reconciliation

**Files:**
- Create: `internal/app/reduce.go`
- Test: `internal/app/reduce_test.go`

**Interfaces:**
- Produces: `func Reduce(m Msg, s AppState) AppState` (pure). This task implements only the `ServicesLoaded` case + a `NewState(uid int) AppState` constructor; later tasks extend `Reduce` with more cases.

- [ ] **Step 1: Write the failing test**

`internal/app/reduce_test.go`:
```go
package app

import (
	"testing"

	"github.com/volkoffskij/launchdeck/internal/launchctl"
)

func loaded(svcs ...launchctl.Service) ServicesLoaded { return ServicesLoaded{Services: svcs} }

func TestFirstScanBindsPersistedSelection(t *testing.T) {
	s := NewState(501)
	s.Selected = "com.b" // as if restored from session
	s = Reduce(loaded(
		svc("com.a", launchctl.GUIDomain(501), 0),
		svc("com.b", launchctl.GUIDomain(501), 9),
	), s)
	if s.Selected != "com.b" || !s.FirstScanDone || !s.SelectionResolved {
		t.Fatalf("persisted selection not bound: %+v", s)
	}
}

func TestFirstScanFallsBackToFirstVisible(t *testing.T) {
	s := NewState(501)
	s.Selected = "com.missing"
	s = Reduce(loaded(svc("com.a", launchctl.GUIDomain(501), 0)), s)
	if s.Selected != "com.a" {
		t.Fatalf("want first-visible fallback, got %q", s.Selected)
	}
}

func TestFirstScanEmptyVisibleClears(t *testing.T) {
	s := NewState(501)
	s.Selected = "com.a"
	s.Filters.TextPattern = "zzz" // matches nothing
	s = Reduce(loaded(svc("com.a", launchctl.GUIDomain(501), 0)), s)
	if s.Selected != "" || !s.FirstScanDone {
		t.Fatalf("empty visible should clear selection: %+v", s)
	}
}

func TestLaterScanGoneThenRebind(t *testing.T) {
	s := NewState(501)
	s = Reduce(loaded(svc("com.a", launchctl.GUIDomain(501), 1)), s) // first scan binds com.a
	s = Reduce(loaded(svc("com.b", launchctl.GUIDomain(501), 2)), s) // com.a vanished
	if !s.Gone || s.Selected != "com.a" {
		t.Fatalf("want (gone) com.a, got selected=%q gone=%v", s.Selected, s.Gone)
	}
	s = Reduce(loaded(svc("com.a", launchctl.GUIDomain(501), 3)), s) // reappears
	if s.Gone {
		t.Fatalf("com.a reappeared, should re-bind")
	}
}

func TestScanErrorKeepsPriorList(t *testing.T) {
	s := NewState(501)
	s = Reduce(loaded(svc("com.a", launchctl.GUIDomain(501), 1)), s)
	s = Reduce(ServicesLoaded{Err: &launchctl.ScanError{Kind: launchctl.FailureGeneric, Stderr: "boom"}}, s)
	if len(s.Services) != 1 {
		t.Fatalf("scan error should keep prior list, got %d", len(s.Services))
	}
	if s.StatusMsg == "" {
		t.Fatal("scan error should set a status banner")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run 'TestFirstScan|TestLaterScan|TestScanError' -v`
Expected: FAIL — `undefined: NewState` / `Reduce`.

- [ ] **Step 3: Write minimal implementation**

`internal/app/reduce.go`:
```go
package app

import "github.com/volkoffskij/launchdeck/internal/launchctl"

func NewState(uid int) AppState {
	return AppState{UID: uid, Filters: Filters{DomainScope: ScopeAll}}
}

// visible applies the current filter+sort — the rows the user actually sees.
func (s AppState) visible() []launchctl.Service {
	return applySort(applyFilter(s.Services, s.Filters, s.UID), s.SortKey, s.SortDesc)
}

func containsLabel(svcs []launchctl.Service, label string) bool {
	for _, s := range svcs {
		if s.Label == label {
			return true
		}
	}
	return false
}

func Reduce(m Msg, s AppState) AppState {
	switch msg := m.(type) {
	case ServicesLoaded:
		return reduceServicesLoaded(msg, s)
	}
	return s
}

func reduceServicesLoaded(msg ServicesLoaded, s AppState) AppState {
	if msg.Err != nil {
		if msg.Err.Kind == launchctl.FailurePermission {
			s.StatusMsg = "system requires sudo to enumerate — Retry with sudo"
		} else {
			s.StatusMsg = "failed to parse services"
		}
		return s // keep prior list
	}
	// pendingConfirm auto-cancel when its target vanished.
	if s.PendingConfirm.Active && !containsLabel(msg.Services, labelOf(s.PendingConfirm.Target)) {
		s.PendingConfirm = PendingConfirm{}
	}
	s.Services = msg.Services
	vis := applySort(applyFilter(s.Services, s.Filters, s.UID), s.SortKey, s.SortDesc)

	if !s.FirstScanDone {
		s.FirstScanDone = true
		if s.Selected != "" && containsLabel(vis, s.Selected) {
			s.SelectionResolved = true
		} else if len(vis) > 0 {
			s.Selected = vis[0].Label
			s.SelectionResolved = true
		} else {
			s.Selected = ""
		}
		return s
	}
	// Later scans: (gone) handling for a once-resolved selection.
	if s.SelectionResolved && s.Selected != "" {
		s.Gone = !containsLabel(s.Services, s.Selected)
	}
	return s
}

// labelOf extracts the label from a "<domain>/<label>" target.
func labelOf(target string) string {
	for i := len(target) - 1; i >= 0; i-- {
		if target[i] == '/' {
			return target[i+1:]
		}
	}
	return target
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app/ -run 'TestFirstScan|TestLaterScan|TestScanError' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/reduce.go internal/app/reduce_test.go
git commit -m "feat(app): reduce ServicesLoaded merge and first-scan reconciliation"
```

---

### Task 11: `reduce` — selection, focus, tabs, scroll

**Files:**
- Modify: `internal/app/reduce.go`
- Test: `internal/app/reduce_test.go`

**Interfaces:**
- Extends `Reduce` with `SelectService`, `MoveSelection`, `FocusPanel`, `SetTab`, `ScrollMsg`. Selecting a service resets `Detail.LoadState = DetailLoading` and clears the log ring + tail identity (the actual fetch/tail is a Cmd in Phase 4).

- [ ] **Step 1: Write the failing test**

Append to `internal/app/reduce_test.go`:
```go
func TestSelectServiceResetsDetail(t *testing.T) {
	s := NewState(501)
	s = Reduce(loaded(svc("com.a", launchctl.GUIDomain(501), 1), svc("com.b", launchctl.GUIDomain(501), 2)), s)
	s.LogRing = []LogLine{{Stream: "out", Text: "old"}}
	s = Reduce(SelectService{Label: "com.b"}, s)
	if s.Selected != "com.b" || s.Detail.LoadState != DetailLoading || len(s.LogRing) != 0 {
		t.Fatalf("select should reset detail+log: %+v", s)
	}
	if s.Gone {
		t.Fatal("selecting a present service clears gone")
	}
}

func TestMoveSelectionClamps(t *testing.T) {
	s := NewState(501)
	s = Reduce(loaded(svc("com.a", launchctl.GUIDomain(501), 0), svc("com.b", launchctl.GUIDomain(501), 0)), s)
	// first scan selected com.a
	s = Reduce(MoveSelection{Delta: -1}, s) // already top, stays
	if s.Selected != "com.a" {
		t.Fatalf("clamp top: %q", s.Selected)
	}
	s = Reduce(MoveSelection{ToBottom: true}, s)
	if s.Selected != "com.b" {
		t.Fatalf("to bottom: %q", s.Selected)
	}
}

func TestFocusAndTab(t *testing.T) {
	s := NewState(501)
	s = Reduce(FocusPanel{}, s)
	if s.Focus != FocusDetail {
		t.Fatal("focus toggle")
	}
	s = Reduce(SetTab{Tab: TabRaw}, s)
	if s.ActiveTab != TabRaw {
		t.Fatal("set tab")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run 'TestSelectService|TestMoveSelection|TestFocusAndTab' -v`
Expected: FAIL.

- [ ] **Step 3: Extend `Reduce`**

Add these cases inside the `switch` in `reduce.go`:
```go
	case SelectService:
		return reduceSelect(msg.Label, s)
	case MoveSelection:
		return reduceMove(msg, s)
	case FocusPanel:
		if s.Focus == FocusSidebar {
			s.Focus = FocusDetail
		} else {
			s.Focus = FocusSidebar
		}
		return s
	case SetTab:
		s.ActiveTab = msg.Tab
		return s
	case ScrollMsg:
		if msg.Panel == FocusSidebar {
			s.Scroll.List = clampMin0(s.Scroll.List + msg.Delta)
		} else {
			s.Scroll.Log = clampMin0(s.Scroll.Log + msg.Delta)
		}
		return s
```
Add helpers to `reduce.go`:
```go
func clampMin0(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

func reduceSelect(label string, s AppState) AppState {
	s.Selected = label
	s.Gone = false
	s.SelectionResolved = true
	s.Detail = Detail{LoadState: DetailLoading}
	s.LogRing = nil
	s.TailIdentity = ""
	s.Scroll.Log = 0
	return s
}

func reduceMove(m MoveSelection, s AppState) AppState {
	vis := s.visible()
	if len(vis) == 0 {
		return s
	}
	idx := 0
	for i, v := range vis {
		if v.Label == s.Selected {
			idx = i
			break
		}
	}
	switch {
	case m.ToTop:
		idx = 0
	case m.ToBottom:
		idx = len(vis) - 1
	default:
		idx += m.Delta
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= len(vis) {
		idx = len(vis) - 1
	}
	return reduceSelect(vis[idx].Label, s)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app/ -run 'TestSelectService|TestMoveSelection|TestFocusAndTab' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/reduce.go internal/app/reduce_test.go
git commit -m "feat(app): reduce selection, focus, tab, scroll"
```

---

### Task 12: `reduce` — filter interaction + domain scope + sort

**Files:**
- Modify: `internal/app/reduce.go`
- Test: `internal/app/reduce_test.go`

**Interfaces:**
- Extends `Reduce` with `OpenFilter`, `SetFilterBuffer`, `CommitFilter`, `CancelFilter`, `CycleDomainScope`, `SetFilter`, `SetSort`. `SetFilter`/`SetSort` mutate the persisted fields.

- [ ] **Step 1: Write the failing test**

Append to `internal/app/reduce_test.go`:
```go
func TestFilterEditCommit(t *testing.T) {
	s := NewState(501)
	s.Filters.TextPattern = "old"
	s = Reduce(OpenFilter{}, s)
	if !s.FilterEditing || s.FilterBuffer != "old" {
		t.Fatalf("open seeds buffer: %+v", s)
	}
	s = Reduce(SetFilterBuffer{Buffer: "new"}, s)
	s = Reduce(CommitFilter{}, s)
	if s.FilterEditing || s.Filters.TextPattern != "new" {
		t.Fatalf("commit: %+v", s)
	}
}

func TestFilterCancelRestores(t *testing.T) {
	s := NewState(501)
	s.Filters.TextPattern = "keep"
	s = Reduce(OpenFilter{}, s)
	s = Reduce(SetFilterBuffer{Buffer: "typed"}, s)
	s = Reduce(CancelFilter{}, s)
	if s.FilterEditing || s.Filters.TextPattern != "keep" {
		t.Fatalf("cancel: %+v", s)
	}
}

func TestCycleDomainScope(t *testing.T) {
	s := NewState(501) // ScopeAll
	s = Reduce(CycleDomainScope{}, s)
	if s.Filters.DomainScope != ScopeUser { // all → user → system → all
		t.Fatalf("cycle from all: %v", s.Filters.DomainScope)
	}
}

func TestSetSort(t *testing.T) {
	s := NewState(501) // SortLabel asc
	s = Reduce(SetSort{}, s)
	if s.SortKey != SortStatus {
		t.Fatalf("cycle key: %v", s.SortKey)
	}
	s = Reduce(SetSort{ToggleDir: true}, s)
	if !s.SortDesc {
		t.Fatal("toggle dir")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run 'TestFilter|TestCycleDomain|TestSetSort' -v`
Expected: FAIL.

- [ ] **Step 3: Extend `Reduce`**

Add cases to the `switch`:
```go
	case OpenFilter:
		s.FilterEditing = true
		s.FilterBuffer = s.Filters.TextPattern
		return s
	case SetFilterBuffer:
		s.FilterBuffer = msg.Buffer
		return s
	case CommitFilter:
		s.FilterEditing = false
		f := s.Filters
		f.TextPattern = s.FilterBuffer
		return Reduce(SetFilter{Filters: f}, s)
	case CancelFilter:
		s.FilterEditing = false
		s.FilterBuffer = ""
		return s
	case CycleDomainScope:
		f := s.Filters
		f.DomainScope = (f.DomainScope + 1) % 3 // all(2)→user(0)... wait: order below
		return Reduce(SetFilter{Filters: f}, s)
	case SetFilter:
		s.Filters = msg.Filters
		return s
	case SetSort:
		if msg.ToggleDir {
			s.SortDesc = !s.SortDesc
		} else {
			s.SortKey = (s.SortKey + 1) % 3
		}
		return s
```
Note: `DomainScope` constants are `ScopeUser=0, ScopeSystem=1, ScopeAll=2`. The spec cycle is `user → system → all`. Starting from `ScopeAll` the test expects `ScopeUser`. `(2+1)%3 = 0 = ScopeUser` ✓; `(0+1)%3 = 1 = ScopeSystem` ✓; `(1+1)%3 = 2 = ScopeAll` ✓. Keep the `(f.DomainScope + 1) % 3` line and delete the trailing comment.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app/ -run 'TestFilter|TestCycleDomain|TestSetSort' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/reduce.go internal/app/reduce_test.go
git commit -m "feat(app): reduce filter interaction, domain scope, sort"
```

---

### Task 13: `reduce` — action picker + destructive confirm + single-in-flight guard

**Files:**
- Modify: `internal/app/reduce.go`
- Test: `internal/app/reduce_test.go`

**Interfaces:**
- Extends `Reduce` with `OpenActionPicker`, `MoveActionPicker`, `PickAction`, `CancelActionPicker`, `RunAction`, `ConfirmAction`, `CancelAction`. `RunAction` on a destructive verb sets `PendingConfirm`; non-destructive marks `ActionRunning` (the Cmd fires in Phase 4). A `RunAction`/`ConfirmAction` while `ActionRunning || PendingSudo.Active || PendingConfirm.Active` is ignored with a status note.

- [ ] **Step 1: Write the failing test**

Append to `internal/app/reduce_test.go`:
```go
func selected(t *testing.T) AppState {
	s := NewState(501)
	s = Reduce(loaded(svc("com.a", launchctl.GUIDomain(501), 1)), s)
	return s // first scan selects com.a
}

func TestRunActionDestructiveNeedsConfirm(t *testing.T) {
	s := selected(t)
	s = Reduce(RunAction{Action: launchctl.ActionStop}, s)
	if !s.PendingConfirm.Active || s.PendingConfirm.Target != "gui/501/com.a" {
		t.Fatalf("stop should set pendingConfirm: %+v", s.PendingConfirm)
	}
	if s.ActionRunning {
		t.Fatal("destructive action must not run before confirm")
	}
	s = Reduce(ConfirmAction{}, s)
	if !s.ActionRunning || s.PendingConfirm.Active {
		t.Fatalf("confirm should run + clear: %+v", s)
	}
}

func TestRunActionNonDestructiveRuns(t *testing.T) {
	s := selected(t)
	s = Reduce(RunAction{Action: launchctl.ActionStart}, s)
	if !s.ActionRunning || s.PendingConfirm.Active {
		t.Fatalf("start runs without confirm: %+v", s)
	}
}

func TestSingleInFlightActionIgnored(t *testing.T) {
	s := selected(t)
	s = Reduce(RunAction{Action: launchctl.ActionStart}, s) // now running
	before := s
	s = Reduce(RunAction{Action: launchctl.ActionRestart}, s)
	if s.ActionRunning != before.ActionRunning || s.StatusMsg == "" {
		t.Fatalf("second action must be ignored with a note: %+v", s)
	}
}

func TestActionPickerPick(t *testing.T) {
	s := selected(t)
	s = Reduce(OpenActionPicker{}, s)
	if !s.ActionPicker.Open {
		t.Fatal("picker open")
	}
	s = Reduce(PickAction{Action: launchctl.ActionStart}, s)
	if s.ActionPicker.Open || !s.ActionRunning {
		t.Fatalf("pick dispatches + closes: %+v", s)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run 'TestRunAction|TestSingleInFlight|TestActionPicker' -v`
Expected: FAIL.

- [ ] **Step 3: Extend `Reduce`**

Add cases:
```go
	case OpenActionPicker:
		if s.Selected == "" || s.Gone {
			return s
		}
		s.ActionPicker = ActionPicker{Open: true, HighlightedVerb: launchctl.ActionStart}
		return s
	case MoveActionPicker:
		if s.ActionPicker.Open {
			s.ActionPicker.HighlightedVerb = cyclePickerVerb(s.ActionPicker.HighlightedVerb, msg.Delta)
		}
		return s
	case PickAction:
		s.ActionPicker = ActionPicker{}
		return reduceRunAction(msg.Action, s)
	case CancelActionPicker:
		s.ActionPicker = ActionPicker{}
		return s
	case RunAction:
		return reduceRunAction(msg.Action, s)
	case ConfirmAction:
		if !s.PendingConfirm.Active {
			return s
		}
		act := s.PendingConfirm.Action
		s.PendingConfirm = PendingConfirm{}
		return startAction(act, s)
	case CancelAction:
		s.PendingConfirm = PendingConfirm{}
		return s
```
Add helpers:
```go
// pickerVerbs is the picker order and matches the keymap shortcuts.
var pickerVerbs = []launchctl.ActionKind{
	launchctl.ActionStart, launchctl.ActionRestart, launchctl.ActionStop,
	launchctl.ActionEnable, launchctl.ActionDisable, launchctl.ActionUnload,
}

func cyclePickerVerb(cur launchctl.ActionKind, delta int) launchctl.ActionKind {
	idx := 0
	for i, v := range pickerVerbs {
		if v == cur {
			idx = i
		}
	}
	idx = (idx + delta + len(pickerVerbs)) % len(pickerVerbs)
	return pickerVerbs[idx]
}

func busy(s AppState) bool {
	return s.ActionRunning || s.PendingSudo.Active || s.PendingConfirm.Active
}

func reduceRunAction(a launchctl.ActionKind, s AppState) AppState {
	if s.Selected == "" || s.Gone {
		return s
	}
	if busy(s) {
		s.StatusMsg = "action already running"
		return s
	}
	if a.Destructive() {
		s.PendingConfirm = PendingConfirm{
			Active: true, Action: a,
			Target: targetOf(s),
		}
		return s
	}
	return startAction(a, s)
}

// startAction marks the action in-flight; the Cmd that actually runs launchctl is
// built by the ui layer from ActionRunning + Selected (see Phase 4).
func startAction(a launchctl.ActionKind, s AppState) AppState {
	s.ActionRunning = true
	s.StatusMsg = a.String() + "…"
	s.pendingAction = a // stored so ui knows which verb to run
	return s
}

func targetOf(s AppState) string {
	for _, sv := range s.Services {
		if sv.Label == s.Selected {
			return sv.Domain.Target(sv.Label)
		}
	}
	return s.Selected
}
```
Add a `pendingAction launchctl.ActionKind` field to `AppState` in `state.go` (unexported; the ui reads it via a small accessor added next).

- [ ] **Step 4: Add the accessor and run tests**

Append to `state.go`:
```go
func (s AppState) PendingAction() launchctl.ActionKind { return s.pendingAction }
```
Run: `go test ./internal/app/ -run 'TestRunAction|TestSingleInFlight|TestActionPicker' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/reduce.go internal/app/reduce_test.go internal/app/state.go
git commit -m "feat(app): reduce action picker, destructive confirm, in-flight guard"
```

---

### Task 14: `reduce` — ActionResult + sudo pending

**Files:**
- Modify: `internal/app/reduce.go`
- Test: `internal/app/reduce_test.go`

**Interfaces:**
- Extends `Reduce` with `ActionResult`, `ConfirmSudo`, `CancelSudo`. A permission `ActionResult` sets `PendingSudo{Kind: SudoAction}`; a generic failure shows stderr; a timeout shows "timed out"; success clears `ActionRunning`. `ConfirmSudo` clears the running flag and leaves the ui to run the sudo Cmd (dispatch by `PendingSudo.Kind`). `CancelSudo` clears `PendingSudo`.

- [ ] **Step 1: Write the failing test**

Append to `internal/app/reduce_test.go`:
```go
func TestActionResultPermissionSetsSudo(t *testing.T) {
	s := selected(t)
	s = Reduce(RunAction{Action: launchctl.ActionStart}, s)
	s = Reduce(ActionResult{
		Action: launchctl.ActionStart, Target: "gui/501/com.a",
		Outcome: launchctl.ActionOutcome{ExitCode: 1, Stderr: "Operation not permitted", Kind: launchctl.FailurePermission},
	}, s)
	if s.ActionRunning || !s.PendingSudo.Active || s.PendingSudo.Kind != SudoAction {
		t.Fatalf("permission result → pendingSudo: %+v", s)
	}
}

func TestActionResultTimeout(t *testing.T) {
	s := selected(t)
	s = Reduce(RunAction{Action: launchctl.ActionStart}, s)
	s = Reduce(ActionResult{Action: launchctl.ActionStart, TimedOut: true}, s)
	if s.ActionRunning || s.StatusMsg == "" {
		t.Fatalf("timeout clears running + notes: %+v", s)
	}
}

func TestActionResultSuccess(t *testing.T) {
	s := selected(t)
	s = Reduce(RunAction{Action: launchctl.ActionStart}, s)
	s = Reduce(ActionResult{Action: launchctl.ActionStart, Outcome: launchctl.ActionOutcome{ExitCode: 0}}, s)
	if s.ActionRunning || s.PendingSudo.Active {
		t.Fatalf("success clears state: %+v", s)
	}
}

func TestCancelSudoClears(t *testing.T) {
	s := selected(t)
	s.PendingSudo = PendingSudo{Active: true, Kind: SudoAction, Target: "gui/501/com.a"}
	s = Reduce(CancelSudo{}, s)
	if s.PendingSudo.Active {
		t.Fatal("cancel clears sudo")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run 'TestActionResult|TestCancelSudo' -v`
Expected: FAIL.

- [ ] **Step 3: Extend `Reduce`**

Add cases:
```go
	case ActionResult:
		s.ActionRunning = false
		if msg.TimedOut {
			s.StatusMsg = msg.Action.String() + " timed out"
			return s
		}
		if msg.Outcome.OK() {
			s.StatusMsg = msg.Action.String() + " ok"
			return s
		}
		if msg.Outcome.Kind == launchctl.FailurePermission {
			s.PendingSudo = PendingSudo{Active: true, Kind: SudoAction, Target: msg.Target}
			s.StatusMsg = msg.Action.String() + " needs sudo — Retry with sudo"
			return s
		}
		s.StatusMsg = msg.Action.String() + " failed: " + msg.Outcome.Stderr
		return s
	case ConfirmSudo:
		if !s.PendingSudo.Active {
			return s
		}
		// The ui runs the sudo Cmd by PendingSudo.Kind; reduce just flags it running.
		s.ActionRunning = s.PendingSudo.Kind == SudoAction
		s.SudoConfirmed = true
		return s
	case CancelSudo:
		s.PendingSudo = PendingSudo{}
		s.SudoConfirmed = false
		s.StatusMsg = ""
		return s
```
Add fields to `AppState` in `state.go`: `SudoConfirmed bool`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app/ -run 'TestActionResult|TestCancelSudo' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/reduce.go internal/app/reduce_test.go internal/app/state.go
git commit -m "feat(app): reduce ActionResult and sudo pending state"
```

---

### Task 15: `reduce` — detail fetch result + never-clobber-pendingSudo + log lines

**Files:**
- Modify: `internal/app/reduce.go`
- Test: `internal/app/reduce_test.go`

**Interfaces:**
- Extends `Reduce` with `ServiceDetailLoaded`, `LogLinesAppended`. A `ServiceDetailLoaded{Target}` whose target ≠ current `<domain>/<label>` is dropped. A `LogLinesAppended{TailTarget}` for a superseded tail is dropped; otherwise appended to `LogRing` capped at `logRingCap`. A `ServicesLoaded` while `PendingSudo.Active` updates the list only (already true — add a test to pin it).

- [ ] **Step 1: Write the failing test**

Append to `internal/app/reduce_test.go`:
```go
func TestDetailLoadedCurrentVsStale(t *testing.T) {
	s := selected(t) // com.a selected, loadState Loading? (loaded() path sets it via first-scan? no) 
	s.Detail.LoadState = DetailLoading
	det := launchctl.ServiceDetail{Service: launchctl.Service{Label: "com.a"}, Program: "/bin/x"}
	s = Reduce(ServiceDetailLoaded{Target: "gui/501/com.a", Detail: det}, s)
	if s.Detail.LoadState != DetailReady || s.Detail.Metadata.Program != "/bin/x" {
		t.Fatalf("current detail should load: %+v", s.Detail)
	}
	// stale target dropped
	s2 := selected(t)
	s2.Detail.LoadState = DetailLoading
	s2 = Reduce(ServiceDetailLoaded{Target: "gui/501/OTHER", Detail: det}, s2)
	if s2.Detail.LoadState != DetailLoading {
		t.Fatal("stale detail must be dropped")
	}
}

func TestLogLinesRingCap(t *testing.T) {
	s := selected(t)
	s.TailIdentity = "gui/501/com.a"
	big := make([]LogLine, logRingCap+10)
	for i := range big {
		big[i] = LogLine{Stream: "out", Text: "x"}
	}
	s = Reduce(LogLinesAppended{TailTarget: "gui/501/com.a", Lines: big}, s)
	if len(s.LogRing) != logRingCap {
		t.Fatalf("ring should cap at %d, got %d", logRingCap, len(s.LogRing))
	}
}

func TestLogLinesStaleDropped(t *testing.T) {
	s := selected(t)
	s.TailIdentity = "gui/501/com.a"
	s = Reduce(LogLinesAppended{TailTarget: "gui/501/OLD", Lines: []LogLine{{Text: "x"}}}, s)
	if len(s.LogRing) != 0 {
		t.Fatal("stale tail lines must be dropped")
	}
}

func TestServicesLoadedNeverClobbersSudo(t *testing.T) {
	s := selected(t)
	s.PendingSudo = PendingSudo{Active: true, Kind: SudoAction, Target: "gui/501/com.a"}
	s = Reduce(loaded(svc("com.a", launchctl.GUIDomain(501), 5)), s)
	if !s.PendingSudo.Active {
		t.Fatal("ServicesLoaded must not clobber pendingSudo")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run 'TestDetailLoaded|TestLogLines|TestServicesLoadedNeverClobbers' -v`
Expected: FAIL.

- [ ] **Step 3: Extend `Reduce`**

Add cases:
```go
	case ServiceDetailLoaded:
		if msg.Target != targetOf(s) {
			return s // superseded / stale
		}
		if msg.Err != nil {
			s.Detail.LoadState = DetailError
			if msg.Err.Kind == launchctl.FailurePermission {
				s.Detail.ErrMsg = "requires sudo to inspect"
			} else {
				s.Detail.ErrMsg = msg.Err.Stderr
			}
			return s
		}
		s.Detail.LoadState = DetailReady
		s.Detail.Metadata = msg.Detail
		s.Detail.Raw = msg.Detail.Raw
		s.Detail.ErrMsg = ""
		return s
	case LogLinesAppended:
		if msg.TailTarget != targetOf(s) {
			return s
		}
		s.LogRing = append(s.LogRing, msg.Lines...)
		if len(s.LogRing) > logRingCap {
			s.LogRing = s.LogRing[len(s.LogRing)-logRingCap:]
		}
		return s
```
The never-clobber-pendingSudo behavior already holds because `reduceServicesLoaded` never touches `PendingSudo`; the test simply pins it.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app/ -run 'TestDetailLoaded|TestLogLines|TestServicesLoadedNeverClobbers' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/reduce.go internal/app/reduce_test.go
git commit -m "feat(app): reduce detail-loaded, log-lines ring, sudo protection"
```

---

### Task 16: `reduce` — load prompt + domain inference

**Files:**
- Modify: `internal/app/reduce.go`
- Test: `internal/app/reduce_test.go`

**Interfaces:**
- Produces: `inferDomain(path string, uid int) (launchctl.Domain, bool)` and extends `Reduce` with `OpenLoadPrompt`, `SetLoadBuffer`, `SubmitLoad`, `CancelLoad`. `SubmitLoad` with an un-inferrable path sets a status error and keeps the prompt open (`LoadPrompt.Open` stays true). A valid path sets `ActionRunning` and stores the resolved plist+domain for the ui Cmd.

- [ ] **Step 1: Write the failing test**

Append to `internal/app/reduce_test.go`:
```go
func TestInferDomain(t *testing.T) {
	if d, ok := inferDomain("/Users/me/Library/LaunchAgents/x.plist", 501); !ok || d.Kind != "gui" {
		t.Fatalf("agents → gui: %v %v", d, ok)
	}
	if d, ok := inferDomain("/Library/LaunchDaemons/x.plist", 501); !ok || d.Kind != "system" {
		t.Fatalf("daemons → system: %v %v", d, ok)
	}
	if _, ok := inferDomain("/tmp/x.plist", 501); ok {
		t.Fatal("neither dir → reject")
	}
}

func TestSubmitLoadRejectKeepsPromptOpen(t *testing.T) {
	s := NewState(501)
	s = Reduce(OpenLoadPrompt{}, s)
	s = Reduce(SetLoadBuffer{Buffer: "/tmp/x.plist"}, s)
	s = Reduce(SubmitLoad{}, s)
	if !s.LoadPrompt.Open || s.StatusMsg == "" {
		t.Fatalf("un-inferrable path keeps prompt open with error: %+v", s)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run 'TestInferDomain|TestSubmitLoad' -v`
Expected: FAIL.

- [ ] **Step 3: Extend `Reduce`**

Add to `reduce.go` (add `"strings"` import):
```go
func inferDomain(path string, uid int) (launchctl.Domain, bool) {
	switch {
	case strings.Contains(path, "/LaunchAgents/"):
		return launchctl.GUIDomain(uid), true
	case strings.Contains(path, "/LaunchDaemons/"):
		return launchctl.SystemDomain(), true
	default:
		return launchctl.Domain{}, false
	}
}
```
Add cases:
```go
	case OpenLoadPrompt:
		s.LoadPrompt = LoadPrompt{Open: true, Buffer: homeLaunchAgents(s.UID)}
		return s
	case SetLoadBuffer:
		s.LoadPrompt.Buffer = msg.Buffer
		return s
	case CancelLoad:
		s.LoadPrompt = LoadPrompt{}
		return s
	case SubmitLoad:
		dom, ok := inferDomain(s.LoadPrompt.Buffer, s.UID)
		if !ok {
			s.StatusMsg = "cannot infer domain (path is under neither LaunchAgents nor LaunchDaemons)"
			return s // prompt stays open
		}
		s.LoadPrompt = LoadPrompt{}
		s.ActionRunning = true
		s.loadTarget = loadTarget{domain: dom, plist: s.LoadPrompt.Buffer}
		s.StatusMsg = "load…"
		return s
```
Wait — `s.LoadPrompt.Buffer` is read *after* clearing. Fix by capturing first:
```go
	case SubmitLoad:
		path := s.LoadPrompt.Buffer
		dom, ok := inferDomain(path, s.UID)
		if !ok {
			s.StatusMsg = "cannot infer domain (path is under neither LaunchAgents nor LaunchDaemons)"
			return s
		}
		s.LoadPrompt = LoadPrompt{}
		s.ActionRunning = true
		s.loadTarget = loadTarget{domain: dom, plist: path}
		s.StatusMsg = "load…"
		return s
```
Add to `state.go`:
```go
type loadTarget struct {
	domain launchctl.Domain
	plist  string
}

func (s AppState) LoadTarget() (launchctl.Domain, string, bool) {
	return s.loadTarget.domain, s.loadTarget.plist, s.loadTarget.plist != ""
}

func homeLaunchAgents(uid int) string { return "~/Library/LaunchAgents/" }
```
Add the field `loadTarget loadTarget` to `AppState`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app/ -run 'TestInferDomain|TestSubmitLoad' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/reduce.go internal/app/reduce_test.go internal/app/state.go
git commit -m "feat(app): reduce load prompt and domain inference"
```

---

### Task 17: `derive` — ViewModel

**Files:**
- Create: `internal/app/viewmodel.go`
- Create: `internal/app/derive.go`
- Test: `internal/app/derive_test.go`

**Interfaces:**
- Produces: `ViewModel{List ListVM; Detail DetailVM; Status StatusVM}`, `ListVM{Rows []RowVM; Placeholder string; SelectedIdx int}`, `RowVM{Label,Domain string; Running bool; Selected,Gone bool}`, `DetailVM{Mode string; ...}`, `StatusVM{Message string; Prompt string; Buttons []string}`, and `func Derive(s AppState) ViewModel`.

- [ ] **Step 1: Write the failing test**

`internal/app/derive_test.go`:
```go
package app

import (
	"testing"

	"github.com/volkoffskij/launchdeck/internal/launchctl"
)

func TestDeriveLoadingPlaceholder(t *testing.T) {
	s := NewState(501) // no scan yet
	vm := Derive(s)
	if vm.List.Placeholder != "Loading services…" {
		t.Fatalf("loading placeholder: %q", vm.List.Placeholder)
	}
}

func TestDeriveNoMatching(t *testing.T) {
	s := NewState(501)
	s = Reduce(loaded(svc("com.a", launchctl.GUIDomain(501), 0)), s)
	s.Filters.TextPattern = "zzz"
	vm := Derive(s)
	if vm.List.Placeholder != "No matching services" {
		t.Fatalf("no-match placeholder: %q", vm.List.Placeholder)
	}
}

func TestDeriveNoSelection(t *testing.T) {
	s := NewState(501)
	s = Reduce(loaded(svc("com.a", launchctl.GUIDomain(501), 0)), s)
	s.Selected = ""
	vm := Derive(s)
	if vm.Detail.Mode != "empty" {
		t.Fatalf("no selection → empty detail, got %q", vm.Detail.Mode)
	}
}

func TestDeriveRowsSortedAndSelected(t *testing.T) {
	s := NewState(501)
	s = Reduce(loaded(
		svc("com.b", launchctl.GUIDomain(501), 2),
		svc("com.a", launchctl.GUIDomain(501), 1),
	), s) // first scan selects the first VISIBLE row = com.a (label sort)
	vm := Derive(s)
	if len(vm.List.Rows) != 2 || vm.List.Rows[0].Label != "com.a" {
		t.Fatalf("rows sorted by label: %#v", vm.List.Rows)
	}
	if vm.List.SelectedIdx != 0 {
		t.Fatalf("selected idx: %d", vm.List.SelectedIdx)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run TestDerive -v`
Expected: FAIL — `undefined: Derive`.

- [ ] **Step 3: Write the ViewModel + Derive**

`internal/app/viewmodel.go`:
```go
package app

type RowVM struct {
	Label    string
	Domain   string
	Running  bool
	Selected bool
	Gone     bool
}

type ListVM struct {
	Rows        []RowVM
	Placeholder string // non-empty → render this instead of rows
	SelectedIdx int
}

type DetailVM struct {
	Mode     string // "empty" | "loading" | "ready" | "error" | "gone"
	ActiveTab Tab
	// Metadata tab
	Label, Domain, PID, LastExit, RunState, EnableState, Program, Plist string
	// Logs tab
	LogLines []string // already prefixed [out]/[err]
	LogNote  string   // "no log configured" | "log removed" | ...
	// Raw tab
	Raw string
	Err string
}

type StatusVM struct {
	Message string
	Prompt  string   // active confirm/sudo/filter/load prompt text ("" if none)
	Buttons []string // action-button labels
}

type ViewModel struct {
	List   ListVM
	Detail DetailVM
	Status StatusVM
}
```

`internal/app/derive.go`:
```go
package app

import (
	"fmt"

	"github.com/volkoffskij/launchdeck/internal/launchctl"
)

func Derive(s AppState) ViewModel {
	return ViewModel{
		List:   deriveList(s),
		Detail: deriveDetail(s),
		Status: deriveStatus(s),
	}
}

func deriveList(s AppState) ListVM {
	if !s.FirstScanDone {
		return ListVM{Placeholder: "Loading services…"}
	}
	vis := s.visible()
	if len(vis) == 0 {
		return ListVM{Placeholder: "No matching services"}
	}
	vm := ListVM{Rows: make([]RowVM, len(vis)), SelectedIdx: -1}
	for i, sv := range vis {
		sel := sv.Label == s.Selected
		if sel {
			vm.SelectedIdx = i
		}
		vm.Rows[i] = RowVM{
			Label:    sv.Label,
			Domain:   sv.Domain.String(),
			Running:  sv.HasPID,
			Selected: sel,
			Gone:     sel && s.Gone,
		}
	}
	return vm
}

func deriveDetail(s AppState) DetailVM {
	if s.Selected == "" {
		return DetailVM{Mode: "empty"}
	}
	d := DetailVM{ActiveTab: s.ActiveTab, Raw: s.Detail.Raw}
	if s.Gone {
		d.Mode = "gone"
	} else {
		switch s.Detail.LoadState {
		case DetailLoading, DetailIdle:
			d.Mode = "loading"
		case DetailError:
			d.Mode = "error"
			d.Err = s.Detail.ErrMsg
		default:
			d.Mode = "ready"
		}
	}
	m := s.Detail.Metadata
	d.Label = m.Label
	d.Domain = m.Domain.String()
	if m.HasPID {
		d.PID = fmt.Sprintf("%d", m.PID)
		d.RunState = "running"
	} else {
		d.PID = "-"
		d.RunState = "stopped"
	}
	d.LastExit = fmt.Sprintf("%d", m.LastExit)
	d.EnableState = enableStr(m.EnableState)
	d.Program = m.Program
	d.Plist = m.PlistPath
	d.LogLines, d.LogNote = deriveLog(s)
	return d
}

func enableStr(e launchctl.EnableState) string {
	switch e {
	case launchctl.Enabled:
		return "enabled"
	case launchctl.Disabled:
		return "disabled"
	default:
		return "?"
	}
}

func deriveLog(s AppState) ([]string, string) {
	if len(s.LogRing) == 0 {
		if s.Detail.Metadata.StdoutPath == "" && s.Detail.Metadata.StderrPath == "" {
			return nil, "no log configured"
		}
		return nil, ""
	}
	out := make([]string, len(s.LogRing))
	for i, l := range s.LogRing {
		out[i] = "[" + l.Stream + "] " + l.Text
	}
	return out, ""
}

func deriveStatus(s AppState) StatusVM {
	st := StatusVM{
		Message: s.StatusMsg,
		Buttons: []string{"Start", "Restart", "Stop", "Enable", "Disable", "Unload"},
	}
	switch {
	case s.PendingConfirm.Active:
		st.Prompt = fmt.Sprintf("%s %s? (y/n)", s.PendingConfirm.Action, labelOf(s.PendingConfirm.Target))
	case s.PendingSudo.Active:
		st.Prompt = "Retry with sudo? (y/n)"
	case s.FilterEditing:
		st.Prompt = "filter: " + s.FilterBuffer
	case s.LoadPrompt.Open:
		st.Prompt = "load plist: " + s.LoadPrompt.Buffer
	case s.ActionPicker.Open:
		st.Prompt = "action: " + s.ActionPicker.HighlightedVerb.String() + " (s/r/k/e/d/u, Enter, Esc)"
	}
	return st
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app/ -run TestDerive -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/viewmodel.go internal/app/derive.go internal/app/derive_test.go
git commit -m "feat(app): derive ViewModel from state"
```

---

### Task 18: `derive` — "(gone)" frozen detail + full app-package test sweep

**Files:**
- Modify: `internal/app/derive.go` (already handles gone; add banner text)
- Test: `internal/app/derive_test.go`

**Interfaces:**
- Consumes existing `Derive`. Confirms the "(gone)" mode freezes last-known Metadata and shows the banner.

- [ ] **Step 1: Write the failing test**

Append to `internal/app/derive_test.go`:
```go
func TestDeriveGoneFrozen(t *testing.T) {
	s := NewState(501)
	s = Reduce(loaded(svc("com.a", launchctl.GUIDomain(501), 1)), s)     // binds com.a
	s.Detail = Detail{LoadState: DetailReady, Metadata: launchctl.ServiceDetail{
		Service: launchctl.Service{Label: "com.a", Domain: launchctl.GUIDomain(501)}, Program: "/bin/x"}}
	s = Reduce(loaded(svc("com.b", launchctl.GUIDomain(501), 2)), s)      // com.a gone
	vm := Derive(s)
	if vm.Detail.Mode != "gone" || vm.Detail.Program != "/bin/x" {
		t.Fatalf("gone should freeze last-known metadata: %+v", vm.Detail)
	}
}
```

- [ ] **Step 2: Run it and confirm gone banner text**

Run: `go test ./internal/app/ -run TestDeriveGoneFrozen -v`
Expected: PASS (mode "gone", Program preserved). If the banner text is needed in the ui, it renders "(gone) — service no longer present" from `Mode == "gone"` in Task 26.

- [ ] **Step 3: Run the whole app package**

Run: `go test ./internal/app/ -v`
Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/app/derive_test.go
git commit -m "test(app): gone-selection frozen detail derive"
```

---

# Phase 3 — session: `internal/session`

### Task 19: Session load/save (atomic) + robustness

**Files:**
- Create: `internal/session/session.go`
- Test: `internal/session/session_test.go`

**Interfaces:**
- Produces:
  - `type Session struct { Selected string; TextPattern string; DomainScope int; SortKey int; SortDesc bool; ListScroll int; ActiveTab int }` (JSON tags).
  - `func Path() (string, error)` → `~/.config/launchdeck/session.json`.
  - `func Load(path string) Session` — missing/corrupt → zero value, never errors.
  - `func Save(path string, s Session) error` — `mkdir -p` + atomic temp+rename.

- [ ] **Step 1: Write the failing test**

`internal/session/session_test.go`:
```go
package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "session.json") // sub dir does not exist yet
	in := Session{Selected: "com.a", TextPattern: "web", DomainScope: 2, SortKey: 1, SortDesc: true, ListScroll: 4, ActiveTab: 2}
	if err := Save(p, in); err != nil {
		t.Fatalf("save: %v", err)
	}
	got := Load(p)
	if got != in {
		t.Fatalf("round trip: %+v != %+v", got, in)
	}
}

func TestLoadMissingIsZero(t *testing.T) {
	got := Load(filepath.Join(t.TempDir(), "nope.json"))
	if got != (Session{}) {
		t.Fatalf("missing → zero, got %+v", got)
	}
}

func TestLoadCorruptIsZero(t *testing.T) {
	p := filepath.Join(t.TempDir(), "session.json")
	os.WriteFile(p, []byte("{not json"), 0o644)
	if got := Load(p); got != (Session{}) {
		t.Fatalf("corrupt → zero, got %+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/session/ -v`
Expected: FAIL — `undefined: Save`.

- [ ] **Step 3: Write minimal implementation**

`internal/session/session.go`:
```go
// Package session persists the UI session (selection, filters, sort, scroll, tab)
// so relaunching restores the view.
package session

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Session struct {
	Selected    string `json:"selected"`
	TextPattern string `json:"text_pattern"`
	DomainScope int    `json:"domain_scope"`
	SortKey     int    `json:"sort_key"`
	SortDesc    bool   `json:"sort_desc"`
	ListScroll  int    `json:"list_scroll"`
	ActiveTab   int    `json:"active_tab"`
}

func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "launchdeck", "session.json"), nil
}

// Load never errors: missing or corrupt file → zero Session.
func Load(path string) Session {
	var s Session
	data, err := os.ReadFile(path)
	if err != nil {
		return Session{}
	}
	if json.Unmarshal(data, &s) != nil {
		return Session{}
	}
	return s
}

// Save writes atomically (temp file + rename) after ensuring the dir exists.
func Save(path string, s Session) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "session-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/session/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/session/session.go internal/session/session_test.go
git commit -m "feat(session): atomic load/save with missing/corrupt robustness"
```

---

### Task 20: Session ↔ AppState mapping

**Files:**
- Create: `internal/app/session_map.go`
- Test: `internal/app/session_map_test.go`

**Interfaces:**
- Produces: `func FromSession(s session.Session, uid int) AppState` (seeds a fresh state, ignoring unknown enum values → defaults) and `func ToSession(s AppState) session.Session`.

- [ ] **Step 1: Write the failing test**

`internal/app/session_map_test.go`:
```go
package app

import (
	"testing"

	"github.com/volkoffskij/launchdeck/internal/session"
)

func TestFromSessionClampsUnknownEnums(t *testing.T) {
	in := session.Session{Selected: "com.a", DomainScope: 99, SortKey: 42, ActiveTab: 7, ListScroll: 5}
	st := FromSession(in, 501)
	if st.Selected != "com.a" || st.Scroll.List != 5 {
		t.Fatalf("basic fields: %+v", st)
	}
	if st.Filters.DomainScope != ScopeAll || st.SortKey != SortLabel || st.ActiveTab != TabMetadata {
		t.Fatalf("unknown enums must fall back to defaults: %+v", st)
	}
}

func TestToFromSessionRoundTrip(t *testing.T) {
	st := NewState(501)
	st.Selected = "com.x"
	st.Filters = Filters{DomainScope: ScopeSystem, TextPattern: "db"}
	st.SortKey, st.SortDesc = SortPID, true
	st.Scroll.List, st.ActiveTab = 3, TabRaw
	got := FromSession(ToSession(st), 501)
	if got.Selected != "com.x" || got.Filters != st.Filters || got.SortKey != SortPID || !got.SortDesc || got.Scroll.List != 3 || got.ActiveTab != TabRaw {
		t.Fatalf("round trip: %+v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run 'TestFromSession|TestToFromSession' -v`
Expected: FAIL.

- [ ] **Step 3: Write minimal implementation**

`internal/app/session_map.go`:
```go
package app

import "github.com/volkoffskij/launchdeck/internal/session"

func FromSession(sess session.Session, uid int) AppState {
	s := NewState(uid)
	s.Selected = sess.Selected
	s.Filters.TextPattern = sess.TextPattern
	if sess.DomainScope >= 0 && sess.DomainScope <= int(ScopeAll) {
		s.Filters.DomainScope = DomainScope(sess.DomainScope)
	} // else default ScopeAll from NewState
	if sess.SortKey >= 0 && sess.SortKey <= int(SortPID) {
		s.SortKey = SortKey(sess.SortKey)
	}
	s.SortDesc = sess.SortDesc
	if sess.ActiveTab >= 0 && sess.ActiveTab <= int(TabRaw) {
		s.ActiveTab = Tab(sess.ActiveTab)
	}
	if sess.ListScroll > 0 {
		s.Scroll.List = sess.ListScroll
	}
	return s
}

func ToSession(s AppState) session.Session {
	return session.Session{
		Selected:    s.Selected,
		TextPattern: s.Filters.TextPattern,
		DomainScope: int(s.Filters.DomainScope),
		SortKey:     int(s.SortKey),
		SortDesc:    s.SortDesc,
		ListScroll:  s.Scroll.List,
		ActiveTab:   int(s.ActiveTab),
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app/ -run 'TestFromSession|TestToFromSession' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/session_map.go internal/app/session_map_test.go
git commit -m "feat(app): session <-> state mapping with enum clamping"
```

---

# Phase 4 — ui: `internal/ui/bubbletea` + `cmd`

> The ui layer is verified by a compiling smoke run + the manual sudo checklist, not unit tests (spec Testing). Keep each task's deliverable independently runnable via `go build ./...`.

### Task 21: Add dependencies

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add the Charm + bubblezone deps**

Run:
```bash
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/lipgloss@latest
go get github.com/charmbracelet/bubbles@latest
go get github.com/lrstanley/bubblezone@latest
go mod tidy
```
Expected: `go.mod` lists the four deps; `go build ./...` still succeeds (no ui code yet).

- [ ] **Step 2: Commit**

```bash
git add go.mod go.sum
git commit -m "build: add bubbletea, lipgloss, bubbles, bubblezone"
```

---

### Task 22: The Bubble Tea model skeleton + Cmd builders

**Files:**
- Create: `internal/ui/bubbletea/model.go`
- Create: `internal/ui/bubbletea/cmds.go`

**Interfaces:**
- Produces: `type Model struct` wrapping `app.AppState` + `*launchctl.Client` + a save-debounce channel; `New(st app.AppState, c *launchctl.Client) Model`; `Init()`, `Update()`, `View()`; and Cmd builders `pollCmd`, `detailCmd`, `logTailCmd`, `actionCmd`, `bootstrapCmd`, `sudoInspectCmd`, `sudoEnumerateCmd`, `sudoActionCmd` returning `tea.Cmd` that emit `app.Msg`s.

- [ ] **Step 1: Write the model skeleton**

`internal/ui/bubbletea/model.go`:
```go
package bubbletea

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/volkoffskij/launchdeck/internal/app"
	"github.com/volkoffskij/launchdeck/internal/launchctl"
	"github.com/volkoffskij/launchdeck/internal/session"
)

type Model struct {
	st       app.AppState
	client   *launchctl.Client
	width    int
	height   int
	pollBusy bool
	saveAt   time.Time // debounce marker
	dirty    bool
}

func New(st app.AppState, c *launchctl.Client) Model {
	return Model{st: st, client: c}
}

func (m Model) Init() tea.Cmd {
	zone.NewGlobal()
	return tea.Batch(pollCmd(m.client, m.st.UID), tea.EnterAltScreen)
}

// tickMsg is a local Bubble Tea message (tea.Msg is interface{}); it is handled
// by its own case in Update and is NOT an app.Msg.
type tickMsg struct{}

func tick() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m Model) Update(raw tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := raw.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.MouseMsg:
		return m.handleMouse(msg)
	case tickMsg:
		var cmds []tea.Cmd
		if !m.pollBusy {
			m.pollBusy = true
			cmds = append(cmds, pollCmd(m.client, m.st.UID))
		}
		cmds = append(cmds, tick())
		return m, tea.Batch(cmds...)
	case app.Msg:
		return m.applyIntent(msg)
	}
	return m, nil
}

// applyIntent runs reduce, then fires any Cmd the new state implies.
func (m Model) applyIntent(msg app.Msg) (tea.Model, tea.Cmd) {
	prevSel := m.st.Selected
	if _, ok := msg.(app.ServicesLoaded); ok {
		m.pollBusy = false
	}
	m.st = app.Reduce(msg, m.st)
	cmds := m.followUps(msg, prevSel)
	m.maybeSave()
	return m, tea.Batch(cmds...)
}
```

- [ ] **Step 2: Write the Cmd builders**

`internal/ui/bubbletea/cmds.go`:
```go
package bubbletea

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/volkoffskij/launchdeck/internal/app"
	"github.com/volkoffskij/launchdeck/internal/launchctl"
)

func pollCmd(c *launchctl.Client, uid int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		var all []launchctl.Service
		gui, err := c.ScanDomain(ctx, launchctl.GUIDomain(uid))
		if err != nil {
			if se, ok := err.(*launchctl.ScanError); ok {
				return app.ServicesLoaded{Err: se}
			}
			return app.ServicesLoaded{Err: &launchctl.ScanError{Kind: launchctl.FailureGeneric, Stderr: err.Error()}}
		}
		all = append(all, gui...)
		// system scan is best-effort; a permission error just omits system rows.
		if sys, serr := c.ScanDomain(ctx, launchctl.SystemDomain()); serr == nil {
			all = append(all, sys...)
		}
		return app.ServicesLoaded{Services: all}
	}
}

func detailCmd(c *launchctl.Client, d launchctl.Domain, label string) tea.Cmd {
	target := d.Target(label)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		det, err := c.Print(ctx, d, label)
		if err != nil {
			if se, ok := err.(*launchctl.ScanError); ok {
				return app.ServiceDetailLoaded{Target: target, Err: se}
			}
			return app.ServiceDetailLoaded{Target: target, Err: &launchctl.ScanError{Kind: launchctl.FailureGeneric, Stderr: err.Error()}}
		}
		return app.ServiceDetailLoaded{Target: target, Detail: det}
	}
}

func actionCmd(c *launchctl.Client, a launchctl.ActionKind, d launchctl.Domain, label string) tea.Cmd {
	target := d.Target(label)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		out := c.Action(ctx, a, d, label)
		timedOut := ctx.Err() == context.DeadlineExceeded
		return app.ActionResult{Action: a, Target: target, Outcome: out, TimedOut: timedOut}
	}
}

func bootstrapCmd(c *launchctl.Client, d launchctl.Domain, plist string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		out := c.Bootstrap(ctx, d, plist)
		return app.ActionResult{Action: launchctl.ActionLoad, Target: d.String(), Outcome: out, TimedOut: ctx.Err() == context.DeadlineExceeded}
	}
}
```
(The log-tail and sudo Cmds are added in Tasks 24–25.)

- [ ] **Step 3: Verify it compiles (stubs for `followUps`/`handleKey`/etc. come next)**

Add temporary stubs at the bottom of `model.go` so the package builds:
```go
func (m Model) followUps(app.Msg, string) []tea.Cmd { return nil }
func (m Model) maybeSave()                           {}
func (m Model) handleKey(tea.KeyMsg) (tea.Model, tea.Cmd)     { return m, nil }
func (m Model) handleMouse(tea.MouseMsg) (tea.Model, tea.Cmd) { return m, nil }
func (m Model) View() string                                 { return "" }
```
Run: `go build ./...`
Expected: builds.

- [ ] **Step 4: Commit**

```bash
git add internal/ui/bubbletea/model.go internal/ui/bubbletea/cmds.go
git commit -m "feat(ui): bubbletea model skeleton and core Cmd builders"
```

---

### Task 23: Follow-up Cmds (detail fetch on select, action run, save debounce)

**Files:**
- Modify: `internal/ui/bubbletea/model.go`

**Interfaces:**
- Replaces the `followUps` and `maybeSave` stubs. `followUps` fires: `detailCmd` when selection changed to a present service; `actionCmd`/`bootstrapCmd` when `ActionRunning` flipped true; re-fetch detail after an `ActionResult` for the selected service. `maybeSave` writes the session (debounced by comparing `ToSession`).

- [ ] **Step 1: Implement `followUps`**

Replace the `followUps` stub in `model.go`:
```go
func (m *Model) selectedService() (launchctl.Domain, string, bool) {
	for _, s := range m.st.Services {
		if s.Label == m.st.Selected {
			return s.Domain, s.Label, true
		}
	}
	return launchctl.Domain{}, "", false
}

func (m Model) followUps(msg app.Msg, prevSel string) []tea.Cmd {
	var cmds []tea.Cmd
	// Selection changed to a present, non-gone service → fetch detail + start tail.
	if m.st.Selected != prevSel && m.st.Selected != "" && !m.st.Gone {
		if d, label, ok := m.selectedService(); ok {
			cmds = append(cmds, detailCmd(m.client, d, label))
			cmds = append(cmds, logTailCmd(d, label, m.tailCancel())) // Task 24
		}
	}
	// A run just started (ActionRunning flipped by reduce): fire the launchctl Cmd.
	if m.st.ActionRunning && !actionAlreadyDispatched(msg) {
		if d, plist, ok := m.st.LoadTarget(); ok {
			cmds = append(cmds, bootstrapCmd(m.client, d, plist))
		} else if d, label, ok := m.selectedService(); ok {
			cmds = append(cmds, actionCmd(m.client, m.st.PendingAction(), d, label))
		}
	}
	// After an action on the selected service, re-fetch its detail (~2s freshness).
	if ar, ok := msg.(app.ActionResult); ok && !m.st.ActionRunning {
		if d, label, ok := m.selectedService(); ok && d.Target(label) == ar.Target {
			cmds = append(cmds, detailCmd(m.client, d, label))
		}
	}
	return cmds
}

// actionAlreadyDispatched avoids re-firing the Cmd on the same message that set
// ActionRunning=true when that message is itself the ActionResult finishing it.
func actionAlreadyDispatched(msg app.Msg) bool {
	switch msg.(type) {
	case app.ActionResult, app.ServicesLoaded, app.ServiceDetailLoaded, app.LogLinesAppended:
		return true
	default:
		return false
	}
}
```
Note: `tailCancel()` returns a per-tail cancel func; stub it as returning `nil` until Task 24 wires the tail goroutine:
```go
func (m Model) tailCancel() context.CancelFunc { return func() {} }
```

- [ ] **Step 2: Implement `maybeSave`**

Replace the `maybeSave` stub. Because `Update` returns a value receiver, hold the debounce state in the Model and persist on change:
```go
func (m *Model) maybeSave() {
	next := app.ToSession(m.st)
	if next == m.lastSaved {
		return
	}
	m.lastSaved = next
	if p, err := session.Path(); err == nil {
		_ = session.Save(p, next) // best-effort; a save error is non-fatal
	}
}
```
Add fields to `Model`: `lastSaved session.Session`. Change `applyIntent` to call `m.maybeSave()` on the pointer: since `Update` uses a value receiver and reassigns `m`, convert `maybeSave`/`followUps`/`selectedService` receivers to pointer as shown and call them on `&m`. Update `applyIntent`:
```go
	m.st = app.Reduce(msg, m.st)
	cmds := (&m).followUps(msg, prevSel)
	(&m).maybeSave()
	return m, tea.Batch(cmds...)
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: builds (View still returns "").

- [ ] **Step 4: Commit**

```bash
git add internal/ui/bubbletea/model.go
git commit -m "feat(ui): follow-up Cmds for detail fetch, action run, session save"
```

---

### Task 24: Log tail Cmd (initial buffer + follow + file states)

**Files:**
- Create: `internal/ui/bubbletea/tail.go`
- Modify: `internal/ui/bubbletea/model.go` (real `tailCancel`)

**Interfaces:**
- Produces: `logTailCmd(d launchctl.Domain, label string, out chan<- app.Msg, ...)`. Reads the initial buffer (last 64 KB then last 500 lines per path), interleaves `[out]`/`[err]`, then follows, emitting `app.LogLinesAppended{TailTarget}`. Cancellable; on cancel closes files and stops.

- [ ] **Step 1: Implement the tail**

`internal/ui/bubbletea/tail.go`:
```go
package bubbletea

import (
	"bufio"
	"context"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/volkoffskij/launchdeck/internal/app"
	"github.com/volkoffskij/launchdeck/internal/launchctl"
)

const (
	tailWindowBytes = 64 * 1024
	tailInitLines   = 500
)

// initialBuffer reads the last tailWindowBytes of a path and keeps the last
// tailInitLines of that read.
func initialBuffer(path, stream string) []app.LogLine {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil
	}
	start := int64(0)
	if info.Size() > tailWindowBytes {
		start = info.Size() - tailWindowBytes
	}
	if _, err := f.Seek(start, 0); err != nil {
		return nil
	}
	var lines []app.LogLine
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), tailWindowBytes)
	for sc.Scan() {
		lines = append(lines, app.LogLine{Stream: stream, Text: sc.Text()})
	}
	if len(lines) > tailInitLines {
		lines = lines[len(lines)-tailInitLines:]
	}
	return lines
}

// logTailCmd emits an initial LogLinesAppended, then a Cmd that polls the files
// for growth every 500ms until ctx is cancelled. (A poll loop is simpler and
// robust to rotation/truncation than an fsnotify follow — ponytail: upgrade to
// fsnotify only if 500ms feels laggy.)
func logTailCmd(ctx context.Context, d launchctl.Domain, det launchctl.ServiceDetail) tea.Cmd {
	target := d.Target(det.Label)
	return func() tea.Msg {
		paths := logPaths(det)
		if len(paths) == 0 {
			return app.LogLinesAppended{TailTarget: target, State: "removed"}
		}
		var initial []app.LogLine
		for _, p := range paths {
			initial = append(initial, initialBuffer(p.path, p.stream)...)
		}
		return app.LogLinesAppended{TailTarget: target, Lines: initial}
	}
}

type logPath struct {
	path, stream string
}

func logPaths(det launchctl.ServiceDetail) []logPath {
	out := det.StdoutPath
	errp := det.StderrPath
	switch {
	case out != "" && errp != "" && out == errp:
		return []logPath{{out, "out"}}
	case out != "" && errp != "":
		return []logPath{{out, "out"}, {errp, "err"}}
	case out != "":
		return []logPath{{out, "out"}}
	case errp != "":
		return []logPath{{errp, "err"}}
	default:
		return nil
	}
}

var _ = strings.TrimSpace
var _ = time.Second
```
Note: this task ships the **initial buffer**; live-follow growth is a `tea.Tick`-driven re-read added in a follow-up if needed. `logTailCmd` takes the fetched `ServiceDetail` (which carries the log paths), so the tail starts after the detail fetch resolves. Adjust `followUps` (Task 23) to start the tail from the `ServiceDetailLoaded` handler instead of on select:
- In `followUps`, remove the `logTailCmd` call from the selection branch; add: when `msg` is `app.ServiceDetailLoaded` for the current target and `LoadState == DetailReady`, append `logTailCmd(context.Background(), d, m.st.Detail.Metadata)`.

- [ ] **Step 2: Simplify `tailCancel`**

Since the tail now emits a one-shot initial buffer (no long-lived goroutine yet), replace `tailCancel()` usage: delete the `tailCancel` stub and the earlier `logTailCmd(... m.tailCancel())` call (already removed in Step 1). Set `m.st.TailIdentity` when starting a tail by adding a small local field update in the `ServiceDetailLoaded` branch.

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: builds.

- [ ] **Step 4: Commit**

```bash
git add internal/ui/bubbletea/tail.go internal/ui/bubbletea/model.go
git commit -m "feat(ui): log tail initial buffer with out/err interleave"
```

---

### Task 25: sudo Cmds (ExecProcess action retry + captured inspect/enumerate)

**Files:**
- Create: `internal/ui/bubbletea/sudo.go`
- Modify: `internal/ui/bubbletea/model.go` (dispatch on `ConfirmSudo`)

**Interfaces:**
- Produces: `sudoActionCmd` (via `tea.ExecProcess("sudo","launchctl",...)`, fire-and-forget → `ActionResult`), `sudoInspectCmd` (captured `sudo launchctl print <target>` → `ServiceDetailLoaded`), `sudoEnumerateCmd` (captured `sudo launchctl print system` → `ServicesLoaded`). Dispatch in `followUps` when `msg` is `app.ConfirmSudo` by `PendingSudo.Kind`.

- [ ] **Step 1: Implement the sudo Cmds**

`internal/ui/bubbletea/sudo.go`:
```go
package bubbletea

import (
	"bytes"
	"context"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/volkoffskij/launchdeck/internal/app"
	"github.com/volkoffskij/launchdeck/internal/launchctl"
)

// sudoActionCmd suspends the TUI, lets sudo prompt on the real tty, runs the
// action, and resumes. Fire-and-forget: the password never touches our code.
func sudoActionCmd(a launchctl.ActionKind, target string, argv []string) tea.Cmd {
	c := exec.Command("sudo", append([]string{"launchctl"}, argv...)...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		out := launchctl.ActionOutcome{}
		if err != nil {
			out.ExitCode = 1
			out.Stderr = err.Error()
		}
		return app.ActionResult{Action: a, Target: target, Outcome: out}
	})
}

// sudoInspectCmd captures stdout (sudo prompts on the tty, not stdout).
func sudoInspectCmd(d launchctl.Domain, label string) tea.Cmd {
	target := d.Target(label)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "sudo", "launchctl", "print", target)
		var out, errb bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errb
		if err := cmd.Run(); err != nil {
			return app.ServiceDetailLoaded{Target: target, Err: &launchctl.ScanError{Kind: launchctl.FailureGeneric, Stderr: errb.String()}}
		}
		// parseServiceDetail is unexported; expose a helper (Task 25b) or re-print via client.
		return app.ServiceDetailLoaded{Target: target, Detail: launchctl.ParseDetail(out.String(), launchctl.Service{Label: label, Domain: d})}
	}
}
```
This needs an **exported** parser. Add to `internal/launchctl/parse.go`:
```go
// ParseDetail is the exported entry point for callers that already captured a
// `launchctl print <domain>/<label>` dump (e.g. the sudo inspect retry).
func ParseDetail(dump string, svc Service) ServiceDetail { return parseServiceDetail(dump, svc) }
```
Similarly add `func ParseScan(dump string, d Domain) ([]Service, error) { return parseDomainScan(dump, d) }` and a `sudoEnumerateCmd` that runs `sudo launchctl print system`, parses via `ParseScan`, and returns `app.ServicesLoaded{Services: merged}` (merge = replace prior system rows; simplest correct: return only the parsed system rows and let the next normal poll re-merge — acceptable per spec).

- [ ] **Step 2: Dispatch on ConfirmSudo in `followUps`**

Add to `followUps`:
```go
	if _, ok := msg.(app.ConfirmSudo); ok && m.st.PendingSudo.Active {
		ps := m.st.PendingSudo
		switch ps.Kind {
		case app.SudoAction:
			if d, label, ok := m.selectedService(); ok {
				cmds = append(cmds, sudoActionCmd(m.st.PendingAction(), ps.Target, actionArgvFor(m.st.PendingAction(), d.Target(label))))
			}
		case app.SudoInspect:
			if d, label, ok := m.selectedService(); ok {
				cmds = append(cmds, sudoInspectCmd(d, label))
			}
		case app.SudoEnumerate:
			cmds = append(cmds, sudoEnumerateCmd())
		}
	}
```
Expose `actionArgvFor` from `launchctl` (export `actionArgs` as `ActionArgs`) so the ui can build the sudo argv.

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: builds. Run `go test ./internal/launchctl/ ./internal/app/` to confirm the exported wrappers didn't break unit tests. Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/ui/bubbletea/sudo.go internal/launchctl/parse.go internal/launchctl/client.go internal/ui/bubbletea/model.go
git commit -m "feat(ui): sudo action/inspect/enumerate retry commands"
```

---

### Task 26: Rendering — layout, list, detail, statusbar, min-size gate

**Files:**
- Create: `internal/ui/bubbletea/view.go`, `list.go`, `detail.go`, `statusbar.go`

**Interfaces:**
- Produces: `func (m Model) View() string` that computes the sidebar/detail/status layout from `m.width/m.height`, gates on the 60×20 minimum, marks bubblezones for rows/buttons/tabs, and renders `app.Derive(m.st)`.

- [ ] **Step 1: Write the top-level view + min-size gate**

Replace the `View` stub in `model.go` with a call into `view.go`. `internal/ui/bubbletea/view.go`:
```go
package bubbletea

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/volkoffskij/launchdeck/internal/app"
)

const (
	minWidth  = 60
	minHeight = 20
)

func (m Model) render() string {
	if m.width < minWidth || m.height < minHeight {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			"terminal too small (need ≥60×20)")
	}
	vm := app.Derive(m.st)
	sidebarW := clampInt(int(float64(m.width)*0.33), 24, 48)
	detailW := m.width - sidebarW - 1
	bodyH := m.height - 1 // status row

	sidebar := renderList(vm.List, sidebarW, bodyH, m.st.Focus == app.FocusSidebar)
	detail := renderDetail(vm.Detail, detailW, bodyH)
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, " ", detail)
	status := renderStatus(vm.Status, m.width)
	return zone.Scan(lipgloss.JoinVertical(lipgloss.Left, body, status))
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

var _ = strings.TrimSpace
```
Change `model.go`'s `View`:
```go
func (m Model) View() string { return m.render() }
```

- [ ] **Step 2: Write the list, detail, and statusbar renderers**

`internal/ui/bubbletea/list.go`:
```go
package bubbletea

import (
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/volkoffskij/launchdeck/internal/app"
)

func renderList(vm app.ListVM, w, h int, focused bool) string {
	border := lipgloss.NormalBorder()
	style := lipgloss.NewStyle().Width(w).Height(h).Border(border)
	if vm.Placeholder != "" {
		return style.Render(vm.Placeholder)
	}
	var b []string
	for _, r := range vm.Rows {
		dot := "○"
		if r.Running {
			dot = "●"
		}
		line := dot + " " + ellipsize(r.Label, w-4)
		if r.Gone {
			line += " (gone)"
		}
		row := lipgloss.NewStyle().Render(line)
		if r.Selected {
			row = lipgloss.NewStyle().Reverse(true).Render(line)
		}
		b = append(b, zone.Mark("row:"+r.Label, row))
	}
	return style.Render(lipgloss.JoinVertical(lipgloss.Left, b...))
}

func ellipsize(s string, max int) string {
	if max < 1 {
		max = 1
	}
	if len(s) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return s[:max-1] + "…"
}
```

`internal/ui/bubbletea/detail.go`:
```go
package bubbletea

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/volkoffskij/launchdeck/internal/app"
)

func renderDetail(vm app.DetailVM, w, h int) string {
	style := lipgloss.NewStyle().Width(w).Height(h).Border(lipgloss.NormalBorder())
	if vm.Mode == "empty" {
		return style.Render("Select a service")
	}
	tabs := renderTabs(vm.ActiveTab)
	var body string
	switch vm.ActiveTab {
	case app.TabMetadata:
		if vm.Mode == "loading" {
			body = "Loading detail…"
		} else if vm.Mode == "error" {
			body = vm.Err
		} else {
			body = strings.Join([]string{
				"label:     " + vm.Label,
				"domain:    " + vm.Domain,
				"pid:       " + vm.PID,
				"last exit: " + vm.LastExit,
				"run:       " + vm.RunState,
				"enable:    " + vm.EnableState,
				"program:   " + vm.Program,
				"plist:     " + vm.Plist,
			}, "\n")
		}
	case app.TabLogs:
		if vm.LogNote != "" {
			body = vm.LogNote
		} else {
			body = strings.Join(vm.LogLines, "\n")
		}
	case app.TabRaw:
		body = vm.Raw
	}
	if vm.Mode == "gone" {
		body = "(gone) — service no longer present\n\n" + body
	}
	return style.Render(tabs + "\n" + body)
}

func renderTabs(active app.Tab) string {
	names := []string{"Metadata", "Logs", "Raw"}
	var out []string
	for i, n := range names {
		s := " " + n + " "
		if app.Tab(i) == active {
			s = lipgloss.NewStyle().Reverse(true).Render(s)
		}
		out = append(out, zone.Mark("tab:"+n, s))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, out...)
}
```

`internal/ui/bubbletea/statusbar.go`:
```go
package bubbletea

import (
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/volkoffskij/launchdeck/internal/app"
)

func renderStatus(vm app.StatusVM, w int) string {
	if vm.Prompt != "" {
		return lipgloss.NewStyle().Width(w).Reverse(true).Render(vm.Prompt)
	}
	var btns []string
	for _, b := range vm.Buttons {
		btns = append(btns, zone.Mark("btn:"+b, "["+b+"]"))
	}
	line := lipgloss.JoinHorizontal(lipgloss.Top, btns...)
	if vm.Message != "" {
		line += "  " + vm.Message
	}
	return lipgloss.NewStyle().Width(w).Render(line)
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: builds.

- [ ] **Step 4: Commit**

```bash
git add internal/ui/bubbletea/view.go internal/ui/bubbletea/list.go internal/ui/bubbletea/detail.go internal/ui/bubbletea/statusbar.go internal/ui/bubbletea/model.go
git commit -m "feat(ui): render layout, list, detail tabs, status bar"
```

---

### Task 27: Keymap → intents (with modal suppression)

**Files:**
- Create: `internal/ui/bubbletea/keys.go`
- Modify: `internal/ui/bubbletea/model.go` (`handleKey` → dispatch)

**Interfaces:**
- Produces: `func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd)` mapping keys to `app.Msg` per the spec Keymap, honoring modal suppression (filter/load/picker/confirm/sudo open → only that modal's keys live). Emits `tea.Quit` on `Quit` after a final save.

- [ ] **Step 1: Implement `handleKey`**

`internal/ui/bubbletea/keys.go`:
```go
package bubbletea

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/volkoffskij/launchdeck/internal/app"
	"github.com/volkoffskij/launchdeck/internal/launchctl"
)

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	st := m.st

	// Modal suppression: only the open modal's keys are live.
	switch {
	case st.FilterEditing:
		return m.filterKey(msg)
	case st.LoadPrompt.Open:
		return m.loadKey(msg)
	case st.ActionPicker.Open:
		return m.pickerKey(k)
	case st.PendingConfirm.Active || st.PendingSudo.Active:
		return m.promptKey(k)
	}

	// Global keys.
	switch k {
	case "q", "ctrl+c":
		(&m).maybeSaveFinal()
		return m, tea.Quit
	case "up", "k":
		if st.Focus == app.FocusSidebar {
			return m.applyIntent(app.MoveSelection{Delta: -1})
		}
		return m.applyIntent(app.ScrollMsg{Panel: app.FocusDetail, Delta: -1})
	case "down", "j":
		if st.Focus == app.FocusSidebar {
			return m.applyIntent(app.MoveSelection{Delta: 1})
		}
		return m.applyIntent(app.ScrollMsg{Panel: app.FocusDetail, Delta: 1})
	case "home":
		return m.applyIntent(app.MoveSelection{ToTop: true})
	case "end":
		return m.applyIntent(app.MoveSelection{ToBottom: true})
	case "pgup":
		return m.applyIntent(app.MoveSelection{Delta: -10})
	case "pgdown":
		return m.applyIntent(app.MoveSelection{Delta: 10})
	case "tab":
		return m.applyIntent(app.FocusPanel{})
	case "1":
		return m.applyIntent(app.SetTab{Tab: app.TabMetadata})
	case "2":
		return m.applyIntent(app.SetTab{Tab: app.TabLogs})
	case "3":
		return m.applyIntent(app.SetTab{Tab: app.TabRaw})
	case "left":
		return m.applyIntent(app.SetTab{Tab: prevTab(st.ActiveTab)})
	case "right":
		return m.applyIntent(app.SetTab{Tab: nextTab(st.ActiveTab)})
	case "a":
		return m.applyIntent(app.OpenActionPicker{})
	case "/":
		return m.applyIntent(app.OpenFilter{})
	case "d":
		return m.applyIntent(app.CycleDomainScope{})
	case "s":
		return m.applyIntent(app.SetSort{})
	case "S":
		return m.applyIntent(app.SetSort{ToggleDir: true})
	case "L":
		return m.applyIntent(app.OpenLoadPrompt{})
	case "r":
		return m, pollCmd(m.client, m.st.UID)
	}
	return m, nil
}

func prevTab(t app.Tab) app.Tab {
	if t == app.TabMetadata {
		return app.TabRaw
	}
	return t - 1
}
func nextTab(t app.Tab) app.Tab {
	if t == app.TabRaw {
		return app.TabMetadata
	}
	return t + 1
}

func (m Model) filterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		return m.applyIntent(app.CommitFilter{})
	case "esc":
		return m.applyIntent(app.CancelFilter{})
	case "ctrl+u":
		return m.applyIntent(app.SetFilterBuffer{Buffer: ""})
	case "backspace":
		b := m.st.FilterBuffer
		if b != "" {
			b = b[:len(b)-1]
		}
		return m.applyIntent(app.SetFilterBuffer{Buffer: b})
	default:
		if len(msg.Runes) == 1 {
			return m.applyIntent(app.SetFilterBuffer{Buffer: m.st.FilterBuffer + string(msg.Runes)})
		}
	}
	return m, nil
}

func (m Model) loadKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		return m.applyIntent(app.SubmitLoad{})
	case "esc":
		return m.applyIntent(app.CancelLoad{})
	case "backspace":
		b := m.st.LoadPrompt.Buffer
		if b != "" {
			b = b[:len(b)-1]
		}
		return m.applyIntent(app.SetLoadBuffer{Buffer: b})
	default:
		if len(msg.Runes) == 1 {
			return m.applyIntent(app.SetLoadBuffer{Buffer: m.st.LoadPrompt.Buffer + string(msg.Runes)})
		}
	}
	return m, nil
}

func (m Model) pickerKey(k string) (tea.Model, tea.Cmd) {
	verbs := map[string]launchctl.ActionKind{
		"s": launchctl.ActionStart, "r": launchctl.ActionRestart, "k": launchctl.ActionStop,
		"e": launchctl.ActionEnable, "d": launchctl.ActionDisable, "u": launchctl.ActionUnload,
	}
	switch k {
	case "esc":
		return m.applyIntent(app.CancelActionPicker{})
	case "up":
		return m.applyIntent(app.MoveActionPicker{Delta: -1})
	case "down":
		return m.applyIntent(app.MoveActionPicker{Delta: 1})
	case "enter":
		return m.applyIntent(app.PickAction{Action: m.st.ActionPicker.HighlightedVerb})
	}
	if v, ok := verbs[k]; ok {
		return m.applyIntent(app.PickAction{Action: v})
	}
	return m, nil
}

func (m Model) promptKey(k string) (tea.Model, tea.Cmd) {
	yes := k == "y" || k == "enter"
	no := k == "n" || k == "esc"
	if m.st.PendingConfirm.Active {
		if yes {
			return m.applyIntent(app.ConfirmAction{})
		}
		if no {
			return m.applyIntent(app.CancelAction{})
		}
	}
	if m.st.PendingSudo.Active {
		if yes {
			return m.applyIntent(app.ConfirmSudo{})
		}
		if no {
			return m.applyIntent(app.CancelSudo{})
		}
	}
	return m, nil
}
```
Add `maybeSaveFinal` to `model.go` (forces a save regardless of debounce):
```go
func (m *Model) maybeSaveFinal() {
	if p, err := session.Path(); err == nil {
		_ = session.Save(p, app.ToSession(m.st))
	}
}
```
Change `handleKey`'s receiver usage: it calls `m.applyIntent`, which returns `(tea.Model, tea.Cmd)`; that's fine. Ensure `applyIntent` has a value receiver returning the updated model.

- [ ] **Step 2: Verify it compiles**

Run: `go build ./...`
Expected: builds.

- [ ] **Step 3: Commit**

```bash
git add internal/ui/bubbletea/keys.go internal/ui/bubbletea/model.go
git commit -m "feat(ui): keymap to intents with modal suppression"
```

---

### Task 28: Mouse → intents (bubblezone hit-testing, modal-suppressed)

**Files:**
- Create: `internal/ui/bubbletea/mouse.go`
- Modify: `internal/ui/bubbletea/model.go` (`handleMouse` → dispatch)

**Interfaces:**
- Produces: `func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd)`. A left click hit-tests zones (`row:`, `tab:`, `btn:`); wheel scroll targets the hovered zone. While any modal is open, non-modal clicks/scroll are suppressed.

- [ ] **Step 1: Implement `handleMouse`**

`internal/ui/bubbletea/mouse.go`:
```go
package bubbletea

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/volkoffskij/launchdeck/internal/app"
	"github.com/volkoffskij/launchdeck/internal/launchctl"
)

func modalOpen(s app.AppState) bool {
	return s.FilterEditing || s.LoadPrompt.Open || s.ActionPicker.Open ||
		s.PendingConfirm.Active || s.PendingSudo.Active
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if modalOpen(m.st) {
		return m, nil // mouse is modal-suppressed
	}
	switch msg.Action {
	case tea.MouseActionPress:
		if msg.Button == tea.MouseButtonWheelUp {
			return m.applyIntent(app.ScrollMsg{Panel: hoveredPanel(msg), Delta: -3})
		}
		if msg.Button == tea.MouseButtonWheelDown {
			return m.applyIntent(app.ScrollMsg{Panel: hoveredPanel(msg), Delta: 3})
		}
		if msg.Button != tea.MouseButtonLeft {
			return m, nil
		}
		if id, ok := hitZone(msg, "row:"); ok {
			return m.applyIntent(app.SelectService{Label: id})
		}
		if id, ok := hitZone(msg, "tab:"); ok {
			return m.applyIntent(app.SetTab{Tab: tabByName(id)})
		}
		if id, ok := hitZone(msg, "btn:"); ok {
			return m.applyIntent(app.RunAction{Action: actionByName(id)})
		}
	}
	return m, nil
}

func hitZone(msg tea.MouseMsg, prefix string) (string, bool) {
	for _, z := range zone.GetAll() {
		if strings.HasPrefix(z.ID(), prefix) && z.InBounds(msg) {
			return strings.TrimPrefix(z.ID(), prefix), true
		}
	}
	return "", false
}

func hoveredPanel(msg tea.MouseMsg) app.Focus {
	if _, ok := hitZone(msg, "row:"); ok {
		return app.FocusSidebar
	}
	return app.FocusDetail
}

func tabByName(n string) app.Tab {
	switch n {
	case "Logs":
		return app.TabLogs
	case "Raw":
		return app.TabRaw
	default:
		return app.TabMetadata
	}
}

func actionByName(n string) launchctl.ActionKind {
	switch n {
	case "Restart":
		return launchctl.ActionRestart
	case "Stop":
		return launchctl.ActionStop
	case "Enable":
		return launchctl.ActionEnable
	case "Disable":
		return launchctl.ActionDisable
	case "Unload":
		return launchctl.ActionUnload
	default:
		return launchctl.ActionStart
	}
}
```
Note: confirm the bubblezone API names (`zone.GetAll`, `z.ID()`, `z.InBounds`) against the installed version; if the helper differs, use `zone.Get(id).InBounds(msg)` per known ids instead. Adjust imports accordingly.

- [ ] **Step 2: Enable mouse in the program (Task 29 wires `tea.WithMouseAllMotion`)**

- [ ] **Step 3: Verify it compiles**

Run: `go build ./...`
Expected: builds.

- [ ] **Step 4: Commit**

```bash
git add internal/ui/bubbletea/mouse.go
git commit -m "feat(ui): mouse hit-testing to intents, modal-suppressed"
```

---

### Task 29: `main` — startup checks, session load, signal handling, run

**Files:**
- Create: `cmd/launchdeck/main.go`

**Interfaces:**
- Consumes: `launchctl.New`, `session.Path/Load`, `app.FromSession`, `bubbletea.New`, `tea.NewProgram`.

- [ ] **Step 1: Write `main.go`**

`cmd/launchdeck/main.go`:
```go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/volkoffskij/launchdeck/internal/app"
	"github.com/volkoffskij/launchdeck/internal/launchctl"
	"github.com/volkoffskij/launchdeck/internal/session"
	ui "github.com/volkoffskij/launchdeck/internal/ui/bubbletea"
)

func main() {
	if runtime.GOOS != "darwin" {
		fmt.Fprintln(os.Stderr, "launchdeck: macOS only")
		os.Exit(1)
	}
	if _, err := exec.LookPath("launchctl"); err != nil {
		fmt.Fprintln(os.Stderr, "launchdeck: launchctl not found in PATH")
		os.Exit(1)
	}

	uid := os.Getuid()
	var st app.AppState
	if p, err := session.Path(); err == nil {
		st = app.FromSession(session.Load(p), uid)
	} else {
		st = app.NewState(uid)
	}

	m := ui.New(st, launchctl.New())
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseAllMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "launchdeck:", err)
		os.Exit(1)
	}
}
```
Bubble Tea already saves on `q`/`ctrl+c` via `maybeSaveFinal` in `handleKey`. (SIGINT that Bubble Tea converts to a `ctrl+c` KeyMsg routes through the same save; the ExecProcess exception holds because during a sudo retry the child owns the tty.)

- [ ] **Step 2: Build and smoke-run**

Run:
```bash
go build ./...
go vet ./...
go run ./cmd/launchdeck
```
Expected: the TUI launches, shows "Loading services…" then a live list; arrow keys move selection; `1/2/3` switch tabs; `q` quits and writes `~/.config/launchdeck/session.json`. (Run interactively on a Mac.)

- [ ] **Step 3: Commit**

```bash
git add cmd/launchdeck/main.go
git commit -m "feat(cmd): launchdeck entry point with startup checks and mouse"
```

---

### Task 30: Full test sweep + README smoke checklist

**Files:**
- Create: `README.md`

- [ ] **Step 1: Run the whole test suite**

Run: `go test ./...`
Expected: all unit tests PASS; integration tests SKIP unless on darwin / opted-in.

- [ ] **Step 2: Run the Tier-2 mutating test on a Mac**

Run: `LAUNCHDECK_INTEGRATION=1 go test ./internal/launchctl/ -run TestIntegrationActionRoundTrip -v`
Expected: PASS on macOS.

- [ ] **Step 3: Write the README with the manual sudo checklist**

`README.md` (short): what it is, `go build -o launchdeck ./cmd/launchdeck`, keymap table, and the **Manual sudo checklist** from the spec (system-domain action → permission detected → Retry with sudo → password → success/refresh; plus cancel, wrong-password, and Ctrl-C-at-prompt paths clear `pendingSudo` and never lose state).

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: README with build, keymap, and manual sudo checklist"
```

---

## Self-Review Notes

- **Spec coverage:** Goals 1–5 map to Tasks 10/12 (list+filter), 11/17 (select+tabs), 13/14/25 (actions+sudo), 19/20/23 (persistence), 28 (mouse). Data Flow → Tasks 22–24. Keymap → 27. Layout/resize → 26. Scope Filter → 9/12. Sorting → 9. Error Handling → 4/10/15. Testing Tier 0 → app+launchctl tests; Tier 1/2 → Task 6; Manual → Task 30.
- **Deferred simplifications (revisit if the smoke run demands it):** the log tail ships an initial buffer only (live-follow growth via a `tea.Tick` re-read is a small add — Task 24 note); `sudoEnumerateCmd` returns parsed system rows and lets the next poll re-merge rather than merging in place. Both are spec-acceptable and isolated to the ui layer.
- **Known API-verification points during build:** exact bubblezone hit-test API (Task 28), `tea.MouseMsg` field names for the installed Bubble Tea version (Task 28), and `tea.ExecProcess` signature (Task 25). Each is called out inline where it matters.
