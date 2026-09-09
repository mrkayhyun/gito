package git

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestGetUndoInfoAfterCommit(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	// After the initial commit, HEAD@{1} doesn't exist, so no undo.
	info, err := GetUndoInfo()
	if err != nil {
		t.Fatalf("GetUndoInfo: %v", err)
	}
	if info != nil {
		t.Fatalf("expected nil info for initial commit, got %+v", info)
	}

	// Make a second commit.
	addCommit(t, "a.txt", "a\n", "add a")

	info, err = GetUndoInfo()
	if err != nil {
		t.Fatalf("GetUndoInfo: %v", err)
	}
	if info == nil {
		t.Fatalf("expected non-nil info after second commit")
	}
	if info.CurrentSubject != "add a" {
		t.Errorf("CurrentSubject = %q, want %q", info.CurrentSubject, "add a")
	}
	if info.PreviousSubject != "initial commit" {
		t.Errorf("PreviousSubject = %q, want %q", info.PreviousSubject, "initial commit")
	}
	if info.Action == "" {
		t.Errorf("Action should not be empty")
	}
	if info.CurrentHash == "" || info.PreviousHash == "" {
		t.Errorf("hashes should not be empty: %+v", info)
	}
}

func TestRunUndoSoft(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	addCommit(t, "a.txt", "a\n", "add a")
	addCommit(t, "b.txt", "b\n", "add b")

	// Before undo: HEAD is "add b".
	headBefore, _ := exec.Command("git", "log", "-1", "--pretty=format:%s").Output()
	if strings.TrimSpace(string(headBefore)) != "add b" {
		t.Fatalf("precondition: HEAD subject = %q", string(headBefore))
	}

	if err := RunUndo(); err != nil {
		t.Fatalf("RunUndo: %v", err)
	}

	// After undo: HEAD should be "add a".
	headAfter, _ := exec.Command("git", "log", "-1", "--pretty=format:%s").Output()
	if strings.TrimSpace(string(headAfter)) != "add a" {
		t.Errorf("after undo, HEAD subject = %q, want %q", string(headAfter), "add a")
	}

	// With --soft, b.txt should still exist in the working tree as a staged change.
	if _, err := os.Stat("b.txt"); err != nil {
		t.Errorf("b.txt should still exist after soft undo: %v", err)
	}
}

func TestRunUndoHard(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	addCommit(t, "a.txt", "a\n", "add a")
	addCommit(t, "b.txt", "b\n", "add b")

	if err := RunUndoHard(); err != nil {
		t.Fatalf("RunUndoHard: %v", err)
	}

	headAfter, _ := exec.Command("git", "log", "-1", "--pretty=format:%s").Output()
	if strings.TrimSpace(string(headAfter)) != "add a" {
		t.Errorf("after hard undo, HEAD subject = %q, want %q", string(headAfter), "add a")
	}

	// With --hard, b.txt should be gone from the working tree.
	if _, err := os.Stat("b.txt"); !os.IsNotExist(err) {
		t.Errorf("b.txt should NOT exist after hard undo, stat err = %v", err)
	}
}

func TestRunUndoDirtyGuard(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	addCommit(t, "a.txt", "a\n", "add a")
	addCommit(t, "b.txt", "b\n", "add b")

	// Make an unstaged modification.
	if err := os.WriteFile("a.txt", []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := RunUndo()
	if err == nil {
		t.Fatalf("expected error on dirty working tree")
	}
	if !strings.Contains(err.Error(), "unstaged changes") {
		t.Errorf("unexpected error: %v", err)
	}
}
