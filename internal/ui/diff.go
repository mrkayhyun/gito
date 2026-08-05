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

type diffPane int

const (
	diffPanePick diffPane = iota
	diffPaneView
)

// ── model ─────────────────────────────────────────────────────────────────────

// The picker selects a base ref, then a target ref, then shows base..target.
type diffModel struct {
	refs   []string
	cursor int
	offset int // first visible row of the ref list
	pane   diffPane

	base   string // selected base (empty until chosen)
	target string

	vp      viewport.Model
	vpReady bool

	helpOpen bool
	errMsg   string
	lay      layout
}

// ── messages ──────────────────────────────────────────────────────────────────

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

// ── Init ─────────────────────────────────────────────────────────────────────

func (m diffModel) Init() tea.Cmd { return nil }

// ── Update ───────────────────────────────────────────────────────────────────

func (m diffModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.lay = m.lay.resize(msg.Width, msg.Height)
		if m.vpReady {
			m.vp.Width = m.lay.Width
			m.vp.Height = m.viewRows()
		}
		m.offset = m.window().Offset

	case diffContentMsg:
		m.vp = viewport.New(m.lay.norm().Width, m.viewRows())
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

// viewRows is the height of the diff viewport: header, the base/target summary,
// a blank separator and the footer come off the terminal height.
func (m diffModel) viewRows() int { return bodyRows(m.lay, 4) }

// listRows is how many refs fit under the picker header, banners included.
func (m diffModel) listRows() int { return bodyRows(m.lay, len(m.pickHead())+1) }

// window is the scrolling state of the ref list.
func (m diffModel) window() listWindow {
	return listWindow{
		Cursor: m.cursor,
		Offset: m.offset,
		Total:  len(m.refs),
		Rows:   m.listRows(),
	}.clamp()
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
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		if m.base != "" {
			m.base = "" // step back to base selection
			return m, nil
		}
		return m, tea.Quit
	case "?":
		m.helpOpen = true
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
	m.offset = m.window().Offset
	return m, nil
}

// ── hints ────────────────────────────────────────────────────────────────────

func diffPickHints() []keyHint {
	return []keyHint{
		moveHint(),
		{Keys: "enter", Desc: i18n.T("key.select")},
		{Keys: "esc", Desc: i18n.T("key.back")},
		quitHint(),
	}
}

// ── View ─────────────────────────────────────────────────────────────────────

func (m diffModel) View() string {
	if m.pane == diffPaneView {
		return m.viewDiff()
	}
	return m.viewPick()
}

// position reports "cursor/total" without depending on the visible row count.
func (m diffModel) position() string {
	return listWindow{Cursor: m.cursor, Total: len(m.refs), Rows: 1}.position()
}

// pickHead is every line above the ref list: the header with the 1/2 - 2/2 step
// indicator in its meta cell, the chosen base and the error banner.
func (m diffModel) pickHead() []string {
	l := m.lay.norm()

	meta := i18n.T("diff.step_base")
	if m.base != "" {
		meta = i18n.T("diff.step_target")
	}
	if pos := m.position(); pos != "" {
		meta += "  " + pos
	}
	lines := []string{header(l, "diff", "", meta), ""}

	if m.base != "" {
		base := style.Label.Render(i18n.T("diff.label_base")) + " " + style.RefBase.Render(m.base)
		lines = append(lines, style.Truncate(base, l.Width), "")
	}
	if b := banner(l, bannerError, m.errMsg); b != "" {
		lines = append(lines, b, "")
	}
	return lines
}

func (m diffModel) viewPick() string {
	l := m.lay.norm()
	hints := diffPickHints()
	head := strings.Join(m.pickHead(), "\n")
	foot := footer(l, hints, true)

	if m.helpOpen {
		return frameOverlay(l, head, hints, foot)
	}

	if len(m.refs) == 0 {
		body := style.MetaDim.Render(i18n.T("diff.no_refs"))
		return frameFull(l, head, style.Truncate(body, l.Width), foot)
	}

	w := m.window()
	rl := listLayout(l, w)
	start, end := w.bounds()
	var lines []string
	for i := start; i < end; i++ {
		lines = append(lines, row(rl, i == w.Cursor, style.Ref.Render(m.refs[i])))
	}
	return frameFull(l, head, listBody(l, w, lines), foot)
}

func (m diffModel) viewDiff() string {
	l := m.lay.norm()

	summary := style.Label.Render(i18n.T("diff.label_base")) + " " + style.RefBase.Render(m.base) +
		"  " + style.MetaDim.Render(style.G.Arrow) + "  " +
		style.Label.Render(i18n.T("diff.label_target")) + " " + style.RefTarget.Render(m.target)
	head := strings.Join([]string{
		header(l, "diff", "", ""),
		style.Truncate(summary, l.Width),
		"",
	}, "\n")
	foot := footer(l, scrollHints(), false)

	if !m.vpReady {
		return frameFull(l, head, style.MetaDim.Render("  "+i18n.T("common.loading")), foot)
	}
	return frameFull(l, head, m.vp.View(), foot)
}

// ── RunDiff ──────────────────────────────────────────────────────────────────

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
	p := tea.NewProgram(diffModel{refs: refs, lay: newLayout()}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
