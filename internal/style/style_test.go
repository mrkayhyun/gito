package style

import (
	"reflect"
	"strings"
	"testing"
)

// glyphFields returns every glyph in a table as name/value pairs so the tests
// cover the whole struct instead of a hand-maintained subset.
func glyphFields(g Glyphs) map[string]string {
	v := reflect.ValueOf(g)
	t := v.Type()
	out := make(map[string]string, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		out[t.Field(i).Name] = v.Field(i).String()
	}
	return out
}

// iconNames are the launcher icons, one per entry in ui.MenuItems.
var iconNames = []string{
	"IconStatus", "IconCommit", "IconLog", "IconBranch", "IconDiff",
	"IconStash", "IconTag", "IconRemote", "IconReflog", "IconBlame",
}

func TestASCIIGlyphsArePureASCII(t *testing.T) {
	restore := UseASCII(true)
	defer restore()

	for name, val := range glyphFields(G) {
		if val == "" {
			t.Errorf("glyph %s is empty in the ASCII table", name)
			continue
		}
		for i := 0; i < len(val); i++ {
			if val[i] > 0x7F {
				t.Errorf("glyph %s = %q is not pure ASCII", name, val)
				break
			}
		}
	}
}

func TestASCIILauncherIconsAreOneColumn(t *testing.T) {
	restore := UseASCII(true)
	defer restore()

	fields := glyphFields(G)
	for _, name := range iconNames {
		icon, ok := fields[name]
		if !ok {
			t.Fatalf("glyph table has no field %s", name)
		}
		if w := DisplayWidth(icon); w != 1 {
			t.Errorf("ASCII icon %s = %q is %d columns wide, want 1", name, icon, w)
		}
	}
}

func TestUnicodeGlyphsAreExpected(t *testing.T) {
	restore := UseASCII(false)
	defer restore()

	want := map[string]string{
		"Cursor":      "\u25b6",
		"Check":       "\u2713",
		"Up":          "\u2191",
		"Down":        "\u2193",
		"Ellipsis":    "\u2026",
		"Rule":        "\u2500",
		"Crumb":       "\u203a",
		"Arrow":       "\u2192",
		"ScrollTrack": "\u2502",
		"ScrollThumb": "\u2588",
		"IconStatus":  "\u25cd",
		"IconCommit":  "\u270e",
		"IconLog":     "\u2261",
		"IconBranch":  "\u2442",
		"IconDiff":    "\u21c4",
		"IconStash":   "\u229f",
		"IconTag":     "\u2302",
		"IconRemote":  "\u2601",
		"IconReflog":  "\u21ba",
		"IconBlame":   "\u25ce",
	}
	fields := glyphFields(G)
	for name, exp := range want {
		if got := fields[name]; got != exp {
			t.Errorf("Unicode glyph %s = %q, want %q", name, got, exp)
		}
	}
	// Every field must be populated in both tables.
	for name, val := range fields {
		if val == "" {
			t.Errorf("glyph %s is empty in the Unicode table", name)
		}
	}
}

func TestUseASCIIRestores(t *testing.T) {
	before := G
	restore := UseASCII(!reflect.DeepEqual(before, ASCIIGlyphs))
	restore()
	if !reflect.DeepEqual(G, before) {
		t.Fatal("UseASCII restore func did not put the previous table back")
	}
}

func TestDetectGlyphsFromEnvironment(t *testing.T) {
	tests := []struct {
		name      string
		env       map[string]string
		wantASCII bool
	}{
		{"posix locale falls back", map[string]string{"LC_ALL": "C", "LC_CTYPE": "", "LANG": "POSIX"}, true},
		{"utf-8 lang enables unicode", map[string]string{"LC_ALL": "", "LC_CTYPE": "", "LANG": "en_US.UTF-8"}, false},
		{"utf8 ctype enables unicode", map[string]string{"LC_ALL": "", "LC_CTYPE": "ko_KR.utf8", "LANG": ""}, false},
		{"gito_ascii overrides utf-8", map[string]string{"GITO_ASCII": "1", "LANG": "en_US.UTF-8"}, true},
		{"gito_ascii yes overrides", map[string]string{"GITO_ASCII": "YES", "LANG": "en_US.UTF-8"}, true},
		{"nothing set falls back", map[string]string{"LC_ALL": "", "LC_CTYPE": "", "LANG": ""}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GITO_ASCII", "")
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			got := detectGlyphs()
			gotASCII := reflect.DeepEqual(got, ASCIIGlyphs)
			if gotASCII != tc.wantASCII {
				t.Errorf("detectGlyphs() ascii=%v, want %v", gotASCII, tc.wantASCII)
			}
		})
	}
}

func TestTruncateShortStringUnchanged(t *testing.T) {
	restore := UseASCII(false)
	defer restore()

	const s = "feat: add chrome"
	if got := Truncate(s, 40); got != s {
		t.Errorf("Truncate(%q, 40) = %q, want it unchanged", s, got)
	}
	if got := Truncate(s, DisplayWidth(s)); got != s {
		t.Errorf("Truncate at exactly the display width changed the string: %q", got)
	}
}

