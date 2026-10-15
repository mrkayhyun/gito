package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mrkayhyun/gito/internal/git"
	"github.com/mrkayhyun/gito/internal/i18n"
	"github.com/mrkayhyun/gito/internal/style"
)

// ── lipgloss styles for the commit list ──────────────────────────────────────

var (
	hashStyle   = lipgloss.NewStyle().Foreground(style.Yellow).Bold(true)
	dateStyle   = lipgloss.NewStyle().Foreground(style.Green)
	msgStyle    = lipgloss.NewStyle().Foreground(style.Text)
	authorStyle = lipgloss.NewStyle().Foreground(style.MutedAlt)

	selBar     = lipgloss.NewStyle().Background(style.SelBg)
	selHash    = hashStyle.Background(style.SelBg)
	selDate    = dateStyle.Background(style.SelBg)
	selMsg     = msgStyle.Bold(true).Background(style.SelBg)
	selAuthor  = authorStyle.Background(style.SelBg)
	cursorGlyp = lipgloss.NewStyle().Foreground(style.Accent).Bold(true)
)

// ── model ────────────────────────────────────────────────────────────────────

type logPane int

const (
	paneList   logPane = iota
	paneDetail         // showing git show output
)

const (
	// logLimit is how many commits the list loads up front.
	logLimit = 500
	// deepSearchLimit caps matches returned by a full-history search.
	deepSearchLimit = 1000
)

type logModel struct {
	commits []git.CommitEntry
	paths   []string // optional pathspecs from `gito log <path>...`
	// truncated is true when history is longer than what was loaded, so a
	// committed search (enter) has to walk the full history via git.
	truncated bool
	cursor    int
	offset    int // first visible row index
	pane      logPane

	// search narrows the list; searching is true while the input has focus.
	search    textinput.Model
	searching bool

	// Full-history search state. Results apply only while the input still
	// holds deepQuery; editing the query falls back to local filtering.
	deepQuery   string
	deepDone    bool
	deepResults []git.CommitEntry
	deepErr     error

	detail git.CommitEntry // commit shown in the detail pane

	vp        viewport.Model
	vpReady   bool
	vpContent string // raw content waiting to be set when vp is init'd

	width  int
	height int
}

func newLogModel(commits []git.CommitEntry) logModel {
	search := textinput.New()
	search.Placeholder = i18n.T("log.ph_search")
	search.CharLimit = 200
	return logModel{commits: commits, search: search}
}

// matchCommit reports whether c matches every whitespace-separated term of
// query (case-insensitive). A term matches a substring of the subject or
// author, or a prefix of the full hash.
func matchCommit(c git.CommitEntry, query string) bool {
	for _, term := range strings.Fields(strings.ToLower(query)) {
		if !strings.Contains(strings.ToLower(c.Subject), term) &&
			!strings.Contains(strings.ToLower(c.Author), term) &&
			!strings.HasPrefix(strings.ToLower(c.Hash), term) {
			return false
		}
	}
	return true
}

// filtered returns the commits matching the current search query.
func (m logModel) filtered() []git.CommitEntry {
	q := strings.TrimSpace(m.search.Value())
	if q == "" {
		return m.commits
	}
	if m.deepDone && q == m.deepQuery && m.deepErr == nil {
		return m.deepResults
	}
	var out []git.CommitEntry
	for _, c := range m.commits {
		if matchCommit(c, q) {
			out = append(out, c)
		}
	}
	return out
}

// deepPending reports whether a full-history search for the current query is
// still running.
func (m logModel) deepPending() bool {
	return !m.deepDone && m.deepQuery != "" && m.deepQuery == strings.TrimSpace(m.search.Value())
}

// ── tea messages ─────────────────────────────────────────────────────────────

type detailReadyMsg struct{ content string }

type deepSearchMsg struct {
	query   string
	results []git.CommitEntry
	err     error
}

func searchHistory(query string, paths []string) tea.Cmd {
	return func() tea.Msg {
		res, err := git.SearchLog(func(c git.CommitEntry) bool {
			return matchCommit(c, query)
		}, deepSearchLimit, paths...)
		return deepSearchMsg{query: query, results: res, err: err}
	}
}

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

	// ---- full-history search finished ----
	case deepSearchMsg:
		if msg.query != m.deepQuery {
			return m, nil // stale: a newer search superseded it
		}
		m.deepDone = true
		m.deepResults = msg.results
		m.deepErr = msg.err
		m.cursor, m.offset = 0, 0
		return m, nil

	// ---- keyboard ----
	case tea.KeyMsg:
		if m.pane == paneDetail {
			return m.updateDetail(msg)
		}
		if m.searching {
			return m.updateSearch(msg)
		}
		return m.updateList(msg)
	}
	if m.searching {
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		return m, cmd
	}
	return m, nil
}

