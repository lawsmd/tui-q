// Package fake is an in-memory provider with fixture tickets. The stock
// binary and the UI tests run against it, so the interface can be built and
// tried with no network and no account.
package fake

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lawsmd/tui-q/provider"
	"github.com/lawsmd/tui-q/ticket"
)

// Kind is the profile value that selects this provider.
const Kind = "fake"

func init() {
	provider.Register(Kind, func(cfg provider.Config) (provider.Provider, error) {
		kinds, err := provider.PickKinds(cfg.CommentKinds,
			provider.CommentKind{ID: provider.KindPublic, Name: "public"},
			provider.CommentKind{ID: provider.KindInternal, Name: "internal"})
		if err != nil {
			return nil, fmt.Errorf("fake: %w", err)
		}
		p := New(300 * time.Millisecond)
		p.Kinds = kinds
		return p, nil
	})
}

// Statuses are the workflow states of the fixture tickets, in order.
var Statuses = []string{"Open", "In progress", "Waiting", "Done"}

// Priorities are the values of the priority field.
var Priorities = []string{"Low", "Medium", "High"}

// People are the users a fixture ticket can be assigned to. The first is
// the one the fake credentials belong to.
var People = []string{"sam", "alex", "jo"}

// Resolutions are the values the transition to Done asks for.
var Resolutions = []string{"Fixed", "Won't fix"}

// FieldResolution is the field the transition to Done requires.
const FieldResolution = "resolution"

func options(names []string) []ticket.Option {
	out := make([]ticket.Option, len(names))
	for i, name := range names {
		out[i] = ticket.Option{ID: name, Name: name}
	}
	return out
}

// editable is the edit description every fixture ticket carries.
func editable() []ticket.FieldSpec {
	return []ticket.FieldSpec{
		{Name: ticket.FieldAssignee, Label: "Assignee", Kind: ticket.KindUser},
		{Name: "labels", Label: "Labels", Kind: ticket.KindTags},
		{Name: ticket.FieldPriority, Label: "Priority", Kind: ticket.KindSelect, Required: true, Options: options(Priorities)},
		{Name: ticket.FieldTitle, Label: "Title", Kind: ticket.KindText, Required: true},
	}
}

// Provider serves the fixture tickets. Delay is added to every call so the
// status bar and write lock can be seen working.
type Provider struct {
	Delay time.Duration
	// FailWrites, when set, is returned by every call that would change
	// a ticket.
	FailWrites error
	// Kinds are the kinds of comment the comment box offers.
	Kinds []provider.CommentKind

	mu      sync.Mutex
	tickets []ticket.Ticket
}

// New returns a Provider holding a fresh copy of the fixtures.
func New(delay time.Duration) *Provider {
	return &Provider{Delay: delay, tickets: fixtures()}
}

