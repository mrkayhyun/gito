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

type blamePane int

const (
	blamePanePick blamePane = iota
	blamePaneView
)

type blameModel struct {
	files    []string
	filter   textinput.Model
	cursor   int
	offset   int
	pane     blamePane
	selected string

	vp      viewport.Model
	vpReady bool

	width, height int
}

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

func (m blameModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.vpReady {
			m.vp.Width = msg.Width
			m.vp.Height = m.blameVPHeight()
		}
	case blameContentMsg:
		m.vp = viewport.New(m.width, m.blameVPHeight())
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

func (m blameModel) blameVPHeight() int {
	h := m.height - 3
	if h < 1 {
		return 1
	}
	return h
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
	vis := m.visibleRows()
	switch msg.String() {
	case "ctrl+c", "esc":
		return m, tea.Quit
	case "up", "ctrl+p":
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.offset {
				m.offset = m.cursor
			}
		}
		return m, nil
	case "down", "ctrl+n":
		if m.cursor < len(filtered)-1 {
			m.cursor++
			if m.cursor >= m.offset+vis {
				m.offset = m.cursor - vis + 1
			}
		}
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

func (m blameModel) visibleRows() int {
	v := m.height - 5
	if v < 1 {
		return 10
	}
	return v
}

func (m blameModel) View() string {
	if m.pane == blamePaneView {
		return m.viewBlame()
	}
	return m.viewPick()
}

func (m blameModel) viewPick() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito blame") + "\n\n")
	sb.WriteString(style.Label.Render(i18n.T("common.search")) + m.filter.View() + "\n\n")

	filtered := m.filtered()
	if len(filtered) == 0 {
		sb.WriteString(style.Dimmed.Render(i18n.T("blame.none")) + "\n")
	} else {
		vis := m.visibleRows()
		end := m.offset + vis
		if end > len(filtered) {
			end = len(filtered)
		}
		for i := m.offset; i < end; i++ {
			f := filtered[i]
			if i == m.cursor {
				sb.WriteString(style.Selected.Render("▶ "+f) + "\n")
			} else {
				sb.WriteString(style.Normal.Render("  "+f) + "\n")
			}
		}
	}
	sb.WriteString("\n" + style.Dimmed.Render(i18n.T("blame.hint_pick")))
	return sb.String()
}

func (m blameModel) viewBlame() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito blame  ›  ") + style.Label.Render(m.selected) + "\n")
	sb.WriteString(style.Dimmed.Render(i18n.T("hint.scroll_back")) + "\n\n")
	if !m.vpReady {
		sb.WriteString(style.Dimmed.Render("  " + i18n.T("common.loading")))
		return sb.String()
	}
	sb.WriteString(m.vp.View())
	return sb.String()
}

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

	p := tea.NewProgram(blameModel{files: files, filter: filter}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
