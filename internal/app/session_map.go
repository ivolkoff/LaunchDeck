package app

import "github.com/volkoffskij/launchdeck/internal/session"

func FromSession(sess session.Session, uid int) AppState {
	s := NewState(uid)
	s.Selected = sess.Selected
	s.Filters.TextPattern = sess.TextPattern
	if sess.DomainScope >= 0 && sess.DomainScope <= int(ScopeAll) {
		s.Filters.DomainScope = DomainScope(sess.DomainScope)
	} // else default ScopeAll from NewState
	if sess.SortKey >= 0 && sess.SortKey <= int(SortPID) {
		s.SortKey = SortKey(sess.SortKey)
	}
	s.SortDesc = sess.SortDesc
	if sess.ActiveTab >= 0 && sess.ActiveTab <= int(TabRaw) {
		s.ActiveTab = Tab(sess.ActiveTab)
	}
	if sess.ListScroll > 0 {
		s.Scroll.List = sess.ListScroll
	}
	return s
}

func ToSession(s AppState) session.Session {
	return session.Session{
		Selected:    s.Selected,
		TextPattern: s.Filters.TextPattern,
		DomainScope: int(s.Filters.DomainScope),
		SortKey:     int(s.SortKey),
		SortDesc:    s.SortDesc,
		ListScroll:  s.Scroll.List,
		ActiveTab:   int(s.ActiveTab),
	}
}
