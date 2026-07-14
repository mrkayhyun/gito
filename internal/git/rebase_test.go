package git

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// ── BuildRebaseTodo unit tests ─────────────────────────────────────────────────

func TestBuildRebaseTodoPickOnly(t *testing.T) {
	steps := []RebaseStep{
		{Hash: "aaaa", Action: ActionPick},
		{Hash: "bbbb", Action: ActionPick},
	}
	got, err := BuildRebaseTodo(steps, make([]string, len(steps)))
	if err != nil {
		t.Fatalf("BuildRebaseTodo: %v", err)
	}
	want := "pick aaaa\npick bbbb\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildRebaseTodoSquashFixup(t *testing.T) {
	steps := []RebaseStep{
		{Hash: "aaaa", Action: ActionPick},
		{Hash: "bbbb", Action: ActionSquash},
		{Hash: "cccc", Action: ActionFixup},
	}
	got, err := BuildRebaseTodo(steps, make([]string, len(steps)))
	if err != nil {
		t.Fatalf("BuildRebaseTodo: %v", err)
	}
	want := "pick aaaa\nsquash bbbb\nfixup cccc\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildRebaseTodoReword(t *testing.T) {
	path := "/tmp/gito-reword-123.txt"
	steps := []RebaseStep{
		{Hash: "aaaa", Action: ActionReword, NewMessage: "new subject"},
	}
	got, err := BuildRebaseTodo(steps, []string{path})
	if err != nil {
		t.Fatalf("BuildRebaseTodo: %v", err)
	}
	want := "pick aaaa\nexec git commit --amend -F '/tmp/gito-reword-123.txt'\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if !strings.Contains(got, path) {
		t.Errorf("exec line should reference the message path %q, got %q", path, got)
	}
}

func TestBuildRebaseTodoDrop(t *testing.T) {
	steps := []RebaseStep{
		{Hash: "aaaa", Action: ActionPick},
		{Hash: "bbbb", Action: ActionDrop},
	}
	got, err := BuildRebaseTodo(steps, make([]string, len(steps)))
	if err != nil {
		t.Fatalf("BuildRebaseTodo: %v", err)
	}
	want := "pick aaaa\ndrop bbbb\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildRebaseTodoErrFirstSquash(t *testing.T) {
	steps := []RebaseStep{
		{Hash: "aaaa", Action: ActionSquash},
		{Hash: "bbbb", Action: ActionPick},
	}
	if _, err := BuildRebaseTodo(steps, make([]string, len(steps))); err == nil {
		t.Errorf("expected error when first kept step is squash")
	}
}

func TestBuildRebaseTodoErrAllDrop(t *testing.T) {
	steps := []RebaseStep{
		{Hash: "aaaa", Action: ActionDrop},
		{Hash: "bbbb", Action: ActionDrop},
	}
	if _, err := BuildRebaseTodo(steps, make([]string, len(steps))); err == nil {
		t.Errorf("expected error when all steps drop")
	}
}

func TestBuildRebaseTodoErrRewordMissing(t *testing.T) {
	// missing path
	steps := []RebaseStep{{Hash: "aaaa", Action: ActionReword, NewMessage: "x"}}
	if _, err := BuildRebaseTodo(steps, []string{""}); err == nil {
		t.Errorf("expected error when reword message path is empty")
	}
	// missing message
	steps = []RebaseStep{{Hash: "aaaa", Action: ActionReword, NewMessage: ""}}
	if _, err := BuildRebaseTodo(steps, []string{"/tmp/x.txt"}); err == nil {
		t.Errorf("expected error when reword message is empty")
	}
}

func TestBuildRebaseTodoErrEmpty(t *testing.T) {
	if _, err := BuildRebaseTodo(nil, nil); err == nil {
		t.Errorf("expected error for empty steps")
	}
}

func TestRebaseActionString(t *testing.T) {
	cases := map[RebaseAction]string{
		ActionPick:   "pick",
		ActionReword: "reword",
		ActionSquash: "squash",
		ActionFixup:  "fixup",
		ActionDrop:   "drop",
	}
	for a, want := range cases {
		if got := a.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", a, got, want)
		}
	}
}

// ── integration test helpers ───────────────────────────────────────────────────

// addCommit writes a file and commits it, returning nothing (the commit becomes HEAD).
func addCommit(t *testing.T, file, content, msg string) {
	t.Helper()
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, ".", "add", "-A")
	run(t, ".", "commit", "-q", "-m", msg)
}

