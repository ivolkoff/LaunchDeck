package bubbletea

import (
	"bufio"
	"context"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/volkoffskij/launchdeck/internal/app"
	"github.com/volkoffskij/launchdeck/internal/launchctl"
)

const (
	tailWindowBytes = 64 * 1024
	tailInitLines   = 500
)

// initialBuffer reads the last tailWindowBytes of a path and keeps the last
// tailInitLines of that read.
func initialBuffer(path, stream string) []app.LogLine {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil
	}
	start := int64(0)
	if info.Size() > tailWindowBytes {
		start = info.Size() - tailWindowBytes
	}
	if _, err := f.Seek(start, 0); err != nil {
		return nil
	}
	var lines []app.LogLine
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), tailWindowBytes)
	for sc.Scan() {
		lines = append(lines, app.LogLine{Stream: stream, Text: sc.Text()})
	}
	if len(lines) > tailInitLines {
		lines = lines[len(lines)-tailInitLines:]
	}
	return lines
}

// logTailCmd reads the initial log buffer for det's stdout/stderr paths and
// emits it as a single app.LogLinesAppended. Live-follow growth (ctx is
// reserved for its cancellation) is a documented deferral — ponytail: add a
// tea.Tick-driven re-read here only if the initial snapshot proves too stale
// in practice.
func logTailCmd(ctx context.Context, d launchctl.Domain, det launchctl.ServiceDetail) tea.Cmd {
	target := d.Target(det.Label)
	return func() tea.Msg {
		paths := logPaths(det)
		if len(paths) == 0 {
			return app.LogLinesAppended{TailTarget: target, State: "removed"}
		}
		var initial []app.LogLine
		for _, p := range paths {
			initial = append(initial, initialBuffer(p.path, p.stream)...)
		}
		return app.LogLinesAppended{TailTarget: target, Lines: initial}
	}
}

type logPath struct {
	path, stream string
}

// logPaths resolves det's stdout/stderr into read targets: equal paths follow
// once as [out]; distinct paths are both read; only one configured yields
// just that one.
func logPaths(det launchctl.ServiceDetail) []logPath {
	out := det.StdoutPath
	errp := det.StderrPath
	switch {
	case out != "" && errp != "" && out == errp:
		return []logPath{{out, "out"}}
	case out != "" && errp != "":
		return []logPath{{out, "out"}, {errp, "err"}}
	case out != "":
		return []logPath{{out, "out"}}
	case errp != "":
		return []logPath{{errp, "err"}}
	default:
		return nil
	}
}
