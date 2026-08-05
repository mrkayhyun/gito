package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"gito/internal/git"
	"gito/internal/i18n"
	"gito/internal/style"
)

// ── panes ─────────────────────────────────────────────────────────────────────

type remotePane int

const (
	remotePaneList remotePane = iota
	remotePaneOutput
)

// ── model ─────────────────────────────────────────────────────────────────────

type remoteModel struct {
	remotes     []git.RemoteEntry
	cursor      int
	offset      int // first visible row of the remote list
	pane        remotePane
	ahead       int
	behind      int
	hasUpstream bool

	vp      viewport.Model
	vpReady bool

	spin       spinner.Model // animates while a fetch is in flight
	loading    bool
	helpOpen   bool
	errMsg     string
	successMsg string
	lay        layout
}

// newFetchSpinner builds the in-flight indicator. Its frames come from the
// glyph table's world: the braille animation on a UTF-8 terminal and the ASCII
// |/-\ one everywhere else, so a POSIX-locale session gets no replacement
// boxes.
func newFetchSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	if style.G == style.ASCIIGlyphs {
		s.Spinner = spinner.Line
	}
	s.Style = style.KeyCap
	return s
}

// ── messages ──────────────────────────────────────────────────────────────────

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

// ── Init ─────────────────────────────────────────────────────────────────────

func (m remoteModel) Init() tea.Cmd { return doRemoteLoad() }

// ── Update ───────────────────────────────────────────────────────────────────

func (m remoteModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.lay = m.lay.resize(msg.Width, msg.Height)
		if m.vpReady {
			m.vp.Width = m.lay.Width
			m.vp.Height = m.outputRows()
		}
		m.offset = m.window().Offset

	case spinner.TickMsg:
		// The animation only runs while a fetch is in flight, so completion or
		// failure stops it by simply not asking for the next frame.
		if !m.loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case remoteListMsg:
		m.remotes = msg.remotes
		m.ahead, m.behind, m.hasUpstream = msg.ahead, msg.behind, msg.hasUpstream
		m.loading = false
		if m.cursor >= len(m.remotes) && len(m.remotes) > 0 {
			m.cursor = len(m.remotes) - 1
		}
		m.offset = m.window().Offset

	case remoteErrMsg:
		m.errMsg = msg.err.Error()
		m.loading = false

	case remoteFetchMsg:
		m.loading = false
		m.pane = remotePaneOutput
		m.vp = viewport.New(m.lay.norm().Width, m.outputRows())
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

// outputRows is the height of the fetch-output viewport: header, blank
// separator and footer come off the terminal height.
func (m remoteModel) outputRows() int { return bodyRows(m.lay, 3) }

// listRows is how many remotes fit under the list header, banners included.
func (m remoteModel) listRows() int { return bodyRows(m.lay, len(m.listHead())+1) }

// window is the scrolling state of the remote list.
func (m remoteModel) window() listWindow {
	return listWindow{
		Cursor: m.cursor,
		Offset: m.offset,
		Total:  len(m.remotes),
		Rows:   m.listRows(),
	}.clamp()
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
		if m.cursor < len(m.remotes)-1 {
			m.cursor++
		}
	case "f": // fetch selected remote
		if m.cursor < len(m.remotes) {
			m.loading = true
			m.successMsg = ""
			return m, tea.Batch(doFetch(m.remotes[m.cursor].Name), m.spin.Tick)
		}
	case "F": // fetch all
		m.loading = true
		m.successMsg = ""
		return m, tea.Batch(doFetch(""), m.spin.Tick)
	case "r": // refresh ahead/behind
		return m, doRemoteLoad()
	}
	m.offset = m.window().Offset
	return m, nil
}

// ── hints ────────────────────────────────────────────────────────────────────

func remoteListHints() []keyHint {
	return []keyHint{
		{Keys: "f", Desc: i18n.T("key.fetch")},
		{Keys: "F", Desc: i18n.T("key.fetch_all")},
		{Keys: "r", Desc: i18n.T("key.refresh")},
		moveHint(),
		quitHint(),
	}
}

