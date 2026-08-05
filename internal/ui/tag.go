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

type tagPane int

const (
	tagPaneList tagPane = iota
	tagPaneShow
	tagPaneCreate
)

// ── model ─────────────────────────────────────────────────────────────────────

type tagModel struct {
	tags   []git.TagEntry
	cursor int
	offset int // first visible row of the tag list
	pane   tagPane

	vp      viewport.Model
	vpReady bool

	// create form
	nameInput textinput.Model
	msgInput  textinput.Model
	createIdx int // 0 = name field, 1 = message field

	confirmDelete       bool
	confirmRemoteDelete bool
	helpOpen            bool
	errMsg              string
	successMsg          string
	lay                 layout
}

// ── messages ──────────────────────────────────────────────────────────────────

type tagListMsg struct{ tags []git.TagEntry }
type tagErrMsg struct{ err error }
type tagShowMsg struct{ content string }

func doTagLoad() tea.Cmd {
	return func() tea.Msg {
		tags, err := git.GetTags()
		if err != nil {
			return tagErrMsg{err}
		}
		return tagListMsg{tags}
	}
}

func doTagShow(name string) tea.Cmd {
	return func() tea.Msg {
		content, err := git.ShowTag(name)
		if err != nil {
			return tagShowMsg{"Error: " + err.Error()}
		}
		if content == "" {
			return tagShowMsg{i18n.T("common.empty")}
		}
		return tagShowMsg{content}
	}
}

// ── Init ─────────────────────────────────────────────────────────────────────

func (m tagModel) Init() tea.Cmd { return doTagLoad() }

// ── Update ───────────────────────────────────────────────────────────────────

func (m tagModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.lay = m.lay.resize(msg.Width, msg.Height)
		if m.vpReady {
			m.vp.Width = m.lay.Width
			m.vp.Height = m.showRows()
		}
		m.offset = m.window().Offset
		m.fitFormWidth()

	case tagListMsg:
		m.tags = msg.tags
		if m.cursor >= len(m.tags) && len(m.tags) > 0 {
			m.cursor = len(m.tags) - 1
		}
		m.offset = m.window().Offset

	case tagErrMsg:
		m.errMsg = msg.err.Error()

	case tagShowMsg:
		m.vp = viewport.New(m.lay.norm().Width, m.showRows())
		m.vp.SetContent(msg.content)
		m.vpReady = true

	case tea.KeyMsg:
		switch m.pane {
		case tagPaneShow:
			return m.updateShow(msg)
		case tagPaneCreate:
			return m.updateCreate(msg)
		default:
			return m.updateList(msg)
		}
	}
	return m, nil
}

// showRows is the height of the tag detail viewport: header, tag summary, blank
// separator and footer come off the terminal height.
func (m tagModel) showRows() int { return bodyRows(m.lay, 4) }

// listRows is how many tags fit under the list header, banners included.
func (m tagModel) listRows() int { return bodyRows(m.lay, len(m.listHead())+1) }

// window is the scrolling state of the tag list.
func (m tagModel) window() listWindow {
	return listWindow{
		Cursor: m.cursor,
		Offset: m.offset,
		Total:  len(m.tags),
		Rows:   m.listRows(),
	}.clamp()
}

// fitFormWidth keeps the create-form inputs inside the terminal, so a narrow
// window cannot make the form wrap.
func (m *tagModel) fitFormWidth() {
	w := max(m.lay.norm().Width-tagLabelWidth()-6, 10)
	m.nameInput.Width = w
	m.msgInput.Width = w
}

// tagLabelWidth is the column width shared by the two form labels so the
// fields line up in every locale.
func tagLabelWidth() int {
	return max(
		style.DisplayWidth(i18n.T("tag.field_name")),
		style.DisplayWidth(i18n.T("tag.field_message")),
	) + 1 // trailing colon
}

func (m tagModel) updateShow(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.pane = tagPaneList
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

func (m tagModel) updateCreate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.pane = tagPaneList
		m.nameInput.Blur()
		m.msgInput.Blur()
		return m, nil
	case "tab", "down":
		m.createIdx = (m.createIdx + 1) % 2
		return m, m.focusCreateField()
	case "shift+tab", "up":
		m.createIdx = (m.createIdx + 1) % 2
		return m, m.focusCreateField()
	case "enter":
		// enter on the name field advances to message; on message it submits.
		if m.createIdx == 0 {
			m.createIdx = 1
			return m, m.focusCreateField()
		}
		name := strings.TrimSpace(m.nameInput.Value())
		if name == "" {
			m.errMsg = i18n.T("tag.err_name_required")
			return m, nil
		}
		if err := git.CreateTag(name, m.msgInput.Value(), ""); err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		m.successMsg = i18n.T("tag.created") + name
		m.pane = tagPaneList
		m.nameInput.Blur()
		m.msgInput.Blur()
		return m, doTagLoad()
	}

	var cmd tea.Cmd
	if m.createIdx == 0 {
		m.nameInput, cmd = m.nameInput.Update(msg)
	} else {
		m.msgInput, cmd = m.msgInput.Update(msg)
	}
	return m, cmd
}

