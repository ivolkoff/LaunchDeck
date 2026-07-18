package bubbletea

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ivolkoff/launchdeck/internal/app"
	"github.com/ivolkoff/launchdeck/internal/launchctl"
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
