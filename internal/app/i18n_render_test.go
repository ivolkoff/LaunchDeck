package app

import (
	"testing"

	"github.com/volkoffskij/launchdeck/internal/i18n"
	"github.com/volkoffskij/launchdeck/internal/launchctl"
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
