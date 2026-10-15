package style

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Palette. Every color adapts to the terminal background: Dark is the
// original gito color, Light is a darker/more saturated counterpart that stays
// readable on white. lipgloss picks one at render time (see SetTheme).
var (
	Accent    = lipgloss.AdaptiveColor{Light: "#5A3FC0", Dark: "#7D56F4"}
	OnAccent  = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#FAFAFA"}
	Text      = lipgloss.AdaptiveColor{Light: "#1A1A1A", Dark: "#ECECEC"}
	TextSoft  = lipgloss.AdaptiveColor{Light: "#333333", Dark: "#DDDDDD"}
	Muted     = lipgloss.AdaptiveColor{Light: "#8A8A8A", Dark: "#626262"}
	MutedAlt  = lipgloss.AdaptiveColor{Light: "#8A8A8A", Dark: "#636363"}
	LabelGray = lipgloss.AdaptiveColor{Light: "#555555", Dark: "#A9A9A9"}
	Yellow    = lipgloss.AdaptiveColor{Light: "#9A7D00", Dark: "#F1C40F"}
	Green     = lipgloss.AdaptiveColor{Light: "#1E8449", Dark: "#27AE60"}
	GreenLite = lipgloss.AdaptiveColor{Light: "#1E8449", Dark: "#2ECC71"}
	GreenOK   = lipgloss.AdaptiveColor{Light: "#027A50", Dark: "#04B575"}
	Red       = lipgloss.AdaptiveColor{Light: "#B03A2E", Dark: "#E74C3C"}
	Pink      = lipgloss.AdaptiveColor{Light: "#C2185B", Dark: "#FF5F87"}
	Blue      = lipgloss.AdaptiveColor{Light: "#1F618D", Dark: "#3498DB"}
	Purple    = lipgloss.AdaptiveColor{Light: "#6C3483", Dark: "#9B59B6"}
	PurpleDk  = lipgloss.AdaptiveColor{Light: "#5B2C6F", Dark: "#8E44AD"}
	Orange    = lipgloss.AdaptiveColor{Light: "#9C640C", Dark: "#F39C12"}
	OrangeDk  = lipgloss.AdaptiveColor{Light: "#A04000", Dark: "#E67E22"}
	SelBg     = lipgloss.AdaptiveColor{Light: "#E2DCFA", Dark: "#312E55"}
	PanelBg   = lipgloss.AdaptiveColor{Light: "#F0EDFB", Dark: "#1E1B2E"}
)

var (
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(Accent).
		Padding(0, 1)

	Selected = lipgloss.NewStyle().
			Bold(true).
			Foreground(OnAccent).
			Background(Accent).
			Padding(0, 1)

	Normal = lipgloss.NewStyle().
		Foreground(TextSoft).
		Padding(0, 1)

	Dimmed = lipgloss.NewStyle().
		Foreground(Muted).
		Padding(0, 1)

	Success = lipgloss.NewStyle().
		Bold(true).
		Foreground(GreenOK)

	Failure = lipgloss.NewStyle().
		Bold(true).
		Foreground(Pink)

	Label = lipgloss.NewStyle().
		Bold(true).
		Foreground(LabelGray)

	Border = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Accent).
		Padding(0, 1)
)

// SetTheme forces the palette variant. "light" and "dark" override lipgloss's
// background detection (useful when the terminal does not answer the query,
// e.g. over some SSH/tmux setups); anything else, including "" and "auto",
// keeps auto-detection. It reports whether the value was recognized.
func SetTheme(theme string) bool {
	switch strings.ToLower(strings.TrimSpace(theme)) {
	case "light":
		lipgloss.SetHasDarkBackground(false)
	case "dark":
		lipgloss.SetHasDarkBackground(true)
	case "", "auto":
		// Resolve detection now, before any Bubble Tea program owns stdin:
		// the background query reads the terminal's reply from stdin and
		// would otherwise race the TUI's input reader on first render.
		lipgloss.SetHasDarkBackground(lipgloss.HasDarkBackground())
	default:
		return false
	}
	return true
}
