package ui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"gito/internal/i18n"
	"gito/internal/style"
)

// MenuItem describes one selectable command in the launcher.
type MenuItem struct {
	Key     string // subcommand name, e.g. "commit"
	Icon    string
	descKey string // i18n message key for the localized description
}

// Desc returns the command's description in the active UI language.
func (m MenuItem) Desc() string { return i18n.T(m.descKey) }

// MenuItems is the canonical list of commands shown in the launcher and help.
// main.go reuses this for its dispatch table and text help so they never drift.
var MenuItems = []MenuItem{
	{"status", "◍", "menu.status"},
	{"commit", "✎", "menu.commit"},
	{"log", "≡", "menu.log"},
	{"branch", "⑂", "menu.branch"},
	{"diff", "⇄", "menu.diff"},
	{"stash", "⊟", "menu.stash"},
	{"tag", "⌂", "menu.tag"},
	{"remote", "☁", "menu.remote"},
	{"reflog", "↺", "menu.reflog"},
	{"blame", "◎", "menu.blame"},
}

type menuModel struct {
	cursor int
	chosen string
	quit   bool
}

func (m menuModel) Init() tea.Cmd { return nil }

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
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
		if len(key.String()) == 1 {
			c := key.String()[0]
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
	}
	return m, nil
}

func (m menuModel) View() string {
	var sb strings.Builder
	sb.WriteString(style.Title.Render("gito") + style.Dimmed.Render("  TUI git helper") + "\n")
	sb.WriteString(style.Dimmed.Render(i18n.T("menu.prompt")) + "\n\n")

	for i, item := range MenuItems {
		num := style.Dimmed.Render(fmt.Sprintf("%d", i+1))
		line := fmt.Sprintf("%s %-7s %s", item.Icon, item.Key, item.Desc())
		if i == m.cursor {
			sb.WriteString(num + " " + style.Selected.Render("▶ "+line) + "\n")
		} else {
			sb.WriteString(num + " " + style.Normal.Render("  "+line) + "\n")
		}
	}

	sb.WriteString("\n" + style.Dimmed.Render(i18n.T("menu.hint")))
	return sb.String()
}

// RunMenu shows the interactive launcher and returns the chosen subcommand key,
// or "" if the user quit without choosing.
func RunMenu() string {
	result, err := tea.NewProgram(menuModel{}).Run()
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
