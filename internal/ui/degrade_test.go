package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"gito/internal/git"
	"gito/internal/i18n"
	"gito/internal/style"
)

// degrade_test.go covers the interactions the rest of the suite never drives:
// a resize that arrives while a confirmation is armed or the key overlay is
// open (both change how many rows the body may use, mid-flight), the overlay's
// behaviour on a terminal too short to hold it, and the scrollbar column.

// bodyLines returns the lines of a framed alt-screen view between the head and
// the footer, given how many lines the head occupies.
func bodyLines(view string, headLines int) []string {
	lines := strings.Split(view, "\n")
	if len(lines) <= headLines+1 {
		return nil
	}
	return lines[headLines : len(lines)-1]
}

// countListRows reports how many list rows carrying a marker are rendered. Only
// rows have the two-column cursor gutter, so a confirmation prompt that quotes
// the same path is not miscounted as a row.
func countListRows(view, marker string) int {
	n := 0
	for _, line := range strings.Split(view, "\n") {
		plain := stripSGR(line)
		gutter := strings.HasPrefix(plain, "  ") || strings.HasPrefix(plain, style.G.Cursor+" ")
		if gutter && strings.Contains(plain, marker) {
			n++
		}
	}
	return n
}

// hasMoreNote reports whether a view carries the overlay's "+N more" note for
// any plausible count. It is built from the message catalogue rather than a
// literal so the check holds in every locale.
func hasMoreNote(plain string) bool {
	for hidden := 1; hidden <= 40; hidden++ {
		if strings.Contains(plain, i18n.Tf("help.more", hidden)) {
			return true
		}
	}
	return false
}

