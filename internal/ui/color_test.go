package ui

import (
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"gito/internal/style"
)

// color_test.go covers what the rest of the suite is blind to: the escape
// sequences gito actually emits. Lip Gloss reports no color profile under
// `go test`, because the test binary's stdout is not a terminal, so every style
// renders as the identity function and a defect in how colors are composed is
// invisible. style.UseColor forces a profile (and a known background) for the
// duration of a test, which is what makes these assertions possible.

// ── SGR replay ───────────────────────────────────────────────────────────────

// bgRun reports how many leading display columns of s are painted with a
// background color, replaying the SGR sequences the way a terminal would. It is
// the measure that matters for the selected-row bar: a highlight that stops
// after the first cell has a short run, however wide the line is.
func bgRun(s string) int {
	cols, bg := 0, false
	for i := 0; i < len(s); {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j >= len(s) {
				break // an unterminated sequence prints nothing
			}
			bg = applySGR(s[i+2:j], bg)
			i = j + 1
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		w := style.DisplayWidth(string(r))
		if w > 0 && !bg {
			return cols // the first unpainted column ends the run
		}
		cols += w
		i += size
	}
	return cols
}

// applySGR interprets the background-related parameters of one SGR sequence and
// reports whether a background is set afterwards. The extended color forms
// (38;5;n, 38;2;r;g;b and their 48 counterparts) carry arguments that must be
// skipped, or a green channel of 49 would read as "default background".
func applySGR(params string, bg bool) bool {
	if params == "" {
		return false // ESC[m is a full reset, like ESC[0m
	}
	fields := strings.Split(params, ";")
	for i := 0; i < len(fields); i++ {
		n, err := strconv.Atoi(fields[i])
		if err != nil {
			continue
		}
		switch {
		case n == 0, n == 49:
			bg = false
		case n == 38, n == 48:
			if n == 48 {
				bg = true
			}
			// Skip the arguments of the extended color form.
			if i+1 < len(fields) {
				switch fields[i+1] {
				case "5":
					i += 2
				case "2":
					i += 4
				}
			}
		case n >= 40 && n <= 47, n >= 100 && n <= 107:
			bg = true
		}
	}
	return bg
}

func TestBgRunReadsSGRBackgrounds(t *testing.T) {
	restore := style.UseColor(true)
	defer restore()

	if got := bgRun("plain"); got != 0 {
		t.Errorf("bgRun of an unstyled string = %d, want 0", got)
	}
	if got := bgRun(style.RowSel.Render("abcd")); got != 4 {
		t.Errorf("bgRun of a fully painted string = %d, want 4", got)
	}
	// The defect this file exists to catch: an inner reset ends the background.
	ragged := style.RowSel.Render("ab" + style.Hash.Render("cd") + "ef")
	if got := bgRun(ragged); got >= 6 {
		t.Errorf("bgRun of a string whose background is cleared mid-line = %d, want < 6", got)
	}
}

// ── the selected-row bar ──────────────────────────────────────────────────────

// TestSelectedRowIsPaintedEdgeToEdge is the regression test for the highlight
// that used to stop at the first cell: row() wraps content that is already
// styled per cell, and Lip Gloss copies the inner reset through verbatim, so a
// plain Render left everything after the first cell on the default background.
func TestSelectedRowIsPaintedEdgeToEdge(t *testing.T) {
	restore := style.UseColor(true)
	defer restore()

	l := newLayout().resize(60, 24)
	content := style.Hash.Render("deadbee") + " " +
		style.Date.Render("2024-01-01") + " " +
		style.Subject.Render("fix: something small") + " " +
		style.AuthorName.Render("(Ada)")

	sel := row(l, true, content)
	if w := widthOf(sel); w != l.Width {
		t.Fatalf("selected row is %d columns wide, want %d", w, l.Width)
	}
	if got := bgRun(sel); got != l.Width {
		t.Errorf("the selection background covers %d of %d columns; the bar must reach the edge",
			got, l.Width)
	}
	// The cells keep their own colors: painting the row is not a repaint.
	for _, cell := range []string{"deadbee", "2024-01-01", "fix: something small", "(Ada)"} {
		if !strings.Contains(stripSGR(sel), cell) {
			t.Errorf("selected row lost the %q cell", cell)
		}
	}

	if got := bgRun(row(l, false, content)); got != 0 {
		t.Errorf("an unselected row painted %d columns; only the cursor row is highlighted", got)
	}
}

