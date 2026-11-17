package ui

import (
	"context"
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

[ticket]
comments = true
`

// drain runs a command and everything it leads to, feeding each message
// back into the model. Spinner ticks are dropped so the loop ends.
func drain(t *testing.T, m *Model, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
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
	m := New(prof, fake.New(0))
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
	for _, want := range []string{"Open 5", "All 12", "DEMO-101", "refresh", "5 tickets"} {
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
	for _, want := range []string{"DEMO-102", "open in browser", "Steps to reproduce", "Comments", "Status"} {
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
	m := newModel(t, 110, 10) // 6 table rows for 12 tickets
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

func TestFrameMatchesPaneAtEverySize(t *testing.T) {
	m := newModel(t, 110, 30)
	clickLayer(t, m, layerID(idRow, 0))
	for _, showTicket := range []bool{false, true} {
		if !showTicket {
			m.activeTab = -1
		} else {
			m.activeTab = 0
		}
		for w := minWidth; w <= 160; w += 7 {
			for _, h := range []int{minHeight, 15, 40} {
				send(t, m, tea.WindowSizeMsg{Width: w, Height: h})
				out := m.View().Content
				if gw, gh := lipgloss.Width(out), lipgloss.Height(out); gw != w || gh != h {
					t.Fatalf("ticket=%v pane %dx%d drew %dx%d", showTicket, w, h, gw, gh)
				}
			}
		}
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
