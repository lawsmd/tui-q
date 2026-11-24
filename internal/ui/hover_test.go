package ui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lawsmd/tui-q/ext"
	"github.com/lawsmd/tui-q/internal/ops"
)

// swapped reports whether the layer with the given ID is drawn with its
// colours swapped in the current frame.
func swapped(t *testing.T, m *Model, id string) bool {
	t.Helper()
	m.View()
	l := m.comp.GetLayer(id)
	if l == nil {
		t.Fatalf("no layer %q on screen", id)
	}
	return strings.HasPrefix(l.GetContent(), "\x1b[7m")
}

// moveOver moves the mouse, with no button down, onto the layer with the
// given ID.
func moveOver(t *testing.T, m *Model, id string) {
	t.Helper()
	m.View()
	l := m.comp.GetLayer(id)
	if l == nil {
		t.Fatalf("no layer %q on screen", id)
	}
	send(t, m, tea.MouseMotionMsg{X: l.GetX(), Y: l.GetY()})
}

func TestInvertSwapsEveryStyledRun(t *testing.T) {
	s := DefaultStyles()
	plain := "copy link"
	text := s.Button.Render("copy") + s.Label.Render(" link") + "\n" + plain
	got := invert(text)
	if ansi.Strip(got) != ansi.Strip(text) {
		t.Fatalf("invert changed the text: %q", ansi.Strip(got))
	}
	for i, line := range strings.Split(got, "\n") {
		if !strings.HasPrefix(line, "\x1b[7m") || !strings.HasSuffix(line, "\x1b[m") {
			t.Errorf("line %d is not wrapped in reverse video: %q", i, line)
		}
		// Reverse video is on again after every reset but the last.
		if resets, ons := strings.Count(line, "\x1b[m"), strings.Count(line, "\x1b[7m"); ons != resets {
			t.Errorf("line %d has %d resets and reverse video turned on %d times: %q", i, resets, ons, line)
		}
	}
}

func TestButtonUnderTheMouseSwapsItsColours(t *testing.T) {
	m := newModel(t, 110, 30)
	clickLayer(t, m, layerID(idRow, 0))
	plain := m.View().Content
	if swapped(t, m, idCopyLink) {
		t.Fatal("the button is lit with the mouse elsewhere")
	}

	moveOver(t, m, idCopyLink)
	if !swapped(t, m, idCopyLink) || swapped(t, m, idOpen) {
		t.Fatalf("copy link lit=%v, open lit=%v, want only the one under the mouse", swapped(t, m, idCopyLink), swapped(t, m, idOpen))
	}
	lit := m.View().Content
	if lit == plain || ansi.Strip(lit) != ansi.Strip(plain) {
		t.Fatal("the frame with the button lit must differ from the plain one in colour alone")
	}

	send(t, m, tea.MouseMotionMsg{X: 0, Y: 12})
	if swapped(t, m, idCopyLink) {
		t.Fatal("the button stayed lit after the mouse left it")
	}
	if got := m.View().Content; got != plain {
		t.Fatal("the frame did not go back to what it was")
	}

	// A press on the button leaves it lit: the mouse is still over it.
	clickLayer(t, m, idCopyLink)
	if !swapped(t, m, idCopyLink) {
		t.Fatal("the button went dark when it was clicked")
	}
}

func TestRowsAndTabsAreNotLit(t *testing.T) {
	m := newModel(t, 110, 30)
	plain := m.View().Content
	for _, id := range []string{layerID(idRow, 1), layerID(idQueueTab, 1), idStatus} {
		moveOver(t, m, id)
		if got := m.View().Content; got != plain {
			t.Errorf("the frame changed with the mouse over %q", id)
		}
	}
	// The marks on a row are lit, each on its own.
	moveOver(t, m, layerID(idRowLink, 1))
	if !swapped(t, m, layerID(idRowLink, 1)) || swapped(t, m, layerID(idRowLink, 0)) || swapped(t, m, layerID(idMore, 1)) {
		t.Fatal("the copy mark of the row under the mouse is not the one mark lit")
	}
}

