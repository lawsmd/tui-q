package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/lawsmd/tui-q/ext"
	"github.com/lawsmd/tui-q/ticket"
)

// testActions is one action that asks a question and moves the ticket, and
// one that is offered only for tickets in progress and always fails.
func testActions(ran *[]ext.Run) []ext.Action {
	return []ext.Action{
		{
			Name: "hand over",
			Fields: func(ticket.Ticket) []ticket.FieldSpec {
				return []ticket.FieldSpec{{Name: "team", Label: "Team", Kind: ticket.KindSelect, Required: true,
					Options: []ticket.Option{{ID: "1", Name: "Billing"}, {ID: "2", Name: "Platform"}}}}
			},
			Run: func(ctx context.Context, r ext.Run) (string, error) {
				*ran = append(*ran, r)
				r.Progress("moving the ticket")
				trs, err := r.Provider.Transitions(ctx, r.Ticket.ID)
				if err != nil {
					return "", err
				}
				for _, tr := range trs {
					if tr.To == "In progress" {
						return "Handed " + r.Ticket.Key + " to " + r.Values["team"].Text, r.Provider.Transition(ctx, r.Ticket.ID, tr, nil)
					}
				}
				return "", errors.New("no move to In progress")
			},
		},
		{
			Name:    "break",
			Applies: func(t ticket.Ticket) bool { return t.Status == "In progress" },
			Run: func(context.Context, ext.Run) (string, error) {
				return "", errors.New("it broke")
			},
		},
	}
}

func TestActionAsksThenRunsFromTheTicketView(t *testing.T) {
	m := newModel(t, 110, 30)
	var ran []ext.Run
	m.actions = testActions(&ran)
	key := m.queues[0].tickets[0].Key
	clickLayer(t, m, layerID(idRow, 0))

	out := screen(m)
	if !strings.Contains(out, "hand over") || strings.Contains(out, "break") {
		t.Fatalf("buttons for an open ticket:\n%s", out)
	}
	clickLayer(t, m, layerID(idAction, 0))
	if m.form == nil || len(ran) != 0 {
		t.Fatalf("form=%v runs=%d, want a form and no run yet", m.form != nil, len(ran))
	}
	checkFrames(t, m, "action form")

	// The second option, then save.
	send(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	send(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(ran) != 1 || ran[0].Ticket.Key != key || ran[0].Values["team"].Text != "Platform" {
		t.Fatalf("runs = %+v", ran)
	}
	if got := stored(t, m, key).Status; got != "In progress" {
		t.Fatalf("status = %q", got)
	}
	if m.notice != "Handed "+key+" to Platform" {
		t.Fatalf("notice = %q", m.notice)
	}
	// The ticket was read back, so the second action is now offered, and
	// the Open queue no longer holds the ticket.
	if !strings.Contains(screen(m), "break") {
		t.Fatalf("no button for the second action:\n%s", screen(m))
	}
	for _, tk := range m.queues[0].tickets {
		if tk.Key == key {
			t.Fatal("the queues did not reload")
		}
	}

	clickLayer(t, m, layerID(idAction, 1))
	failed, ok := m.ops.Failed()
	if !ok || !strings.Contains(failed.Err.Error(), "break: "+key+": it broke") {
		t.Fatalf("failure = %v (%v)", failed.Err, ok)
	}
	if m.ops.Locked() {
		t.Fatal("input stayed locked after the action failed")
	}
}

func TestActionCancelledInTheFormDoesNotRun(t *testing.T) {
	m := newModel(t, 110, 30)
	var ran []ext.Run
	m.actions = testActions(&ran)

	clickLayer(t, m, layerID(idMore, 0))
	for _, it := range m.menu.items {
		if it.label == "break" {
			t.Fatal("the menu offers an action that does not apply")
		}
	}
	clickItem(t, m, "hand over")
	if m.form == nil {
		t.Fatal("no form")
	}
	send(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.form != nil || len(ran) != 0 || m.ops.Busy() {
		t.Fatalf("after esc: form=%v runs=%d busy=%v", m.form != nil, len(ran), m.ops.Busy())
	}
}

func TestStatusBarShowsTheStepOfARunningAction(t *testing.T) {
	m := newModel(t, 110, 30)
	step := make(chan struct{})
	finish := make(chan struct{})
	m.actions = []ext.Action{{Name: "copy", Run: func(_ context.Context, r ext.Run) (string, error) {
		r.Progress("file 2 of 3")
		close(step)
		<-finish
		return "", nil
	}}}
	cmd := m.runAction(0, m.queues[0].tickets[0])
	done := make(chan struct{})
	go func() {
		defer close(done)
		drain(t, m, cmd)
	}()
	<-step
	if out := screen(m); !strings.Contains(out, "copy: DEMO-101: file 2 of 3 (input locked)") {
		t.Errorf("status bar while the action runs:\n%s", out)
	}
	close(finish)
	<-done
	if m.ops.Busy() {
		t.Error("still busy after the action ended")
	}
}
