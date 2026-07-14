package ui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"gito/internal/style"
)

// MenuItem describes one selectable command in the launcher.
type MenuItem struct {
	Key  string // subcommand name, e.g. "commit"
	Icon string
	Desc string
}

// MenuItems is the canonical list of commands shown in the launcher and help.
// main.go reuses this for its dispatch table and text help so they never drift.
var MenuItems = []MenuItem{
	{"status", "◍", "변경 파일 스테이징 / diff / discard"},
	{"commit", "✎", "대화형 커밋 마법사 (5단계)"},
	{"log", "≡", "커밋 로그 탐색 + 상세 diff"},
	{"branch", "⑂", "브랜치 전환 / 생성 / 이름변경 / 삭제"},
	{"diff", "⇄", "두 ref(브랜치·태그) 비교"},
	{"stash", "⊟", "스태시 pop / apply / diff / drop"},
	{"tag", "⌂", "태그 생성 / 삭제 / push"},
	{"remote", "☁", "원격 목록 / fetch / ahead-behind"},
	{"reflog", "↺", "reflog 탐색 + 커밋 복구"},
	{"blame", "◎", "파일 라인별 blame"},
	{"rebase", "⤳", "안전한 대화형 리베이스 (정리 / 재정렬)"},
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
		// number shortcuts 1-9 for quick selection
		if len(key.String()) == 1 {
			c := key.String()[0]
			if c >= '1' && c <= '9' {
				idx := int(c - '1')
				if idx < len(MenuItems) {
					m.chosen = MenuItems[idx].Key
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
	sb.WriteString(style.Dimmed.Render("실행할 명령을 선택하세요") + "\n\n")

	for i, item := range MenuItems {
		num := style.Dimmed.Render(fmt.Sprintf("%d", i+1))
		line := fmt.Sprintf("%s %-7s %s", item.Icon, item.Key, item.Desc)
		if i == m.cursor {
			sb.WriteString(num + " " + style.Selected.Render("▶ "+line) + "\n")
		} else {
			sb.WriteString(num + " " + style.Normal.Render("  "+line) + "\n")
		}
	}

	sb.WriteString("\n" + style.Dimmed.Render(
		"↑/↓ j/k: 이동   1-9: 바로 선택   enter: 실행   q/esc: 종료",
	))
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