// updateSearch handles keys while the search input is focused. The list is
// filtered live; enter keeps the filter and returns to navigation, esc clears it.
func (m logModel) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.search.SetValue("")
		m.search.Blur()
		m.searching = false
		m.cursor, m.offset = 0, 0
		return m, nil
	case "enter":
		m.search.Blur()
		m.searching = false
		q := strings.TrimSpace(m.search.Value())
		if m.truncated && q != "" && q != m.deepQuery {
			m.deepQuery = q
			m.deepDone = false
			m.deepResults, m.deepErr = nil, nil
			return m, searchHistory(q, m.paths)
		}
		return m, nil
	case "up", "down":
		return m.updateList(msg)
	}
	prev := m.search.Value()
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)
	if m.search.Value() != prev {
		m.cursor, m.offset = 0, 0
	}
	return m, cmd
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
	if m.searching || m.search.Value() != "" {
		v -= 2 // search line + blank
	}
	if v < 1 {
		return 10
	}
	return v
}

func (m logModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	vis := m.visibleRows()
	commits := m.filtered()
	switch msg.String() {
	case "esc":
		// esc first clears an active filter, then quits.
		if m.search.Value() != "" {
			m.search.SetValue("")
			m.cursor, m.offset = 0, 0
			return m, nil
		}
		return m, tea.Quit
	case "ctrl+c", "q":
		return m, tea.Quit
	case "/":
		m.searching = true
		return m, m.search.Focus()
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.offset {
				m.offset = m.cursor
			}
		}
	case "down", "j":
		if m.cursor < len(commits)-1 {
			m.cursor++
			if m.cursor >= m.offset+vis {
				m.offset = m.cursor - vis + 1
			}
		}
	case "g": // jump to top
		m.cursor = 0
		m.offset = 0
	case "G": // jump to bottom
		m.cursor = len(commits) - 1
		if m.cursor < 0 {
			m.cursor = 0
		}
		if m.cursor-vis+1 > 0 {
			m.offset = m.cursor - vis + 1
		}
	case "enter":
		if m.cursor >= len(commits) {
			return m, nil
		}
		m.pane = paneDetail
		m.vpReady = false
		m.detail = commits[m.cursor]
		return m, fetchDetail(m.detail.Hash)
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
	commits := m.filtered()
	title := "gito log"
	if len(m.paths) > 0 {
		title += "  ›  " + strings.Join(m.paths, " ")
	}
	sb.WriteString(style.Title.Render(title))
	total := fmt.Sprintf("%d", len(m.commits))
	if m.truncated {
		total += "+"
	}
	switch {
	case m.deepPending():
		sb.WriteString(style.Dimmed.Render("  " + i18n.T("log.searching_history")))
	case m.deepDone && m.deepQuery == strings.TrimSpace(m.search.Value()) && m.deepErr != nil:
		sb.WriteString(style.Failure.Render("  " + m.deepErr.Error()))
	case m.deepDone && m.deepQuery == strings.TrimSpace(m.search.Value()):
		sb.WriteString(style.Dimmed.Render(fmt.Sprintf("  %d %s", len(commits), i18n.T("log.full_history"))))
	case len(commits) == len(m.commits):
		sb.WriteString(style.Dimmed.Render(fmt.Sprintf("  %s commits", total)))
	default:
		sb.WriteString(style.Dimmed.Render(fmt.Sprintf("  %d / %s commits", len(commits), total)))
	}
	sb.WriteString("\n")
	if m.searching {
		sb.WriteString(style.Dimmed.Render(i18n.T("log.hint_search")))
	} else {
		sb.WriteString(style.Dimmed.Render(i18n.T("log.hint_list")))
	}
	sb.WriteString("\n\n")
	if m.searching || m.search.Value() != "" {
		sb.WriteString(style.Label.Render(i18n.T("common.search")) + m.search.View() + "\n\n")
	}

	if len(commits) == 0 {
		if m.deepPending() {
			return sb.String()
		}
		key := "log.no_match"
		if m.truncated && strings.TrimSpace(m.search.Value()) != m.deepQuery {
			key = "log.no_match_more" // local miss; history beyond the first page is unsearched
		}
		sb.WriteString(style.Dimmed.Render(i18n.T(key)) + "\n")
		return sb.String()
	}

	vis := m.visibleRows()
	end := m.offset + vis
	if end > len(commits) {
		end = len(commits)
	}

	maxSubject := m.width - 30 // leave room for hash + date + author
	if maxSubject < 20 {
		maxSubject = 20
	}

	for i := m.offset; i < end; i++ {
		c := commits[i]
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
	c := m.detail

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

// RunLog opens the log viewer. paths optionally limit history to commits
// touching those pathspecs (`gito log <path>...`).
func RunLog(paths []string) {
	entries, err := git.GetLogEntries(logLimit, paths...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if len(entries) == 0 {
		if len(paths) > 0 && git.HasCommits() {
			fmt.Println(i18n.T("log.none_path"))
		} else {
			fmt.Println(i18n.T("log.none"))
		}
		return
	}

	m := newLogModel(entries)
	m.paths = paths
	m.truncated = len(entries) == logLimit
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
