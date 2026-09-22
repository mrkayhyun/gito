package ui

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mrkayhyun/gito/internal/git"
	"github.com/mrkayhyun/gito/internal/i18n"
	"github.com/mrkayhyun/gito/internal/style"
)

// ── styles ────────────────────────────────────────────────────────────────────

var (
	stagedColor    = lipgloss.NewStyle().Foreground(lipgloss.Color("#2ECC71"))
	unstagedColor  = lipgloss.NewStyle().Foreground(lipgloss.Color("#E74C3C"))
	untrackedColor = lipgloss.NewStyle().Foreground(lipgloss.Color("#F39C12"))
	sectionHead    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#9B59B6"))
	selRowBg       = lipgloss.NewStyle().Background(lipgloss.Color("#1E1B2E"))
)

// ── data ─────────────────────────────────────────────────────────────────────

type statusSection int

const (
	secStaged statusSection = iota
	secUnstaged
	secUntracked
)

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
	pane    statusPane

	vp           viewport.Model
	vpReady      bool
	patch        git.FilePatch
	hunk         int
	diffEntry    statusEntry
	diffRequest  uint64
	diffBusy     bool
	diffApplying bool

	confirmDiscard bool
	errMsg         string
	width, height  int
}

// ── messages ──────────────────────────────────────────────────────────────────

type statusEntriesMsg struct{ entries []statusEntry }
type statusErrMsg struct{ err error }
type statusDiffMsg struct {
	patch   git.FilePatch
	request uint64
	err     error
}
type statusHunkDoneMsg struct {
	request uint64
	err     error
}

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

func doStatusDiff(e statusEntry, request uint64) tea.Cmd {
	return func() tea.Msg {
		if e.section == secUntracked {
			return statusDiffMsg{patch: git.FilePatch{Content: i18n.T("status.untracked_note")}, request: request}
		}
		patch, err := git.GetFilePatch(e.file.Path, e.section == secStaged)
		return statusDiffMsg{patch: patch, request: request, err: err}
	}
}

// ── Init ─────────────────────────────────────────────────────────────────────

func (m statusModel) Init() tea.Cmd { return doStatusLoad() }

// ── Update ───────────────────────────────────────────────────────────────────

