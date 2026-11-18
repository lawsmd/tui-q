// Package ext is how an instance module adds behaviour of its own to
// tui-q. An instance builds the values defined here and passes them to
// tuiq.Run.
package ext

import (
	"context"
	"time"

	"github.com/lawsmd/tui-q/provider"
	"github.com/lawsmd/tui-q/ticket"
)

// DefaultTimeout bounds an action that sets no timeout of its own.
const DefaultTimeout = 5 * time.Minute

// Action is something an instance can do to a ticket beyond what the core
// does. It is shown as a button in the ticket view and as a line in the
// context menu of a row or card.
type Action struct {
	// Name is the text of the button and of the menu line.
	Name string
	// Applies reports whether the action is offered for a ticket. Nil
	// offers it for every ticket.
	Applies func(t ticket.Ticket) bool
	// Fields lists what to ask the user before the action runs. The
	// answers are collected in a form, and cancelling the form cancels the
	// action. Nil, or an empty list, runs the action at once.
	Fields func(t ticket.Ticket) []ticket.FieldSpec
	// Timeout bounds one run. Zero means DefaultTimeout.
	Timeout time.Duration
	// Run does the work. Input is locked while it runs. The text it
	// returns is shown in the status bar when it ends; the ticket is then
	// read again and every queue reloads.
	Run func(ctx context.Context, r Run) (string, error)
}

// Run is what one run of an action is given.
type Run struct {
	// Ticket is the ticket as the interface last read it. From a queue it
	// holds only what the queue's search returned.
	Ticket ticket.Ticket
	// Values are the answers to the action's Fields, by field name.
	Values map[string]ticket.Value
	// Provider is the platform the ticket came from. An action that needs
	// more than the interface offers asserts it to the platform's own
	// type and uses its client.
	Provider provider.Provider
	// Progress names the step that is starting, for the status bar.
	Progress func(step string)
}
