package git

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestGetCherryPickCandidates(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	// Create a feature branch with extra commits.
	run(t, ".", "checkout", "-b", "feature")
	addCommit(t, "feat1.txt", "f1\n", "feat: add f1")
	addCommit(t, "feat2.txt", "f2\n", "feat: add f2")

	// Switch back to the original branch.
	run(t, ".", "checkout", "-")

	// Get candidates from feature.
	commits, err := GetCherryPickCandidates("feature", 50)
	if err != nil {
		t.Fatalf("GetCherryPickCandidates: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 candidates, got %d: %+v", len(commits), commits)
	}
	// Oldest first.
	if commits[0].Subject != "feat: add f1" {
		t.Errorf("expected first candidate 'feat: add f1', got %q", commits[0].Subject)
	}
	if commits[1].Subject != "feat: add f2" {
		t.Errorf("expected second candidate 'feat: add f2', got %q", commits[1].Subject)
	}
	// Fields should be populated.
	for _, c := range commits {
		if c.Hash == "" || c.Short == "" || c.Author == "" || c.Date == "" {
			t.Errorf("incomplete commit fields: %+v", c)
		}
	}
}

func TestGetCherryPickCandidatesNone(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	// Create a branch at the same point (no divergence).
	run(t, ".", "checkout", "-b", "same-point")
	run(t, ".", "checkout", "-")

	commits, err := GetCherryPickCandidates("same-point", 50)
	if err != nil {
		t.Fatalf("GetCherryPickCandidates: %v", err)
	}
	if len(commits) != 0 {
		t.Errorf("expected 0 candidates when branches are at same point, got %d", len(commits))
	}
}

func TestGetCherryPickCandidatesInvalidRef(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	_, err := GetCherryPickCandidates("-invalid", 50)
	if err == nil {
		t.Errorf("expected error for invalid ref name")
	}
}

func TestRunCherryPick(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	// Create a feature branch.
	run(t, ".", "checkout", "-b", "feature")
	addCommit(t, "feat1.txt", "f1\n", "feat: add f1")
	addCommit(t, "feat2.txt", "f2\n", "feat: add f2")

	// Get the hashes.
	out, err := exec.Command("git", "log", "-2", "--pretty=format:%H").Output()
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	// lines[0] is feat2, lines[1] is feat1 (newest first)
	hash1 := lines[1]
	hash2 := lines[0]

	// Switch back to main branch.
	run(t, ".", "checkout", "-")

	// Cherry-pick both commits.
	if err := RunCherryPick([]string{hash1, hash2}); err != nil {
		t.Fatalf("RunCherryPick: %v", err)
	}

	// Verify files exist.
	if _, err := os.Stat("feat1.txt"); err != nil {
		t.Errorf("feat1.txt should exist after cherry-pick: %v", err)
	}
	if _, err := os.Stat("feat2.txt"); err != nil {
		t.Errorf("feat2.txt should exist after cherry-pick: %v", err)
	}
}

func TestRunCherryPickConflictAutoAbort(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	// Make a change on main that will conflict.
	addCommit(t, "conflict.txt", "main content\n", "main: add conflict.txt")

	// Create a feature branch from the initial commit and make a conflicting change.
	run(t, ".", "checkout", "-b", "feature", "HEAD~1")
	addCommit(t, "conflict.txt", "feature content\n", "feat: add conflict.txt")

	// Get the feature commit hash.
	out, err := exec.Command("git", "log", "-1", "--pretty=format:%H").Output()
	if err != nil {
		t.Fatal(err)
	}
	featHash := strings.TrimSpace(string(out))

	// Switch back to main.
	run(t, ".", "checkout", "-")

	origHead, _ := revParse(t, "HEAD")

	// Cherry-pick should fail and auto-abort.
	err = RunCherryPick([]string{featHash})
	if err == nil {
		t.Fatalf("expected error from conflicting cherry-pick")
	}
	if !strings.Contains(err.Error(), "auto-aborted") {
		t.Errorf("error should mention auto-abort, got: %v", err)
	}

	// HEAD should be unchanged.
	afterHead, _ := revParse(t, "HEAD")
	if origHead != afterHead {
		t.Errorf("HEAD changed after auto-abort: %s -> %s", origHead, afterHead)
	}
}

func TestRunCherryPickEmpty(t *testing.T) {
	if err := RunCherryPick(nil); err == nil {
		t.Errorf("expected error for empty hashes")
	}
}

func TestRunCherryPickDirtyGuard(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	// Create a dirty file.
	if err := os.WriteFile("README.md", []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := RunCherryPick([]string{"abc123"})
	if err == nil {
		t.Errorf("expected error on dirty working tree")
	}
	if !strings.Contains(err.Error(), "commit or stash") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetLocalBranches(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	run(t, ".", "checkout", "-b", "dev")
	run(t, ".", "checkout", "-b", "staging")

	branches, current, err := GetLocalBranches()
	if err != nil {
		t.Fatalf("GetLocalBranches: %v", err)
	}
	if current != "staging" {
		t.Errorf("expected current=staging, got %q", current)
	}
	if !contains(branches, "dev") {
		t.Errorf("expected 'dev' in branches: %v", branches)
	}
	if !contains(branches, "staging") {
		t.Errorf("expected 'staging' in branches: %v", branches)
	}
}