func (m statusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.vpReady {
			m.vp.Width = msg.Width
			m.vp.Height = m.vpHeight()
		}

	case statusEntriesMsg:
		m.entries = msg.entries
		if m.cursor >= len(m.entries) && len(m.entries) > 0 {
			m.cursor = len(m.entries) - 1
		}
		m.errMsg = ""

	case statusErrMsg:
		m.errMsg = msg.err.Error()

	case statusDiffMsg:
		if m.pane != statusPaneDiff || msg.request != m.diffRequest {
			return m, nil
		}
		m.diffBusy = false
		m.patch = msg.patch
		m.errMsg = ""
		if msg.err != nil {
			m.errMsg = msg.err.Error()
		}
		if m.hunk >= len(m.patch.Hunks) {
			m.hunk = len(m.patch.Hunks) - 1
		}
		if m.hunk < 0 {
			m.hunk = 0
		}
		m.vp = viewport.New(m.width, m.vpHeight())
		m.vpReady = true
		m.renderHunks()

	case statusHunkDoneMsg:
		if m.pane != statusPaneDiff || msg.request != m.diffRequest {
			return m, nil
		}
		m.diffApplying = false
		m.diffBusy = false
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			if errors.Is(msg.err, git.ErrStalePatch) {
				m.errMsg = i18n.T("status.hunk_stale")
			}
			return m, nil
		}
		return m.loadDiff()

	case tea.KeyMsg:
		if m.pane == statusPaneDiff {
			return m.updateDiff(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

func (m statusModel) vpHeight() int {
	h := m.height - 6
	if h < 1 {
		return 1
	}
	return h
}

func (m statusModel) updateDiff(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if m.diffApplying {
		return m, nil
	}
	switch msg.String() {
	case "q", "esc":
		m.pane = statusPaneList
		m.vpReady = false
		m.diffBusy = false
		return m, doStatusLoad()
	case "r":
		if !m.diffBusy {
			return m.loadDiff()
		}
	case "n", "p":
		if m.diffBusy || len(m.patch.Hunks) == 0 {
			return m, nil
		}
		if msg.String() == "n" && m.hunk+1 < len(m.patch.Hunks) {
			m.hunk++
		}
		if msg.String() == "p" && m.hunk > 0 {
			m.hunk--
		}
		m.renderHunks()
		return m, nil
	case " ":
		if m.diffBusy || !m.vpReady || len(m.patch.Hunks) == 0 {
			return m, nil
		}
		m.diffBusy, m.diffApplying = true, true
		m.errMsg = ""
		patch, hunk, request := m.patch, m.hunk, m.diffRequest
		return m, func() tea.Msg { return statusHunkDoneMsg{request: request, err: git.ApplyHunk(patch, hunk)} }
	}
	if m.vpReady {
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m statusModel) loadDiff() (tea.Model, tea.Cmd) {
	m.diffRequest++
	m.diffBusy = true
	m.vpReady = false
	m.errMsg = ""
	return m, doStatusDiff(m.diffEntry, m.diffRequest)
}

func (m *statusModel) renderHunks() {
	content := m.patch.Content
	if content == "" {
		content = i18n.T("status.no_diff")
	}
	lines := strings.Split(content, "\n")
	selectedLine := -1
	if len(m.patch.Hunks) > 0 {
		selectedLine = m.patch.Hunks[m.hunk].StartLine
	}
	for i, line := range lines {
		switch {
		case i == selectedLine:
			lines[i] = sectionHead.Render("▶ " + line)
		case strings.HasPrefix(line, "@@ "):
			lines[i] = style.Label.Render("  " + line)
		case strings.HasPrefix(line, "+"):
			lines[i] = stagedColor.Render("  " + line)
		case strings.HasPrefix(line, "-"):
			lines[i] = unstagedColor.Render("  " + line)
		default:
			lines[i] = "  " + line
		}
	}
	m.vp.SetContent(strings.Join(lines, "\n"))
	if selectedLine >= 0 {
		m.vp.SetYOffset(selectedLine)
	}
}

func (m statusModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// discard confirmation overlay
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

	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit
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
			m.diffEntry = m.entries[m.cursor]
			m.hunk = 0
			return m.loadDiff()
		}
	case "D":
		if m.cursor < len(m.entries) {
			e := m.entries[m.cursor]
			if e.section == secUnstaged {
				m.confirmDiscard = true
			}
		}
	}
	return m, nil
}

// ── View ─────────────────────────────────────────────────────────────────────

func (m statusModel) View() string {
	if m.pane == statusPaneDiff {
		return m.viewDiff()
	}
	return m.viewList()
}

func (m statusModel) viewList() string {
	var sb strings.Builder

	nStaged, nUnstaged, nUntracked := 0, 0, 0
	for _, e := range m.entries {
		switch e.section {
		case secStaged:
			nStaged++
		case secUnstaged:
			nUnstaged++
		case secUntracked:
			nUntracked++
		}
	}

	sb.WriteString(style.Title.Render("gito status"))
	sb.WriteString(style.Dimmed.Render(fmt.Sprintf(
		"  staged:%d  unstaged:%d  untracked:%d", nStaged, nUnstaged, nUntracked,
	)) + "\n")
	sb.WriteString(style.Dimmed.Render(i18n.T("status.hint_list")) + "\n\n")

	if m.confirmDiscard && m.cursor < len(m.entries) {
		sb.WriteString(style.Failure.Render(
			i18n.Tf("status.discard_confirm", m.entries[m.cursor].file.Path),
		) + "\n")
		sb.WriteString(style.Label.Render(i18n.T("common.confirm_yn")) + "\n\n")
	}
	if m.errMsg != "" {
		sb.WriteString(style.Failure.Render("! "+m.errMsg) + "\n\n")
	}

	if len(m.entries) == 0 {
		sb.WriteString(style.Dimmed.Render(i18n.T("status.clean")) + "\n")
		return sb.String()
	}

	prevSec := statusSection(-1)
	for i, e := range m.entries {
		// section header
		if e.section != prevSec {
			prevSec = e.section
			switch e.section {
			case secStaged:
				sb.WriteString("\n" + sectionHead.Render("── Staged ──") + "\n")
			case secUnstaged:
				sb.WriteString("\n" + sectionHead.Render("── Unstaged ──") + "\n")
			case secUntracked:
				sb.WriteString("\n" + sectionHead.Render("── Untracked ──") + "\n")
			}
		}

		xy := string([]byte{e.file.Staged, e.file.Unstaged})
		path := e.file.Path

		var row string
		switch e.section {
		case secStaged:
			row = stagedColor.Render(xy) + " " + stagedColor.Render(path)
		case secUnstaged:
			row = unstagedColor.Render(xy) + " " + unstagedColor.Render(path)
		case secUntracked:
			row = untrackedColor.Render("??") + " " + untrackedColor.Render(path)
		}

		if i == m.cursor {
			sb.WriteString(cursorGlyp.Render("▶") + " " + selRowBg.Render(row) + "\n")
		} else {
			sb.WriteString("  " + row + "\n")
		}
	}

	return sb.String()
}

func (m statusModel) viewDiff() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito status  ›  diff") + "\n")
	{
		e := m.diffEntry
		var c lipgloss.Style
		switch e.section {
		case secStaged:
			c = stagedColor
		case secUnstaged:
			c = unstagedColor
		default:
			c = untrackedColor
		}
		sb.WriteString(c.Render(e.file.Path))
	}
	sb.WriteString("\n")
	hint := i18n.T("status.hunk_hint_readonly")
	if len(m.patch.Hunks) > 0 {
		if m.diffEntry.section == secStaged {
			hint = i18n.T("status.hunk_hint_unstage")
		} else {
			hint = i18n.T("status.hunk_hint_stage")
		}
	}
	sb.WriteString(style.Dimmed.Render(hint) + "\n")
	if m.errMsg != "" {
		sb.WriteString(style.Failure.Render(m.errMsg))
	} else if m.diffBusy {
		sb.WriteString(style.Dimmed.Render(i18n.T("common.loading")))
	} else if len(m.patch.Hunks) > 0 {
		sb.WriteString(style.Label.Render(i18n.Tf("status.hunk_position", m.hunk+1, len(m.patch.Hunks))))
	} else if m.patch.Content != "" {
		sb.WriteString(style.Dimmed.Render(i18n.T("status.hunk_readonly")))
	}
	sb.WriteString("\n\n")

	if !m.vpReady {
		sb.WriteString(style.Dimmed.Render("  " + i18n.T("common.loading")))
		return sb.String()
	}
	sb.WriteString(m.vp.View())
	return sb.String()
}

// ── RunStatus ────────────────────────────────────────────────────────────────

func RunStatus() {
	p := tea.NewProgram(statusModel{}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
