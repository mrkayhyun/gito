package ui

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/mrkayhyun/gito/internal/git"
	"github.com/mrkayhyun/gito/internal/i18n"
)

// newTagListModel builds a tagModel sitting on the list pane with a hand-built
// tags slice so the confirmation state machine can be exercised without a real
// git repository.
func newTagListModel() tagModel {
	return tagModel{
		tags: []git.TagEntry{
			{Name: "v1.0.0"},
			{Name: "v0.9.0"},
		},
		cursor: 0,
		pane:   tagPaneList,
	}
}

func TestTagKeyMsgStrings(t *testing.T) {
	cases := []struct {
		msg  interface{ String() string }
		want string
	}{
		{keyMsg("y"), "y"},
		{keyMsg("n"), "n"},
		{keyMsg("P"), "P"},
		{keyMsg("D"), "D"},
		{enterKey(), "enter"},
		{escKey(), "esc"},
	}
	for _, c := range cases {
		if got := c.msg.String(); got != c.want {
			t.Errorf("String() = %q, want %q", got, c.want)
		}
	}
}

func TestTagRemoteDeleteGate(t *testing.T) {
	m := newTagListModel()

	updated, _ := m.Update(keyMsg("P"))
	m2 := updated.(tagModel)
	if !m2.confirmRemoteDelete {
		t.Errorf("pressing 'P' should set confirmRemoteDelete=true")
	}
	if m2.confirmDelete {
		t.Errorf("pressing 'P' should leave confirmDelete=false")
	}
	if m2.successMsg != "" {
		t.Errorf("pressing 'P' should not set successMsg, got %q", m2.successMsg)
	}
	if m2.errMsg != "" {
		t.Errorf("pressing 'P' should not set errMsg, got %q", m2.errMsg)
	}
}

func TestTagLocalDeleteGate(t *testing.T) {
	m := newTagListModel()

	updated, _ := m.Update(keyMsg("D"))
	m2 := updated.(tagModel)
	if !m2.confirmDelete {
		t.Errorf("pressing 'D' should set confirmDelete=true")
	}
	if m2.confirmRemoteDelete {
		t.Errorf("pressing 'D' should leave confirmRemoteDelete=false")
	}
	if m2.successMsg != "" {
		t.Errorf("pressing 'D' should not set successMsg, got %q", m2.successMsg)
	}
}

func TestTagDeleteMutualExclusivity(t *testing.T) {
	// From a clean state, arming the remote-delete flag must not leave the
	// local-delete flag set, and vice-versa: the two are never armed together.
	m := newTagListModel()
	updated, _ := m.Update(keyMsg("P"))
	m2 := updated.(tagModel)
	if !m2.confirmRemoteDelete || m2.confirmDelete {
		t.Errorf("'P' should arm remote-delete exclusively (got delete=%v remote=%v)", m2.confirmDelete, m2.confirmRemoteDelete)
	}

	m = newTagListModel()
	updated, _ = m.Update(keyMsg("D"))
	m3 := updated.(tagModel)
	if !m3.confirmDelete || m3.confirmRemoteDelete {
		t.Errorf("'D' should arm local-delete exclusively (got delete=%v remote=%v)", m3.confirmDelete, m3.confirmRemoteDelete)
	}

	// Pressing the opposite delete key while one confirmation is armed is
	// treated as a cancel (non-'y'): it clears the active flag and does not arm
	// the other one.
	m = newTagListModel()
	m.confirmRemoteDelete = true
	updated, _ = m.Update(keyMsg("D"))
	m4 := updated.(tagModel)
	if m4.confirmRemoteDelete || m4.confirmDelete {
		t.Errorf("'D' while confirmRemoteDelete armed should cancel (got delete=%v remote=%v)", m4.confirmDelete, m4.confirmRemoteDelete)
	}

	m = newTagListModel()
	m.confirmDelete = true
	updated, _ = m.Update(keyMsg("P"))
	m5 := updated.(tagModel)
	if m5.confirmDelete || m5.confirmRemoteDelete {
		t.Errorf("'P' while confirmDelete armed should cancel (got delete=%v remote=%v)", m5.confirmDelete, m5.confirmRemoteDelete)
	}
}

