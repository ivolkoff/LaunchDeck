package bubbletea

import (
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/volkoffskij/launchdeck/internal/app"
	"github.com/volkoffskij/launchdeck/internal/i18n"
)

// renderStatus renders exactly one row of width w.
//
// The truncate pass must be its own Render: within a single style lipgloss
// applies Width() first, which WRAPS an over-long line onto extra rows, and
// MaxWidth() then only trims those already-split rows. A long message (a failed
// action's stderr) would push the layout past the bottom of the terminal.
func truncateLine(s string, w int) string {
	return lipgloss.NewStyle().MaxWidth(w).Render(s)
}

// buttonKey maps an action-button label to the key that runs it in the action
// picker (opened with `a`). Must stay in sync with keys.go's pickerKey map.
// The direct letters (s/r/d…) are taken by global keys, so the verb keys only
// act inside the picker — hence the `a→` hint prefix.
var buttonKey = map[string]string{
	"Start":   "s",
	"Restart": "r",
	"Stop":    "k",
	"Enable":  "e",
	"Disable": "d",
	"Unload":  "u",
}

func (m Model) renderStatus(vm app.StatusVM, w int) string {
	th := m.theme
	if vm.Prompt != "" {
		return th.sel().Width(w).Render(truncateLine(vm.Prompt, w))
	}
	// `a→` tells the user to press `a` (opens the picker), then the letter shown
	// on each button; the whole `[s Start]` chip is also a mouse-click zone.
	btns := []string{zone.Mark("btn:actions", th.accent().Render("a→"))}
	for _, b := range vm.Buttons {
		label := "[" + th.accent().Render(buttonKey[b]) + " " + i18n.T("btn."+b) + "]"
		btns = append(btns, zone.Mark("btn:"+b, label))
	}
	line := lipgloss.JoinHorizontal(lipgloss.Top, btns...)
	if vm.Message != "" {
		line += "  " + th.muted().Render(vm.Message)
	}
	return lipgloss.NewStyle().Width(w).Render(truncateLine(line, w))
}
