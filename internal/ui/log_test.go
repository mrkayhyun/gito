package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mrkayhyun/gito/internal/git"
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
