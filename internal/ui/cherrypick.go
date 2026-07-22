package ui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gito/internal/git"
	"gito/internal/style"
)

var (
	cpHashStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F1C40F")).Bold(true)
	cpSubStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#ECECEC"))
	cpDateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#27AE60"))
	cpPickStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true)
)

type cpPane int

const (
	cpPaneBranch cpPane = iota
	cpPaneCommit
	cpPaneConfirm
	cpPaneResult
)

type cpModel struct {
	pane cpPane

	branches      []string
	currentBranch string
	branchCursor  int

	commits      []git.CherryPickCommit
	selected     []bool // parallel to commits
	commitCursor int
	offset       int

	resultMsg string
	resultErr bool

	width, height int
}

// ── messages ──────────────────────────────────────────────────────────────────

type cpBranchMsg struct {
	branches []string
	current  string
}

type cpCommitsMsg struct {
	commits []git.CherryPickCommit
}

type cpDoneMsg struct {
	err   error
	count int
}

type cpErrMsg struct{ err error }

func doLoadBranches() tea.Cmd {
	return func() tea.Msg {
		branches, current, err := git.GetLocalBranches()
		if err != nil {
			return cpErrMsg{err}
		}
		return cpBranchMsg{branches: branches, current: current}
	}
}

func doLoadCPCommits(branch string) tea.Cmd {
	return func() tea.Msg {
		commits, err := git.GetCherryPickCandidates(branch, 50)
		if err != nil {
			return cpErrMsg{err}
		}
		return cpCommitsMsg{commits: commits}
	}
}

func doCherryPick(hashes []string) tea.Cmd {
	return func() tea.Msg {
		err := git.RunCherryPick(hashes)
		return cpDoneMsg{err: err, count: len(hashes)}
	}
}

// ── Init ─────────────────────────────────────────────────────────────────────

func (m cpModel) Init() tea.Cmd { return doLoadBranches() }

// ── Update ───────────────────────────────────────────────────────────────────

func (m cpModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case cpBranchMsg:
		m.currentBranch = msg.current
		// filter out the current branch
		for _, b := range msg.branches {
			if b != msg.current {
				m.branches = append(m.branches, b)
			}
		}
	case cpCommitsMsg:
		m.commits = msg.commits
		m.selected = make([]bool, len(msg.commits))
		m.commitCursor = 0
		m.offset = 0
		if len(m.commits) == 0 {
			m.pane = cpPaneResult
			m.resultErr = false
			m.resultMsg = "선택한 브랜치에 cherry-pick 할 새 커밋이 없습니다."
		} else {
			m.pane = cpPaneCommit
		}
	case cpDoneMsg:
		m.pane = cpPaneResult
		if msg.err != nil {
			m.resultErr = true
			m.resultMsg = msg.err.Error()
		} else {
			m.resultErr = false
			m.resultMsg = fmt.Sprintf("%d개 커밋을 성공적으로 cherry-pick 했습니다.", msg.count)
		}
	case cpErrMsg:
		m.pane = cpPaneResult
		m.resultErr = true
		m.resultMsg = msg.err.Error()
	case tea.KeyMsg:
		if m.pane == cpPaneResult {
			return m, tea.Quit
		}
		switch m.pane {
		case cpPaneBranch:
			return m.updateBranch(msg)
		case cpPaneCommit:
			return m.updateCommit(msg)
		case cpPaneConfirm:
			return m.updateConfirm(msg)
		}
	}
	return m, nil
}

func (m cpModel) updateBranch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit
	case "up", "k":
		if m.branchCursor > 0 {
			m.branchCursor--
		}
	case "down", "j":
		if m.branchCursor < len(m.branches)-1 {
			m.branchCursor++
		}
	case "g":
		m.branchCursor = 0
	case "G":
		if len(m.branches) > 0 {
			m.branchCursor = len(m.branches) - 1
		}
	case "enter":
		if m.branchCursor < len(m.branches) {
			return m, doLoadCPCommits(m.branches[m.branchCursor])
		}
	}
	return m, nil
}

func (m cpModel) visibleRows() int {
	v := m.height - 8
	if v < 1 {
		return 10
	}
	return v
}

func (m cpModel) updateCommit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	vis := m.visibleRows()
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit
	case "up", "k":
		if m.commitCursor > 0 {
			m.commitCursor--
			if m.commitCursor < m.offset {
				m.offset = m.commitCursor
			}
		}
	case "down", "j":
		if m.commitCursor < len(m.commits)-1 {
			m.commitCursor++
			if m.commitCursor >= m.offset+vis {
				m.offset = m.commitCursor - vis + 1
			}
		}
	case "g":
		m.commitCursor, m.offset = 0, 0
	case "G":
		m.commitCursor = len(m.commits) - 1
		if m.commitCursor-vis+1 > 0 {
			m.offset = m.commitCursor - vis + 1
		}
	case " ", "space":
		if m.commitCursor < len(m.selected) {
			m.selected[m.commitCursor] = !m.selected[m.commitCursor]
		}
	case "a":
		// toggle all
		allSelected := true
		for _, s := range m.selected {
			if !s {
				allSelected = false
				break
			}
		}
		for i := range m.selected {
			m.selected[i] = !allSelected
		}
	case "enter":
		// proceed to confirm if anything selected
		count := 0
		for _, s := range m.selected {
			if s {
				count++
			}
		}
		if count == 0 {
			return m, nil
		}
		m.pane = cpPaneConfirm
	}
	return m, nil
}

