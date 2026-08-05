package ui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"gito/internal/i18n"
	"gito/internal/style"
)

// MenuItem describes one selectable command in the launcher.
type MenuItem struct {
	Key     string // subcommand name, e.g. "commit"
	descKey string // i18n message key for the localized description
}

// Desc returns the command's description in the active UI language.
func (m MenuItem) Desc() string { return i18n.T(m.descKey) }

// Icon returns the launcher glyph for the command. It is read from the active
// glyph table on every render instead of being frozen into MenuItems, so a
// non-UTF-8 terminal gets the one-column ASCII stand-in rather than a
// replacement box.
func (m MenuItem) Icon() string {
	switch m.Key {
	case "status":
		return style.G.IconStatus
	case "commit":
		return style.G.IconCommit
	case "log":
		return style.G.IconLog
	case "branch":
		return style.G.IconBranch
	case "diff":
		return style.G.IconDiff
	case "stash":
		return style.G.IconStash
	case "tag":
		return style.G.IconTag
	case "remote":
		return style.G.IconRemote
	case "reflog":
		return style.G.IconReflog
	case "blame":
		return style.G.IconBlame
	}
	return style.G.Crumb
}

// MenuItems is the canonical list of commands shown in the launcher and help.
// main.go reuses this for its dispatch table and text help so they never drift,
// which is why the entries, their order and their keys are fixed.
var MenuItems = []MenuItem{
	{"status", "menu.status"},
	{"commit", "menu.commit"},
	{"log", "menu.log"},
	{"branch", "menu.branch"},
	{"diff", "menu.diff"},
	{"stash", "menu.stash"},
	{"tag", "menu.tag"},
	{"remote", "menu.remote"},
	{"reflog", "menu.reflog"},
	{"blame", "menu.blame"},
}

// ── model ─────────────────────────────────────────────────────────────────────

type menuModel struct {
	cursor int
	offset int // first visible row, so a short terminal still shows the cursor
	chosen string
	quit   bool
	lay    layout
}

func (m menuModel) Init() tea.Cmd { return nil }

// ── Update ───────────────────────────────────────────────────────────────────

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.lay = m.lay.resize(msg.Width, msg.Height)
		m.offset = m.window().Offset
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.quit = true
			return m, tea.Quit
		case "up", "k", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j", "ctrl+n":
			if m.cursor < len(MenuItems)-1 {
				m.cursor++
			}
		case "g":
			m.cursor = 0
		case "G":
			m.cursor = len(MenuItems) - 1
		case "enter", "l", "right":
			m.chosen = MenuItems[m.cursor].Key
			return m, tea.Quit
		}
		// number shortcuts 1-9 for quick selection, plus '0' for the 10th item.
		if len(msg.String()) == 1 {
			c := msg.String()[0]
			if c >= '1' && c <= '9' {
				idx := int(c - '1')
				if idx < len(MenuItems) {
					m.chosen = MenuItems[idx].Key
					return m, tea.Quit
				}
			} else if c == '0' {
				if len(MenuItems) >= 10 {
					m.chosen = MenuItems[9].Key
					return m, tea.Quit
				}
			}
		}
		m.offset = m.window().Offset
	}
	return m, nil
}

// listRows is how many launcher entries fit under the header.
func (m menuModel) listRows() int { return bodyRows(m.lay, len(m.head())+1) }

// window is the scrolling state of the launcher list.
func (m menuModel) window() listWindow {
	return listWindow{
		Cursor: m.cursor,
		Offset: m.offset,
		Total:  len(MenuItems),
		Rows:   m.listRows(),
	}.clamp()
}

// ── hints ────────────────────────────────────────────────────────────────────

func menuHints() []keyHint {
	return []keyHint{
		moveHint(),
		{Keys: "1-9,0", Desc: i18n.T("key.quick_select")},
		{Keys: "enter", Desc: i18n.T("key.run")},
		quitHint(),
	}
}

// ── View ─────────────────────────────────────────────────────────────────────

// head is the launcher's title block.
func (m menuModel) head() []string {
	l := m.lay.norm()
	return []string{
		header(l, "", "", listWindow{Cursor: m.cursor, Total: len(MenuItems), Rows: 1}.position()),
		style.Truncate(style.MetaDim.Render(i18n.T("menu.prompt")), l.Width),
		"",
	}
}

// menuKeyWidth is the column the command names are padded to, so the
// descriptions line up.
func menuKeyWidth() int {
	w := 0
	for _, item := range MenuItems {
		w = max(w, style.DisplayWidth(item.Key))
	}
	return w
}

func (m menuModel) View() string {
	l := m.lay.norm()
	head := m.head()
	foot := footer(l, menuHints(), false)

	keyW := menuKeyWidth()
	w := m.window()
	start, end := w.bounds()

	var lines []string
	for i := start; i < end; i++ {
		item := MenuItems[i]
		// The shortcut of the tenth entry is '0', matching the key handler.
		badge := style.KeyCap.Render(fmt.Sprintf("%d", (i+1)%10))
		content := badge + " " + item.Icon() + " " +
			style.Pad(style.Subject.Render(item.Key), keyW) + "  " +
			style.MetaDim.Render(item.Desc())
		lines = append(lines, row(l, i == w.Cursor, content))
	}

	// The launcher runs without alt screen and prints its result to stdout, so
	// it must never pad to the terminal height.
	return frameInlineFit(l, head, lines, foot)
}

// ── RunMenu ──────────────────────────────────────────────────────────────────

// RunMenu shows the interactive launcher and returns the chosen subcommand key,
// or "" if the user quit without choosing.
func RunMenu() string {
	result, err := tea.NewProgram(menuModel{lay: newLayout()}).Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	final, ok := result.(menuModel)
	if !ok || final.quit {
		return ""
	}
	return final.chosen
}
