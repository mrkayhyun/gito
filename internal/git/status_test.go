package git

import (
	"os"
	"testing"
)

// TestFileStatusPredicates pins the byte-level porcelain column semantics that
// the TUI relies on to classify each file as staged / unstaged / untracked.
// These predicates are pure logic (no git invocation), so a regression here is
// a silent misclassification, not a crash — exactly what a table test guards.
func TestFileStatusPredicates(t *testing.T) {
	cases := []struct {
		name                              string
		staged, unstaged                  byte
		wantStaged, wantUnstaged, wantUnt bool
	}{
		{"clean", ' ', ' ', false, false, false},
		{"modified-staged", 'M', ' ', true, false, false},
		{"modified-unstaged", ' ', 'M', false, true, false},
		{"modified-both", 'M', 'M', true, true, false},
		{"added-staged", 'A', ' ', true, false, false},
		{"deleted-unstaged", ' ', 'D', false, true, false},
		{"renamed-staged", 'R', ' ', true, false, false},
		{"untracked", '?', '?', false, false, true},
		// A single '?' in only one column is not the untracked state; git only
		// ever emits "??" for untracked, so the predicate must require both.
		{"question-staged-only", '?', ' ', false, false, false},
		{"question-unstaged-only", ' ', '?', false, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := FileStatus{Staged: tc.staged, Unstaged: tc.unstaged}
			if got := f.IsStaged(); got != tc.wantStaged {
				t.Errorf("IsStaged() = %v, want %v", got, tc.wantStaged)
			}
			if got := f.IsUnstaged(); got != tc.wantUnstaged {
				t.Errorf("IsUnstaged() = %v, want %v", got, tc.wantUnstaged)
			}
			if got := f.IsUntracked(); got != tc.wantUnt {
				t.Errorf("IsUntracked() = %v, want %v", got, tc.wantUnt)
			}
		})
	}
}

// TestGetFileStatuses exercises the porcelain parser end-to-end against a real
// repo: a staged add, an unstaged modification, and an untracked file must each
// be parsed into the correct FileStatus. This is the parser the whole status UI
// is built on.
func TestGetFileStatuses(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	// README.md already committed by setupRepo. Modify it (unstaged),
	// stage a new file, and leave a third file untracked.
	if err := os.WriteFile("README.md", []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("staged.txt", []byte("s\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("untracked.txt", []byte("u\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, ".", "add", "staged.txt")

	files, err := GetFileStatuses()
	if err != nil {
		t.Fatalf("GetFileStatuses: %v", err)
	}

	byPath := make(map[string]FileStatus, len(files))
	for _, f := range files {
		byPath[f.Path] = f
	}

	if f, ok := byPath["staged.txt"]; !ok {
		t.Error("staged.txt missing from statuses")
	} else if !f.IsStaged() || f.IsUnstaged() {
		t.Errorf("staged.txt: IsStaged=%v IsUnstaged=%v, want true/false", f.IsStaged(), f.IsUnstaged())
	}

	if f, ok := byPath["README.md"]; !ok {
		t.Error("README.md missing from statuses")
	} else if !f.IsUnstaged() {
		t.Errorf("README.md: IsUnstaged=%v, want true", f.IsUnstaged())
	}

	if f, ok := byPath["untracked.txt"]; !ok {
		t.Error("untracked.txt missing from statuses")
	} else if !f.IsUntracked() {
		t.Errorf("untracked.txt: IsUntracked=%v, want true", f.IsUntracked())
	}
}

// TestGetFileStatusesRename pins the "old -> new" rename split, which populates
// OldPath — a parsing branch with its own SplitN logic that nothing else covers.
func TestGetFileStatusesRename(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	run(t, ".", "mv", "README.md", "READ.md")

	files, err := GetFileStatuses()
	if err != nil {
		t.Fatalf("GetFileStatuses: %v", err)
	}

	var found bool
	for _, f := range files {
		if f.Path == "READ.md" {
			found = true
			if f.OldPath != "README.md" {
				t.Errorf("rename OldPath = %q, want %q", f.OldPath, "README.md")
			}
			if !f.IsStaged() {
				t.Errorf("staged rename: IsStaged=%v, want true", f.IsStaged())
			}
		}
	}
	if !found {
		t.Errorf("renamed file READ.md not found in statuses: %+v", files)
	}
}

// TestGetStatusAndStageAll covers GetStatus (short format) and StageAll, which
// the commit flow depends on. A clean tree yields empty; after a change and
// StageAll, the file shows staged.
func TestGetStatusAndStageAll(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	s, err := GetStatus()
	if err != nil {
		t.Fatalf("GetStatus (clean): %v", err)
	}
	if s != "" {
		t.Errorf("clean tree GetStatus = %q, want empty", s)
	}

	if err := os.WriteFile("new.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := StageAll(); err != nil {
		t.Fatalf("StageAll: %v", err)
	}

	files, err := GetFileStatuses()
	if err != nil {
		t.Fatalf("GetFileStatuses: %v", err)
	}
	var staged bool
	for _, f := range files {
		if f.Path == "new.txt" && f.IsStaged() {
			staged = true
		}
	}
	if !staged {
		t.Error("StageAll did not stage new.txt")
	}
}

// TestGetFileDiff covers both the unstaged and staged (--cached) diff paths.
func TestGetFileDiff(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	if err := os.WriteFile("README.md", []byte("line one\nline two\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	unstaged, err := GetFileDiff("README.md", false)
	if err != nil {
		t.Fatalf("GetFileDiff unstaged: %v", err)
	}
	if unstaged == "" {
		t.Error("expected non-empty unstaged diff for modified README.md")
	}

	run(t, ".", "add", "README.md")

	staged, err := GetFileDiff("README.md", true)
	if err != nil {
		t.Fatalf("GetFileDiff staged: %v", err)
	}
	if staged == "" {
		t.Error("expected non-empty staged diff after add")
	}
}

// TestGetStashes covers the stash-list parser, including the "WIP on <branch>:"
// prefix stripping that populates Branch.
func TestGetStashes(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	// No stashes initially.
	stashes, err := GetStashes()
	if err != nil {
		t.Fatalf("GetStashes (empty): %v", err)
	}
	if len(stashes) != 0 {
		t.Fatalf("expected 0 stashes, got %d", len(stashes))
	}

	// Create a change and stash it.
	if err := os.WriteFile("README.md", []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, ".", "stash", "push", "-m", "work in progress")

	stashes, err = GetStashes()
	if err != nil {
		t.Fatalf("GetStashes: %v", err)
	}
	if len(stashes) != 1 {
		t.Fatalf("expected 1 stash, got %d: %+v", len(stashes), stashes)
	}
	if stashes[0].Ref != "stash@{0}" {
		t.Errorf("stash Ref = %q, want stash@{0}", stashes[0].Ref)
	}
	// Branch is parsed from the "WIP on <branch>:" reflog subject.
	if stashes[0].Branch == "" {
		t.Errorf("stash Branch not parsed: %+v", stashes[0])
	}
}
