package ui

import (
	"strings"
	"testing"

	"gito/internal/style"
)

// widthOf measures display columns, never bytes: chrome renders lipgloss
// escapes and ANSI-colored git output, so len() would be meaningless.
func widthOf(s string) int { return style.DisplayWidth(s) }

func lineCount(s string) int { return len(strings.Split(s, "\n")) }

func TestLayoutDefaultsTo80x24(t *testing.T) {
	l := newLayout()
	if l.Width != 80 || l.Height != 24 {
		t.Fatalf("newLayout() = %dx%d, want 80x24", l.Width, l.Height)
	}
	// A zero-value layout (a model that has not seen tea.WindowSizeMsg yet)
	// must measure the same, so nothing ever renders into 0 columns.
	var zero layout
	if n := zero.norm(); n.Width != 80 || n.Height != 24 {
		t.Fatalf("zero layout normalizes to %dx%d, want 80x24", n.Width, n.Height)
	}
}

func TestLayoutResizeClampsDegenerateSizes(t *testing.T) {
	tests := []struct{ w, h, wantW, wantH int }{
		{120, 40, 120, 40},
		{0, 0, minCols, minRows},
		{-5, -5, minCols, minRows},
		{10, 3, minCols, minRows},
		{minCols, minRows, minCols, minRows},
	}
	for _, tc := range tests {
		got := newLayout().resize(tc.w, tc.h)
		if got.Width != tc.wantW || got.Height != tc.wantH {
			t.Errorf("resize(%d, %d) = %dx%d, want %dx%d", tc.w, tc.h, got.Width, got.Height, tc.wantW, tc.wantH)
		}
	}
}

func TestBodyRowsIsAlwaysPositive(t *testing.T) {
	l := newLayout().resize(80, 6)
	for _, chromeLines := range []int{0, 3, 5, 6, 20, 100} {
		if got := bodyRows(l, chromeLines); got < 1 {
			t.Errorf("bodyRows(6 rows, chrome=%d) = %d, want >= 1", chromeLines, got)
		}
	}
	if got := bodyRows(newLayout(), 4); got != 20 {
		t.Errorf("bodyRows(24 rows, chrome=4) = %d, want 20", got)
	}
}

func TestHeaderIsOneLineWithinWidth(t *testing.T) {
	tests := []struct {
		name             string
		w                int
		cmd, crumb, meta string
	}{
		{"plain", 80, "tag", "", ""},
		{"crumb", 80, "tag", "show v1.2.3", ""},
		{"meta", 80, "log", "", "12/240"},
		{"all", 80, "status", "diff internal/ui/chrome.go", "3/9"},
		{"narrow", minCols, "status", "diff internal/ui/chrome.go", "3/9"},
		{"overflowing crumb", 30, "reflog", strings.Repeat("very-long-branch-name/", 6), "100/100"},
		{"multiline input", 80, "blame", "a\nb", "1\n2"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := newLayout().resize(tc.w, 24)
			got := header(l, tc.cmd, tc.crumb, tc.meta)
			if strings.Contains(got, "\n") {
				t.Fatalf("header emitted more than one line: %q", got)
			}
			if w := widthOf(got); w > l.Width {
				t.Fatalf("header is %d columns wide, want <= %d: %q", w, l.Width, got)
			}
			if !strings.Contains(got, "gito") {
				t.Errorf("header lost the app name: %q", got)
			}
		})
	}
}

func TestHeaderRightAlignsMeta(t *testing.T) {
	l := newLayout().resize(60, 24)
	got := header(l, "log", "", "3/12")
	if widthOf(got) != l.Width {
		t.Fatalf("header with meta is %d columns, want exactly %d: %q", widthOf(got), l.Width, got)
	}
	if !strings.HasSuffix(stripSGR(got), "3/12") {
		t.Errorf("meta cell is not flush right: %q", got)
	}
}

