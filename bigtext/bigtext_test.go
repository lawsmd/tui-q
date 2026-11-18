package bigtext

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestGlyphsAreSixEvenLines(t *testing.T) {
	for r, g := range glyphs {
		for _, l := range g {
			if len(l) != len(g[0]) || strings.Trim(l, ".#") != "" {
				t.Errorf("%q has the line %q", r, l)
			}
		}
	}
}

func TestRender(t *testing.T) {
	for _, text := range []string{"In Progress", "Open", "Waiting 12"} {
		rows, err := Render(Block, text)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != Height {
			t.Errorf("%d rows for %q", len(rows), text)
		}
		for _, row := range rows {
			if lipgloss.Width(row) != lipgloss.Width(rows[0]) {
				t.Errorf("rows of %q differ in width", text)
			}
		}
		t.Logf("\n%s", strings.Join(rows, "\n"))
	}
	if rows, _ := Render(Block, "I"); strings.Join(rows, "|") != "▀█▀| █ |▄█▄" {
		t.Errorf("I is drawn as %q", rows)
	}
	if _, err := Render(Block, "né"); err == nil {
		t.Error("a character with no shape draws")
	}
	if _, err := Render("nonesuch", "Open"); err == nil || Has("nonesuch") {
		t.Error("an unknown font draws")
	}
}
