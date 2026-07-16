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
		return app.ServiceDetailLoaded{Target: target, Detail: launchctl.ParseDetail(out.String(), launchctl.Service{Label: label, Domain: d})}
	}
}
