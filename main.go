package main

import (
	"fmt"
	"os"
	"strings"

	"gito/internal/git"
	"gito/internal/style"
	"gito/internal/ui"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	// No arguments → launch the interactive command picker.
	if len(os.Args) < 2 {
		cmd := ui.RunMenu()
		if cmd == "" {
			return // user quit the menu
		}
		dispatch(cmd)
		return
	}

	switch os.Args[1] {
	case "help", "-h", "--help":
		printHelp()
	case "version", "--version", "-v":
		fmt.Printf("gito %s\n", version)
	case "menu":
		if cmd := ui.RunMenu(); cmd != "" {
			dispatch(cmd)
		}
	default:
		dispatch(os.Args[1])
	}
}

// dispatch runs a subcommand by name. All subcommands require a git repository,
// so it verifies that first and prints a friendly message otherwise.
func dispatch(cmd string) {
	if !isKnown(cmd) {
		fmt.Fprintln(os.Stderr, style.Failure.Render("알 수 없는 명령: "+cmd)+"\n")
		printHelp()
		os.Exit(1)
	}

	if !git.IsRepo() {
		fmt.Fprintln(os.Stderr, style.Failure.Render("여기는 git 저장소가 아닙니다."))
		fmt.Fprintln(os.Stderr, style.Dimmed.Render("git 저장소 안에서 실행하거나 'git init'으로 새로 만드세요."))
		os.Exit(1)
	}

	switch cmd {
	case "commit":
		ui.RunCommit()
	case "log":
		ui.RunLog()
	case "branch":
		ui.RunBranch()
	case "status":
		ui.RunStatus()
	case "stash":
		ui.RunStash()
	case "tag":
		ui.RunTag()
	case "remote":
		ui.RunRemote()
	case "diff":
		ui.RunDiff()
	case "reflog":
		ui.RunReflog()
	case "blame":
		ui.RunBlame()
	case "rebase":
		ui.RunRebase()
	case "cherry-pick":
		ui.RunCherryPick()
	case "undo":
		ui.RunUndo()
	case "worktree":
		ui.RunWorktree()
	}
}

func isKnown(cmd string) bool {
	for _, item := range ui.MenuItems {
		if item.Key == cmd {
			return true
		}
	}
	return false
}

func printHelp() {
	var sb strings.Builder
	sb.WriteString("gito - TUI git helper\n\n")
	sb.WriteString("Usage:\n")
	sb.WriteString("  gito            대화형 런처 메뉴 실행\n")
	sb.WriteString("  gito <command>  아래 명령 중 하나를 바로 실행\n\n")
	sb.WriteString("Commands:\n")
	for _, item := range ui.MenuItems {
		sb.WriteString(fmt.Sprintf("  gito %-8s %s\n", item.Key, item.Desc))
	}
	sb.WriteString("\nOther:\n")
	sb.WriteString("  gito menu       런처 메뉴 열기\n")
	sb.WriteString("  gito version    버전 출력\n")
	sb.WriteString("  gito help       이 도움말 출력\n\n")
	sb.WriteString("Config (optional):\n")
	sb.WriteString("  ./gito.json  or  ~/.config/gito/config.json\n")
	sb.WriteString("  { \"commit_types\": [ {\"key\": \"feat\", \"label\": \"feat  새 기능\"} ] }\n")
	fmt.Print(sb.String())
}
