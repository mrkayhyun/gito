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
	undoHashStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#F1C40F")).Bold(true)
	undoSelStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#8E44AD"))
	undoSubStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#ECECEC"))
	undoRemovedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F87"))
	undoRestoreStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#27AE60"))
)

// kindStyle colours the operation-kind badge so destructive-looking operations
// (reset, rebase) stand out from ordinary ones.
func kindStyle(kind string) lipgloss.Style {
	switch kind {
	case "reset", "rebase":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#E67E22")).Bold(true)
	case "amend", "revert":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#9B59B6")).Bold(true)
	case "merge", "pull", "cherry-pick":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#3498DB")).Bold(true)
	case "commit":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#27AE60")).Bold(true)
	default:
		return style.Dimmed
	}
}

type undoMode int

const (
	undoModeList undoMode = iota
	undoModeConfirm
)

type undoModel struct {
	ops    []git.UndoableOp
	cursor int
	offset int
	mode   undoMode

	preview    git.UndoPreview
	previewErr string

	errMsg string

	finished  bool
	resultMsg string
	resultErr bool

	width, height int
}

// ── messages ──────────────────────────────────────────────────────────────────

type undoDoneMsg struct {
	backupRef string
	err       error
}

func doUndo(op git.UndoableOp) tea.Cmd {
	return func() tea.Msg {
		backupRef, err := git.RunUndo(op)
		return undoDoneMsg{backupRef: backupRef, err: err}
	}
}

// ── Init / Update ──────────────────────────────────────────────────────────────

func (m undoModel) Init() tea.Cmd { return nil }

func (m undoModel) visibleRows() int {
	v := m.height - 6
	if v < 1 {
		return 10
	}
	return v
}

func (m undoModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case undoDoneMsg:
		m.finished = true
		if msg.err != nil {
			m.resultErr = true
			m.resultMsg = msg.err.Error()
		} else {
			m.resultErr = false
			m.resultMsg = fmt.Sprintf(
				"되돌리기 완료. 백업 ref: %s\n이 되돌리기를 취소하려면: git reset --hard %s   (또는 gito undo 를 다시 실행)",
				msg.backupRef, msg.backupRef,
			)
		}
		return m, nil
	case tea.KeyMsg:
		if m.finished {
			return m, tea.Quit
		}
		if m.mode == undoModeConfirm {
			return m.updateConfirm(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

func (m undoModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.errMsg = ""
	vis := m.visibleRows()
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit
	case "up", "k", "ctrl+p":
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.offset {
				m.offset = m.cursor
			}
		}
	case "down", "j", "ctrl+n":
		if m.cursor < len(m.ops)-1 {
			m.cursor++
			if m.cursor >= m.offset+vis {
				m.offset = m.cursor - vis + 1
			}
		}
	case "g":
		m.cursor, m.offset = 0, 0
	case "G":
		m.cursor = len(m.ops) - 1
		if m.cursor-vis+1 > 0 {
			m.offset = m.cursor - vis + 1
		}
	case "enter", "l", "right":
		if m.cursor >= len(m.ops) {
			return m, nil
		}
		preview, err := git.PreviewUndo(m.ops[m.cursor])
		if err != nil {
			m.previewErr = err.Error()
			m.preview = git.UndoPreview{}
		} else {
			m.previewErr = ""
			m.preview = preview
		}
		m.mode = undoModeConfirm
		return m, nil
	}
	return m, nil
}

func (m undoModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if m.preview.NoChange || m.previewErr != "" {
			// Nothing to do (or preview failed): fall back to the list rather
			// than run a no-op / risky reset.
			m.mode = undoModeList
			return m, nil
		}
		return m, doUndo(m.ops[m.cursor])
	case "ctrl+c":
		return m, tea.Quit
	default:
		m.mode = undoModeList
		return m, nil
	}
}

// ── View ─────────────────────────────────────────────────────────────────────

func (m undoModel) View() string {
	if m.finished {
		return m.viewResult()
	}
	if m.mode == undoModeConfirm {
		return m.viewConfirm()
	}
	return m.viewList()
}

