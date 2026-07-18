package bubbletea

import (
	"context"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/volkoffskij/launchdeck/internal/app"
	"github.com/volkoffskij/launchdeck/internal/launchctl"
	"github.com/volkoffskij/launchdeck/internal/session"
)

type Model struct {
	st          app.AppState
	client      *launchctl.Client
	theme       Theme
	width       int
	height      int
	pollBusy    bool
	dragging    bool            // divider drag in progress
	mouseOff    bool            // mouse capture released for native terminal text selection
	helpOpen    bool            // help overlay shown
	detailCache []string        // wrapped detail-body lines; recomputed only on content/width change
	lastSaved   session.Session // debounce marker: last state written to disk
}

func New(st app.AppState, c *launchctl.Client) Model {
	return Model{st: st, client: c, theme: DefaultTheme()}
}

// WithTheme returns a copy of the model using the given theme.
func (m Model) WithTheme(t Theme) Model {
	m.theme = t
	return m
}

func (m Model) Init() tea.Cmd {
	zone.NewGlobal()
	return tea.Batch(pollCmd(m.client, m.st.UID), tea.EnterAltScreen)
}

// tickMsg is a local Bubble Tea message (tea.Msg is interface{}); it is handled
// by its own case in Update and is NOT an app.Msg.
type tickMsg struct{}

func tick() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m Model) Update(raw tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := raw.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		listH, logH := m.viewportHeights()
		m.st = app.Reduce(app.WindowResized{Width: msg.Width, ListViewportH: listH, LogViewportH: logH}, m.st)
		(&m).refreshDetailCache() // width changed → re-wrap
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.MouseMsg:
		return m.handleMouse(msg)
	case tickMsg:
		var cmds []tea.Cmd
		if !m.pollBusy {
			m.pollBusy = true
			cmds = append(cmds, pollCmd(m.client, m.st.UID))
		}
		// Live logs: re-read the selected service's tail every tick so the Logs
		// tab updates in near real time (≤ the 2s poll interval).
		if m.st.Selected != "" && !m.st.Gone && m.st.Detail.LoadState == app.DetailReady {
			if d, _, ok := m.selectedService(); ok {
				cmds = append(cmds, logTailCmd(context.Background(), d, m.st.Detail.Metadata))
			}
		}
		(&m).maybeSave() // debounced session write: at most once per 2s tick
		cmds = append(cmds, tick())
		return m, tea.Batch(cmds...)
	case app.Msg:
		return m.applyIntent(msg)
	}
	return m, nil
}

// affectsDetailScroll reports whether an intent can change the detail panel's
// scroll offset — the only cases the Scroll.Log re-clamp is needed. A sidebar
// scroll is excluded: it never touches Scroll.Log.
func affectsDetailScroll(msg app.Msg) bool {
	switch m := msg.(type) {
	case app.ScrollMsg:
		return m.Panel == app.FocusDetail
	case app.SetTab, app.SelectService, app.MoveSelection, app.FocusPanel,
		app.WindowResized, app.ServiceDetailLoaded, app.LogLinesAppended:
		return true
	default:
		return false
	}
}

// affectsDetailContent reports whether an intent can change the detail body's
// text or the width it's wrapped to — the only cases the detail cache must be
// rebuilt. A sidebar scroll (and most other intents) leave it untouched.
func affectsDetailContent(msg app.Msg) bool {
	switch msg.(type) {
	case app.SetTab, app.SelectService, app.MoveSelection,
		app.ServiceDetailLoaded, app.LogLinesAppended, app.ServicesLoaded,
		app.SetSidebarWidth, app.WindowResized:
		return true
	default:
		return false
	}
}

// refreshDetailCache recomputes the wrapped detail-body lines for the current
// selection, tab, and panel width.
func (m *Model) refreshDetailCache() {
	vm := app.Derive(m.st)
	if vm.Detail.Mode == "empty" {
		m.detailCache = nil
		return
	}
	m.detailCache = detailLines(vm.Detail, m.detailContentW(), m.theme)
}

// applyIntent runs reduce, then fires any Cmd the new state implies.
func (m Model) applyIntent(msg app.Msg) (tea.Model, tea.Cmd) {
	prevSel := m.st.Selected
	if _, ok := msg.(app.ServicesLoaded); ok {
		m.pollBusy = false
	}
	m.st = app.Reduce(msg, m.st)
	// The detail body (word-wrapped, possibly thousands of log/raw lines) is
	// cached and only recomputed when its content or width changes — never on a
	// sidebar scroll — so wheel-scrolling the list stays cheap.
	if affectsDetailContent(msg) {
		(&m).refreshDetailCache()
	}
	// Bound Scroll.Log only for intents that touch the detail scroll; it reads the
	// cache, so it's cheap.
	if affectsDetailScroll(msg) {
		m.st.Scroll.Log = m.clampLogScroll()
	}
	// Session save is NOT done here (it would hit the disk on every scroll notch);
	// it's debounced onto the 2s tick and forced on quit.
	cmds := (&m).followUps(msg, prevSel)
	return m, tea.Batch(cmds...)
}

