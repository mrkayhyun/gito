package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"gito/internal/git"
	"gito/internal/i18n"
	"gito/internal/style"
)

// ── modes ─────────────────────────────────────────────────────────────────────

type reflogMode int

const (
	reflogModeList reflogMode = iota
	reflogModeBranch
)

// ── model ─────────────────────────────────────────────────────────────────────

type reflogModel struct {
	entries []git.ReflogEntry
	cursor  int
	offset  int // first visible row of the reflog list
	mode    reflogMode

	input textinput.Model // new branch name for recovery

	helpOpen   bool
	errMsg     string
	successMsg string
	lay        layout
}

// ── Init ─────────────────────────────────────────────────────────────────────

func (m reflogModel) Init() tea.Cmd { return nil }

// ── Update ───────────────────────────────────────────────────────────────────

func (m reflogModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.lay = m.lay.resize(msg.Width, msg.Height)
		m.offset = m.window().Offset
		m.fitFormWidth()
	case tea.KeyMsg:
		if m.mode == reflogModeBranch {
			return m.updateBranch(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

// listRows is how many reflog entries fit under the header, banners included.
func (m reflogModel) listRows() int { return bodyRows(m.lay, len(m.listHead())+1) }

// window is the scrolling state of the reflog list.
func (m reflogModel) window() listWindow {
	return listWindow{
		Cursor: m.cursor,
		Offset: m.offset,
		Total:  len(m.entries),
		Rows:   m.listRows(),
	}.clamp()
}

// fitFormWidth keeps the recover form's input inside the terminal.
func (m *reflogModel) fitFormWidth() {
	m.input.Width = max(m.lay.norm().Width-6, 10)
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

	m.errMsg = ""
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
	case "g":
		m.cursor, m.offset = 0, 0
	case "G":
		m.cursor = len(m.entries) - 1
		if m.cursor < 0 {
			m.cursor = 0
		}
	case "b": // recover: create branch at this reflog entry (non-destructive)
		if m.cursor < len(m.entries) {
			m.mode = reflogModeBranch
			m.successMsg = ""
			m.input.SetValue("")
			m.fitFormWidth()
			return m, m.input.Focus()
		}
	}
	m.offset = m.window().Offset
	return m, nil
}

// ── hints ────────────────────────────────────────────────────────────────────

func reflogListHints() []keyHint {
	return []keyHint{
		moveHint(),
		{Keys: "g/G", Desc: i18n.T("key.top_bottom")},
		{Keys: "b", Desc: i18n.T("key.branch_here")},
		quitHint(),
	}
}

func reflogRecoverHints() []keyHint {
	return []keyHint{
		{Keys: "enter", Desc: i18n.T("key.confirm")},
		{Keys: "esc", Desc: i18n.T("key.cancel")},
	}
}

// ── View ─────────────────────────────────────────────────────────────────────

func (m reflogModel) View() string {
	if m.mode == reflogModeBranch {
		return m.viewBranch()
	}
	return m.viewList()
}

// position reports "cursor/total" without depending on the visible row count.
func (m reflogModel) position() string {
	return listWindow{Cursor: m.cursor, Total: len(m.entries), Rows: 1}.position()
}

func (m reflogModel) viewBranch() string {
	l := m.lay.norm()

	head := header(l, "reflog", i18n.T("reflog.crumb_recover"), m.position()) + "\n"
	foot := footer(l, reflogRecoverHints(), false)

	var lines []string
	if m.cursor < len(m.entries) {
		e := m.entries[m.cursor]
		target := style.Label.Render(i18n.T("reflog.target")) +
			style.Hash.Render(e.Short) + " " +
			style.RefBase.Render(e.Selector) + " " +
			style.Subject.Render(e.Subject)
		lines = append(lines, style.Truncate(target, l.Width), "")
	}
	lines = append(lines,
		style.Truncate(style.Label.Render(i18n.T("reflog.new_branch_name")), l.Width),
		"",
		style.Truncate(m.input.View(), l.Width),
	)
	if b := banner(l, bannerError, m.errMsg); b != "" {
		lines = append(lines, "", b)
	}
	return frameFull(l, head, strings.Join(lines, "\n"), foot)
}

// listHead is every line above the reflog list: header and the live banners.
func (m reflogModel) listHead() []string {
	l := m.lay.norm()

	meta := i18n.Tf("meta.entries", len(m.entries))
	if pos := m.position(); pos != "" {
		meta += "  " + pos
	}
	lines := []string{header(l, "reflog", "", meta), ""}

	if b := banner(l, bannerError, m.errMsg); b != "" {
		lines = append(lines, b, "")
	}
	if b := banner(l, bannerSuccess, m.successMsg); b != "" {
		lines = append(lines, b, "")
	}
	return lines
}

func (m reflogModel) viewList() string {
	l := m.lay.norm()
	hints := reflogListHints()
	head := strings.Join(m.listHead(), "\n")
	foot := footer(l, hints, true)

	if m.helpOpen {
		return frameFull(l, head, helpOverlay(l, hints), foot)
	}

	if len(m.entries) == 0 {
		body := style.MetaDim.Render(i18n.T("reflog.no_entries"))
		return frameFull(l, head, style.Truncate(body, l.Width), foot)
	}

	w := m.window()
	start, end := w.bounds()
	var lines []string
	for i := start; i < end; i++ {
		lines = append(lines, row(l, i == w.Cursor, reflogLine(l, m.entries[i])))
	}
	return frameFull(l, head, strings.Join(lines, "\n"), foot)
}

// reflogLine renders one entry as hash, selector, action and subject. The two
// padded columns shrink with the terminal instead of being frozen at 12 and 24
// columns, and both padding and trimming measure display width so a wide-rune
// action still lines up.
func reflogLine(l layout, e git.ReflogEntry) string {
	selW := min(12, max(l.norm().Width/6, 6))
	actW := min(24, max(l.norm().Width/4, 8))
	return style.Hash.Render(e.Short) + " " +
		style.RefBase.Render(style.Pad(style.Truncate(e.Selector, selW), selW)) + " " +
		style.Date.Render(style.Pad(style.Truncate(e.Action, actW), actW)) + " " +
		style.Subject.Render(e.Subject)
}

// ── RunReflog ────────────────────────────────────────────────────────────────

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

	m := reflogModel{entries: entries, input: input, lay: newLayout()}
	m.fitFormWidth()

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
