package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gito/internal/git"
	"gito/internal/style"
)

var (
	wtPathStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#3498DB")).Bold(true)
	wtBranchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#27AE60"))
	wtHashStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#F1C40F"))
)

type wtPane int

const (
	wtPaneList wtPane = iota
	wtPaneAdd
	wtPaneResult
)

type wtModel struct {
	pane   wtPane
	trees  []git.WorktreeEntry
	cursor int

	// add form
	pathInput   textinput.Model
	branchInput textinput.Model
	addIdx      int  // 0 = path, 1 = branch
	newBranch   bool // whether to create a new branch

	confirmRemove bool

	errMsg     string
	successMsg string

	width, height int
}

// ── messages ──────────────────────────────────────────────────────────────────

type wtListMsg struct{ trees []git.WorktreeEntry }
type wtErrMsg struct{ err error }

func doLoadWorktrees() tea.Cmd {
	return func() tea.Msg {
		trees, err := git.GetWorktrees()
		if err != nil {
			return wtErrMsg{err}
		}
		return wtListMsg{trees}
	}
}

// ── Init ─────────────────────────────────────────────────────────────────────

func (m wtModel) Init() tea.Cmd { return doLoadWorktrees() }

// ── Update ───────────────────────────────────────────────────────────────────

func (m wtModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case wtListMsg:
		m.trees = msg.trees
		if m.cursor >= len(m.trees) && len(m.trees) > 0 {
			m.cursor = len(m.trees) - 1
		}
	case wtErrMsg:
		m.errMsg = msg.err.Error()
	case tea.KeyMsg:
		if m.pane == wtPaneResult {
			return m, tea.Quit
		}
		switch m.pane {
		case wtPaneAdd:
			return m.updateAdd(msg)
		default:
			return m.updateList(msg)
		}
	}
	return m, nil
}

func (m wtModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirmRemove {
		switch msg.String() {
		case "y", "Y":
			m.confirmRemove = false
			if m.cursor < len(m.trees) {
				path := m.trees[m.cursor].Path
				if err := git.RemoveWorktree(path, false); err != nil {
					m.errMsg = err.Error()
				} else {
					m.successMsg = "Removed worktree: " + path
				}
			}
			return m, doLoadWorktrees()
		default:
			m.confirmRemove = false
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
		if m.cursor < len(m.trees)-1 {
			m.cursor++
		}
	case "g":
		m.cursor = 0
	case "G":
		if len(m.trees) > 0 {
			m.cursor = len(m.trees) - 1
		}
	case "a":
		m.newBranch = false
		m.pathInput.SetValue("")
		m.branchInput.SetValue("")
		m.addIdx = 0
		m.pane = wtPaneAdd
		return m, m.focusAddField()
	case "n":
		m.newBranch = true
		m.pathInput.SetValue("")
		m.branchInput.SetValue("")
		m.addIdx = 0
		m.pane = wtPaneAdd
		return m, m.focusAddField()
	case "D":
		if m.cursor < len(m.trees) && m.cursor > 0 { // don't remove main worktree
			m.confirmRemove = true
		}
	}
	return m, nil
}

func (m *wtModel) focusAddField() tea.Cmd {
	if m.addIdx == 0 {
		m.branchInput.Blur()
		return m.pathInput.Focus()
	}
	m.pathInput.Blur()
	return m.branchInput.Focus()
}

func (m wtModel) updateAdd(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.pane = wtPaneList
		m.pathInput.Blur()
		m.branchInput.Blur()
		return m, nil
	case "tab", "down":
		m.addIdx = (m.addIdx + 1) % 2
		return m, m.focusAddField()
	case "shift+tab", "up":
		m.addIdx = (m.addIdx + 1) % 2
		return m, m.focusAddField()
	case "enter":
		if m.addIdx == 0 {
			m.addIdx = 1
			return m, m.focusAddField()
		}
		path := strings.TrimSpace(m.pathInput.Value())
		branch := strings.TrimSpace(m.branchInput.Value())
		if path == "" || branch == "" {
			m.errMsg = "path and branch are required"
			return m, nil
		}
		if err := git.AddWorktree(path, branch, m.newBranch); err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		m.successMsg = "Created worktree at " + path
		m.pane = wtPaneList
		m.pathInput.Blur()
		m.branchInput.Blur()
		return m, doLoadWorktrees()
	}

	var cmd tea.Cmd
	if m.addIdx == 0 {
		m.pathInput, cmd = m.pathInput.Update(msg)
	} else {
		m.branchInput, cmd = m.branchInput.Update(msg)
	}
	return m, cmd
}

