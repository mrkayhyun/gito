package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"gito/internal/git"
	"gito/internal/style"
)

type branchModel struct {
	branches []string
	current  string
	filter   textinput.Model
	cursor   int
	err      error
	done     bool
	switched string
}

func newBranchModel(branches []string, current string) branchModel {
	filter := textinput.New()
	filter.Placeholder = "filter branches..."
	filter.CharLimit = 100
	cmd := filter.Focus()
	_ = cmd

	return branchModel{
		branches: branches,
		current:  current,
		filter:   filter,
	}
}

func (m branchModel) filteredBranches() []string {
	query := strings.ToLower(m.filter.Value())
	if query == "" {
		return m.branches
	}
	var result []string
	for _, b := range m.branches {
		if strings.Contains(strings.ToLower(b), query) {
			result = append(result, b)
		}
	}
	return result
}

func (m branchModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m branchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		filtered := m.filteredBranches()
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "ctrl+n":
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}
			return m, nil
		case "enter":
			if len(filtered) > 0 {
				selected := filtered[m.cursor]
				if selected == m.current {
					return m, tea.Quit
				}
				err := git.SwitchBranch(selected)
				m.err = err
				if err == nil {
					m.switched = selected
					m.done = true
				}
				return m, tea.Quit
			}
			return m, nil
		}

		prevFilter := m.filter.Value()
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		if m.filter.Value() != prevFilter {
			m.cursor = 0
		}
		return m, cmd
	}

	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	return m, cmd
}

func (m branchModel) View() string {
	var sb strings.Builder

	sb.WriteString(style.Title.Render("gito branch") + "\n\n")
	sb.WriteString(style.Label.Render("Search: ") + m.filter.View() + "\n\n")

	filtered := m.filteredBranches()
	if len(filtered) == 0 {
		sb.WriteString(style.Dimmed.Render("  No branches found") + "\n")
	} else {
		for i, b := range filtered {
			prefix := "  "
			if b == m.current {
				prefix = "* "
			}
			if i == m.cursor {
				sb.WriteString(style.Selected.Render(prefix+b) + "\n")
			} else if b == m.current {
				sb.WriteString(style.Success.Render(prefix+b) + "\n")
			} else {
				sb.WriteString(style.Normal.Render(prefix+b) + "\n")
			}
		}
	}

	sb.WriteString("\n" + style.Dimmed.Render("↑/↓: 이동   enter: 브랜치 전환   esc: 종료"))
	return sb.String()
}

func RunBranch() {
	branches, current, err := git.GetBranches()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	m := newBranchModel(branches, current)
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	final := result.(branchModel)
	if final.err != nil {
		fmt.Fprintln(os.Stderr, style.Failure.Render("Switch failed: "+final.err.Error()))
		os.Exit(1)
	}
	if final.done {
		fmt.Println(style.Success.Render("✓ Switched to: " + final.switched))
	}
}
