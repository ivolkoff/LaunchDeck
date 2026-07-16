// Package launchctl wraps the macOS launchctl CLI: enumeration, inspection,
// and lifecycle actions. It has zero TUI dependencies.
package launchctl

import "strconv"

// Domain is a launchd domain target. Kind is "gui" or "system".
type Domain struct {
	Kind string
	UID  int // meaningful only when Kind == "gui"
}

func GUIDomain(uid int) Domain { return Domain{Kind: "gui", UID: uid} }
func SystemDomain() Domain     { return Domain{Kind: "system"} }

func (d Domain) String() string {
	if d.Kind == "gui" {
		return "gui/" + strconv.Itoa(d.UID)
	}
	return "system"
}

// Target builds the "<domain>/<label>" specifier launchctl verbs take.
func (d Domain) Target(label string) string { return d.String() + "/" + label }

type RunState int

const (
	Stopped RunState = iota
	Running
)

type EnableState int

const (
	EnableUnknown EnableState = iota
	Enabled
	Disabled
)

// Service is one row from a domain scan.
type Service struct {
	Label    string
	Domain   Domain
	PID      int // 0 when HasPID is false
	HasPID   bool
	LastExit int
}

func (s Service) RunState() RunState {
	if s.HasPID {
		return Running
	}
	return Stopped
}

// ServiceDetail is the parsed `launchctl print <domain>/<label>` dump.
type ServiceDetail struct {
	Service
	Program     string
	Args        []string
	PlistPath   string
	StdoutPath  string
	StderrPath  string
	EnableState EnableState
	Raw         string // the full dump, always populated
}

type ActionKind int

const (
	ActionStart   ActionKind = iota // kickstart
	ActionRestart                   // kickstart -k
	ActionStop                      // kill TERM
	ActionEnable                    // enable
	ActionDisable                   // disable
	ActionUnload                    // bootout
	ActionLoad                      // bootstrap (uses a plist path, not a label target)
)

func (a ActionKind) String() string {
	switch a {
	case ActionStart:
		return "start"
	case ActionRestart:
		return "restart"
	case ActionStop:
		return "stop"
	case ActionEnable:
		return "enable"
	case ActionDisable:
		return "disable"
	case ActionUnload:
		return "unload"
	case ActionLoad:
		return "load"
	default:
		return "unknown"
	}
}

// Destructive reports whether the action needs an in-TUI confirm.
func (a ActionKind) Destructive() bool {
	return a == ActionStop || a == ActionDisable || a == ActionUnload
}
