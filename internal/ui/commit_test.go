package ui

import "testing"

// driveToConfirm advances a fresh commit model to stepConfirm as a pure state
// transition (no git calls): type -> scope -> subject (non-empty) -> body ->
// confirm. The subject is made non-empty by feeding rune keys through Update so
// the empty-subject guard does not block advancing.
func driveToConfirm(t *testing.T) commitModel {
	t.Helper()
	m := newCommitModel()

	updated, _ := m.Update(enterKey())                    // stepType -> stepScope
	updated, _ = updated.(commitModel).Update(enterKey()) // stepScope -> stepSubject

	sm := updated.(commitModel)
	if sm.step != stepSubject {
		t.Fatalf("precondition: expected stepSubject, got %v", sm.step)
	}

	// type a non-empty subject so the guard allows advancing.
	updated, _ = sm.Update(keyMsg("fix bug"))
	updated, _ = updated.(commitModel).Update(enterKey()) // stepSubject -> stepBody

	bm := updated.(commitModel)
	if bm.step != stepBody {
		t.Fatalf("precondition: expected stepBody, got %v", bm.step)
	}

	updated, _ = bm.Update(enterKey()) // stepBody -> stepConfirm
	cm := updated.(commitModel)
	if cm.step != stepConfirm {
		t.Fatalf("precondition: expected stepConfirm, got %v", cm.step)
	}
	return cm
}

func TestCommitConfirmEscReturnsToBody(t *testing.T) {
	cm := driveToConfirm(t)

	updated, _ := cm.Update(escKey())
	if got := updated.(commitModel).step; got != stepBody {
		t.Errorf("esc at stepConfirm should return to stepBody, got %v", got)
	}
}