// ── View ─────────────────────────────────────────────────────────────────────

func (m wtModel) View() string {
	switch m.pane {
	case wtPaneAdd:
		return m.viewAdd()
	case wtPaneResult:
		return m.viewResult()
	default:
		return m.viewList()
	}
}

func (m wtModel) viewList() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito worktree"))
	sb.WriteString(style.Dimmed.Render(fmt.Sprintf("  %d worktrees", len(m.trees))) + "\n")
	sb.WriteString(style.Dimmed.Render(
		"↑/↓ j/k: 이동   a: 기존 브랜치로 추가   n: 새 브랜치로 추가   D: 삭제   q/esc: 종료",
	) + "\n\n")

	if m.confirmRemove && m.cursor < len(m.trees) {
		sb.WriteString(style.Failure.Render(
			"worktree를 삭제하시겠습니까? "+m.trees[m.cursor].Path,
		) + "\n")
		sb.WriteString(style.Label.Render("y: 확인   다른 키: 취소") + "\n\n")
	}
	if m.errMsg != "" {
		sb.WriteString(style.Failure.Render("! "+m.errMsg) + "\n\n")
	}
	if m.successMsg != "" {
		sb.WriteString(style.Success.Render("✓ "+m.successMsg) + "\n\n")
	}

	if len(m.trees) == 0 {
		sb.WriteString(style.Dimmed.Render("  No worktrees found.") + "\n")
		return sb.String()
	}

	for i, wt := range m.trees {
		branch := wt.Branch
		if branch == "" {
			branch = "(bare)"
		}
		row := wtPathStyle.Render(wt.Path) + "  " +
			wtBranchStyle.Render(branch) + "  " +
			wtHashStyle.Render(wt.Head)

		if i == m.cursor {
			sb.WriteString(cursorGlyp.Render("▶") + " " + selRowBg.Render(row) + "\n")
		} else {
			sb.WriteString("  " + row + "\n")
		}
	}
	return sb.String()
}

func (m wtModel) viewAdd() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito worktree  ›  add") + "\n\n")

	if m.newBranch {
		sb.WriteString(style.Dimmed.Render("새 브랜치를 생성하고 worktree에 연결합니다.") + "\n\n")
	} else {
		sb.WriteString(style.Dimmed.Render("기존 브랜치를 사용하여 worktree를 추가합니다.") + "\n\n")
	}

	pathLabel := style.Label.Render("Path:   ")
	branchLabel := style.Label.Render("Branch: ")
	if m.addIdx == 0 {
		pathLabel = style.Selected.Render("Path:")
	} else {
		branchLabel = style.Selected.Render("Branch:")
	}
	sb.WriteString(pathLabel + " " + m.pathInput.View() + "\n\n")
	sb.WriteString(branchLabel + " " + m.branchInput.View() + "\n\n")

	if m.errMsg != "" {
		sb.WriteString(style.Failure.Render("! "+m.errMsg) + "\n")
	}
	sb.WriteString("\n" + style.Dimmed.Render("tab: 필드 이동   enter: 다음/생성   esc: 취소"))
	return sb.String()
}

func (m wtModel) viewResult() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito worktree  ›  결과") + "\n\n")
	sb.WriteString(style.Success.Render("✓ "+m.successMsg) + "\n")
	sb.WriteString("\n" + style.Dimmed.Render("아무 키나 누르면 종료합니다."))
	return sb.String()
}

// ── RunWorktree ──────────────────────────────────────────────────────────────

func RunWorktree() {
	pathInput := textinput.New()
	pathInput.Placeholder = "../my-worktree"
	pathInput.CharLimit = 200

	branchInput := textinput.New()
	branchInput.Placeholder = "feature-branch"
	branchInput.CharLimit = 100

	m := wtModel{
		pathInput:   pathInput,
		branchInput: branchInput,
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
