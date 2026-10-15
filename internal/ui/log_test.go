package ui

import (
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
