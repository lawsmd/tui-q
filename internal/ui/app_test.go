package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lawsmd/tui-q/profile"
	"github.com/lawsmd/tui-q/provider/fake"
)

const testProfile = `
[provider]
kind = "fake"

[[queue]]
name = "Open"
query = "status=Open"
columns = ["key", "title", "type", "priority", "labels", "created"]

[[queue]]
name = "All"
query = ""

[[queue]]
name = "Board"
query = ""
layout = "board"
lanes = ["Open", "In progress", "Waiting", "Done"]

[ticket]
sidebar = ["status", "type", "assignee", "priority", "labels", "region"]
comments = true
`

// drain runs a command and everything it leads to, feeding each message
// back into the model. Spinner ticks are dropped so the loop ends.
func drain(t *testing.T, m *Model, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	msg := cmd()
	// A text field's cursor blinks for ever; drop its ticks too.
	if strings.Contains(fmt.Sprintf("%T", msg), "cursor.") {
		return
	}
	switch msg := msg.(type) {
	case nil, spinner.TickMsg:
	case tea.BatchMsg:
		for _, c := range msg {
			drain(t, m, c)
		}
	default:
		_, next := m.Update(msg)
		drain(t, m, next)
	}
}

func send(t *testing.T, m *Model, msg tea.Msg) {
	t.Helper()
	_, cmd := m.Update(msg)
	drain(t, m, cmd)
}

func newModel(t *testing.T, w, h int) *Model {
	t.Helper()
	prof, err := profile.Parse(testProfile)
	if err != nil {
		t.Fatal(err)
	}
	m := New(prof, fake.New(0), Options{})
	send(t, m, tea.WindowSizeMsg{Width: w, Height: h})
	drain(t, m, m.Init())
	return m
}

func screen(m *Model) string { return ansi.Strip(m.View().Content) }

// clickLayer presses and releases the left button on the first cell of the
// layer with the given ID in the current frame.
func clickLayer(t *testing.T, m *Model, id string) {
	t.Helper()
	m.View()
	layer := m.comp.GetLayer(id)
	if layer == nil {
		t.Fatalf("no layer %q on screen", id)
	}
	x, y := layer.GetX(), layer.GetY()
	if got := m.comp.Hit(x, y).ID(); got != id {
		t.Fatalf("cell %d,%d hits %q, want %q", x, y, got, id)
	}
	send(t, m, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	send(t, m, tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft})
}

