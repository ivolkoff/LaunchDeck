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
	vh := h - 2 // NormalBorder() adds 2 lines (top+bottom)
	if vh < 1 {
		vh = 1
	}
	start := 0
	if sel := vm.SelectedIdx; sel >= vh {
		start = sel - vh + 1
	}
	if maxStart := len(vm.Rows) - vh; maxStart < 0 {
		maxStart = 0
	} else if start > maxStart {
		start = maxStart
	}
	end := start + vh
	if end > len(vm.Rows) {
		end = len(vm.Rows)
	}
	var b []string
	for _, r := range vm.Rows[start:end] {
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
	// Height must match the actual row count rendered, not the full budget h:
	// lipgloss pads short content up to Height() before adding the border, so
	// leaving it at h would re-inflate the box by the padding we just trimmed.
	return style.Height(len(b)).Render(lipgloss.JoinVertical(lipgloss.Left, b...))
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
