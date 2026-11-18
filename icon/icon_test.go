package icon

import (
	"testing"

	"charm.land/lipgloss/v2"
)

func TestEveryGlyphIsOneCell(t *testing.T) {
	seen := map[string]string{}
	for _, name := range Names() {
		g, _ := Glyph(name)
		if w := lipgloss.Width(g); w != 1 {
			t.Errorf("%s (%q) is %d cells wide", name, g, w)
		}
		if other, ok := seen[g]; ok {
			t.Errorf("%s and %s share %q", name, other, g)
		}
		seen[g] = name
	}
	if lipgloss.Width(Other) != 1 {
		t.Errorf("Other is not one cell wide")
	}
}
