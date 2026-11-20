// Package provider defines what tui-q needs from a ticketing platform.
package provider

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"sync"

	"github.com/lawsmd/tui-q/ticket"
)

// Config is the [provider] section of a profile.
type Config struct {
	// Kind picks the platform implementation, such as "jira" or "fake".
	Kind string `toml:"kind"`
	// BaseURL is the site or organisation URL.
	BaseURL string `toml:"base_url"`
	// UserEnv and TokenEnv name the environment variables holding the
	// credentials. A profile never holds a secret.
	UserEnv  string `toml:"user_env"`
	TokenEnv string `toml:"token_env"`
	// DefaultProject is the project a bare ticket number is looked up in,
	// on a platform whose ticket keys carry a project.
	DefaultProject string `toml:"default_project"`
	// CommentKinds turns on a choice of comment kind in the comment box,
	// on a platform that has kinds: the IDs to offer, such as "internal"
	// and "public", with the one a new comment starts as first.
	CommentKinds []string `toml:"comment_kinds"`
	// Fields maps a profile field name to the platform's field ID.
	Fields map[string]string `toml:"-"`
}

// Page is one page of the results of a query.
type Page struct {
	Tickets []ticket.Ticket
	// Next is the cursor that asks for the page after this one. It is
	// empty on the last page.
	Next string
	// Total is how many tickets the query matches, or -1 when the
	// platform does not say.
	Total int
}

// Provider is one ticketing platform. Every method may block on the network;
// the UI calls them only through the operation runner.
type Provider interface {
	// Resolve reads what a user typed or pasted to name a ticket, such as
	// a number, a key or a link, and returns the ID that Get takes. It
	// does not use the network. KeyRef and NumberRef do the reading for
	// the two common shapes of ID.
	Resolve(input string) (string, error)
	// Search runs a query written in the platform's own query language.
	Search(ctx context.Context, query string) ([]ticket.Ticket, error)
	// SearchPage runs a query and returns one page of at most limit
	// tickets. An empty cursor asks for the first page, and the Next of a
	// page asks for the one after it. A cursor is only good for the query
	// and limit it came from.
	SearchPage(ctx context.Context, query, cursor string, limit int) (Page, error)
	// Get loads one ticket in full, with body and comments.
	Get(ctx context.Context, id string) (ticket.Ticket, error)
	// Transitions lists the status changes open to a ticket now.
	Transitions(ctx context.Context, id string) ([]ticket.Transition, error)
	// Transition applies one. fields carries any values the platform demands.
	Transition(ctx context.Context, id string, tr ticket.Transition, fields map[string]ticket.Value) error
	// Assign sets the assignee. An empty userID clears it.
	Assign(ctx context.Context, id, userID string) error
	// UpdateField writes one field.
	UpdateField(ctx context.Context, id, field string, v ticket.Value) error
	// Me is the user the credentials belong to.
	Me(ctx context.Context) (ticket.User, error)
	// Users lists the people a ticket can be assigned to whose names match
	// query.
	Users(ctx context.Context, id, query string) ([]ticket.User, error)
	// FieldLabels gives the platform's own name for each custom field of
	// the profile, keyed by the profile's field name.
	FieldLabels(ctx context.Context) (map[string]string, error)
}

// Comment formats: the forms a Commenter can take a comment body in.
const (
	// CommentMarkdown is Markdown text.
	CommentMarkdown = "markdown"
	// CommentJira is Atlassian Document Format, as JSON.
	CommentJira = "jira"
)

// Commenter is a Provider that can add a comment to a ticket. The interface
// offers the comment box only for a provider that implements it.
type Commenter interface {
	// CommentFormat names the form AddComment takes a body in: one of the
	// Comment constants. The comment box writes Markdown and converts it
	// to this form when the comment is sent.
	CommentFormat() string
	// CommentKinds lists the kinds of comment the box lets the user pick
	// from, the one a new comment starts as first. A provider with no such
	// choice returns none.
	CommentKinds() []CommentKind
	// AddComment adds a comment to a ticket. kind is the ID of one of
	// CommentKinds, or empty when there are none.
	AddComment(ctx context.Context, id, kind, body string) error
}

// Mentioner is a Provider whose search for people to mention in a comment
// differs from Users. The comment box asks Users where a provider does not
// implement it.
type Mentioner interface {
	// Mentionable lists the people who can be mentioned in a comment on a
	// ticket and whose names match query.
	Mentionable(ctx context.Context, id, query string) ([]ticket.User, error)
}

// CommentKind is one way a platform can post a comment, such as a reply
// everyone on the ticket sees or a note kept to the team working it.
type CommentKind struct {
	// ID is what a profile lists in comment_kinds and what AddComment takes.
	ID string
	// Name is the word the comment box shows.
	Name string
}

// The comment kinds that more than one platform has.
const (
	// KindPublic is a comment everyone with the ticket can read.
	KindPublic = "public"
	// KindInternal is a note kept to the team working the ticket.
	KindInternal = "internal"
)

// PickKinds returns the kinds a profile asks for, in the profile's order,
// out of the ones a platform has. It is what a provider's CommentKinds
// answers with.
func PickKinds(want []string, known ...CommentKind) ([]CommentKind, error) {
	var out []CommentKind
	for _, id := range want {
		i := slices.IndexFunc(known, func(k CommentKind) bool { return k.ID == id })
		if i < 0 {
			ids := make([]string, len(known))
			for j, k := range known {
				ids[j] = k.ID
			}
			return nil, fmt.Errorf("[provider] comment_kinds: unknown kind %q (have %v)", id, ids)
		}
		if !slices.Contains(out, known[i]) {
			out = append(out, known[i])
		}
	}
	return out, nil
}

// Factorybuilds a Provider from its config.
type Factory func(Config) (Provider, error)

var (
	mu        sync.RWMutex
	factories = map[string]Factory{}
)

// Register makes a platform available under kind. Platform packages call it
// from init.
func Register(kind string, f Factory) {
	mu.Lock()
	defer mu.Unlock()
	factories[kind] = f
}

// New builds the Provider named by cfg.Kind.
func New(cfg Config) (Provider, error) {
	mu.RLock()
	f, ok := factories[cfg.Kind]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown provider kind %q (have %v)", cfg.Kind, Kinds())
	}
	return f(cfg)
}

// Kinds lists the registered platform kinds.
func Kinds() []string {
	mu.RLock()
	defer mu.RUnlock()
	kinds := make([]string, 0, len(factories))
	for k := range factories {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	return kinds
}
