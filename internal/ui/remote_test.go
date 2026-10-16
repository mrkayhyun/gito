package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mrkayhyun/gito/internal/git"
	"github.com/mrkayhyun/gito/internal/i18n"
)

func remoteTestModel(hasUpstream bool, ahead, behind int, branch string) remoteModel {
	return remoteModel{
		remotes:     []git.RemoteEntry{{Name: "origin"}, {Name: "fork"}},
		hasUpstream: hasUpstream,
		ahead:       ahead,
		behind:      behind,
		branch:      branch,
	}
}

func press(t *testing.T, m remoteModel, k tea.KeyMsg) (remoteModel, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(k)
	return updated.(remoteModel), cmd
}

func TestRemotePushAsksFirstAndCancels(t *testing.T) {
	m := remoteTestModel(true, 2, 0, "main")
	m, cmd := press(t, m, keyMsg("p"))
	if !m.confirmPush || cmd != nil {
		t.Fatal("p should open the confirmation prompt without running git")
	}
	if !strings.Contains(m.View(), i18n.Tf("remote.confirm_push", 2, "main")) {
		t.Error("prompt should state how many commits go to the upstream")
	}
	for _, k := range []tea.KeyMsg{keyMsg("n"), escKey()} {
		m2 := m
		m2, cmd = press(t, m2, k)
		if m2.confirmPush || m2.loading || cmd != nil {
			t.Errorf("%q should cancel the push", k.String())
		}
	}
	m, cmd = press(t, m, keyMsg("y"))
	if cmd == nil || !m.loading || m.running != "push" {
		t.Error("y should start the push")
	}
}

func TestRemotePushNewBranchTargetsSelectedRemote(t *testing.T) {
	m := remoteTestModel(false, 0, 0, "feature")
	m, _ = press(t, m, keyMsg("j")) // select "fork"
	m, _ = press(t, m, keyMsg("p"))
	if !m.confirmPush || m.pushRemote != "fork" {
		t.Fatalf("push without upstream should target the selected remote, got %q", m.pushRemote)
	}
	if !strings.Contains(m.View(), i18n.Tf("remote.confirm_push_new", "feature", "fork")) {
		t.Error("prompt should say the branch will be tracked on the selected remote")
	}
}

func TestRemotePushGuards(t *testing.T) {
	cases := []struct {
		name string
		m    remoteModel
		want string
	}{
		{"detached", remoteTestModel(true, 1, 0, ""), i18n.T("remote.detached")},
		{"behind", remoteTestModel(true, 1, 3, "main"), i18n.Tf("remote.pull_first", 3)},
		{"up to date", remoteTestModel(true, 0, 0, "main"), i18n.T("remote.nothing_to_push")},
	}
	for _, tc := range cases {
		m, cmd := press(t, tc.m, keyMsg("p"))
		if m.confirmPush || cmd != nil {
			t.Errorf("%s: push must not be offered", tc.name)
		}
		if !strings.Contains(m.errMsg+m.successMsg, tc.want) {
			t.Errorf("%s: message = %q/%q, want %q", tc.name, m.errMsg, m.successMsg, tc.want)
		}
	}
}

func TestRemotePullGuards(t *testing.T) {
	m, cmd := press(t, remoteTestModel(false, 0, 0, "main"), keyMsg("l"))
	if cmd != nil || m.errMsg != i18n.T("remote.pull_needs_upstream") {
		t.Errorf("pull without upstream: cmd=%v err=%q", cmd != nil, m.errMsg)
	}
	m, cmd = press(t, remoteTestModel(true, 0, 0, ""), keyMsg("l"))
	if cmd != nil || m.errMsg != i18n.T("remote.detached") {
		t.Errorf("pull on detached HEAD: cmd=%v err=%q", cmd != nil, m.errMsg)
	}
	m, cmd = press(t, remoteTestModel(true, 0, 2, "main"), keyMsg("l"))
	if cmd == nil || m.running != "pull" {
		t.Error("pull with an upstream should run immediately (fast-forward only)")
	}
}

func TestRemoteIgnoresKeysWhileRunning(t *testing.T) {
	m := remoteTestModel(true, 1, 0, "main")
	m.loading = true
	m, cmd := press(t, m, keyMsg("p"))
	if m.confirmPush || cmd != nil {
		t.Error("a second operation must not start while one is running")
	}
	if _, cmd = press(t, m, keyMsg("q")); cmd == nil {
		t.Error("q should still quit while an operation runs")
	}
}

func TestRemoteOutputShowsOperation(t *testing.T) {
	m := remoteTestModel(true, 0, 0, "main")
	m.loading = true
	m, cmd := press(t, m, tea.KeyMsg{}) // no-op key while loading
	_ = cmd
	updated, cmd := m.Update(remoteOutputMsg{"pull", "Fast-forward"})
	m = updated.(remoteModel)
	if m.pane != remotePaneOutput || m.loading || cmd == nil {
		t.Fatal("output should open the output pane and refresh ahead/behind")
	}
	if !strings.Contains(m.View(), "gito remote  ›  pull") {
		t.Error("output pane title should name the operation")
	}
}
