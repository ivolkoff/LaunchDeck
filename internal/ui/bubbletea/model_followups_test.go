package bubbletea

import (
	"testing"

	"github.com/ivolkoff/launchdeck/internal/app"
	"github.com/ivolkoff/launchdeck/internal/launchctl"
)

// A selection restored from a saved session is set at construction, so no
// SelectService msg ever fires for it. followUps must still fetch its detail
// when the first scan resolves it, or the detail panel sits on "Loading…"
// forever. On later steady-state polls it must NOT re-fetch.
func TestFollowUpsFetchesRestoredSelectionDetail(t *testing.T) {
	st := app.NewState(501)
	st.Services = []launchctl.Service{{Label: "com.foo", Domain: launchctl.GUIDomain(501)}}
	st.Selected = "com.foo"
	st.FirstScanDone = true
	m := New(st, nil)

	// First scan just resolved the restored selection (prevFirstScan=false),
	// selection unchanged → one detail fetch.
	cmds := (&m).followUps(app.ServicesLoaded{Services: st.Services}, "com.foo", false)
	if len(cmds) != 1 {
		t.Fatalf("restored selection: got %d cmds, want 1 (detail fetch)", len(cmds))
	}

	// Steady-state poll (prevFirstScan=true), selection unchanged → no re-fetch.
	cmds = (&m).followUps(app.ServicesLoaded{Services: st.Services}, "com.foo", true)
	if len(cmds) != 0 {
		t.Fatalf("steady poll: got %d cmds, want 0", len(cmds))
	}
}
