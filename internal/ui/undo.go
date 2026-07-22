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
	undoHashStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#F1C40F")).Bold(true)
	undoSubStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#ECECEC"))
	undoActionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#3498DB")).Bold(true)
)

type undoPane int

const (
	undoPaneInfo undoPane = iota
	undoPaneConfirm
	undoPaneResult
)

type undoModel struct {
	pane undoPane
	info *git.UndoInfo

	useHard bool // whether to use --hard (user choice)

	resultMsg string
	resultErr bool

	width, height int
}

// ── messages ──────────────────────────────────────────────────────────────────

type undoInfoMsg struct{ info *git.UndoInfo }
type undoErrMsg struct{ err error }
type undoDoneMsg struct{ err error }

func doLoadUndoInfo() tea.Cmd {
	return func() tea.Msg {
		info, err := git.GetUndoInfo()
		if err != nil {
			return undoErrMsg{err}
		}
		return undoInfoMsg{info}
	}
}

func doRunUndo(hard bool) tea.Cmd {
	return func() tea.Msg {
		var err error
		if hard {
			err = git.RunUndoHard()
		} else {
			err = git.RunUndo()
		}
		return undoDoneMsg{err}
	}
}

// ── Init ─────────────────────────────────────────────────────────────────────

func (m undoModel) Init() tea.Cmd { return doLoadUndoInfo() }

// ── Update ───────────────────────────────────────────────────────────────────

func (m undoModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case undoInfoMsg:
		m.info = msg.info
		if m.info == nil {
			m.pane = undoPaneResult
			m.resultErr = false
			m.resultMsg = "되돌릴 작업이 없습니다 (reflog에 이전 HEAD가 없음)."
		}
	case undoErrMsg:
		m.pane = undoPaneResult
		m.resultErr = true
		m.resultMsg = msg.err.Error()
	case undoDoneMsg:
		m.pane = undoPaneResult
		if msg.err != nil {
			m.resultErr = true
			m.resultMsg = msg.err.Error()
		} else {
			m.resultErr = false
			if m.useHard {
				m.resultMsg = "되돌리기 완료 (--hard). 작업 디렉토리가 이전 상태로 복원되었습니다."
			} else {
				m.resultMsg = "되돌리기 완료 (--soft). 변경 사항이 스테이징 영역에 보존되었습니다."
			}
		}
	case tea.KeyMsg:
		if m.pane == undoPaneResult {
			return m, tea.Quit
		}
		switch m.pane {
		case undoPaneInfo:
			return m.updateInfo(msg)
		case undoPaneConfirm:
			return m.updateConfirm(msg)
		}
	}
	return m, nil
}

func (m undoModel) updateInfo(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit
	case "s":
		// soft undo
		m.useHard = false
		m.pane = undoPaneConfirm
	case "h":
		// hard undo
		m.useHard = true
		m.pane = undoPaneConfirm
	case "enter":
		// default: soft
		m.useHard = false
		m.pane = undoPaneConfirm
	}
	return m, nil
}

func (m undoModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		return m, doRunUndo(m.useHard)
	case "ctrl+c":
		return m, tea.Quit
	default:
		m.pane = undoPaneInfo
	}
	return m, nil
}

// ── View ─────────────────────────────────────────────────────────────────────

func (m undoModel) View() string {
	switch m.pane {
	case undoPaneInfo:
		return m.viewInfo()
	case undoPaneConfirm:
		return m.viewConfirm()
	case undoPaneResult:
		return m.viewResult()
	default:
		return ""
	}
}

func (m undoModel) viewInfo() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito undo"))
	sb.WriteString(style.Dimmed.Render("  마지막 git 작업 되돌리기") + "\n\n")

	if m.info == nil {
		sb.WriteString(style.Dimmed.Render("  로딩 중...") + "\n")
		return sb.String()
	}

	sb.WriteString(style.Label.Render("마지막 작업:") + " " +
		undoActionStyle.Render(m.info.Action) + "\n\n")

	sb.WriteString(style.Label.Render("현재 HEAD:") + "\n")
	sb.WriteString("  " + undoHashStyle.Render(m.info.CurrentHash[:7]) + " " +
		undoSubStyle.Render(m.info.CurrentSubject) + "\n\n")

	sb.WriteString(style.Label.Render("되돌릴 대상 (HEAD@{1}):") + "\n")
	sb.WriteString("  " + undoHashStyle.Render(m.info.PreviousHash[:7]) + " " +
		undoSubStyle.Render(m.info.PreviousSubject) + "\n\n")

	sb.WriteString(style.Dimmed.Render("────────────────────────────────────────") + "\n")
	sb.WriteString(style.Label.Render("s") + style.Dimmed.Render("/enter: soft 되돌리기 (변경 사항 보존)") + "\n")
	sb.WriteString(style.Label.Render("h") + style.Dimmed.Render(":      hard 되돌리기 (변경 사항 삭제)") + "\n")
	sb.WriteString(style.Dimmed.Render("q/esc: 취소") + "\n")

	return sb.String()
}

func (m undoModel) viewConfirm() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito undo  ›  확인") + "\n\n")

	mode := "soft (변경 사항 보존)"
	if m.useHard {
		mode = "hard (변경 사항 삭제)"
	}
	sb.WriteString(style.Label.Render("모드: ") + undoActionStyle.Render(mode) + "\n\n")

	if m.info != nil {
		sb.WriteString(style.Label.Render("HEAD를 다음으로 이동:") + "\n")
		sb.WriteString("  " + undoHashStyle.Render(m.info.PreviousHash[:7]) + " " +
			undoSubStyle.Render(m.info.PreviousSubject) + "\n\n")
	}

	if m.useHard {
		sb.WriteString(style.Failure.Render("⚠ hard reset은 작업 디렉토리의 변경 사항을 삭제합니다!") + "\n")
	}
	sb.WriteString(style.Failure.Render("정말 실행하시겠습니까?") + "\n")
	sb.WriteString(style.Label.Render("y: 실행   다른 키: 취소") + "\n")
	return sb.String()
}

func (m undoModel) viewResult() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito undo  ›  결과") + "\n\n")
	if m.resultErr {
		sb.WriteString(style.Failure.Render("! "+m.resultMsg) + "\n")
	} else {
		sb.WriteString(style.Success.Render("✓ "+m.resultMsg) + "\n")
	}
	sb.WriteString("\n" + style.Dimmed.Render("아무 키나 누르면 종료합니다."))
	return sb.String()
}

// ── RunUndo ──────────────────────────────────────────────────────────────────

func RunUndo() {
	m := undoModel{}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
