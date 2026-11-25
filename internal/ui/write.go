package ui

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/lawsmd/tui-q/internal/ops"
	"github.com/lawsmd/tui-q/provider"
	"github.com/lawsmd/tui-q/ticket"
)

// transitionsMsg carries the moves open to a ticket, for the menu that was
// asked for at x, y.
type transitionsMsg struct {
	t    ticket.Ticket
	trs  []ticket.Transition
	x, y int
}

// usersMsg carries the result of a user search.
type usersMsg struct {
	t     ticket.Ticket
	query string
	users []ticket.User
	x, y  int
}

// changedMsg reports a finished write. t is the ticket as the platform now
// has it, or empty when it could not be read back.
type changedMsg struct {
	t    ticket.Ticket
	trs  []ticket.Transition
	note string
}

// needFieldsMsg reports that a move was not made because the platform asks
// for values with it.
type needFieldsMsg struct {
	t  ticket.Ticket
	tr ticket.Transition
}

// write runs a provider call that changes a ticket. Input is locked until
// it ends. undo, when not nil, is called if the write fails. A write makes
// several requests, and fn names each to report before making it, so the
// status bar says which one the user is waiting on.
func (m *Model) write(label string, undo func(), fn ops.StepFunc) tea.Cmd {
	m.undo = undo
	return m.spinWith(m.ops.StartSteps(label, ops.Write, 0, fn))
}

// reloaded reads a ticket back after a write and wraps it for the model.
func (m *Model) reloaded(ctx context.Context, report func(string), id, note string) tea.Msg {
	report("reading it back")
	t, trs, err := m.read(ctx, id)
	if err != nil {
		// The write went through, so this is not a failure of the write.
		return changedMsg{note: note + ". Reading it back failed."}
	}
	return changedMsg{t: t, trs: trs, note: note}
}

// moveTo moves a ticket to the status with the given name. When the move
// asks for values that fields does not hold, nothing is written and the
// model is told to ask for them.
//
// known is the move itself when the caller has it, as a menu of moves
// does: the moves open to the ticket are then not read again first. A card
// dropped on a lane knows the name of the status alone and passes nil.
func (m *Model) moveTo(t ticket.Ticket, status string, known *ticket.Transition, fields map[string]ticket.Value, undo func()) tea.Cmd {
	label := fmt.Sprintf("Moving %s to %s", t.Key, status)
	return m.write(label, undo, func(ctx context.Context, report func(string)) (tea.Msg, error) {
		tr := known
		if tr == nil {
			report("reading the moves open to it")
			trs, err := m.prov.Transitions(ctx, t.ID)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", t.Key, err)
			}
			for i := range trs {
				if strings.EqualFold(trs[i].To, status) {
					tr = &trs[i]
					break
				}
			}
		}
		if tr == nil {
			return nil, fmt.Errorf("%s cannot go from %s to %s", t.Key, t.Status, status)
		}
		for _, spec := range tr.Fields {
			if _, ok := fields[spec.Name]; spec.Required && !ok {
				return needFieldsMsg{t: t, tr: *tr}, nil
			}
		}
		report("moving it")
		if err := m.prov.Transition(ctx, t.ID, *tr, fields); err != nil {
			return nil, fmt.Errorf("%s: %w", t.Key, err)
		}
		return m.reloaded(ctx, report, t.ID, fmt.Sprintf("Moved %s to %s", t.Key, tr.To)), nil
	})
}

// askFields opens a form for the values a move requires, and makes the
// move when the form is saved.
func (m *Model) askFields(t ticket.Ticket, tr ticket.Transition) tea.Cmd {
	var specs []ticket.FieldSpec
	for _, spec := range tr.Fields {
		if !spec.Required {
			continue
		}
		if !formable(spec) {
			m.notice = fmt.Sprintf("Moving %s to %s needs %s, which cannot be entered here. Open it in the browser.", t.Key, tr.To, spec.Label)
			return nil
		}
		specs = append(specs, spec)
	}
	title := fmt.Sprintf("Move %s to %s", t.Key, tr.To)
	return m.openForm(title, specs, nil, func(values map[string]ticket.Value) tea.Cmd {
		return m.moveTo(t, tr.To, &tr, values, nil)
	})
}

