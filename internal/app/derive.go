package app

import (
	"fmt"

	"github.com/volkoffskij/launchdeck/internal/i18n"
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
		return ListVM{Placeholder: i18n.T("list.loading")}
	}
	vis := s.visible()
	if len(vis) == 0 {
		return ListVM{Placeholder: i18n.T("list.empty")}
	}
	viewportH := s.ListViewportH
	if viewportH < 1 {
		viewportH = 1
	}
	maxStart := len(vis) - viewportH
	if maxStart < 0 {
		maxStart = 0
	}
	start := s.Scroll.List
	if start < 0 {
		start = 0
	} else if start > maxStart {
		start = maxStart
	}
	end := start + viewportH
	if end > len(vis) {
		end = len(vis)
	}
	window := vis[start:end]
	vm := ListVM{Rows: make([]RowVM, len(window)), SelectedIdx: -1}
	for i, sv := range window {
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
		d.RunState = i18n.T("runstate.running")
	} else {
		d.PID = "-"
		d.RunState = i18n.T("runstate.stopped")
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
		return i18n.T("enable.enabled")
	case launchctl.Disabled:
		return i18n.T("enable.disabled")
	default:
		return "?"
	}
}

func deriveLog(s AppState) ([]string, string) {
	if len(s.LogRing) == 0 {
		if s.Detail.Metadata.StdoutPath == "" && s.Detail.Metadata.StderrPath == "" {
			return nil, i18n.T("log.none")
		}
		return nil, ""
	}
	// Newest first: the ring appends chronologically (oldest .. newest), so
	// reverse it for display — the freshest line sits at the top of the panel.
	n := len(s.LogRing)
	out := make([]string, n)
	for i, l := range s.LogRing {
		out[n-1-i] = "[" + l.Stream + "] " + l.Text
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
		st.Prompt = i18n.Tf("prompt.confirm", verb(s.PendingConfirm.Action), labelOf(s.PendingConfirm.Target))
	case s.PendingSudo.Active:
		st.Prompt = i18n.T("prompt.sudo")
	case s.FilterEditing:
		st.Prompt = i18n.T("prompt.filter") + s.FilterBuffer
	case s.LoadPrompt.Open:
		st.Prompt = i18n.T("prompt.load") + s.LoadPrompt.Buffer
	case s.ActionPicker.Open:
		st.Prompt = i18n.Tf("prompt.action", verb(s.ActionPicker.HighlightedVerb))
	}
	return st
}
