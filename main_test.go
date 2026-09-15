package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/mrkayhyun/gito/internal/ui"
)

// TestIsKnownMatchesMenuItems pins the command-recognition gate: every command
// shown in the launcher must be accepted by isKnown, and an unrecognized name
// must be rejected. isKnown is the guard that decides whether dispatch proceeds
// or exits 1 with an "unknown command" error, so a regression here would either
// break a real command or let an unknown token through the front door.
func TestIsKnownMatchesMenuItems(t *testing.T) {
	for _, item := range ui.MenuItems {
		if !isKnown(item.Key) {
			t.Errorf("isKnown(%q) = false, but it is a MenuItems command", item.Key)
		}
	}
	for _, bogus := range []string{"", "nope", "COMMIT", "commit ", "--commit", "rm-rf"} {
		if isKnown(bogus) {
			t.Errorf("isKnown(%q) = true, expected false for an unknown command", bogus)
		}
	}
}

// TestDispatchCoversEveryMenuItem enforces the invariant documented on MenuItems
// in internal/ui/menu.go: "main.go reuses this for its dispatch table ... so they
// never drift." isKnown() gates on MenuItems, but dispatch() has a SEPARATE,
// hand-maintained switch. If a command is added to MenuItems without a matching
// dispatch case, isKnown returns true yet the command silently does nothing.
// This parses main.go's dispatch() switch and asserts every MenuItems key has a
// case, catching that drift at test time instead of at a user's terminal.
func TestDispatchCoversEveryMenuItem(t *testing.T) {
	cases := dispatchSwitchCases(t)
	for _, item := range ui.MenuItems {
		if !cases[item.Key] {
			t.Errorf("dispatch() has no case for MenuItems command %q — it would be a silent no-op", item.Key)
		}
	}
}

// dispatchSwitchCases parses main.go and returns the set of string case values
// in the command switch inside dispatch(). It keys on the switch whose tag is the
// identifier `cmd` (dispatch's parameter), so it ignores the os.Args[1] switch in
// main().
func dispatchSwitchCases(t *testing.T) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}
	found := map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		sw, ok := n.(*ast.SwitchStmt)
		if !ok {
			return true
		}
		ident, ok := sw.Tag.(*ast.Ident)
		if !ok || ident.Name != "cmd" {
			return true
		}
		for _, stmt := range sw.Body.List {
			cc, ok := stmt.(*ast.CaseClause)
			if !ok {
				continue
			}
			for _, expr := range cc.List {
				if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					found[strings.Trim(lit.Value, `"`)] = true
				}
			}
		}
		return true
	})
	if len(found) == 0 {
		t.Fatal("no `switch cmd` cases found in main.go — parser assumption broke")
	}
	return found
}

// TestPrintHelpListsEveryCommand ensures the text help stays complete: every
// launcher command must appear in `gito help` output. Help is the CLI's primary
// discoverability surface, so a command missing from it is effectively hidden.
func TestPrintHelpListsEveryCommand(t *testing.T) {
	out := captureStdout(t, printHelp)
	for _, item := range ui.MenuItems {
		if !strings.Contains(out, "gito "+item.Key) {
			t.Errorf("printHelp() output does not list command %q", item.Key)
		}
	}
}

// captureStdout runs fn with os.Stdout redirected to a pipe and returns what it wrote.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				sb.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
		done <- sb.String()
	}()
	fn()
	_ = w.Close()
	os.Stdout = orig
	return <-done
}
