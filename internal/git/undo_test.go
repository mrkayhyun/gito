package git

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// cleanRev resolves a ref to its full hash without the shared revParse helper's
// "--end-of-options" echo, so results can be compared against clean hashes taken
// from the reflog or from PreviewUndo (which resolves HEAD the same way).
func cleanRev(t *testing.T, ref string) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", ref).Output()
	if err != nil {
		t.Fatalf("git rev-parse %s: %v", ref, err)
	}
	return strings.TrimSpace(string(out))
}

// undoCommit writes a file and commits it, making the new commit HEAD. It is
// self-contained (relies only on the shared run/setupRepo helpers) so the undo
// tests do not depend on helpers defined for other features.
func undoCommit(t *testing.T, file, content, msg string) {
	t.Helper()
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, ".", "add", "-A")
	run(t, ".", "commit", "-q", "-m", msg)
}

// ── classifyUndoKind unit tests ────────────────────────────────────────────────

func TestClassifyUndoKind(t *testing.T) {
	cases := map[string]string{
		"commit: add feature":            "commit",
		"commit (initial): first":        "commit",
		"commit (amend): reword it":      "amend",
		"reset: moving to HEAD~1":        "reset",
		"rebase (finish): returning":     "rebase",
		"rebase -i (finish): onto abcd":  "rebase",
		"merge topic: Merge made":        "merge",
		"pull: Fast-forward":             "pull",
		"cherry-pick: pick that":         "cherry-pick",
		"revert: revert bad commit":      "revert",
		"clone: from git@example":        "clone",
		"branch: Created from HEAD":      "branch",
		"checkout: moving from a to b":   "checkout",
		"something totally unrecognized": "other",
	}
	for subject, want := range cases {
		if got := classifyUndoKind(subject); got != want {
			t.Errorf("classifyUndoKind(%q) = %q, want %q", subject, got, want)
		}
	}
}

// ── buildUndoOps unit tests ────────────────────────────────────────────────────

func TestBuildUndoOpsPairsEntries(t *testing.T) {
	entries := []reflogRecord{
		{full: "cccc", short: "ccc", selector: "HEAD@{0}", reflogSubject: "commit: c", commitSubject: "c"},
		{full: "bbbb", short: "bbb", selector: "HEAD@{1}", reflogSubject: "commit: b", commitSubject: "b"},
		{full: "aaaa", short: "aaa", selector: "HEAD@{2}", reflogSubject: "commit (initial): a", commitSubject: "a"},
	}
	ops := buildUndoOps(entries)
	// The oldest entry has no predecessor, so only two ops are produced.
	if len(ops) != 2 {
		t.Fatalf("expected 2 ops from 3 entries, got %d", len(ops))
	}
	// op[0] undoes the newest operation, returning HEAD to the state at HEAD@{1}.
	if ops[0].Selector != "HEAD@{0}" || ops[0].FromHash != "bbbb" || ops[0].FromSubject != "b" {
		t.Errorf("op[0] mismatch: %+v", ops[0])
	}
	if ops[0].Kind != "commit" || ops[0].ToShort != "ccc" {
		t.Errorf("op[0] kind/to mismatch: %+v", ops[0])
	}
	// op[1] returns HEAD to the initial commit at HEAD@{2}.
	if ops[1].Selector != "HEAD@{1}" || ops[1].FromHash != "aaaa" || ops[1].FromSubject != "a" {
		t.Errorf("op[1] mismatch: %+v", ops[1])
	}
}

func TestBuildUndoOpsEmptyAndSingle(t *testing.T) {
	if ops := buildUndoOps(nil); len(ops) != 0 {
		t.Errorf("expected no ops from nil entries, got %d", len(ops))
	}
	single := []reflogRecord{{full: "aaaa", selector: "HEAD@{0}", reflogSubject: "commit (initial): a"}}
	if ops := buildUndoOps(single); len(ops) != 0 {
		t.Errorf("expected no ops from a single entry (nothing to return to), got %d", len(ops))
	}
}

// ── RunUndo / PreviewUndo integration tests ────────────────────────────────────

func TestUndoLastCommit(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	undoCommit(t, "a.txt", "a\n", "add a")
	beforeB := cleanRev(t, "HEAD")
	undoCommit(t, "b.txt", "b\n", "add b")

	ops, err := RecentUndoableOps(50)
	if err != nil {
		t.Fatalf("RecentUndoableOps: %v", err)
	}
	if len(ops) == 0 {
		t.Fatalf("expected undoable ops after two commits")
	}
	// The newest op undoes "add b", returning HEAD to the "add a" commit.
	top := ops[0]
	if top.FromHash != beforeB {
		t.Fatalf("newest op should return to %s, got %s", beforeB, top.FromHash)
	}

	preview, err := PreviewUndo(top)
	if err != nil {
		t.Fatalf("PreviewUndo: %v", err)
	}
	if len(preview.Removed) != 1 || preview.Removed[0].Subject != "add b" {
		t.Errorf("expected 'add b' to be the single removed commit, got %+v", preview.Removed)
	}
	if len(preview.Restored) != 0 {
		t.Errorf("expected nothing restored when undoing the latest commit, got %+v", preview.Restored)
	}

	backupRef, err := RunUndo(top)
	if err != nil {
		t.Fatalf("RunUndo: %v", err)
	}
	if backupRef == "" {
		t.Fatalf("expected a non-empty backup ref")
	}
	if after := cleanRev(t, "HEAD"); after != beforeB {
		t.Errorf("HEAD after undo = %s, want %s", after, beforeB)
	}
	// b.txt should be gone; a.txt should remain.
	if _, err := os.Stat("b.txt"); !os.IsNotExist(err) {
		t.Errorf("expected b.txt removed after undoing 'add b', stat err=%v", err)
	}
	if _, err := os.Stat("a.txt"); err != nil {
		t.Errorf("a.txt should remain after undo: %v", err)
	}
}

