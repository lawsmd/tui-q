package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lawsmd/tui-q/internal/ops"
	"github.com/lawsmd/tui-q/ticket"
)

// greyed reports whether every row above the status bar is drawn in the
// grey of a view that takes no input.
func greyed(m *Model) bool {
	rows := strings.Split(m.View().Content, "\n")
	grey := m.styles.Disabled.Render("x")
	start := grey[:strings.Index(grey, "x")]
	for _, row := range rows[:len(rows)-statusRows] {
		if strings.TrimSpace(ansi.Strip(row)) != "" && !strings.HasPrefix(row, start) {
			return false
		}
	}
	return true
}

func idle(context.Context) (tea.Msg, error) { return nil, nil }

func TestOpeningATicketLocksAndGreysUntilItLoads(t *testing.T) {
	m := newModel(t, 110, 30)
	m.View()
	row := m.comp.GetLayer(layerID(idRow, 0))
	x, y := row.GetX(), row.GetY()
	_, down := m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	_, up := m.Update(tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft})
	if len(m.tabs) != 1 || m.activeTab != 0 {
		t.Fatalf("the click did not open the ticket at once: tabs=%d active=%d", len(m.tabs), m.activeTab)
	}
	if !m.blocked() || !greyed(m) {
		t.Fatalf("blocked=%v greyed=%v while the ticket loads, want both", m.blocked(), greyed(m))
	}
	rows := strings.Split(screen(m), "\n")
	if status := rows[len(rows)-1]; !strings.Contains(status, "Loading DEMO-101") {
		t.Fatalf("status bar = %q", status)
	}
	// Esc would go back to the queue.
	send(t, m, esc)
	if m.activeTab != 0 {
		t.Fatal("a key got through while the ticket loaded")
	}

	drain(t, m, tea.Batch(down, up))
	if m.blocked() || greyed(m) {
		t.Fatalf("blocked=%v greyed=%v after the ticket loaded, want neither", m.blocked(), greyed(m))
	}
	send(t, m, esc)
	if m.activeTab != -1 {
		t.Fatal("esc did not go back to the queue after the ticket loaded")
	}
}

func TestAReadInTheBackgroundLeavesInputFree(t *testing.T) {
	m := newModel(t, 110, 30)
	m.start("Reading the notes", ops.Background, idle)
	if m.blocked() || greyed(m) {
		t.Fatalf("blocked=%v greyed=%v during a read in the background, want neither", m.blocked(), greyed(m))
	}
	if out := screen(m); !strings.Contains(out, "Reading the notes") {
		t.Fatalf("the status bar does not name the read:\n%s", out)
	}
	clickLayer(t, m, layerID(idQueueTab, 1))
	if m.activeQueue != 1 {
		t.Fatal("a click was refused during a read in the background")
	}
}

func TestAFailedQueueReadLiftsTheGrey(t *testing.T) {
	m := newModel(t, 110, 30)
	m.refresh()
	read := m.ops.Last()
	if !m.refreshing() || !greyed(m) {
		t.Fatal("the queue is not grey while it is read again")
	}
	// Something else is still running when the read fails.
	m.start("Reading the notes", ops.Background, idle)
	send(t, m, ops.DoneMsg{ID: read, Err: errors.New("the site said no")})
	if m.refreshing() || m.blocked() || greyed(m) {
		t.Fatalf("refreshing=%v blocked=%v greyed=%v after the read failed", m.refreshing(), m.blocked(), greyed(m))
	}
	if !strings.Contains(screen(m), "the site said no") {
		t.Fatalf("the status bar does not show the failure:\n%s", screen(m))
	}
}

func TestAnAnswerToAnOlderQueueReadIsDropped(t *testing.T) {
	m := newModel(t, 110, 30)
	q := m.queues[0]
	had := len(q.tickets)
	m.loadQueue(0)
	older := q.newest[fromQueue]
	m.loadQueue(0)
	newer := q.newest[fromQueue]

	// The newer read answers first, then the older one with other tickets.
	one := []ticket.Ticket{q.tickets[0]}
	send(t, m, queueLoadedMsg{index: 0, seq: newer, tickets: one})
	send(t, m, queueLoadedMsg{index: 0, seq: older, tickets: nil})
	if len(q.tickets) != 1 {
		t.Fatalf("the queue holds %d tickets after the older answer, want the 1 of the newer (it had %d)", len(q.tickets), had)
	}
}

func TestAnOpenFormTakesNoKeysDuringALock(t *testing.T) {
	m := newModel(t, 110, 30)
	clickLayer(t, m, layerID(idRow, 0))
	spec := ticket.FieldSpec{Name: "name", Label: "Name", Kind: ticket.KindText}
	drain(t, m, m.openForm("Rename", []ticket.FieldSpec{spec}, nil, func(map[string]ticket.Value) tea.Cmd { return nil }))
	if m.form == nil {
		t.Fatal("the form did not open")
	}
	m.start("Loading the moves open to DEMO-101", ops.Wait, idle)
	send(t, m, esc)
	if m.form == nil {
		t.Fatal("esc closed the form while input was locked")
	}
	send(t, m, tea.PasteMsg{Content: "pasted"})
	if strings.Contains(screen(m), "pasted") {
		t.Fatal("pasted text reached the form while input was locked")
	}
}