func (m *tagModel) focusCreateField() tea.Cmd {
	if m.createIdx == 0 {
		m.msgInput.Blur()
		return m.nameInput.Focus()
	}
	m.nameInput.Blur()
	return m.msgInput.Focus()
}

func (m tagModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// local delete confirmation
	if m.confirmDelete {
		switch msg.String() {
		case "y", "Y":
			m.confirmDelete = false
			if m.cursor < len(m.tags) {
				name := m.tags[m.cursor].Name
				if err := git.DeleteTag(name); err != nil {
					m.errMsg = err.Error()
					return m, nil
				}
				m.successMsg = i18n.T("tag.deleted") + name
			}
			return m, doTagLoad()
		default:
			m.confirmDelete = false
		}
		return m, nil
	}

	// remote delete confirmation
	if m.confirmRemoteDelete {
		switch msg.String() {
		case "y", "Y":
			m.confirmRemoteDelete = false
			if m.cursor < len(m.tags) {
				name := m.tags[m.cursor].Name
				if err := git.DeleteRemoteTag(name, "origin"); err != nil {
					m.errMsg = err.Error()
					return m, nil
				}
				m.successMsg = i18n.Tf("tag.deleted_origin", name)
			}
			return m, doTagLoad()
		default:
			m.confirmRemoteDelete = false
		}
		return m, nil
	}

	// Key overlay. It sits below both confirmations so an armed confirmation
	// still treats every key as confirm-or-cancel.
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
		if m.cursor < len(m.tags)-1 {
			m.cursor++
		}
	case "g":
		m.cursor = 0
		m.offset = 0
	case "G":
		m.cursor = len(m.tags) - 1
		if m.cursor < 0 {
			m.cursor = 0
		}
	case "enter", "d": // show detail/diff
		if m.cursor < len(m.tags) {
			m.confirmDelete = false
			m.confirmRemoteDelete = false
			m.pane = tagPaneShow
			m.vpReady = false
			return m, doTagShow(m.tags[m.cursor].Name)
		}
	case "c": // create tag on HEAD
		m.confirmDelete = false
		m.confirmRemoteDelete = false
		m.nameInput.SetValue("")
		m.msgInput.SetValue("")
		m.createIdx = 0
		m.pane = tagPaneCreate
		m.fitFormWidth()
		return m, m.focusCreateField()
	case "p": // push tag to origin
		if m.cursor < len(m.tags) {
			name := m.tags[m.cursor].Name
			if err := git.PushTag(name, "origin"); err != nil {
				m.errMsg = err.Error()
				return m, nil
			}
			m.successMsg = i18n.Tf("tag.pushed", name)
		}
	case "P": // delete tag on origin
		if m.cursor < len(m.tags) {
			m.confirmDelete = false
			m.confirmRemoteDelete = true
		}
	case "D": // delete
		if m.cursor < len(m.tags) {
			m.confirmRemoteDelete = false
			m.confirmDelete = true
		}
	}
	m.offset = m.window().Offset
	return m, nil
}

// ── hints ────────────────────────────────────────────────────────────────────

func tagListHints() []keyHint {
	return []keyHint{
		{Keys: "enter/d", Desc: i18n.T("key.detail")},
		{Keys: "c", Desc: i18n.T("key.create")},
		{Keys: "p", Desc: i18n.T("key.push")},
		{Keys: "P", Desc: i18n.T("key.remote_delete")},
		{Keys: "D", Desc: i18n.T("key.delete")},
		moveHint(),
		{Keys: "g/G", Desc: i18n.T("key.top_bottom")},
		quitHint(),
	}
}

func tagCreateHints() []keyHint {
	return []keyHint{
		{Keys: "tab", Desc: i18n.T("key.field")},
		{Keys: "enter", Desc: i18n.T("key.next")},
		{Keys: "esc", Desc: i18n.T("key.cancel")},
	}
}

// ── View ─────────────────────────────────────────────────────────────────────

func (m tagModel) View() string {
	switch m.pane {
	case tagPaneShow:
		return m.viewShow()
	case tagPaneCreate:
		return m.viewCreate()
	default:
		return m.viewList()
	}
}

