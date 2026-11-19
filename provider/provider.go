// Package provider defines what tui-q needs from a ticketing platform.
package provider

import (
	"context"
	"fmt"
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

// Factory builds a Provider from its config.
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
