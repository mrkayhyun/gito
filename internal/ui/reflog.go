package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gito/internal/git"
	"gito/internal/i18n"
	"gito/internal/style"
)

var (
	reflogHashStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F1C40F")).Bold(true)
	reflogSelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#8E44AD"))
	reflogActStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#27AE60"))
	reflogSubStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#ECECEC"))
)

type reflogMode int

const (
	reflogModeList reflogMode = iota
	reflogModeBranch
)

type reflogModel struct {
	entries []git.ReflogEntry
	cursor  int
	offset  int
	mode    reflogMode

	input textinput.Model // new branch name for recovery

	errMsg        string
	successMsg    string
	width, height int
}

func (m reflogModel) Init() tea.Cmd { return nil }

func (m reflogModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		if m.mode == reflogModeBranch {
			return m.updateBranch(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

func (m reflogModel) visibleRows() int {
	v := m.height - 4
	if v < 1 {
		return 10
	}
	return v
}

func (m reflogModel) updateBranch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode = reflogModeList
		m.input.Blur()
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.input.Value())
		if name == "" {
			m.errMsg = i18n.T("branch.err_name_required")
			return m, nil
		}
		if m.cursor < len(m.entries) {
			ref := m.entries[m.cursor].Selector
			if err := git.CreateBranchAt(name, ref); err != nil {
				m.errMsg = err.Error()
				return m, nil
			}
			m.successMsg = i18n.Tf("reflog.created_at", name, ref)
		}
		m.mode = reflogModeList
		m.input.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m reflogModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.errMsg = ""
	vis := m.visibleRows()
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.offset {
				m.offset = m.cursor
			}
		}
	case "down", "j":
		if m.cursor < len(m.entries)-1 {
			m.cursor++
			if m.cursor >= m.offset+vis {
				m.offset = m.cursor - vis + 1
			}
		}
	case "g":
		m.cursor, m.offset = 0, 0
	case "G":
		m.cursor = len(m.entries) - 1
		if m.cursor-vis+1 > 0 {
			m.offset = m.cursor - vis + 1
		}
	case "b": // recover: create branch at this reflog entry (non-destructive)
		if m.cursor < len(m.entries) {
			m.mode = reflogModeBranch
			m.successMsg = ""
			m.input.SetValue("")
			return m, m.input.Focus()
		}
	}
	return m, nil
}

func (m reflogModel) View() string {
	if m.mode == reflogModeBranch {
		return m.viewBranch()
	}
	return m.viewList()
}

func (m reflogModel) viewBranch() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito reflog  ›  recover") + "\n\n")
	if m.cursor < len(m.entries) {
		e := m.entries[m.cursor]
		sb.WriteString(style.Label.Render(i18n.T("reflog.target")) +
			reflogHashStyle.Render(e.Short) + " " +
			reflogSelStyle.Render(e.Selector) + " " +
			reflogSubStyle.Render(e.Subject) + "\n\n")
	}
	sb.WriteString(style.Label.Render(i18n.T("reflog.new_branch_name")) + "\n\n")
	sb.WriteString(m.input.View() + "\n")
	if m.errMsg != "" {
		sb.WriteString("\n" + style.Failure.Render("! "+m.errMsg) + "\n")
	}
	sb.WriteString("\n" + style.Dimmed.Render(i18n.T("reflog.hint_recover")))
	return sb.String()
}

func (m reflogModel) viewList() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito reflog"))
	sb.WriteString(style.Dimmed.Render(fmt.Sprintf("  %d entries", len(m.entries))) + "\n")
	sb.WriteString(style.Dimmed.Render(i18n.T("reflog.hint_list")) + "\n\n")

	if m.errMsg != "" {
		sb.WriteString(style.Failure.Render("! "+m.errMsg) + "\n\n")
	}
	if m.successMsg != "" {
		sb.WriteString(style.Success.Render("✓ "+m.successMsg) + "\n\n")
	}

	if len(m.entries) == 0 {
		sb.WriteString(style.Dimmed.Render(i18n.T("reflog.no_entries")) + "\n")
		return sb.String()
	}

	vis := m.visibleRows()
	end := m.offset + vis
	if end > len(m.entries) {
		end = len(m.entries)
	}
	for i := m.offset; i < end; i++ {
		e := m.entries[i]
		action := e.Action
		if len([]rune(action)) > 24 {
			action = string([]rune(action)[:23]) + "…"
		}
		row := reflogHashStyle.Render(e.Short) + " " +
			reflogSelStyle.Render(fmt.Sprintf("%-12s", e.Selector)) + " " +
			reflogActStyle.Render(fmt.Sprintf("%-24s", action)) + " " +
			reflogSubStyle.Render(e.Subject)
		if i == m.cursor {
			sb.WriteString(cursorGlyp.Render("▶") + " " + selRowBg.Render(row) + "\n")
		} else {
			sb.WriteString("  " + row + "\n")
		}
	}
	return sb.String()
}

func RunReflog() {
	entries, err := git.GetReflog(500)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if len(entries) == 0 {
		fmt.Println(i18n.T("reflog.none"))
		return
	}
	input := textinput.New()
	input.Placeholder = "recovered-branch"
	input.CharLimit = 100

	p := tea.NewProgram(reflogModel{entries: entries, input: input}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
