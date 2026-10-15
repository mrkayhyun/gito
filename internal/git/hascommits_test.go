package git

import (
	"os"
	"testing"
)

// A freshly initialized repo has an unborn HEAD: git log / git reflog exit 128
// there. gito should report "no commits" instead of an opaque exit status.
func TestLogAndReflogOnUnbornRepo(t *testing.T) {
	dir := t.TempDir()
	run(t, dir, "init", "-q")
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()

	if HasCommits() {
		t.Fatal("HasCommits() = true on an unborn repo")
	}
	if entries, err := GetLogEntries(10); err != nil || len(entries) != 0 {
		t.Errorf("GetLogEntries on unborn repo = %v, %v; want empty, nil", entries, err)
	}
	if entries, err := GetReflog(10); err != nil || len(entries) != 0 {
		t.Errorf("GetReflog on unborn repo = %v, %v; want empty, nil", entries, err)
	}
}

func TestHasCommitsAfterFirstCommit(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()
	if !HasCommits() {
		t.Fatal("HasCommits() = false after a commit")
	}
	entries, err := GetLogEntries(10)
	if err != nil || len(entries) != 1 {
		t.Fatalf("GetLogEntries = %v, %v; want 1 entry", entries, err)
	}
}