// assign gives a ticket to a user. A user with no ID clears the assignee.
func (m *Model) assign(t ticket.Ticket, u ticket.User) tea.Cmd {
	label, note := "Assigning "+t.Key+" to "+u.Name, "Assigned "+t.Key+" to "+u.Name
	if u.ID == "" {
		label, note = "Unassigning "+t.Key, "Unassigned "+t.Key
	}
	return m.write(label, nil, func(ctx context.Context, report func(string)) (tea.Msg, error) {
		report("writing the change")
		if err := m.prov.Assign(ctx, t.ID, u.ID); err != nil {
			return nil, fmt.Errorf("%s: %w", t.Key, err)
		}
		return m.reloaded(ctx, report, t.ID, note), nil
	})
}

// assignByName gives a ticket to the user with exactly this name, which is
// all a lane of a board knows. An empty name clears the assignee.
func (m *Model) assignByName(t ticket.Ticket, name string, undo func()) tea.Cmd {
	label := "Assigning " + t.Key + " to " + name
	if name == "" {
		label = "Unassigning " + t.Key
	}
	return m.write(label, undo, func(ctx context.Context, report func(string)) (tea.Msg, error) {
		id := ""
		if name != "" {
			report("looking up " + name)
			users, err := m.prov.Users(ctx, t.ID, name)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", t.Key, err)
			}
			for _, u := range users {
				if strings.EqualFold(u.Name, name) {
					id = u.ID
				}
			}
			if id == "" {
				return nil, fmt.Errorf("%s: no assignable user is named %s", t.Key, name)
			}
		}
		report("writing the change")
		if err := m.prov.Assign(ctx, t.ID, id); err != nil {
			return nil, fmt.Errorf("%s: %w", t.Key, err)
		}
		if name == "" {
			return m.reloaded(ctx, report, t.ID, "Unassigned "+t.Key), nil
		}
		return m.reloaded(ctx, report, t.ID, "Assigned "+t.Key+" to "+name), nil
	})
}

// setField writes one field of a ticket.
func (m *Model) setField(t ticket.Ticket, name string, v ticket.Value, undo func()) tea.Cmd {
	label := m.label(name)
	return m.write("Setting "+label+" on "+t.Key, undo, func(ctx context.Context, report func(string)) (tea.Msg, error) {
		report("writing the change")
		if err := m.prov.UpdateField(ctx, t.ID, name, v); err != nil {
			return nil, fmt.Errorf("%s: %w", t.Key, err)
		}
		return m.reloaded(ctx, report, t.ID, "Set "+label+" on "+t.Key), nil
	})
}

// loadTransitions fetches the moves open to a ticket for a menu at x, y.
func (m *Model) loadTransitions(t ticket.Ticket, x, y int) tea.Cmd {
	return m.start("Loading the moves open to "+t.Key, ops.Wait, func(ctx context.Context) (tea.Msg, error) {
		trs, err := m.prov.Transitions(ctx, t.ID)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", t.Key, err)
		}
		return transitionsMsg{t: t, trs: trs, x: x, y: y}, nil
	})
}

// findUsers searches for people a ticket can be assigned to, for a menu
// at x, y.
func (m *Model) findUsers(t ticket.Ticket, query string, x, y int) tea.Cmd {
	return m.start("Searching for "+query, ops.Wait, func(ctx context.Context) (tea.Msg, error) {
		users, err := m.prov.Users(ctx, t.ID, query)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", t.Key, err)
		}
		return usersMsg{t: t, query: query, users: users, x: x, y: y}, nil
	})
}

