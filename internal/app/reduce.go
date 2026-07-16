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
	case OpenActionPicker:
		if s.Selected == "" || s.Gone {
			return s
		}
		s.ActionPicker = ActionPicker{Open: true, HighlightedVerb: launchctl.ActionStart}
		return s
	case MoveActionPicker:
		if s.ActionPicker.Open {
			s.ActionPicker.HighlightedVerb = cyclePickerVerb(s.ActionPicker.HighlightedVerb, msg.Delta)
		}
		return s
	case PickAction:
		s.ActionPicker = ActionPicker{}
		return reduceRunAction(msg.Action, s)
	case CancelActionPicker:
		s.ActionPicker = ActionPicker{}
		return s
	case RunAction:
		return reduceRunAction(msg.Action, s)
	case ConfirmAction:
		if !s.PendingConfirm.Active {
			return s
		}
		act := s.PendingConfirm.Action
		s.PendingConfirm = PendingConfirm{}
		return startAction(act, s)
	case CancelAction:
		s.PendingConfirm = PendingConfirm{}
		return s
	case ActionResult:
		s.ActionRunning = false
		if msg.TimedOut {
			s.StatusMsg = msg.Action.String() + " timed out"
			return s
		}
		if msg.Outcome.OK() {
			s.StatusMsg = msg.Action.String() + " ok"
			return s
		}
		if msg.Outcome.Kind == launchctl.FailurePermission {
			s.PendingSudo = PendingSudo{Active: true, Kind: SudoAction, Target: msg.Target}
			s.StatusMsg = msg.Action.String() + " needs sudo — Retry with sudo"
			return s
		}
		s.StatusMsg = msg.Action.String() + " failed: " + msg.Outcome.Stderr
		return s
	case ConfirmSudo:
		if !s.PendingSudo.Active {
			return s
		}
		// The ui runs the sudo Cmd by PendingSudo.Kind; reduce just flags it running.
		s.ActionRunning = s.PendingSudo.Kind == SudoAction
		s.SudoConfirmed = true
		return s
	case CancelSudo:
		s.PendingSudo = PendingSudo{}
		s.SudoConfirmed = false
		s.StatusMsg = ""
		return s
	case ServiceDetailLoaded:
		if msg.Target != targetOf(s) {
			return s // superseded / stale
		}
		if msg.Err != nil {
			s.Detail.LoadState = DetailError
			if msg.Err.Kind == launchctl.FailurePermission {
				s.Detail.ErrMsg = "requires sudo to inspect"
			} else {
				s.Detail.ErrMsg = msg.Err.Stderr
			}
			return s
		}
		s.Detail.LoadState = DetailReady
		s.Detail.Metadata = msg.Detail
		s.Detail.Raw = msg.Detail.Raw
		s.Detail.ErrMsg = ""
		return s
	case LogLinesAppended:
		if msg.TailTarget != targetOf(s) {
			return s
		}
		s.LogRing = append(s.LogRing, msg.Lines...)
		if len(s.LogRing) > logRingCap {
			s.LogRing = s.LogRing[len(s.LogRing)-logRingCap:]
		}
		return s
	}
	return s
}

// pickerVerbs is the picker order and matches the keymap shortcuts.
var pickerVerbs = []launchctl.ActionKind{
	launchctl.ActionStart, launchctl.ActionRestart, launchctl.ActionStop,
	launchctl.ActionEnable, launchctl.ActionDisable, launchctl.ActionUnload,
}

func cyclePickerVerb(cur launchctl.ActionKind, delta int) launchctl.ActionKind {
	idx := 0
	for i, v := range pickerVerbs {
		if v == cur {
			idx = i
		}
	}
	idx = (idx + delta + len(pickerVerbs)) % len(pickerVerbs)
	return pickerVerbs[idx]
}

func busy(s AppState) bool {
	return s.ActionRunning || s.PendingSudo.Active || s.PendingConfirm.Active
}

func reduceRunAction(a launchctl.ActionKind, s AppState) AppState {
	if s.Selected == "" || s.Gone {
		return s
	}
	if busy(s) {
		s.StatusMsg = "action already running"
		return s
	}
	if a.Destructive() {
		s.PendingConfirm = PendingConfirm{
			Active: true, Action: a,
			Target: targetOf(s),
		}
		return s
	}
	return startAction(a, s)
}

// startAction marks the action in-flight; the Cmd that actually runs launchctl is
// built by the ui layer from ActionRunning + Selected (see Phase 4).
func startAction(a launchctl.ActionKind, s AppState) AppState {
	s.ActionRunning = true
	s.StatusMsg = a.String() + "…"
	s.pendingAction = a // stored so ui knows which verb to run
	return s
}

func targetOf(s AppState) string {
	for _, sv := range s.Services {
		if sv.Label == s.Selected {
			return sv.Domain.Target(sv.Label)
		}
	}
	return s.Selected
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
