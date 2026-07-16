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

// ActionArgs builds the launchctl argv for a, given target ("<domain>/<label>").
func ActionArgs(a ActionKind, target string) []string {
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
	args := ActionArgs(a, d.Target(label))
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
