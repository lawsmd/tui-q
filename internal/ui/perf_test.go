package ui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/lawsmd/tui-q/internal/ops"
)

// forgetRendered empties the store of rendered Markdown.
func forgetRendered() {
	renderLock.Lock()
	defer renderLock.Unlock()
	clear(rendered)
}

// isRendered reports whether a text has been rendered at a width.
func isRendered(text string, w int) bool {
	renderLock.Lock()
	defer renderLock.Unlock()
	_, ok := rendered[renderKey{w, text}]
	return ok
}

func TestAMouseMoveThatChangesNothingDrawsNothing(t *testing.T) {
	m := newModel(t, 110, 30)
	clickLayer(t, m, layerID(idRow, 0))
	send(t, m, tea.MouseMotionMsg{X: 40, Y: 20})
	first := m.View()
	frame := m.comp

	// The mouse crosses text, where nothing is lit.
	for x := 41; x < 60; x++ {
		send(t, m, tea.MouseMotionMsg{X: x, Y: 20})
		if got := m.View(); m.comp != frame || got.Content != first.Content {
			t.Fatalf("the frame was drawn again for a move to column %d", x)
		}
	}

	// It reaches a button, which is lit: a new frame.
	moveOver(t, m, idCopyLink)
	if m.View(); m.comp == frame || !swapped(t, m, idCopyLink) {
		t.Fatal("the frame was not drawn again when the mouse reached a button")
	}

	// Any other message draws a frame, though the mouse has not moved.
	send(t, m, tea.MouseMotionMsg{X: 40, Y: 20})
	m.View()
	frame = m.comp
	send(t, m, tea.MouseMotionMsg{X: 41, Y: 20})
	m.start("Reading the notes", ops.Background, idle)
	send(t, m, m.spin.Tick())
	if m.View(); m.comp == frame {
		t.Fatal("the frame was not drawn again for a message that was not a move of the mouse")
	}
	if got := screen(m); !strings.Contains(got, "Reading the notes") {
		t.Fatalf("the frame drawn after it does not show what changed:\n%s", got)
	}
}

func TestRenderedMarkdownIsKeptAndNotShared(t *testing.T) {
	forgetRendered()
	s := DefaultStyles()
	text := "A **bold** start.\n\n- one\n- two"
	first, _ := s.markdownFiles(text, 40)
	if len(first) == 0 || !isRendered(text, 40) {
		t.Fatalf("rendered %d lines, kept=%v", len(first), isRendered(text, 40))
	}
	want := slices.Clone(first)
	// The caller cuts and pads the lines it is given.
	for i := range first {
		first[i] = "changed"
	}
	if again, _ := s.markdownFiles(text, 40); !slices.Equal(again, want) {
		t.Fatalf("a second render gave %q, want %q", again, want)
	}
	if isRendered(text, 41) {
		t.Fatal("a render at one width is kept for another")
	}
}

func TestReadingATicketRendersItBeforeItIsDrawn(t *testing.T) {
	m := newModel(t, 110, 40)
	clickLayer(t, m, layerID(idRow, 0))
	m.View()
	w := int(m.drawnWidth.Load())
	if w < 1 || w != m.tabs[0].mainWidth {
		t.Fatalf("the width drawn at is noted as %d, and the column is %d wide", w, m.tabs[0].mainWidth)
	}

	// The ticket is read again: the command renders what it read.
	forgetRendered()
	tk := m.tabs[0].t
	tk.Body = "The export **drops rows**."
	m.prewarm(tk)
	if !isRendered(tk.Body, w) {
		t.Fatal("the body was not rendered by the command that read the ticket")
	}
	for i, c := range tk.Comments {
		if !isRendered(c.Body, w-4+markdownMargin) {
			t.Fatalf("comment %d was not rendered by the command that read the ticket", i)
		}
	}
}

// A message that changes the view may be handled with no frame drawn after
// it. A move of the mouse that follows must not bring the old frame back.
func TestAMouseMoveDoesNotBringBackAnOlderFrame(t *testing.T) {
	m := newModel(t, 110, 30)
	// clickLayer draws before the click and not after it.
	clickLayer(t, m, layerID(idRow, 0))
	send(t, m, tea.MouseMotionMsg{X: 40, Y: 20})
	if got := screen(m); !strings.Contains(got, "copy link") {
		t.Fatalf("the frame after the move is not the ticket that was opened:\n%s", got)
	}
}