func TestFooterFitsWidthAndDropsTrailingHints(t *testing.T) {
	hints := []keyHint{
		{"↑/↓ j/k", "move"},
		{"g/G", "top/bottom"},
		{"enter", "detail"},
		{"c", "create"},
		{"p", "push"},
		{"P", "remote delete"},
		{"D", "delete"},
		{"q/esc", "quit"},
	}
	l := newLayout().resize(40, 24)
	got := footer(l, hints, true)

	if strings.Contains(got, "\n") {
		t.Fatalf("footer emitted more than one line: %q", got)
	}
	if w := widthOf(got); w > l.Width {
		t.Fatalf("footer is %d columns wide, want <= %d: %q", w, l.Width, got)
	}
	plain := stripSGR(got)
	if !strings.Contains(plain, "move") {
		t.Errorf("footer dropped the first (most important) hint: %q", plain)
	}
	if strings.Contains(plain, "remote delete") {
		t.Errorf("footer kept a hint that cannot fit 40 columns: %q", plain)
	}
	if !strings.Contains(plain, "?") {
		t.Errorf("footer dropped hints without offering the help indicator: %q", plain)
	}
}

func TestFooterKeepsEverythingWhenItFits(t *testing.T) {
	hints := []keyHint{{"j/k", "move"}, {"q", "quit"}}
	got := stripSGR(footer(newLayout(), hints, false))
	for _, want := range []string{"j/k", "move", "q", "quit"} {
		if !strings.Contains(got, want) {
			t.Errorf("footer = %q, want it to contain %q", got, want)
		}
	}
	if strings.Contains(got, "?") {
		t.Errorf("footer added a help indicator with no overlay and nothing dropped: %q", got)
	}
}

func TestFooterSurvivesAbsurdlyNarrowWidth(t *testing.T) {
	l := newLayout().resize(1, 3) // clamped to minCols x minRows
	got := footer(l, []keyHint{{"↑/↓ j/k", "move around the list"}}, false)
	if strings.Contains(got, "\n") {
		t.Fatalf("footer wrapped: %q", got)
	}
	if w := widthOf(got); w > l.Width {
		t.Fatalf("footer is %d columns wide, want <= %d: %q", w, l.Width, got)
	}
}

func TestFooterEmptyWithoutHints(t *testing.T) {
	if got := footer(newLayout(), nil, false); got != "" {
		t.Errorf("footer(nil, false) = %q, want empty", got)
	}
	if got := footer(newLayout(), nil, true); got == "" {
		t.Error("footer(nil, true) should still offer the help indicator")
	}
}

func TestHelpOverlayListsEveryHint(t *testing.T) {
	hints := []keyHint{
		{"↑/↓ j/k", "move"},
		{"g/G", "top/bottom"},
		{"enter", "detail"},
		{"P", "remote delete"},
		{"q/esc", "quit"},
	}
	got := stripSGR(helpOverlay(newLayout(), newLayout().Height, hints))
	for _, h := range hints {
		if !strings.Contains(got, h.Keys) {
			t.Errorf("help overlay is missing keys %q:\n%s", h.Keys, got)
		}
		if !strings.Contains(got, h.Desc) {
			t.Errorf("help overlay is missing description %q:\n%s", h.Desc, got)
		}
	}
	for _, line := range strings.Split(got, "\n") {
		if w := widthOf(line); w > newLayout().Width {
			t.Errorf("help overlay line is %d columns wide: %q", w, line)
		}
	}
}

func TestBannerKinds(t *testing.T) {
	restore := style.UseASCII(true)
	defer restore()

	l := newLayout()
	if got := banner(l, bannerError, "boom"); !strings.Contains(stripSGR(got), "! boom") {
		t.Errorf("error banner = %q, want the bang glyph and the message", got)
	}
	if got := banner(l, bannerSuccess, "done"); !strings.Contains(stripSGR(got), "+ done") {
		t.Errorf("success banner = %q, want the check glyph and the message", got)
	}
	if got := banner(l, bannerInfo, "fetching"); !strings.Contains(stripSGR(got), "fetching") {
		t.Errorf("info banner = %q, want the message", got)
	}
	if got := banner(l, bannerError, ""); got != "" {
		t.Errorf("empty banner = %q, want empty so callers can inline it", got)
	}
	long := banner(newLayout().resize(30, 24), bannerError, strings.Repeat("failure ", 20))
	if w := widthOf(long); w > 30 {
		t.Errorf("banner is %d columns wide, want <= 30", w)
	}
	if strings.Contains(long, "\n") {
		t.Errorf("banner emitted more than one line: %q", long)
	}
}

