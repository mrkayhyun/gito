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
	stashRefStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#F1C40F")).Bold(true)
	stashBranchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#27AE60"))
	stashMsgStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#ECECEC"))
)

// ── panes ─────────────────────────────────────────────────────────────────────

type stashPane int

const (
	stashPaneList stashPane = iota
	stashPaneShow
)

// ── model ─────────────────────────────────────────────────────────────────────

type stashModel struct {
	stashes []git.StashEntry
	cursor  int
	pane    stashPane

	vp      viewport.Model
	vpReady bool

	confirmDrop   bool
	errMsg        string
	successMsg    string
	width, height int
}

// ── messages ──────────────────────────────────────────────────────────────────

type stashListMsg struct{ stashes []git.StashEntry }
type stashErrMsg struct{ err error }
type stashShowMsg struct{ content string }

func doStashLoad() tea.Cmd {
	return func() tea.Msg {
		stashes, err := git.GetStashes()
		if err != nil {
			return stashErrMsg{err}
		}
		return stashListMsg{stashes}
	}
}

func doStashShow(ref string) tea.Cmd {
	return func() tea.Msg {
		content, err := git.StashShow(ref)
		if err != nil {
			return stashShowMsg{"Error: " + err.Error()}
		}
		if content == "" {
			return stashShowMsg{i18n.T("stash.empty_show")}
		}
		return stashShowMsg{content}
	}
}

// ── Init ─────────────────────────────────────────────────────────────────────

func (m stashModel) Init() tea.Cmd { return doStashLoad() }

// ── Update ───────────────────────────────────────────────────────────────────

func (m stashModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.vpReady {
			m.vp.Width = msg.Width
			m.vp.Height = m.stashVPHeight()
		}

	case stashListMsg:
		m.stashes = msg.stashes
		if m.cursor >= len(m.stashes) && len(m.stashes) > 0 {
			m.cursor = len(m.stashes) - 1
		}

	case stashErrMsg:
		m.errMsg = msg.err.Error()

	case stashShowMsg:
		m.vp = viewport.New(m.width, m.stashVPHeight())
		m.vp.SetContent(msg.content)
		m.vpReady = true

	case tea.KeyMsg:
		if m.pane == stashPaneShow {
			return m.updateShow(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

func (m stashModel) stashVPHeight() int {
	h := m.height - 4
	if h < 1 {
		return 1
	}
	return h
}

func (m stashModel) updateShow(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.pane = stashPaneList
		m.vpReady = false
		return m, nil
	}
	if m.vpReady {
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m stashModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// drop confirmation
	if m.confirmDrop {
		switch msg.String() {
		case "y", "Y":
			m.confirmDrop = false
			if m.cursor < len(m.stashes) {
				if err := git.StashDrop(m.stashes[m.cursor].Ref); err != nil {
					m.errMsg = err.Error()
					return m, nil
				}
				m.successMsg = i18n.T("stash.dropped") + m.stashes[m.cursor].Ref
			}
			return m, doStashLoad()
		default:
			m.confirmDrop = false
		}
		return m, nil
	}

	m.errMsg = ""
	m.successMsg = ""

	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.stashes)-1 {
			m.cursor++
		}
	case "enter", "p": // pop
		if m.cursor < len(m.stashes) {
			ref := m.stashes[m.cursor].Ref
			if err := git.StashPop(ref); err != nil {
				m.errMsg = err.Error()
				return m, nil
			}
			m.successMsg = i18n.T("stash.popped") + ref
			return m, doStashLoad()
		}
	case "a": // apply (keep stash)
		if m.cursor < len(m.stashes) {
			ref := m.stashes[m.cursor].Ref
			if err := git.StashApply(ref); err != nil {
				m.errMsg = err.Error()
				return m, nil
			}
			m.successMsg = i18n.T("stash.applied") + ref
		}
	case "d": // show diff
		if m.cursor < len(m.stashes) {
			m.pane = stashPaneShow
			m.vpReady = false
			return m, doStashShow(m.stashes[m.cursor].Ref)
		}
	case "D": // drop
		if m.cursor < len(m.stashes) {
			m.confirmDrop = true
		}
	}
	return m, nil
}

// ── View ─────────────────────────────────────────────────────────────────────

func (m stashModel) View() string {
	if m.pane == stashPaneShow {
		return m.viewShow()
	}
	return m.viewList()
}

func (m stashModel) viewList() string {
	var sb strings.Builder

	sb.WriteString(style.Title.Render("gito stash"))
	sb.WriteString(style.Dimmed.Render(fmt.Sprintf("  %d entries", len(m.stashes))) + "\n")
	sb.WriteString(style.Dimmed.Render(i18n.T("stash.hint_list")) + "\n\n")

	if m.confirmDrop && m.cursor < len(m.stashes) {
		sb.WriteString(style.Failure.Render(
			i18n.Tf("stash.drop_confirm", m.stashes[m.cursor].Ref),
		) + "\n")
		sb.WriteString(style.Label.Render(i18n.T("common.confirm_yn")) + "\n\n")
	}
	if m.errMsg != "" {
		sb.WriteString(style.Failure.Render("! "+m.errMsg) + "\n\n")
	}
	if m.successMsg != "" {
		sb.WriteString(style.Success.Render("✓ "+m.successMsg) + "\n\n")
	}

	if len(m.stashes) == 0 {
		sb.WriteString(style.Dimmed.Render(i18n.T("stash.none")) + "\n")
		return sb.String()
	}

	for i, s := range m.stashes {
		ref := stashRefStyle.Render(s.Ref)
		branch := ""
		if s.Branch != "" {
			branch = stashBranchStyle.Render(" ("+s.Branch+")") + " "
		}
		msg := stashMsgStyle.Render(s.Subject)

		if i == m.cursor {
			sb.WriteString(cursorGlyp.Render("▶") + " " + ref + " " + branch + msg + "\n")
		} else {
			sb.WriteString("  " + ref + " " + branch + msg + "\n")
		}
	}

	return sb.String()
}

func (m stashModel) viewShow() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito stash  ›  show") + "\n")
	if m.cursor < len(m.stashes) {
		s := m.stashes[m.cursor]
		sb.WriteString(stashRefStyle.Render(s.Ref) + "  " + stashMsgStyle.Render(s.Subject))
	}
	sb.WriteString(style.Dimmed.Render(i18n.T("hint.scroll_back")) + "\n\n")

	if !m.vpReady {
		sb.WriteString(style.Dimmed.Render("  " + i18n.T("common.loading")))
		return sb.String()
	}
	sb.WriteString(m.vp.View())
	return sb.String()
}

// ── RunStash ─────────────────────────────────────────────────────────────────

func RunStash() {
	p := tea.NewProgram(stashModel{}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
