package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"gito/internal/git"
	"gito/internal/i18n"
)

// screens_test.go covers the behaviour the shared chrome gave the four
// list-plus-viewport screens: lists that scroll instead of rendering their
// whole slice, views that never outgrow the terminal, a `?` key table, and
// destructive confirmations that render through the danger bar while keeping
// their y/N gating.

const testRows = 200

// ── model builders ───────────────────────────────────────────────────────────

// bigStatus builds a working tree far dirtier than any terminal is tall, with
// the entries grouped the way doStatusLoad groups them (staged, then unstaged,
// then untracked).
func bigStatus() statusModel {
	m := statusModel{lay: layout{Width: 80, Height: 24}}
	for _, sec := range []statusSection{secStaged, secUnstaged, secUntracked} {
		for i := 0; i < testRows; i++ {
			f := git.FileStatus{Staged: 'M', Unstaged: ' ', Path: statusPath(sec, i)}
			if sec == secUnstaged {
				f.Staged, f.Unstaged = ' ', 'M'
			}
			if sec == secUntracked {
				f.Staged, f.Unstaged = '?', '?'
			}
			m.entries = append(m.entries, statusEntry{file: f, section: sec})
		}
	}
	return m
}

func statusPath(sec statusSection, i int) string {
	return fmt.Sprintf("dir/s%d-file-%03d.go", int(sec), i)
}

func bigStash() stashModel {
	m := stashModel{lay: layout{Width: 80, Height: 24}}
	for i := 0; i < testRows; i++ {
		m.stashes = append(m.stashes, git.StashEntry{
			Ref:     fmt.Sprintf("stash@{%d}", i),
			Branch:  "main",
			Subject: fmt.Sprintf("wip-%03d", i),
		})
	}
	return m
}

func bigTag() tagModel {
	m := tagModel{lay: layout{Width: 80, Height: 24}}
	for i := 0; i < testRows; i++ {
		m.tags = append(m.tags, git.TagEntry{
			Name:       fmt.Sprintf("v9.%03d.0", i),
			TargetHash: "abc1234",
			Date:       "2024-01-01",
			Subject:    "release",
		})
	}
	return m
}

func bigLog() logModel {
	m := logModel{lay: layout{Width: 80, Height: 24}}
	for i := 0; i < testRows; i++ {
		m.commits = append(m.commits, git.CommitEntry{
			Hash:    fmt.Sprintf("%040d", i),
			Short:   fmt.Sprintf("c%06d", i),
			Date:    "2024-01-01",
			Subject: "subject",
			Author:  "Ada",
		})
	}
	return m
}

// cursorOf reads the cursor out of whichever screen model it is handed.
func cursorOf(t *testing.T, m tea.Model) int {
	t.Helper()
	switch v := m.(type) {
	case statusModel:
		return v.cursor
	case stashModel:
		return v.cursor
	case tagModel:
		return v.cursor
	case logModel:
		return v.cursor
	}
	t.Fatalf("unknown model %T", m)
	return -1
}

// ── scrolling ────────────────────────────────────────────────────────────────

// TestListsScrollAndKeepCursorVisible is the regression test for the screens
// that used to loop over their entire slice: moving the cursor past the bottom
// of the window must scroll instead of rendering a view taller than the
// terminal with the cursor on an invisible row.
func TestListsScrollAndKeepCursorVisible(t *testing.T) {
	cases := []struct {
		name  string
		model tea.Model
		label func(i int) string
	}{
		{"status", bigStatus(), func(i int) string { return statusPath(secStaged, i) }},
		{"stash", bigStash(), func(i int) string { return fmt.Sprintf("wip-%03d", i) }},
		{"tag", bigTag(), func(i int) string { return fmt.Sprintf("v9.%03d.0", i) }},
		{"log", bigLog(), func(i int) string { return fmt.Sprintf("c%06d", i) }},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := c.model
			for step := 1; step <= 60; step++ {
				m, _ = m.Update(keyMsg("j"))

				if got := cursorOf(t, m); got != step {
					t.Fatalf("after %d j presses cursor = %d, want %d", step, got, step)
				}
				v := m.View()
				if n := lineCount(v); n > 24 {
					t.Fatalf("after %d j presses the view is %d lines, terminal is 24", step, n)
				}
				want := c.label(step)
				if !strings.Contains(stripSGR(v), want) {
					t.Fatalf("after %d j presses the cursor row %q is not rendered", step, want)
				}
			}

			// Walking back up must keep the cursor visible too.
			for step := 59; step >= 0; step-- {
				m, _ = m.Update(keyMsg("k"))
				if !strings.Contains(stripSGR(m.View()), c.label(step)) {
					t.Fatalf("scrolling back up hid cursor row %q", c.label(step))
				}
			}
		})
	}
}

