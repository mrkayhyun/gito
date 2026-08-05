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

// ── data ─────────────────────────────────────────────────────────────────────

type statusSection int

const (
	secStaged statusSection = iota
	secUnstaged
	secUntracked
)

// label is the localized name of the section, used both for the section rule
// inside the list and for the count badges in the header.
func (s statusSection) label() string {
	switch s {
	case secStaged:
		return i18n.T("status.section_staged")
	case secUnstaged:
		return i18n.T("status.section_unstaged")
	default:
		return i18n.T("status.section_untracked")
	}
}

// paint colors text with the semantic style of the working-tree state.
func (s statusSection) paint(text string) string {
	switch s {
	case secStaged:
		return style.Staged.Render(text)
	case secUnstaged:
		return style.Unstaged.Render(text)
	default:
		return style.Untracked.Render(text)
	}
}

type statusEntry struct {
	file    git.FileStatus
	section statusSection
}

// ── panes ─────────────────────────────────────────────────────────────────────

type statusPane int

const (
	statusPaneList statusPane = iota
	statusPaneDiff
)

// ── model ─────────────────────────────────────────────────────────────────────

type statusModel struct {
	entries []statusEntry
	cursor  int
	offset  int // first visible row of the file list
	pane    statusPane

	vp      viewport.Model
	vpReady bool

	confirmDiscard bool
	helpOpen       bool
	errMsg         string
	lay            layout
}

// ── messages ──────────────────────────────────────────────────────────────────

type statusEntriesMsg struct{ entries []statusEntry }
type statusErrMsg struct{ err error }
type statusDiffMsg struct{ content string }

func doStatusLoad() tea.Cmd {
	return func() tea.Msg {
		files, err := git.GetFileStatuses()
		if err != nil {
			return statusErrMsg{err}
		}
		var entries []statusEntry
		for _, f := range files {
			if f.IsStaged() {
				entries = append(entries, statusEntry{f, secStaged})
			}
		}
		for _, f := range files {
			if f.IsUnstaged() && !f.IsUntracked() {
				entries = append(entries, statusEntry{f, secUnstaged})
			}
		}
		for _, f := range files {
			if f.IsUntracked() {
				entries = append(entries, statusEntry{f, secUntracked})
			}
		}
		return statusEntriesMsg{entries}
	}
}

func doStatusDiff(e statusEntry) tea.Cmd {
	return func() tea.Msg {
		if e.section == secUntracked {
			return statusDiffMsg{i18n.T("status.untracked_note")}
		}
		content, err := git.GetFileDiff(e.file.Path, e.section == secStaged)
		if err != nil {
			return statusDiffMsg{"Error: " + err.Error()}
		}
		if content == "" {
			return statusDiffMsg{i18n.T("status.no_diff")}
		}
		return statusDiffMsg{content}
	}
}

// ── Init ─────────────────────────────────────────────────────────────────────

func (m statusModel) Init() tea.Cmd { return doStatusLoad() }

// ── Update ───────────────────────────────────────────────────────────────────