func TestQueueLoadsAndDraws(t *testing.T) {
	m := newModel(t, 110, 30)
	out := screen(m)
	for _, want := range []string{"Open (5)", "All (12)", "DEMO-101", "refresh", "5 tickets"} {
		if !strings.Contains(out, want) {
			t.Errorf("screen lacks %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "DEMO-104") {
		t.Error("an In progress ticket is shown in the Open queue")
	}
}

func TestClickRowOpensTicketTab(t *testing.T) {
	m := newModel(t, 110, 30)
	clickLayer(t, m, layerID(idRow, 1))
	if len(m.tabs) != 1 || m.activeTab != 0 {
		t.Fatalf("tabs=%d activeTab=%d, want one tab showing", len(m.tabs), m.activeTab)
	}
	if m.tabs[0].loading {
		t.Fatal("ticket still marked loading after the load finished")
	}
	out := screen(m)
	for _, want := range []string{"DEMO-102", "open in browser", "copy link", "Steps to reproduce", "Comments", "Status"} {
		if !strings.Contains(out, want) {
			t.Errorf("ticket view lacks %q\n%s", want, out)
		}
	}

	// Clicking the same row again reuses the tab.
	clickLayer(t, m, layerID(idQueueTab, 0))
	if m.activeTab != -1 {
		t.Fatal("clicking a queue tab did not return to the queue")
	}
	clickLayer(t, m, layerID(idRow, 1))
	if len(m.tabs) != 1 {
		t.Fatalf("reopening made %d tabs, want 1", len(m.tabs))
	}

	clickLayer(t, m, layerID(idTabClose, 0))
	if len(m.tabs) != 0 || m.activeTab != -1 {
		t.Fatalf("after close: tabs=%d activeTab=%d", len(m.tabs), m.activeTab)
	}
}

func TestSwitchQueueTab(t *testing.T) {
	m := newModel(t, 110, 30)
	clickLayer(t, m, layerID(idQueueTab, 1))
	if m.activeQueue != 1 {
		t.Fatalf("activeQueue=%d, want 1", m.activeQueue)
	}
	if !strings.Contains(screen(m), "DEMO-112") {
		t.Error("the All queue does not show its last ticket")
	}
}

func TestWheelScrollsAndClamps(t *testing.T) {
	m := newModel(t, 110, 11) // 6 table rows for 12 tickets
	clickLayer(t, m, layerID(idQueueTab, 1))
	for i := 0; i < 10; i++ {
		send(t, m, tea.MouseWheelMsg{X: 5, Y: 5, Button: tea.MouseWheelDown})
		m.View()
	}
	q := m.queues[1]
	if want := len(q.tickets) - 6; q.offset != want {
		t.Fatalf("offset=%d after scrolling past the end, want %d", q.offset, want)
	}
	if !strings.Contains(screen(m), "DEMO-112") {
		t.Error("last ticket not visible at the bottom")
	}
}

// checkFrames draws the model at a spread of pane sizes and fails when a
// frame is not exactly the size of the pane.
func checkFrames(t *testing.T, m *Model, what string) {
	t.Helper()
	for w := minWidth; w <= 160; w += 7 {
		for _, h := range []int{minHeight, 15, 40} {
			send(t, m, tea.WindowSizeMsg{Width: w, Height: h})
			out := m.View().Content
			if gw, gh := lipgloss.Width(out), lipgloss.Height(out); gw != w || gh != h {
				t.Fatalf("%s: pane %dx%d drew %dx%d", what, w, h, gw, gh)
			}
		}
	}
}

func TestFrameMatchesPaneAtEverySize(t *testing.T) {
	m := newModel(t, 110, 30)
	clickLayer(t, m, layerID(idRow, 0))
	checkFrames(t, m, "ticket")
	clickLayer(t, m, idCommentOrder)
	m.tabs[0].offset, m.tabs[0].sideOffset = 1000, 1000
	checkFrames(t, m, "ticket scrolled to the end")
	m.activeTab = -1
	checkFrames(t, m, "table")

	m.activeQueue = 2
	checkFrames(t, m, "board")
	m.openMenu(m.queues[2].tickets[0], 150, 38)
	checkFrames(t, m, "menu")
	m.menu = nil
	m.drag = &drag{index: 0, x: 159, y: 39}
	checkFrames(t, m, "drag")
}

func TestCommentsAreNewestFirstUntilToggled(t *testing.T) {
	m := newModel(t, 110, 60)
	clickLayer(t, m, layerID(idRow, 1))
	order := func() (newest, oldest int) {
		out := screen(m)
		newest, oldest = strings.Index(out, "A fix is in review"), strings.Index(out, "Still happening")
		if newest < 0 || oldest < 0 {
			t.Fatalf("comments not both on screen\n%s", out)
		}
		return newest, oldest
	}
	if newest, oldest := order(); newest > oldest {
		t.Fatalf("the oldest comment is first by default\n%s", screen(m))
	}
	if out := screen(m); !strings.Contains(out, "Comments (3)") || !strings.Contains(out, "newest first") {
		t.Fatalf("no count or order on the comments rule\n%s", out)
	}

	clickLayer(t, m, idCommentOrder)
	if !m.prefs.CommentsOldestFirst {
		t.Fatal("the click did not change the setting")
	}
	if newest, oldest := order(); newest < oldest {
		t.Fatalf("the newest comment is still first\n%s", screen(m))
	}
	if !strings.Contains(screen(m), "oldest first") {
		t.Error("the control does not name the new order")
	}
}

func TestSidebarStaysWhileMainScrolls(t *testing.T) {
	m := newModel(t, 110, 14)
	clickLayer(t, m, layerID(idRow, 1))
	m.View()
	tab := m.tabs[0]
	if tab.sideX == 0 {
		t.Fatal("the sidebar is not beside the main column at 110 wide")
	}
	for i := 0; i < 30; i++ {
		send(t, m, tea.MouseWheelMsg{X: 5, Y: 8, Button: tea.MouseWheelDown})
		m.View()
	}
	out := screen(m)
	if tab.offset == 0 || tab.sideOffset != 0 {
		t.Fatalf("offset=%d sideOffset=%d after scrolling the main column", tab.offset, tab.sideOffset)
	}
	for _, want := range []string{"DEMO-102", "Details", "Status", "Still happening"} {
		if !strings.Contains(out, want) {
			t.Errorf("scrolled ticket lacks %q\n%s", want, out)
		}
	}

	// The wheel over the sidebar moves the sidebar alone.
	main := tab.offset
	send(t, m, tea.MouseWheelMsg{X: tab.sideX + 2, Y: 8, Button: tea.MouseWheelDown})
	m.View()
	if tab.offset != main || tab.sideOffset == 0 {
		t.Fatalf("offset=%d (was %d) sideOffset=%d after scrolling the sidebar", tab.offset, main, tab.sideOffset)
	}
}

func TestBoardGroupsTicketsIntoLanes(t *testing.T) {
	m := newModel(t, 130, 40)
	clickLayer(t, m, layerID(idQueueTab, 2))
	out := screen(m)
	for _, want := range []string{"Open  5", "In progress", "Waiting", "Done", "DEMO-101", "╭"} {
		if !strings.Contains(out, want) {
			t.Errorf("board lacks %q\n%s", want, out)
		}
	}
	clickLayer(t, m, layerID(idCard, 0))
	if len(m.tabs) != 1 {
		t.Fatal("clicking a card did not open the ticket")
	}
}

func TestBoardScrollsSidewaysWhenNarrow(t *testing.T) {
	m := newModel(t, 60, 30) // room for two of the four lanes
	clickLayer(t, m, layerID(idQueueTab, 2))
	if out := screen(m); strings.Contains(out, "Waiting") {
		t.Fatalf("third lane drawn in a 60-wide pane\n%s", out)
	}
	clickLayer(t, m, idRight)
	clickLayer(t, m, idRight)
	send(t, m, tea.KeyPressMsg{Code: tea.KeyRight}) // one more than there is room for
	out := screen(m)
	if !strings.Contains(out, "Done") || strings.Contains(out, "Open  5") {
		t.Fatalf("board did not scroll to its last lanes\n%s", out)
	}
	if m.queues[2].laneOffset != 2 {
		t.Fatalf("laneOffset=%d, want it clamped to 2", m.queues[2].laneOffset)
	}
}

// dragTo presses on one layer, moves to the bottom of another and releases
// there.
func dragTo(t *testing.T, m *Model, from, to string) {
	t.Helper()
	m.View()
	a, b := m.comp.GetLayer(from), m.comp.GetLayer(to)
	if a == nil || b == nil {
		t.Fatalf("layers %q, %q not both on screen", from, to)
	}
	ax, ay := a.GetX()+2, a.GetY()+2
	bx, by := b.GetX()+3, b.GetY()+b.Height()-1
	send(t, m, tea.MouseClickMsg{X: ax, Y: ay, Button: tea.MouseLeft})
	send(t, m, tea.MouseMotionMsg{X: ax + 1, Y: ay, Button: tea.MouseLeft})
	m.View()
	send(t, m, tea.MouseMotionMsg{X: bx, Y: by, Button: tea.MouseLeft})
	m.View()
	if m.drag == nil {
		t.Fatal("no drag in progress after moving a pressed card")
	}
	send(t, m, tea.MouseReleaseMsg{X: bx, Y: by, Button: tea.MouseLeft})
}

func TestDraggedCardFollowsThePointer(t *testing.T) {
	m := newModel(t, 130, 40)
	clickLayer(t, m, layerID(idQueueTab, 2))
	q := m.queues[2]
	card := q.lanes()[0].tickets[0]
	m.View()
	l := m.comp.GetLayer(layerID(idCard, card))
	x, y := l.GetX()+2, l.GetY()+2
	send(t, m, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	send(t, m, tea.MouseMotionMsg{X: x + 60, Y: y + 10, Button: tea.MouseLeft})
	m.View()
	// The key is on screen twice: the card left behind and the carried copy.
	if n := strings.Count(screen(m), q.tickets[card].Key); n != 2 {
		t.Fatalf("key drawn %d times during a drag, want 2\n%s", n, screen(m))
	}
	// The carried copy must not hide the lane from the hit test.
	if kind, _ := splitID(m.comp.Hit(x+60, y+10).ID()); kind != idLane && kind != idCard {
		t.Fatalf("pointer hits %q under the carried card", kind)
	}
	send(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.drag != nil {
		t.Fatal("esc did not cancel the drag")
	}
}

func TestMenuOpensAndRuns(t *testing.T) {
	m := newModel(t, 110, 30)

	// The menu button on a row.
	clickLayer(t, m, layerID(idMore, 1))
	if m.menu == nil {
		t.Fatal("menu button did not open the menu")
	}
	out := screen(m)
	for _, want := range []string{"Open in browser", "Copy key", "DEMO-102"} {
		if !strings.Contains(out, want) {
			t.Errorf("menu lacks %q\n%s", want, out)
		}
	}
	// A click elsewhere closes it and does not reach what is underneath.
	clickLayer(t, m, idRefresh)
	if m.menu != nil || len(m.tabs) != 0 {
		t.Fatalf("click outside: menu open=%v, tabs=%d", m.menu != nil, len(m.tabs))
	}

	// Right-click and middle-click on a row.
	for _, button := range []tea.MouseButton{tea.MouseRight, tea.MouseMiddle} {
		m.View()
		row := m.comp.GetLayer(layerID(idRow, 2))
		send(t, m, tea.MouseClickMsg{X: row.GetX() + 4, Y: row.GetY(), Button: button})
		if m.menu == nil || m.menu.title != "DEMO-103" {
			t.Fatalf("button %v did not open the menu for the row", button)
		}
		m.menu = nil
	}

	clickLayer(t, m, layerID(idMore, 2))
	clickLayer(t, m, layerID(idMenuItem, 0))
	if m.menu != nil || len(m.tabs) != 1 || m.tabs[0].t.Key != "DEMO-103" {
		t.Fatalf("Open: menu open=%v, tabs=%d", m.menu != nil, len(m.tabs))
	}
}

func TestTableScrollsSideways(t *testing.T) {
	m := newModel(t, 60, 20)
	out := screen(m)
	if strings.Contains(out, "Created") {
		t.Fatalf("last column drawn in a 60-wide pane\n%s", out)
	}
	for i := 0; i < 6; i++ {
		send(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
		m.View()
	}
	out = screen(m)
	if !strings.Contains(out, "Created") || !strings.Contains(out, "DEMO-101") || !strings.Contains(out, "Title") {
		t.Fatalf("scrolled table lacks the last column, the key or the title\n%s", out)
	}
	for i := 0; i < 6; i++ {
		send(t, m, tea.KeyPressMsg{Code: tea.KeyLeft})
		m.View()
	}
	if m.queues[0].colOffset != 0 {
		t.Fatalf("colOffset=%d after scrolling back, want 0", m.queues[0].colOffset)
	}
}

func TestTooSmallPane(t *testing.T) {
	m := newModel(t, 30, 5)
	if !strings.Contains(screen(m), "Pane too small") {
		t.Fatalf("no notice in a 30x5 pane:\n%s", screen(m))
	}
}

func TestWriteLocksInput(t *testing.T) {
	m := newModel(t, 110, 30)
	m.View()
	// Start a write and leave it unfinished.
	m.start("Moving DEMO-101", true, func(context.Context) (tea.Msg, error) { return nil, nil })
	if !strings.Contains(screen(m), "input locked") {
		t.Error("status bar does not say input is locked")
	}
	row := m.comp.GetLayer(layerID(idRow, 0))
	send(t, m, tea.MouseClickMsg{X: row.GetX(), Y: row.GetY(), Button: tea.MouseLeft})
	send(t, m, tea.MouseReleaseMsg{X: row.GetX(), Y: row.GetY(), Button: tea.MouseLeft})
	if len(m.tabs) != 0 {
		t.Fatal("a click got through while a write was running")
	}
}
