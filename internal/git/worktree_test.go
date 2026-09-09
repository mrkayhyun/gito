package git

import (
	"testing"
)

func TestGetWorktrees(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	trees, err := GetWorktrees()
	if err != nil {
		t.Fatalf("GetWorktrees: %v", err)
	}
	// At minimum, the main worktree should exist.
	if len(trees) < 1 {
		t.Fatalf("expected at least 1 worktree, got %d", len(trees))
	}
	// The first entry should have a non-empty path and branch.
	if trees[0].Path == "" {
		t.Errorf("first worktree has empty path")
	}
	if trees[0].Head == "" {
		t.Errorf("first worktree has empty HEAD")
	}
}

func TestAddAndRemoveWorktree(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	// Add a worktree with a new branch.
	wtPath := t.TempDir() + "/wt-test"
	if err := AddWorktree(wtPath, "wt-branch", true); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}

	trees, err := GetWorktrees()
	if err != nil {
		t.Fatalf("GetWorktrees: %v", err)
	}
	found := false
	for _, wt := range trees {
		if wt.Branch == "wt-branch" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("wt-branch worktree not found in list: %+v", trees)
	}

	// Remove the worktree.
	if err := RemoveWorktree(wtPath, false); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}

	trees, err = GetWorktrees()
	if err != nil {
		t.Fatalf("GetWorktrees after remove: %v", err)
	}
	for _, wt := range trees {
		if wt.Branch == "wt-branch" {
			t.Errorf("wt-branch worktree should be removed but is still present")
		}
	}
}

func TestAddWorktreeInvalidBranch(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	err := AddWorktree("/tmp/wt-bad", "-invalid", true)
	if err == nil {
		t.Errorf("expected error for invalid branch name")
	}
}
