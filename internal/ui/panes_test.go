package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"gito/internal/git"
	"gito/internal/i18n"
	"gito/internal/style"
)

// panes_test.go covers the screens migrated onto the shared chrome last: diff,
// remote, reflog, blame, menu, branch and commit. It asserts the behaviour they
// gained - lists that scroll, sizes they now store, an animated fetch
// indicator, a live commit preview and one reconciled body height per pane -
// so a reverted migration fails here instead of on a user's terminal.

// ── model builders ───────────────────────────────────────────────────────────

func bigDiff() diffModel {
	m := diffModel{lay: layout{Width: 80, Height: 24}}
	for i := 0; i < testRows; i++ {
		m.refs = append(m.refs, fmt.Sprintf("ref-%03d", i))
	}
	return m
}

func bigRemote() remoteModel {
	m := remoteModel{lay: layout{Width: 80, Height: 24}, spin: newFetchSpinner()}
	for i := 0; i < testRows; i++ {
		m.remotes = append(m.remotes, git.RemoteEntry{
			Name:     fmt.Sprintf("origin-%03d", i),
			FetchURL: "git@example.com:acme/gito.git",
		})
	}
	return m
}

func bigReflog() reflogModel {
	m := reflogModel{lay: layout{Width: 80, Height: 24}, input: textinput.New()}
	for i := 0; i < testRows; i++ {
		m.entries = append(m.entries, git.ReflogEntry{
			Short:    fmt.Sprintf("h%06d", i),
			Selector: fmt.Sprintf("HEAD@{%d}", i),
			Action:   "commit: a rather long action description that needs trimming",
			Subject:  "subject",
		})
	}
	return m
}

func bigBlame() blameModel {
	filter := textinput.New()
	m := blameModel{lay: layout{Width: 80, Height: 24}, filter: filter}
	for i := 0; i < testRows; i++ {
		m.files = append(m.files, fmt.Sprintf("pkg/file-%03d.go", i))
	}
	return m
}

func bigBranch() branchModel {
	filter := textinput.New()
	input := textinput.New()
	m := branchModel{
		lay:     layout{Width: 80, Height: 24},
		filter:  filter,
		input:   input,
		current: "branch-000",
	}
	for i := 0; i < testRows; i++ {
		m.branches = append(m.branches, fmt.Sprintf("branch-%03d", i))
	}
	return m
}

// ── scrolling ────────────────────────────────────────────────────────────────

// TestRemainingListsScrollAndKeepCursorVisible is the regression test for the
// lists that used to render their whole slice (diff, remote, branch) and for
// the two that windowed by hand (reflog, blame): the cursor must stay bounded
// and visible while the view stays inside the terminal.
func TestRemainingListsScrollAndKeepCursorVisible(t *testing.T) {
	cases := []struct {
		name  string
		model tea.Model
		down  tea.KeyMsg
		up    tea.KeyMsg
		label func(i int) string
		cur   func(tea.Model) int
	}{
		{
			name: "diff", model: bigDiff(), down: keyMsg("j"), up: keyMsg("k"),
			label: func(i int) string { return fmt.Sprintf("ref-%03d", i) },
			cur:   func(m tea.Model) int { return m.(diffModel).cursor },
		},
		{
			name: "remote", model: bigRemote(), down: keyMsg("j"), up: keyMsg("k"),
			label: func(i int) string { return fmt.Sprintf("origin-%03d", i) },
			cur:   func(m tea.Model) int { return m.(remoteModel).cursor },
		},
		{
			name: "reflog", model: bigReflog(), down: keyMsg("j"), up: keyMsg("k"),
			label: func(i int) string { return fmt.Sprintf("h%06d", i) },
			cur:   func(m tea.Model) int { return m.(reflogModel).cursor },
		},
		{
			name: "blame", model: bigBlame(), down: keyMsg("down"), up: keyMsg("up"),
			label: func(i int) string { return fmt.Sprintf("pkg/file-%03d.go", i) },
			cur:   func(m tea.Model) int { return m.(blameModel).cursor },
		},
		{
			name: "branch", model: bigBranch(), down: keyMsg("down"), up: keyMsg("up"),
			label: func(i int) string { return fmt.Sprintf("branch-%03d", i) },
			cur:   func(m tea.Model) int { return m.(branchModel).cursor },
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := c.model
			for step := 1; step <= 60; step++ {
				m, _ = m.Update(c.down)

				if got := c.cur(m); got != step {
					t.Fatalf("after %d down presses cursor = %d, want %d", step, got, step)
				}
				v := m.View()
				if n := lineCount(v); n > 24 {
					t.Fatalf("after %d down presses the view is %d lines, terminal is 24", step, n)
				}
				if want := c.label(step); !strings.Contains(stripSGR(v), want) {
					t.Fatalf("after %d down presses the cursor row %q is not rendered", step, want)
				}
			}

			for step := 59; step >= 0; step-- {
				m, _ = m.Update(c.up)
				if !strings.Contains(stripSGR(m.View()), c.label(step)) {
					t.Fatalf("scrolling back up hid cursor row %q", c.label(step))
				}
			}
		})
	}
}

