package ui

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mrkayhyun/gito/internal/git"
)

func newReflogTestModel(entries []git.ReflogEntry) reflogModel {
	return reflogModel{entries: entries, input: textinput.New()}
}

func sampleReflog() []git.ReflogEntry {
	return []git.ReflogEntry{
		{Short: "aaa1111", Selector: "HEAD@{0}", Action: "commit", Subject: "third"},
		{Short: "bbb2222", Selector: "HEAD@{1}", Action: "commit", Subject: "second"},
		{Short: "ccc3333", Selector: "HEAD@{2}", Action: "commit (initial)", Subject: "first"},
	}
}

func TestReflogNavigationBounds(t *testing.T) {
	m := newReflogTestModel(sampleReflog())
	step := func(k string) {
		updated, _ := m.Update(keyMsg(k))
		m = updated.(reflogModel)
	}
	step("k")
	if m.cursor != 0 {
		t.Errorf("k at top should stay at 0, got %d", m.cursor)
	}
	step("G")
	if m.cursor != 2 {
		t.Errorf("G should jump to last entry, got %d", m.cursor)
	}
	step("j")
	if m.cursor != 2 {
		t.Errorf("j at bottom should stay at 2, got %d", m.cursor)
	}
	step("g")
	if m.cursor != 0 || m.offset != 0 {
		t.Errorf("g should jump to top, got cursor=%d offset=%d", m.cursor, m.offset)
	}
}

func TestReflogQuitKeys(t *testing.T) {
	for _, k := range []tea.KeyMsg{keyMsg("q"), escKey(), ctrlCKey()} {
		_, cmd := newReflogTestModel(sampleReflog()).Update(k)
		if cmd == nil {
			t.Fatalf("%q should quit", k.String())
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("%q should return tea.Quit", k.String())
		}
	}
}

func TestReflogBranchModeEscCancels(t *testing.T) {
	m := newReflogTestModel(sampleReflog())
	updated, _ := m.Update(keyMsg("b"))
	m = updated.(reflogModel)
	if m.mode != reflogModeBranch {
		t.Fatal("b should open the recover-branch prompt")
	}
	updated, cmd := m.Update(escKey())
	m = updated.(reflogModel)
	if m.mode != reflogModeList || cmd != nil {
		t.Error("esc in the prompt should return to the list without quitting")
	}
}

func TestReflogBranchRequiresName(t *testing.T) {
	m := newReflogTestModel(sampleReflog())
	updated, _ := m.Update(keyMsg("b"))
	updated, _ = updated.(reflogModel).Update(enterKey())
	m = updated.(reflogModel)
	if m.mode != reflogModeBranch || m.errMsg == "" {
		t.Errorf("empty name should keep the prompt open with an error, mode=%v err=%q", m.mode, m.errMsg)
	}
}

// End to end: recover a commit that is no longer on any branch.
func TestReflogRecoverCreatesBranchAtEntry(t *testing.T) {
	setupUILocalTagRepo(t)
	if err := os.WriteFile("lost.txt", []byte("lost\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, ".", "add", "-A")
	runGit(t, ".", "commit", "-q", "-m", "lost work")
	lost := strings.TrimSpace(gitOut(t, "rev-parse", "HEAD"))
	runGit(t, ".", "reset", "-q", "--hard", "HEAD~1")

	entries, err := git.GetReflog(10)
	if err != nil {
		t.Fatal(err)
	}
	idx := -1
	for i, e := range entries {
		if strings.HasPrefix(lost, e.Short) {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("lost commit %s not found in reflog %+v", lost, entries)
	}

	m := newReflogTestModel(entries)
	for i := 0; i < idx; i++ {
		updated, _ := m.Update(keyMsg("j"))
		m = updated.(reflogModel)
	}
	updated, _ := m.Update(keyMsg("b"))
	m = updated.(reflogModel)
	m.input.SetValue("rescued")
	updated, _ = m.Update(enterKey())
	m = updated.(reflogModel)
	if m.errMsg != "" || m.successMsg == "" {
		t.Fatalf("recover failed: err=%q success=%q", m.errMsg, m.successMsg)
	}
	if got := strings.TrimSpace(gitOut(t, "rev-parse", "rescued")); got != lost {
		t.Errorf("branch rescued points at %s, want %s", got, lost)
	}
}

func TestReflogRecoverRejectsInvalidName(t *testing.T) {
	setupUILocalTagRepo(t)
	entries, err := git.GetReflog(10)
	if err != nil {
		t.Fatal(err)
	}
	m := newReflogTestModel(entries)
	updated, _ := m.Update(keyMsg("b"))
	m = updated.(reflogModel)
	m.input.SetValue("--force")
	updated, _ = m.Update(enterKey())
	m = updated.(reflogModel)
	if m.errMsg == "" {
		t.Error("an option-looking branch name must be rejected")
	}
	if out := gitOut(t, "for-each-ref", "--format=%(refname:short)", "refs/heads/"); strings.Count(strings.TrimSpace(out), "\n") != 0 {
		t.Errorf("no branch should have been created, got %q", out)
	}
}

func gitOut(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}
