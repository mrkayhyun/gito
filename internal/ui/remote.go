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
	remoteNameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F1C40F")).Bold(true)
	remoteURLStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#ECECEC"))
	aheadStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#2ECC71")).Bold(true)
	behindStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#E74C3C")).Bold(true)
)

type remotePane int

const (
	remotePaneList remotePane = iota
	remotePaneOutput
)

type remoteModel struct {
	remotes     []git.RemoteEntry
	cursor      int
	pane        remotePane
	ahead       int
	behind      int
	hasUpstream bool

	vp      viewport.Model
	vpReady bool

	loading       bool
	errMsg        string
	successMsg    string
	width, height int
}

type remoteListMsg struct {
	remotes     []git.RemoteEntry
	ahead       int
	behind      int
	hasUpstream bool
}
type remoteErrMsg struct{ err error }
type remoteFetchMsg struct{ output string }

func doRemoteLoad() tea.Cmd {
	return func() tea.Msg {
		remotes, err := git.GetRemotes()
		if err != nil {
			return remoteErrMsg{err}
		}
		ahead, behind, has := git.GetAheadBehind()
		return remoteListMsg{remotes, ahead, behind, has}
	}
}

func doFetch(remote string) tea.Cmd {
	return func() tea.Msg {
		out, err := git.Fetch(remote)
		if err != nil {
			return remoteErrMsg{err}
		}
		return remoteFetchMsg{out}
	}
}

func (m remoteModel) Init() tea.Cmd { return doRemoteLoad() }

func (m remoteModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.vpReady {
			m.vp.Width = msg.Width
			m.vp.Height = m.remoteVPHeight()
		}

	case remoteListMsg:
		m.remotes = msg.remotes
		m.ahead, m.behind, m.hasUpstream = msg.ahead, msg.behind, msg.hasUpstream
		m.loading = false
		if m.cursor >= len(m.remotes) && len(m.remotes) > 0 {
			m.cursor = len(m.remotes) - 1
		}

	case remoteErrMsg:
		m.errMsg = msg.err.Error()
		m.loading = false

	case remoteFetchMsg:
		m.loading = false
		m.pane = remotePaneOutput
		m.vp = viewport.New(m.width, m.remoteVPHeight())
		m.vp.SetContent(msg.output)
		m.vpReady = true
		return m, doRemoteLoad()

	case tea.KeyMsg:
		if m.pane == remotePaneOutput {
			return m.updateOutput(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

func (m remoteModel) remoteVPHeight() int {
	h := m.height - 4
	if h < 1 {
		return 1
	}
	return h
}

func (m remoteModel) updateOutput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.pane = remotePaneList
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

func (m remoteModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.errMsg = ""
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.remotes)-1 {
			m.cursor++
		}
	case "f": // fetch selected remote
		if m.cursor < len(m.remotes) {
			m.loading = true
			m.successMsg = ""
			return m, doFetch(m.remotes[m.cursor].Name)
		}
	case "F": // fetch all
		m.loading = true
		m.successMsg = ""
		return m, doFetch("")
	case "r": // refresh ahead/behind
		return m, doRemoteLoad()
	}
	return m, nil
}

func (m remoteModel) View() string {
	if m.pane == remotePaneOutput {
		return m.viewOutput()
	}
	return m.viewList()
}

func (m remoteModel) viewList() string {
	var sb strings.Builder

	sb.WriteString(style.Title.Render("gito remote"))
	sb.WriteString(style.Dimmed.Render(fmt.Sprintf("  %d remotes", len(m.remotes))) + "\n")
	sb.WriteString(style.Dimmed.Render(i18n.T("remote.hint_list")) + "\n\n")

	// upstream ahead/behind summary for current branch
	if m.hasUpstream {
		sb.WriteString(style.Label.Render(i18n.T("remote.upstream_status")))
		sb.WriteString(aheadStyle.Render(fmt.Sprintf("↑%d", m.ahead)) + " ")
		sb.WriteString(behindStyle.Render(fmt.Sprintf("↓%d", m.behind)))
		if m.ahead == 0 && m.behind == 0 {
			sb.WriteString(style.Success.Render(i18n.T("remote.up_to_date")))
		}
		sb.WriteString("\n\n")
	} else {
		sb.WriteString(style.Dimmed.Render(i18n.T("remote.no_upstream")) + "\n\n")
	}

	if m.loading {
		sb.WriteString(style.Label.Render(i18n.T("remote.fetching")) + "\n\n")
	}
	if m.errMsg != "" {
		sb.WriteString(style.Failure.Render("! "+m.errMsg) + "\n\n")
	}
	if m.successMsg != "" {
		sb.WriteString(style.Success.Render("✓ "+m.successMsg) + "\n\n")
	}

	if len(m.remotes) == 0 {
		sb.WriteString(style.Dimmed.Render(i18n.T("remote.none")) + "\n")
		return sb.String()
	}

	for i, r := range m.remotes {
		row := remoteNameStyle.Render(r.Name) + "  " + remoteURLStyle.Render(r.FetchURL)
		if i == m.cursor {
			sb.WriteString(style.Cursor.Render(style.G.Cursor) + " " + style.RowSel.Render(row) + "\n")
		} else {
			sb.WriteString("  " + row + "\n")
		}
	}
	return sb.String()
}

func (m remoteModel) viewOutput() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito remote  ›  fetch") + "\n")
	sb.WriteString(style.Dimmed.Render(i18n.T("hint.scroll_back")) + "\n\n")
	if !m.vpReady {
		sb.WriteString(style.Dimmed.Render("  " + i18n.T("common.loading")))
		return sb.String()
	}
	sb.WriteString(m.vp.View())
	return sb.String()
}

func RunRemote() {
	p := tea.NewProgram(remoteModel{}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
