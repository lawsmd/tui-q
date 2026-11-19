package ui

import (
	"fmt"
	"image/color"
	"testing"
	"time"

	"github.com/lawsmd/tui-q/ticket"
)

func TestCommenterColoursFollowFirstComment(t *testing.T) {
	palette := DefaultStyles().Commenters
	now := time.Now()
	// Listed newest first, to show the order of the list does not matter.
	comments := []ticket.Comment{
		{Author: "alex", Created: now},
		{Author: "reporter", Created: now.Add(-time.Hour)},
		{Author: "sam", Created: now.Add(-2 * time.Hour)},
		{Author: "reporter", Created: now.Add(-3 * time.Hour)},
	}
	got := commenterColours(comments, palette)
	want := map[string]color.Color{"reporter": palette[0], "sam": palette[1], "alex": palette[2]}
	if len(got) != len(want) {
		t.Fatalf("%d authors given a colour, want %d", len(got), len(want))
	}
	for author, colour := range want {
		if got[author] != colour {
			t.Errorf("%s has %v, want %v", author, got[author], colour)
		}
	}
}

func TestCommenterColoursRepeatPastThePalette(t *testing.T) {
	palette := DefaultStyles().Commenters
	now := time.Now()
	var comments []ticket.Comment
	for i := range len(palette) + 1 {
		comments = append(comments, ticket.Comment{
			Author:  fmt.Sprintf("user%d", i),
			Created: now.Add(time.Duration(i) * time.Minute),
		})
	}
	got := commenterColours(comments, palette)
	if last := fmt.Sprintf("user%d", len(palette)); got[last] != palette[0] {
		t.Errorf("%s has %v, want the first colour again", last, got[last])
	}
	if len(commenterColours(comments, nil)) != 0 {
		t.Error("an empty palette gave out colours")
	}
}

func TestCommenterPaletteHasNoRepeats(t *testing.T) {
	seen := map[color.Color]bool{}
	for _, c := range DefaultStyles().Commenters {
		if seen[c] {
			t.Errorf("%v is in the palette twice", c)
		}
		seen[c] = true
	}
}
