package ui

import (
	"fmt"
	"strings"

	"gito/internal/i18n"
	"gito/internal/style"
)

// chrome.go is the single owner of gito's screen layout: how tall a body may
// be, what a header line looks like, how key hints are fitted into the width
// available, how a selected row is painted, how a list scrolls, and how the
// pieces are framed into a final View.
//
// Everything here is a pure function of its arguments. There are no git calls
// and no I/O, so each primitive is unit-testable on its own and screens keep
// their "state in the model, side effects in internal/git" shape.

// ── layout ─────────────────────────────────────────────────────────────────

const (
	// Terminal size assumed before the first tea.WindowSizeMsg arrives. A
	// model that renders (or builds a viewport) before that message still gets
	// usable dimensions instead of 0x0.
	defaultCols = 80
	defaultRows = 24

	// Floors. Below these a terminal gets a degraded layout rather than
	// negative widths and panics.
	minCols = 20
	minRows = 6
)

// layout is the terminal geometry a screen renders into.
type layout struct {
	Width  int
	Height int
}

// newLayout returns the pre-WindowSizeMsg default geometry.
func newLayout() layout { return layout{Width: defaultCols, Height: defaultRows} }

// resize applies a new terminal size, clamping it to the floors.
func (l layout) resize(w, h int) layout {
	return layout{Width: max(w, minCols), Height: max(h, minRows)}
}

// norm makes a zero-value layout behave like newLayout, so a model that has
// not been resized yet still measures 80x24. Every primitive below normalizes
// its layout first.
func (l layout) norm() layout {
	if l.Width <= 0 {
		l.Width = defaultCols
	}
	if l.Height <= 0 {
		l.Height = defaultRows
	}
	return layout{Width: max(l.Width, minCols), Height: max(l.Height, minRows)}
}

// bodyRows converts terminal height into the number of rows a body may use.
// This is the only place that arithmetic lives: chromeLines is how many lines
// the caller spends on header, footer, banners and blank separators, counted
// from what it actually renders instead of a per-screen magic constant. The
// result is always at least 1.
func bodyRows(l layout, chromeLines int) int {
	return max(l.norm().Height-chromeLines, 1)
}

// ── header ─────────────────────────────────────────────────────────────────