func (m cpModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		var hashes []string
		for i, s := range m.selected {
			if s {
				hashes = append(hashes, m.commits[i].Hash)
			}
		}
		return m, doCherryPick(hashes)
	case "ctrl+c":
		return m, tea.Quit
	default:
		m.pane = cpPaneCommit
	}
	return m, nil
}

// ── View ─────────────────────────────────────────────────────────────────────

func (m cpModel) View() string {
	switch m.pane {
	case cpPaneBranch:
		return m.viewBranch()
	case cpPaneCommit:
		return m.viewCommit()
	case cpPaneConfirm:
		return m.viewConfirm()
	case cpPaneResult:
		return m.viewResult()
	default:
		return ""
	}
}

func (m cpModel) viewBranch() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito cherry-pick"))
	sb.WriteString(style.Dimmed.Render("  소스 브랜치를 선택하세요") + "\n")
	sb.WriteString(style.Dimmed.Render(fmt.Sprintf("현재 브랜치: %s", m.currentBranch)) + "\n")
	sb.WriteString(style.Dimmed.Render("↑/↓ j/k: 이동   enter: 선택   q/esc: 종료") + "\n\n")

	if len(m.branches) == 0 {
		sb.WriteString(style.Dimmed.Render("  다른 로컬 브랜치가 없습니다.") + "\n")
		return sb.String()
	}

	for i, b := range m.branches {
		if i == m.branchCursor {
			sb.WriteString(cursorGlyp.Render("▶") + " " + selRowBg.Render(b) + "\n")
		} else {
			sb.WriteString("  " + b + "\n")
		}
	}
	return sb.String()
}

func (m cpModel) viewCommit() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito cherry-pick"))
	sb.WriteString(style.Dimmed.Render(fmt.Sprintf("  %s 에서 가져올 커밋 선택", m.branches[m.branchCursor])) + "\n")
	sb.WriteString(style.Dimmed.Render("space: 선택/해제   a: 전체토글   enter: 확인   q/esc: 종료") + "\n\n")

	count := 0
	for _, s := range m.selected {
		if s {
			count++
		}
	}
	if count > 0 {
		sb.WriteString(cpPickStyle.Render(fmt.Sprintf("  %d개 선택됨", count)) + "\n\n")
	}

	vis := m.visibleRows()
	end := m.offset + vis
	if end > len(m.commits) {
		end = len(m.commits)
	}
	for i := m.offset; i < end; i++ {
		c := m.commits[i]
		mark := "○"
		if m.selected[i] {
			mark = cpPickStyle.Render("●")
		}
		row := mark + " " + cpHashStyle.Render(c.Short) + " " +
			cpDateStyle.Render(c.Date) + " " +
			cpSubStyle.Render(c.Subject)

		if i == m.commitCursor {
			sb.WriteString(cursorGlyp.Render("▶") + " " + selRowBg.Render(row) + "\n")
		} else {
			sb.WriteString("  " + row + "\n")
		}
	}
	return sb.String()
}

func (m cpModel) viewConfirm() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito cherry-pick  ›  확인") + "\n\n")
	sb.WriteString(style.Label.Render("아래 커밋을 현재 브랜치에 cherry-pick 합니다:") + "\n\n")

	for i, c := range m.commits {
		if !m.selected[i] {
			continue
		}
		sb.WriteString("  " + cpHashStyle.Render(c.Short) + " " +
			cpDateStyle.Render(c.Date) + " " +
			cpSubStyle.Render(c.Subject) + "\n")
	}
	sb.WriteString("\n" + style.Dimmed.Render("충돌 발생 시 자동으로 abort 하여 원래 상태로 돌아갑니다.") + "\n")
	sb.WriteString(style.Failure.Render("정말 실행하시겠습니까?") + "\n")
	sb.WriteString(style.Label.Render("y: 실행   다른 키: 취소") + "\n")
	return sb.String()
}

func (m cpModel) viewResult() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito cherry-pick  ›  결과") + "\n\n")
	if m.resultErr {
		sb.WriteString(style.Failure.Render("! "+m.resultMsg) + "\n")
	} else {
		sb.WriteString(style.Success.Render("✓ "+m.resultMsg) + "\n")
	}
	sb.WriteString("\n" + style.Dimmed.Render("아무 키나 누르면 종료합니다."))
	return sb.String()
}

// ── RunCherryPick ────────────────────────────────────────────────────────────

func RunCherryPick() {
	m := cpModel{}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
