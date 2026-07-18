package bubbletea

import (
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/volkoffskij/launchdeck/internal/app"
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

func renderStatus(vm app.StatusVM, w int) string {
	if vm.Prompt != "" {
		return lipgloss.NewStyle().Width(w).Reverse(true).Render(truncateLine(vm.Prompt, w))
	}
	var btns []string
	for _, b := range vm.Buttons {
		btns = append(btns, zone.Mark("btn:"+b, "["+b+"]"))
	}
	line := lipgloss.JoinHorizontal(lipgloss.Top, btns...)
	if vm.Message != "" {
		line += "  " + vm.Message
	}
	return lipgloss.NewStyle().Width(w).Render(truncateLine(line, w))
}