func TestTruncateLongStringEndsWithEllipsis(t *testing.T) {
	for _, ascii := range []bool{false, true} {
		restore := UseASCII(ascii)
		s := strings.Repeat("ab", 40)
		got := Truncate(s, 12)
		if w := DisplayWidth(got); w > 12 {
			t.Errorf("ascii=%v: Truncate produced %d columns, want <= 12", ascii, w)
		}
		if !strings.HasSuffix(got, G.Ellipsis) {
			t.Errorf("ascii=%v: Truncate(%q, 12) = %q, want the ellipsis glyph %q at the end", ascii, s, got, G.Ellipsis)
		}
		restore()
	}
}

func TestTruncateMeasuresDisplayWidthNotBytes(t *testing.T) {
	restore := UseASCII(false)
	defer restore()

	// A pre-colored string like the ones internal/git returns: many more bytes
	// than columns, so len() would truncate far too early.
	colored := "\x1b[32m+added line\x1b[0m"
	if DisplayWidth(colored) != len("+added line") {
		t.Fatalf("DisplayWidth counted escapes: got %d", DisplayWidth(colored))
	}
	if got := Truncate(colored, 11); got != colored {
		t.Errorf("Truncate cut an 11-column colored string at width 11: %q", got)
	}
	cut := Truncate(colored, 6)
	if w := DisplayWidth(cut); w > 6 {
		t.Errorf("Truncate(colored, 6) is %d columns wide: %q", w, cut)
	}
}

func TestTruncateNonPositiveWidth(t *testing.T) {
	if got := Truncate("anything", 0); got != "" {
		t.Errorf("Truncate(_, 0) = %q, want empty", got)
	}
	if got := Truncate("anything", -3); got != "" {
		t.Errorf("Truncate(_, -3) = %q, want empty", got)
	}
}

func TestPad(t *testing.T) {
	if got := Pad("abc", 6); got != "abc   " {
		t.Errorf("Pad(\"abc\", 6) = %q, want %q", got, "abc   ")
	}
	if got := Pad("abcdef", 3); got != "abcdef" {
		t.Errorf("Pad should not shorten: got %q", got)
	}
	colored := "\x1b[33mdeadbeef\x1b[0m"
	if got := DisplayWidth(Pad(colored, 12)); got != 12 {
		t.Errorf("Pad of a colored string is %d columns, want 12", got)
	}
}

func TestLegacyStylesStillRender(t *testing.T) {
	// main.go and the un-migrated screens depend on these eight names.
	for name, st := range map[string]interface{ Render(...string) string }{
		"Title":    Title,
		"Selected": Selected,
		"Normal":   Normal,
		"Dimmed":   Dimmed,
		"Success":  Success,
		"Failure":  Failure,
		"Label":    Label,
		"Border":   Border,
	} {
		if got := st.Render("x"); !strings.Contains(got, "x") {
			t.Errorf("style.%s.Render(\"x\") = %q, want it to contain the text", name, got)
		}
	}
}

func TestBoxBordersFollowTheGlyphTable(t *testing.T) {
	// A box drawn with Unicode corners on a terminal that cannot render them is
	// exactly the replacement-box problem the glyph table exists to avoid.
	restore := UseASCII(false)
	unicode := Box().Render("x") + Overlay().Render("x")
	restore()

	restore = UseASCII(true)
	ascii := Box().Render("x") + Overlay().Render("x")
	restore()

	for _, r := range ascii {
		if r > 127 && r != 0x1b {
			t.Errorf("the ASCII box renders the non-ASCII rune %q: %q", r, ascii)
			break
		}
	}
	if ascii == unicode {
		t.Error("Box/Overlay render identically with and without the ASCII table")
	}
	for _, want := range []string{"╭", "╰"} {
		if !strings.Contains(unicode, want) {
			t.Errorf("the Unicode box lost its %q corner", want)
		}
	}
}

func TestSemanticStylesHaveNoPadding(t *testing.T) {
	// chrome.go composes these into fixed-width rows, so padding would break
	// the width math.
	semantic := map[string]interface {
		GetHorizontalPadding() int
		GetVerticalPadding() int
	}{
		"Hash": Hash, "Date": Date, "AuthorName": AuthorName, "Subject": Subject,
		"MetaDim": MetaDim, "Ref": Ref, "RefBase": RefBase, "RefTarget": RefTarget,
		"Badge": Badge, "KeyCap": KeyCap, "KeyDesc": KeyDesc, "SectionHead": SectionHead,
		"Cursor": Cursor, "RowSel": RowSel, "Staged": Staged, "Unstaged": Unstaged,
		"Untracked": Untracked, "Ahead": Ahead, "Behind": Behind,
		"HeaderBar": HeaderBar, "FooterBar": FooterBar, "OverlayBox": OverlayBox,
		"DangerBar": DangerBar, "ScrollTrack": ScrollTrack, "ScrollThumb": ScrollThumb,
	}
	for name, st := range semantic {
		if p := st.GetHorizontalPadding(); p != 0 {
			t.Errorf("style.%s carries %d columns of horizontal padding, want 0", name, p)
		}
		if p := st.GetVerticalPadding(); p != 0 {
			t.Errorf("style.%s carries %d rows of vertical padding, want 0", name, p)
		}
	}
}
