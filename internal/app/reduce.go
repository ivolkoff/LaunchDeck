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
		f.DomainScope = (f.DomainScope + 1) % 3
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
	}
	return s
}

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
