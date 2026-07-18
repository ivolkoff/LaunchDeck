package bubbletea

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/volkoffskij/launchdeck/internal/app"
	"github.com/volkoffskij/launchdeck/internal/launchctl"
)

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	st := m.st

	// Help overlay: while open, any of ?/esc/q closes it; everything else is
	// swallowed so the overlay is read-only.
	if m.helpOpen {
		if k == "?" || k == "esc" || k == "q" {
			m.helpOpen = false
		}
		return m, nil
	}
	if k == "?" {
		m.helpOpen = true
		return m, nil
	}

	// Modal suppression: only the open modal's keys are live.
	switch {
	case st.FilterEditing:
		return m.filterKey(msg)
	case st.LoadPrompt.Open:
		return m.loadKey(msg)
	case st.ActionPicker.Open:
		return m.pickerKey(k)
	case st.PendingConfirm.Active || st.PendingSudo.Active:
		return m.promptKey(k)
	}

	// Global keys.
	switch k {
	case "q", "ctrl+c":
		(&m).maybeSaveFinal()
		return m, tea.Quit
	case "up", "k":
		if st.Focus == app.FocusSidebar {
			return m.applyIntent(app.MoveSelection{Delta: -1})
		}
		return m.applyIntent(app.ScrollMsg{Panel: app.FocusDetail, Delta: -1})
	case "down", "j":
		if st.Focus == app.FocusSidebar {
			return m.applyIntent(app.MoveSelection{Delta: 1})
		}
		return m.applyIntent(app.ScrollMsg{Panel: app.FocusDetail, Delta: 1})
	case "home":
		return m.applyIntent(app.MoveSelection{ToTop: true})
	case "end":
		return m.applyIntent(app.MoveSelection{ToBottom: true})
	case "pgup":
		return m.applyIntent(app.MoveSelection{Delta: -10})
	case "pgdown":
		return m.applyIntent(app.MoveSelection{Delta: 10})
	case "tab":
		return m.applyIntent(app.FocusPanel{})
	case "1":
		return m.applyIntent(app.SetTab{Tab: app.TabMetadata})
	case "2":
		return m.applyIntent(app.SetTab{Tab: app.TabLogs})
	case "3":
		return m.applyIntent(app.SetTab{Tab: app.TabRaw})
	case "left":
		return m.applyIntent(app.SetTab{Tab: prevTab(st.ActiveTab)})
	case "right":
		return m.applyIntent(app.SetTab{Tab: nextTab(st.ActiveTab)})
	case "a":
		return m.applyIntent(app.OpenActionPicker{})
	case "/":
		return m.applyIntent(app.OpenFilter{})
	case "d":
		return m.applyIntent(app.CycleDomainScope{})
	case "s":
		return m.applyIntent(app.SetSort{})
	case "S":
		return m.applyIntent(app.SetSort{ToggleDir: true})
	case "L":
		return m.applyIntent(app.OpenLoadPrompt{})
	case "r":
		return m, pollCmd(m.client, m.st.UID)
	case "ctrl+u":
		return m.applyIntent(app.ScrollMsg{Panel: st.Focus, Delta: -10})
	case "ctrl+d":
		return m.applyIntent(app.ScrollMsg{Panel: st.Focus, Delta: 10})
	case "m":
		// Release/recapture the mouse. Released, the terminal's own text
		// selection (drag + ⌘C) works again; recaptured, clicks/drag/wheel do.
		m.mouseOff = !m.mouseOff
		if m.mouseOff {
			return m, tea.DisableMouse
		}
		return m, tea.EnableMouseCellMotion
	}
	return m, nil
}

func prevTab(t app.Tab) app.Tab {
	if t == app.TabMetadata {
		return app.TabRaw
	}
	return t - 1
}
func nextTab(t app.Tab) app.Tab {
	if t == app.TabRaw {
		return app.TabMetadata
	}
	return t + 1
}

func (m Model) filterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		return m.applyIntent(app.CommitFilter{})
	case "esc":
		return m.applyIntent(app.CancelFilter{})
	case "ctrl+u":
		return m.applyIntent(app.SetFilterBuffer{Buffer: ""})
	case "backspace":
		b := m.st.FilterBuffer
		if b != "" {
			b = b[:len(b)-1]
		}
		return m.applyIntent(app.SetFilterBuffer{Buffer: b})
	default:
		if len(msg.Runes) == 1 {
			return m.applyIntent(app.SetFilterBuffer{Buffer: m.st.FilterBuffer + string(msg.Runes)})
		}
	}
	return m, nil
}

func (m Model) loadKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		return m.applyIntent(app.SubmitLoad{})
	case "esc":
		return m.applyIntent(app.CancelLoad{})
	case "backspace":
		b := m.st.LoadPrompt.Buffer
		if b != "" {
			b = b[:len(b)-1]
		}
		return m.applyIntent(app.SetLoadBuffer{Buffer: b})
	default:
		if len(msg.Runes) == 1 {
			return m.applyIntent(app.SetLoadBuffer{Buffer: m.st.LoadPrompt.Buffer + string(msg.Runes)})
		}
	}
	return m, nil
}

func (m Model) pickerKey(k string) (tea.Model, tea.Cmd) {
	verbs := map[string]launchctl.ActionKind{
		"s": launchctl.ActionStart, "r": launchctl.ActionRestart, "k": launchctl.ActionStop,
		"e": launchctl.ActionEnable, "d": launchctl.ActionDisable, "u": launchctl.ActionUnload,
	}
	switch k {
	case "esc":
		return m.applyIntent(app.CancelActionPicker{})
	case "up":
		return m.applyIntent(app.MoveActionPicker{Delta: -1})
	case "down":
		return m.applyIntent(app.MoveActionPicker{Delta: 1})
	case "enter":
		return m.applyIntent(app.PickAction{Action: m.st.ActionPicker.HighlightedVerb})
	}
	if v, ok := verbs[k]; ok {
		return m.applyIntent(app.PickAction{Action: v})
	}
	return m, nil
}

func (m Model) promptKey(k string) (tea.Model, tea.Cmd) {
	yes := k == "y" || k == "enter"
	no := k == "n" || k == "esc"
	if m.st.PendingConfirm.Active {
		if yes {
			return m.applyIntent(app.ConfirmAction{})
		}
		if no {
			return m.applyIntent(app.CancelAction{})
		}
	}
	if m.st.PendingSudo.Active {
		if yes {
			return m.applyIntent(app.ConfirmSudo{})
		}
		if no {
			return m.applyIntent(app.CancelSudo{})
		}
	}
	return m, nil
}
