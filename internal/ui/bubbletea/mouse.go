package bubbletea

import (
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/volkoffskij/launchdeck/internal/app"
	"github.com/volkoffskij/launchdeck/internal/launchctl"
)

var tabNames = []string{"Metadata", "Logs", "Raw"}
var buttonNames = []string{"Start", "Restart", "Stop", "Enable", "Disable", "Unload"}

func modalOpen(s app.AppState) bool {
	return s.FilterEditing || s.LoadPrompt.Open || s.ActionPicker.Open ||
		s.PendingConfirm.Active || s.PendingSudo.Active
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.mouseOff {
		return m, nil // mouse released for native terminal text selection
	}
	// Divider drag is handled before the modal guard: a release must always end a
	// drag, and a drag isn't a modal.
	switch msg.Action {
	case tea.MouseActionMotion:
		if m.dragging {
			// The divider follows the cursor: the sidebar's outer width becomes the
			// cursor column. reduce clamps it to a safe range.
			return m.applyIntent(app.SetSidebarWidth{W: msg.X})
		}
		return m, nil
	case tea.MouseActionRelease:
		m.dragging = false
		return m, nil
	}

	if modalOpen(m.st) {
		return m, nil // mouse is modal-suppressed
	}
	if msg.Action != tea.MouseActionPress {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		return m.applyIntent(app.ScrollMsg{Panel: m.hoveredPanel(msg), Delta: -3})
	case tea.MouseButtonWheelDown:
		return m.applyIntent(app.ScrollMsg{Panel: m.hoveredPanel(msg), Delta: 3})
	case tea.MouseButtonLeft:
		// A 1-col divider is a hard target, so grab within ±1 of its column.
		if abs(msg.X-m.sidebarW()) <= 1 {
			m.dragging = true
			return m.applyIntent(app.SetSidebarWidth{W: msg.X})
		}
		if label, ok := m.hitRow(msg); ok {
			return m.applyIntent(app.SelectService{Label: label})
		}
		if name, ok := hitTab(msg); ok {
			return m.applyIntent(app.SetTab{Tab: tabByName(name)})
		}
		if name, ok := hitButton(msg); ok {
			return m.applyIntent(app.RunAction{Action: actionByName(name)})
		}
	}
	return m, nil
}

// hitRow returns the clicked row's label, or "", false.
func (m Model) hitRow(msg tea.MouseMsg) (string, bool) {
	for _, r := range app.Derive(m.st).List.Rows {
		if zone.Get("row:" + r.Label).InBounds(msg) {
			return r.Label, true
		}
	}
	return "", false
}

// hitTab returns the clicked tab's name, or "", false.
func hitTab(msg tea.MouseMsg) (string, bool) {
	for _, n := range tabNames {
		if zone.Get("tab:" + n).InBounds(msg) {
			return n, true
		}
	}
	return "", false
}

// hitButton returns the clicked button's name, or "", false.
func hitButton(msg tea.MouseMsg) (string, bool) {
	for _, n := range buttonNames {
		if zone.Get("btn:" + n).InBounds(msg) {
			return n, true
		}
	}
	return "", false
}

func (m Model) hoveredPanel(msg tea.MouseMsg) app.Focus {
	if _, ok := m.hitRow(msg); ok {
		return app.FocusSidebar
	}
	return app.FocusDetail
}

func tabByName(n string) app.Tab {
	switch n {
	case "Logs":
		return app.TabLogs
	case "Raw":
		return app.TabRaw
	default:
		return app.TabMetadata
	}
}

func actionByName(n string) launchctl.ActionKind {
	switch n {
	case "Restart":
		return launchctl.ActionRestart
	case "Stop":
		return launchctl.ActionStop
	case "Enable":
		return launchctl.ActionEnable
	case "Disable":
		return launchctl.ActionDisable
	case "Unload":
		return launchctl.ActionUnload
	default:
		return launchctl.ActionStart
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
