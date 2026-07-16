package bubbletea

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/volkoffskij/launchdeck/internal/app"
)

func renderDetail(vm app.DetailVM, w, h, logScroll int) string {
	style := lipgloss.NewStyle().Width(w).Height(h).Border(lipgloss.NormalBorder())
	if vm.Mode == "empty" {
		return style.Render("Select a service")
	}
	tabs := renderTabs(vm.ActiveTab)
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
	if scrollable {
		body = scrollLines(body, h, logScroll)
	}
	content := tabs + "\n" + body
	if scrollable {
		// Height must match the windowed content, not the full budget h:
		// lipgloss pads short content up to Height() before adding the
		// border, so leaving it at h would re-inflate the box by the
		// padding we just trimmed via scrollLines.
		style = style.Height(strings.Count(content, "\n") + 1)
	}
	return style.Render(content)
}

// scrollLines slices body to the visible window starting at logScroll lines
// in, clamped so the window never runs past the end of the content.
func scrollLines(body string, h, logScroll int) string {
	lines := strings.Split(body, "\n")
	vh := h - 4 // border (2) + tabs line (1) + separator (1)
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
