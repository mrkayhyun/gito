package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mrkayhyun/gito/internal/git"
	"github.com/mrkayhyun/gito/internal/i18n"
	"github.com/mrkayhyun/gito/internal/style"
)

var (
	remoteNameStyle = lipgloss.NewStyle().Foreground(style.Yellow).Bold(true)
	remoteURLStyle  = lipgloss.NewStyle().Foreground(style.Text)
	aheadStyle      = lipgloss.NewStyle().Foreground(style.GreenLite).Bold(true)
	behindStyle     = lipgloss.NewStyle().Foreground(style.Red).Bold(true)
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

	branch string // current branch; "" when HEAD is detached

	// confirmPush is set while "push?" awaits y; pushRemote is the remote
	// a branch without upstream will be pushed to (and tracked on).
	confirmPush bool
	pushRemote  string

	loading       bool
	running       string // git subcommand in flight, for the status line
	outputTitle   string
	errMsg        string
	successMsg    string
	width, height int
}

type remoteListMsg struct {
	remotes     []git.RemoteEntry
	ahead       int
	behind      int
	hasUpstream bool
	branch      string
}
type remoteErrMsg struct{ err error }

// remoteOutputMsg carries the output of a fetch/push/pull for the output pane.
type remoteOutputMsg struct{ title, output string }

func doRemoteLoad() tea.Cmd {
	return func() tea.Msg {
		remotes, err := git.GetRemotes()
		if err != nil {
			return remoteErrMsg{err}
		}
		ahead, behind, has := git.GetAheadBehind()
		branch, _ := git.CurrentBranch()
		return remoteListMsg{remotes, ahead, behind, has, branch}
	}
}

func doFetch(remote string) tea.Cmd {
	return func() tea.Msg {
		out, err := git.Fetch(remote)
		if err != nil {
			return remoteErrMsg{err}
		}
		return remoteOutputMsg{"fetch", out}
	}
}

func doPush(remote string) tea.Cmd {
	return func() tea.Msg {
		out, err := git.Push(remote)
		if err != nil {
			return remoteErrMsg{err}
		}
		return remoteOutputMsg{"push", out}
	}
}

func doPull() tea.Cmd {
	return func() tea.Msg {
		out, err := git.Pull()
		if err != nil {
			return remoteErrMsg{err}
		}
		return remoteOutputMsg{"pull", out}
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
		m.branch = msg.branch
		m.loading = false
		if m.cursor >= len(m.remotes) && len(m.remotes) > 0 {
			m.cursor = len(m.remotes) - 1
		}

	case remoteErrMsg:
		m.errMsg = msg.err.Error()
		m.loading = false
		// A failed push/pull may still have fetched or changed refs.
		return m, doRemoteLoad()

	case remoteOutputMsg:
		m.loading = false
		m.outputTitle = msg.title
		m.pane = remotePaneOutput
		m.vp = viewport.New(m.width, m.remoteVPHeight())
		m.vp.SetContent(msg.output)
		m.vpReady = true
		return m, doRemoteLoad()

	case tea.KeyMsg:
		if m.pane == remotePaneOutput {
			return m.updateOutput(msg)
		}
		if m.confirmPush {
			return m.updateConfirmPush(msg)
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

// updateConfirmPush handles the y/other answer to the push prompt. Pushing
// publishes commits, so it always asks first; esc or any other key cancels.
func (m remoteModel) updateConfirmPush(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.confirmPush = false
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if msg.String() != "y" {
		return m, nil
	}
	return m.start("push", doPush(m.pushRemote))
}

func (m remoteModel) start(op string, cmd tea.Cmd) (tea.Model, tea.Cmd) {
	m.loading = true
	m.running = op
	m.successMsg = ""
	return m, cmd
}

// requestPush validates that a push makes sense and, if so, opens the
// confirmation prompt.
func (m remoteModel) requestPush() (tea.Model, tea.Cmd) {
	switch {
	case m.branch == "":
		m.errMsg = i18n.T("remote.detached")
	case m.hasUpstream && m.behind > 0:
		m.errMsg = i18n.Tf("remote.pull_first", m.behind)
	case m.hasUpstream && m.ahead == 0:
		m.successMsg = i18n.T("remote.nothing_to_push")
	case !m.hasUpstream && m.cursor >= len(m.remotes):
		m.errMsg = i18n.T("remote.none")
	default:
		m.pushRemote = ""
		if !m.hasUpstream {
			m.pushRemote = m.remotes[m.cursor].Name
		}
		m.confirmPush = true
	}
	return m, nil
}

func (m remoteModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.loading {
		// One git operation at a time; only quitting is allowed meanwhile.
		if k := msg.String(); k == "ctrl+c" || k == "q" || k == "esc" {
			return m, tea.Quit
		}
		return m, nil
	}
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
			return m.start("fetch", doFetch(m.remotes[m.cursor].Name))
		}
	case "F": // fetch all
		return m.start("fetch", doFetch(""))
	case "p": // push current branch (asks first)
		m.successMsg = ""
		return m.requestPush()
	case "l": // fast-forward pull
		m.successMsg = ""
		switch {
		case m.branch == "":
			m.errMsg = i18n.T("remote.detached")
		case !m.hasUpstream:
			m.errMsg = i18n.T("remote.pull_needs_upstream")
		default:
			return m.start("pull", doPull())
		}
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

	if m.confirmPush {
		var q string
		if m.pushRemote == "" {
			q = i18n.Tf("remote.confirm_push", m.ahead, m.branch)
		} else {
			q = i18n.Tf("remote.confirm_push_new", m.branch, m.pushRemote)
		}
		sb.WriteString(style.Label.Render(q) + "\n")
		sb.WriteString(style.Dimmed.Render(i18n.T("common.confirm_yn")) + "\n\n")
	}
	if m.loading {
		sb.WriteString(style.Label.Render(i18n.Tf("remote.running", m.running)) + "\n\n")
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
			sb.WriteString(cursorGlyp.Render("▶") + " " + selRowBg.Render(row) + "\n")
		} else {
			sb.WriteString("  " + row + "\n")
		}
	}
	return sb.String()
}

func (m remoteModel) viewOutput() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito remote  ›  "+m.outputTitle) + "\n")
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
