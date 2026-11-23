package ui

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/lawsmd/tui-q/ext"
	"github.com/lawsmd/tui-q/ticket"
)

// testSection has a clickable state line with one action that asks a
// question and one that does not, and a faint line under it. It counts its
// reads, and the state line names the last action run.
func testSection(reads *atomic.Int32, last *string) ext.Section {
	return ext.Section{
		Title: "probe",
		Read: func(_ context.Context, t ticket.Ticket) ([]ext.Line, error) {
			reads.Add(1)
			state := "state " + *last
			acts := []ext.Action{
				{Name: "pause", Run: func(context.Context, ext.Run) (string, error) { *last = "paused"; return "", nil }},
				{Name: "finish",
					Fields: func(ticket.Ticket) []ticket.FieldSpec {
						return []ticket.FieldSpec{{Name: "why", Label: "Why", Kind: ticket.KindText, Required: true}}
					},
					Run: func(_ context.Context, r ext.Run) (string, error) {
						*last = "finished " + r.Values["why"].Text
						return "", nil
					}},
			}
			return []ext.Line{
				{Text: state, Actions: acts},
				{Text: "a long faint line that has to wrap onto more than one line of the sidebar", Faint: true},
			}, nil
		},
	}
}

func TestSectionSitsAboveDetailsAtEverySize(t *testing.T) {
	for _, w := range []int{110, 60} {
		m := newModel(t, w, 30)
		var reads atomic.Int32
		last := "new"
		m.sections = []ext.Section{testSection(&reads, &last)}
		clickLayer(t, m, layerID(idRow, 0))
		out := screen(m)
		probe, details := strings.Index(out, "probe"), strings.Index(out, "Details")
		if probe < 0 || details < 0 || probe > details {
			t.Fatalf("width %d: section not above Details\n%s", w, out)
		}
		if !strings.Contains(out, "state new") {
			t.Fatalf("width %d: no state line\n%s", w, out)
		}
		checkFrames(t, m, "ticket with a section")
		m.tabs[0].offset, m.tabs[0].sideOffset = 1000, 1000
		checkFrames(t, m, "ticket with a section, scrolled")
	}
}

func TestSectionLineRunsItsActionsAndReadsAgain(t *testing.T) {
	m := newModel(t, 110, 30)
	var reads atomic.Int32
	last := "new"
	m.sections = []ext.Section{testSection(&reads, &last)}
	clickLayer(t, m, layerID(idRow, 0))
	if reads.Load() != 1 {
		t.Fatalf("reads after opening = %d", reads.Load())
	}

	clickLayer(t, m, layerID(idSection, 0))
	if m.menu == nil {
		t.Fatal("no menu after a click on the state line")
	}
	clickItem(t, m, "pause")
	if reads.Load() != 2 || !strings.Contains(screen(m), "state paused") {
		t.Fatalf("reads = %d after the action\n%s", reads.Load(), screen(m))
	}

	clickLayer(t, m, layerID(idSection, 0))
	clickItem(t, m, "finish")
	if m.form == nil {
		t.Fatal("no form for an action with a question")
	}
	typeText(t, m, "done")
	send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if !strings.Contains(screen(m), "state finished done") {
		t.Fatalf("after the form\n%s", screen(m))
	}
}

func TestSectionShowsItsErrorAndSkipsTicketsItDoesNotApplyTo(t *testing.T) {
	m := newModel(t, 110, 30)
	first := m.queues[0].tickets[0].ID
	m.sections = []ext.Section{{
		Title:   "probe",
		Applies: func(t ticket.Ticket) bool { return t.ID == first },
		Read: func(context.Context, ticket.Ticket) ([]ext.Line, error) {
			return nil, errors.New("cannot read the probe")
		},
	}}
	clickLayer(t, m, layerID(idRow, 0))
	if out := screen(m); !strings.Contains(out, "cannot read the") {
		t.Fatalf("no error in the section\n%s", out)
	}
	clickLayer(t, m, layerID(idQueueTab, 0))
	clickLayer(t, m, layerID(idRow, 1))
	if out := screen(m); strings.Contains(out, "probe") {
		t.Fatalf("section shown on a ticket it does not apply to\n%s", out)
	}
}

func TestSectionReadForAnotherTicketIsDropped(t *testing.T) {
	m := newModel(t, 110, 30)
	var reads atomic.Int32
	last := "new"
	m.sections = []ext.Section{testSection(&reads, &last)}
	clickLayer(t, m, layerID(idRow, 0))
	send(t, m, sectionReadMsg{ticketID: "nobody", i: 0, lines: []ext.Line{{Text: "stale"}}})
	if strings.Contains(screen(m), "stale") {
		t.Fatal("a read for another ticket was shown")
	}
}
