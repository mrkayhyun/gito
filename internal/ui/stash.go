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
	offset  int // first visible row of the stash list
	pane    stashPane

	vp      viewport.Model
	vpReady bool

	confirmDrop bool
	helpOpen    bool
	errMsg      string
	successMsg  string
	lay         layout
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
		m.lay = m.lay.resize(msg.Width, msg.Height)
		if m.vpReady {
			m.vp.Width = m.lay.Width
			m.vp.Height = m.showRows()
		}
		m.offset = m.window().Offset

	case stashListMsg:
		m.stashes = msg.stashes
		if m.cursor >= len(m.stashes) && len(m.stashes) > 0 {
			m.cursor = len(m.stashes) - 1
		}
		m.offset = m.window().Offset

	case stashErrMsg:
		m.errMsg = msg.err.Error()

	case stashShowMsg:
		m.vp = viewport.New(m.lay.norm().Width, m.showRows())
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

// showRows is the height of the diff viewport: header, stash summary, blank
// separator and footer are subtracted from the terminal height.
func (m stashModel) showRows() int { return bodyRows(m.lay, 4) }

// listRows is how many stashes fit under the list header, banners included.
func (m stashModel) listRows() int { return bodyRows(m.lay, len(m.listHead())+1) }

// window is the scrolling state of the stash list.
func (m stashModel) window() listWindow {
	return listWindow{
		Cursor: m.cursor,
		Offset: m.offset,
		Total:  len(m.stashes),
		Rows:   m.listRows(),
	}.clamp()
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
	// Drop confirmation. Kept first and unchanged: any key that is not 'y'
	// cancels, so nothing below can intercept a confirmation.
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
	m.successMsg = ""

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
	m.offset = m.window().Offset
	return m, nil
}

// ── hints ────────────────────────────────────────────────────────────────────

func stashListHints() []keyHint {
	return []keyHint{
		{Keys: "enter/p", Desc: i18n.T("key.pop")},
		{Keys: "a", Desc: i18n.T("key.apply")},
		{Keys: "d", Desc: i18n.T("key.diff")},
		{Keys: "D", Desc: i18n.T("key.drop")},
		moveHint(),
		quitHint(),
	}
}

// ── View ─────────────────────────────────────────────────────────────────────

func (m stashModel) View() string {
	if m.pane == stashPaneShow {
		return m.viewShow()
	}
	return m.viewList()
}

// position reports "cursor/total" without depending on the visible row count.
func (m stashModel) position() string {
	return listWindow{Cursor: m.cursor, Total: len(m.stashes), Rows: 1}.position()
}

// listHead is every line above the stash list: header, blank separator and the
// live banners. Its length is what bodyRows subtracts.
func (m stashModel) listHead() []string {
	l := m.lay.norm()

	meta := i18n.Tf("meta.stashes", len(m.stashes))
	if pos := m.position(); pos != "" {
		meta += "  " + pos
	}
	lines := []string{header(l, "stash", "", meta), ""}

	if m.confirmDrop && m.cursor < len(m.stashes) {
		prompt := i18n.Tf("stash.drop_confirm", m.stashes[m.cursor].Ref)
		lines = append(lines, splitLines(confirmBar(l, prompt))...)
		lines = append(lines, "")
	}
	if b := banner(l, bannerError, m.errMsg); b != "" {
		lines = append(lines, b, "")
	}
	if b := banner(l, bannerSuccess, m.successMsg); b != "" {
		lines = append(lines, b, "")
	}
	return lines
}

func (m stashModel) viewList() string {
	l := m.lay.norm()
	hints := stashListHints()
	head := strings.Join(m.listHead(), "\n")
	foot := footer(l, hints, true)

	if m.helpOpen {
		return frameOverlay(l, head, hints, foot)
	}

	if len(m.stashes) == 0 {
		body := style.MetaDim.Render(i18n.T("stash.none"))
		return frameFull(l, head, style.Truncate(body, l.Width), foot)
	}

	w := m.window()
	rl := listLayout(l, w)
	start, end := w.bounds()
	var lines []string
	for i := start; i < end; i++ {
		lines = append(lines, row(rl, i == w.Cursor, stashLine(m.stashes[i])))
	}
	return frameFull(l, head, listBody(l, w, lines), foot)
}

// stashLine renders one stash as ref, originating branch and subject.
func stashLine(s git.StashEntry) string {
	line := style.Ref.Render(s.Ref) + " "
	if s.Branch != "" {
		line += style.Date.Render("("+s.Branch+")") + " "
	}
	return line + style.Subject.Render(s.Subject)
}

func (m stashModel) viewShow() string {
	l := m.lay.norm()

	summary := ""
	if m.cursor < len(m.stashes) {
		s := m.stashes[m.cursor]
		summary = style.Ref.Render(s.Ref) + "  " + style.Subject.Render(s.Subject)
	}
	head := strings.Join([]string{
		header(l, "stash", i18n.T("key.diff"), m.position()),
		style.Truncate(summary, l.Width),
		"",
	}, "\n")
	foot := footer(l, scrollHints(), false)

	if !m.vpReady {
		return frameFull(l, head, style.MetaDim.Render("  "+i18n.T("common.loading")), foot)
	}
	return frameFull(l, head, m.vp.View(), foot)
}

// ── RunStash ─────────────────────────────────────────────────────────────────

func RunStash() {
	p := tea.NewProgram(stashModel{lay: newLayout()}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