// position reports "cursor/total" without depending on the visible row count.
func (m tagModel) position() string {
	return listWindow{Cursor: m.cursor, Total: len(m.tags), Rows: 1}.position()
}

// listHead is every line above the tag list: header, blank separator and the
// live banners, including both destructive-confirmation bars.
func (m tagModel) listHead() []string {
	l := m.lay.norm()

	meta := i18n.Tf("meta.tags", len(m.tags))
	if pos := m.position(); pos != "" {
		meta += "  " + pos
	}
	lines := []string{header(l, "tag", "", meta), ""}

	if m.confirmDelete && m.cursor < len(m.tags) {
		prompt := i18n.Tf("tag.delete_confirm", m.tags[m.cursor].Name)
		lines = append(lines, splitLines(confirmBar(l, prompt))...)
		lines = append(lines, "")
	}
	if m.confirmRemoteDelete && m.cursor < len(m.tags) {
		prompt := i18n.Tf("tag.remote_delete_confirm", m.tags[m.cursor].Name)
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

func (m tagModel) viewList() string {
	l := m.lay.norm()
	hints := tagListHints()
	head := strings.Join(m.listHead(), "\n")
	foot := footer(l, hints, true)

	if m.helpOpen {
		return frameOverlay(l, head, hints, foot)
	}

	if len(m.tags) == 0 {
		body := style.MetaDim.Render(i18n.T("tag.none"))
		return frameFull(l, head, style.Truncate(body, l.Width), foot)
	}

	w := m.window()
	rl := listLayout(l, w)
	start, end := w.bounds()
	var lines []string
	for i := start; i < end; i++ {
		lines = append(lines, row(rl, i == w.Cursor, tagLine(m.tags[i])))
	}
	return frameFull(l, head, listBody(l, w, lines), foot)
}

// tagLine renders one tag as name, kind, target hash, date and subject.
func tagLine(t git.TagEntry) string {
	kind := "lw"
	if t.Annotated {
		kind = "annot"
	}
	return style.Ref.Render(t.Name) + " " +
		style.AuthorName.Render("["+kind+"]") + " " +
		style.Hash.Render(t.TargetHash) + " " +
		style.Date.Render(t.Date) + " " +
		style.Subject.Render(t.Subject)
}

func (m tagModel) viewShow() string {
	l := m.lay.norm()

	summary := ""
	if m.cursor < len(m.tags) {
		t := m.tags[m.cursor]
		summary = style.Ref.Render(t.Name) + "  " + style.Subject.Render(t.Subject)
	}
	head := strings.Join([]string{
		header(l, "tag", i18n.T("key.detail"), m.position()),
		style.Truncate(summary, l.Width),
		"",
	}, "\n")
	foot := footer(l, scrollHints(), false)

	if !m.vpReady {
		return frameFull(l, head, style.MetaDim.Render("  "+i18n.T("common.loading")), foot)
	}
	return frameFull(l, head, m.vp.View(), foot)
}

func (m tagModel) viewCreate() string {
	l := m.lay.norm()

	head := header(l, "tag", i18n.T("key.create"), "") + "\n"
	foot := footer(l, tagCreateHints(), false)

	labelW := tagLabelWidth()
	nameLabel := style.Pad(i18n.T("tag.field_name")+":", labelW)
	msgLabel := style.Pad(i18n.T("tag.field_message")+":", labelW)
	if m.createIdx == 0 {
		nameLabel, msgLabel = style.Badge.Render(nameLabel), style.Label.Render(msgLabel)
	} else {
		nameLabel, msgLabel = style.Label.Render(nameLabel), style.Badge.Render(msgLabel)
	}

	lines := []string{
		style.Truncate(style.MetaDim.Render(i18n.T("tag.create_on_head")), l.Width),
		"",
		style.Truncate(nameLabel+" "+m.nameInput.View(), l.Width),
		"",
		style.Truncate(msgLabel+" "+m.msgInput.View(), l.Width),
		"",
		style.Truncate(style.MetaDim.Render(i18n.T("tag.create_note")), l.Width),
	}
	if b := banner(l, bannerError, m.errMsg); b != "" {
		lines = append(lines, "", b)
	}
	return frameFull(l, head, strings.Join(lines, "\n"), foot)
}

// ── RunTag ───────────────────────────────────────────────────────────────────

func RunTag() {
	name := textinput.New()
	name.Placeholder = "v1.0.0"
	name.CharLimit = 100

	msg := textinput.New()
	msg.Placeholder = i18n.T("tag.ph_msg")
	msg.CharLimit = 200

	m := tagModel{
		nameInput: name,
		msgInput:  msg,
		lay:       newLayout(),
	}
	m.fitFormWidth()

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