// replaceTicket puts t in place of the ticket with the same ID in every
// queue and open tab.
func (m *Model) replaceTicket(t ticket.Ticket) {
	for _, q := range m.queues {
		for i := range q.tickets {
			if q.tickets[i].ID != t.ID {
				continue
			}
			if q.cfg.Recent {
				q.tickets[i] = keepTouch(t, q.tickets[i])
				continue
			}
			q.tickets[i] = t
		}
	}
	for _, tab := range m.tabs {
		if tab.t.ID == t.ID {
			tab.set(t)
		}
	}
}

// keepTouch returns t with the fields a recent queue set on old, which a
// ticket read from the platform lacks.
func keepTouch(t, old ticket.Ticket) ticket.Ticket {
	fields := make(map[string]ticket.Value, len(t.Fields)+2)
	for k, v := range t.Fields {
		fields[k] = v
	}
	for _, name := range []string{provider.FieldTouched, provider.FieldChange} {
		if v, ok := old.Fields[name]; ok {
			fields[name] = v
		}
	}
	t.Fields = fields
	return t
}

// withField returns a copy of t with one field changed, for showing a
// change on screen before the platform confirms it.
func withField(t ticket.Ticket, name, value string) ticket.Ticket {
	switch name {
	case ticket.FieldStatus:
		t.Status = value
	case ticket.FieldAssignee:
		t.Assignee = value
	case ticket.FieldPriority:
		t.Priority = value
	default:
		fields := make(map[string]ticket.Value, len(t.Fields)+1)
		for k, v := range t.Fields {
			fields[k] = v
		}
		fields[name] = ticket.Text(value)
		t.Fields = fields
	}
	return t
}

// drop ends a drag. A card dropped on another lane is moved there at once
// on screen and the change is written to the platform; when the write
// fails the card goes back.
func (m *Model) drop(ev hitEvent) tea.Cmd {
	d := m.drag
	m.drag = nil
	q := m.queues[m.activeQueue]
	if d == nil || d.index >= len(q.tickets) {
		return nil
	}
	old := q.tickets[d.index]
	target := q.laneName(ev.Over)
	if target == "" || strings.EqualFold(target, q.laneOf(old)) {
		return nil
	}
	value := target
	if target == noLane {
		value = ""
	}
	undo := func() { m.replaceTicket(old) }

	field := q.cfg.GroupBy
	switch field {
	case ticket.FieldStatus:
		if value == "" {
			return nil
		}
		q.tickets[d.index] = withField(old, field, value)
		return m.moveTo(old, value, nil, nil, undo)
	case ticket.FieldAssignee:
		q.tickets[d.index] = withField(old, field, value)
		return m.assignByName(old, value, undo)
	case ticket.FieldKey, ticket.FieldTitle, ticket.FieldType, ticket.FieldBody, ticket.FieldCreated, ticket.FieldUpdated:
		m.notice = "Cards on this board cannot be moved: its lanes group by " + m.label(field) + "."
		return nil
	}
	q.tickets[d.index] = withField(old, field, value)
	return m.setField(old, field, ticket.Text(value), undo)
}

// moveMenu lists the moves open to a ticket.
func (m *Model) moveMenu(t ticket.Ticket, trs []ticket.Transition, x, y int) {
	if len(trs) == 0 {
		m.notice = t.Key + " has no moves open to it."
		return
	}
	var items []menuItem
	for _, tr := range trs {
		label := tr.To
		if tr.Name != "" && !strings.EqualFold(tr.Name, tr.To) {
			label += "  (" + tr.Name + ")"
		}
		items = append(items, menuItem{label, func() tea.Cmd { return m.moveTo(t, tr.To, &tr, nil, nil) }})
	}
	m.showMenu("Move "+t.Key+" to", x, y, items)
}

