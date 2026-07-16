package bubbletea

import (
	"os/exec"

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
