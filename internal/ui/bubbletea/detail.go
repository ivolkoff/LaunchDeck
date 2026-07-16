package bubbletea

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/volkoffskij/launchdeck/internal/app"
)

func renderDetail(vm app.DetailVM, w, h int) string {
	style := lipgloss.NewStyle().Width(w).Height(h).Border(lipgloss.NormalBorder())
	if vm.Mode == "empty" {
		return style.Render("Select a service")
	}
	tabs := renderTabs(vm.ActiveTab)
	var body string
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
		}
	case app.TabRaw:
		body = vm.Raw
	}
	if vm.Mode == "gone" {
		body = "(gone) — service no longer present\n\n" + body
	}
	return style.Render(tabs + "\n" + body)
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
