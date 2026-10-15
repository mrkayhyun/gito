package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestBlameDirectModeLoadsAndQuits(t *testing.T) {
	m := blameModel{selected: "README.md", pane: blamePaneView, direct: true}
	if m.Init() == nil {
		t.Fatal("direct mode should start loading blame immediately")
	}
	updated, _ := m.Update(blameContentMsg{"line"})
	m = updated.(blameModel)
	for _, k := range []tea.KeyMsg{keyMsg("q"), escKey()} {
		_, cmd := m.Update(k)
		if cmd == nil {
			t.Fatalf("%q in direct mode should quit", k.String())
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("%q in direct mode should return tea.Quit", k.String())
		}
	}
}

func TestBlamePickerModeEscReturnsToList(t *testing.T) {
	m := blameModel{files: []string{"README.md"}, selected: "README.md", pane: blamePaneView}
	updated, cmd := m.Update(escKey())
	if updated.(blameModel).pane != blamePanePick || cmd != nil {
		t.Error("esc in picker mode should return to the file list, not quit")
	}
}
