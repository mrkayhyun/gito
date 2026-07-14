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
	rebaseHashStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F1C40F")).Bold(true)
	rebaseSubStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#ECECEC"))
	rebaseDropStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Strikethrough(true)

	rebasePickStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#27AE60")).Bold(true)
	rebaseRewordStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#3498DB")).Bold(true)
	rebaseSquashStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E67E22")).Bold(true)
	rebaseFixupStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#9B59B6")).Bold(true)
	rebaseDropTag     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F87")).Bold(true)
)

type rebaseMode int

const (
	rebaseModeList rebaseMode = iota
	rebaseModeReword
	rebaseModeConfirm
)

type rebaseModel struct {
	commits    []git.RebaseCommit // oldest first (git-todo order); top of list = oldest
	actions    []git.RebaseAction // parallel to commits
	newMsgs    []string           // parallel to commits (reword messages)
	origHashes []string           // original order to detect reordering

	base        string
	hasUpstream bool

	cursor int
	offset int
	mode   rebaseMode

	input textinput.Model // reword message

	errMsg     string
	successMsg string

	finished  bool
	resultMsg string
	resultErr bool

	width, height int
}

// ── messages ──────────────────────────────────────────────────────────────────

type rebaseDoneMsg struct {
	backupRef string
	err       error
}

func doRebase(base string, steps []git.RebaseStep) tea.Cmd {
	return func() tea.Msg {
		backupRef, err := git.RunInteractiveRebase(base, steps)
		return rebaseDoneMsg{backupRef: backupRef, err: err}
	}
}

// ── helpers ────────────────────────────────────────────────────────────────────

// actionLabel returns the short git-todo verb for an action.
func actionLabel(a git.RebaseAction) string { return a.String() }

// actionStyle returns the lipgloss style used to render an action label.
func actionStyle(a git.RebaseAction) lipgloss.Style {
	switch a {
	case git.ActionReword:
		return rebaseRewordStyle
	case git.ActionSquash:
		return rebaseSquashStyle
	case git.ActionFixup:
		return rebaseFixupStyle
	case git.ActionDrop:
		return rebaseDropTag
	default:
		return rebasePickStyle
	}
}

// buildSteps assembles the rebase steps from the current model state.
func (m rebaseModel) buildSteps() []git.RebaseStep {
	steps := make([]git.RebaseStep, len(m.commits))
	for i, c := range m.commits {
		steps[i] = git.RebaseStep{
			Hash:       c.Hash,
			Action:     m.actions[i],
			NewMessage: m.newMsgs[i],
		}
	}
	return steps
}

// changed reports whether the user modified anything (an action was set or the
// order was altered) so an all-pick, original-order plan is treated as a no-op.
func (m rebaseModel) changed() bool {
	for i, a := range m.actions {
		if a != git.ActionPick {
			return true
		}
		if i < len(m.origHashes) && m.commits[i].Hash != m.origHashes[i] {
			return true
		}
	}
	return false
}

// validate builds the todo with placeholder reword paths to surface any plan
// error before execution. It never touches the repository.
func (m rebaseModel) validate() error {
	steps := m.buildSteps()
	rewordPaths := make([]string, len(steps))
	for i, s := range steps {
		if s.Action == git.ActionReword {
			rewordPaths[i] = "placeholder"
		}
	}
	_, err := git.BuildRebaseTodo(steps, rewordPaths)
	return err
}

func (m rebaseModel) visibleRows() int {
	v := m.height - 8
	if v < 1 {
		return 10
	}
	return v
}

// ── Init ─────────────────────────────────────────────────────────────────────

func (m rebaseModel) Init() tea.Cmd { return nil }

// ── Update ───────────────────────────────────────────────────────────────────

func (m rebaseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case rebaseDoneMsg:
		m.finished = true
		if msg.err != nil {
			m.resultErr = true
			m.resultMsg = msg.err.Error()
		} else {
			m.resultErr = false
			m.resultMsg = fmt.Sprintf(
				"리베이스 완료. 백업 ref: %s\n되돌리려면: git reset --hard %s   (또는 gito reflog 로 복구)",
				msg.backupRef, msg.backupRef,
			)
		}
		return m, nil
	case tea.KeyMsg:
		if m.finished {
			return m, tea.Quit
		}
		switch m.mode {
		case rebaseModeReword:
			return m.updateReword(msg)
		case rebaseModeConfirm:
			return m.updateConfirm(msg)
		default:
			return m.updateList(msg)
		}
	}
	return m, nil
}