// ── window sizing ────────────────────────────────────────────────────────────

// layoutOf reads the stored layout out of the three models that used to drop
// tea.WindowSizeMsg entirely.
func layoutOf(t *testing.T, m tea.Model) layout {
	t.Helper()
	switch v := m.(type) {
	case menuModel:
		return v.lay
	case branchModel:
		return v.lay
	case commitModel:
		return v.lay
	}
	t.Fatalf("unknown model %T", m)
	return layout{}
}

// TestInlineScreensStoreAndObeyResize covers menu, branch and commit, which
// stored no size at all before the migration: the resize must land in the model
// and the rendered view must respect it. The inline screens must also never pad
// to the terminal height, which would scroll the user's shell history away.
func TestInlineScreensStoreAndObeyResize(t *testing.T) {
	panes := []struct {
		name  string
		model tea.Model
	}{
		{"menu", menuModel{}},
		{"branch list", bigBranch()},
		{"branch create", func() tea.Model { m := bigBranch(); m.mode = branchModeCreate; return m }()},
		{"commit type", newCommitModel()},
		{"commit scope", func() tea.Model { m := newCommitModel(); m.step = stepScope; return m }()},
		{"commit subject", func() tea.Model { m := newCommitModel(); m.step = stepSubject; return m }()},
		{"commit body", func() tea.Model { m := newCommitModel(); m.step = stepBody; return m }()},
	}
	sizes := []struct{ w, h int }{{40, 10}, {200, 50}}

	for _, p := range panes {
		for _, s := range sizes {
			t.Run(fmt.Sprintf("%s %dx%d", p.name, s.w, s.h), func(t *testing.T) {
				m, _ := p.model.Update(tea.WindowSizeMsg{Width: s.w, Height: s.h})

				if got := layoutOf(t, m); got.Width != s.w || got.Height != s.h {
					t.Fatalf("resize was not stored: layout = %dx%d, want %dx%d",
						got.Width, got.Height, s.w, s.h)
				}

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

// TestInlineScreensDoNotPadToTerminalHeight is the other half of the inline
// contract: with little to show they must render few lines, because padding to
// the terminal height would scroll the shell history away on screens that print
// their result to stdout after exiting.
func TestInlineScreensDoNotPadToTerminalHeight(t *testing.T) {
	shortBranch := branchModel{
		filter:   textinput.New(),
		input:    textinput.New(),
		branches: []string{"main", "dev"},
		current:  "main",
	}

	panes := []struct {
		name  string
		model tea.Model
	}{
		{"menu", menuModel{}},
		{"branch list", shortBranch},
		{"commit type", newCommitModel()},
		{"commit scope", func() tea.Model { m := newCommitModel(); m.step = stepScope; return m }()},
	}

	for _, p := range panes {
		t.Run(p.name, func(t *testing.T) {
			m, _ := p.model.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
			if n := lineCount(m.View()); n >= 50 {
				t.Errorf("view is %d lines on a 50-row terminal: an inline screen must not pad", n)
			}
		})
	}
}

// TestAltScreenPanesFitTerminalSize is the same check for the migrated
// alt-screen panes, which do pin their footer to the bottom row.
func TestAltScreenPanesFitTerminalSize(t *testing.T) {
	panes := []struct {
		name  string
		model tea.Model
	}{
		{"diff pick", bigDiff()},
		{"diff view", func() tea.Model { m := bigDiff(); m.pane, m.base, m.target = diffPaneView, "main", "dev"; return m }()},
		{"remote list", bigRemote()},
		{"remote output", func() tea.Model { m := bigRemote(); m.pane = remotePaneOutput; return m }()},
		{"reflog list", bigReflog()},
		{"reflog recover", func() tea.Model { m := bigReflog(); m.mode = reflogModeBranch; return m }()},
		{"blame pick", bigBlame()},
		{"blame view", func() tea.Model { m := bigBlame(); m.pane, m.selected = blamePaneView, "pkg/file-001.go"; return m }()},
	}
	sizes := []struct{ w, h int }{{40, 10}, {200, 50}}

	for _, p := range panes {
		for _, s := range sizes {
			t.Run(fmt.Sprintf("%s %dx%d", p.name, s.w, s.h), func(t *testing.T) {
				m, _ := p.model.Update(tea.WindowSizeMsg{Width: s.w, Height: s.h})
				lines := strings.Split(m.View(), "\n")
				if len(lines) != s.h {
					t.Fatalf("view is %d lines, want exactly the %d terminal rows", len(lines), s.h)
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

// ── one reconciled body height per pane ──────────────────────────────────────

// TestBodyHeightsAreReconciled pins the arithmetic blame used to get wrong in
// two different ways at once (m.height-3 for its viewport, m.height-5 for its
// file list): for every pane the chrome lines plus the body rows must add up to
// exactly the terminal height.
func TestBodyHeightsAreReconciled(t *testing.T) {
	const h = 30

	t.Run("blame pick", func(t *testing.T) {
		m := bigBlame()
		updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: h})
		bm := updated.(blameModel)

		if got := len(bm.pickHead()) + bm.listRows() + 1; got != h {
			t.Errorf("head(%d) + rows(%d) + footer(1) = %d, want %d",
				len(bm.pickHead()), bm.listRows(), got, h)
		}
		rows := 0
		for _, ln := range strings.Split(bm.View(), "\n") {
			if strings.Contains(stripSGR(ln), "pkg/file-") {
				rows++
			}
		}
		if rows != bm.listRows() {
			t.Errorf("rendered %d file rows, listRows() says %d", rows, bm.listRows())
		}
	})

	t.Run("blame view", func(t *testing.T) {
		m := bigBlame()
		m.pane, m.selected = blamePaneView, "pkg/file-001.go"
		updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: h})
		withContent, _ := updated.Update(blameContentMsg{content: strings.Repeat("line\n", 200)})
		bm := withContent.(blameModel)

		// header + blank + viewport + footer.
		if got := 2 + bm.vp.Height + 1; got != h {
			t.Errorf("chrome(3) + viewport(%d) = %d, want %d", bm.vp.Height, got, h)
		}
		if bm.vp.Height != bm.viewRows() {
			t.Errorf("viewport height %d disagrees with viewRows() %d", bm.vp.Height, bm.viewRows())
		}
	})

	t.Run("reflog list", func(t *testing.T) {
		m := bigReflog()
		updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: h})
		rm := updated.(reflogModel)

		if got := len(rm.listHead()) + rm.listRows() + 1; got != h {
			t.Errorf("head(%d) + rows(%d) + footer(1) = %d, want %d",
				len(rm.listHead()), rm.listRows(), got, h)
		}
		rows := 0
		for _, ln := range strings.Split(rm.View(), "\n") {
			if strings.Contains(stripSGR(ln), "HEAD@{") {
				rows++
			}
		}
		if rows != rm.listRows() {
			t.Errorf("rendered %d reflog rows, listRows() says %d", rows, rm.listRows())
		}
	})
}

// TestReflogColumnsShrinkWithTheTerminal checks the padded selector and action
// columns are cut to fit instead of being frozen at 12 and 24 columns.
func TestReflogColumnsShrinkWithTheTerminal(t *testing.T) {
	e := git.ReflogEntry{
		Short:    "abc1234",
		Selector: "HEAD@{123}",
		Action:   "commit: an action label that is far too long for a narrow terminal",
		Subject:  "subject",
	}
	for _, w := range []int{minCols, 40, 80, 200} {
		l := layout{Width: w, Height: 24}
		if got := widthOf(row(l, false, reflogLine(l, e))); got != w {
			t.Errorf("row at width %d measured %d columns", w, got)
		}
	}
}

// ── help overlay on the newly migrated list panes ────────────────────────────

// TestHelpOverlayOnMigratedPanes covers the three panes that gained the '?' key
// table in this migration. blame, branch, menu and commit deliberately do NOT
// bind '?': their panes either read printable runes into a text field or are
// short enough for the footer.
func TestHelpOverlayOnMigratedPanes(t *testing.T) {
	cases := []struct {
		name  string
		model tea.Model
		hints []keyHint
	}{
		{"diff", bigDiff(), diffPickHints()},
		{"remote", bigRemote(), remoteListHints()},
		{"reflog", bigReflog(), reflogListHints()},
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

// ── remote fetch spinner ─────────────────────────────────────────────────────

// TestRemoteSpinnerRunsWhileFetchingAndStops asserts the static "fetching..."
// text became an animation that is driven by spinner.TickMsg and stops as soon
// as the fetch completes or fails.
func TestRemoteSpinnerRunsWhileFetchingAndStops(t *testing.T) {
	fetching := i18n.T("remote.fetching")

	m := bigRemote()
	m.loading = true

	// A tick while a fetch is in flight advances the animation and asks for the
	// next frame.
	first := stripSGR(m.View())
	if !strings.Contains(first, fetching) {
		t.Fatalf("in-flight fetch does not render %q", fetching)
	}
	ticked, cmd := m.Update(spinner.TickMsg{})
	if cmd == nil {
		t.Error("a tick during a fetch should schedule the next frame")
	}
	tm := ticked.(remoteModel)
	if tm.spin.View() == m.spin.View() {
		t.Error("a tick during a fetch should advance the spinner frame")
	}

	// Completion stops it: no more frames are requested and the indicator goes.
	done, _ := tm.Update(remoteFetchMsg{output: "Fetching origin"})
	dm := done.(remoteModel)
	if dm.loading {
		t.Error("remoteFetchMsg should clear the loading flag")
	}
	if _, cmd := dm.Update(spinner.TickMsg{}); cmd != nil {
		t.Error("a tick after completion should not schedule another frame")
	}
	dm.pane = remotePaneList // look at the list pane again
	if strings.Contains(stripSGR(dm.View()), fetching) {
		t.Error("the fetch indicator is still rendered after completion")
	}

	// So does a failure.
	failed, _ := tm.Update(remoteErrMsg{err: fmt.Errorf("boom")})
	fm := failed.(remoteModel)
	if fm.loading {
		t.Error("remoteErrMsg should clear the loading flag")
	}
	if strings.Contains(stripSGR(fm.View()), fetching) {
		t.Error("the fetch indicator is still rendered after a failure")
	}
}

// TestRemoteFetchIndicatorWithoutSpinnerFrames guards the degenerate case: a
// model built without newFetchSpinner must not leak the bubbles "(error)"
// placeholder into the UI.
func TestRemoteFetchIndicatorWithoutSpinnerFrames(t *testing.T) {
	m := remoteModel{lay: newLayout(), loading: true}
	plain := stripSGR(m.View())
	if strings.Contains(plain, "(error)") {
		t.Error("a frameless spinner leaked its placeholder into the view")
	}
	if !strings.Contains(plain, i18n.T("remote.fetching")) {
		t.Error("the fetch indicator disappeared without a spinner")
	}
}

// ── commit wizard ────────────────────────────────────────────────────────────

// driveToSubject advances a fresh commit model to the subject step with a scope
// and a subject typed in, as a pure state transition.
func driveToSubject(t *testing.T) commitModel {
	t.Helper()
	m := newCommitModel()

	updated, _ := m.Update(enterKey()) // type -> scope
	updated, _ = updated.(commitModel).Update(keyMsg("ui"))
	updated, _ = updated.(commitModel).Update(enterKey()) // scope -> subject
	updated, _ = updated.(commitModel).Update(keyMsg("rebuild the chrome"))

	cm := updated.(commitModel)
	if cm.step != stepSubject {
		t.Fatalf("precondition: expected stepSubject, got %v", cm.step)
	}
	if cm.scope.Value() != "ui" {
		t.Fatalf("precondition: scope = %q, want %q", cm.scope.Value(), "ui")
	}
	return cm
}

// TestCommitPreviewShowsComposedMessage asserts the wizard previews the message
// from the scope step onward instead of only at the confirmation step.
func TestCommitPreviewShowsComposedMessage(t *testing.T) {
	cm := driveToSubject(t)

	want := fmt.Sprintf("%s(ui): rebuild the chrome", cm.commitType())
	if got := stripSGR(cm.View()); !strings.Contains(got, want) {
		t.Errorf("subject step does not preview %q", want)
	}

	// The scope step previews what exists so far.
	scoped := cm
	scoped.step = stepScope
	if got := stripSGR(scoped.View()); !strings.Contains(got, cm.commitType()+"(ui):") {
		t.Errorf("scope step does not preview the composed prefix")
	}

	// The type step has nothing to preview yet.
	typed := newCommitModel()
	if len(typed.previewBox(newLayout())) != 0 {
		t.Errorf("the type step should not render a preview box")
	}
}

// TestCommitConfirmKeepsChoicesOnShortTerminal asserts the confirmation step
// budgets its rows: the bordered review box degrades to a plain message line
// before the y/n/a/e choices are ever dropped.
func TestCommitConfirmKeepsChoicesOnShortTerminal(t *testing.T) {
	base := driveToSubject(t)
	base.step = stepConfirm
	want := fmt.Sprintf("%s(ui): rebuild the chrome", base.commitType())

	tall, _ := base.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	tallView := stripSGR(tall.View())
	if !strings.Contains(tallView, want) {
		t.Errorf("the confirmation step does not show the message %q", want)
	}
	if len(tall.(commitModel).previewBox(layout{Width: 80, Height: 24})) == 0 {
		t.Error("the confirmation step should have a bordered review box on a tall terminal")
	}

	short, _ := base.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	shortView := stripSGR(short.View())
	if !strings.Contains(shortView, style.Truncate(want, 40)) {
		t.Errorf("the short confirmation step does not show the message: %q", shortView)
	}
	for _, choice := range []string{
		i18n.T("commit.yes"), i18n.T("commit.no"),
		i18n.T("commit.amend"), i18n.T("commit.edit"),
	} {
		if !strings.Contains(shortView, choice) {
			t.Errorf("the short confirmation step dropped the %q choice", choice)
		}
	}
	if n := lineCount(short.View()); n > 12 {
		t.Errorf("the short confirmation step is %d lines, terminal has 12", n)
	}
}

// TestCommitProgressDegradesOnNarrowTerminal asserts the progress row is a full
// step list when it fits and a compact "n/5 Step" indicator when it does not,
// always on one line and inside the width.
func TestCommitProgressDegradesOnNarrowTerminal(t *testing.T) {
	names := commitStepNames()

	wide := commitProgress(layout{Width: 200, Height: 24}, stepSubject)
	for _, n := range names {
		if !strings.Contains(stripSGR(wide), n) {
			t.Errorf("the wide progress row is missing step %q", n)
		}
	}

	narrow := commitProgress(layout{Width: 24, Height: 10}, stepSubject)
	plain := stripSGR(narrow)
	if !strings.Contains(plain, fmt.Sprintf("%d/%d", int(stepSubject)+1, len(names))) {
		t.Errorf("the narrow progress row lost its compact indicator: %q", plain)
	}
	if strings.Contains(plain, names[len(names)-1]) {
		t.Errorf("the narrow progress row still lists every step: %q", plain)
	}
	for _, s := range []string{wide, narrow} {
		if strings.Contains(s, "\n") {
			t.Error("the progress row must stay on one line")
		}
	}
	if w := widthOf(narrow); w > 24 {
		t.Errorf("the narrow progress row is %d columns wide, terminal has 24", w)
	}
}

// ── menu ─────────────────────────────────────────────────────────────────────

// TestMenuIconsComeFromTheGlyphTable asserts the launcher icons degrade to the
// ASCII table instead of being frozen Unicode literals in MenuItems.
func TestMenuIconsComeFromTheGlyphTable(t *testing.T) {
	unicodeIcons := []string{
		style.UnicodeGlyphs.IconStatus, style.UnicodeGlyphs.IconCommit,
		style.UnicodeGlyphs.IconLog, style.UnicodeGlyphs.IconBranch,
		style.UnicodeGlyphs.IconDiff, style.UnicodeGlyphs.IconStash,
		style.UnicodeGlyphs.IconTag, style.UnicodeGlyphs.IconRemote,
		style.UnicodeGlyphs.IconReflog, style.UnicodeGlyphs.IconBlame,
	}

	restore := style.UseASCII(false)
	for i, item := range MenuItems {
		if got := item.Icon(); got != unicodeIcons[i] {
			t.Errorf("MenuItems[%d] (%s) icon = %q, want the Unicode glyph %q",
				i, item.Key, got, unicodeIcons[i])
		}
	}
	restore()

	restore = style.UseASCII(true)
	defer restore()
	view := stripSGR(menuModel{lay: newLayout()}.View())
	for i, item := range MenuItems {
		if item.Icon() == unicodeIcons[i] {
			t.Errorf("MenuItems[%d] (%s) still uses its Unicode icon on an ASCII terminal",
				i, item.Key)
		}
		if !strings.Contains(view, item.Key) {
			t.Errorf("the launcher does not list %q", item.Key)
		}
	}
	for _, glyph := range unicodeIcons {
		if strings.Contains(view, glyph) {
			t.Errorf("the ASCII launcher still renders the Unicode glyph %q", glyph)
		}
	}
}

// TestMenuBadgesMatchTheNumberShortcuts asserts each row is labelled with the
// key that selects it, including the '0' of the tenth entry.
func TestMenuBadgesMatchTheNumberShortcuts(t *testing.T) {
	lines := strings.Split(stripSGR(menuModel{lay: newLayout()}.View()), "\n")
	for i, item := range MenuItems {
		want := fmt.Sprintf("%d", (i+1)%10)
		found := false
		for _, ln := range lines {
			if strings.Contains(ln, item.Key) && strings.Contains(ln, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("the %q row is not badged with its %q shortcut", item.Key, want)
		}
	}
}

// ── branch confirmations ─────────────────────────────────────────────────────

// TestBranchDeleteConfirmationRendersThroughTheDangerBar checks both delete
// prompts render through the shared confirm bar and that the y/N gating is
// untouched: any key that is not 'y' cancels.
func TestBranchDeleteConfirmationRendersThroughTheDangerBar(t *testing.T) {
	yn := i18n.T("common.confirm_yn")

	cases := []struct {
		name   string
		force  bool
		prompt string
	}{
		{"safe delete", false, promptOf("branch.delete_confirm")},
		{"force delete", true, promptOf("branch.force_delete_confirm")},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := bigBranch()
			m.cursor = 5
			m.confirm = true
			m.confirmForce = c.force

			plain := stripSGR(m.View())
			if !strings.Contains(plain, c.prompt) {
				t.Errorf("armed confirmation does not render the prompt %q", c.prompt)
			}
			if !strings.Contains(plain, yn) {
				t.Errorf("armed confirmation does not render the y/N line %q", yn)
			}

			cancelled, _ := m.Update(keyMsg("n"))
			cm := cancelled.(branchModel)
			if cm.confirm {
				t.Error("pressing 'n' should clear the confirmation flag")
			}
			if strings.Contains(stripSGR(cm.View()), c.prompt) {
				t.Error("cancelled confirmation should stop rendering the prompt")
			}
		})
	}
}

// ── empty states ─────────────────────────────────────────────────────────────

// TestMigratedEmptyListsRenderTheirNotice checks the zero-entry path of each
// newly migrated list still frames instead of dividing by an empty window.
func TestMigratedEmptyListsRenderTheirNotice(t *testing.T) {
	cases := []struct {
		name  string
		model tea.Model
		want  string
	}{
		{"diff", diffModel{}, strings.TrimSpace(i18n.T("diff.no_refs"))},
		{"remote", remoteModel{}, strings.TrimSpace(i18n.T("remote.none"))},
		{"reflog", reflogModel{}, strings.TrimSpace(i18n.T("reflog.no_entries"))},
		{"blame", blameModel{}, strings.TrimSpace(i18n.T("blame.none"))},
		{"branch", branchModel{}, strings.TrimSpace(i18n.T("branch.no_branches"))},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := c.model.View()
			if !strings.Contains(stripSGR(v), c.want) {
				t.Errorf("empty list does not render %q", c.want)
			}
			if n := lineCount(v); n > newLayout().Height {
				t.Errorf("empty view is %d lines, want at most the 80x24 default", n)
			}
		})
	}
}
