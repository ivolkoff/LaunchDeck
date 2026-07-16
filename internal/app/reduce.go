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