func (m rebaseModel) updateReword(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode = rebaseModeList
		m.input.Blur()
		return m, nil
	case "enter":
		val := strings.TrimSpace(m.input.Value())
		if val == "" {
			m.errMsg = "새 커밋 메시지를 입력하세요"
			return m, nil
		}
		m.newMsgs[m.cursor] = val
		m.actions[m.cursor] = git.ActionReword
		m.errMsg = ""
		m.mode = rebaseModeList
		m.input.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m rebaseModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		return m, doRebase(m.base, m.buildSteps())
	case "ctrl+c":
		return m, tea.Quit
	default:
		m.mode = rebaseModeList
		return m, nil
	}
}

func (m rebaseModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.errMsg = ""
	m.successMsg = ""
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
		if m.cursor < len(m.commits)-1 {
			m.cursor++
			if m.cursor >= m.offset+vis {
				m.offset = m.cursor - vis + 1
			}
		}
	case "g":
		m.cursor, m.offset = 0, 0
	case "G":
		m.cursor = len(m.commits) - 1
		if m.cursor-vis+1 > 0 {
			m.offset = m.cursor - vis + 1
		}
	case "p":
		m.actions[m.cursor] = git.ActionPick
	case "r":
		m.mode = rebaseModeReword
		m.input.SetValue(m.commits[m.cursor].Subject)
		m.input.CursorEnd()
		return m, m.input.Focus()
	case "s":
		m.actions[m.cursor] = git.ActionSquash
	case "f":
		m.actions[m.cursor] = git.ActionFixup
	case "d":
		m.actions[m.cursor] = git.ActionDrop
	case "K", "shift+up", "ctrl+up":
		if m.cursor > 0 {
			i := m.cursor
			m.commits[i], m.commits[i-1] = m.commits[i-1], m.commits[i]
			m.actions[i], m.actions[i-1] = m.actions[i-1], m.actions[i]
			m.newMsgs[i], m.newMsgs[i-1] = m.newMsgs[i-1], m.newMsgs[i]
			m.cursor--
			if m.cursor < m.offset {
				m.offset = m.cursor
			}
		}
	case "J", "shift+down", "ctrl+down":
		if m.cursor < len(m.commits)-1 {
			i := m.cursor
			m.commits[i], m.commits[i+1] = m.commits[i+1], m.commits[i]
			m.actions[i], m.actions[i+1] = m.actions[i+1], m.actions[i]
			m.newMsgs[i], m.newMsgs[i+1] = m.newMsgs[i+1], m.newMsgs[i]
			m.cursor++
			if m.cursor >= m.offset+vis {
				m.offset = m.cursor - vis + 1
			}
		}
	case "enter":
		if !m.changed() {
			m.successMsg = "변경 사항이 없습니다. (모두 pick, 순서 그대로)"
			return m, nil
		}
		if err := m.validate(); err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		m.mode = rebaseModeConfirm
		return m, nil
	}
	return m, nil
}

// ── View ─────────────────────────────────────────────────────────────────────

func (m rebaseModel) View() string {
	if m.finished {
		return m.viewResult()
	}
	switch m.mode {
	case rebaseModeReword:
		return m.viewReword()
	case rebaseModeConfirm:
		return m.viewConfirm()
	default:
		return m.viewList()
	}
}

func (m rebaseModel) scopeHeader() string {
	if m.hasUpstream {
		return "upstream(@{upstream}) 이후의 미푸시 커밋"
	}
	return fmt.Sprintf("최근 %d개 커밋 (upstream 없음)", len(m.commits))
}

