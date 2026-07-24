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

// ── lipgloss styles for the commit list ──────────────────────────────────────

var (
	hashStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#F1C40F")).Bold(true)
	dateStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#27AE60"))
	msgStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#ECECEC"))
	authorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#636363"))

	selBar     = lipgloss.NewStyle().Background(lipgloss.Color("#312E55"))
	selHash    = hashStyle.Background(lipgloss.Color("#312E55"))
	selDate    = dateStyle.Background(lipgloss.Color("#312E55"))
	selMsg     = msgStyle.Bold(true).Background(lipgloss.Color("#312E55"))
	selAuthor  = authorStyle.Background(lipgloss.Color("#312E55"))
	cursorGlyp = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4")).Bold(true)
)

// ── model ────────────────────────────────────────────────────────────────────

type logPane int

const (
	paneList   logPane = iota
	paneDetail         // showing git show output
)

type logModel struct {
	commits []git.CommitEntry
	cursor  int
	offset  int // first visible row index
	pane    logPane

	vp        viewport.Model
	vpReady   bool
	vpContent string // raw content waiting to be set when vp is init'd

	width  int
	height int
}

// ── tea messages ─────────────────────────────────────────────────────────────

type detailReadyMsg struct{ content string }

func fetchDetail(hash string) tea.Cmd {
	return func() tea.Msg {
		out, err := git.GetCommitDetail(hash)
		if err != nil {
			return detailReadyMsg{"Error loading detail: " + err.Error()}
		}
		return detailReadyMsg{out}
	}
}

// ── Init ─────────────────────────────────────────────────────────────────────

func (m logModel) Init() tea.Cmd { return nil }

// ── Update ───────────────────────────────────────────────────────────────────

func (m logModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// ---- window resize ----
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.pane == paneDetail {
			if m.vpReady {
				m.vp.Width = msg.Width
				m.vp.Height = m.detailHeight()
			}
		}
		return m, nil

	// ---- detail content loaded ----
	case detailReadyMsg:
		m.vpContent = msg.content
		m.vp = viewport.New(m.width, m.detailHeight())
		m.vp.SetContent(msg.content)
		m.vpReady = true
		return m, nil

	// ---- keyboard ----
	case tea.KeyMsg:
		if m.pane == paneDetail {
			return m.updateDetail(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

func (m logModel) detailHeight() int {
	h := m.height - 3 // header (2 lines) + blank line
	if h < 1 {
		h = 1
	}
	return h
}

func (m logModel) visibleRows() int {
	v := m.height - 3 // title + hint + blank
	if v < 1 {
		return 10
	}
	return v
}

func (m logModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		if m.cursor < len(m.commits)-1 {
			m.cursor++
			if m.cursor >= m.offset+vis {
				m.offset = m.cursor - vis + 1
			}
		}
	case "g": // jump to top
		m.cursor = 0
		m.offset = 0
	case "G": // jump to bottom
		m.cursor = len(m.commits) - 1
		if m.cursor-vis+1 > 0 {
			m.offset = m.cursor - vis + 1
		}
	case "enter":
		if len(m.commits) == 0 {
			return m, nil
		}
		m.pane = paneDetail
		m.vpReady = false
		return m, fetchDetail(m.commits[m.cursor].Hash)
	}
	return m, nil
}

func (m logModel) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.pane = paneList
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

// ── View ─────────────────────────────────────────────────────────────────────

func (m logModel) View() string {
	if m.pane == paneDetail {
		return m.viewDetail()
	}
	return m.viewList()
}

func (m logModel) viewList() string {
	var sb strings.Builder

	// header
	sb.WriteString(style.Title.Render("gito log"))
	sb.WriteString(style.Dimmed.Render(fmt.Sprintf("  %d commits", len(m.commits))))
	sb.WriteString("\n")
	sb.WriteString(style.Dimmed.Render(i18n.T("log.hint_list")))
	sb.WriteString("\n\n")

	vis := m.visibleRows()
	end := m.offset + vis
	if end > len(m.commits) {
		end = len(m.commits)
	}

	maxSubject := m.width - 30 // leave room for hash + date + author
	if maxSubject < 20 {
		maxSubject = 20
	}

	for i := m.offset; i < end; i++ {
		c := m.commits[i]
		subj := c.Subject
		if len([]rune(subj)) > maxSubject {
			subj = string([]rune(subj)[:maxSubject-1]) + "…"
		}

		if i == m.cursor {
			sb.WriteString(cursorGlyp.Render("▶") + " ")
			sb.WriteString(selHash.Render(c.Short) + " ")
			sb.WriteString(selDate.Render(c.Date) + " ")
			sb.WriteString(selMsg.Render(subj) + " ")
			sb.WriteString(selAuthor.Render("(" + c.Author + ")"))
			sb.WriteString(selBar.Render(""))
		} else {
			sb.WriteString("  ")
			sb.WriteString(hashStyle.Render(c.Short) + " ")
			sb.WriteString(dateStyle.Render(c.Date) + " ")
			sb.WriteString(msgStyle.Render(subj) + " ")
			sb.WriteString(authorStyle.Render("(" + c.Author + ")"))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (m logModel) viewDetail() string {
	c := m.commits[m.cursor]

	header := style.Title.Render("gito log  ›  detail") + "\n" +
		hashStyle.Render(c.Short) + "  " +
		dateStyle.Render(c.Date) + "  " +
		msgStyle.Render(c.Subject) + "  " +
		authorStyle.Render("("+c.Author+")") + "\n" +
		style.Dimmed.Render(i18n.T("log.hint_detail")) + "\n\n"

	if !m.vpReady {
		return header + style.Dimmed.Render("  "+i18n.T("common.loading"))
	}
	return header + m.vp.View()
}

// ── RunLog ───────────────────────────────────────────────────────────────────

func RunLog() {
	entries, err := git.GetLogEntries(500)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if len(entries) == 0 {
		fmt.Println(i18n.T("log.none"))
		return
	}

	m := logModel{commits: entries}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
