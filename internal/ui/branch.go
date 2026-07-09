package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"gito/internal/git"
	"gito/internal/style"
)

type branchMode int

const (
	branchModeList branchMode = iota
	branchModeCreate
	branchModeRename
)

type branchModel struct {
	branches     []string
	current      string
	filter       textinput.Model
	input        textinput.Model // used for create / rename
	mode         branchMode
	cursor       int
	err          error
	done         bool
	switched     string
	msg          string // status message (success/info)
	confirm      bool   // delete confirmation active
	confirmForce bool
}

func newBranchModel(branches []string, current string) branchModel {
	filter := textinput.New()
	filter.Placeholder = "filter branches..."
	filter.CharLimit = 100
	filter.Focus()

	input := textinput.New()
	input.CharLimit = 100

	return branchModel{
		branches: branches,
		current:  current,
		filter:   filter,
		input:    input,
	}
}

func (m branchModel) filteredBranches() []string {
	query := strings.ToLower(m.filter.Value())
	if query == "" {
		return m.branches
	}
	var result []string
	for _, b := range m.branches {
		if strings.Contains(strings.ToLower(b), query) {
			result = append(result, b)
		}
	}
	return result
}

func (m branchModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m branchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, isKey := msg.(tea.KeyMsg)
	if !isKey {
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		return m, cmd
	}

	switch m.mode {
	case branchModeCreate:
		return m.updateInput(keyMsg, false)
	case branchModeRename:
		return m.updateInput(keyMsg, true)
	default:
		return m.updateList(keyMsg)
	}
}

func (m branchModel) updateInput(msg tea.KeyMsg, rename bool) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.mode = branchModeList
		m.input.Blur()
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.input.Value())
		if name == "" {
			m.err = fmt.Errorf("branch name is required")
			return m, nil
		}
		var err error
		if rename {
			filtered := m.filteredBranches()
			if m.cursor < len(filtered) {
				err = git.RenameBranch(filtered[m.cursor], name)
			}
		} else {
			err = git.CreateBranch(name, true) // create + checkout
		}
		if err != nil {
			m.err = err
			return m, nil
		}
		if rename {
			m.msg = "Renamed to " + name
		} else {
			m.msg = "Created & switched to " + name
		}
		m.mode = branchModeList
		m.input.Blur()
		branches, current, gerr := git.GetBranches()
		if gerr == nil {
			m.branches, m.current = branches, current
		}
		m.cursor = 0
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m branchModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	filtered := m.filteredBranches()

	// delete confirmation
	if m.confirm {
		switch msg.String() {
		case "y", "Y":
			m.confirm = false
			if m.cursor < len(filtered) {
				target := filtered[m.cursor]
				if err := git.DeleteBranch(target, m.confirmForce); err != nil {
					m.err = err
				} else {
					m.msg = "Deleted " + target
					branches, current, gerr := git.GetBranches()
					if gerr == nil {
						m.branches, m.current = branches, current
					}
					if m.cursor > 0 {
						m.cursor--
					}
				}
			}
			return m, nil
		default:
			m.confirm = false
			return m, nil
		}
	}

	switch msg.String() {
	case "ctrl+c", "esc":
		return m, tea.Quit
	case "up", "ctrl+p":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "down", "ctrl+n":
		if m.cursor < len(filtered)-1 {
			m.cursor++
		}
		return m, nil
	case "enter":
		if len(filtered) > 0 {
			selected := filtered[m.cursor]
			if selected == m.current {
				return m, tea.Quit
			}
			err := git.SwitchBranch(selected)
			m.err = err
			if err == nil {
				m.switched = selected
				m.done = true
			}
			return m, tea.Quit
		}
		return m, nil
	case "ctrl+b": // create new branch
		m.mode = branchModeCreate
		m.err = nil
		m.msg = ""
		m.input.SetValue("")
		m.input.Placeholder = "new branch name"
		return m, m.input.Focus()
	case "ctrl+r": // rename selected
		if m.cursor < len(filtered) {
			m.mode = branchModeRename
			m.err = nil
			m.msg = ""
			m.input.SetValue(filtered[m.cursor])
			m.input.Placeholder = "new name"
			return m, m.input.Focus()
		}
		return m, nil
	case "ctrl+d": // delete (safe)
		if m.cursor < len(filtered) {
			target := filtered[m.cursor]
			if target == m.current {
				m.err = fmt.Errorf("cannot delete the current branch")
				return m, nil
			}
			if git.IsRemoteBranch(target) {
				m.err = fmt.Errorf("cannot delete a remote-tracking branch here")
				return m, nil
			}
			m.err = nil
			m.confirm = true
			m.confirmForce = false
		}
		return m, nil
	case "ctrl+x": // force delete
		if m.cursor < len(filtered) {
			target := filtered[m.cursor]
			if target == m.current {
				m.err = fmt.Errorf("cannot delete the current branch")
				return m, nil
			}
			if git.IsRemoteBranch(target) {
				m.err = fmt.Errorf("cannot delete a remote-tracking branch here")
				return m, nil
			}
			m.err = nil
			m.confirm = true
			m.confirmForce = true
		}
		return m, nil
	}

	prevFilter := m.filter.Value()
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	if m.filter.Value() != prevFilter {
		m.cursor = 0
	}
	return m, cmd
}