func (m rebaseModel) viewList() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito rebase"))
	sb.WriteString(style.Dimmed.Render("  "+m.scopeHeader()) + "\n")
	sb.WriteString(style.Dimmed.Render("맨 위 행이 가장 오래된 커밋입니다.") + "\n")
	sb.WriteString(style.Dimmed.Render(
		"p:pick r:reword s:squash f:fixup d:drop  K/J:이동  enter:확인  esc:취소",
	) + "\n\n")

	if m.errMsg != "" {
		sb.WriteString(style.Failure.Render("! "+m.errMsg) + "\n\n")
	}
	if m.successMsg != "" {
		sb.WriteString(style.Success.Render("✓ "+m.successMsg) + "\n\n")
	}

	vis := m.visibleRows()
	end := m.offset + vis
	if end > len(m.commits) {
		end = len(m.commits)
	}
	for i := m.offset; i < end; i++ {
		c := m.commits[i]
		act := m.actions[i]
		label := actionStyle(act).Render(fmt.Sprintf("%-6s", actionLabel(act)))

		subject := c.Subject
		if act == git.ActionReword && m.newMsgs[i] != "" {
			subject = m.newMsgs[i]
		}
		var subj string
		if act == git.ActionDrop {
			subj = rebaseDropStyle.Render(subject)
		} else {
			subj = rebaseSubStyle.Render(subject)
		}
		row := label + " " + rebaseHashStyle.Render(c.Short) + " " + subj

		if i == m.cursor {
			sb.WriteString(cursorGlyp.Render("▶") + " " + selRowBg.Render(row) + "\n")
		} else {
			sb.WriteString("  " + row + "\n")
		}
	}
	return sb.String()
}

func (m rebaseModel) viewReword() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito rebase  ›  reword") + "\n\n")
	if m.cursor < len(m.commits) {
		c := m.commits[m.cursor]
		sb.WriteString(style.Label.Render("대상: ") +
			rebaseHashStyle.Render(c.Short) + " " +
			rebaseSubStyle.Render(c.Subject) + "\n\n")
	}
	sb.WriteString(style.Label.Render("새 커밋 메시지:") + "\n\n")
	sb.WriteString(m.input.View() + "\n")
	if m.errMsg != "" {
		sb.WriteString("\n" + style.Failure.Render("! "+m.errMsg) + "\n")
	}
	sb.WriteString("\n" + style.Dimmed.Render("enter: 저장   esc: 취소"))
	return sb.String()
}

func (m rebaseModel) viewConfirm() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito rebase  ›  확인") + "\n\n")
	sb.WriteString(style.Label.Render("아래 계획대로 히스토리를 다시 씁니다:") + "\n\n")

	for i, c := range m.commits {
		act := m.actions[i]
		label := actionStyle(act).Render(fmt.Sprintf("%-6s", actionLabel(act)))
		subject := c.Subject
		if act == git.ActionReword && m.newMsgs[i] != "" {
			subject = m.newMsgs[i]
		}
		var subj string
		if act == git.ActionDrop {
			subj = rebaseDropStyle.Render(subject)
		} else {
			subj = rebaseSubStyle.Render(subject)
		}
		sb.WriteString("  " + label + " " + rebaseHashStyle.Render(c.Short) + " " + subj + "\n")
	}

	note := "실행 전 원래 HEAD를 가리키는 백업 ref가 자동 생성됩니다. "
	if m.hasUpstream {
		note += "upstream 이후의 아직 푸시하지 않은 커밋만 영향을 받습니다."
	} else {
		note += "upstream이 없어 최근 로컬 커밋을 대상으로 합니다. 이미 푸시된 커밋이 포함될 수 있으니 주의하세요."
	}
	sb.WriteString("\n" + style.Dimmed.Render(note) + "\n")
	sb.WriteString(style.Failure.Render("정말 실행하시겠습니까?") + "\n")
	sb.WriteString(style.Label.Render("y: 실행   다른 키: 취소") + "\n")
	return sb.String()
}

func (m rebaseModel) viewResult() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito rebase  ›  결과") + "\n\n")
	if m.resultErr {
		sb.WriteString(style.Failure.Render("! "+m.resultMsg) + "\n")
	} else {
		sb.WriteString(style.Success.Render("✓ "+m.resultMsg) + "\n")
	}
	sb.WriteString("\n" + style.Dimmed.Render("아무 키나 누르면 종료합니다."))
	return sb.String()
}

// ── RunRebase ──────────────────────────────────────────────────────────────────

func RunRebase() {
	commits, base, hasUpstream, err := git.RebasePlan()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if len(commits) == 0 {
		fmt.Println("정리할 커밋이 없습니다.")
		return
	}

	origHashes := make([]string, len(commits))
	for i, c := range commits {
		origHashes[i] = c.Hash
	}

	input := textinput.New()
	input.Placeholder = "새 커밋 메시지"
	input.CharLimit = 200

	m := rebaseModel{
		commits:     commits,
		actions:     make([]git.RebaseAction, len(commits)),
		newMsgs:     make([]string, len(commits)),
		origHashes:  origHashes,
		base:        base,
		hasUpstream: hasUpstream,
		input:       input,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
