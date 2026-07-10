package ui

import "testing"

func TestMenuMovementClamps(t *testing.T) {
	// up at the top stays at 0.
	m := menuModel{cursor: 0}
	updated, _ := m.Update(keyMsg("k"))
	if got := updated.(menuModel).cursor; got != 0 {
		t.Errorf("up at top should clamp to 0, got %d", got)
	}

	// down past the last item stays at the last index.
	last := len(MenuItems) - 1
	m = menuModel{cursor: last}
	updated, _ = m.Update(keyMsg("j"))
	if got := updated.(menuModel).cursor; got != last {
		t.Errorf("down at bottom should clamp to %d, got %d", last, got)
	}

	// down from the middle advances by one.
	m = menuModel{cursor: 0}
	updated, _ = m.Update(keyMsg("j"))
	if got := updated.(menuModel).cursor; got != 1 {
		t.Errorf("down should advance to 1, got %d", got)
	}
}

func TestMenuJumpToEnds(t *testing.T) {
	m := menuModel{cursor: 3}
	updated, _ := m.Update(keyMsg("g"))
	if got := updated.(menuModel).cursor; got != 0 {
		t.Errorf("'g' should jump to first, got %d", got)
	}

	m = menuModel{cursor: 0}
	updated, _ = m.Update(keyMsg("G"))
	if got := updated.(menuModel).cursor; got != len(MenuItems)-1 {
		t.Errorf("'G' should jump to last (%d), got %d", len(MenuItems)-1, got)
	}
}

func TestMenuEnterSelection(t *testing.T) {
	m := menuModel{cursor: 2}
	updated, _ := m.Update(enterKey())
	final := updated.(menuModel)
	if final.chosen != MenuItems[2].Key {
		t.Errorf("enter should choose MenuItems[2].Key = %q, got %q", MenuItems[2].Key, final.chosen)
	}
	if final.quit {
		t.Errorf("enter selection should not set quit")
	}
}

func TestMenuNumberSelection(t *testing.T) {
	// '1' selects the first item.
	m := menuModel{}
	updated, _ := m.Update(keyMsg("1"))
	if got := updated.(menuModel).chosen; got != MenuItems[0].Key {
		t.Errorf("'1' should select MenuItems[0].Key = %q, got %q", MenuItems[0].Key, got)
	}

	// '3' selects the third item regardless of cursor position.
	m = menuModel{cursor: 5}
	updated, _ = m.Update(keyMsg("3"))
	if got := updated.(menuModel).chosen; got != MenuItems[2].Key {
		t.Errorf("'3' should select MenuItems[2].Key = %q, got %q", MenuItems[2].Key, got)
	}
}

func TestMenuZeroSelectsTenthItem(t *testing.T) {
	if len(MenuItems) < 10 {
		t.Skipf("MenuItems has %d entries, need at least 10 for the '0' shortcut", len(MenuItems))
	}
	m := menuModel{}
	updated, _ := m.Update(keyMsg("0"))
	final := updated.(menuModel)
	if final.chosen != MenuItems[9].Key {
		t.Errorf("'0' should select MenuItems[9].Key = %q, got %q", MenuItems[9].Key, final.chosen)
	}
	if final.chosen != "blame" {
		t.Errorf("'0' should select the 10th command \"blame\", got %q", final.chosen)
	}
	if final.quit {
		t.Errorf("'0' selection should not set quit")
	}
}

func TestMenuQuitKeys(t *testing.T) {
	for _, k := range []string{"q", "esc", "ctrl+c"} {
		var msg = keyMsg(k)
		switch k {
		case "esc":
			msg = escKey()
		case "ctrl+c":
			msg = ctrlCKey()
		}
		m := menuModel{cursor: 1}
		updated, _ := m.Update(msg)
		final := updated.(menuModel)
		if !final.quit {
			t.Errorf("%q should set quit=true", k)
		}
		if final.chosen != "" {
			t.Errorf("%q should not set chosen, got %q", k, final.chosen)
		}
	}
}
