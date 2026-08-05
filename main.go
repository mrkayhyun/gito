package main

import (
	"fmt"
	"os"
	"strings"

	"gito/internal/config"
	"gito/internal/git"
	"gito/internal/i18n"
	"gito/internal/style"
	"gito/internal/ui"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	// Resolve the UI language before any output: first from the environment
	// (auto-detection), then let an optional gito.json "lang" field override it.
	i18n.Init()
	if cfg := config.Load(); cfg.Lang != "" {
		i18n.SetLang(i18n.Parse(cfg.Lang))
	}

	// No arguments → launch the interactive command picker.
	if len(os.Args) < 2 {
		launchMenu()
		return
	}

	switch os.Args[1] {
	case "help", "-h", "--help":
		printHelp()
	case "version", "--version", "-v":
		fmt.Printf("gito %s\n", version)
	case "menu":
		launchMenu()
	default:
		dispatch(os.Args[1])
	}
}

// launchMenu shows the interactive picker and runs whatever it returns.
func launchMenu() {
	style.Detect()
	if cmd := ui.RunMenu(); cmd != "" {
		dispatch(cmd)
	}
}

// dispatch runs a subcommand by name. All subcommands require a git repository,
// so it verifies that first and prints a friendly message otherwise.
func dispatch(cmd string) {
	if !isKnown(cmd) {
		fmt.Fprintln(os.Stderr, style.Failure.Render(i18n.T("err.unknown_command")+cmd)+"\n")
		printHelp()
		os.Exit(1)
	}

	if !git.IsRepo() {
		fmt.Fprintln(os.Stderr, style.Failure.Render(i18n.T("err.not_a_repo")))
		fmt.Fprintln(os.Stderr, style.Dimmed.Render(i18n.T("err.not_a_repo_hint")))
		os.Exit(1)
	}

	// Resolve the color profile and the light/dark background before the screen
	// starts, while gito still owns the terminal in cooked mode: the adaptive
	// colors would otherwise trigger that detection during the first render,
	// from inside a Bubble Tea program that has taken stdin over. It is only
	// done on the paths that actually draw a UI, so `gito version` and `gito
	// help` still talk to nothing but their own output stream.
	style.Detect()

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
	sb.WriteString("  gito            " + i18n.T("help.run_menu") + "\n")
	sb.WriteString("  gito <command>  " + i18n.T("help.run_command") + "\n\n")
	sb.WriteString("Commands:\n")
	for _, item := range ui.MenuItems {
		sb.WriteString(fmt.Sprintf("  gito %-8s %s\n", item.Key, item.Desc()))
	}
	sb.WriteString("\nOther:\n")
	sb.WriteString("  gito menu       " + i18n.T("help.menu") + "\n")
	sb.WriteString("  gito version    " + i18n.T("help.version") + "\n")
	sb.WriteString("  gito help       " + i18n.T("help.help") + "\n\n")
	sb.WriteString("Config (optional):\n")
	sb.WriteString("  ./gito.json  or  ~/.config/gito/config.json\n")
	sb.WriteString("  { \"lang\": \"en\", \"commit_types\": [ {\"key\": \"feat\", \"label\": \"feat  ...\"} ] }\n")
	fmt.Print(sb.String())
}
