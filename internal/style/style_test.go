package style

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestSetTheme(t *testing.T) {
	defer lipgloss.SetHasDarkBackground(lipgloss.HasDarkBackground())

	if !SetTheme("light") || lipgloss.HasDarkBackground() {
		t.Error(`SetTheme("light") should select the light palette`)
	}
	if !SetTheme(" DARK ") || !lipgloss.HasDarkBackground() {
		t.Error(`SetTheme(" DARK ") should select the dark palette (trimmed, case-insensitive)`)
	}
	for _, v := range []string{"", "auto"} {
		if !SetTheme(v) {
			t.Errorf("SetTheme(%q) should be accepted as auto-detect", v)
		}
	}
	if SetTheme("solarized") {
		t.Error(`SetTheme("solarized") should report an unknown theme`)
	}
}

// Dark values must stay the original gito colors so the default look is
// unchanged for dark-terminal users; light values must differ to be useful.
func TestPaletteLightDiffersFromDark(t *testing.T) {
	for name, c := range map[string]lipgloss.AdaptiveColor{
		"Accent": Accent, "Text": Text, "Yellow": Yellow, "Green": Green,
		"Red": Red, "Blue": Blue, "SelBg": SelBg, "Muted": Muted,
	} {
		if c.Light == "" || c.Dark == "" || c.Light == c.Dark {
			t.Errorf("%s: light %q / dark %q should both be set and differ", name, c.Light, c.Dark)
		}
	}
}