// TestSelectedRowIsPaintedOnEveryList walks the real screens: a list whose line
// builder is changed to emit pre-styled cells must not silently go back to a
// ragged highlight.
func TestSelectedRowIsPaintedOnEveryList(t *testing.T) {
	restoreColor := style.UseColor(true)
	defer restoreColor()
	restoreGlyphs := style.UseASCII(false)
	defer restoreGlyphs()

	const w, h = 80, 24
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
		{"blame", bigBlame()},
		{"branch", bigBranch()},
		{"menu", menuModel{}},
	}

	for _, p := range panes {
		t.Run(p.name, func(t *testing.T) {
			m, _ := p.model.Update(tea.WindowSizeMsg{Width: w, Height: h})
			// Move once so the cursor is on a row that is not the first line of
			// the body either.
			m, _ = m.Update(keyMsg("down"))

			var selected string
			for _, line := range strings.Split(m.View(), "\n") {
				if strings.HasPrefix(stripSGR(line), style.G.Cursor+" ") {
					selected = line
					break
				}
			}
			if selected == "" {
				t.Fatalf("no cursor row found in the view")
			}

			// The scrollbar column, when the list scrolls, is the one cell
			// outside the bar.
			if got, want := bgRun(selected), widthOf(selected)-1; got < want {
				t.Errorf("the selection background covers %d of %d columns: %q",
					got, widthOf(selected), stripSGR(selected))
			}
		})
	}
}

// TestConfirmBarIsPaintedFullWidth is the same check for the danger bar, which
// is the other full-width painted line and the one that must never look ragged
// while a destructive action is armed.
func TestConfirmBarIsPaintedFullWidth(t *testing.T) {
	restore := style.UseColor(true)
	defer restore()

	l := newLayout().resize(50, 24)
	bar := strings.Split(confirmBar(l, "Delete this tag? v1.0.0"), "\n")[0]
	if got := bgRun(bar); got != l.Width {
		t.Errorf("the danger bar covers %d of %d columns", got, l.Width)
	}
}

// TestStylesResolveOnBothBackgrounds asserts the adaptive theme really does
// resolve differently on a light and a dark terminal - the reason gito resolves
// the background once in main before any Bubble Tea program starts.
func TestStylesResolveOnBothBackgrounds(t *testing.T) {
	restoreDark := style.UseColor(true)
	dark := style.Hash.Render("abc123")
	restoreDark()

	restoreLight := style.UseColor(false)
	light := style.Hash.Render("abc123")
	restoreLight()

	if dark == light {
		t.Errorf("the adaptive theme rendered identically on both backgrounds: %q", dark)
	}
	for _, got := range []string{dark, light} {
		if !strings.Contains(got, "\x1b[") {
			t.Errorf("a forced color profile produced no escape sequence: %q", got)
		}
	}
}

// TestNoColorProfileRendersPlainText documents the other end: with no profile
// active (a pipe, NO_COLOR, this test binary) every style is the identity
// function, so painting a row must not leave stray escapes behind.
func TestNoColorProfileRendersPlainText(t *testing.T) {
	l := newLayout().resize(40, 24)
	sel := row(l, true, style.Hash.Render("abc123")+" tail")
	if strings.Contains(sel, "\x1b") {
		t.Errorf("row emitted escape sequences with no color profile: %q", sel)
	}
	if w := widthOf(sel); w != l.Width {
		t.Errorf("uncolored row is %d columns wide, want %d", w, l.Width)
	}
}
