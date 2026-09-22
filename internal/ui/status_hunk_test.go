package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/mrkayhyun/gito/internal/git"
)

func TestStatusDiffStagesSelectedHunk(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	var before strings.Builder
	for i := 1; i <= 30; i++ {
		fmt.Fprintf(&before, "line %02d\n", i)
	}
	if err := os.WriteFile("file.txt", []byte(before.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "file.txt")
	runGit(t, dir, "commit", "-qm", "initial")
	changed := strings.ReplaceAll(before.String(), "line 02", "first edit")
	changed = strings.ReplaceAll(changed, "line 25", "second edit")
	if err := os.WriteFile("file.txt", []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	m := statusModel{width: 100, height: 30}
	loaded, _ := m.Update(doStatusLoad()())
	m = loaded.(statusModel)
	opened, load := m.Update(keyMsg("d"))
	if load == nil {
		t.Fatal("diff must load asynchronously")
	}
	ready, _ := opened.Update(load())
	selected, _ := ready.Update(keyMsg("n"))
	applying, apply := selected.Update(keyMsg(" "))
	if apply == nil {
		t.Fatal("space must apply selected hunk asynchronously")
	}
	_, duplicate := applying.Update(keyMsg(" "))
	if duplicate != nil {
		t.Fatal("must not submit the same hunk twice while applying")
	}
	finished, reload := applying.Update(apply())
	if reload == nil {
		t.Fatal("successful apply must refresh the diff")
	}
	refreshed, _ := finished.Update(reload())
	index, err := exec.Command("git", "show", ":file.txt").Output()
	if err != nil {
		t.Fatal(err)
	}
	want := strings.ReplaceAll(before.String(), "line 25", "second edit")
	if string(index) != want {
		t.Fatalf("wrong hunk staged: %s", index)
	}
	working, err := os.ReadFile("file.txt")
	if err != nil || string(working) != changed {
		t.Fatalf("working file changed: %q, %v", working, err)
	}
	back, reloadList := refreshed.Update(escKey())
	if reloadList == nil {
		t.Fatal("returning from diff must refresh staged/unstaged sections")
	}
	list, _ := back.Update(reloadList())
	entries := list.(statusModel).entries
	if len(entries) != 2 || entries[0].section != secStaged || entries[1].section != secUnstaged {
		t.Fatalf("want both index and working tree entries: %+v", entries)
	}
	// Open the newly staged entry and unstage its only hunk through the same UI.
	opened, load = list.Update(keyMsg("d"))
	ready, _ = opened.Update(load())
	applying, apply = ready.Update(keyMsg(" "))
	if apply == nil {
		t.Fatal("staged diff must allow unstaging")
	}
	finished, reload = applying.Update(apply())
	if reload == nil {
		t.Fatal("unstaging must refresh the diff")
	}
	refreshed, _ = finished.Update(reload())
	if len(refreshed.(statusModel).patch.Hunks) != 0 {
		t.Fatal("unstaged hunk still selectable in index view")
	}
	index, err = exec.Command("git", "show", ":file.txt").Output()
	if err != nil || string(index) != before.String() {
		t.Fatalf("unstaging did not restore index: %q, %v", index, err)
	}
	working, err = os.ReadFile("file.txt")
	if err != nil || string(working) != changed {
		t.Fatalf("unstaging changed working file: %q, %v", working, err)
	}
}

func TestStatusDiffReadOnlyAndLateLoad(t *testing.T) {
	m := statusModel{width: 80, height: 24, entries: []statusEntry{{file: git.FileStatus{Path: "new.txt"}, section: secUntracked}}}
	opened, load := m.Update(keyMsg("d"))
	ready, _ := opened.Update(load())
	_, apply := ready.Update(keyMsg(" "))
	if apply != nil {
		t.Fatal("untracked diff must not apply a patch")
	}
	back, _ := ready.Update(escKey())
	late, _ := back.Update(load())
	if late.(statusModel).vpReady {
		t.Fatal("late diff response must not update a closed diff")
	}
}
