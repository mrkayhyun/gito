package ui

import tea "github.com/charmbracelet/bubbletea"

// keyMsg builds a synthetic key-press message for printable keys so tests can
// drive a model's Update directly without a running Bubble Tea program. The
// resulting msg.String() matches what the Update switch statements expect
// (e.g. "y", "n", "P", "D").
func keyMsg(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// enterKey and escKey are the special-key equivalents whose msg.String()
// yields "enter" and "esc" respectively.
func enterKey() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyEnter} }
func escKey() tea.KeyMsg   { return tea.KeyMsg{Type: tea.KeyEsc} }
func ctrlCKey() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyCtrlC} }
