package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"gito/internal/git"
	"gito/internal/i18n"
	"gito/internal/style"
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

	helpOpen bool
	lay      layout
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
		m.lay = m.lay.resize(msg.Width, msg.Height)
		if m.vpReady {
			m.vp.Width = m.lay.Width
			m.vp.Height = m.detailRows()
		}
		m.offset = m.window().Offset
		return m, nil

	// ---- detail content loaded ----
	case detailReadyMsg:
		m.vpContent = msg.content
		m.vp = viewport.New(m.lay.norm().Width, m.detailRows())
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

// detailRows is the height of the detail viewport: the pane spends one line on
// the header, one on the commit summary, one blank separator and one footer.
func (m logModel) detailRows() int { return bodyRows(m.lay, 4) }

// listRows is how many commits fit under the list header and above the footer.
func (m logModel) listRows() int { return bodyRows(m.lay, 3) }

// window is the scrolling state of the commit list.
func (m logModel) window() listWindow {
	return listWindow{
		Cursor: m.cursor,
		Offset: m.offset,
		Total:  len(m.commits),
		Rows:   m.listRows(),
	}.clamp()
}

func (m logModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.helpOpen {
		switch msg.String() {
		case "?", "q", "esc":
			m.helpOpen = false
		case "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit
	case "?":
		m.helpOpen = true
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.commits)-1 {
			m.cursor++
		}
	case "g": // jump to top
		m.cursor = 0
		m.offset = 0
	case "G": // jump to bottom
		m.cursor = len(m.commits) - 1
	case "enter":
		if len(m.commits) == 0 {
			return m, nil
		}
		m.pane = paneDetail
		m.vpReady = false
		return m, fetchDetail(m.commits[m.cursor].Hash)
	}
	w := m.window()
	m.cursor, m.offset = w.Cursor, w.Offset
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

// ── hints ────────────────────────────────────────────────────────────────────

func logListHints() []keyHint {
	return []keyHint{
		moveHint(),
		{Keys: "g/G", Desc: i18n.T("key.top_bottom")},
		{Keys: "enter", Desc: i18n.T("key.detail")},
		quitHint(),
	}
}

// ── View ─────────────────────────────────────────────────────────────────────

func (m logModel) View() string {
	if m.pane == paneDetail {
		return m.viewDetail()
	}
	return m.viewList()
}

// commitLine renders one commit as hash, date, subject and author. The subject
// is cut with the ANSI-aware helper so a colored cell is measured in columns.
func (m logModel) commitLine(l layout, c git.CommitEntry) string {
	// Room for hash, date, author and the row gutter.
	maxSubject := max(l.Width-30, 20)
	return style.Hash.Render(c.Short) + " " +
		style.Date.Render(c.Date) + " " +
		style.Subject.Render(style.Truncate(c.Subject, maxSubject)) + " " +
		style.AuthorName.Render("("+c.Author+")")
}

func (m logModel) viewList() string {
	l := m.lay.norm()
	hints := logListHints()

	meta := i18n.Tf("meta.commits", len(m.commits))
	if pos := m.window().position(); pos != "" {
		meta += "  " + pos
	}
	head := header(l, "log", "", meta) + "\n"
	foot := footer(l, hints, true)

	if m.helpOpen {
		return frameFull(l, head, helpOverlay(l, hints), foot)
	}

	w := m.window()
	start, end := w.bounds()
	var lines []string
	for i := start; i < end; i++ {
		lines = append(lines, row(l, i == w.Cursor, m.commitLine(l, m.commits[i])))
	}
	return frameFull(l, head, strings.Join(lines, "\n"), foot)
}

func (m logModel) viewDetail() string {
	l := m.lay.norm()

	summary := ""
	// RunLog never starts on an empty log, but the model must not index into an
	// empty slice if it is driven there anyway.
	if m.cursor < len(m.commits) {
		c := m.commits[m.cursor]
		summary = style.Hash.Render(c.Short) + "  " +
			style.Date.Render(c.Date) + "  " +
			style.Subject.Render(c.Subject) + "  " +
			style.AuthorName.Render("("+c.Author+")")
	}
	head := strings.Join([]string{
		header(l, "log", i18n.T("key.detail"), ""),
		style.Truncate(summary, l.Width),
		"",
	}, "\n")
	foot := footer(l, scrollHints(), false)

	if !m.vpReady {
		return frameFull(l, head, style.MetaDim.Render("  "+i18n.T("common.loading")), foot)
	}
	return frameFull(l, head, m.vp.View(), foot)
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

	m := logModel{commits: entries, lay: newLayout()}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
