package style

import "github.com/charmbracelet/lipgloss"

var (
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	Selected = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	Normal = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#DDDDDD")).
		Padding(0, 1)

	Dimmed = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Padding(0, 1)

	Success = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#04B575"))

	Failure = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF5F87"))

	Label = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#A9A9A9"))

	Border = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(0, 1)
)