func (m statusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.lay = m.lay.resize(msg.Width, msg.Height)
		if m.vpReady {
			m.vp.Width = m.lay.Width
			m.vp.Height = m.diffRows()
		}
		m.offset = m.window().Offset

	case statusEntriesMsg:
		m.entries = msg.entries
		if m.cursor >= len(m.entries) && len(m.entries) > 0 {
			m.cursor = len(m.entries) - 1
		}
		m.errMsg = ""
		m.offset = m.window().Offset

	case statusErrMsg:
		m.errMsg = msg.err.Error()

	case statusDiffMsg:
		m.vp = viewport.New(m.lay.norm().Width, m.diffRows())
		m.vp.SetContent(msg.content)
		m.vpReady = true

	case tea.KeyMsg:
		if m.pane == statusPaneDiff {
			return m.updateDiff(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

// diffRows is the height of the diff viewport: everything the diff pane does
// not spend on its header, its file line, the blank separator and the footer.
func (m statusModel) diffRows() int { return bodyRows(m.lay, 4) }

// listRows is how many file rows fit under the list header. The section rules
// share the body with the files, so their worst case is reserved up front,
// which keeps the rendered list within the terminal no matter where the cursor
// sits.
func (m statusModel) listRows() int {
	return max(bodyRows(m.lay, len(m.listHead())+1)-m.sectionCount(), 1)
}

// window is the scrolling state of the file list.
func (m statusModel) window() listWindow {
	return listWindow{
		Cursor: m.cursor,
		Offset: m.offset,
		Total:  len(m.entries),
		Rows:   m.listRows(),
	}.clamp()
}

// sectionCount is how many of the three sections the current entries cover.
func (m statusModel) sectionCount() int {
	seen := map[statusSection]bool{}
	for _, e := range m.entries {
		seen[e.section] = true
	}
	return len(seen)
}

func (m statusModel) count(sec statusSection) int {
	n := 0
	for _, e := range m.entries {
		if e.section == sec {
			n++
		}
	}
	return n
}

func (m statusModel) updateDiff(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.pane = statusPaneList
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

func (m statusModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Discard confirmation. Kept first and unchanged: any key that is not 'y'
	// cancels, so the help overlay below can never intercept a confirmation.
	if m.confirmDiscard {
		switch msg.String() {
		case "y", "Y":
			m.confirmDiscard = false
			if m.cursor < len(m.entries) {
				if err := git.DiscardFile(m.entries[m.cursor].file.Path); err != nil {
					m.errMsg = err.Error()
					return m, nil
				}
			}
			return m, doStatusLoad()
		default:
			m.confirmDiscard = false
		}
		return m, nil
	}

	// Key overlay. While it is open it owns '?', 'q' and 'esc'.
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
		if m.cursor < len(m.entries)-1 {
			m.cursor++
		}
	case " ":
		if m.cursor < len(m.entries) {
			e := m.entries[m.cursor]
			var err error
			if e.section == secStaged {
				err = git.UnstageFile(e.file.Path)
			} else {
				err = git.StageFile(e.file.Path)
			}
			if err != nil {
				m.errMsg = err.Error()
				return m, nil
			}
			return m, doStatusLoad()
		}
	case "a":
		if err := git.StageAll(); err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		return m, doStatusLoad()
	case "d":
		if m.cursor < len(m.entries) {
			m.pane = statusPaneDiff
			m.vpReady = false
			return m, doStatusDiff(m.entries[m.cursor])
		}
	case "D":
		if m.cursor < len(m.entries) {
			e := m.entries[m.cursor]
			if e.section == secUnstaged {
				m.confirmDiscard = true
			}
		}
	}
	m.offset = m.window().Offset
	return m, nil
}

// ── hints ────────────────────────────────────────────────────────────────────

func statusListHints() []keyHint {
	return []keyHint{
		{Keys: "space", Desc: i18n.T("key.stage_toggle")},
		{Keys: "a", Desc: i18n.T("key.stage_all")},
		{Keys: "d", Desc: i18n.T("key.diff")},
		{Keys: "D", Desc: i18n.T("key.discard")},
		moveHint(),
		quitHint(),
	}
}

// ── View ─────────────────────────────────────────────────────────────────────

func (m statusModel) View() string {
	if m.pane == statusPaneDiff {
		return m.viewDiff()
	}
	return m.viewList()
}

// listMeta is the header's right-hand cell: one colored badge per non-empty
// section plus the cursor position.
func (m statusModel) listMeta() string {
	var parts []string
	for _, sec := range []statusSection{secStaged, secUnstaged, secUntracked} {
		if n := m.count(sec); n > 0 {
			parts = append(parts, sec.paint(fmt.Sprintf("%s %d", sec.label(), n)))
		}
	}
	if pos := m.position(); pos != "" {
		parts = append(parts, pos)
	}
	return strings.Join(parts, "  ")
}

// position reports "cursor/total" independently of how many rows are visible,
// so it can be used while the row count is still being computed.
func (m statusModel) position() string {
	return listWindow{Cursor: m.cursor, Total: len(m.entries), Rows: 1}.position()
}

// listHead is every line above the file list: the header, a blank separator
// and whatever banners are currently live. Its length is what bodyRows needs,
// which is why the head is built as lines rather than one string.
func (m statusModel) listHead() []string {
	l := m.lay.norm()
	lines := []string{header(l, "status", "", m.listMeta()), ""}
	if m.confirmDiscard && m.cursor < len(m.entries) {
		prompt := i18n.Tf("status.discard_confirm", m.entries[m.cursor].file.Path)
		lines = append(lines, splitLines(confirmBar(l, prompt))...)
		lines = append(lines, "")
	}
	if b := banner(l, bannerError, m.errMsg); b != "" {
		lines = append(lines, b, "")
	}
	return lines
}

func (m statusModel) viewList() string {
	l := m.lay.norm()
	hints := statusListHints()
	head := strings.Join(m.listHead(), "\n")
	foot := footer(l, hints, true)

	if m.helpOpen {
		return frameOverlay(l, head, hints, foot)
	}

	if len(m.entries) == 0 {
		body := style.MetaDim.Render(i18n.T("status.clean"))
		return frameFull(l, head, style.Truncate(body, l.Width), foot)
	}

	w := m.window()
	rl := listLayout(l, w)
	start, end := w.bounds()
	var lines []string
	prevSec := statusSection(-1)
	for i := start; i < end; i++ {
		e := m.entries[i]
		if e.section != prevSec {
			prevSec = e.section
			lines = append(lines, m.sectionRule(rl, e.section))
		}
		xy := string([]byte{e.file.Staged, e.file.Unstaged})
		if e.section == secUntracked {
			xy = "??"
		}
		content := e.section.paint(xy) + " " + e.section.paint(e.file.Path)
		lines = append(lines, row(rl, i == w.Cursor, content))
	}
	return frameFull(l, head, listBody(l, w, lines), foot)
}

// sectionRule draws the "── Staged ──" separator from the glyph table, so a
// non-UTF-8 terminal gets "-- Staged --" instead of replacement boxes.
func (m statusModel) sectionRule(l layout, sec statusSection) string {
	rule := strings.Repeat(style.G.Rule, 2)
	return style.Truncate(style.SectionHead.Render(rule+" "+sec.label()+" "+rule), l.Width)
}

func (m statusModel) viewDiff() string {
	l := m.lay.norm()

	info := ""
	if m.cursor < len(m.entries) {
		e := m.entries[m.cursor]
		info = e.section.paint(e.file.Path)
	}
	meta := i18n.Tf("meta.files", len(m.entries))
	if pos := m.position(); pos != "" {
		meta += "  " + pos
	}
	head := strings.Join([]string{
		header(l, "status", i18n.T("key.diff"), meta),
		style.Truncate(info, l.Width),
		"",
	}, "\n")
	foot := footer(l, scrollHints(), false)

	if !m.vpReady {
		return frameFull(l, head, style.MetaDim.Render("  "+i18n.T("common.loading")), foot)
	}
	return frameFull(l, head, m.vp.View(), foot)
}

// ── RunStatus ────────────────────────────────────────────────────────────────

func RunStatus() {
	p := tea.NewProgram(statusModel{lay: newLayout()}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
