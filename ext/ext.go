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

// DetailsSection is the title of the sidebar section that holds the
// profile's fields.
const DetailsSection = "Details"

// Action is something an instance can do to a ticket beyond what the core
// does. It is shown as a button in the ticket view and as a line in the
// context menu of a row or card.
type Action struct {
	// Name is the text of the button and of the menu line.
	Name string
	// Section is the title of the sidebar section whose button the action
	// is: DetailsSection, or the Title of one of the instance's sections.
	// Its button is drawn at the foot of that section. Empty, or the title
	// of a section not shown for the ticket, puts the button in the row
	// over the title.
	Section string
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

// Fields are fields an instance adds to the sidebar of the ticket view for
// some tickets. They are drawn under the profile's sidebar fields, in the
// order given, and are shown and changed as those are. A field the sidebar
// already has is not drawn again.
type Fields struct {
	// Names are the fields, by the names the profile gives them. A field
	// of the platform has to be in the profile's [fields] to be read.
	Names []string
	// Applies reports whether the fields are shown for a ticket. Nil shows
	// them for every ticket.
	Applies func(t ticket.Ticket) bool
}

// Section is a block of lines an instance adds to the top of the sidebar of
// the ticket view, above the profile's fields. It is read each time the
// ticket is loaded and after every change made to it.
type Section struct {
	// Title is the name on the rule over the section.
	Title string
	// Applies reports whether the section is shown for a ticket. Nil shows
	// it for every ticket.
	Applies func(t ticket.Ticket) bool
	// Read returns the lines of the section for a ticket. It runs as a
	// read: input stays free, and an error is shown in place of the lines.
	Read func(ctx context.Context, t ticket.Ticket) ([]Line, error)
}

// Line is one line of a section. A line with actions can be clicked, and a
// click opens a menu of them. A chosen action runs as one from WithActions
// does, and the section is read again after it.
type Line struct {
	Text    string
	Faint   bool
	Actions []Action
}
