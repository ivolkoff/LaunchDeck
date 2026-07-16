package launchctl

import (
	"bufio"
	"errors"
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