func logSubjects(t *testing.T, n int) []string {
	t.Helper()
	out, err := exec.Command("git", "log", fmt.Sprintf("-n%d", n), "--pretty=format:%s").Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	var subs []string
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if l != "" {
			subs = append(subs, l)
		}
	}
	return subs
}

func countAbove(t *testing.T, base string) int {
	t.Helper()
	out, err := exec.Command("git", "rev-list", "--count", "--end-of-options", base+"..HEAD").Output()
	if err != nil {
		t.Fatalf("git rev-list --count: %v", err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatalf("parse count: %v", err)
	}
	return n
}

func revParse(t *testing.T, ref string) (string, error) {
	out, err := exec.Command("git", "rev-parse", "--end-of-options", ref).Output()
	return strings.TrimSpace(string(out)), err
}

// ── RebasePlan + RunInteractiveRebase integration tests ─────────────────────────

func TestRebaseSquash(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	addCommit(t, "a.txt", "a\n", "add a")
	addCommit(t, "b.txt", "b\n", "add b")
	addCommit(t, "c.txt", "c\n", "add c")

	commits, base, hasUpstream, err := RebasePlan()
	if err != nil {
		t.Fatalf("RebasePlan: %v", err)
	}
	if hasUpstream {
		t.Fatalf("expected no upstream in temp repo")
	}
	if len(commits) != 3 {
		t.Fatalf("expected 3 editable commits, got %d: %+v", len(commits), commits)
	}
	// oldest first
	if commits[0].Subject != "add a" || commits[2].Subject != "add c" {
		t.Fatalf("commits not oldest-first: %+v", commits)
	}

	steps := []RebaseStep{
		{Hash: commits[0].Hash, Action: ActionPick},
		{Hash: commits[1].Hash, Action: ActionSquash},
		{Hash: commits[2].Hash, Action: ActionSquash},
	}
	if _, err := RunInteractiveRebase(base, steps); err != nil {
		t.Fatalf("RunInteractiveRebase: %v", err)
	}
	if n := countAbove(t, base); n != 1 {
		t.Errorf("expected 1 commit above base after squash, got %d", n)
	}
	// tree content preserved
	for _, f := range []string{"a.txt", "b.txt", "c.txt"} {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("expected %s to be preserved after squash: %v", f, err)
		}
	}
}

func TestRebaseDrop(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	addCommit(t, "a.txt", "a\n", "add a")
	addCommit(t, "b.txt", "b\n", "add b")
	addCommit(t, "c.txt", "c\n", "add c")

	commits, base, _, err := RebasePlan()
	if err != nil {
		t.Fatalf("RebasePlan: %v", err)
	}
	// drop the middle commit ("add b")
	steps := []RebaseStep{
		{Hash: commits[0].Hash, Action: ActionPick},
		{Hash: commits[1].Hash, Action: ActionDrop},
		{Hash: commits[2].Hash, Action: ActionPick},
	}
	if _, err := RunInteractiveRebase(base, steps); err != nil {
		t.Fatalf("RunInteractiveRebase: %v", err)
	}
	if _, err := os.Stat("b.txt"); !os.IsNotExist(err) {
		t.Errorf("expected b.txt to be gone after dropping 'add b', stat err=%v", err)
	}
	if _, err := os.Stat("a.txt"); err != nil {
		t.Errorf("a.txt should remain: %v", err)
	}
	if _, err := os.Stat("c.txt"); err != nil {
		t.Errorf("c.txt should remain: %v", err)
	}
}

func TestRebaseReorder(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	addCommit(t, "a.txt", "a\n", "add a")
	addCommit(t, "b.txt", "b\n", "add b")

	commits, base, _, err := RebasePlan()
	if err != nil {
		t.Fatalf("RebasePlan: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 editable commits, got %d", len(commits))
	}
	// swap the order: apply "add b" first, then "add a"
	steps := []RebaseStep{
		{Hash: commits[1].Hash, Action: ActionPick},
		{Hash: commits[0].Hash, Action: ActionPick},
	}
	if _, err := RunInteractiveRebase(base, steps); err != nil {
		t.Fatalf("RunInteractiveRebase: %v", err)
	}
	subs := logSubjects(t, 2)
	// newest first: "add a" now on top, "add b" below
	if subs[0] != "add a" || subs[1] != "add b" {
		t.Errorf("reorder failed, got subjects %v", subs)
	}
}