func (m undoModel) viewList() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito undo"))
	sb.WriteString(style.Dimmed.Render("  최근 git 작업을 골라 그 이전 상태로 되돌립니다") + "\n")
	sb.WriteString(style.Dimmed.Render(
		"↑/↓ j/k: 이동   g/G: 처음/끝   enter: 미리보기·되돌리기   q/esc: 종료",
	) + "\n\n")

	if m.errMsg != "" {
		sb.WriteString(style.Failure.Render("! "+m.errMsg) + "\n\n")
	}
	if len(m.ops) == 0 {
		sb.WriteString(style.Dimmed.Render("  되돌릴 수 있는 작업이 없습니다.") + "\n")
		return sb.String()
	}

	vis := m.visibleRows()
	end := m.offset + vis
	if end > len(m.ops) {
		end = len(m.ops)
	}
	for i := m.offset; i < end; i++ {
		op := m.ops[i]
		badge := kindStyle(op.Kind).Render(fmt.Sprintf("%-11s", op.Kind))
		row := badge + " " +
			undoSelStyle.Render(fmt.Sprintf("%-9s", op.Selector)) + " " +
			undoSubStyle.Render(truncate(op.Description, 52))
		if i == m.cursor {
			sb.WriteString(cursorGlyp.Render("▶") + " " + selRowBg.Render(row) + "\n")
		} else {
			sb.WriteString("  " + row + "\n")
		}
	}
	return sb.String()
}

func (m undoModel) viewConfirm() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito undo  ›  미리보기") + "\n\n")

	op := m.ops[m.cursor]
	sb.WriteString(style.Label.Render("되돌릴 작업: ") +
		kindStyle(op.Kind).Render(op.Kind) + " " +
		undoSelStyle.Render(op.Selector) + " " +
		undoSubStyle.Render(op.Description) + "\n")
	sb.WriteString(style.Label.Render("돌아갈 상태: ") +
		undoHashStyle.Render(op.FromShort) + " " +
		undoSubStyle.Render(op.FromSubject) + "\n\n")

	if m.previewErr != "" {
		sb.WriteString(style.Failure.Render("! "+m.previewErr) + "\n")
		sb.WriteString("\n" + style.Dimmed.Render("아무 키나 누르면 목록으로 돌아갑니다.") + "\n")
		return sb.String()
	}
	if m.preview.NoChange {
		sb.WriteString(style.Success.Render("✓ HEAD가 이미 이 상태입니다. 되돌릴 것이 없습니다.") + "\n")
		sb.WriteString("\n" + style.Dimmed.Render("아무 키나 누르면 목록으로 돌아갑니다.") + "\n")
		return sb.String()
	}

	if len(m.preview.Removed) > 0 {
		sb.WriteString(style.Label.Render(fmt.Sprintf("사라지는 커밋 (%d):", len(m.preview.Removed))) + "\n")
		for _, c := range clampChanges(m.preview.Removed, 8) {
			sb.WriteString("  " + undoRemovedStyle.Render("− "+c.Short) + " " + undoSubStyle.Render(truncate(c.Subject, 60)) + "\n")
		}
		if len(m.preview.Removed) > 8 {
			sb.WriteString(style.Dimmed.Render(fmt.Sprintf("  … 외 %d개", len(m.preview.Removed)-8)) + "\n")
		}
		sb.WriteString("\n")
	}
	if len(m.preview.Restored) > 0 {
		sb.WriteString(style.Label.Render(fmt.Sprintf("복원되는 커밋 (%d):", len(m.preview.Restored))) + "\n")
		for _, c := range clampChanges(m.preview.Restored, 8) {
			sb.WriteString("  " + undoRestoreStyle.Render("+ "+c.Short) + " " + undoSubStyle.Render(truncate(c.Subject, 60)) + "\n")
		}
		if len(m.preview.Restored) > 8 {
			sb.WriteString(style.Dimmed.Render(fmt.Sprintf("  … 외 %d개", len(m.preview.Restored)-8)) + "\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(style.Dimmed.Render(
		"실행 전 현재 HEAD를 가리키는 백업 ref가 자동 생성됩니다. 추적 중인 변경사항이 있으면 먼저 커밋/스태시해야 합니다.",
	) + "\n")
	sb.WriteString(style.Failure.Render("정말 이 상태로 되돌리시겠습니까?") + "\n")
	sb.WriteString(style.Label.Render("y: 되돌리기   다른 키: 취소") + "\n")
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

// ── helpers ────────────────────────────────────────────────────────────────────

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max < 1 {
		return ""
	}
	return string(r[:max-1]) + "…"
}

func clampChanges(changes []git.UndoChange, max int) []git.UndoChange {
	if len(changes) <= max {
		return changes
	}
	return changes[:max]
}

// RunUndo shows the interactive undo picker.
func RunUndo() {
	ops, err := git.RecentUndoableOps(100)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if len(ops) == 0 {
		fmt.Println("되돌릴 수 있는 작업이 없습니다.")
		return
	}

	p := tea.NewProgram(undoModel{ops: ops}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
