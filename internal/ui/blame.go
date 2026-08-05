package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"gito/internal/git"
	"gito/internal/i18n"
	"gito/internal/style"
)

// ── panes ─────────────────────────────────────────────────────────────────────

type blamePane int

const (
	blamePanePick blamePane = iota
	blamePaneView
)

// ── model ─────────────────────────────────────────────────────────────────────

type blameModel struct {
	files    []string
	filter   textinput.Model
	cursor   int
	offset   int // first visible row of the file list
	pane     blamePane
	selected string

	vp      viewport.Model
	vpReady bool

	lay layout
}

// ── messages ──────────────────────────────────────────────────────────────────

type blameContentMsg struct{ content string }

func doBlame(path string) tea.Cmd {
	return func() tea.Msg {
		out, err := git.GetBlame(path)
		if err != nil {
			return blameContentMsg{"Error: " + err.Error()}
		}
		if out == "" {
			return blameContentMsg{i18n.T("blame.empty")}
		}
		return blameContentMsg{out}
	}
}

// ── Init ─────────────────────────────────────────────────────────────────────

func (m blameModel) Init() tea.Cmd { return textinput.Blink }

func (m blameModel) filtered() []string {
	q := strings.ToLower(m.filter.Value())
	if q == "" {
		return m.files
	}
	var out []string
	for _, f := range m.files {
		if strings.Contains(strings.ToLower(f), q) {
			out = append(out, f)
		}
	}
	return out
}

// ── Update ───────────────────────────────────────────────────────────────────

func (m blameModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.lay = m.lay.resize(msg.Width, msg.Height)
		if m.vpReady {
			m.vp.Width = m.lay.Width
			m.vp.Height = m.viewRows()
		}
		m.fitFilterWidth()
		m.offset = m.window().Offset
	case blameContentMsg:
		m.vp = viewport.New(m.lay.norm().Width, m.viewRows())
		m.vp.SetContent(msg.content)
		m.vpReady = true
	case tea.KeyMsg:
		if m.pane == blamePaneView {
			return m.updateView(msg)
		}
		return m.updatePick(msg)
	}
	// let the filter input process non-key messages (blink)
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	return m, cmd
}

// viewRows is the height of the blame viewport. It replaces the two constants
// this screen used to disagree on (m.height-3 for the viewport, m.height-5 for
// the file list): both panes now measure their own chrome and ask bodyRows.
func (m blameModel) viewRows() int { return bodyRows(m.lay, 3) }

// listRows is how many files fit under the picker header and filter field.
func (m blameModel) listRows() int { return bodyRows(m.lay, len(m.pickHead())+1) }

// window is the scrolling state of the (filtered) file list.
func (m blameModel) window() listWindow {
	return listWindow{
		Cursor: m.cursor,
		Offset: m.offset,
		Total:  len(m.filtered()),
		Rows:   m.listRows(),
	}.clamp()
}

// fitFilterWidth keeps the filter field inside the terminal.
func (m *blameModel) fitFilterWidth() {
	label := style.DisplayWidth(i18n.T("common.search"))
	m.filter.Width = max(m.lay.norm().Width-label-6, 10)
}

func (m blameModel) updateView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.pane = blamePanePick
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

func (m blameModel) updatePick(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	filtered := m.filtered()
	switch msg.String() {
	case "ctrl+c", "esc":
		return m, tea.Quit
	case "up", "ctrl+p":
		if m.cursor > 0 {
			m.cursor--
		}
		m.offset = m.window().Offset
		return m, nil
	case "down", "ctrl+n":
		if m.cursor < len(filtered)-1 {
			m.cursor++
		}
		m.offset = m.window().Offset
		return m, nil
	case "enter":
		if m.cursor < len(filtered) {
			m.selected = filtered[m.cursor]
			m.pane = blamePaneView
			m.vpReady = false
			return m, doBlame(m.selected)
		}
		return m, nil
	}
	prev := m.filter.Value()
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	if m.filter.Value() != prev {
		m.cursor = 0
		m.offset = 0
	}
	return m, cmd
}

// ── hints ────────────────────────────────────────────────────────────────────

// blamePickHints is rendered in the footer rather than behind a '?' overlay:
// the filter textinput consumes printable runes, so '?' is not a free key here.
func blamePickHints() []keyHint {
	return []keyHint{
		{Keys: "a-z", Desc: i18n.T("key.filter")},
		arrowMoveHint(),
		{Keys: "enter", Desc: i18n.T("key.view_blame")},
		escQuitHint(),
	}
}

// ── View ─────────────────────────────────────────────────────────────────────

func (m blameModel) View() string {
	if m.pane == blamePaneView {
		return m.viewBlame()
	}
	return m.viewPick()
}

// position reports "cursor/total" without depending on the visible row count.
func (m blameModel) position() string {
	return listWindow{Cursor: m.cursor, Total: len(m.filtered()), Rows: 1}.position()
}

// pickHead is every line above the file list: header and the filter field.
func (m blameModel) pickHead() []string {
	l := m.lay.norm()

	meta := i18n.Tf("meta.files", len(m.filtered()))
	if pos := m.position(); pos != "" {
		meta += "  " + pos
	}
	search := style.Label.Render(i18n.T("common.search")) + m.filter.View()
	return []string{
		header(l, "blame", "", meta),
		"",
		style.Truncate(search, l.Width),
		"",
	}
}

func (m blameModel) viewPick() string {
	l := m.lay.norm()
	head := strings.Join(m.pickHead(), "\n")
	foot := footer(l, blamePickHints(), false)

	filtered := m.filtered()
	if len(filtered) == 0 {
		body := style.MetaDim.Render(i18n.T("blame.none"))
		return frameFull(l, head, style.Truncate(body, l.Width), foot)
	}

	w := m.window()
	rl := listLayout(l, w)
	start, end := w.bounds()
	var lines []string
	for i := start; i < end; i++ {
		lines = append(lines, row(rl, i == w.Cursor, style.Subject.Render(filtered[i])))
	}
	return frameFull(l, head, listBody(l, w, lines), foot)
}

func (m blameModel) viewBlame() string {
	l := m.lay.norm()

	head := header(l, "blame", m.selected, "") + "\n"
	foot := footer(l, scrollHints(), false)

	if !m.vpReady {
		return frameFull(l, head, style.MetaDim.Render("  "+i18n.T("common.loading")), foot)
	}
	return frameFull(l, head, m.vp.View(), foot)
}

// ── RunBlame ─────────────────────────────────────────────────────────────────

func RunBlame() {
	files, err := git.ListTrackedFiles()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if len(files) == 0 {
		fmt.Println(i18n.T("blame.no_tracked"))
		return
	}
	filter := textinput.New()
	filter.Placeholder = i18n.T("blame.ph_filter")
	filter.CharLimit = 200
	filter.Focus()

	m := blameModel{files: files, filter: filter, lay: newLayout()}
	m.fitFilterWidth()

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