// boxRows reports how many content lines an ASCII-bordered overlay box drew,
// counting the lines between its two border lines. That count is the row budget
// helpOverlay was given whenever the table did not fit it, which is what lets
// these tests assert one exact outcome per budget instead of accepting either of
// two. It returns 0 for a borderless table - what a budget below three rows
// renders - and -1 for the half-open box a frame that cut the overlay used to
// leave behind.
func boxRows(view string) int {
	borders, first, last := 0, -1, -1
	for i, line := range strings.Split(stripSGR(view), "\n") {
		if strings.Contains(line, "+") && strings.Trim(line, "+-") == "" {
			borders++
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	switch borders {
	case 0:
		return 0
	case 1:
		return -1
	}
	return last - first - 1
}

// ── resize while a confirmation is armed ─────────────────────────────────────

// TestResizeWhileConfirmingKeepsTheFrameExact drives tea.WindowSizeMsg into a
// model whose danger bar is already up. The confirmation grows the head by three
// lines, so the body must shrink by three in the same frame: the frame stays
// exactly as tall as the terminal and the prompt stays on screen.
func TestResizeWhileConfirmingKeepsTheFrameExact(t *testing.T) {
	yn := i18n.T("common.confirm_yn")

	cases := []struct {
		name   string
		armed  tea.Model
		plain  tea.Model
		prompt string
		marker string
	}{
		{
			name:   "status discard",
			armed:  func() tea.Model { m := bigStatus(); m.confirmDiscard = true; return m }(),
			plain:  bigStatus(),
			prompt: promptOf("status.discard_confirm"),
			marker: "dir/s",
		},
		{
			name:   "stash drop",
			armed:  func() tea.Model { m := bigStash(); m.confirmDrop = true; return m }(),
			plain:  bigStash(),
			prompt: promptOf("stash.drop_confirm"),
			marker: "stash@{",
		},
		{
			name:   "tag delete",
			armed:  func() tea.Model { m := bigTag(); m.confirmDelete = true; return m }(),
			plain:  bigTag(),
			prompt: promptOf("tag.delete_confirm"),
			marker: "v9.",
		},
	}

	sizes := []struct{ w, h int }{{40, 10}, {80, 24}, {200, 50}, {minCols, minRows}}

	for _, c := range cases {
		for _, s := range sizes {
			t.Run(fmt.Sprintf("%s %dx%d", c.name, s.w, s.h), func(t *testing.T) {
				armed, _ := c.armed.Update(tea.WindowSizeMsg{Width: s.w, Height: s.h})
				view := armed.View()

				if n := lineCount(view); n != s.h {
					t.Fatalf("resized confirmation view is %d lines, want exactly %d", n, s.h)
				}
				for i, line := range strings.Split(view, "\n") {
					if w := widthOf(line); w > s.w {
						t.Fatalf("line %d is %d columns wide, terminal has %d: %q",
							i, w, s.w, stripSGR(line))
					}
				}
				plain := stripSGR(view)
				if !strings.Contains(plain, style.Truncate(c.prompt, s.w)) {
					t.Errorf("the resized view dropped the prompt %q", c.prompt)
				}
				if !strings.Contains(plain, style.Truncate(yn, s.w)) {
					t.Errorf("the resized view dropped the y/N line")
				}

				// The three lines the confirmation adds (danger bar, y/N,
				// separator) must come off the list, not off the terminal.
				unarmed, _ := c.plain.Update(tea.WindowSizeMsg{Width: s.w, Height: s.h})
				got := countListRows(view, c.marker)
				want := countListRows(unarmed.View(), c.marker) - 3
				if want < 0 {
					want = 0
				}
				if got != want {
					t.Errorf("the armed list rendered %d rows, want %d (three fewer than unarmed)",
						got, want)
				}
			})
		}
	}
}

// TestResizeWhileConfirmingOnInlineScreen is the same check for branch, which
// runs inline: it must not pad, so the assertion is a ceiling rather than an
// equality, but the prompt must survive the resize just the same.
func TestResizeWhileConfirmingOnInlineScreen(t *testing.T) {
	prompt := promptOf("branch.delete_confirm")

	for _, s := range []struct{ w, h int }{{40, 10}, {80, 24}, {minCols, minRows}} {
		t.Run(fmt.Sprintf("%dx%d", s.w, s.h), func(t *testing.T) {
			m := bigBranch()
			m.cursor, m.confirm = 5, true

			resized, _ := m.Update(tea.WindowSizeMsg{Width: s.w, Height: s.h})
			view := resized.View()
			if n := lineCount(view); n > s.h {
				t.Fatalf("inline confirmation view is %d lines, terminal has %d", n, s.h)
			}
			for i, line := range strings.Split(view, "\n") {
				if w := widthOf(line); w > s.w {
					t.Fatalf("line %d is %d columns wide, terminal has %d: %q",
						i, w, s.w, stripSGR(line))
				}
			}
			if !strings.Contains(stripSGR(view), style.Truncate(prompt, s.w)) {
				t.Errorf("the resized inline confirmation dropped the prompt %q", prompt)
			}
		})
	}
}

// ── resize while the key overlay is open ─────────────────────────────────────

// TestResizeWhileHelpOverlayIsOpen resizes with the `?` table up. The box must
// still be framed exactly and, crucially, still be a closed box: a truncated
// overlay loses its bottom border, which is what a short terminal used to get.
func TestResizeWhileHelpOverlayIsOpen(t *testing.T) {
	restore := style.UseASCII(true) // deterministic ASCII box borders
	defer restore()

	title := i18n.T("help.keys_title")
	panes := []struct {
		name  string
		model tea.Model
	}{
		{"status", bigStatus()},
		{"log", bigLog()},
		{"stash", bigStash()},
		{"tag", bigTag()},
		{"diff", bigDiff()},
		{"remote", bigRemote()},
		{"reflog", bigReflog()},
	}
	sizes := []struct{ w, h int }{{40, 8}, {40, 10}, {80, 24}, {200, 50}, {minCols, minRows}}

	for _, p := range panes {
		for _, s := range sizes {
			t.Run(fmt.Sprintf("%s %dx%d", p.name, s.w, s.h), func(t *testing.T) {
				opened, _ := p.model.Update(keyMsg("?"))
				resized, _ := opened.Update(tea.WindowSizeMsg{Width: s.w, Height: s.h})
				view := resized.View()

				if n := lineCount(view); n != s.h {
					t.Fatalf("overlay view is %d lines, want exactly %d", n, s.h)
				}
				plain := stripSGR(view)

				// The expectation is exact rather than "title or count": the
				// title survives every budget above one row, and only at the
				// one-row floor does its single row go to the "+N more" note
				// instead, because a titled empty box - what 20x6 used to
				// render - tells the user nothing. The budget is read back from
				// the box the overlay drew rather than restated from each pane's
				// chrome, which varies with the banners the head carries.
				switch rows := boxRows(view); {
				case rows < 0:
					// A half-open box means the frame cut the overlay, which is
					// what a short terminal used to get.
					t.Fatalf("the overlay box was cut open by the frame:\n%s", plain)
				case rows <= 1:
					// Either the one-row floor or the borderless table below
					// three rows - remote's upstream banner leaves that little
					// at 20x6. Both drop hints, so both must count them.
					if !hasMoreNote(plain) {
						t.Errorf("the overlay does not say how many hints it hid:\n%s", plain)
					}
				case !strings.Contains(plain, style.Truncate(title, s.w)):
					t.Errorf("the overlay dropped its title %q at a budget of %d rows:\n%s",
						title, rows, plain)
				}
			})
		}
	}
}

// ── the overlay on a terminal too short for it ───────────────────────────────

// TestHelpOverlayFitsTheRowsItIsGiven asserts the table shrinks to its budget
// and says how many hints it hid, instead of being cut open by the frame.
func TestHelpOverlayFitsTheRowsItIsGiven(t *testing.T) {
	restore := style.UseASCII(true)
	defer restore()

	hints := []keyHint{
		{"j/k", "move"}, {"g/G", "top/bottom"}, {"enter", "detail"},
		{"c", "create"}, {"p", "push"}, {"P", "remote delete"},
		{"D", "delete"}, {"r", "refresh"}, {"q/esc", "quit"},
	}
	l := newLayout().resize(60, 24)

	for _, rows := range []int{3, 4, 5, 8, 12, 24} {
		t.Run(fmt.Sprintf("%d rows", rows), func(t *testing.T) {
			box := helpOverlay(l, rows, hints)
			lines := strings.Split(box, "\n")

			if len(lines) > rows {
				t.Fatalf("the overlay is %d lines, budget is %d:\n%s", len(lines), rows, stripSGR(box))
			}
			for i, line := range lines {
				if w := widthOf(line); w > l.Width {
					t.Errorf("line %d is %d columns wide, want <= %d", i, w, l.Width)
				}
			}
			first, last := stripSGR(lines[0]), stripSGR(lines[len(lines)-1])
			for _, border := range []string{first, last} {
				if strings.Trim(border, "+-") != "" {
					t.Fatalf("the box is not closed - border line is %q:\n%s", border, stripSGR(box))
				}
			}
			plain := stripSGR(box)

			// The budget is what the box has left for content after its two
			// border rows, and the expectation for it is exact: the title above
			// one row, the "+N more" note at the one-row floor where the title
			// would leave an empty box behind.
			budget := rows
			if rows >= 3 {
				budget = rows - 2
			}
			if budget > 1 {
				if !strings.Contains(plain, i18n.T("help.keys_title")) {
					t.Errorf("the overlay at %d rows (a budget of %d) dropped its title:\n%s",
						rows, budget, plain)
				}
			} else if !hasMoreNote(plain) {
				t.Errorf("the overlay at %d rows does not say how many hints it hid:\n%s", rows, plain)
			}

			// Every hint fits at 24 rows; below that the tail is replaced by a
			// note that says how many are hidden.
			shown := 0
			for _, h := range hints {
				if strings.Contains(plain, h.Desc) {
					shown++
				}
			}
			if rows >= len(hints)+3 {
				if shown != len(hints) {
					t.Errorf("a %d-row budget shows %d of %d hints, want all", rows, shown, len(hints))
				}
				return
			}
			if shown == len(hints) {
				t.Errorf("a %d-row budget shows every hint; it cannot fit", rows)
			}
			// Whatever the budget - including the one-row floor, where the note
			// is all there is room for - the count of hidden hints is on screen.
			// The note is matched as the catalogue renders it, so a hint whose
			// own text happens to carry the digit cannot stand in for it, and the
			// check still holds in every locale.
			if hidden := len(hints) - shown; !strings.Contains(plain, i18n.Tf("help.more", hidden)) {
				t.Errorf("the overlay hid %d hint(s) without saying so:\n%s", hidden, plain)
			}
		})
	}
}

// TestHelpOverlayOnAShortTerminalStaysClosed is the same defect seen through a
// real screen: at eight rows the box has to shed hints, and it must shed them
// itself rather than let frameFull cut its bottom border off.
func TestHelpOverlayOnAShortTerminalStaysClosed(t *testing.T) {
	restore := style.UseASCII(true)
	defer restore()

	opened, _ := bigStatus().Update(keyMsg("?"))
	resized, _ := opened.Update(tea.WindowSizeMsg{Width: 60, Height: 8})
	view := resized.View()

	body := bodyLines(view, 2) // header + blank separator
	if len(body) == 0 {
		t.Fatal("no body lines in the framed overlay")
	}
	top, bottom := stripSGR(body[0]), stripSGR(body[len(body)-1])
	if strings.Trim(top, "+-") != "" || strings.Trim(bottom, "+-") != "" {
		t.Errorf("the overlay box is open on a short terminal: top %q, bottom %q\n%s",
			top, bottom, stripSGR(view))
	}
	if !strings.Contains(stripSGR(view), i18n.T("help.keys_title")) {
		t.Error("the short overlay lost its title")
	}
}

// ── the scrollbar column ─────────────────────────────────────────────────────

// TestScrollbarTracksTheWindow asserts the position affordance is actually
// rendered: a list longer than its window gets a thumb that follows the offset,
// and the column costs the row a column rather than the terminal.
func TestScrollbarTracksTheWindow(t *testing.T) {
	restore := style.UseASCII(false)
	defer restore()

	const w, h = 80, 24
	m, _ := bigLog().Update(tea.WindowSizeMsg{Width: w, Height: h})

	// thumbRows reports the first and last body row the thumb covers.
	thumbRows := func(view string) (int, int) {
		first, last := -1, -1
		for i, line := range bodyLines(view, 2) {
			if strings.HasSuffix(stripSGR(line), style.G.ScrollThumb) {
				if first < 0 {
					first = i
				}
				last = i
			}
		}
		return first, last
	}

	top := m.View()
	for i, line := range strings.Split(top, "\n") {
		if got := widthOf(line); got > w {
			t.Fatalf("line %d is %d columns wide with the scrollbar, terminal has %d", i, got, w)
		}
	}
	if first, _ := thumbRows(top); first != 0 {
		t.Errorf("with the cursor at the top the thumb starts on body row %d, want 0", first)
	}

	bottom, _ := m.Update(keyMsg("G")) // jump to the last commit
	body := bodyLines(bottom.View(), 2)
	if _, last := thumbRows(bottom.View()); last != len(body)-1 {
		t.Errorf("with the cursor at the end the thumb ends on body row %d, want the last (%d)",
			last, len(body)-1)
	}

	// A list that fits its window shows no bar at all: no wasted column and no
	// meaningless full-height thumb.
	short := bigLog()
	short.commits = short.commits[:3]
	fits, _ := short.Update(tea.WindowSizeMsg{Width: w, Height: h})
	for _, line := range bodyLines(fits.View(), 2) {
		plain := stripSGR(line)
		if strings.HasSuffix(plain, style.G.ScrollThumb) || strings.HasSuffix(plain, style.G.ScrollTrack) {
			t.Errorf("a list that fits its window still drew a scrollbar: %q", plain)
		}
	}
}

// statusWith builds a status model of n entries laid out the way doStatusLoad
// lays them out: three contiguous groups in section order, so the body carries
// one section rule per group on top of its rows. It is the one pane whose body
// is not rows alone, which is where the scrollbar column has to be measured.
func statusWith(n int, path func(int) string) statusModel {
	m := statusModel{lay: layout{Width: 80, Height: 24}}
	secs := []statusSection{secStaged, secUnstaged, secUntracked}
	for i := 0; i < max(n, 0); i++ {
		sec := secs[i*len(secs)/max(n, 1)]
		f := git.FileStatus{Staged: 'M', Unstaged: ' ', Path: path(i)}
		switch sec {
		case secUnstaged:
			f.Staged, f.Unstaged = ' ', 'M'
		case secUntracked:
			f.Staged, f.Unstaged = '?', '?'
		}
		m.entries = append(m.entries, statusEntry{file: f, section: sec})
	}
	return m
}

// scanBar reports, over the body lines of a framed view, the first and last line
// the thumb covers and the last line carrying any scrollbar cell at all. The
// blank lines frameFull pads a short body with carry none, so the last cell is
// the bottom of the track rather than the bottom of the pane.
func scanBar(view string, headLines int) (firstThumb, lastThumb, lastCell int) {
	firstThumb, lastThumb, lastCell = -1, -1, -1
	for i, line := range bodyLines(view, headLines) {
		switch plain := stripSGR(line); {
		case strings.HasSuffix(plain, style.G.ScrollThumb):
			if firstThumb < 0 {
				firstThumb = i
			}
			lastThumb, lastCell = i, i
		case strings.HasSuffix(plain, style.G.ScrollTrack):
			lastCell = i
		}
	}
	return firstThumb, lastThumb, lastCell
}

// TestScrollbarOnAnInterleavedBody drives the scrollbar on status, whose body
// mixes section rules into its rows. The rules stretch the track without
// belonging to the window, so folding them into the row count both silenced the
// bar for a list barely longer than its window - 19 to 21 changed files over
// three sections at 80x24 - and sized the thumb against lines rather than
// entries, which pinned it to the bottom after a single 'j'.
func TestScrollbarOnAnInterleavedBody(t *testing.T) {
	restore := style.UseASCII(false)
	defer restore()

	const w, h = 80, 24
	const headLines = 2 // header + blank separator

	for _, n := range []int{19, 20, 21, 22, 25, 40, 200} {
		t.Run(fmt.Sprintf("%d entries", n), func(t *testing.T) {
			paths := func(i int) string { return fmt.Sprintf("dir/file-%03d.go", i) }

			resized, _ := statusWith(n, paths).Update(tea.WindowSizeMsg{Width: w, Height: h})
			top := resized.View()
			sm := resized.(statusModel)
			if !sm.window().scrolls() {
				t.Fatalf("%d entries do not scroll in %d rows; pick a bigger list", n, sm.listRows())
			}

			body := bodyLines(top, headLines)
			cells := 0
			for i, line := range body {
				plain := stripSGR(line)
				if plain == "" {
					continue // frameFull's padding below the last line
				}
				if got := widthOf(line); got != w {
					t.Fatalf("body line %d is %d columns wide, want exactly %d: %q", i, got, w, plain)
				}
				if !strings.HasSuffix(plain, style.G.ScrollThumb) &&
					!strings.HasSuffix(plain, style.G.ScrollTrack) {
					t.Fatalf("body line %d has a reserved but empty scrollbar column: %q", i, plain)
				}
				cells++
			}

			first, last, _ := scanBar(top, headLines)
			if first != 0 {
				t.Errorf("with the cursor at the top the thumb starts on body line %d, want 0", first)
			}
			if last-first+1 >= cells {
				t.Errorf("the thumb covers all %d cells of the track; %d entries in %d rows is not the whole list",
					cells, n, sm.listRows())
			}
			// The thumb is the window's share of the LIST - rows of entries -
			// stretched over the track, not the body's share of its own lines:
			// counting the section rules as rows made it read nearly full height.
			if size, want := last-first+1, max(cells*sm.listRows()/n, 1); size > want+1 {
				t.Errorf("the thumb covers %d of %d cells for %d entries in %d rows, want about %d",
					size, cells, n, sm.listRows(), want)
			}

			// One 'j' does not scroll the window, so the thumb must not reach the
			// bottom of the track either.
			moved, _ := resized.Update(keyMsg("j"))
			if _, movedLast, movedCell := scanBar(moved.View(), headLines); movedLast == movedCell {
				t.Errorf("the thumb hit the bottom of the track after a single 'j' with %d entries", n)
			}

			// Scrolled to the end it does reach the bottom of the track, which is
			// the last line carrying a cell rather than the last line of the pane.
			end := statusWith(n, paths)
			end.cursor = n - 1
			scrolled, _ := end.Update(tea.WindowSizeMsg{Width: w, Height: h})
			if _, endLast, endCell := scanBar(scrolled.View(), headLines); endLast != endCell {
				t.Errorf("with the cursor at the end the thumb ends on body line %d, want the track's last (%d)",
					endLast, endCell)
			}
		})
	}
}

// ── list content at the column floor ─────────────────────────────────────────

// TestListRowsAtTheColumnFloorKeepTheirContent goes to the 20-column floor the
// README documents and looks at what the rows SAY, not just how wide they are:
// reserving the scrollbar column used to be undone by row()'s normalization and
// cut back by listBody, which spent the last column of every row on an ellipsis
// even when the content was three characters long.
func TestListRowsAtTheColumnFloorKeepTheirContent(t *testing.T) {
	restore := style.UseASCII(false) // the real "…" tail, not its ASCII stand-in
	defer restore()

	t.Run("primitives", func(t *testing.T) {
		l := newLayout().resize(minCols, minRows)
		w := listWindow{Cursor: 0, Total: 50, Rows: 3}
		rl := listLayout(l, w)
		if rl.Width != l.Width-1 {
			t.Fatalf("listLayout reserved %d of %d columns, want one for the bar", rl.Width, l.Width)
		}

		lines := []string{row(rl, true, "ok"), row(rl, false, "ok")}
		for i, line := range strings.Split(listBody(l, w, lines), "\n") {
			plain := stripSGR(line)
			if got := widthOf(line); got != l.Width {
				t.Errorf("row %d is %d columns wide, want exactly %d: %q", i, got, l.Width, plain)
			}
			if !strings.Contains(plain, "ok") {
				t.Errorf("row %d lost its content: %q", i, plain)
			}
			if strings.Contains(plain, style.G.Ellipsis) {
				t.Errorf("row %d was truncated although %q fits %d columns: %q",
					i, "ok", rl.Width, plain)
			}
		}
	})

	t.Run("status at the floor", func(t *testing.T) {
		m := statusWith(30, func(i int) string { return fmt.Sprintf("f%d.go", i) })
		resized, _ := m.Update(tea.WindowSizeMsg{Width: minCols, Height: minRows})
		view := resized.View()

		if n := lineCount(view); n != minRows {
			t.Fatalf("the floor view is %d lines, want exactly %d", n, minRows)
		}
		seen := false
		for i, line := range bodyLines(view, 2) {
			plain := stripSGR(line)
			if plain == "" {
				continue
			}
			seen = true
			if got := widthOf(line); got != minCols {
				t.Errorf("body line %d is %d columns wide, want exactly %d: %q", i, got, minCols, plain)
			}
			if strings.Contains(plain, style.G.Ellipsis) {
				t.Errorf("body line %d carries an ellipsis although its content fits: %q", i, plain)
			}
		}
		if !seen {
			t.Fatal("no list content at the column floor")
		}
		if !strings.Contains(stripSGR(view), "f0.go") {
			t.Error("the floor view dropped the file name of its only visible row")
		}
	})
}
