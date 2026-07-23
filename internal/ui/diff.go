package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gito/internal/git"
	"gito/internal/i18n"
	"gito/internal/style"
)

var (
	refStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#F1C40F")).Bold(true)
	refBaseStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E74C3C")).Bold(true)
	refTgtStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#2ECC71")).Bold(true)
)

type diffPane int

const (
	diffPanePick diffPane = iota
	diffPaneView
)

// The picker selects a base ref, then a target ref, then shows base..target.
type diffModel struct {
	refs   []string
	cursor int
	pane   diffPane

	base   string // selected base (empty until chosen)
	target string

	vp      viewport.Model
	vpReady bool

	errMsg        string
	width, height int
}

type diffContentMsg struct{ content string }

func doDiff(base, target string) tea.Cmd {
	return func() tea.Msg {
		out, err := git.GetDiffBetween(base, target)
		if err != nil {
			return diffContentMsg{"Error: " + err.Error()}
		}
		if strings.TrimSpace(out) == "" {
			return diffContentMsg{i18n.T("diff.none")}
		}
		return diffContentMsg{out}
	}
}

func (m diffModel) Init() tea.Cmd { return nil }

func (m diffModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.vpReady {
			m.vp.Width = msg.Width
			m.vp.Height = m.diffVPHeight()
		}

	case diffContentMsg:
		m.vp = viewport.New(m.width, m.diffVPHeight())
		m.vp.SetContent(msg.content)
		m.vpReady = true

	case tea.KeyMsg:
		if m.pane == diffPaneView {
			return m.updateView(msg)
		}
		return m.updatePick(msg)
	}
	return m, nil
}

func (m diffModel) diffVPHeight() int {
	h := m.height - 4
	if h < 1 {
		return 1
	}
	return h
}

func (m diffModel) updateView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.pane = diffPanePick
		m.vpReady = false
		m.base = ""
		m.target = ""
		return m, nil
	}
	if m.vpReady {
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m diffModel) updatePick(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		if m.base != "" {
			m.base = "" // step back to base selection
			return m, nil
		}
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.refs)-1 {
			m.cursor++
		}
	case "enter":
		if len(m.refs) == 0 {
			return m, nil
		}
		sel := m.refs[m.cursor]
		if m.base == "" {
			m.base = sel
			return m, nil
		}
		m.target = sel
		if m.base == m.target {
			m.errMsg = i18n.T("diff.same")
			return m, nil
		}
		m.errMsg = ""
		m.pane = diffPaneView
		m.vpReady = false
		return m, doDiff(m.base, m.target)
	}
	return m, nil
}

func (m diffModel) View() string {
	if m.pane == diffPaneView {
		return m.viewDiff()
	}
	return m.viewPick()
}

func (m diffModel) viewPick() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito diff"))

	step := i18n.T("diff.step_base")
	if m.base != "" {
		step = i18n.T("diff.step_target")
	}
	sb.WriteString(style.Dimmed.Render("  "+step) + "\n")
	sb.WriteString(style.Dimmed.Render(i18n.T("diff.hint_pick")) + "\n\n")

	if m.base != "" {
		sb.WriteString(style.Label.Render("base: ") + refBaseStyle.Render(m.base) + "\n\n")
	}
	if m.errMsg != "" {
		sb.WriteString(style.Failure.Render("! "+m.errMsg) + "\n\n")
	}

	if len(m.refs) == 0 {
		sb.WriteString(style.Dimmed.Render(i18n.T("diff.no_refs")) + "\n")
		return sb.String()
	}

	for i, r := range m.refs {
		disp := refStyle.Render(r)
		if i == m.cursor {
			sb.WriteString(cursorGlyp.Render("▶") + " " + selRowBg.Render(disp) + "\n")
		} else {
			sb.WriteString("  " + disp + "\n")
		}
	}
	return sb.String()
}

func (m diffModel) viewDiff() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito diff  ›  ") +
		refBaseStyle.Render(m.base) + style.Dimmed.Render(" .. ") + refTgtStyle.Render(m.target) + "\n")
	sb.WriteString(style.Dimmed.Render(i18n.T("hint.scroll_back")) + "\n\n")
	if !m.vpReady {
		sb.WriteString(style.Dimmed.Render("  " + i18n.T("common.loading")))
		return sb.String()
	}
	sb.WriteString(m.vp.View())
	return sb.String()
}

func RunDiff() {
	refs, err := git.GetRefs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if len(refs) == 0 {
		fmt.Println(i18n.T("diff.no_compare"))
		return
	}
	p := tea.NewProgram(diffModel{refs: refs}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
