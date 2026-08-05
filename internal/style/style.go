// Package style owns gito's entire visual language: a semantic color theme,
// the styles built from it, a Unicode/ASCII glyph table and ANSI-aware width
// helpers.
//
// Three rules keep the layer usable from every screen:
//
//  1. Colors are roles, not literals. Screens ask for style.Hash or
//     style.Danger-flavored styles instead of re-declaring hex codes.
//  2. Every color is a lipgloss.AdaptiveColor so the UI stays readable on
//     light-background terminals. Lip Gloss degrades hex to whatever the
//     terminal reports and termenv honors NO_COLOR, so nothing here assumes
//     truecolor.
//  3. No style in the semantic set carries Padding, because internal/ui/chrome
//     composes them into fixed-width rows and padding would break the math.
//     The eight legacy styles (Title, Selected, Normal, Dimmed, Success,
//     Failure, Label, Border) keep the padding they have always had so screens
//     that have not been migrated yet render exactly as before.
package style

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// ── theme ──────────────────────────────────────────────────────────────────

// theme names the color roles the UI is allowed to talk about. Fields are
// intentionally semantic ("Danger", "Meta") rather than descriptive ("red",
// "yellow") so a palette change never requires touching a screen.
type theme struct {
	Accent    lipgloss.AdaptiveColor // brand purple: titles, borders, cursors
	AccentAlt lipgloss.AdaptiveColor // secondary accent: section headings
	Fg        lipgloss.AdaptiveColor // primary text
	FgMuted   lipgloss.AdaptiveColor // secondary text, labels
	FgSubtle  lipgloss.AdaptiveColor // tertiary text, hints, authors
	SelFg     lipgloss.AdaptiveColor // text on an accent background
	SelBg     lipgloss.AdaptiveColor // selected-row background
	Success   lipgloss.AdaptiveColor // staged / succeeded
	Warning   lipgloss.AdaptiveColor // untracked / needs attention
	Danger    lipgloss.AdaptiveColor // unstaged / failed / destructive
	Meta      lipgloss.AdaptiveColor // hashes, refs, tag names
	Time      lipgloss.AdaptiveColor // dates and relative times
	Author    lipgloss.AdaptiveColor // author names, kinds
	Border    lipgloss.AdaptiveColor // box borders
}

// th is the active theme. The Dark values are the colors gito has always
// shipped (the brand accent #7D56F4 and the #F1C40F / #27AE60 log palette) so
// the common case looks unchanged; the Light values are darker equivalents
// chosen for contrast on a white terminal.
var th = theme{
	Accent:    lipgloss.AdaptiveColor{Light: "#5B3FBF", Dark: "#7D56F4"},
	AccentAlt: lipgloss.AdaptiveColor{Light: "#7D3C98", Dark: "#9B59B6"},
	Fg:        lipgloss.AdaptiveColor{Light: "#1C1C1C", Dark: "#ECECEC"},
	FgMuted:   lipgloss.AdaptiveColor{Light: "#4E4E4E", Dark: "#A9A9A9"},
	FgSubtle:  lipgloss.AdaptiveColor{Light: "#767676", Dark: "#636363"},
	SelFg:     lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#FAFAFA"},
	SelBg:     lipgloss.AdaptiveColor{Light: "#DCD6F7", Dark: "#312E55"},
	Success:   lipgloss.AdaptiveColor{Light: "#1E8449", Dark: "#2ECC71"},
	Warning:   lipgloss.AdaptiveColor{Light: "#B9770E", Dark: "#F39C12"},
	Danger:    lipgloss.AdaptiveColor{Light: "#C0392B", Dark: "#E74C3C"},
	Meta:      lipgloss.AdaptiveColor{Light: "#8A6D00", Dark: "#F1C40F"},
	Time:      lipgloss.AdaptiveColor{Light: "#1D8348", Dark: "#27AE60"},
	Author:    lipgloss.AdaptiveColor{Light: "#767676", Dark: "#636363"},
	Border:    lipgloss.AdaptiveColor{Light: "#5B3FBF", Dark: "#7D56F4"},
}

// ── legacy styles ──────────────────────────────────────────────────────────

