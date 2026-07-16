package launchctl

import (
	"bufio"
	"errors"
	"regexp"
	"strconv"
	"strings"
)

// parseDomainScan extracts the services table from a `launchctl print <domain>`
// dump. Each row is "<pid>\t<last-exit>\t<label>"; "-" means absent.
func parseDomainScan(dump string, d Domain) ([]Service, error) {
	start := strings.Index(dump, "services = {")
	if start < 0 {
		return nil, errors.New("launchctl print: no services block")
	}
	rest := dump[start+len("services = {"):]
	end := strings.Index(rest, "}")
	if end < 0 {
		return nil, errors.New("launchctl print: unterminated services block")
	}
	block := rest[:end]

	var out []Service
	sc := bufio.NewScanner(strings.NewReader(block))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pidTok, exitTok, label := fields[0], fields[1], fields[len(fields)-1]
		svc := Service{Label: label, Domain: d}
		if pidTok != "-" {
			if pid, err := strconv.Atoi(pidTok); err == nil {
				svc.PID, svc.HasPID = pid, true
			}
		}
		if exitTok != "-" {
			if code, err := strconv.Atoi(exitTok); err == nil {
				svc.LastExit = code
			}
		}
		out = append(out, svc)
	}
	return out, sc.Err()
}

// parseServiceDetail best-effort-parses a `launchctl print <domain>/<label>`
// dump. It never errors: Raw always holds the full dump so the UI can fall back
// to it when a field is missing or the format drifts.
func parseServiceDetail(dump string, svc Service) ServiceDetail {
	d := ServiceDetail{Service: svc, Raw: dump, EnableState: EnableUnknown}
	sc := bufio.NewScanner(strings.NewReader(dump))
	inArgs := false
	for sc.Scan() {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		if inArgs {
			if trimmed == "}" {
				inArgs = false
				continue
			}
			if trimmed != "" {
				d.Args = append(d.Args, trimmed)
			}
			continue
		}
		if trimmed == "arguments = {" {
			inArgs = true
			continue
		}
		key, val, ok := strings.Cut(trimmed, " = ")
		if !ok {
			continue
		}
		switch key {
		case "path":
			d.PlistPath = val
		case "program":
			d.Program = val
		case "stdout path":
			d.StdoutPath = val
		case "stderr path":
			d.StderrPath = val
		case "disabled":
			if val == "true" {
				d.EnableState = Disabled
			} else if val == "false" {
				d.EnableState = Enabled
			}
		}
	}
	return d
}

type FailureKind int

const (
	FailureGeneric FailureKind = iota
	FailurePermission
)

var errnoPermRe = regexp.MustCompile(`\berrno (1|13)\b`)

var permPhrases = []string{
	"operation not permitted",
	"permission denied",
	"not privileged",
	"requires root",
}

// ClassifyFailure decides whether a non-zero launchctl result is a permission
// failure (→ offer sudo retry) or a generic one (→ show stderr).
func ClassifyFailure(exitCode int, stderr string) FailureKind {
	if exitCode == 0 {
		return FailureGeneric
	}
	low := strings.ToLower(stderr)
	for _, p := range permPhrases {
		if strings.Contains(low, p) {
			return FailurePermission
		}
	}
	if errnoPermRe.MatchString(low) {
		return FailurePermission
	}
	return FailureGeneric
}