// ── window sizing ────────────────────────────────────────────────────────────

// TestScreensFitTerminalSize asserts every pane obeys tea.WindowSizeMsg: no
// more lines than the terminal has rows and no line wider than it has columns,
// measured in display columns rather than bytes.
func TestScreensFitTerminalSize(t *testing.T) {
	panes := []struct {
		name  string
		model tea.Model
	}{
		{"status list", bigStatus()},
		{"status diff", func() tea.Model { m := bigStatus(); m.pane = statusPaneDiff; return m }()},
		{"stash list", bigStash()},
		{"tag list", bigTag()},
		{"tag create", func() tea.Model { m := bigTag(); m.pane = tagPaneCreate; return m }()},
		{"log list", bigLog()},
		{"log detail", func() tea.Model { m := bigLog(); m.pane = paneDetail; return m }()},
	}
	sizes := []struct{ w, h int }{{40, 10}, {200, 50}}

	for _, p := range panes {
		for _, s := range sizes {
			t.Run(fmt.Sprintf("%s %dx%d", p.name, s.w, s.h), func(t *testing.T) {
				m, _ := p.model.Update(tea.WindowSizeMsg{Width: s.w, Height: s.h})
				lines := strings.Split(m.View(), "\n")
				if len(lines) > s.h {
					t.Fatalf("view is %d lines, terminal has %d rows", len(lines), s.h)
				}
				for i, ln := range lines {
					if w := widthOf(ln); w > s.w {
						t.Fatalf("line %d is %d columns wide, terminal has %d: %q",
							i, w, s.w, stripSGR(ln))
					}
				}
			})
		}
	}
}

// ── help overlay ─────────────────────────────────────────────────────────────

// TestHelpOverlayToggles asserts `?` opens the key table on all four list
// panes, that the table carries every hint, and that `?`, `q` and `esc` all
// close it without quitting the program.
func TestHelpOverlayToggles(t *testing.T) {
	cases := []struct {
		name  string
		model tea.Model
		hints []keyHint
	}{
		{"status", bigStatus(), statusListHints()},
		{"stash", bigStash(), stashListHints()},
		{"tag", bigTag(), tagListHints()},
		{"log", bigLog(), logListHints()},
	}
	title := i18n.T("help.keys_title")

	for _, c := range cases {
		for _, closer := range []tea.KeyMsg{keyMsg("?"), keyMsg("q"), escKey()} {
			t.Run(c.name+"/"+closer.String(), func(t *testing.T) {
				opened, _ := c.model.Update(keyMsg("?"))
				plain := stripSGR(opened.View())
				if !strings.Contains(plain, title) {
					t.Fatalf("'?' did not open the key table (title %q missing)", title)
				}
				for _, h := range c.hints {
					if !strings.Contains(plain, h.Keys) {
						t.Errorf("key table is missing the %q badge", h.Keys)
					}
					if !strings.Contains(plain, h.Desc) {
						t.Errorf("key table is missing the %q description", h.Desc)
					}
				}
				if n := lineCount(opened.View()); n > 24 {
					t.Errorf("key table view is %d lines, terminal is 24", n)
				}

				closed, cmd := opened.Update(closer)
				if cmd != nil {
					t.Errorf("%q should close the overlay, not quit", closer.String())
				}
				if strings.Contains(stripSGR(closed.View()), title) {
					t.Errorf("%q did not close the key table", closer.String())
				}
			})
		}
	}
}

// TestHelpKeyIsInertWhileConfirming keeps the safety semantics explicit: a
// keypress while a confirmation is armed is a cancel, so '?' must not open the
// overlay behind a live y/N prompt.
func TestHelpKeyIsInertWhileConfirming(t *testing.T) {
	tagM := bigTag()
	tagM.confirmDelete = true
	updated, _ := tagM.Update(keyMsg("?"))
	got := updated.(tagModel)
	if got.confirmDelete {
		t.Errorf("'?' during a tag confirmation should cancel it")
	}
	if got.helpOpen {
		t.Errorf("'?' during a tag confirmation should not open the key table")
	}

	statusM := bigStatus()
	statusM.confirmDiscard = true
	updated, _ = statusM.Update(keyMsg("?"))
	gotStatus := updated.(statusModel)
	if gotStatus.confirmDiscard || gotStatus.helpOpen {
		t.Errorf("'?' during a status confirmation should cancel it and open nothing")
	}

	stashM := bigStash()
	stashM.confirmDrop = true
	updated, _ = stashM.Update(keyMsg("?"))
	gotStash := updated.(stashModel)
	if gotStash.confirmDrop || gotStash.helpOpen {
		t.Errorf("'?' during a stash confirmation should cancel it and open nothing")
	}
}

