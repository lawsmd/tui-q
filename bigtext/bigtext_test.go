package bigtext

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestEveryFontDrawsAHeading(t *testing.T) {
	names := Fonts()
	if len(names) == 0 {
		t.Fatal("no fonts")
	}
	for _, name := range names {
		if !Has(name) {
			t.Errorf("Has(%q) is false", name)
		}
		for _, text := range []string{"In Progress", "Open", "Waiting 12"} {
			rows, err := Render(name, text)
			if err != nil {
				t.Errorf("%s: %v", name, err)
				continue
			}
			if len(rows) == 0 || len(rows) > MaxHeight {
				t.Errorf("%s: %d rows for %q", name, len(rows), text)
			}
			for _, row := range rows {
				if lipgloss.Width(row) != lipgloss.Width(rows[0]) {
					t.Errorf("%s: rows of %q differ in width:\n%s", name, text, strings.Join(rows, "\n"))
				}
			}
			t.Logf("%s\n%s", name, strings.Join(rows, "\n"))
		}
	}
}

func TestUnknownFont(t *testing.T) {
	if Has("nonesuch") {
		t.Error("Has finds a font that is not there")
	}
	if _, err := Render("nonesuch", "Open"); err == nil {
		t.Error("an unknown font draws")
	}
}
