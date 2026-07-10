package ui

import (
	"fmt"
	"os"
	"strings"

	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"gito/internal/config"
	"gito/internal/git"
	"gito/internal/style"
)

// subjectRecommendedLen is the Conventional Commits recommended maximum length
// for a commit subject line. The textinput hard CharLimit stays higher (72);
// this is a soft guideline surfaced to the user.
const subjectRecommendedLen = 50

// subjectLenHint returns a counter label like "32/50" for the given subject
// rune count and reports whether the recommended length has been exceeded.
// warn becomes true only once n is strictly greater than the recommended limit.
func subjectLenHint(n int) (label string, warn bool) {
	return fmt.Sprintf("%d/%d", n, subjectRecommendedLen), n > subjectRecommendedLen
}

type commitStep int

const (
	stepType commitStep = iota
	stepScope
	stepSubject
	stepBody
	stepConfirm
	stepDone
)

type commitModel struct {
	step       commitStep
	cursor     int
	typeIdx    int
	typeKeys   []string // e.g. ["feat","fix",...]
	typeLabels []string // e.g. ["feat   새 기능",...]
	scope      textinput.Model
	subject    textinput.Model
	body       textinput.Model
	err        error
	done       bool
	amend      bool // committed via --amend
}

func newCommitModel() commitModel {
	scope := textinput.New()
	scope.Placeholder = "optional scope (e.g. auth, api)"
	scope.CharLimit = 50

	subject := textinput.New()
	subject.Placeholder = "short description"
	subject.CharLimit = 72

	body := textinput.New()
	body.Placeholder = "optional longer description"
	body.CharLimit = 500

	cfg := config.Load()
	keys := make([]string, len(cfg.CommitTypes))
	labels := make([]string, len(cfg.CommitTypes))
	for i, ct := range cfg.CommitTypes {
		keys[i] = ct.Key
		labels[i] = ct.Label
	}

	return commitModel{
		step:       stepType,
		typeKeys:   keys,
		typeLabels: labels,
		scope:      scope,
		subject:    subject,
		body:       body,
	}
}

func (m commitModel) buildMessage() string {
	t := m.typeKeys[m.typeIdx]
	scope := m.scope.Value()
	subject := m.subject.Value()
	body := m.body.Value()

	msg := t
	if scope != "" {
		msg += "(" + scope + ")"
	}
	msg += ": " + subject
	if body != "" {
		msg += "\n\n" + body
	}
	return msg
}

func (m commitModel) Init() tea.Cmd {
	return nil
}

func (m commitModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.step {
		case stepType:
			switch msg.String() {
			case "ctrl+c", "q":
				return m, tea.Quit
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.typeLabels)-1 {
					m.cursor++
				}
			case "enter":
				m.typeIdx = m.cursor
				m.step = stepScope
				cmd := m.scope.Focus()
				return m, cmd
			}

		case stepScope:
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.step = stepType
				m.scope.Blur()
				return m, nil
			case "enter":
				m.step = stepSubject
				m.scope.Blur()
				cmd := m.subject.Focus()
				return m, cmd
			}
			var cmd tea.Cmd
			m.scope, cmd = m.scope.Update(msg)
			return m, cmd

		case stepSubject:
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.step = stepScope
				m.subject.Blur()
				cmd := m.scope.Focus()
				return m, cmd
			case "enter":
				if m.subject.Value() == "" {
					return m, nil
				}
				m.step = stepBody
				m.subject.Blur()
				cmd := m.body.Focus()
				return m, cmd
			}
			var cmd tea.Cmd
			m.subject, cmd = m.subject.Update(msg)
			return m, cmd

		case stepBody:
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.step = stepSubject
				m.body.Blur()
				cmd := m.subject.Focus()
				return m, cmd
			case "enter":
				m.step = stepConfirm
				m.body.Blur()
				return m, nil
			}
			var cmd tea.Cmd
			m.body, cmd = m.body.Update(msg)
			return m, cmd

		case stepConfirm:
			switch msg.String() {
			case "ctrl+c", "n":
				return m, tea.Quit
			case "y", "enter":
				err := git.Commit(m.buildMessage())
				m.err = err
				m.done = true
				return m, tea.Quit
			case "a": // amend the previous commit instead of creating a new one
				err := git.CommitAmend(m.buildMessage())
				m.err = err
				m.done = true
				m.amend = true
				return m, tea.Quit
			case "e":
				m.step = stepSubject
				cmd := m.subject.Focus()
				return m, cmd
			}
		}
	}
	return m, nil
}