// ── destructive confirmations ────────────────────────────────────────────────

// promptOf strips the %s placeholder tail off a confirm message so the
// assertion stays locale-independent.
func promptOf(key string) string {
	return strings.TrimSpace(strings.SplitN(i18n.T(key), "%s", 2)[0])
}

// TestConfirmBarRendering asserts each armed confirmation renders its
// localized prompt plus the shared y/N line, and that a non-'y' key still
// clears the flag.
func TestConfirmBarRendering(t *testing.T) {
	yn := i18n.T("common.confirm_yn")

	cases := []struct {
		name    string
		armed   tea.Model
		prompt  string
		cleared func(tea.Model) bool
	}{
		{
			name:    "status discard",
			armed:   func() tea.Model { m := bigStatus(); m.confirmDiscard = true; return m }(),
			prompt:  promptOf("status.discard_confirm"),
			cleared: func(m tea.Model) bool { return !m.(statusModel).confirmDiscard },
		},
		{
			name:    "stash drop",
			armed:   func() tea.Model { m := bigStash(); m.confirmDrop = true; return m }(),
			prompt:  promptOf("stash.drop_confirm"),
			cleared: func(m tea.Model) bool { return !m.(stashModel).confirmDrop },
		},
		{
			name:    "tag delete",
			armed:   func() tea.Model { m := bigTag(); m.confirmDelete = true; return m }(),
			prompt:  promptOf("tag.delete_confirm"),
			cleared: func(m tea.Model) bool { return !m.(tagModel).confirmDelete },
		},
		{
			name:    "tag remote delete",
			armed:   func() tea.Model { m := bigTag(); m.confirmRemoteDelete = true; return m }(),
			prompt:  promptOf("tag.remote_delete_confirm"),
			cleared: func(m tea.Model) bool { return !m.(tagModel).confirmRemoteDelete },
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plain := stripSGR(c.armed.View())
			if !strings.Contains(plain, c.prompt) {
				t.Errorf("armed confirmation does not render the prompt %q", c.prompt)
			}
			if !strings.Contains(plain, yn) {
				t.Errorf("armed confirmation does not render the y/N line %q", yn)
			}
			if n := lineCount(c.armed.View()); n > 24 {
				t.Errorf("armed confirmation view is %d lines, terminal is 24", n)
			}

			cancelled, _ := c.armed.Update(keyMsg("n"))
			if !c.cleared(cancelled) {
				t.Errorf("pressing 'n' should clear the confirmation flag")
			}
			if strings.Contains(stripSGR(cancelled.View()), c.prompt) {
				t.Errorf("cancelled confirmation should stop rendering the prompt")
			}
		})
	}
}

// ── empty and degenerate states ──────────────────────────────────────────────

// TestLogDetailWithoutCommitsDoesNotPanic guards the detail pane against an
// empty commit list. RunLog returns early on an empty log, but the model must
// not index m.commits[m.cursor] unguarded.
func TestLogDetailWithoutCommitsDoesNotPanic(t *testing.T) {
	m := logModel{pane: paneDetail}
	if v := m.View(); v == "" {
		t.Fatalf("empty detail pane rendered nothing")
	}

	scrolled, _ := m.Update(keyMsg("j"))
	if v := scrolled.View(); v == "" {
		t.Fatalf("empty detail pane rendered nothing after a keypress")
	}

	back, _ := m.Update(escKey())
	if got := back.(logModel).pane; got != paneList {
		t.Fatalf("esc should return to the list pane, got %v", got)
	}
	if v := back.View(); v == "" {
		t.Fatalf("empty list pane rendered nothing")
	}
}

// TestEmptyListsRenderTheirNotice checks the zero-entry path of each list still
// frames correctly instead of dividing by an empty window.
func TestEmptyListsRenderTheirNotice(t *testing.T) {
	cases := []struct {
		name  string
		model tea.Model
		want  string
	}{
		{"status", statusModel{}, strings.TrimSpace(i18n.T("status.clean"))},
		{"stash", stashModel{}, strings.TrimSpace(i18n.T("stash.none"))},
		{"tag", tagModel{}, strings.TrimSpace(i18n.T("tag.none"))},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := c.model.View()
			if n := lineCount(v); n != 24 {
				t.Errorf("default view is %d lines, want the 80x24 default", n)
			}
			if !strings.Contains(stripSGR(v), c.want) {
				t.Errorf("empty list does not render %q", c.want)
			}
		})
	}
}