// These eight names predate the theme and are referenced from main.go and
// every screen, so they keep both their names and their padding. Only their
// colors moved: each one is now derived from a theme role.
var (
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(th.Accent).
		Padding(0, 1)

	Selected = lipgloss.NewStyle().
			Bold(true).
			Foreground(th.SelFg).
			Background(th.Accent).
			Padding(0, 1)

	Normal = lipgloss.NewStyle().
		Foreground(th.Fg).
		Padding(0, 1)

	Dimmed = lipgloss.NewStyle().
		Foreground(th.FgSubtle).
		Padding(0, 1)

	Success = lipgloss.NewStyle().
		Bold(true).
		Foreground(th.Success)

	Failure = lipgloss.NewStyle().
		Bold(true).
		Foreground(th.Danger)

	Label = lipgloss.NewStyle().
		Bold(true).
		Foreground(th.FgMuted)

	Border = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.Border).
		Padding(0, 1)
)

// ── semantic styles ────────────────────────────────────────────────────────

// The semantic set replaces the per-screen lipgloss literals. None of these
// carries Padding: chrome owns all spacing.
var (
	// Commit / tag / stash list cells.
	Hash       = lipgloss.NewStyle().Foreground(th.Meta)
	Date       = lipgloss.NewStyle().Foreground(th.Time)
	AuthorName = lipgloss.NewStyle().Foreground(th.Author)
	Subject    = lipgloss.NewStyle().Foreground(th.Fg)
	MetaDim    = lipgloss.NewStyle().Foreground(th.FgMuted)

	// Refs: plain, and the base/target pair the diff picker marks up.
	Ref       = lipgloss.NewStyle().Foreground(th.Meta)
	RefBase   = lipgloss.NewStyle().Bold(true).Foreground(th.Accent)
	RefTarget = lipgloss.NewStyle().Bold(true).Foreground(th.Success)

	// Chrome accents.
	Badge       = lipgloss.NewStyle().Bold(true).Foreground(th.SelFg).Background(th.Accent)
	KeyCap      = lipgloss.NewStyle().Bold(true).Foreground(th.Accent)
	KeyDesc     = lipgloss.NewStyle().Foreground(th.FgSubtle)
	SectionHead = lipgloss.NewStyle().Bold(true).Foreground(th.AccentAlt)
	Cursor      = lipgloss.NewStyle().Bold(true).Foreground(th.Accent)
	RowSel      = lipgloss.NewStyle().Foreground(th.Fg).Background(th.SelBg)

	// Working-tree states.
	Staged    = lipgloss.NewStyle().Foreground(th.Success)
	Unstaged  = lipgloss.NewStyle().Foreground(th.Danger)
	Untracked = lipgloss.NewStyle().Foreground(th.Warning)
	Ahead     = lipgloss.NewStyle().Foreground(th.Success)
	Behind    = lipgloss.NewStyle().Foreground(th.Warning)

	// Frame pieces.
	HeaderBar   = lipgloss.NewStyle().Bold(true).Foreground(th.Accent)
	FooterBar   = lipgloss.NewStyle().Foreground(th.FgSubtle)
	OverlayBox  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(th.Border)
	DangerBar   = lipgloss.NewStyle().Bold(true).Foreground(th.SelFg).Background(th.Danger)
	ScrollTrack = lipgloss.NewStyle().Foreground(th.FgSubtle)
	ScrollThumb = lipgloss.NewStyle().Foreground(th.Accent)
)

// ── glyphs ─────────────────────────────────────────────────────────────────

// Glyphs is every non-ASCII character the UI prints, so a terminal that cannot
// render them can be served an ASCII table instead of replacement boxes.
type Glyphs struct {
	Cursor      string // selected-row marker
	Check       string // success mark
	Bang        string // error mark
	Up          string // ahead arrow
	Down        string // behind arrow
	Ellipsis    string // truncation tail
	Rule        string // horizontal rule
	Crumb       string // breadcrumb separator
	Arrow       string // "a -> b"
	ScrollTrack string // scrollbar gutter
	ScrollThumb string // scrollbar position

	// Launcher icons, one per entry in ui.MenuItems. Each ASCII stand-in is
	// exactly one column wide so the menu stays aligned either way.
	IconStatus string
	IconCommit string
	IconLog    string
	IconBranch string
	IconDiff   string
	IconStash  string
	IconTag    string
	IconRemote string
	IconReflog string
	IconBlame  string
}