// assignMenu offers the ways to change who a ticket is assigned to.
func (m *Model) assignMenu(t ticket.Ticket, x, y int) {
	var items []menuItem
	if m.me.ID != "" && !strings.EqualFold(t.Assignee, m.me.Name) {
		items = append(items, menuItem{"Me", func() tea.Cmd { return m.assign(t, m.me) }})
	}
	items = append(items, menuItem{"Someone else...", func() tea.Cmd { return m.askUser(t, x, y) }})
	if t.Assignee != "" {
		items = append(items, menuItem{"Nobody", func() tea.Cmd { return m.assign(t, ticket.User{}) }})
	}
	m.showMenu("Assign "+t.Key+" to", x, y, items)
}

// askUser asks for part of a name, then searches for it.
func (m *Model) askUser(t ticket.Ticket, x, y int) tea.Cmd {
	const name = "name"
	spec := ticket.FieldSpec{Name: name, Label: "Name to search for", Kind: ticket.KindText, Required: true}
	return m.openForm("Assign "+t.Key, []ticket.FieldSpec{spec}, nil, func(values map[string]ticket.Value) tea.Cmd {
		return m.findUsers(t, values[name].Text, x, y)
	})
}

// userMenu lists the people a search found.
func (m *Model) userMenu(msg usersMsg) {
	if len(msg.users) == 0 {
		m.notice = fmt.Sprintf("Nobody %s can be assigned to matches %q.", msg.t.Key, msg.query)
		return
	}
	var items []menuItem
	for _, u := range msg.users {
		items = append(items, menuItem{u.Name, func() tea.Cmd { return m.assign(msg.t, u) }})
	}
	m.showMenu("Assign "+msg.t.Key+" to", msg.x, msg.y, items)
}

// editable reports whether a field of an open ticket can be changed by
// clicking it.
func (tab *ticketTab) editable(name string) bool {
	if name == ticket.FieldStatus {
		return len(tab.transitions) > 0
	}
	spec, ok := tab.t.Spec(name)
	if !ok {
		return false
	}
	switch spec.Kind {
	case ticket.KindUser:
		return name == ticket.FieldAssignee
	case ticket.KindSelect:
		return len(spec.Options) > 0
	case ticket.KindRichText:
		// Writing rich text from here would drop its formatting.
		return false
	case ticket.KindTicket:
		// Picked from a search, which no form has.
		return true
	}
	return formable(spec)
}

// editField starts the edit of one field of an open ticket: a menu for a
// field with a fixed set of values, the search box for a list of tags or
// for a field that holds a ticket, and a form for the rest.
func (m *Model) editField(tab *ticketTab, name string, x, y int) tea.Cmd {
	t := tab.t
	switch name {
	case ticket.FieldStatus:
		m.moveMenu(t, tab.transitions, x, y)
		return nil
	case ticket.FieldAssignee:
		m.assignMenu(t, x, y)
		return nil
	}
	spec, ok := t.Spec(name)
	if !ok {
		return nil
	}
	spec.Label = m.label(name)
	current := t.Field(name)
	if spec.Kind == ticket.KindTags || spec.Kind == ticket.KindTicket {
		return m.openPicker(tab, name)
	}
	if spec.Kind == ticket.KindSelect {
		var items []menuItem
		for _, o := range spec.Options {
			items = append(items, menuItem{o.Name, func() tea.Cmd { return m.setField(t, name, ticket.Text(o.Name), nil) }})
		}
		if !spec.Required && current.String() != "" {
			items = append(items, menuItem{"(clear)", func() tea.Cmd { return m.setField(t, name, ticket.Value{}, nil) }})
		}
		m.showMenu(spec.Label, x, y, items)
		return nil
	}
	title := "Edit " + spec.Label + " on " + t.Key
	if name == ticket.FieldTitle {
		title = "Rename " + t.Key
	}
	values := map[string]ticket.Value{name: current}
	return m.openForm(title, []ticket.FieldSpec{spec}, values, func(values map[string]ticket.Value) tea.Cmd {
		return m.setField(t, name, values[name], nil)
	})
}
