package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func enter(t *testing.T, m *Model) {
	t.Helper()
	send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
}

func TestJumpBoxOpensATicket(t *testing.T) {
	m := newModel(t, 110, 30)
	// DEMO-104 is in progress, so the Open queue does not list it.
	if strings.Contains(screen(m), "DEMO-104") {
		t.Fatal("the ticket is on screen before the jump")
	}

	// A bare number is looked up in the default project.
	clickLayer(t, m, idJump)
	if !m.jumping {
		t.Fatal("a click on the box did not give it the keyboard")
	}
	// The keys go to the box: q does not quit and r does not reload.
	typeText(t, m, "104")
	if got := m.jump.Value(); got != "104" {
		t.Fatalf("box holds %q", got)
	}
	checkFrames(t, m, "jump box")
	enter(t, m)
	if m.jumping || len(m.tabs) != 1 || m.activeTab != 0 || m.tabs[0].t.Key != "DEMO-104" {
		t.Fatalf("after a number: jumping=%v tabs=%d active=%d", m.jumping, len(m.tabs), m.activeTab)
	}
	if out := screen(m); !strings.Contains(out, "Webhook retries") || !strings.Contains(out, "move") {
		t.Fatalf("the ticket view is not shown:\n%s", out)
	}

	// A pasted link opens its ticket in a second tab.
	send(t, m, tea.KeyPressMsg{Code: '/', Text: "/"})
	if !m.jumping || m.jump.Value() != "" {
		t.Fatal("the slash key did not open an empty box")
	}
	send(t, m, tea.PasteMsg{Content: "https://example.com/browse/DEMO-111?focusedCommentId=5\n"})
	enter(t, m)
	if len(m.tabs) != 2 || m.activeTab != 1 || m.tabs[1].t.Key != "DEMO-111" {
		t.Fatalf("after a link: tabs=%d active=%d", len(m.tabs), m.activeTab)
	}

	// A key in any case goes back to the tab the ticket already has.
	send(t, m, tea.KeyPressMsg{Code: '/', Text: "/"})
	typeText(t, m, "demo-104")
	enter(t, m)
	if len(m.tabs) != 2 || m.activeTab != 0 {
		t.Fatalf("after a key: tabs=%d active=%d", len(m.tabs), m.activeTab)
	}
}

func TestJumpBoxFailures(t *testing.T) {
	m := newModel(t, 110, 30)

	// Text with no ticket in it is refused in the status bar.
	clickLayer(t, m, idJump)
	typeText(t, m, "hello")
	enter(t, m)
	if len(m.tabs) != 0 || !strings.Contains(screen(m), "no ticket key") {
		t.Fatalf("after bad text: tabs=%d\n%s", len(m.tabs), screen(m))
	}

	// A ticket that does not exist fails as a read and opens no tab.
	clickLayer(t, m, idJump)
	typeText(t, m, "9999")
	enter(t, m)
	if len(m.tabs) != 0 || !strings.Contains(screen(m), "Failed: DEMO-9999") {
		t.Fatalf("after a missing ticket: tabs=%d\n%s", len(m.tabs), screen(m))
	}

	// Esc and a click elsewhere both give the keyboard back.
	clickLayer(t, m, idJump)
	send(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.jumping {
		t.Fatal("esc left the box open")
	}
	clickLayer(t, m, idJump)
	clickLayer(t, m, layerID(idRow, 0))
	if m.jumping || len(m.tabs) != 1 {
		t.Fatalf("after a click on a row: jumping=%v tabs=%d", m.jumping, len(m.tabs))
	}
}

func TestJumpBoxIsAButtonInANarrowPane(t *testing.T) {
	// With no room beside the queue tabs the button is left out, and the
	// slash key still opens the box.
	m := newModel(t, 56, 20)
	m.View()
	if m.comp.GetLayer(idJump) != nil || m.comp.GetLayer(layerID(idQueueTab, 2)) == nil {
		t.Fatal("the button took the room of a queue tab")
	}
	send(t, m, tea.KeyPressMsg{Code: '/', Text: "/"})
	if !m.jumping || !strings.Contains(screen(m), jumpHint) {
		t.Fatal("the slash key did not open the box in a narrow pane")
	}
	send(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})

	send(t, m, tea.WindowSizeMsg{Width: 66, Height: 20})
	if out := screen(m); !strings.Contains(out, "go to") || strings.Contains(out, "go to ticket") {
		t.Fatalf("narrow pane:\n%s", out)
	}
	clickLayer(t, m, idJump)
	if out := screen(m); !strings.Contains(out, jumpHint) {
		t.Fatalf("the open box does not show its hint:\n%s", out)
	}
}