func TestUndoBackupRefRecoversOriginalHead(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	undoCommit(t, "a.txt", "a\n", "add a")
	undoCommit(t, "b.txt", "b\n", "add b")
	origHead := cleanRev(t, "HEAD")

	ops, err := RecentUndoableOps(50)
	if err != nil {
		t.Fatalf("RecentUndoableOps: %v", err)
	}
	backupRef, err := RunUndo(ops[0])
	if err != nil {
		t.Fatalf("RunUndo: %v", err)
	}
	// The backup ref must point at the tip we just moved away from, so the undo
	// is itself recoverable.
	if got := cleanRev(t, backupRef); got != origHead {
		t.Errorf("backup ref points at %s, want original HEAD %s", got, origHead)
	}
}

func TestUndoDirtyGuard(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	undoCommit(t, "a.txt", "a\n", "add a")
	undoCommit(t, "b.txt", "b\n", "add b")
	origHead := cleanRev(t, "HEAD")

	ops, err := RecentUndoableOps(50)
	if err != nil {
		t.Fatalf("RecentUndoableOps: %v", err)
	}

	// A tracked, uncommitted change must block the undo so it cannot be lost.
	if err := os.WriteFile("a.txt", []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RunUndo(ops[0]); err == nil {
		t.Errorf("expected error on dirty working tree")
	}
	if after := cleanRev(t, "HEAD"); after != origHead {
		t.Errorf("HEAD changed despite dirty guard: %s -> %s", origHead, after)
	}
}

func TestUndoUntrackedFileAllowed(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	undoCommit(t, "a.txt", "a\n", "add a")
	undoCommit(t, "b.txt", "b\n", "add b")

	ops, err := RecentUndoableOps(50)
	if err != nil {
		t.Fatalf("RecentUndoableOps: %v", err)
	}

	// An untracked file a reset would never remove must not block the undo.
	if err := os.WriteFile("scratch.tmp", []byte("scratch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RunUndo(ops[0]); err != nil {
		t.Fatalf("expected undo to succeed despite an untracked file: %v", err)
	}
	if _, err := os.Stat("scratch.tmp"); err != nil {
		t.Errorf("untracked file should be preserved: %v", err)
	}
}

func TestUndoResetIsRecoverable(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	undoCommit(t, "a.txt", "a\n", "add a")
	undoCommit(t, "b.txt", "b\n", "add b")
	tipWithB := cleanRev(t, "HEAD")

	// Simulate a destructive mistake: hard reset back one commit, discarding
	// "add b" from the branch tip.
	run(t, ".", "reset", "--hard", "HEAD~1")
	if _, err := os.Stat("b.txt"); !os.IsNotExist(err) {
		t.Fatalf("precondition: b.txt should be gone after reset, stat err=%v", err)
	}

	ops, err := RecentUndoableOps(50)
	if err != nil {
		t.Fatalf("RecentUndoableOps: %v", err)
	}
	// The newest op describes the reset; undoing it should restore "add b".
	top := ops[0]
	if top.Kind != "reset" {
		t.Fatalf("expected newest op to be a reset, got %q (%s)", top.Kind, top.Description)
	}
	preview, err := PreviewUndo(top)
	if err != nil {
		t.Fatalf("PreviewUndo: %v", err)
	}
	if len(preview.Restored) != 1 || preview.Restored[0].Subject != "add b" {
		t.Errorf("expected 'add b' to be restored by undoing the reset, got %+v", preview.Restored)
	}

	if _, err := RunUndo(top); err != nil {
		t.Fatalf("RunUndo: %v", err)
	}
	if after := cleanRev(t, "HEAD"); after != tipWithB {
		t.Errorf("undo of reset should restore tip %s, got %s", tipWithB, after)
	}
	if _, err := os.Stat("b.txt"); err != nil {
		t.Errorf("b.txt should be restored after undoing the reset: %v", err)
	}
}

func TestUndoNoChangeWhenAlreadyAtTarget(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	undoCommit(t, "a.txt", "a\n", "add a")
	undoCommit(t, "b.txt", "b\n", "add b")

	// An op whose target is exactly the current HEAD must report NoChange and
	// refuse to run rather than perform a pointless reset.
	head := cleanRev(t, "HEAD")
	op := UndoableOp{Selector: "HEAD@{0}", FromHash: head, FromShort: head[:7]}

	preview, err := PreviewUndo(op)
	if err != nil {
		t.Fatalf("PreviewUndo: %v", err)
	}
	if !preview.NoChange {
		t.Errorf("expected NoChange when target equals HEAD, got %+v", preview)
	}
	if _, err := RunUndo(op); err == nil || !strings.Contains(err.Error(), "nothing to undo") {
		t.Errorf("expected RunUndo to refuse a no-op undo, got err=%v", err)
	}
}