func TestRebaseReword(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	addCommit(t, "a.txt", "a\n", "add a")
	addCommit(t, "b.txt", "b\n", "add b")

	commits, base, _, err := RebasePlan()
	if err != nil {
		t.Fatalf("RebasePlan: %v", err)
	}
	const newMsg = "reworded top commit"
	steps := []RebaseStep{
		{Hash: commits[0].Hash, Action: ActionPick},
		{Hash: commits[1].Hash, Action: ActionReword, NewMessage: newMsg},
	}
	if _, err := RunInteractiveRebase(base, steps); err != nil {
		t.Fatalf("RunInteractiveRebase: %v", err)
	}
	out, err := exec.Command("git", "log", "-1", "--format=%s").Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != newMsg {
		t.Errorf("reword failed, HEAD subject = %q, want %q", got, newMsg)
	}
}

func TestRebaseBackupRef(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	addCommit(t, "a.txt", "a\n", "add a")
	addCommit(t, "b.txt", "b\n", "add b")

	origHead, err := revParse(t, "HEAD")
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}

	commits, base, _, err := RebasePlan()
	if err != nil {
		t.Fatalf("RebasePlan: %v", err)
	}
	steps := []RebaseStep{
		{Hash: commits[0].Hash, Action: ActionPick},
		{Hash: commits[1].Hash, Action: ActionReword, NewMessage: "changed"},
	}
	backupRef, err := RunInteractiveRebase(base, steps)
	if err != nil {
		t.Fatalf("RunInteractiveRebase: %v", err)
	}
	if backupRef == "" {
		t.Fatalf("expected a non-empty backup ref")
	}
	got, err := revParse(t, backupRef)
	if err != nil {
		t.Fatalf("backup ref %s does not resolve: %v", backupRef, err)
	}
	if got != origHead {
		t.Errorf("backup ref points at %s, want original HEAD %s", got, origHead)
	}
}

func TestRebaseUntrackedFileAllowed(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	addCommit(t, "a.txt", "a\n", "add a")
	addCommit(t, "b.txt", "b\n", "add b")

	commits, base, _, err := RebasePlan()
	if err != nil {
		t.Fatalf("RebasePlan: %v", err)
	}

	// An untracked file the rebase will never touch must not block the rebase.
	if err := os.WriteFile("scratch.tmp", []byte("scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	steps := []RebaseStep{
		{Hash: commits[0].Hash, Action: ActionPick},
		{Hash: commits[1].Hash, Action: ActionReword, NewMessage: "reworded"},
	}
	if _, err := RunInteractiveRebase(base, steps); err != nil {
		t.Fatalf("expected rebase to succeed despite an untracked file: %v", err)
	}
	// The untracked file should still be present afterwards.
	if _, err := os.Stat("scratch.tmp"); err != nil {
		t.Errorf("untracked file should be preserved: %v", err)
	}
}

func TestRebaseConflictAutoAbort(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	// Two commits that both touch the same file. Reordering them makes the
	// second patch (an edit of an existing file) apply onto a base where the
	// file does not exist yet, which git cannot resolve -> conflict.
	addCommit(t, "f.txt", "one\n", "add f")
	addCommit(t, "f.txt", "two\n", "edit f")

	origHead, err := revParse(t, "HEAD")
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}

	commits, base, _, err := RebasePlan()
	if err != nil {
		t.Fatalf("RebasePlan: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 editable commits, got %d", len(commits))
	}

	// Reorder: apply "edit f" before "add f" to force a conflict.
	steps := []RebaseStep{
		{Hash: commits[1].Hash, Action: ActionPick},
		{Hash: commits[0].Hash, Action: ActionPick},
	}
	if _, err := RunInteractiveRebase(base, steps); err == nil {
		t.Fatalf("expected the conflicting rebase to fail")
	}

	// HEAD must be unchanged and no rebase may be left in progress.
	after, err := revParse(t, "HEAD")
	if err != nil {
		t.Fatalf("rev-parse HEAD after: %v", err)
	}
	if after != origHead {
		t.Errorf("HEAD changed after auto-abort: %s -> %s", origHead, after)
	}
	if rebaseInProgress(t) {
		t.Errorf("a rebase is still in progress after auto-abort")
	}
}

