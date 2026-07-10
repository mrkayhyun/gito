package ui

import "testing"

func TestSubjectLenHint(t *testing.T) {
	cases := []struct {
		name      string
		n         int
		wantLabel string
		wantWarn  bool
	}{
		{"below limit", 32, "32/50", false},
		{"zero", 0, "0/50", false},
		{"just below boundary", 49, "49/50", false},
		{"at boundary", 50, "50/50", false},
		{"just above boundary", 51, "51/50", true},
		{"well above", 72, "72/50", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			label, warn := subjectLenHint(tc.n)
			if label != tc.wantLabel {
				t.Errorf("subjectLenHint(%d) label = %q, want %q", tc.n, label, tc.wantLabel)
			}
			if warn != tc.wantWarn {
				t.Errorf("subjectLenHint(%d) warn = %v, want %v", tc.n, warn, tc.wantWarn)
			}
		})
	}
}
