package bubbletea

import (
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/volkoffskij/launchdeck/internal/app"
)

func renderStatus(vm app.StatusVM, w int) string {
	if vm.Prompt != "" {
		return lipgloss.NewStyle().Width(w).Reverse(true).Render(vm.Prompt)
	}
	var btns []string
	for _, b := range vm.Buttons {
		btns = append(btns, zone.Mark("btn:"+b, "["+b+"]"))
	}
	line := lipgloss.JoinHorizontal(lipgloss.Top, btns...)
	if vm.Message != "" {
		line += "  " + vm.Message
	}
	return lipgloss.NewStyle().Width(w).Render(line)
}
