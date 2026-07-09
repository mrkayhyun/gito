package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gito/internal/git"
	"gito/internal/style"
)

var (
	tagNameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F1C40F")).Bold(true)
	tagHashStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#8E44AD"))
	tagDateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#27AE60"))
	tagMsgStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#ECECEC"))
	tagKindStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#636363"))
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
	pane   tagPane

	vp      viewport.Model
	vpReady bool

	// create form
	nameInput textinput.Model
	msgInput  textinput.Model
	createIdx int // 0 = name field, 1 = message field

	confirmDelete bool
	errMsg        string
	successMsg    string
	width, height int
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
			return tagShowMsg{"(empty)"}
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
		m.width, m.height = msg.Width, msg.Height
		if m.vpReady {
			m.vp.Width = msg.Width
			m.vp.Height = m.tagVPHeight()
		}

	case tagListMsg:
		m.tags = msg.tags
		if m.cursor >= len(m.tags) && len(m.tags) > 0 {
			m.cursor = len(m.tags) - 1
		}

	case tagErrMsg:
		m.errMsg = msg.err.Error()

	case tagShowMsg:
		m.vp = viewport.New(m.width, m.tagVPHeight())
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

func (m tagModel) tagVPHeight() int {
	h := m.height - 4
	if h < 1 {
		return 1
	}
	return h
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
			m.errMsg = "tag name is required"
			return m, nil
		}
		if err := git.CreateTag(name, m.msgInput.Value(), ""); err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		m.successMsg = "Created tag " + name
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
	// delete confirmation
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
				m.successMsg = "Deleted tag " + name
			}
			return m, doTagLoad()
		default:
			m.confirmDelete = false
		}
		return m, nil
	}

	m.errMsg = ""
	m.successMsg = ""

	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit
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
	case "G":
		m.cursor = len(m.tags) - 1
		if m.cursor < 0 {
			m.cursor = 0
		}
	case "enter", "d": // show detail/diff
		if m.cursor < len(m.tags) {
			m.pane = tagPaneShow
			m.vpReady = false
			return m, doTagShow(m.tags[m.cursor].Name)
		}
	case "c": // create tag on HEAD
		m.nameInput.SetValue("")
		m.msgInput.SetValue("")
		m.createIdx = 0
		m.pane = tagPaneCreate
		return m, m.focusCreateField()
	case "p": // push tag to origin
		if m.cursor < len(m.tags) {
			name := m.tags[m.cursor].Name
			if err := git.PushTag(name, "origin"); err != nil {
				m.errMsg = err.Error()
				return m, nil
			}
			m.successMsg = "Pushed " + name + " to origin"
		}
	case "P": // delete tag on origin
		if m.cursor < len(m.tags) {
			name := m.tags[m.cursor].Name
			if err := git.DeleteRemoteTag(name, "origin"); err != nil {
				m.errMsg = err.Error()
				return m, nil
			}
			m.successMsg = "Deleted " + name + " on origin"
		}
	case "D": // delete
		if m.cursor < len(m.tags) {
			m.confirmDelete = true
		}
	}
	return m, nil
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

func (m tagModel) viewList() string {
	var sb strings.Builder

	sb.WriteString(style.Title.Render("gito tag"))
	sb.WriteString(style.Dimmed.Render(fmt.Sprintf("  %d tags", len(m.tags))) + "\n")
	sb.WriteString(style.Dimmed.Render(
		"↑/↓ j/k: 이동   enter/d: 상세   c: 생성   p: push   P: 원격삭제   D: 삭제   q/esc: 종료",
	) + "\n\n")

	if m.confirmDelete && m.cursor < len(m.tags) {
		sb.WriteString(style.Failure.Render(
			"태그를 삭제하시겠습니까? "+m.tags[m.cursor].Name,
		) + "\n")
		sb.WriteString(style.Label.Render("y: 확인   다른 키: 취소") + "\n\n")
	}
	if m.errMsg != "" {
		sb.WriteString(style.Failure.Render("! "+m.errMsg) + "\n\n")
	}
	if m.successMsg != "" {
		sb.WriteString(style.Success.Render("✓ "+m.successMsg) + "\n\n")
	}

	if len(m.tags) == 0 {
		sb.WriteString(style.Dimmed.Render("  No tags. Press 'c' to create one on HEAD.") + "\n")
		return sb.String()
	}

	for i, t := range m.tags {
		kind := "lw"
		if t.Annotated {
			kind = "annot"
		}
		row := tagNameStyle.Render(t.Name) + " " +
			tagKindStyle.Render("["+kind+"]") + " " +
			tagHashStyle.Render(t.TargetHash) + " " +
			tagDateStyle.Render(t.Date) + " " +
			tagMsgStyle.Render(t.Subject)

		if i == m.cursor {
			sb.WriteString(cursorGlyp.Render("▶") + " " + selRowBg.Render(row) + "\n")
		} else {
			sb.WriteString("  " + row + "\n")
		}
	}

	return sb.String()
}

func (m tagModel) viewShow() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito tag  ›  show") + "\n")
	if m.cursor < len(m.tags) {
		t := m.tags[m.cursor]
		sb.WriteString(tagNameStyle.Render(t.Name) + "  " + tagMsgStyle.Render(t.Subject))
	}
	sb.WriteString(style.Dimmed.Render("   ↑/↓: scroll   q/esc: back") + "\n\n")

	if !m.vpReady {
		sb.WriteString(style.Dimmed.Render("  Loading..."))
		return sb.String()
	}
	sb.WriteString(m.vp.View())
	return sb.String()
}

func (m tagModel) viewCreate() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito tag  ›  create") + "\n\n")
	sb.WriteString(style.Dimmed.Render("HEAD 커밋에 태그를 생성합니다.") + "\n\n")

	nameLabel := style.Label.Render("Name:    ")
	msgLabel := style.Label.Render("Message: ")
	if m.createIdx == 0 {
		nameLabel = style.Selected.Render("Name:")
	} else {
		msgLabel = style.Selected.Render("Message:")
	}
	sb.WriteString(nameLabel + " " + m.nameInput.View() + "\n\n")
	sb.WriteString(msgLabel + " " + m.msgInput.View() + "\n\n")
	sb.WriteString(style.Dimmed.Render("메시지가 비어있으면 lightweight 태그, 있으면 annotated 태그로 생성됩니다.") + "\n")

	if m.errMsg != "" {
		sb.WriteString("\n" + style.Failure.Render("! "+m.errMsg) + "\n")
	}

	sb.WriteString("\n" + style.Dimmed.Render("tab: 필드 이동   enter: 다음/생성   esc: 취소"))
	return sb.String()
}

// ── RunTag ───────────────────────────────────────────────────────────────────

func RunTag() {
	name := textinput.New()
	name.Placeholder = "v1.0.0"
	name.CharLimit = 100

	msg := textinput.New()
	msg.Placeholder = "optional annotation message"
	msg.CharLimit = 200

	m := tagModel{
		nameInput: name,
		msgInput:  msg,
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