func TestTagRemoteDeleteCancelOnNonY(t *testing.T) {
	m := newTagListModel()
	m.confirmRemoteDelete = true

	updated, _ := m.Update(keyMsg("n"))
	m2 := updated.(tagModel)
	if m2.confirmRemoteDelete {
		t.Errorf("pressing 'n' should cancel confirmRemoteDelete")
	}
	if m2.successMsg != "" || m2.errMsg != "" {
		t.Errorf("cancelling should not set success/err messages (success=%q err=%q)", m2.successMsg, m2.errMsg)
	}
}

func TestTagLocalDeleteCancelOnNonY(t *testing.T) {
	m := newTagListModel()
	m.confirmDelete = true

	updated, _ := m.Update(keyMsg("n"))
	m2 := updated.(tagModel)
	if m2.confirmDelete {
		t.Errorf("pressing 'n' should cancel confirmDelete")
	}
	if m2.successMsg != "" || m2.errMsg != "" {
		t.Errorf("cancelling should not set success/err messages (success=%q err=%q)", m2.successMsg, m2.errMsg)
	}
}

func TestTagConfirmPromptRendering(t *testing.T) {
	// Derive the expected prompt text (minus the %s tag-name placeholder) from
	// the active i18n catalog so this test is locale-independent.
	remotePrompt := strings.TrimSpace(strings.SplitN(i18n.T("tag.remote_delete_confirm"), "%s", 2)[0])
	localPrompt := strings.TrimSpace(strings.SplitN(i18n.T("tag.delete_confirm"), "%s", 2)[0])

	// No flag set: neither prompt appears.
	m := newTagListModel()
	if v := m.View(); strings.Contains(v, remotePrompt) || strings.Contains(v, localPrompt) {
		t.Errorf("no confirmation prompt should render when no flag is set")
	}

	// Remote flag set: only remote prompt appears.
	m = newTagListModel()
	m.confirmRemoteDelete = true
	if v := m.View(); !strings.Contains(v, remotePrompt) {
		t.Errorf("remote prompt missing when confirmRemoteDelete=true")
	}

	// Local flag set: local prompt appears.
	m = newTagListModel()
	m.confirmDelete = true
	if v := m.View(); !strings.Contains(v, localPrompt) {
		t.Errorf("local prompt missing when confirmDelete=true")
	}
}

// TestTagRemoteDeleteEndToEnd proves that the remote-delete confirmation gate
// actually guards a destructive git operation: 'P' then 'n' leaves the remote
// tag intact, while 'P' then 'y' removes it from the remote.
func TestTagRemoteDeleteEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	bareDir := setupUITagRepo(t)

	tags, err := git.GetTags()
	if err != nil {
		t.Fatalf("GetTags: %v", err)
	}
	if len(tags) == 0 {
		t.Fatalf("expected at least one tag after setup")
	}

	// (1) 'P' then 'n' must NOT delete the remote tag.
	m := tagModel{tags: tags, cursor: 0, pane: tagPaneList}
	updated, _ := m.Update(keyMsg("P"))
	m2 := updated.(tagModel)
	if !m2.confirmRemoteDelete {
		t.Fatalf("'P' should arm remote-delete confirmation")
	}
	updated, _ = m2.Update(keyMsg("n"))
	m3 := updated.(tagModel)
	if m3.confirmRemoteDelete {
		t.Fatalf("'n' should cancel remote-delete confirmation")
	}
	if got := lsRemoteTags(t, bareDir); !strings.Contains(got, "v1.2.3") {
		t.Fatalf("cancel should leave remote tag intact, ls-remote:\n%s", got)
	}

	// (2) 'P' then 'y' must delete the remote tag.
	m = tagModel{tags: tags, cursor: 0, pane: tagPaneList}
	updated, _ = m.Update(keyMsg("P"))
	m2 = updated.(tagModel)
	updated, cmd := m2.Update(keyMsg("y"))
	m4 := updated.(tagModel)
	if m4.confirmRemoteDelete {
		t.Fatalf("'y' should clear the remote-delete confirmation")
	}
	if m4.errMsg != "" {
		t.Fatalf("remote delete returned error: %s", m4.errMsg)
	}
	if cmd == nil {
		t.Fatalf("'y' should return a reload command")
	}
	if got := lsRemoteTags(t, bareDir); strings.Contains(got, "v1.2.3") {
		t.Fatalf("confirm should remove the remote tag, ls-remote:\n%s", got)
	}
}

