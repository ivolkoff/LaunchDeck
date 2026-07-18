package bubbletea

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/volkoffskij/launchdeck/internal/app"
)

// renderDetail renders a box whose TOTAL OUTER size (including the border)
// is exactly w x h.
func renderDetail(vm app.DetailVM, w, h, logScroll int) string {
	border := lipgloss.NormalBorder()
	style := lipgloss.NewStyle().Border(border)
	contentW := w - style.GetHorizontalFrameSize()
	contentH := h - style.GetVerticalFrameSize()
	if contentW < 1 {
		contentW = 1
	}
	if contentH < 1 {
		contentH = 1
	}
	style = style.Width(contentW).Height(contentH)
	if vm.Mode == "empty" {
		return style.Render("Select a service")
	}
	tabs := renderTabs(vm.ActiveTab)
	bodyH := contentH - 1 // tabs line
	if bodyH < 1 {
		bodyH = 1
	}
	var body string
	scrollable := false
	switch vm.ActiveTab {
	case app.TabMetadata:
		if vm.Mode == "loading" {
			body = "Loading detail…"
		} else if vm.Mode == "error" {
			body = vm.Err
		} else {
			body = strings.Join([]string{
				"label:     " + vm.Label,
				"domain:    " + vm.Domain,
				"pid:       " + vm.PID,
				"last exit: " + vm.LastExit,
				"run:       " + vm.RunState,
				"enable:    " + vm.EnableState,
				"program:   " + vm.Program,
				"plist:     " + vm.Plist,
			}, "\n")
		}
	case app.TabLogs:
		if vm.LogNote != "" {
			body = vm.LogNote
		} else {
			body = strings.Join(vm.LogLines, "\n")
			scrollable = true
		}
	case app.TabRaw:
		body = vm.Raw
		scrollable = true
	}
	if vm.Mode == "gone" {
		body = "(gone) — service no longer present\n\n" + body
	}
	// Truncate every body line to the panel width BEFORE windowing: lipgloss
	// wraps a line wider than Width(), turning one log line into several screen
	// rows and blowing the box past its height budget. Truncating keeps
	// one line = one row, so the windowing below is exact.
	body = truncateLine(body, contentW)
	if scrollable {
		body = scrollLines(body, bodyH, logScroll)
	}
	content := tabs + "\n" + body
	return style.Render(content)
}

// scrollLines slices body to the visible window starting at logScroll lines
// in, clamped so the window never runs past the end of the content. vh is
// the number of body rows available (border + tabs line already subtracted
// by the caller).
func scrollLines(body string, vh, logScroll int) string {
	lines := strings.Split(body, "\n")
	if vh < 1 {
		vh = 1
	}
	maxStart := len(lines) - vh
	if maxStart < 0 {
		maxStart = 0
	}
	start := logScroll
	if start < 0 {
		start = 0
	}
	if start > maxStart {
		start = maxStart
	}
	end := start + vh
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[start:end], "\n")
}

func renderTabs(active app.Tab) string {
	names := []string{"Metadata", "Logs", "Raw"}
	var out []string
	for i, n := range names {
		s := " " + n + " "
		if app.Tab(i) == active {
			s = lipgloss.NewStyle().Reverse(true).Render(s)
		}
		out = append(out, zone.Mark("tab:"+n, s))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, out...)
}