func (m *Model) selectedService() (launchctl.Domain, string, bool) {
	for _, s := range m.st.Services {
		if s.Label == m.st.Selected {
			return s.Domain, s.Label, true
		}
	}
	return launchctl.Domain{}, "", false
}

// followUps fires the Cmds implied by the new state: a detail re-fetch when
// selection changes, the launchctl Cmd behind a just-started action, and a
// detail re-fetch after that action completes on the selected service.
func (m *Model) followUps(msg app.Msg, prevSel string) []tea.Cmd {
	var cmds []tea.Cmd
	// Selection changed to a present, non-gone service → fetch detail.
	if m.st.Selected != prevSel && m.st.Selected != "" && !m.st.Gone {
		if d, label, ok := m.selectedService(); ok {
			cmds = append(cmds, detailCmd(m.client, d, label))
		}
	}
	// Detail just loaded for the current selection: start the log tail.
	if dl, ok := msg.(app.ServiceDetailLoaded); ok && m.st.Detail.LoadState == app.DetailReady {
		if d, label, ok := m.selectedService(); ok && d.Target(label) == dl.Target {
			m.st.TailIdentity = dl.Target
			cmds = append(cmds, logTailCmd(context.Background(), d, m.st.Detail.Metadata))
		}
	}
	// A run just started (ActionRunning flipped by reduce): fire the launchctl Cmd.
	if m.st.ActionRunning && !actionAlreadyDispatched(msg) {
		if d, plist, ok := m.st.LoadTarget(); ok {
			cmds = append(cmds, bootstrapCmd(m.client, d, expandHome(plist)))
		} else if d, label, ok := m.selectedService(); ok {
			cmds = append(cmds, actionCmd(m.client, m.st.PendingAction(), d, label))
		}
	}
	// After an action on the selected service, re-fetch its detail (~2s freshness).
	if ar, ok := msg.(app.ActionResult); ok && !m.st.ActionRunning {
		if d, label, ok := m.selectedService(); ok && d.Target(label) == ar.Target {
			cmds = append(cmds, detailCmd(m.client, d, label))
		}
	}
	// Confirmed sudo retry: fire the sudo-action Cmd.
	if _, ok := msg.(app.ConfirmSudo); ok && m.st.PendingSudo.Active {
		ps := m.st.PendingSudo
		if d, label, ok := m.selectedService(); ok {
			cmds = append(cmds, sudoActionCmd(m.st.PendingAction(), ps.Target, launchctl.ActionArgs(m.st.PendingAction(), d.Target(label))))
		}
	}
	return cmds
}

// actionAlreadyDispatched avoids re-firing the Cmd on the same message that set
// ActionRunning=true when that message is itself the ActionResult finishing it.
func actionAlreadyDispatched(msg app.Msg) bool {
	switch msg.(type) {
	case app.ActionResult, app.ServicesLoaded, app.ServiceDetailLoaded, app.LogLinesAppended, app.ConfirmSudo:
		return true
	default:
		return false
	}
}

// maybeSave persists the session, debounced by comparing against the last
// saved snapshot so unrelated ticks/messages don't trigger disk writes.
func (m *Model) maybeSave() {
	next := app.ToSession(m.st)
	if next == m.lastSaved {
		return
	}
	m.lastSaved = next
	if p, err := session.Path(); err == nil {
		_ = session.Save(p, next) // best-effort; a save error is non-fatal
	}
}

// maybeSaveFinal persists the session unconditionally (bypasses the debounce
// check in maybeSave), used just before quitting so the final state is saved.
func (m *Model) maybeSaveFinal() {
	if p, err := session.Path(); err == nil {
		_ = session.Save(p, app.ToSession(m.st))
	}
}

// --- Temporary stubs; replaced by later tasks. ---

func (m Model) View() string { return m.render() }

// expandHome expands a leading "~" or "~/" to the user's home directory:
// exec.Command does not do shell tilde-expansion, so a literal "~/..." path
// passed straight to launchctl fails with ENOENT.
func expandHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return home + path[1:]
	}
	return path
}
