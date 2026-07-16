package app

import "github.com/volkoffskij/launchdeck/internal/launchctl"

type Focus int

const (
	FocusSidebar Focus = iota
	FocusDetail
)

type Tab int

const (
	TabMetadata Tab = iota
	TabLogs
	TabRaw
)

type DomainScope int

const (
	ScopeUser DomainScope = iota
	ScopeSystem
	ScopeAll
)

type LoadState int

const (
	DetailIdle LoadState = iota
	DetailLoading
	DetailReady
	DetailError
)

type SudoKind int

const (
	SudoAction SudoKind = iota
	SudoInspect
	SudoEnumerate
)

type Filters struct {
	DomainScope DomainScope
	TextPattern string
}

type Scroll struct {
	List int
	Log  int
}

type LogLine struct {
	Stream string // "out" or "err"
	Text   string
}

type Detail struct {
	LoadState LoadState
	Metadata  launchctl.ServiceDetail
	Raw       string
	ErrMsg    string
}

type ActionPicker struct {
	Open            bool
	HighlightedVerb launchctl.ActionKind
}

type PendingConfirm struct {
	Active bool
	Action launchctl.ActionKind
	Target string // "<domain>/<label>" captured at prompt-open
}

type PendingSudo struct {
	Active bool
	Kind   SudoKind
	Target string
}

type LoadPrompt struct {
	Open       bool
	Buffer     string
	Candidates []string
	Highlight  int
}

// AppState is the whole application state. reduce is the only mutator.
type AppState struct {
	Services []launchctl.Service // domain-scoped scan result (unfiltered, unsorted)
	Selected string              // selected label ("" = none)
	Gone     bool                // selected label absent from the latest scan

	Filters       Filters
	FilterEditing bool
	FilterBuffer  string

	SortKey  SortKey
	SortDesc bool

	Scroll    Scroll
	Focus     Focus
	ActiveTab Tab

	Detail       Detail
	LogRing      []LogLine // capped at logRingCap
	TailIdentity string    // "<domain>/<label>" the current tail follows

	StatusMsg string

	ActionPicker   ActionPicker
	PendingConfirm PendingConfirm
	PendingSudo    PendingSudo
	LoadPrompt     LoadPrompt
	ActionRunning  bool
	SudoConfirmed  bool
	pendingAction  launchctl.ActionKind // verb behind ActionRunning; read via PendingAction()

	FirstScanDone     bool
	SelectionResolved bool

	UID int
}

func (s AppState) PendingAction() launchctl.ActionKind { return s.pendingAction }

const logRingCap = 5000

type SortKey int

const (
	SortLabel SortKey = iota
	SortStatus
	SortPID
)
