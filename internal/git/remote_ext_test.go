package git

import (
	"os/exec"
	"strings"
	"testing"
)

// newBareRemote creates a bare repo to serve as 'origin' and returns its path.
// It must be called after setupRepo has chdir'd into the working repo.
func newBareRemote(t *testing.T) string {
	t.Helper()
	bareDir := t.TempDir()
	run(t, bareDir, "init", "--bare", "-q")
	run(t, ".", "remote", "add", "origin", bareDir)
	return bareDir
}

// remoteTags returns the raw `git ls-remote --tags <dir>` output for assertions.
func remoteTags(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "ls-remote", "--tags", dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git ls-remote --tags %s: %v\n%s", dir, err, out)
	}
	return string(out)
}

// gitOut runs a git command in dir and returns its trimmed stdout, failing the
// test on error. Used to capture values (branch names, hashes) for assertions.
func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v (in %s): %v\n%s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestPushAndDeleteRemoteTag(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	bareDir := newBareRemote(t)

	// create a local tag then push it to the bare origin.
	if err := CreateTag("v9.9.9", "", ""); err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	// pass "" to exercise the default-to-origin behaviour.
	if err := PushTag("v9.9.9", ""); err != nil {
		t.Fatalf("PushTag: %v", err)
	}

	if got := remoteTags(t, bareDir); !strings.Contains(got, "v9.9.9") {
		t.Fatalf("expected v9.9.9 on remote after PushTag, got:\n%s", got)
	}

	// delete the tag on the remote (explicit remote this time).
	if err := DeleteRemoteTag("v9.9.9", "origin"); err != nil {
		t.Fatalf("DeleteRemoteTag: %v", err)
	}
	if got := remoteTags(t, bareDir); strings.Contains(got, "v9.9.9") {
		t.Fatalf("expected v9.9.9 gone from remote after DeleteRemoteTag, got:\n%s", got)
	}
}

func TestFetchRemote(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	bareDir := newBareRemote(t)

	// Detect the default branch created by `git init` (may be master or main
	// depending on the host git config) rather than hardcoding it.
	branch := gitOut(t, ".", "rev-parse", "--abbrev-ref", "HEAD")

	// Publish the current branch to the bare origin, then fetch once so the
	// remote-tracking ref refs/remotes/origin/<branch> exists and points at the
	// current tip.
	run(t, ".", "push", "origin", "HEAD:refs/heads/"+branch)
	if _, err := Fetch("origin"); err != nil {
		t.Fatalf("Fetch(origin) initial: %v", err)
	}
	trackingRef := "refs/remotes/origin/" + branch
	before := gitOut(t, ".", "rev-parse", trackingRef)

	// Make a NEW commit in a separate clone of the bare repo and push it back to
	// origin. The working repo knows nothing about this commit yet.
	clone := t.TempDir()
	run(t, ".", "clone", "-q", bareDir, clone)
	run(t, clone, "config", "user.email", "test@example.com")
	run(t, clone, "config", "user.name", "Test User")
	run(t, clone, "commit", "-q", "--allow-empty", "-m", "remote ahead")
	newHash := gitOut(t, clone, "rev-parse", "HEAD")
	run(t, clone, "push", "-q", "origin", "HEAD:refs/heads/"+branch)

	// Sanity: the working repo's tracking ref must still be stale before Fetch.
	if before == newHash {
		t.Fatalf("precondition failed: tracking ref already at new commit %s", newHash)
	}

	// Fetch the specific remote (exercises the --end-of-options guarded branch)
	// and assert the transfer actually advanced the remote-tracking ref to the
	// commit that was pushed from the other clone.
	if _, err := Fetch("origin"); err != nil {
		t.Fatalf("Fetch(origin): %v", err)
	}
	after := gitOut(t, ".", "rev-parse", trackingRef)
	if after != newHash {
		t.Errorf("Fetch(origin) did not transfer new commit: tracking ref = %s, want %s (was %s)", after, newHash, before)
	}

	// Fetch all remotes (exercises the --all branch); a clean run is enough here.
	if _, err := Fetch(""); err != nil {
		t.Fatalf("Fetch(all): %v", err)
	}
}

func TestGetAheadBehindWithUpstream(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	newBareRemote(t)

	// publish the current branch and set it as the upstream.
	run(t, ".", "push", "-u", "origin", "HEAD")

	// baseline: in sync with upstream.
	ahead, behind, has := GetAheadBehind()
	if !has {
		t.Fatalf("expected upstream to be configured after push -u")
	}
	if ahead != 0 || behind != 0 {
		t.Fatalf("expected 0/0 ahead/behind right after push, got %d/%d", ahead, behind)
	}

	// create one new local commit -> should be exactly 1 ahead, 0 behind.
	run(t, ".", "commit", "-q", "--allow-empty", "-m", "local ahead")

	ahead, behind, has = GetAheadBehind()
	if !has {
		t.Fatalf("expected hasUpstream true")
	}
	if ahead != 1 {
		t.Errorf("expected ahead=1, got %d", ahead)
	}
	if behind != 0 {
		t.Errorf("expected behind=0, got %d", behind)
	}
}