// header renders exactly one line: "gito <cmd>", an optional breadcrumb after
// the Crumb glyph, and an optional right-aligned meta cell (counts, cursor
// position). The meta cell is sacrificed before the title when the width is
// too small, and the result never contains a newline nor exceeds l.Width.
func header(l layout, cmd, crumb, meta string) string {
	l = l.norm()

	title := "gito"
	if cmd = oneLine(cmd); cmd != "" {
		title += " " + cmd
	}
	crumb, meta = oneLine(crumb), oneLine(meta)

	left := style.HeaderBar.Render(title)
	if crumb != "" {
		left += "  " + style.MetaDim.Render(style.G.Crumb) + " " + style.Subject.Render(crumb)
	}

	leftW, metaW := style.DisplayWidth(left), style.DisplayWidth(meta)
	if meta != "" && leftW+metaW+2 > l.Width {
		// Shrink, then drop, the meta cell before touching the title.
		if room := l.Width - leftW - 2; room >= 4 {
			meta = style.Truncate(meta, room)
			metaW = style.DisplayWidth(meta)
		} else {
			meta, metaW = "", 0
		}
	}
	if metaW == 0 {
		return style.Truncate(left, l.Width)
	}

	gap := l.Width - leftW - metaW
	if gap < 1 {
		left = style.Truncate(left, l.Width-metaW-1)
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + style.MetaDim.Render(meta)
}

// ── key hints ──────────────────────────────────────────────────────────────

// keyHint is one entry of the shared hint vocabulary: the keys that trigger an
// action and a localized description of it.
type keyHint struct {
	Keys string
	Desc string
}

// hintSep separates rendered hints on the footer line.
const hintSep = "   "

// footer renders hints as key badges plus descriptions on a single line that
// fits l.Width. Hints are dropped from the end when they do not fit, and a "?"
// indicator is appended whenever something was dropped or the screen offers a
// help overlay, so the user can always reach the full list.
func footer(l layout, hints []keyHint, hasHelp bool) string {
	l = l.norm()
	if len(hints) == 0 && !hasHelp {
		return ""
	}

	indicator := style.KeyCap.Render("?") + " " + style.FooterBar.Render(i18n.T("key.help"))
	indicatorW := style.DisplayWidth(indicator) + style.DisplayWidth(hintSep)

	rendered := make([]string, 0, len(hints))
	total := 0
	for _, h := range hints {
		s := renderHint(h)
		if s == "" {
			continue
		}
		if len(rendered) > 0 {
			total += style.DisplayWidth(hintSep)
		}
		total += style.DisplayWidth(s)
		rendered = append(rendered, s)
	}

	// Reserve room for the indicator up front when we know it will be shown.
	avail := l.Width
	showIndicator := hasHelp || total > avail
	if showIndicator {
		avail -= indicatorW
	}

	var sb strings.Builder
	used := 0
	dropped := false
	for i, s := range rendered {
		w := style.DisplayWidth(s)
		sep := 0
		if i > 0 {
			sep = style.DisplayWidth(hintSep)
		}
		if used+sep+w > avail {
			if i == 0 {
				// Even the first hint is too wide: keep a truncated version so
				// the footer is never empty.
				s = style.Truncate(s, max(avail, 0))
				sb.WriteString(s)
				used += style.DisplayWidth(s)
			}
			dropped = true
			break
		}
		if i > 0 {
			sb.WriteString(hintSep)
		}
		sb.WriteString(s)
		used += sep + w
	}

	out := sb.String()
	if showIndicator || dropped {
		if out != "" {
			out += hintSep
		}
		out += indicator
	}
	return style.Truncate(out, l.Width)
}

// renderHint styles one hint as a key badge followed by its description.
func renderHint(h keyHint) string {
	keys, desc := oneLine(h.Keys), oneLine(h.Desc)
	switch {
	case keys == "" && desc == "":
		return ""
	case keys == "":
		return style.KeyDesc.Render(desc)
	case desc == "":
		return style.KeyCap.Render(keys)
	}
	return style.KeyCap.Render(keys) + " " + style.KeyDesc.Render(desc)
}

// helpOverlay renders the full hint list as an aligned two-column table inside
// a bordered box, titled with help.keys_title. This is where the long hint
// sentences that used to hard-wrap on an 80-column terminal now live.
func helpOverlay(l layout, hints []keyHint) string {
	l = l.norm()

	keyW := 0
	for _, h := range hints {
		keyW = max(keyW, style.DisplayWidth(oneLine(h.Keys)))
	}

	// 2 border columns plus the 1-column gutter added on each side below.
	inner := max(l.Width-4, minCols-4)

	lines := []string{style.SectionHead.Render(i18n.T("help.keys_title"))}
	for _, h := range hints {
		keys, desc := oneLine(h.Keys), oneLine(h.Desc)
		// style.Pad measures display width, so padding the rendered cell keeps
		// the two columns aligned despite the ANSI escapes.
		line := style.Pad(style.KeyCap.Render(keys), keyW) + "  " + style.KeyDesc.Render(desc)
		lines = append(lines, style.Truncate(line, inner))
	}

	// The gutter is spaces rather than style Padding, because the semantic
	// styles are deliberately padding-free.
	body := 0
	for _, s := range lines {
		body = max(body, style.DisplayWidth(s))
	}
	for i, s := range lines {
		lines[i] = " " + style.Pad(s, body) + " "
	}
	return style.OverlayBox.Render(strings.Join(lines, "\n"))
}

// ── banners ────────────────────────────────────────────────────────────────

// bannerKind selects the glyph and color of a transient message.
type bannerKind int

const (
	bannerInfo bannerKind = iota
	bannerSuccess
	bannerError
)

// banner renders a one-line transient message: the shared replacement for the
// hand-written "! " and "✓ " prefixes each screen used to build itself. An
// empty message renders nothing so callers can inline it unconditionally.
func banner(l layout, kind bannerKind, msg string) string {
	msg = oneLine(msg)
	if msg == "" {
		return ""
	}
	var glyph string
	st := style.MetaDim
	switch kind {
	case bannerSuccess:
		glyph, st = style.G.Check, style.Success
	case bannerError:
		glyph, st = style.G.Bang, style.Failure
	case bannerInfo:
		glyph = style.G.Crumb
	}
	return style.Truncate(st.Render(glyph+" "+msg), l.norm().Width)
}

// confirmBar renders a destructive-action confirmation as a full-width danger
// bar plus the shared y/N line. It is presentation only: the gating logic stays
// in each model, which is what internal/ui/tag_test.go asserts.
func confirmBar(l layout, prompt string) string {
	l = l.norm()
	bar := style.DangerBar.Render(style.Pad(style.Truncate(" "+oneLine(prompt), l.Width), l.Width))
	return bar + "\n" + style.Truncate(style.KeyDesc.Render(i18n.T("common.confirm_yn")), l.Width)
}

// ── rows and scrolling ─────────────────────────────────────────────────────

// row renders one list line across the FULL layout width: the Cursor glyph in
// the gutter when selected, and the selected-row background painted edge to
// edge so the highlight is never ragged. content may already be styled and may
// contain ANSI escapes; it is measured and cut by display width.
func row(l layout, selected bool, content string) string {
	l = l.norm()
	gutter := "  "
	if selected {
		gutter = style.G.Cursor + " "
	}
	line := style.Pad(style.Truncate(gutter+oneLine(content), l.Width), l.Width)
	if selected {
		return style.RowSel.Render(line)
	}
	return line
}

// listWindow is the scrolling state of a list: where the cursor is, which item
// is at the top of the viewport, how many items exist and how many rows are
// visible.
type listWindow struct {
	Cursor int
	Offset int
	Total  int
	Rows   int
}

// clamp keeps Cursor inside the list and scrolls Offset the minimum amount
// needed to keep Cursor visible. It is safe on degenerate input: a negative
// total, zero rows or an out-of-range cursor all come back consistent.
func (w listWindow) clamp() listWindow {
	rows := max(w.Rows, 1)
	total := max(w.Total, 0)

	if total == 0 {
		return listWindow{Cursor: 0, Offset: 0, Total: 0, Rows: w.Rows}
	}

	cursor := min(max(w.Cursor, 0), total-1)
	offset := w.Offset
	switch {
	case total <= rows:
		offset = 0
	default:
		offset = min(max(offset, 0), total-rows)
		if cursor < offset {
			offset = cursor
		}
		if cursor >= offset+rows {
			offset = cursor - rows + 1
		}
	}
	return listWindow{Cursor: cursor, Offset: offset, Total: total, Rows: w.Rows}
}

// bounds returns the half-open range [start, end) of items to render. It
// always lies inside [0, Total].
func (w listWindow) bounds() (int, int) {
	c := w.clamp()
	start := c.Offset
	end := min(start+max(c.Rows, 1), c.Total)
	return start, max(end, start)
}

// position is the "cursor/total" indicator for the header's meta cell. It is
// empty for an empty list, where there is no position to report.
func (w listWindow) position() string {
	c := w.clamp()
	if c.Total == 0 {
		return ""
	}
	return fmt.Sprintf("%d/%d", c.Cursor+1, c.Total)
}

// scrollbar returns the gutter cell for the given visible row (0-based,
// relative to Offset). It is a single space when everything fits, so a short
// list gets no distracting bar. Callers that prefix it to a row must render
// that row into layout{Width: l.Width - 1} so the pair still fits the terminal.
func (w listWindow) scrollbar(visible int) string {
	c := w.clamp()
	rows := max(c.Rows, 1)
	if c.Total <= rows || visible < 0 || visible >= rows {
		return " "
	}
	size := max(rows*rows/c.Total, 1)
	start := min(c.Offset*rows/c.Total, rows-size)
	if visible >= start && visible < start+size {
		return style.ScrollThumb.Render(style.G.ScrollThumb)
	}
	return style.ScrollTrack.Render(style.G.ScrollTrack)
}

// ── frames ─────────────────────────────────────────────────────────────────

// frameFull composes a view for the alt-screen models (status, log, stash,
// tag, remote, diff, reflog, blame). The body is padded with blank lines so the
// footer pins to the bottom row, and cut when it is too tall, so the result is
// always exactly l.Height lines.
func frameFull(l layout, head, body, foot string) string {
	l = l.norm()

	lines := splitLines(head)
	footLines := splitLines(foot)

	avail := max(l.Height-len(lines)-len(footLines), 0)
	bodyLines := splitLines(body)
	if len(bodyLines) > avail {
		bodyLines = bodyLines[:avail]
	}
	lines = append(lines, bodyLines...)
	for i := len(bodyLines); i < avail; i++ {
		lines = append(lines, "")
	}
	lines = append(lines, footLines...)

	if len(lines) > l.Height {
		lines = lines[:l.Height]
	}
	return strings.Join(lines, "\n")
}

// frameInline composes a view for the models that run WITHOUT alt screen
// (menu, branch, commit) and print a result line to stdout after the program
// exits. It never pads to the terminal height, which would scroll the user's
// shell history away.
func frameInline(head, body, foot string) string {
	parts := make([]string, 0, 3)
	for _, s := range []string{head, body, foot} {
		if s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "\n")
}

// ── small helpers ──────────────────────────────────────────────────────────

// oneLine flattens s so a primitive that promises a single line keeps that
// promise even when handed multi-line git output.
func oneLine(s string) string {
	if strings.ContainsAny(s, "\r\n\t") {
		s = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ", "\t", " ").Replace(s)
	}
	return strings.TrimRight(s, " ")
}

// splitLines returns s's lines, or nil for an empty string so that empty
// sections cost no rows.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
