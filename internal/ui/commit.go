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
	"gito/internal/i18n"
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

// ── steps ─────────────────────────────────────────────────────────────────────

type commitStep int

const (
	stepType commitStep = iota
	stepScope
	stepSubject
	stepBody
	stepConfirm
	stepDone
)

// ── model ─────────────────────────────────────────────────────────────────────

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
	lay        layout
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

	m := commitModel{
		step:       stepType,
		typeKeys:   keys,
		typeLabels: labels,
		scope:      scope,
		subject:    subject,
		body:       body,
		lay:        newLayout(),
	}
	m.fitFormWidth()
	return m
}

// commitType is the selected type key, guarded so a model built without a
// config (or before a type is picked) never indexes out of range.
func (m commitModel) commitType() string {
	if m.typeIdx >= 0 && m.typeIdx < len(m.typeKeys) {
		return m.typeKeys[m.typeIdx]
	}
	return ""
}

func (m commitModel) buildMessage() string {
	t := m.commitType()
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

// fitFormWidth keeps the wizard's fields inside the terminal.
func (m *commitModel) fitFormWidth() {
	w := max(m.lay.norm().Width-20, 10)
	m.scope.Width = w
	m.subject.Width = w
	m.body.Width = max(m.lay.norm().Width-6, 10)
}

// ── Init ─────────────────────────────────────────────────────────────────────

func (m commitModel) Init() tea.Cmd {
	return nil
}

// ── Update ───────────────────────────────────────────────────────────────────

func (m commitModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.lay = m.lay.resize(msg.Width, msg.Height)
		m.fitFormWidth()
		return m, nil

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

// ── hints ────────────────────────────────────────────────────────────────────

func (m commitModel) stepHints() []keyHint {
	switch m.step {
	case stepType:
		return []keyHint{
			moveHint(),
			{Keys: "enter", Desc: i18n.T("key.select")},
			{Keys: "q", Desc: i18n.T("key.quit")},
		}
	case stepConfirm:
		return []keyHint{
			{Keys: "esc", Desc: i18n.T("key.back")},
			{Keys: "^c", Desc: i18n.T("key.quit")},
		}
	default:
		return []keyHint{
			{Keys: "enter", Desc: i18n.T("key.next")},
			{Keys: "esc", Desc: i18n.T("key.back")},
			{Keys: "^c", Desc: i18n.T("key.quit")},
		}
	}
}

// ── View ─────────────────────────────────────────────────────────────────────

// commitStepNames are the localized wizard step labels, in order.
func commitStepNames() []string {
	return []string{
		i18n.T("commit.step_type"),
		i18n.T("commit.step_scope"),
		i18n.T("commit.step_subject"),
		i18n.T("commit.step_body"),
		i18n.T("commit.step_confirm"),
	}
}

// commitProgress renders the wizard's progress: numbered steps with the done
// ones marked by the Check glyph, the current one badged and the upcoming ones
// dimmed. When the full row does not fit it degrades to the compact "3/5
// Subject" form instead of wrapping off the terminal.
func commitProgress(l layout, step commitStep) string {
	l = l.norm()
	names := commitStepNames()
	cur := min(max(int(step), 0), len(names)-1)

	parts := make([]string, 0, len(names))
	for i, name := range names {
		num := fmt.Sprintf("%d", i+1)
		switch {
		case i < cur:
			parts = append(parts, style.Success.Render(style.G.Check+num+" "+name))
		case i == cur:
			parts = append(parts, style.Badge.Render(" "+num+" "+name+" "))
		default:
			parts = append(parts, style.MetaDim.Render(num+" "+name))
		}
	}

	full := strings.Join(parts, style.MetaDim.Render(" "+style.G.Arrow+" "))
	if style.DisplayWidth(full) <= l.Width {
		return full
	}
	compact := style.KeyCap.Render(fmt.Sprintf("%d/%d", cur+1, len(names))) +
		" " + style.Subject.Render(names[cur])
	return style.Truncate(compact, l.Width)
}

// composedMessage is the message built so far, one line per line, each cut to w
// display columns.
func (m commitModel) composedMessage(w int) []string {
	var out []string
	for _, ln := range strings.Split(m.buildMessage(), "\n") {
		out = append(out, style.Truncate(ln, w))
	}
	return out
}

// previewBox shows the message composed so far inside the bordered box, from
// the scope step onward, so the result is visible while it is being written
// instead of only on the confirmation screen.
func (m commitModel) previewBox(l layout) []string {
	if m.step < stepScope || len(m.typeKeys) == 0 {
		return nil
	}
	// 2 border columns plus the box's 1-column padding on each side.
	inner := max(l.norm().Width-4, 10)
	return splitLines(style.Box().Render(strings.Join(m.composedMessage(inner), "\n")))
}

// stepBody renders the fields of the current step.
func (m commitModel) stepBody(l layout) []string {
	switch m.step {
	case stepType:
		lines := []string{
			style.Truncate(style.Label.Render(i18n.T("commit.select_type")), l.Width),
			"",
		}
		// head (4) + label + blank + footer come off the terminal height.
		w := listWindow{Cursor: m.cursor, Total: len(m.typeLabels), Rows: bodyRows(l, 7)}.clamp()
		rl := listLayout(l, w)
		start, end := w.bounds()
		var rows []string
		for i := start; i < end; i++ {
			rows = append(rows, row(rl, i == w.Cursor, style.Subject.Render(m.typeLabels[i])))
		}
		return append(lines, splitLines(listBody(l, w, rows))...)

	case stepScope:
		return []string{
			style.Truncate(style.Label.Render(i18n.T("commit.enter_scope")), l.Width),
			"",
			style.Truncate(style.Hash.Render(m.commitType())+"  "+m.scope.View(), l.Width),
		}

	case stepSubject:
		prefix := m.commitType()
		if m.scope.Value() != "" {
			prefix += "(" + m.scope.Value() + ")"
		}
		label, warn := subjectLenHint(utf8.RuneCountInString(m.subject.Value()))
		counter := style.MetaDim.Render(label)
		if warn {
			counter = style.Failure.Render(label + " " + i18n.Tf("commit.over_limit", subjectRecommendedLen))
		}
		return []string{
			style.Truncate(style.Label.Render(i18n.T("commit.enter_subject")), l.Width),
			"",
			style.Truncate(style.Hash.Render(prefix+": ")+m.subject.View(), l.Width),
			style.Truncate(counter, l.Width),
		}

	case stepBody:
		return []string{
			style.Truncate(style.Label.Render(i18n.T("commit.enter_body")), l.Width),
			"",
			style.Truncate(m.body.View(), l.Width),
		}

	case stepConfirm:
		// The choices are the one thing this step cannot lose, so they are
		// budgeted first: the bordered box degrades to a plain message line, and
		// the "previous commit" note is dropped, before they are touched.
		choices := []keyHint{
			{Keys: "y", Desc: i18n.T("commit.yes")},
			{Keys: "n", Desc: i18n.T("commit.no")},
			{Keys: "a", Desc: i18n.T("commit.amend")},
			{Keys: "e", Desc: i18n.T("commit.edit")},
		}
		rendered := make([]string, 0, len(choices))
		for _, c := range choices {
			rendered = append(rendered, renderHint(c))
		}
		tail := []string{
			style.Truncate(style.Label.Render(i18n.T("commit.commit_q")), l.Width),
			style.Truncate(strings.Join(rendered, hintSep), l.Width),
		}

		// The body may use the terminal minus the header block and the footer.
		avail := bodyRows(l, len(commitHeadLines(l, m.step))+1)
		message := m.previewBox(l)
		if 1+len(message)+len(tail)+2 > avail {
			message = m.composedMessage(l.Width)
		}
		if last := git.GetLastCommitSubject(); last != "" &&
			1+len(message)+len(tail)+3 <= avail {
			tail = append(tail, style.Truncate(style.MetaDim.Render(i18n.T("commit.amend_prev")+last), l.Width))
		}

		lines := []string{style.Truncate(style.Label.Render(i18n.T("commit.confirm_msg")), l.Width), ""}
		lines = append(lines, message...)
		lines = append(lines, "")
		return append(lines, tail...)
	}
	return nil
}

// commitHeadLines is the wizard's title block: header, blank, progress row,
// blank.
func commitHeadLines(l layout, step commitStep) []string {
	names := commitStepNames()
	crumb := ""
	if int(step) < len(names) {
		crumb = names[step]
	}
	return []string{
		header(l, "commit", crumb, ""),
		"",
		commitProgress(l, step),
		"",
	}
}

func (m commitModel) View() string {
	l := m.lay.norm()

	head := commitHeadLines(l, m.step)
	foot := footer(l, m.stepHints(), false)
	body := m.stepBody(l)

	// The live preview is the first thing sacrificed on a short terminal: the
	// field being edited matters more than the summary of it. The confirmation
	// step builds its own review box, which is why it is excluded here.
	if m.step < stepConfirm {
		if preview := m.previewBox(l); len(preview) > 0 {
			if len(head)+len(body)+len(preview)+2 <= l.Height {
				body = append(body, "")
				body = append(body, preview...)
			}
		}
	}

	// The wizard runs without alt screen and prints its result to stdout, so it
	// must never pad to the terminal height.
	return frameInlineFit(l, head, body, foot)
}

// ── RunCommit ────────────────────────────────────────────────────────────────

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
			fmt.Println(style.Success.Render(style.G.Check + " " + i18n.T("commit.amended")))
		} else {
			fmt.Println(style.Success.Render(style.G.Check + " " + i18n.T("commit.committed")))
		}
		fmt.Println(style.MetaDim.Render(final.buildMessage()))
	}
}
