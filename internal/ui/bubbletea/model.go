package bubbletea

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/volkoffskij/launchdeck/internal/app"
	"github.com/volkoffskij/launchdeck/internal/launchctl"
)

type Model struct {
	st       app.AppState
	client   *launchctl.Client
	width    int
	height   int
	pollBusy bool
	saveAt   time.Time // debounce marker
	dirty    bool
}

func New(st app.AppState, c *launchctl.Client) Model {
	return Model{st: st, client: c}
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
		cmds = append(cmds, tick())
		return m, tea.Batch(cmds...)
	case app.Msg:
		return m.applyIntent(msg)
	}
	return m, nil
}

// applyIntent runs reduce, then fires any Cmd the new state implies.
func (m Model) applyIntent(msg app.Msg) (tea.Model, tea.Cmd) {
	prevSel := m.st.Selected
	if _, ok := msg.(app.ServicesLoaded); ok {
		m.pollBusy = false
	}
	m.st = app.Reduce(msg, m.st)
	cmds := m.followUps(msg, prevSel)
	m.maybeSave()
	return m, tea.Batch(cmds...)
}

// --- Temporary stubs; replaced by later tasks. ---

func (m Model) followUps(app.Msg, string) []tea.Cmd           { return nil }
func (m Model) maybeSave()                                    {}
func (m Model) handleKey(tea.KeyMsg) (tea.Model, tea.Cmd)     { return m, nil }
func (m Model) handleMouse(tea.MouseMsg) (tea.Model, tea.Cmd) { return m, nil }
func (m Model) View() string                                  { return "" }
