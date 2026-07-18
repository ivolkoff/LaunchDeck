package app

import (
	"testing"

	"github.com/volkoffskij/launchdeck/internal/session"
)

func TestFromSessionClampsUnknownEnums(t *testing.T) {
	in := session.Session{Selected: "com.a", DomainScope: 99, SortKey: 42, ActiveTab: 7, ListScroll: 5}
	st := FromSession(in, 501)
	if st.Selected != "com.a" || st.Scroll.List != 5 {
		t.Fatalf("basic fields: %+v", st)
	}
	if st.Filters.DomainScope != ScopeUser || st.SortKey != SortLabel || st.ActiveTab != TabMetadata {
		t.Fatalf("unknown enums must fall back to defaults: %+v", st)
	}
}

func TestToFromSessionRoundTrip(t *testing.T) {
	st := NewState(501)
	st.Selected = "com.x"
	st.Filters = Filters{DomainScope: ScopeSystem, TextPattern: "db"}
	st.SortKey, st.SortDesc = SortPID, true
	st.Scroll.List, st.ActiveTab = 3, TabRaw
	got := FromSession(ToSession(st), 501)
	if got.Selected != "com.x" || got.Filters != st.Filters || got.SortKey != SortPID || !got.SortDesc || got.Scroll.List != 3 || got.ActiveTab != TabRaw {
		t.Fatalf("round trip: %+v", got)
	}
}
