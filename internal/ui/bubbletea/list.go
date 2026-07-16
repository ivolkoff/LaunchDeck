package bubbletea

import (
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/volkoffskij/launchdeck/internal/app"
)

func renderList(vm app.ListVM, w, h int, focused bool) string {
	border := lipgloss.NormalBorder()
	style := lipgloss.NewStyle().Width(w).Height(h).Border(border)
	if vm.Placeholder != "" {
		return style.Render(vm.Placeholder)
	}
	var b []string
	for _, r := range vm.Rows {
		dot := "○"
		if r.Running {
			dot = "●"
		}
		line := dot + " " + ellipsize(r.Label, w-4)
		if r.Gone {
			line += " (gone)"
		}
		row := lipgloss.NewStyle().Render(line)
		if r.Selected {
			row = lipgloss.NewStyle().Reverse(true).Render(line)
		}
		b = append(b, zone.Mark("row:"+r.Label, row))
	}
	return style.Render(lipgloss.JoinVertical(lipgloss.Left, b...))
}

func ellipsize(s string, max int) string {
	if max < 1 {
		max = 1
	}
	if len(s) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return s[:max-1] + "…"
}