// UnicodeGlyphs is the preferred table, used whenever the locale looks UTF-8.
var UnicodeGlyphs = Glyphs{
	Cursor:      "\u25b6", // ▶
	Check:       "\u2713", // ✓
	Bang:        "!",
	Up:          "\u2191", // ↑
	Down:        "\u2193", // ↓
	Ellipsis:    "\u2026", // …
	Rule:        "\u2500", // ─
	Crumb:       "\u203a", // ›
	Arrow:       "\u2192", // →
	ScrollTrack: "\u2502", // │
	ScrollThumb: "\u2588", // █

	IconStatus: "\u25cd", // ◍
	IconCommit: "\u270e", // ✎
	IconLog:    "\u2261", // ≡
	IconBranch: "\u2442", // ⑂
	IconDiff:   "\u21c4", // ⇄
	IconStash:  "\u229f", // ⊟
	IconTag:    "\u2302", // ⌂
	IconRemote: "\u2601", // ☁
	IconReflog: "\u21ba", // ↺
	IconBlame:  "\u25ce", // ◎
}

// ASCIIGlyphs is the fallback table for non-UTF-8 locales and for anyone who
// sets GITO_ASCII.
var ASCIIGlyphs = Glyphs{
	Cursor:      ">",
	Check:       "+",
	Bang:        "!",
	Up:          "^",
	Down:        "v",
	Ellipsis:    "...",
	Rule:        "-",
	Crumb:       ">",
	Arrow:       "->",
	ScrollTrack: "|",
	ScrollThumb: "#",

	IconStatus: "o",
	IconCommit: "*",
	IconLog:    "=",
	IconBranch: "Y",
	IconDiff:   "~",
	IconStash:  "[",
	IconTag:    "^",
	IconRemote: "@",
	IconReflog: "<",
	IconBlame:  "b",
}

// G is the active glyph table. It is selected once at package init and can be
// swapped in tests through UseASCII.
var G = detectGlyphs()

// detectGlyphs picks the glyph table from the environment: an explicit
// GITO_ASCII wins, otherwise a UTF-8 looking locale enables Unicode, otherwise
// (C/POSIX, common over SSH) ASCII keeps the output legible.
func detectGlyphs() Glyphs {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("GITO_ASCII"))) {
	case "1", "true", "yes", "y", "on":
		return ASCIIGlyphs
	}
	for _, key := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		v := strings.ToLower(os.Getenv(key))
		if strings.Contains(v, "utf-8") || strings.Contains(v, "utf8") {
			return UnicodeGlyphs
		}
	}
	return ASCIIGlyphs
}

// UseASCII forces the ASCII (or Unicode) glyph table and returns a function
// that restores the previous one. It is the test seam for glyph fallback, so
// tests never have to mutate the process environment.
func UseASCII(ascii bool) func() {
	prev := G
	if ascii {
		G = ASCIIGlyphs
	} else {
		G = UnicodeGlyphs
	}
	return func() { G = prev }
}

// ── width helpers ──────────────────────────────────────────────────────────

// DisplayWidth reports how many terminal columns s occupies, ignoring ANSI
// escape sequences. internal/git returns pre-colored git output and lipgloss
// wraps every cell in escapes, so len() is never the right measure.
func DisplayWidth(s string) int { return ansi.StringWidth(s) }

// Truncate shortens s to at most w columns, appending the Ellipsis glyph when
// anything was cut. It is ANSI-aware: escape sequences are preserved and cost
// no columns.
func Truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if DisplayWidth(s) <= w {
		return s
	}
	tail := G.Ellipsis
	if DisplayWidth(tail) >= w {
		tail = ""
	}
	return ansi.Truncate(s, w, tail)
}

// Pad right-pads s with spaces until it occupies w columns. Strings already at
// or past w are returned unchanged, so callers that need a hard limit should
// Truncate first.
func Pad(s string, w int) string {
	gap := w - DisplayWidth(s)
	if gap <= 0 {
		return s
	}
	return s + strings.Repeat(" ", gap)
}
