package bubbletea

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/volkoffskij/launchdeck/internal/app"
	"github.com/volkoffskij/launchdeck/internal/i18n"
)

// renderDetail renders a box whose TOTAL OUTER size (including the border)
// is exactly w x h. Every tab's body is word-wrapped to the panel width and
// windowed by the scroll offset, so a long value reads on several rows instead
// of being cut off, and content past the fold is reachable by scrolling.
func (m Model) renderDetail(vm app.DetailVM, w, h, logScroll int) string {
	th := m.theme
	// No left border: the sidebar's right border is the single-column divider.
	style := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).
		BorderLeft(false).BorderForeground(th.border())
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
		return style.Render(th.muted().Render(i18n.T("detail.select")))
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
		return numberedWrap(body, contentW, th, false)
	case app.TabLogs:
		if vm.LogNote != "" || len(vm.LogLines) == 0 {
			return strings.Split(wrapBody(body, contentW), "\n") // a note, not log lines
		}
		// Numbered like an editor, tag colour-coded. Logs are newest-first, so the
		// numbers descend (top = N, bottom = 1) to match the chronological order.
		return numberedWrap(colorLogTags(body, th), contentW, th, true)
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
			body = i18n.T("detail.loading")
		case "error":
			body = vm.Err
		default:
			body = metaBlock([][2]string{
				{i18n.T("meta.label"), vm.Label},
				{i18n.T("meta.domain"), vm.Domain},
				{i18n.T("meta.pid"), vm.PID},
				{i18n.T("meta.exit"), vm.LastExit},
				{i18n.T("meta.run"), vm.RunState},
				{i18n.T("meta.enable"), vm.EnableState},
				{i18n.T("meta.program"), vm.Program},
				{i18n.T("meta.plist"), vm.Plist},
			})
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
		body = i18n.T("detail.gone") + "\n\n" + body
	}
	return body
}

// metaBlock renders "Label:  value" rows with the colon-suffixed labels padded
// to a common width so the values align, regardless of language. Two spaces
// separate the widest label from its value.
func metaBlock(rows [][2]string) string {
	max := 0
	for _, r := range rows {
		if n := utf8.RuneCountInString(r[0]); n > max {
			max = n
		}
	}
	lines := make([]string, len(rows))
	for i, r := range rows {
		pad := max - utf8.RuneCountInString(r[0])
		lines[i] = r[0] + ":" + strings.Repeat(" ", pad+2) + r[1]
	}
	return strings.Join(lines, "\n")
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
// line-number gutter (per logical line). Wrapped continuation rows keep a blank
// shaded gutter so the content stays aligned, like an editor. When reverse is
// set the numbers descend from N at the top to 1 at the bottom (used for the
// newest-first Logs tab so the numbers track chronological order).
func numberedWrap(body string, w int, th Theme, reverse bool) []string {
	gutterStyle := th.gutter()
	logical := strings.Split(body, "\n")
	n := len(logical)
	gw := len(strconv.Itoa(n)) // number digits
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
		num := i + 1
		if reverse {
			num = n - i
		}
		for j, wl := range strings.Split(wrapBody(line, cw), "\n") {
			if j == 0 {
				g := gutterStyle.Render(fmt.Sprintf(" %*d ", gw, num))
				out = append(out, g+" "+wl)
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
	names := []string{"Metadata", "Logs", "Raw"} // stable zone ids
	var out []string
	for i, n := range names {
		s := " " + i18n.T("tab."+n) + " "
		if app.Tab(i) == active {
			s = m.theme.tabActive().Render(s)
		} else {
			s = m.theme.muted().Render(s)
		}
		out = append(out, zone.Mark("tab:"+n, s))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, out...)
}