func (p *Provider) wait(ctx context.Context) error {
	if p.Delay <= 0 {
		return ctx.Err()
	}
	select {
	case <-time.After(p.Delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Search understands an empty query (every ticket) and "name=value" terms
// joined by "and", matched without regard to case. A value of "*" matches
// anything non-empty and an empty value matches only an empty field.
func (p *Provider) Search(ctx context.Context, query string) ([]ticket.Ticket, error) {
	if err := p.wait(ctx); err != nil {
		return nil, err
	}
	terms, err := parse(query)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []ticket.Ticket
	for _, t := range p.tickets {
		if matches(t, terms) {
			out = append(out, t)
		}
	}
	return out, nil
}

// SearchPage returns one page of what Search finds. The cursor is the
// number of tickets to skip.
func (p *Provider) SearchPage(ctx context.Context, query, cursor string, limit int) (provider.Page, error) {
	all, err := p.Search(ctx, query)
	if err != nil {
		return provider.Page{}, err
	}
	skip := 0
	if cursor != "" {
		if skip, err = strconv.Atoi(cursor); err != nil || skip < 0 {
			return provider.Page{}, fmt.Errorf("bad cursor %q", cursor)
		}
	}
	page := provider.Page{Total: len(all)}
	skip = min(skip, len(all))
	end := min(skip+max(limit, 1), len(all))
	page.Tickets = all[skip:end]
	if end < len(all) {
		page.Next = strconv.Itoa(end)
	}
	return page, nil
}

type term struct{ name, value string }

func parse(query string) ([]term, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	var terms []term
	for _, part := range strings.Split(query, " and ") {
		name, value, ok := strings.Cut(part, "=")
		if !ok {
			return nil, fmt.Errorf("fake query: %q is not name=value", strings.TrimSpace(part))
		}
		terms = append(terms, term{strings.TrimSpace(name), strings.TrimSpace(value)})
	}
	return terms, nil
}

func matches(t ticket.Ticket, terms []term) bool {
	for _, tm := range terms {
		got := t.Field(tm.name).String()
		switch {
		case tm.value == "*":
			if got == "" {
				return false
			}
		case !strings.EqualFold(got, tm.value):
			return false
		}
	}
	return true
}

// Resolve reads a ticket key. A bare number is taken to be in the demo
// project.
func (p *Provider) Resolve(input string) (string, error) {
	return provider.KeyRef(input, "DEMO")
}

func (p *Provider) find(id string) (*ticket.Ticket, error) {
	for i := range p.tickets {
		if p.tickets[i].ID == id {
			return &p.tickets[i], nil
		}
	}
	return nil, fmt.Errorf("ticket %s not found", id)
}

// Get returns one ticket.
func (p *Provider) Get(ctx context.Context, id string) (ticket.Ticket, error) {
	if err := p.wait(ctx); err != nil {
		return ticket.Ticket{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	t, err := p.find(id)
	if err != nil {
		return ticket.Ticket{}, err
	}
	out := *t
	out.Editable = editable()
	return out, nil
}

// Transitions offers every status other than the current one. The move to
// Done asks for a resolution.
func (p *Provider) Transitions(ctx context.Context, id string) ([]ticket.Transition, error) {
	if err := p.wait(ctx); err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	t, err := p.find(id)
	if err != nil {
		return nil, err
	}
	var out []ticket.Transition
	for i, s := range Statuses {
		if s != t.Status {
			tr := ticket.Transition{ID: fmt.Sprint(i), Name: s, To: s}
			if s == "Done" {
				tr.Fields = []ticket.FieldSpec{{
					Name: FieldResolution, Label: "Resolution", Kind: ticket.KindSelect,
					Required: true, Options: options(Resolutions),
				}}
			}
			out = append(out, tr)
		}
	}
	return out, nil
}

// Transition sets the status.
func (p *Provider) Transition(ctx context.Context, id string, tr ticket.Transition, fields map[string]ticket.Value) error {
	if err := p.wait(ctx); err != nil {
		return err
	}
	if p.FailWrites != nil {
		return p.FailWrites
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	t, err := p.find(id)
	if err != nil {
		return err
	}
	if tr.To == "Done" {
		resolution := fields[FieldResolution]
		if resolution.String() == "" {
			return fmt.Errorf("a resolution is required to move %s to Done", id)
		}
		if t.Fields == nil {
			t.Fields = map[string]ticket.Value{}
		}
		t.Fields[FieldResolution] = resolution
	}
	t.Status = tr.To
	t.Updated = time.Now()
	return nil
}

// Assign sets the assignee; the user ID is used as the display name.
func (p *Provider) Assign(ctx context.Context, id, userID string) error {
	if err := p.wait(ctx); err != nil {
		return err
	}
	if p.FailWrites != nil {
		return p.FailWrites
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	t, err := p.find(id)
	if err != nil {
		return err
	}
	t.Assignee = userID
	t.Updated = time.Now()
	return nil
}

// UpdateField writes a built-in or custom field.
func (p *Provider) UpdateField(ctx context.Context, id, field string, v ticket.Value) error {
	if err := p.wait(ctx); err != nil {
		return err
	}
	if p.FailWrites != nil {
		return p.FailWrites
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	t, err := p.find(id)
	if err != nil {
		return err
	}
	switch field {
	case ticket.FieldAssignee:
		t.Assignee = v.ID
	case ticket.FieldTitle:
		t.Title = v.String()
	case ticket.FieldPriority:
		t.Priority = v.String()
	case ticket.FieldBody:
		t.Body = v.String()
	case ticket.FieldKey, ticket.FieldStatus, ticket.FieldCreated, ticket.FieldUpdated:
		return fmt.Errorf("field %q cannot be edited directly", field)
	default:
		if t.Fields == nil {
			t.Fields = map[string]ticket.Value{}
		}
		t.Fields[field] = v
	}
	t.Updated = time.Now()
	return nil
}

// CommentFormat is Markdown, the form the fixtures hold comments in.
func (p *Provider) CommentFormat() string { return provider.CommentMarkdown }

// CommentKinds are the kinds in Kinds.
func (p *Provider) CommentKinds() []provider.CommentKind { return p.Kinds }

// AddComment adds a comment written by the first of People. An internal
// one is marked as such.
func (p *Provider) AddComment(ctx context.Context, id, kind, body string) error {
	if err := p.wait(ctx); err != nil {
		return err
	}
	if p.FailWrites != nil {
		return p.FailWrites
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	t, err := p.find(id)
	if err != nil {
		return err
	}
	now := time.Now()
	if kind != provider.KindInternal {
		kind = ""
	}
	// A new slice, so that a copy of the ticket handed out earlier keeps
	// the comments it had.
	t.Comments = append(append([]ticket.Comment(nil), t.Comments...),
		ticket.Comment{Author: People[0], Created: now, Body: body, Kind: kind})
	t.Updated = now
	return nil
}

// Me is the first of People.
func (p *Provider) Me(ctx context.Context) (ticket.User, error) {
	if err := p.wait(ctx); err != nil {
		return ticket.User{}, err
	}
	return ticket.User{ID: People[0], Name: People[0]}, nil
}

// Users lists the People whose names contain query.
func (p *Provider) Users(ctx context.Context, _, query string) ([]ticket.User, error) {
	if err := p.wait(ctx); err != nil {
		return nil, err
	}
	var out []ticket.User
	for _, name := range People {
		if strings.Contains(name, strings.ToLower(strings.TrimSpace(query))) {
			out = append(out, ticket.User{ID: name, Name: name})
		}
	}
	return out, nil
}

// FieldLabels has nothing to add: the fixtures have no platform names.
func (p *Provider) FieldLabels(context.Context) (map[string]string, error) {
	return nil, nil
}
