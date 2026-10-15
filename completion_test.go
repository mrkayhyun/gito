package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrkayhyun/gito/internal/ui"
)

func TestCompletionListsEveryCommand(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		var buf bytes.Buffer
		if err := writeCompletion(&buf, []string{shell}); err != nil {
			t.Fatalf("%s: %v", shell, err)
		}
		for _, item := range ui.MenuItems {
			if !strings.Contains(buf.String(), item.Key) {
				t.Errorf("%s completion is missing command %q", shell, item.Key)
			}
		}
	}
}

func TestCompletionUsageErrors(t *testing.T) {
	for _, args := range [][]string{nil, {"powershell"}, {"bash", "zsh"}} {
		if err := writeCompletion(&bytes.Buffer{}, args); err == nil {
			t.Errorf("writeCompletion(%v) should fail", args)
		}
	}
}

func TestShellSingleQuote(t *testing.T) {
	if got := shellSingleQuote("it's"); got != `'it'\''s'` {
		t.Errorf("shellSingleQuote = %s", got)
	}
}

// Each generated script must at least parse in its shell. Shells that are not
// installed are skipped (CI has bash; zsh/fish are checked when present).
func TestCompletionScriptsParse(t *testing.T) {
	for shell, flag := range map[string]string{"bash": "-n", "zsh": "-n", "fish": "--no-execute"} {
		path, err := exec.LookPath(shell)
		if err != nil {
			t.Logf("%s not installed; skipping", shell)
			continue
		}
		var buf bytes.Buffer
		if err := writeCompletion(&buf, []string{shell}); err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(t.TempDir(), "gito."+shell)
		if err := os.WriteFile(file, buf.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command(path, flag, file).CombinedOutput(); err != nil {
			t.Errorf("%s rejected its completion script: %v\n%s", shell, err, out)
		}
	}
}
