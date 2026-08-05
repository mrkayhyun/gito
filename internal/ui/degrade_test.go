package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

// borderLines counts the lines of a view that consist only of ASCII box drawing,
// which is how these tests tell a closed overlay box from a cut-open one.
func borderLines(view string) int {
	n := 0
	for _, line := range strings.Split(stripSGR(view), "\n") {
		if strings.Contains(line, "+") && strings.Trim(line, "+-") == "" {
			n++
		}
	}
	return n
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
				if !strings.Contains(plain, style.Truncate(title, s.w)) {
					t.Errorf("the resized overlay lost its title %q", title)
				}

				// Either a closed box, or - when the pane's own head leaves less
				// than three rows - a borderless table. Never a half-open box.
				if borders := borderLines(view); borders == 1 {
					t.Errorf("the overlay box was cut open by the frame:\n%s", plain)
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
			if !strings.Contains(stripSGR(box), i18n.T("help.keys_title")) {
				t.Errorf("the overlay lost its title at %d rows", rows)
			}

			// Every hint fits at 24 rows; below that the tail is replaced by a
			// note that says how many are hidden.
			shown := 0
			for _, h := range hints {
				if strings.Contains(stripSGR(box), h.Desc) {
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
			if hidden := len(hints) - shown; rows > 3 &&
				!strings.Contains(stripSGR(box), fmt.Sprintf("%d", hidden)) {
				t.Errorf("the overlay hid %d hint(s) without saying so:\n%s", hidden, stripSGR(box))
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
