package bubbletea

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
)

// Theme holds the interface colors. Each field is an ANSI/hex color string as
// lipgloss understands it (e.g. "240", "#8be9fd"). It is loaded from
// ~/.config/launchdeck/theme.json; any absent field keeps its default, so a
// partial file only overrides what it names.
type Theme struct {
	Border      string `json:"border"`
	SelectedFg  string `json:"selected_fg"`
	SelectedBg  string `json:"selected_bg"`
	Running     string `json:"running"`
	Stopped     string `json:"stopped"`
	Gone        string `json:"gone"`
	TabActiveFg string `json:"tab_active_fg"`
	TabActiveBg string `json:"tab_active_bg"`
	GutterFg    string `json:"gutter_fg"`
	GutterBg    string `json:"gutter_bg"`
	Accent      string `json:"accent"`
	Muted       string `json:"muted"`
}

// DefaultTheme is the built-in palette used when there is no theme file (or for
// any field a partial file omits).
func DefaultTheme() Theme {
	return Theme{
		Border:      "240",
		SelectedFg:  "231",
		SelectedBg:  "62",
		Running:     "42",
		Stopped:     "244",
		Gone:        "203",
		TabActiveFg: "231",
		TabActiveBg: "62",
		GutterFg:    "250",
		GutterBg:    "238",
		Accent:      "213",
		Muted:       "244",
	}
}

// ThemePath returns ~/.config/launchdeck/theme.json.
func ThemePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "launchdeck", "theme.json"), nil
}

// LoadTheme reads the theme file over the defaults. A missing or corrupt file
// yields the full default theme (never an error) so the UI always has colors.
func LoadTheme(path string) Theme {
	t := DefaultTheme()
	data, err := os.ReadFile(path)
	if err != nil {
		return t
	}
	_ = json.Unmarshal(data, &t) // absent keys keep defaults; a parse error keeps them all
	return t
}

func (t Theme) sel() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(t.SelectedFg)).Background(lipgloss.Color(t.SelectedBg))
}
func (t Theme) border() lipgloss.Color { return lipgloss.Color(t.Border) }
func (t Theme) gone() lipgloss.Style   { return lipgloss.NewStyle().Foreground(lipgloss.Color(t.Gone)) }
func (t Theme) tabActive() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(t.TabActiveFg)).Background(lipgloss.Color(t.TabActiveBg))
}
func (t Theme) gutter() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(t.GutterFg)).Background(lipgloss.Color(t.GutterBg))
}
func (t Theme) accent() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(t.Accent))
}
func (t Theme) muted() lipgloss.Style { return lipgloss.NewStyle().Foreground(lipgloss.Color(t.Muted)) }
