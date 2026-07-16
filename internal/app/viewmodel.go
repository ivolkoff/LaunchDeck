package app

type RowVM struct {
	Label    string
	Domain   string
	Running  bool
	Selected bool
	Gone     bool
}

type ListVM struct {
	Rows        []RowVM
	Placeholder string // non-empty → render this instead of rows
	SelectedIdx int
}

type DetailVM struct {
	Mode      string // "empty" | "loading" | "ready" | "error" | "gone"
	ActiveTab Tab
	// Metadata tab
	Label, Domain, PID, LastExit, RunState, EnableState, Program, Plist string
	// Logs tab
	LogLines []string // already prefixed [out]/[err]
	LogNote  string   // "no log configured" | "log removed" | ...
	// Raw tab
	Raw string
	Err string
}

type StatusVM struct {
	Message string
	Prompt  string   // active confirm/sudo/filter/load prompt text ("" if none)
	Buttons []string // action-button labels
}

type ViewModel struct {
	List   ListVM
	Detail DetailVM
	Status StatusVM
}
