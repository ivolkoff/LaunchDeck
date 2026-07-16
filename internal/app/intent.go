package app

import "github.com/volkoffskij/launchdeck/internal/launchctl"

// Msg is anything reduce ingests: a user Intent or an async data message.
type Msg interface{ isMsg() }

type base struct{}

func (base) isMsg() {}

// --- User intents ---

type SelectService struct {
	base
	Label string
}
type MoveSelection struct {
	base
	Delta           int
	ToTop, ToBottom bool
}                              // Delta ±1 or ±page
type FocusPanel struct{ base } // toggle sidebar↔detail
type SetTab struct {
	base
	Tab Tab
}
type ScrollMsg struct {
	base
	Panel Focus
	Delta int
}

type RunAction struct {
	base
	Action launchctl.ActionKind
}
type ConfirmAction struct{ base }
type CancelAction struct{ base }

type OpenActionPicker struct{ base }
type MoveActionPicker struct {
	base
	Delta int
}
type PickAction struct {
	base
	Action launchctl.ActionKind
}
type CancelActionPicker struct{ base }

type OpenFilter struct{ base }
type SetFilterBuffer struct {
	base
	Buffer string
}
type CommitFilter struct{ base }
type CancelFilter struct{ base }
type CycleDomainScope struct{ base }
type SetFilter struct {
	base
	Filters Filters
} // internal, emitted by commit/cycle

type SetSort struct {
	base
	ToggleDir bool
} // false = cycle key, true = toggle direction

type OpenLoadPrompt struct{ base }
type SetLoadBuffer struct {
	base
	Buffer string
}
type SubmitLoad struct{ base }
type CancelLoad struct{ base }

type ConfirmSudo struct{ base }
type CancelSudo struct{ base }

type Refresh struct{ base }
type Quit struct{ base }

// --- Async data messages ---

type ServicesLoaded struct {
	base
	Services []launchctl.Service
	Err      *launchctl.ScanError // non-nil → keep prior list; permission → enumerate banner
}

type ServiceDetailLoaded struct {
	base
	Target string // "<domain>/<label>" the fetch was for
	Detail launchctl.ServiceDetail
	Err    *launchctl.ScanError
}

type LogLinesAppended struct {
	base
	TailTarget string
	Lines      []LogLine
	State      string // "", "removed", "unreadable"
}

type ActionResult struct {
	base
	Action   launchctl.ActionKind
	Target   string
	Outcome  launchctl.ActionOutcome
	TimedOut bool
}
