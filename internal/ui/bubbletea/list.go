package bubbletea

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/volkoffskij/launchdeck/internal/app"
	"github.com/volkoffskij/launchdeck/internal/i18n"
)

// renderList renders a box whose TOTAL OUTER size (including the border) is
// exactly w x h. vm.Rows is already the windowed set of rows (see app.Derive) —
// this just styles and prints them. No inner padding: the row's own leading
// space gives the selection its margin left of the marker, so all the width
// goes to the labels.
func (m Model) renderList(vm app.ListVM, w, h int) string {
	th := m.theme
	style := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(th.border())
	// lipgloss Width() INCLUDES padding, so set it to w minus the border only;
	// the actual text area is w minus the whole frame (border + padding).
	contentW := w - style.GetHorizontalFrameSize() // text area
	styleW := w - style.GetHorizontalBorderSize()  // what Width() wants
	contentH := h - style.GetVerticalFrameSize()
	if contentW < 1 {
		contentW = 1
	}
	if styleW < 1 {
		styleW = 1
	}
	if contentH < 1 {
		contentH = 1
	}
	style = style.Width(styleW).Height(contentH)
	if vm.Placeholder != "" {
		return style.Render(th.muted().Render(vm.Placeholder))
	}
	selBg := lipgloss.Color(th.SelectedBg)
	var b []string
	for _, r := range vm.Rows {
		dotColor := lipgloss.Color(th.Stopped)
		dot := "○"
		if r.Running {
			dot, dotColor = "●", lipgloss.Color(th.Running)
		}
		// One leading space before the marker so the selection highlight has a
		// colored margin left of the ● instead of starting flush against it; all
		// rows carry it so the dots stay vertically aligned. Reserve 4 cols
		// (lead + ambiguous-width ●/○ up to 2 + gap) so the row never wraps.
		label := ellipsize(r.Label, contentW-4)
		var line string
		if r.Selected {
			// Keep the ● its running/stopped colour ON the selection background so
			// the status still reads when the row is highlighted; the label and the
			// padding carry the plain selection fg/bg.
			dotSeg := lipgloss.NewStyle().Foreground(dotColor).Background(selBg).Render(dot)
			inner := th.sel().Render(" ") + dotSeg + th.sel().Render(" "+label)
			if r.Gone {
				inner += th.sel().Render(i18n.T("list.gone"))
			}
			if pad := (contentW - 1) - lipgloss.Width(inner); pad > 0 {
				inner += th.sel().Render(strings.Repeat(" ", pad))
			}
			line = inner
		} else {
			line = " " + lipgloss.NewStyle().Foreground(dotColor).Render(dot) + " " + label
			if r.Gone {
				line += th.gone().Render(i18n.T("list.gone"))
			}
		}
		b = append(b, zone.Mark("row:"+r.Label, line))
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