func TestRebaseLaunchFailureIntactHead(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	addCommit(t, "a.txt", "a\n", "add a")
	addCommit(t, "b.txt", "b\n", "add b")

	origHead, err := revParse(t, "HEAD")
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}

	commits, _, _, err := RebasePlan()
	if err != nil {
		t.Fatalf("RebasePlan: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 editable commits, got %d", len(commits))
	}

	// A base ref that does not resolve makes `git rebase -i <base>` fail at
	// launch, before any rebase state is created, leaving HEAD fully intact.
	// This exercises the honest-failure branch (no rebase to abort).
	steps := []RebaseStep{
		{Hash: commits[0].Hash, Action: ActionPick},
		{Hash: commits[1].Hash, Action: ActionPick},
	}
	_, err = RunInteractiveRebase("gito-nonexistent-base-ref", steps)
	if err == nil {
		t.Fatalf("expected RunInteractiveRebase to fail for a non-existent base")
	}

	// The error must not falsely claim a rebase is still in progress.
	if strings.Contains(err.Error(), "in progress") {
		t.Errorf("launch-time failure should not claim a rebase is in progress: %v", err)
	}

	// HEAD must be unchanged...
	after, err := revParse(t, "HEAD")
	if err != nil {
		t.Fatalf("rev-parse HEAD after: %v", err)
	}
	if after != origHead {
		t.Errorf("HEAD changed after a launch-time failure: %s -> %s", origHead, after)
	}

	// ...and no rebase state directory may be left behind.
	if rebaseInProgress(t) {
		t.Errorf("no rebase state should exist after a launch-time failure")
	}
}

// rebaseInProgress reports whether git left a rebase state directory behind.
func rebaseInProgress(t *testing.T) bool {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--git-path", "rebase-merge").Output()
	if err != nil {
		t.Fatalf("git rev-parse --git-path: %v", err)
	}
	mergePath := strings.TrimSpace(string(out))
	out, err = exec.Command("git", "rev-parse", "--git-path", "rebase-apply").Output()
	if err != nil {
		t.Fatalf("git rev-parse --git-path: %v", err)
	}
	applyPath := strings.TrimSpace(string(out))
	if _, err := os.Stat(mergePath); err == nil {
		return true
	}
	if _, err := os.Stat(applyPath); err == nil {
		return true
	}
	return false
}

func TestRebasePlanUpstreamScope(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	// Publish the initial commit to a local bare "remote" and set an upstream.
	remote := t.TempDir()
	run(t, ".", "init", "--bare", remote)
	run(t, ".", "remote", "add", "origin", remote)

	branchOut, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse branch: %v", err)
	}
	branch := strings.TrimSpace(string(branchOut))
	run(t, ".", "push", "-u", "origin", branch)

	phOut, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse pushed HEAD: %v", err)
	}
	pushedHead := strings.TrimSpace(string(phOut))

	// Add local commits that are NOT pushed.
	addCommit(t, "a.txt", "a\n", "local a")
	addCommit(t, "b.txt", "b\n", "local b")

	commits, base, hasUpstream, err := RebasePlan()
	if err != nil {
		t.Fatalf("RebasePlan: %v", err)
	}
	if !hasUpstream {
		t.Fatalf("expected hasUpstream=true when a tracking branch is configured")
	}
	if len(commits) != 2 {
		t.Fatalf("expected only the 2 unpushed commits, got %d: %+v", len(commits), commits)
	}
	if commits[0].Subject != "local a" || commits[1].Subject != "local b" {
		t.Errorf("unexpected scoped commits (want oldest-first local a, local b): %+v", commits)
	}
	if base != pushedHead {
		t.Errorf("base = %s, want merge-base at pushed HEAD %s", base, pushedHead)
	}

	// The scoped base + steps should still drive a real rebase (reword here).
	steps := []RebaseStep{
		{Hash: commits[0].Hash, Action: ActionPick},
		{Hash: commits[1].Hash, Action: ActionReword, NewMessage: "reworded local b"},
	}
	if _, err := RunInteractiveRebase(base, steps); err != nil {
		t.Fatalf("RunInteractiveRebase with upstream scope: %v", err)
	}
	if got := logSubjects(t, 1); len(got) == 0 || got[0] != "reworded local b" {
		t.Errorf("reword under upstream scope failed, subjects=%v", got)
	}
}

func TestRebaseDirtyGuard(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	addCommit(t, "a.txt", "a\n", "add a")
	addCommit(t, "b.txt", "b\n", "add b")

	origHead, err := revParse(t, "HEAD")
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}

	commits, base, _, err := RebasePlan()
	if err != nil {
		t.Fatalf("RebasePlan: %v", err)
	}

	// introduce an uncommitted change
	if err := os.WriteFile("a.txt", []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	steps := []RebaseStep{
		{Hash: commits[0].Hash, Action: ActionPick},
		{Hash: commits[1].Hash, Action: ActionReword, NewMessage: "changed"},
	}
	if _, err := RunInteractiveRebase(base, steps); err == nil {
		t.Errorf("expected error on dirty working tree")
	}
	after, err := revParse(t, "HEAD")
	if err != nil {
		t.Fatalf("rev-parse HEAD after: %v", err)
	}
	if after != origHead {
		t.Errorf("HEAD changed despite dirty guard: %s -> %s", origHead, after)
	}
}
