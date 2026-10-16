package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mrkayhyun/gito/internal/git"
	"github.com/mrkayhyun/gito/internal/i18n"
)

func sampleCommits() []git.CommitEntry {
	return []git.CommitEntry{
		{Hash: "aaaa1111", Short: "aaaa111", Subject: "feat(log): add search", Author: "Alice"},
		{Hash: "bbbb2222", Short: "bbbb222", Subject: "fix: crash on empty repo", Author: "Bob"},
		{Hash: "cccc3333", Short: "cccc333", Subject: "docs: README", Author: "Alice"},
	}
}

func TestMatchCommit(t *testing.T) {
	c := sampleCommits()[0]
	cases := []struct {
		query string
		want  bool
	}{
		{"", true},
		{"SEARCH", true},       // case-insensitive subject
		{"alice", true},        // author
		{"aaaa1", true},        // hash prefix
		{"1111", false},        // hash is prefix-only, not substring
		{"feat alice", true},   // all terms must match
		{"feat bob", false},    // one term misses
		{"  log   add ", true}, // extra whitespace ignored
	}
	for _, tc := range cases {
		if got := matchCommit(c, tc.query); got != tc.want {
			t.Errorf("matchCommit(%q) = %v, want %v", tc.query, got, tc.want)
		}
	}
}

func typeInto(t *testing.T, m logModel, s string) logModel {
	t.Helper()
	for _, r := range s {
		updated, _ := m.Update(keyMsg(string(r)))
		m = updated.(logModel)
	}
	return m
}

func TestLogSearchFiltersAndResetsCursor(t *testing.T) {
	m := newLogModel(sampleCommits())
	updated, _ := m.Update(keyMsg("j"))
	m = updated.(logModel)
	if m.cursor != 1 {
		t.Fatalf("precondition: cursor = %d, want 1", m.cursor)
	}

	updated, _ = m.Update(keyMsg("/"))
	m = updated.(logModel)
	if !m.searching {
		t.Fatal("/ should focus the search input")
	}
	// j/k must be typed into the query, not move the cursor.
	m = typeInto(t, m, "alice")
	if got := len(m.filtered()); got != 2 {
		t.Fatalf("filter 'alice' matched %d commits, want 2", got)
	}
	if m.cursor != 0 {
		t.Errorf("cursor should reset to 0 when the query changes, got %d", m.cursor)
	}

	// enter keeps the filter and returns to navigation.
	updated, _ = m.Update(enterKey())
	m = updated.(logModel)
	if m.searching || m.search.Value() != "alice" {
		t.Fatalf("enter should keep filter and leave search mode: searching=%v value=%q", m.searching, m.search.Value())
	}
	updated, _ = m.Update(keyMsg("j"))
	m = updated.(logModel)
	if m.cursor != 1 {
		t.Errorf("j should move within filtered list, cursor = %d", m.cursor)
	}
	updated, _ = m.Update(keyMsg("j"))
	m = updated.(logModel)
	if m.cursor != 1 {
		t.Errorf("cursor should stop at the last filtered row, got %d", m.cursor)
	}

	// enter opens the detail of the filtered selection, not the unfiltered row.
	updated, cmd := m.Update(enterKey())
	m = updated.(logModel)
	if m.pane != paneDetail || cmd == nil {
		t.Fatal("enter should open detail pane")
	}
	if got := m.filtered()[m.cursor].Hash; got != "cccc3333" {
		t.Errorf("detail target = %s, want cccc3333", got)
	}
}

