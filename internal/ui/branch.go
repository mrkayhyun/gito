package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"gito/internal/git"
	"gito/internal/i18n"
	"gito/internal/style"
)

// ── modes ─────────────────────────────────────────────────────────────────────

type branchMode int

const (
	branchModeList branchMode = iota
	branchModeCreate
	branchModeRename
)

// ── model ─────────────────────────────────────────────────────────────────────

type branchModel struct {
	branches     []string
	current      string
	filter       textinput.Model
	input        textinput.Model // used for create / rename
	mode         branchMode
	cursor       int
	offset       int // first visible row of the filtered list
	err          error
	done         bool
	switched     string
	msg          string // status message (success/info)
	confirm      bool   // delete confirmation active
	confirmForce bool
	lay          layout
}

func newBranchModel(branches []string, current string) branchModel {
	filter := textinput.New()
	filter.Placeholder = i18n.T("branch.ph_filter")
	filter.CharLimit = 100
	filter.Focus()

	input := textinput.New()
	input.CharLimit = 100

	m := branchModel{
		branches: branches,
		current:  current,
		filter:   filter,
		input:    input,
		lay:      newLayout(),
	}
	m.fitFormWidth()
	return m
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

// ── Init ─────────────────────────────────────────────────────────────────────

func (m branchModel) Init() tea.Cmd {
	return textinput.Blink
}

// ── Update ───────────────────────────────────────────────────────────────────

func (m branchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.lay = m.lay.resize(size.Width, size.Height)
		m.fitFormWidth()
		m.offset = m.window().Offset
		return m, nil
	}

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

// listRows is how many branches fit under the header, filter and banners.
func (m branchModel) listRows() int { return bodyRows(m.lay, len(m.listHead())+1) }

// window is the scrolling state of the filtered branch list.
func (m branchModel) window() listWindow {
	return listWindow{
		Cursor: m.cursor,
		Offset: m.offset,
		Total:  len(m.filteredBranches()),
		Rows:   m.listRows(),
	}.clamp()
}

// fitFormWidth keeps the filter and the create/rename field inside the
// terminal, so a narrow window cannot make them wrap.
func (m *branchModel) fitFormWidth() {
	label := style.DisplayWidth(i18n.T("common.search"))
	m.filter.Width = max(m.lay.norm().Width-label-6, 10)
	m.input.Width = max(m.lay.norm().Width-6, 10)
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
			m.err = fmt.Errorf("%s", i18n.T("branch.err_name_required"))
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
			m.msg = i18n.T("branch.renamed_to") + name
		} else {
			m.msg = i18n.T("branch.created_switched") + name
		}
		m.mode = branchModeList
		m.input.Blur()
		branches, current, gerr := git.GetBranches()
		if gerr == nil {
			m.branches, m.current = branches, current
		}
		m.cursor, m.offset = 0, 0
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m branchModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	filtered := m.filteredBranches()

	// delete confirmation. Kept first and unchanged: any key that is not 'y'
	// cancels, so nothing below can intercept a confirmation.
	if m.confirm {
		switch msg.String() {
		case "y", "Y":
			m.confirm = false
			if m.cursor < len(filtered) {
				target := filtered[m.cursor]
				if err := git.DeleteBranch(target, m.confirmForce); err != nil {
					m.err = err
				} else {
					m.msg = i18n.T("branch.deleted") + target
					branches, current, gerr := git.GetBranches()
					if gerr == nil {
						m.branches, m.current = branches, current
					}
					if m.cursor > 0 {
						m.cursor--
					}
				}
			}
			m.offset = m.window().Offset
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
		m.offset = m.window().Offset
		return m, nil
	case "down", "ctrl+n":
		if m.cursor < len(filtered)-1 {
			m.cursor++
		}
		m.offset = m.window().Offset
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
		m.input.Placeholder = i18n.T("branch.ph_new")
		return m, m.input.Focus()
	case "ctrl+r": // rename selected
		if m.cursor < len(filtered) {
			m.mode = branchModeRename
			m.err = nil
			m.msg = ""
			m.input.SetValue(filtered[m.cursor])
			m.input.Placeholder = i18n.T("branch.ph_newname")
			return m, m.input.Focus()
		}
		return m, nil
	case "ctrl+d": // delete (safe)
		if m.cursor < len(filtered) {
			target := filtered[m.cursor]
			if target == m.current {
				m.err = fmt.Errorf("%s", i18n.T("branch.err_cur"))
				return m, nil
			}
			if git.IsRemoteBranch(target) {
				m.err = fmt.Errorf("%s", i18n.T("branch.err_remote"))
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
				m.err = fmt.Errorf("%s", i18n.T("branch.err_cur"))
				return m, nil
			}
			if git.IsRemoteBranch(target) {
				m.err = fmt.Errorf("%s", i18n.T("branch.err_remote"))
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
		m.cursor, m.offset = 0, 0
	}
	return m, cmd
}

// ── hints ────────────────────────────────────────────────────────────────────

// branchListHints is rendered in the footer rather than behind a '?' overlay:
// the filter textinput consumes printable runes, so '?' is not free here. This
// is the hint line that used to be one ~100-column i18n sentence.
func branchListHints() []keyHint {
	return []keyHint{
		{Keys: "enter", Desc: i18n.T("key.switch")},
		{Keys: "^b", Desc: i18n.T("key.create")},
		{Keys: "^r", Desc: i18n.T("key.rename")},
		{Keys: "^d", Desc: i18n.T("key.delete")},
		{Keys: "^x", Desc: i18n.T("key.force_delete")},
		arrowMoveHint(),
		escQuitHint(),
	}
}

func branchInputHints() []keyHint {
	return []keyHint{
		{Keys: "enter", Desc: i18n.T("key.confirm")},
		{Keys: "esc", Desc: i18n.T("key.cancel")},
	}
}

// ── View ─────────────────────────────────────────────────────────────────────

func (m branchModel) View() string {
	if m.mode == branchModeCreate || m.mode == branchModeRename {
		return m.viewInput()
	}
	return m.viewList()
}

// position reports "cursor/total" without depending on the visible row count.
func (m branchModel) position() string {
	return listWindow{Cursor: m.cursor, Total: len(m.filteredBranches()), Rows: 1}.position()
}

func (m branchModel) viewInput() string {
	l := m.lay.norm()

	crumb, label := i18n.T("key.create"), i18n.T("branch.new_name")
	if m.mode == branchModeRename {
		crumb, label = i18n.T("key.rename"), i18n.T("branch.rename_to")
	}
	head := []string{header(l, "branch", crumb, ""), ""}
	foot := footer(l, branchInputHints(), false)

	body := []string{
		style.Truncate(style.Label.Render(label), l.Width),
		"",
		style.Truncate(m.input.View(), l.Width),
	}
	if m.err != nil {
		body = append(body, "", banner(l, bannerError, m.err.Error()))
	}
	return frameInlineFit(l, head, body, foot)
}

// listHead is every line above the branch list: header, filter field, the
// destructive-delete confirmation bar and the live banners.
func (m branchModel) listHead() []string {
	l := m.lay.norm()
	filtered := m.filteredBranches()

	meta := i18n.Tf("meta.branches", len(filtered))
	if pos := m.position(); pos != "" {
		meta += "  " + pos
	}
	search := style.Label.Render(i18n.T("common.search")) + m.filter.View()
	lines := []string{
		header(l, "branch", "", meta),
		"",
		style.Truncate(search, l.Width),
		"",
	}

	if m.confirm && m.cursor < len(filtered) {
		key := "branch.delete_confirm"
		if m.confirmForce {
			key = "branch.force_delete_confirm"
		}
		lines = append(lines, splitLines(confirmBar(l, i18n.Tf(key, filtered[m.cursor])))...)
		lines = append(lines, "")
	}
	if m.err != nil {
		lines = append(lines, banner(l, bannerError, m.err.Error()), "")
	}
	if m.msg != "" {
		lines = append(lines, banner(l, bannerSuccess, m.msg), "")
	}
	return lines
}

func (m branchModel) viewList() string {
	l := m.lay.norm()
	head := m.listHead()
	foot := footer(l, branchListHints(), false)

	filtered := m.filteredBranches()
	if len(filtered) == 0 {
		body := style.Truncate(style.MetaDim.Render(i18n.T("branch.no_branches")), l.Width)
		return frameInlineFit(l, head, []string{body}, foot)
	}

	w := m.window()
	start, end := w.bounds()
	var lines []string
	for i := start; i < end; i++ {
		lines = append(lines, row(l, i == w.Cursor, branchLine(filtered[i], m.current)))
	}
	return frameInlineFit(l, head, lines, foot)
}

// branchLine renders one branch, marking the checked-out one with a leading
// star so the current branch is recognizable even on the selected row.
func branchLine(name, current string) string {
	if name == current {
		return style.Staged.Render("* " + name)
	}
	return "  " + style.Subject.Render(name)
}

// ── RunBranch ────────────────────────────────────────────────────────────────

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
		fmt.Fprintln(os.Stderr, style.Failure.Render(i18n.T("branch.switch_failed")+final.err.Error()))
		os.Exit(1)
	}
	if final.done {
		fmt.Println(style.Success.Render(style.G.Check + " " + i18n.T("branch.switched") + final.switched))
	}
}
