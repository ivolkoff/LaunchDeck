package app

import (
	"testing"

	"github.com/ivolkoff/launchdeck/internal/i18n"
	"github.com/ivolkoff/launchdeck/internal/launchctl"
)

func TestDeriveRussianPlaceholder(t *testing.T) {
	i18n.SetLang(i18n.Ru)
	t.Cleanup(func() { i18n.SetLang(i18n.En) })

	s := NewState(501) // FirstScanDone == false → loading placeholder
	if got := Derive(s).List.Placeholder; got != "Загрузка сервисов…" {
		t.Errorf("ru placeholder = %q", got)
	}
}

func TestReduceRussianActionOK(t *testing.T) {
	i18n.SetLang(i18n.Ru)
	t.Cleanup(func() { i18n.SetLang(i18n.En) })

	s := NewState(501)
	s.ActionRunning = true
	out := Reduce(ActionResult{
		Action:  launchctl.ActionRestart,
		Outcome: launchctl.ActionOutcome{ExitCode: 0}, // OK() true
	}, s)
	if out.StatusMsg != "перезапуск: ок" {
		t.Errorf("ru action ok = %q, want %q", out.StatusMsg, "перезапуск: ок")
	}
}

func TestStartActionRussianVerb(t *testing.T) {
	// The action-start status ("restart…") must localize its verb too.
	if got := startAction(launchctl.ActionRestart, NewState(501)).StatusMsg; got != "restart…" {
		t.Errorf("en start = %q, want %q", got, "restart…")
	}
	i18n.SetLang(i18n.Ru)
	t.Cleanup(func() { i18n.SetLang(i18n.En) })
	if got := startAction(launchctl.ActionRestart, NewState(501)).StatusMsg; got != "перезапуск…" {
		t.Errorf("ru start = %q, want %q", got, "перезапуск…")
	}
}
