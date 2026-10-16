package git

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// cloneOf clones bareDir into a fresh temp dir with a test identity and
// returns the path; used to simulate a teammate pushing to the same remote.
func cloneOf(t *testing.T, bareDir string) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "clone", "-q", bareDir, ".")
	run(t, dir, "config", "user.email", "other@example.com")
	run(t, dir, "config", "user.name", "Other")
	return dir
}

func TestPushSetsUpstreamThenPushesAgain(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()
	bareDir := newBareRemote(t)
	branch, err := CurrentBranch()
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := Upstream(); ok {
		t.Fatal("precondition: new branch should have no upstream")
	}
	if _, err := Push("origin"); err != nil {
		t.Fatalf("first push: %v", err)
	}
	if up, ok := Upstream(); !ok || up != "origin/"+branch {
		t.Fatalf("upstream after push = %q, %v; want origin/%s", up, ok, branch)
	}

	addCommit(t, "next.txt", "next\n", "next")
	if ahead, _, _ := GetAheadBehind(); ahead != 1 {
		t.Fatalf("ahead = %d, want 1", ahead)
	}
	// With an upstream, the remote argument is ignored.
	if _, err := Push(""); err != nil {
		t.Fatalf("second push: %v", err)
	}
	if got, want := gitOut(t, bareDir, "rev-parse", branch), gitOut(t, ".", "rev-parse", "HEAD"); got != want {
		t.Errorf("remote %s = %s, want %s", branch, got, want)
	}
}

func TestPushWithoutUpstreamNeedsRemote(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()
	if _, err := Push(""); err == nil {
		t.Error("push without upstream and without remote should fail")
	}
}

func TestPushNeverForces(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()
	bareDir := newBareRemote(t)
	if _, err := Push("origin"); err != nil {
		t.Fatal(err)
	}
	branch, _ := CurrentBranch()

	other := cloneOf(t, bareDir)
	if err := os.WriteFile(other+"/theirs.txt", []byte("theirs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, other, "add", "-A")
	run(t, other, "commit", "-q", "-m", "theirs")
	run(t, other, "push", "-q")
	theirs := gitOut(t, bareDir, "rev-parse", branch)

	addCommit(t, "mine.txt", "mine\n", "mine")
	if _, err := Push(""); err == nil {
		t.Fatal("a non-fast-forward push must be rejected, not forced")
	}
	if got := gitOut(t, bareDir, "rev-parse", branch); got != theirs {
		t.Errorf("remote moved to %s; their commit %s must survive", got, theirs)
	}
}

func TestPullFastForwardsAndRefusesToMerge(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()
	bareDir := newBareRemote(t)
	if _, err := Push("origin"); err != nil {
		t.Fatal(err)
	}

	other := cloneOf(t, bareDir)
	if err := os.WriteFile(other+"/theirs.txt", []byte("theirs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, other, "add", "-A")
	run(t, other, "commit", "-q", "-m", "theirs")
	run(t, other, "push", "-q")

	if _, err := Pull(); err != nil {
		t.Fatalf("fast-forward pull: %v", err)
	}
	if _, err := os.Stat("theirs.txt"); err != nil {
		t.Fatal("pull should have brought in theirs.txt")
	}

	// Diverge: one commit each side. --ff-only must refuse and leave HEAD alone.
	if err := os.WriteFile(other+"/theirs2.txt", []byte("2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, other, "add", "-A")
	run(t, other, "commit", "-q", "-m", "theirs 2")
	run(t, other, "push", "-q")
	addCommit(t, "mine.txt", "mine\n", "mine")
	before := gitOut(t, ".", "rev-parse", "HEAD")
	if _, err := Pull(); err == nil {
		t.Fatal("pull on diverged history must fail under --ff-only")
	} else if strings.Contains(err.Error(), "hint:") || !strings.Contains(err.Error(), "fatal:") {
		t.Errorf("error should be git's fatal line without hints, got %q", err)
	}
	if after := gitOut(t, ".", "rev-parse", "HEAD"); after != before {
		t.Errorf("HEAD moved from %s to %s; a refused pull must not change it", before, after)
	}
	if n := gitOut(t, ".", "rev-list", "--merges", "--count", "HEAD"); n != "0" {
		t.Errorf("pull created %s merge commit(s)", n)
	}
}

func TestPullWithoutUpstream(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()
	if _, err := Pull(); err == nil || !strings.Contains(err.Error(), "upstream") {
		t.Errorf("pull without upstream: err = %v", err)
	}
}

func TestDetachedHead(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()
	run(t, ".", "checkout", "-q", "--detach")
	if _, err := CurrentBranch(); !errors.Is(err, ErrDetachedHead) {
		t.Errorf("CurrentBranch on detached HEAD: err = %v", err)
	}
	if _, err := Push("origin"); !errors.Is(err, ErrDetachedHead) {
		t.Errorf("Push on detached HEAD: err = %v", err)
	}
	if _, err := Pull(); !errors.Is(err, ErrDetachedHead) {
		t.Errorf("Pull on detached HEAD: err = %v", err)
	}
}

func TestStripHints(t *testing.T) {
	in := "hint: Diverging branches can't be fast-forwarded\nhint:\nhint:   git merge --no-ff\nfatal: Not possible to fast-forward, aborting.\n"
	if got := stripHints(in); got != "fatal: Not possible to fast-forward, aborting." {
		t.Errorf("stripHints = %q", got)
	}
}
