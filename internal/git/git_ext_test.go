package git

import (
	"os"
	"strings"
	"testing"
)

func TestBranchLifecycle(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	// create without checkout
	if err := CreateBranch("feature-a", false); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	branches, current, err := GetBranches()
	if err != nil {
		t.Fatalf("GetBranches: %v", err)
	}
	if !contains(branches, "feature-a") {
		t.Fatalf("feature-a not found in %v", branches)
	}
	// still on the original branch
	if current == "feature-a" {
		t.Errorf("should not have switched when checkout=false")
	}

	// create with checkout
	if err := CreateBranch("feature-b", true); err != nil {
		t.Fatalf("CreateBranch checkout: %v", err)
	}
	_, current, _ = GetBranches()
	if current != "feature-b" {
		t.Errorf("expected current branch feature-b, got %q", current)
	}

	// rename feature-a -> feature-a2
	if err := RenameBranch("feature-a", "feature-a2"); err != nil {
		t.Fatalf("RenameBranch: %v", err)
	}
	branches, _, _ = GetBranches()
	if contains(branches, "feature-a") || !contains(branches, "feature-a2") {
		t.Errorf("rename failed: %v", branches)
	}

	// delete feature-a2 (safe delete of a branch with no unique commits)
	if err := DeleteBranch("feature-a2", false); err != nil {
		t.Fatalf("DeleteBranch: %v", err)
	}
	branches, _, _ = GetBranches()
	if contains(branches, "feature-a2") {
		t.Errorf("feature-a2 should be deleted: %v", branches)
	}
}

func TestCommitAmend(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	if got := GetLastCommitSubject(); got != "initial commit" {
		t.Fatalf("GetLastCommitSubject = %q, want %q", got, "initial commit")
	}

	if err := CommitAmend("amended message"); err != nil {
		t.Fatalf("CommitAmend: %v", err)
	}
	if got := GetLastCommitSubject(); got != "amended message" {
		t.Errorf("after amend subject = %q, want %q", got, "amended message")
	}

	// ensure history still has exactly one commit (amend replaces, not adds)
	entries, err := GetLogEntries(10)
	if err != nil {
		t.Fatalf("GetLogEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 commit after amend, got %d", len(entries))
	}
}

func TestGetRefs(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	if err := CreateBranch("dev", false); err != nil {
		t.Fatal(err)
	}
	if err := CreateTag("v1.0.0", "", ""); err != nil {
		t.Fatal(err)
	}
	refs, err := GetRefs()
	if err != nil {
		t.Fatalf("GetRefs: %v", err)
	}
	if !contains(refs, "dev") {
		t.Errorf("branch 'dev' missing from refs: %v", refs)
	}
	if !contains(refs, "v1.0.0") {
		t.Errorf("tag 'v1.0.0' missing from refs: %v", refs)
	}
}

func TestGetDiffBetween(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	run(t, ".", "checkout", "-b", "feature")
	if err := os.WriteFile("README.md", []byte("hello world\nsecond line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, ".", "commit", "-aqm", "extend readme")

	// determine the base branch name (main or master)
	_, base, _ := GetBranches()
	// current is feature; find the other branch
	branches, _, _ := GetBranches()
	baseRef := ""
	for _, b := range branches {
		if b != "feature" {
			baseRef = b
			break
		}
	}
	if baseRef == "" {
		baseRef = base
	}

	out, err := GetDiffBetween(baseRef, "feature")
	if err != nil {
		t.Fatalf("GetDiffBetween: %v", err)
	}
	if !strings.Contains(out, "second line") {
		t.Errorf("diff should mention the added line, got:\n%s", out)
	}
}

func TestGetReflogAndRecover(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	entries, err := GetReflog(50)
	if err != nil {
		t.Fatalf("GetReflog: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected at least one reflog entry")
	}
	if entries[0].Selector == "" || entries[0].Short == "" {
		t.Errorf("reflog entry missing fields: %+v", entries[0])
	}

	// recover the tip into a new branch
	if err := CreateBranchAt("recovered", entries[0].Selector); err != nil {
		t.Fatalf("CreateBranchAt: %v", err)
	}
	branches, _, _ := GetBranches()
	if !contains(branches, "recovered") {
		t.Errorf("recovered branch not created: %v", branches)
	}
}

func TestListTrackedFilesAndBlame(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	files, err := ListTrackedFiles()
	if err != nil {
		t.Fatalf("ListTrackedFiles: %v", err)
	}
	if !contains(files, "README.md") {
		t.Fatalf("README.md not tracked: %v", files)
	}

	out, err := GetBlame("README.md")
	if err != nil {
		t.Fatalf("GetBlame: %v", err)
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("blame output should contain file content, got:\n%s", out)
	}
}

func TestGetRemotes(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	// no remotes yet
	remotes, err := GetRemotes()
	if err != nil {
		t.Fatalf("GetRemotes: %v", err)
	}
	if len(remotes) != 0 {
		t.Fatalf("expected 0 remotes, got %d", len(remotes))
	}

	run(t, ".", "remote", "add", "origin", "https://example.com/repo.git")
	remotes, err = GetRemotes()
	if err != nil {
		t.Fatalf("GetRemotes: %v", err)
	}
	if len(remotes) != 1 || remotes[0].Name != "origin" {
		t.Fatalf("expected origin remote, got %+v", remotes)
	}
	if remotes[0].FetchURL != "https://example.com/repo.git" {
		t.Errorf("unexpected fetch URL: %q", remotes[0].FetchURL)
	}

	// no upstream configured -> hasUpstream false
	_, _, has := GetAheadBehind()
	if has {
		t.Errorf("expected no upstream for a fresh repo")
	}
}

func TestIsRepo(t *testing.T) {
	// inside a repo
	cleanup := setupRepo(t)
	if !IsRepo() {
		cleanup()
		t.Fatalf("IsRepo should be true inside a git repo")
	}
	cleanup()

	// outside any repo: use a fresh temp dir that is not a git repo
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)
	if IsRepo() {
		t.Errorf("IsRepo should be false outside a git repo (%s)", dir)
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
