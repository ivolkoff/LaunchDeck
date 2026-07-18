package bubbletea

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/volkoffskij/launchdeck/internal/app"
)

// renderDetail renders a box whose TOTAL OUTER size (including the border)
// is exactly w x h. Every tab's body is word-wrapped to the panel width and
// windowed by the scroll offset, so a long value reads on several rows instead
// of being cut off, and content past the fold is reachable by scrolling.
func (m Model) renderDetail(vm app.DetailVM, w, h, logScroll int) string {
	th := m.theme
	style := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(th.border())
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
		return style.Render(th.muted().Render("Select a service"))
	}
	tabs := m.renderTabs(vm.ActiveTab)
	bodyH := contentH - 1 // tabs line
	if bodyH < 1 {
		bodyH = 1
	}
	// Use the cached wrapped lines (rebuilt only on content/width change); fall
	// back to computing inline if the cache isn't primed yet.
	lines := m.detailCache
	if lines == nil {
		lines = detailLines(vm, contentW, th)
	}
	body := strings.Join(windowLines(lines, bodyH, logScroll), "\n")
	return style.Render(tabs + "\n" + body)
}

// detailLines produces the final display rows for the active tab, wrapped to
// contentW: Metadata and Logs are word-wrapped (a token wider than the panel —
// e.g. a long plist path — is hard-broken), Raw is word-wrapped with an
// editor-style line-number gutter. Both the renderer and the scroll clamp use
// this so they window over exactly the same rows.
func detailLines(vm app.DetailVM, contentW int, th Theme) []string {
	body := detailBody(vm)
	switch vm.ActiveTab {
	case app.TabRaw:
		return numberedWrap(body, contentW, th)
	case app.TabLogs:
		if vm.LogNote != "" || len(vm.LogLines) == 0 {
			return strings.Split(wrapBody(body, contentW), "\n") // a note, not log lines
		}
		// Numbered like an editor, with the [out]/[err] tag colour-coded.
		return numberedWrap(colorLogTags(body, th), contentW, th)
	default:
		return strings.Split(wrapBody(body, contentW), "\n")
	}
}

// colorLogTags colours the leading "[out]" / "[err]" tag on each log line so the
// two streams read apart: errors in the gone/red colour, stdout muted.
func colorLogTags(body string, th Theme) string {
	lines := strings.Split(body, "\n")
	for i, l := range lines {
		switch {
		case strings.HasPrefix(l, "[err]"):
			lines[i] = th.gone().Render("[err]") + l[len("[err]"):]
		case strings.HasPrefix(l, "[out]"):
			lines[i] = th.muted().Render("[out]") + l[len("[out]"):]
		}
	}
	return strings.Join(lines, "\n")
}

// detailBody builds the raw (unwrapped) body text for the active tab.
func detailBody(vm app.DetailVM) string {
	var body string
	switch vm.ActiveTab {
	case app.TabMetadata:
		switch vm.Mode {
		case "loading":
			body = "Loading detail…"
		case "error":
			body = vm.Err
		default:
			body = strings.Join([]string{
				"Label:     " + vm.Label,
				"Domain:    " + vm.Domain,
				"PID:       " + vm.PID,
				"Last exit: " + vm.LastExit,
				"Run:       " + vm.RunState,
				"Enable:    " + vm.EnableState,
				"Program:   " + vm.Program,
				"Plist:     " + vm.Plist,
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
	return body
}

// wrapBody word-wraps s to w columns (space-aware; a token wider than w is
// hard-broken). Returns a newline-joined block of rows each at most w wide.
func wrapBody(s string, w int) string {
	if w < 1 {
		w = 1
	}
	return strings.TrimRight(lipgloss.NewStyle().Width(w).Render(s), "\n")
}

// numberedWrap word-wraps body to width w with a right-aligned, shaded
// line-number gutter (1-based, per logical line). Wrapped continuation rows keep
// a blank shaded gutter so the content stays aligned, like an editor.
func numberedWrap(body string, w int, th Theme) []string {
	gutterStyle := th.gutter()
	logical := strings.Split(body, "\n")
	gw := len(strconv.Itoa(len(logical))) // number digits
	if gw < 1 {
		gw = 1
	}
	gutterW := gw + 2 // one pad space, digits, one pad space
	cw := w - gutterW - 1
	if cw < 1 {
		cw = 1
	}
	blank := gutterStyle.Render(strings.Repeat(" ", gutterW))
	var out []string
	for i, line := range logical {
		for j, wl := range strings.Split(wrapBody(line, cw), "\n") {
			if j == 0 {
				num := gutterStyle.Render(fmt.Sprintf(" %*d ", gw, i+1))
				out = append(out, num+" "+wl)
			} else {
				out = append(out, blank+" "+wl)
			}
		}
	}
	return out
}

// windowLines returns at most vh lines starting at offset, clamped so the window
// never runs past the end of lines.
func windowLines(lines []string, vh, offset int) []string {
	if vh < 1 {
		vh = 1
	}
	maxStart := len(lines) - vh
	if maxStart < 0 {
		maxStart = 0
	}
	start := offset
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
	return lines[start:end]
}

func (m Model) renderTabs(active app.Tab) string {
	names := []string{"Metadata", "Logs", "Raw"}
	var out []string
	for i, n := range names {
		s := " " + n + " "
		if app.Tab(i) == active {
			s = m.theme.tabActive().Render(s)
		} else {
			s = m.theme.muted().Render(s)
		}
		out = append(out, zone.Mark("tab:"+n, s))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, out...)
}
