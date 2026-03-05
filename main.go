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
	default:
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println(`gito - TUI git helper

Usage:
  gito commit    Interactive commit wizard (5-step)
  gito log       Scrollable git log viewer
  gito branch    Fuzzy branch switcher`)
}
