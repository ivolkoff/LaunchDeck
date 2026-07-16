package app

import (
	"fmt"
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
		big[i] = LogLine{Stream: "out", Text: fmt.Sprintf("%d", i)}
	}
	s = Reduce(LogLinesAppended{TailTarget: "gui/501/com.a", Lines: big}, s)
	if len(s.LogRing) != logRingCap {
		t.Fatalf("ring should cap at %d, got %d", logRingCap, len(s.LogRing))
	}
	if want := "10"; s.LogRing[0].Text != want {
		t.Fatalf("ring should keep newest, first surviving line = %q, want %q", s.LogRing[0].Text, want)
	}
	if want := fmt.Sprintf("%d", logRingCap+9); s.LogRing[len(s.LogRing)-1].Text != want {
		t.Fatalf("ring should keep newest, last surviving line = %q, want %q", s.LogRing[len(s.LogRing)-1].Text, want)
	}
}

func TestDetailLoadedError(t *testing.T) {
	s := selected(t)
	s.Detail.LoadState = DetailLoading
	s = Reduce(ServiceDetailLoaded{
		Target: "gui/501/com.a",
		Err:    &launchctl.ScanError{Kind: launchctl.FailurePermission, Stderr: "denied"},
	}, s)
	if s.Detail.LoadState != DetailError || s.Detail.ErrMsg != "requires sudo to inspect" {
		t.Fatalf("permission error should map to sudo message: %+v", s.Detail)
	}

	s2 := selected(t)
	s2.Detail.LoadState = DetailLoading
	s2 = Reduce(ServiceDetailLoaded{
		Target: "gui/501/com.a",
		Err:    &launchctl.ScanError{Kind: launchctl.FailureGeneric, Stderr: "boom"},
	}, s2)
	if s2.Detail.LoadState != DetailError || s2.Detail.ErrMsg != "boom" {
		t.Fatalf("generic error should surface stderr: %+v", s2.Detail)
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