func TestConfirmBarIsFullWidthAndKeepsTheYNLine(t *testing.T) {
	l := newLayout().resize(50, 24)
	got := confirmBar(l, "Delete this tag? v1.0.0")
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("confirm bar has %d lines, want 2 (danger bar + y/N):\n%s", len(lines), got)
	}
	if w := widthOf(lines[0]); w != l.Width {
		t.Errorf("danger bar is %d columns wide, want the full %d", w, l.Width)
	}
	if !strings.Contains(stripSGR(lines[0]), "Delete this tag?") {
		t.Errorf("danger bar lost the prompt: %q", lines[0])
	}
	if !strings.Contains(stripSGR(lines[1]), "y") {
		t.Errorf("confirm bar lost the y/N line: %q", lines[1])
	}
}

func TestRowPadsSelectionToFullWidth(t *testing.T) {
	l := newLayout().resize(48, 24)

	sel := row(l, true, "deadbee  fix: something small")
	if w := widthOf(sel); w != l.Width {
		t.Errorf("selected row is %d columns wide, want the full %d", w, l.Width)
	}
	if !strings.Contains(stripSGR(sel), style.G.Cursor) {
		t.Errorf("selected row has no cursor glyph: %q", sel)
	}

	plain := row(l, false, "deadbee  fix: something small")
	if w := widthOf(plain); w != l.Width {
		t.Errorf("unselected row is %d columns wide, want the full %d", w, l.Width)
	}

	longer := row(l, true, strings.Repeat("x", 200))
	if w := widthOf(longer); w != l.Width {
		t.Errorf("over-long row is %d columns wide, want exactly %d", w, l.Width)
	}
	if strings.Contains(longer, "\n") {
		t.Errorf("row wrapped: %q", longer)
	}
}

func TestListWindowClampKeepsCursorVisible(t *testing.T) {
	w := listWindow{Total: 100, Rows: 10}

	// Scrolling down past the bottom of the window pulls the offset along.
	w.Cursor = 25
	w = w.clamp()
	if w.Cursor < w.Offset || w.Cursor >= w.Offset+w.Rows {
		t.Fatalf("cursor %d is not visible in [%d,%d)", w.Cursor, w.Offset, w.Offset+w.Rows)
	}
	if w.Offset != 16 {
		t.Errorf("offset = %d, want 16 (cursor - rows + 1)", w.Offset)
	}

	// Scrolling back above the offset pulls it back up.
	w.Cursor = 3
	w = w.clamp()
	if w.Offset != 3 {
		t.Errorf("offset = %d, want 3 after scrolling above the window", w.Offset)
	}
}

func TestListWindowClampBoundsAreSane(t *testing.T) {
	tests := []listWindow{
		{Cursor: 0, Offset: 0, Total: 0, Rows: 10},
		{Cursor: 5, Offset: 40, Total: 3, Rows: 10},
		{Cursor: -7, Offset: -9, Total: 20, Rows: 5},
		{Cursor: 999, Offset: 999, Total: 20, Rows: 5},
		{Cursor: 4, Offset: 2, Total: 8, Rows: 0},
		{Cursor: 2, Offset: 1, Total: -4, Rows: 4},
	}
	for _, in := range tests {
		got := in.clamp()
		if got.Offset < 0 {
			t.Errorf("%+v clamped to a negative offset %d", in, got.Offset)
		}
		if got.Total > 0 && (got.Cursor < 0 || got.Cursor >= got.Total) {
			t.Errorf("%+v clamped to an out-of-range cursor %d (total %d)", in, got.Cursor, got.Total)
		}
		start, end := in.bounds()
		if start < 0 || start > got.Total || end < start || end > got.Total {
			t.Errorf("%+v bounds (%d,%d) escape [0,%d]", in, start, end, got.Total)
		}
	}
}

func TestListWindowNoScrollWhenEverythingFits(t *testing.T) {
	w := listWindow{Cursor: 4, Offset: 3, Total: 5, Rows: 10}.clamp()
	if w.Offset != 0 {
		t.Errorf("offset = %d, want 0 when total <= rows", w.Offset)
	}
	start, end := listWindow{Cursor: 4, Offset: 3, Total: 5, Rows: 10}.bounds()
	if start != 0 || end != 5 {
		t.Errorf("bounds = (%d,%d), want (0,5)", start, end)
	}
}