func (m commitModel) View() string {
	var sb strings.Builder

	sb.WriteString(style.Title.Render("gito commit") + "\n\n")

	stepNames := []string{"Type", "Scope", "Subject", "Body", "Confirm"}
	var indicators []string
	for i, s := range stepNames {
		si := commitStep(i)
		if si == m.step {
			indicators = append(indicators, style.Selected.Render(s))
		} else if si < m.step {
			indicators = append(indicators, style.Success.Render("✓ "+s))
		} else {
			indicators = append(indicators, style.Dimmed.Render(s))
		}
	}
	sb.WriteString(strings.Join(indicators, style.Dimmed.Render(" → ")) + "\n\n")

	switch m.step {
	case stepType:
		sb.WriteString(style.Label.Render("커밋 타입을 선택하세요:") + "\n\n")
		for i, t := range m.typeLabels {
			if i == m.cursor {
				sb.WriteString(style.Selected.Render("▶ "+t) + "\n")
			} else {
				sb.WriteString(style.Normal.Render("  "+t) + "\n")
			}
		}
		sb.WriteString("\n" + style.Dimmed.Render("↑/↓: 이동   enter: 선택   q: 종료"))

	case stepScope:
		sb.WriteString(style.Label.Render("스코프 입력 (선택사항):") + "\n\n")
		sb.WriteString(style.Normal.Render(m.typeKeys[m.typeIdx] + "  "))
		sb.WriteString(m.scope.View() + "\n")
		sb.WriteString("\n" + style.Dimmed.Render("enter: 다음   esc: 이전   ctrl+c: 종료"))

	case stepSubject:
		sb.WriteString(style.Label.Render("커밋 메시지 입력:") + "\n\n")
		prefix := m.typeKeys[m.typeIdx]
		if m.scope.Value() != "" {
			prefix += "(" + m.scope.Value() + ")"
		}
		sb.WriteString(style.Normal.Render(prefix + ": "))
		sb.WriteString(m.subject.View() + "\n")
		label, warn := subjectLenHint(utf8.RuneCountInString(m.subject.Value()))
		if warn {
			sb.WriteString(style.Failure.Render(label+" (권장 "+fmt.Sprintf("%d", subjectRecommendedLen)+"자 초과)") + "\n")
		} else {
			sb.WriteString(style.Dimmed.Render(label) + "\n")
		}
		sb.WriteString("\n" + style.Dimmed.Render("enter: 다음   esc: 이전   ctrl+c: 종료"))

	case stepBody:
		sb.WriteString(style.Label.Render("본문 입력 (선택사항):") + "\n\n")
		sb.WriteString(m.body.View() + "\n")
		sb.WriteString("\n" + style.Dimmed.Render("enter: 다음   esc: 이전   ctrl+c: 종료"))

	case stepConfirm:
		sb.WriteString(style.Label.Render("커밋 메시지 확인:") + "\n\n")
		sb.WriteString(style.Border.Render(m.buildMessage()) + "\n\n")
		sb.WriteString(style.Label.Render("커밋하시겠습니까?  "))
		sb.WriteString(style.Selected.Render(" y ") + style.Normal.Render(" yes   "))
		sb.WriteString(style.Selected.Render(" n ") + style.Normal.Render(" no   "))
		sb.WriteString(style.Selected.Render(" a ") + style.Normal.Render(" amend   "))
		sb.WriteString(style.Selected.Render(" e ") + style.Normal.Render(" edit\n"))
		if last := git.GetLastCommitSubject(); last != "" {
			sb.WriteString("\n" + style.Dimmed.Render("a(amend) 시 대체될 직전 커밋: "+last) + "\n")
		}
	}

	return sb.String()
}

func RunCommit() {
	status, err := git.GetStatus()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if status == "" {
		fmt.Println("Nothing to commit. Use 'git add' to stage changes.")
		return
	}

	m := newCommitModel()
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	final := result.(commitModel)
	if final.err != nil {
		fmt.Fprintln(os.Stderr, style.Failure.Render("Commit failed: "+final.err.Error()))
		os.Exit(1)
	}
	if final.done {
		if final.amend {
			fmt.Println(style.Success.Render("✓ Amended previous commit"))
		} else {
			fmt.Println(style.Success.Render("✓ Committed successfully"))
		}
		fmt.Println(style.Dimmed.Render(final.buildMessage()))
	}
}
