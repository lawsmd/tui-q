package ui

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/lawsmd/tui-q/ext"
	"github.com/lawsmd/tui-q/ticket"
)

// actionsFor lists the instance's actions that are offered for a ticket,
// by their place in the model's list.
func (m *Model) actionsFor(t ticket.Ticket) []int {
	var out []int
	for i, a := range m.actions {
		if a.Applies == nil || a.Applies(t) {
			out = append(out, i)
		}
	}
	return out
}

// runAction starts an action on a ticket. When the action has questions,
// a form asks them first and saving the form starts the action.
func (m *Model) runAction(i int, t ticket.Ticket) tea.Cmd {
	if i < 0 || i >= len(m.actions) {
		return nil
	}
	a := m.actions[i]
	var specs []ticket.FieldSpec
	if a.Fields != nil {
		specs = a.Fields(t)
	}
	if len(specs) == 0 {
		return m.doAction(a, t, nil)
	}
	for _, spec := range specs {
		if !formable(spec) {
			m.notice = fmt.Sprintf("%s asks for %s, which cannot be entered here.", a.Name, spec.Name)
			return nil
		}
	}
	return m.openForm(a.Name+": "+t.Key, specs, nil, func(values map[string]ticket.Value) tea.Cmd {
		return m.doAction(a, t, values)
	})
}

// doAction runs an action as a write: input is locked until it ends, its
// steps are shown in the status bar, and the ticket is read back after it.
func (m *Model) doAction(a ext.Action, t ticket.Ticket, values map[string]ticket.Value) tea.Cmd {
	timeout := a.Timeout
	if timeout <= 0 {
		timeout = ext.DefaultTimeout
	}
	m.undo = nil
	label := a.Name + ": " + t.Key
	return m.spinWith(m.ops.StartSteps(label, true, timeout, func(ctx context.Context, report func(string)) (tea.Msg, error) {
		note, err := a.Run(ctx, ext.Run{Ticket: t, Values: values, Provider: m.prov, Progress: report})
		if err != nil {
			return nil, fmt.Errorf("%s: %w", label, err)
		}
		if note == "" {
			note = label + " finished"
		}
		report("reading " + t.Key + " back")
		return m.reloaded(ctx, t.ID, note), nil
	}))
}