// TestTagLocalDeleteEndToEnd proves that the local-delete confirmation gate
// actually guards a destructive git operation: 'D' then 'n' leaves the local
// tag intact, while 'D' then 'y' removes it via git.DeleteTag.
func TestTagLocalDeleteEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	setupUILocalTagRepo(t)

	tags, err := git.GetTags()
	if err != nil {
		t.Fatalf("GetTags: %v", err)
	}
	if len(tags) == 0 {
		t.Fatalf("expected at least one tag after setup")
	}

	// (1) 'D' then 'n' must NOT delete the local tag.
	m := tagModel{tags: tags, cursor: 0, pane: tagPaneList}
	updated, _ := m.Update(keyMsg("D"))
	m2 := updated.(tagModel)
	if !m2.confirmDelete {
		t.Fatalf("'D' should arm local-delete confirmation")
	}
	updated, _ = m2.Update(keyMsg("n"))
	m3 := updated.(tagModel)
	if m3.confirmDelete {
		t.Fatalf("'n' should cancel local-delete confirmation")
	}
	if got := localTags(t); !strings.Contains(got, "v1.2.3") {
		t.Fatalf("cancel should leave local tag intact, git tag --list:\n%s", got)
	}

	// (2) 'D' then 'y' must delete the local tag.
	m = tagModel{tags: tags, cursor: 0, pane: tagPaneList}
	updated, _ = m.Update(keyMsg("D"))
	m2 = updated.(tagModel)
	updated, cmd := m2.Update(keyMsg("y"))
	m4 := updated.(tagModel)
	if m4.confirmDelete {
		t.Fatalf("'y' should clear the local-delete confirmation")
	}
	if m4.errMsg != "" {
		t.Fatalf("local delete returned error: %s", m4.errMsg)
	}
	if cmd == nil {
		t.Fatalf("'y' should return a reload command")
	}
	if got := localTags(t); strings.Contains(got, "v1.2.3") {
		t.Fatalf("confirm should remove the local tag, git tag --list:\n%s", got)
	}
}

// setupUILocalTagRepo creates a temp working repo (chdir into it, restored on
// cleanup) with a single commit and a local tag v1.2.3. No remote is created,
// since the local-delete path never touches a remote.
func setupUILocalTagRepo(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")
	if err := os.WriteFile(dir+"/README.md", []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "initial commit")

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	runGit(t, ".", "tag", "v1.2.3")
}

// localTags returns the output of `git tag --list` in the current working
// directory for local-tag presence assertions.
func localTags(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("git", "tag", "--list")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git tag --list: %v\n%s", err, out)
	}
	return string(out)
}

// setupUITagRepo creates a temp working repo (chdir into it, restored on
// cleanup), adds a bare origin remote, creates tag v1.2.3, and pushes it. It
// returns the bare remote path for ls-remote assertions.
func setupUITagRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")
	if err := os.WriteFile(dir+"/README.md", []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "initial commit")

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	bareDir := t.TempDir()
	runGit(t, bareDir, "init", "--bare", "-q")
	runGit(t, ".", "remote", "add", "origin", bareDir)
	runGit(t, ".", "tag", "v1.2.3")
	runGit(t, ".", "push", "-q", "origin", "v1.2.3")

	return bareDir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v (in %s): %v\n%s", args, dir, err, out)
	}
}

func lsRemoteTags(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "ls-remote", "--tags", dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git ls-remote --tags %s: %v\n%s", dir, err, out)
	}
	return string(out)
}