func (m branchModel) View() string {
	if m.mode == branchModeCreate || m.mode == branchModeRename {
		return m.viewInput()
	}
	return m.viewList()
}

func (m branchModel) viewInput() string {
	var sb strings.Builder
	title := "gito branch  ›  create"
	label := "새 브랜치 이름 (생성 후 전환):"
	if m.mode == branchModeRename {
		title = "gito branch  ›  rename"
		label = "새 이름으로 변경:"
	}
	sb.WriteString(style.Title.Render(title) + "\n\n")
	sb.WriteString(style.Label.Render(label) + "\n\n")
	sb.WriteString(m.input.View() + "\n")
	if m.err != nil {
		sb.WriteString("\n" + style.Failure.Render("! "+m.err.Error()) + "\n")
	}
	sb.WriteString("\n" + style.Dimmed.Render("enter: 확인   esc: 취소"))
	return sb.String()
}

func (m branchModel) viewList() string {
	var sb strings.Builder

	sb.WriteString(style.Title.Render("gito branch") + "\n\n")
	sb.WriteString(style.Label.Render("Search: ") + m.filter.View() + "\n\n")

	filtered := m.filteredBranches()

	if m.confirm && m.cursor < len(filtered) {
		verb := "삭제"
		if m.confirmForce {
			verb = "강제 삭제(병합 안 된 커밋 포함)"
		}
		sb.WriteString(style.Failure.Render(
			verb+"하시겠습니까? "+filtered[m.cursor],
		) + "\n")
		sb.WriteString(style.Label.Render("y: 확인   다른 키: 취소") + "\n\n")
	}
	if m.err != nil {
		sb.WriteString(style.Failure.Render("! "+m.err.Error()) + "\n\n")
	}
	if m.msg != "" {
		sb.WriteString(style.Success.Render("✓ "+m.msg) + "\n\n")
	}

	if len(filtered) == 0 {
		sb.WriteString(style.Dimmed.Render("  No branches found") + "\n")
	} else {
		for i, b := range filtered {
			prefix := "  "
			if b == m.current {
				prefix = "* "
			}
			if i == m.cursor {
				sb.WriteString(style.Selected.Render(prefix+b) + "\n")
			} else if b == m.current {
				sb.WriteString(style.Success.Render(prefix+b) + "\n")
			} else {
				sb.WriteString(style.Normal.Render(prefix+b) + "\n")
			}
		}
	}

	sb.WriteString("\n" + style.Dimmed.Render(
		"↑/↓: 이동   enter: 전환   ^b: 생성   ^r: 이름변경   ^d: 삭제   ^x: 강제삭제   esc: 종료",
	))
	return sb.String()
}

func RunBranch() {
	branches, current, err := git.GetBranches()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	m := newBranchModel(branches, current)
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	final := result.(branchModel)
	if final.err != nil && final.done {
		fmt.Fprintln(os.Stderr, style.Failure.Render("Switch failed: "+final.err.Error()))
		os.Exit(1)
	}
	if final.done {
		fmt.Println(style.Success.Render("✓ Switched to: " + final.switched))
	}
}
