package ui

import (
	"fmt"
	"os"
	"strings"

	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mrkayhyun/gito/internal/config"
	"github.com/mrkayhyun/gito/internal/git"
	"github.com/mrkayhyun/gito/internal/i18n"
	"github.com/mrkayhyun/gito/internal/style"
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
	typeLabels []string // e.g. ["feat   New feature",...]
	scope      textinput.Model
	subject    textinput.Model
	body       textinput.Model
	err        error
	done       bool
	amend      bool // committed via --amend
}

func newCommitModel() commitModel {
	scope := textinput.New()
	scope.Placeholder = i18n.T("commit.ph_scope")
	scope.CharLimit = 50

	subject := textinput.New()
	subject.Placeholder = i18n.T("commit.ph_subject")
	subject.CharLimit = 72

	body := textinput.New()
	body.Placeholder = i18n.T("commit.ph_body")
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
			case "esc":
				m.step = stepBody
				cmd := m.body.Focus()
				return m, cmd
			}
		}
	}
	return m, nil
}

func (m commitModel) View() string {
	var sb strings.Builder

	sb.WriteString(style.Title.Render("gito commit") + "\n\n")

	stepNames := []string{
		i18n.T("commit.step_type"),
		i18n.T("commit.step_scope"),
		i18n.T("commit.step_subject"),
		i18n.T("commit.step_body"),
		i18n.T("commit.step_confirm"),
	}
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
		sb.WriteString(style.Label.Render(i18n.T("commit.select_type")) + "\n\n")
		for i, t := range m.typeLabels {
			if i == m.cursor {
				sb.WriteString(style.Selected.Render("▶ "+t) + "\n")
			} else {
				sb.WriteString(style.Normal.Render("  "+t) + "\n")
			}
		}
		sb.WriteString("\n" + style.Dimmed.Render(i18n.T("commit.hint_type")))

	case stepScope:
		sb.WriteString(style.Label.Render(i18n.T("commit.enter_scope")) + "\n\n")
		sb.WriteString(style.Normal.Render(m.typeKeys[m.typeIdx] + "  "))
		sb.WriteString(m.scope.View() + "\n")
		sb.WriteString("\n" + style.Dimmed.Render(i18n.T("commit.hint_next")))

	case stepSubject:
		sb.WriteString(style.Label.Render(i18n.T("commit.enter_subject")) + "\n\n")
		prefix := m.typeKeys[m.typeIdx]
		if m.scope.Value() != "" {
			prefix += "(" + m.scope.Value() + ")"
		}
		sb.WriteString(style.Normal.Render(prefix + ": "))
		sb.WriteString(m.subject.View() + "\n")
		label, warn := subjectLenHint(utf8.RuneCountInString(m.subject.Value()))
		if warn {
			sb.WriteString(style.Failure.Render(label+" "+i18n.Tf("commit.over_limit", subjectRecommendedLen)) + "\n")
		} else {
			sb.WriteString(style.Dimmed.Render(label) + "\n")
		}
		sb.WriteString("\n" + style.Dimmed.Render(i18n.T("commit.hint_next")))

	case stepBody:
		sb.WriteString(style.Label.Render(i18n.T("commit.enter_body")) + "\n\n")
		sb.WriteString(m.body.View() + "\n")
		sb.WriteString("\n" + style.Dimmed.Render(i18n.T("commit.hint_next")))

	case stepConfirm:
		sb.WriteString(style.Label.Render(i18n.T("commit.confirm_msg")) + "\n\n")
		sb.WriteString(style.Border.Render(m.buildMessage()) + "\n\n")
		sb.WriteString(style.Label.Render(i18n.T("commit.commit_q")))
		sb.WriteString(style.Selected.Render(" y ") + style.Normal.Render(" "+i18n.T("commit.yes")+"   "))
		sb.WriteString(style.Selected.Render(" n ") + style.Normal.Render(" "+i18n.T("commit.no")+"   "))
		sb.WriteString(style.Selected.Render(" a ") + style.Normal.Render(" "+i18n.T("commit.amend")+"   "))
		sb.WriteString(style.Selected.Render(" e ") + style.Normal.Render(" "+i18n.T("commit.edit")+"\n"))
		sb.WriteString("\n" + style.Dimmed.Render(i18n.T("commit.hint_back_body")))
		if last := git.GetLastCommitSubject(); last != "" {
			sb.WriteString("\n" + style.Dimmed.Render(i18n.T("commit.amend_prev")+last) + "\n")
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
		fmt.Println(i18n.T("commit.nothing"))
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
		fmt.Fprintln(os.Stderr, style.Failure.Render(i18n.T("commit.failed")+final.err.Error()))
		os.Exit(1)
	}
	if final.done {
		if final.amend {
			fmt.Println(style.Success.Render("✓ " + i18n.T("commit.amended")))
		} else {
			fmt.Println(style.Success.Render("✓ " + i18n.T("commit.committed")))
		}
		fmt.Println(style.Dimmed.Render(final.buildMessage()))
	}
}
