package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"gito/internal/git"
	"gito/internal/style"
)

type logModel struct {
	viewport viewport.Model
	ready    bool
	content  string
}

func (m logModel) Init() tea.Cmd {
	return nil
}

func (m logModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		headerHeight := 3
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-headerHeight)
			m.viewport.SetContent(m.content)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - headerHeight
		}
	}
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m logModel) View() string {
	if !m.ready {
		return "\n  Loading..."
	}

	pct := int(m.viewport.ScrollPercent() * 100)
	header := style.Title.Render("gito log") +
		style.Dimmed.Render(fmt.Sprintf("  %d%%", pct)) + "\n" +
		style.Dimmed.Render("↑/↓ j/k: scroll   PgUp/PgDn: page   q: quit") + "\n\n"

	return header + m.viewport.View()
}

func RunLog() {
	content, err := git.GetLog(200)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if content == "" {
		fmt.Println("No commits yet.")
		return
	}

	// Strip ANSI for viewport (plain text display)
	plain := stripANSI(content)
	m := logModel{content: plain}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// stripANSI removes ANSI escape sequences from a string.
func stripANSI(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++ // skip 'm'
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String()
}