// ── View ─────────────────────────────────────────────────────────────────────

func (m remoteModel) View() string {
	if m.pane == remotePaneOutput {
		return m.viewOutput()
	}
	return m.viewList()
}

// position reports "cursor/total" without depending on the visible row count.
func (m remoteModel) position() string {
	return listWindow{Cursor: m.cursor, Total: len(m.remotes), Rows: 1}.position()
}

// upstreamLine summarizes how far the current branch is from its upstream. The
// arrows come from the glyph table, so an ASCII terminal shows ^2 v0.
func (m remoteModel) upstreamLine() string {
	if !m.hasUpstream {
		return style.MetaDim.Render(i18n.T("remote.no_upstream"))
	}
	line := style.Label.Render(i18n.T("remote.upstream_status")) +
		style.Ahead.Render(fmt.Sprintf("%s%d", style.G.Up, m.ahead)) + " " +
		style.Behind.Render(fmt.Sprintf("%s%d", style.G.Down, m.behind))
	if m.ahead == 0 && m.behind == 0 {
		line += style.Success.Render(i18n.T("remote.up_to_date"))
	}
	return line
}

// fetchLine is the animated in-flight indicator. A model built without
// newFetchSpinner (a zero-value spinner has no frames) degrades to the static
// breadcrumb glyph instead of rendering the spinner's "(error)" placeholder.
func (m remoteModel) fetchLine() string {
	frame := style.KeyCap.Render(style.G.Crumb)
	if len(m.spin.Spinner.Frames) > 0 {
		frame = m.spin.View()
	}
	return frame + " " + style.MetaDim.Render(i18n.T("remote.fetching"))
}

// listHead is every line above the remote list: header, upstream summary and
// the live banners, including the fetch indicator.
func (m remoteModel) listHead() []string {
	l := m.lay.norm()

	meta := i18n.Tf("meta.remotes", len(m.remotes))
	if pos := m.position(); pos != "" {
		meta += "  " + pos
	}
	lines := []string{
		header(l, "remote", "", meta),
		"",
		style.Truncate(m.upstreamLine(), l.Width),
		"",
	}

	if m.loading {
		lines = append(lines, style.Truncate(m.fetchLine(), l.Width), "")
	}
	if b := banner(l, bannerError, m.errMsg); b != "" {
		lines = append(lines, b, "")
	}
	if b := banner(l, bannerSuccess, m.successMsg); b != "" {
		lines = append(lines, b, "")
	}
	return lines
}

func (m remoteModel) viewList() string {
	l := m.lay.norm()
	hints := remoteListHints()
	head := strings.Join(m.listHead(), "\n")
	foot := footer(l, hints, true)

	if m.helpOpen {
		return frameOverlay(l, head, hints, foot)
	}

	if len(m.remotes) == 0 {
		body := style.MetaDim.Render(i18n.T("remote.none"))
		return frameFull(l, head, style.Truncate(body, l.Width), foot)
	}

	w := m.window()
	rl := listLayout(l, w)
	start, end := w.bounds()
	var lines []string
	for i := start; i < end; i++ {
		r := m.remotes[i]
		content := style.Ref.Render(r.Name) + "  " + style.Subject.Render(r.FetchURL)
		lines = append(lines, row(rl, i == w.Cursor, content))
	}
	return frameFull(l, head, listBody(l, w, lines), foot)
}

func (m remoteModel) viewOutput() string {
	l := m.lay.norm()

	head := header(l, "remote", i18n.T("key.fetch"), "") + "\n"
	foot := footer(l, scrollHints(), false)

	if !m.vpReady {
		return frameFull(l, head, style.MetaDim.Render("  "+i18n.T("common.loading")), foot)
	}
	return frameFull(l, head, m.vp.View(), foot)
}

// ── RunRemote ────────────────────────────────────────────────────────────────

func RunRemote() {
	m := remoteModel{spin: newFetchSpinner(), lay: newLayout()}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