func TestLogEscClearsFilterBeforeQuitting(t *testing.T) {
	m := newLogModel(sampleCommits())
	updated, _ := m.Update(keyMsg("/"))
	m = typeInto(t, updated.(logModel), "bob")
	updated, _ = m.Update(enterKey())
	m = updated.(logModel)

	updated, cmd := m.Update(escKey())
	m = updated.(logModel)
	if cmd != nil {
		t.Fatal("first esc with an active filter should not quit")
	}
	if m.search.Value() != "" || len(m.filtered()) != 3 {
		t.Fatalf("esc should clear the filter, value=%q matched=%d", m.search.Value(), len(m.filtered()))
	}

	_, cmd = m.Update(escKey())
	if cmd == nil {
		t.Fatal("esc without a filter should quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("esc without a filter should return tea.Quit")
	}
}

func TestLogEscInSearchModeClears(t *testing.T) {
	m := newLogModel(sampleCommits())
	updated, _ := m.Update(keyMsg("/"))
	m = typeInto(t, updated.(logModel), "zzz")
	if len(m.filtered()) != 0 {
		t.Fatal("precondition: 'zzz' should match nothing")
	}
	// enter on an empty result must not panic or open detail.
	updated, _ = m.Update(enterKey())
	m = updated.(logModel)
	updated, _ = m.Update(enterKey())
	if updated.(logModel).pane != paneList {
		t.Fatal("enter with no matches should stay on the list")
	}
	updated, _ = m.Update(keyMsg("/"))
	updated, _ = updated.(logModel).Update(escKey())
	m = updated.(logModel)
	if m.searching || m.search.Value() != "" {
		t.Errorf("esc in search mode should clear and blur: searching=%v value=%q", m.searching, m.search.Value())
	}
}

func TestLogViewEmptyMatch(t *testing.T) {
	m := newLogModel(sampleCommits())
	m.search.SetValue("nothing-matches")
	m.cursor = 0
	_ = m.View() // must not index out of range
	updated, _ := m.Update(keyMsg("G"))
	if c := updated.(logModel).cursor; c != 0 {
		t.Errorf("G on empty result should keep cursor at 0, got %d", c)
	}
}

func TestLogDeepSearchOnlyWhenTruncated(t *testing.T) {
	m := newLogModel(sampleCommits())
	updated, _ := m.Update(keyMsg("/"))
	m = typeInto(t, updated.(logModel), "alice")
	_, cmd := m.Update(enterKey())
	if cmd != nil {
		t.Error("enter should not search history when everything is already loaded")
	}
}

func TestLogDeepSearchAppliesAndExpires(t *testing.T) {
	m := newLogModel(sampleCommits())
	m.truncated = true
	updated, _ := m.Update(keyMsg("/"))
	m = typeInto(t, updated.(logModel), "old")
	updated, cmd := m.Update(enterKey())
	m = updated.(logModel)
	if cmd == nil || m.deepQuery != "old" || !m.deepPending() {
		t.Fatalf("enter on a truncated log should start a history search: cmd=%v query=%q", cmd != nil, m.deepQuery)
	}
	if !strings.Contains(m.View(), i18n.T("log.searching_history")) {
		t.Error("view should show that the history search is running")
	}

	// A stale result for a different query is ignored.
	updated, _ = m.Update(deepSearchMsg{query: "other", results: sampleCommits()})
	m = updated.(logModel)
	if !m.deepPending() {
		t.Fatal("a stale result must not complete the current search")
	}

	old := git.CommitEntry{Hash: "dddd4444", Short: "dddd444", Subject: "old commit beyond the first page", Author: "Carol"}
	updated, _ = m.Update(deepSearchMsg{query: "old", results: []git.CommitEntry{old}})
	m = updated.(logModel)
	if got := m.filtered(); len(got) != 1 || got[0].Hash != old.Hash {
		t.Fatalf("filtered should show full-history results, got %+v", got)
	}

	// Editing the query drops back to local filtering over loaded commits.
	updated, _ = m.Update(keyMsg("/"))
	m = typeInto(t, updated.(logModel), "x")
	if got := len(m.filtered()); got != 0 {
		t.Errorf("after editing the query, local filter should apply; got %d results", got)
	}
}

func TestLogDetailKeepsCommitWhenResultsChange(t *testing.T) {
	m := newLogModel(sampleCommits())
	m.truncated = true
	m.deepQuery = "alice" // a search is in flight
	m.search.SetValue("alice")
	updated, _ := m.Update(enterKey())
	m = updated.(logModel)
	want := m.detail.Hash
	if m.pane != paneDetail || want == "" {
		t.Fatal("precondition: detail pane open")
	}
	updated, _ = m.Update(deepSearchMsg{query: "alice", results: nil})
	m = updated.(logModel)
	if m.detail.Hash != want {
		t.Errorf("detail commit changed under the user: %s -> %s", want, m.detail.Hash)
	}
	_ = m.View() // must not index into the (now empty) result list
}

func manyCommits(n, from int) []git.CommitEntry {
	out := make([]git.CommitEntry, n)
	for i := range out {
		h := fmt.Sprintf("%08x", from+i)
		out[i] = git.CommitEntry{Hash: h, Short: h[:7], Subject: fmt.Sprintf("c%d", from+i), Author: "A"}
	}
	return out
}

func TestLogEditingQueryCancelsPendingSearch(t *testing.T) {
	m := newLogModel(sampleCommits())
	m.truncated = true
	updated, _ := m.Update(keyMsg("/"))
	m = typeInto(t, updated.(logModel), "old")
	updated, _ = m.Update(enterKey())
	m = updated.(logModel)
	if m.deepCancel == nil {
		t.Fatal("precondition: a history search should be running")
	}
	cancelled := false
	m.deepCancel = func() { cancelled = true }

	updated, _ = m.Update(keyMsg("/"))
	m = typeInto(t, updated.(logModel), "x")
	if !cancelled || m.deepCancel != nil || m.deepQuery != "" {
		t.Errorf("editing the query should cancel the search: cancelled=%v query=%q", cancelled, m.deepQuery)
	}
}

func TestLogQuitCancelsPendingSearch(t *testing.T) {
	m := newLogModel(sampleCommits())
	cancelled := false
	m.deepQuery, m.deepCancel = "q", func() { cancelled = true }
	if _, cmd := m.Update(keyMsg("q")); cmd == nil || !cancelled {
		t.Error("quitting should cancel a running history search")
	}
}

func TestLogNewSearchCancelsPrevious(t *testing.T) {
	m := newLogModel(sampleCommits())
	m.truncated = true
	first := false
	m.deepQuery, m.deepCancel = "first", func() { first = true }
	m.search.SetValue("second")
	m.searching = true
	updated, cmd := m.Update(enterKey())
	m = updated.(logModel)
	if !first || cmd == nil || m.deepQuery != "second" {
		t.Errorf("a new search should cancel the old one and start: cancelled=%v query=%q", first, m.deepQuery)
	}
	m.cancelDeep() // don't leak the real search started above
}

func TestLogCancelledResultIgnored(t *testing.T) {
	m := newLogModel(sampleCommits())
	m.deepQuery = "q"
	updated, _ := m.Update(deepSearchMsg{query: "q", err: context.Canceled})
	if updated.(logModel).deepDone {
		t.Error("a cancelled search must not be shown as finished")
	}
}

func TestLogLoadsNextPageAtEnd(t *testing.T) {
	m := newLogModel(manyCommits(logLimit, 0))
	m.truncated = true
	updated, cmd := m.Update(keyMsg("G"))
	m = updated.(logModel)
	if cmd == nil || !m.loadingMore {
		t.Fatal("reaching the last loaded commit should load the next page")
	}
	// Further keys at the end must not queue duplicate loads.
	if _, cmd := m.Update(keyMsg("j")); cmd != nil {
		t.Error("only one page load may be in flight")
	}
	// A stale page (wrong offset) is ignored.
	updated, _ = m.Update(logPageMsg{skip: 123, entries: manyCommits(3, 9000)})
	if len(updated.(logModel).commits) != logLimit {
		t.Fatal("a page for the wrong offset must be ignored")
	}
	updated, _ = m.Update(logPageMsg{skip: logLimit, entries: manyCommits(10, logLimit)})
	m = updated.(logModel)
	if len(m.commits) != logLimit+10 || m.truncated || m.loadingMore {
		t.Fatalf("after a short last page: len=%d truncated=%v loading=%v", len(m.commits), m.truncated, m.loadingMore)
	}
	updated, _ = m.Update(keyMsg("j"))
	if c := updated.(logModel).cursor; c != logLimit {
		t.Errorf("j should continue into the new page, cursor=%d", c)
	}
}

func TestLogNoPagingWhileFiltered(t *testing.T) {
	m := newLogModel(manyCommits(logLimit, 0))
	m.truncated = true
	m.search.SetValue("c1")
	if _, cmd := m.Update(keyMsg("G")); cmd != nil {
		t.Error("paging must not run while a filter is active (enter searches history instead)")
	}
}

func TestLogPageErrorStopsPaging(t *testing.T) {
	m := newLogModel(manyCommits(logLimit, 0))
	m.truncated, m.loadingMore = true, true
	updated, _ := m.Update(logPageMsg{skip: logLimit, err: errors.New("boom")})
	m = updated.(logModel)
	if m.truncated || m.loadingMore || !strings.Contains(m.View(), "boom") {
		t.Error("a page error should stop paging and be shown")
	}
}

func TestLogViewFitsTerminalHeight(t *testing.T) {
	for _, filtered := range []bool{false, true} {
		m := newLogModel(manyCommits(50, 0))
		updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 12})
		m = updated.(logModel)
		if filtered {
			m.search.SetValue("c")
		}
		if lines := strings.Count(m.View(), "\n") + 1; lines > 12 {
			t.Errorf("filtered=%v: view is %d lines in a 12-line terminal; the title scrolls off", filtered, lines)
		}
	}
}