func TestListWindowPosition(t *testing.T) {
	if got := (listWindow{Cursor: 0, Total: 0, Rows: 10}).position(); got != "" {
		t.Errorf("position of an empty list = %q, want empty", got)
	}
	if got := (listWindow{Cursor: 0, Total: 12, Rows: 5}).position(); got != "1/12" {
		t.Errorf("position = %q, want 1/12 (1-based cursor)", got)
	}
	if got := (listWindow{Cursor: 11, Total: 12, Rows: 5}).position(); got != "12/12" {
		t.Errorf("position = %q, want 12/12", got)
	}
}

func TestListWindowScrollbar(t *testing.T) {
	restore := style.UseASCII(true)
	defer restore()

	short := listWindow{Cursor: 0, Total: 4, Rows: 10}
	for i := 0; i < 10; i++ {
		if got := short.scrollbar(i); got != " " {
			t.Fatalf("scrollbar row %d = %q, want a blank gutter when everything fits", i, got)
		}
	}

	long := listWindow{Cursor: 0, Total: 100, Rows: 10}
	thumbs := 0
	for i := 0; i < 10; i++ {
		cell := stripSGR(long.scrollbar(i))
		if widthOf(cell) != 1 {
			t.Fatalf("scrollbar cell %d is %d columns wide, want 1", i, widthOf(cell))
		}
		if cell == style.G.ScrollThumb {
			thumbs++
		}
	}
	if thumbs == 0 {
		t.Error("scrollbar drew no thumb for a list longer than the window")
	}
	if thumbs == 10 {
		t.Error("scrollbar drew nothing but thumb for a 100-item list")
	}
	// The thumb tracks the offset: at the bottom it must be on the last row.
	bottom := listWindow{Cursor: 99, Total: 100, Rows: 10}.clamp()
	if stripSGR(bottom.scrollbar(9)) != style.G.ScrollThumb {
		t.Error("scrollbar thumb is not at the bottom when the list is scrolled to the end")
	}
	if got := long.scrollbar(-1); got != " " {
		t.Errorf("scrollbar(-1) = %q, want a blank gutter", got)
	}
}

func TestFrameFullIsExactlyLayoutHeight(t *testing.T) {
	l := newLayout().resize(60, 12)
	head := header(l, "log", "", "1/3")
	foot := footer(l, []keyHint{{"j/k", "move"}, {"q/esc", "quit"}}, true)

	for _, body := range []string{
		"",
		"one line",
		strings.Join([]string{"a", "b", "c"}, "\n"),
		strings.Repeat("row\n", 50),
	} {
		got := frameFull(l, head, body, foot)
		if n := lineCount(got); n != l.Height {
			t.Errorf("frameFull produced %d lines, want exactly %d", n, l.Height)
		}
		lines := strings.Split(got, "\n")
		if lines[0] != head {
			t.Error("frameFull did not put the header on the first line")
		}
		if lines[len(lines)-1] != foot {
			t.Error("frameFull did not pin the footer to the bottom row")
		}
	}
}

func TestFrameInlineDoesNotPad(t *testing.T) {
	got := frameInline("head", "body1\nbody2", "foot")
	if n := lineCount(got); n != 4 {
		t.Fatalf("frameInline produced %d lines, want 4: %q", n, got)
	}
	if got := frameInline("head", "", ""); got != "head" {
		t.Errorf("frameInline dropped-section handling = %q, want %q", got, "head")
	}
	if n := lineCount(frameInline("head", "body", "foot")); n >= newLayout().Height {
		t.Error("frameInline padded to the terminal height, which would scroll the shell history away")
	}
}

func TestOneLineFlattensInput(t *testing.T) {
	if got := oneLine("a\nb\r\nc\td"); strings.ContainsAny(got, "\r\n\t") {
		t.Errorf("oneLine left a control character in %q", got)
	}
	if got := oneLine("plain"); got != "plain" {
		t.Errorf("oneLine(%q) = %q, want it unchanged", "plain", got)
	}
}

// stripSGR removes ANSI escape sequences so assertions can look at the text a
// user would actually read.
func stripSGR(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			j := i + 1
			for j < len(s) && s[j] != 'm' {
				j++
			}
			i = j + 1
			continue
		}
		sb.WriteByte(s[i])
		i++
	}
	return sb.String()
}
