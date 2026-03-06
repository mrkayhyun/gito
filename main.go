package main

import (
	"fmt"
	"os"

	"gito/internal/ui"
)

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(0)
	}

	switch os.Args[1] {
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
	default:
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println(`gito - TUI git helper

Usage:
  gito commit    Interactive commit wizard (5-step, config-driven types)
  gito log       Scrollable log viewer  (↑/↓ navigate, enter: detail)
  gito branch    Fuzzy branch switcher
  gito status    Interactive stage / unstage / diff / discard
  gito stash     Stash list  (pop / apply / show diff / drop)

Config (optional):
  ./gito.json  or  ~/.config/gito/config.json
  {
    "commit_types": [
      {"key": "feat", "label": "feat   새 기능"},
      {"key": "fix",  "label": "fix    버그 수정"}
    ]
  }`)
}
