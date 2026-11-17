// Package ticket holds the platform-neutral model every provider maps into.
package ticket

import (
	"strings"
	"time"
)

// Built-in field names. A profile refers to these and to its own custom
// field names with the same strings.
const (
	FieldKey      = "key"
	FieldTitle    = "title"
	FieldStatus   = "status"
	FieldType     = "type"
	FieldAssignee = "assignee"
	FieldPriority = "priority"
	FieldCreated  = "created"
	FieldUpdated  = "updated"
	FieldBody     = "body"
)

// Value is the content of one field. Single-valued fields use Text;
// multi-valued fields (labels, tags) use Items.
type Value struct {
	Text  string
	Items []string
}

// String returns the value as one line of text.
func (v Value) String() string {
	if len(v.Items) > 0 {
		return strings.Join(v.Items, ", ")
	}
	return v.Text
}

// Text builds a single-valued Value.
func Text(s string) Value { return Value{Text: s} }

// List builds a multi-valued Value.
func List(items ...string) Value { return Value{Items: items} }

// Ticket is one work item on any platform.
type Ticket struct {
	// ID is what the platform's API takes. Key is what a person reads.
	// On some platforms they are the same string.
	ID  string
	Key string

	Title    string
	Status   string
	Type     string
	Assignee string
	Priority string
	URL      string
	Created  time.Time
	Updated  time.Time

	// Body is the description, converted to Markdown by the provider.
	Body string

	// Fields holds everything else, keyed by the field name the profile uses.
	Fields map[string]Value

	Comments []Comment
}

// Comment is one existing comment, shown read-only.
type Comment struct {
	Author  string
	Created time.Time
	Body    string
}

// Field returns the named field as a Value. Built-in names read the struct;
// any other name reads Fields.
func (t Ticket) Field(name string) Value {
	switch name {
	case FieldKey:
		return Text(t.Key)
	case FieldTitle:
		return Text(t.Title)
	case FieldStatus:
		return Text(t.Status)
	case FieldType:
		return Text(t.Type)
	case FieldAssignee:
		return Text(t.Assignee)
	case FieldPriority:
		return Text(t.Priority)
	case FieldBody:
		return Text(t.Body)
	case FieldCreated:
		return Text(formatTime(t.Created))
	case FieldUpdated:
		return Text(formatTime(t.Updated))
	}
	return t.Fields[name]
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04")
}

// Transition is one status change the platform allows for a ticket right now.
type Transition struct {
	ID   string
	Name string
	// To is the name of the status the ticket ends in.
	To string
}

// User is a person a ticket can be assigned to.
type User struct {
	ID   string
	Name string
}
