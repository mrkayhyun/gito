package ui

import "testing"

func TestCommitTypeEnterAdvancesToScope(t *testing.T) {
	m := newCommitModel()
	if m.step != stepType {
		t.Fatalf("new commit model should start at stepType, got %v", m.step)
	}
	updated, _ := m.Update(enterKey())
	if got := updated.(commitModel).step; got != stepScope {
		t.Errorf("enter at stepType should advance to stepScope, got %v", got)
	}
}

func TestCommitEscReturnsToPreviousStep(t *testing.T) {
	// advance type -> scope, then esc back to type.
	m := newCommitModel()
	updated, _ := m.Update(enterKey())
	m2 := updated.(commitModel)
	if m2.step != stepScope {
		t.Fatalf("precondition: expected stepScope, got %v", m2.step)
	}
	updated, _ = m2.Update(escKey())
	if got := updated.(commitModel).step; got != stepType {
		t.Errorf("esc at stepScope should return to stepType, got %v", got)
	}

	// advance to subject, then esc back to scope.
	updated, _ = m2.Update(enterKey())
	m3 := updated.(commitModel)
	if m3.step != stepSubject {
		t.Fatalf("precondition: expected stepSubject, got %v", m3.step)
	}
	updated, _ = m3.Update(escKey())
	if got := updated.(commitModel).step; got != stepScope {
		t.Errorf("esc at stepSubject should return to stepScope, got %v", got)
	}
}

func TestCommitEmptySubjectDoesNotAdvance(t *testing.T) {
	// navigate type -> scope -> subject with an empty subject.
	m := newCommitModel()
	updated, _ := m.Update(enterKey())                    // type -> scope
	updated, _ = updated.(commitModel).Update(enterKey()) // scope -> subject
	m3 := updated.(commitModel)
	if m3.step != stepSubject {
		t.Fatalf("precondition: expected stepSubject, got %v", m3.step)
	}

	// enter with an empty subject must not advance to stepBody.
	updated, _ = m3.Update(enterKey())
	if got := updated.(commitModel).step; got != stepSubject {
		t.Errorf("empty-subject enter should stay at stepSubject, got %v", got)
	}
}
