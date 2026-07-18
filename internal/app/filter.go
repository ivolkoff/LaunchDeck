package app

import (
	"sort"
	"strings"

	"github.com/ivolkoff/launchdeck/internal/launchctl"
)

func applyFilter(in []launchctl.Service, f Filters, uid int) []launchctl.Service {
	pat := strings.ToLower(f.TextPattern)
	out := make([]launchctl.Service, 0, len(in))
	for _, s := range in {
		switch f.DomainScope {
		case ScopeUser:
			if s.Domain.Kind != "gui" {
				continue
			}
		case ScopeSystem:
			if s.Domain.Kind != "system" {
				continue
			}
		}
		if pat != "" && !strings.Contains(strings.ToLower(s.Label), pat) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// labelLess is the canonical secondary order: case-insensitive, then bytewise.
func labelLess(a, b string) bool {
	la, lb := strings.ToLower(a), strings.ToLower(b)
	if la != lb {
		return la < lb
	}
	return a < b
}

func applySort(in []launchctl.Service, key SortKey, desc bool) []launchctl.Service {
	out := make([]launchctl.Service, len(in))
	copy(out, in)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		var less bool
		switch key {
		case SortLabel:
			if labelLess(a.Label, b.Label) != labelLess(b.Label, a.Label) {
				return labelLess(a.Label, b.Label) != desc
			}
			return false
		case SortStatus:
			if a.HasPID != b.HasPID {
				// running-before-stopped for ascending; direction flips it.
				less = a.HasPID && !b.HasPID
				if desc {
					return !less
				}
				return less
			}
		case SortPID:
			if a.HasPID != b.HasPID {
				return a.HasPID // null PIDs always last regardless of direction
			}
			if a.HasPID && a.PID != b.PID {
				if desc {
					return a.PID > b.PID
				}
				return a.PID < b.PID
			}
		}
		return labelLess(a.Label, b.Label) // secondary tie-break, always ascending
	})
	return out
}