func TestWrappedSidebarButtonLightsAsAWhole(t *testing.T) {
	m := newModel(t, 110, 40)
	m.actions = []ext.Action{{
		Name:    "close the ticket and hand it back to the team that raised it",
		Section: ext.DetailsSection,
		Run:     func(context.Context, ext.Run) (string, error) { return "", nil },
	}}
	clickLayer(t, m, layerID(idRow, 0))
	id := layerID(idAction, 0)
	moveOver(t, m, id)

	// The button is one layer per row, all with the one ID.
	rows := 0
	for _, l := range m.tabs[0].layers(m, m.contentArea()) {
		if l.GetID() != id {
			continue
		}
		rows++
		if got := m.lit(l).GetContent(); !strings.HasPrefix(got, "\x1b[7m") {
			t.Errorf("row %d of the button is not lit: %q", rows, got)
		}
	}
	if rows < 2 {
		t.Fatalf("the button has %d rows, want it wrapped over several", rows)
	}
}

func TestMenuItemUnderTheMouseIsLitAndAHeadingIsNot(t *testing.T) {
	m := newModel(t, 110, 30)
	m.showMenu("Pick", 5, 5, []menuItem{
		{label: "heading"},
		{label: "option", run: func() tea.Cmd { return nil }},
	})
	moveOver(t, m, layerID(idMenuItem, 1))
	if !swapped(t, m, layerID(idMenuItem, 1)) {
		t.Fatal("the item under the mouse is not lit")
	}
	moveOver(t, m, layerID(idMenuItem, 0))
	if swapped(t, m, layerID(idMenuItem, 0)) || swapped(t, m, layerID(idMenuItem, 1)) {
		t.Fatal("a heading is lit, or the item the mouse left still is")
	}
}

func TestNothingIsLitWhileInputIsLocked(t *testing.T) {
	m := newModel(t, 110, 30)
	clickLayer(t, m, layerID(idRow, 0))
	moveOver(t, m, idCopyLink)
	m.start("Loading the moves open to DEMO-101", ops.Wait, idle)
	if swapped(t, m, idCopyLink) {
		t.Fatal("a button is lit while input is locked")
	}
}

func TestWhatIsLitFollowsTheViewUnderAStillMouse(t *testing.T) {
	m := newModel(t, 110, 30)
	m.View()
	link := m.comp.GetLayer(layerID(idRowLink, 0))
	x, y := link.GetX(), link.GetY()
	send(t, m, tea.MouseMotionMsg{X: x, Y: y})
	if !swapped(t, m, layerID(idRowLink, 0)) {
		t.Fatal("the mark under the mouse is not lit")
	}
	// A menu opens over the mark: the mark is under the menu now.
	m.showMenu("Pick", x-2, y-2, []menuItem{
		{label: "one", run: func() tea.Cmd { return nil }},
		{label: "two", run: func() tea.Cmd { return nil }},
		{label: "three", run: func() tea.Cmd { return nil }},
	})
	if swapped(t, m, layerID(idRowLink, 0)) {
		t.Fatal("a mark under an open menu is lit")
	}
	if got := m.comp.Hit(x, y).ID(); got != m.hover {
		t.Fatalf("hover = %q with the mouse over %q", m.hover, got)
	}
}

func TestFirstForeground(t *testing.T) {
	for in, want := range map[string]string{
		"plain":                             "",
		"\x1b[38;5;111mx\x1b[m":             "38;5;111",
		"\x1b[38;2;192;202;245;48;2;1;2;3m": "38;2;192;202;245",
		// The parameters of a background are not read as a colour for text.
		"\x1b[48;2;38;2;3mx\x1b[34mb": "34",
		"\x1b[1;48;5;38;97mx":         "97",
	} {
		if got := firstForeground(in); got != want {
			t.Errorf("firstForeground(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLitButtonIsOneBlockOfColour(t *testing.T) {
	s := DefaultStyles()
	got := invert(s.Button.Render("copy link"))
	fg := firstForeground(s.Button.Render("copy link"))
	// The padding has no colour for text of its own, and takes the button's.
	for _, part := range strings.Split(strings.TrimSuffix(got, "\x1b[m"), "\x1b[m") {
		if !strings.HasPrefix(part, "\x1b[7m\x1b["+fg+"m") {
			t.Errorf("a run of the lit button does not start from the button's text colour: %q", part)
		}
	}
}
