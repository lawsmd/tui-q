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

// BuiltIn reports whether name is one of the built-in field names.
func BuiltIn(name string) bool {
	switch name {
	case FieldKey, FieldTitle, FieldStatus, FieldType, FieldAssignee,
		FieldPriority, FieldCreated, FieldUpdated, FieldBody:
		return true
	}
	return false
}

// Value is the content of one field. Single-valued fields use Text;
// multi-valued fields (labels, tags) use Items.
type Value struct {
	Text  string
	Items []string
	// ID is the platform's identifier for the value when the text alone
	// does not pin it down, as with a user.
	ID string
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
	//
	// Code is written as a fenced block, one block for each region of code
	// on the platform, with its lines separated by single newlines and the
	// platform's name for the language after the opening fence when it
	// gives one. provider.Fence writes that form. The ticket view draws a
	// fenced block as a code block and colours it by language.
	Body string

	// Fields holds everything else, keyed by the field name the profile uses.
	Fields map[string]Value

	Comments []Comment

	// Editable describes the fields the platform lets the user change on
	// this ticket. Only a ticket loaded in full has it.
	Editable []FieldSpec
}

// Spec returns the edit description of a field of this ticket.
func (t Ticket) Spec(name string) (FieldSpec, bool) {
	for _, spec := range t.Editable {
		if spec.Name == name {
			return spec, true
		}
	}
	return FieldSpec{}, false
}

// Kinds of field, which decide how a value is entered.
const (
	// KindNone is a field tui-q cannot edit.
	KindNone = ""
	KindText = "text"
	// KindRichText is formatted text. It can be written from plain
	// paragraphs, which drops any formatting the old value had.
	KindRichText = "richtext"
	KindNumber   = "number"
	// KindSelect takes one of Options and KindMulti any number of them.
	KindSelect = "select"
	KindMulti  = "multi"
	// KindTags is a list of free words.
	KindTags = "tags"
	KindUser = "user"
)

// FieldSpec describes one field a ticket or a transition accepts.
type FieldSpec struct {
	// Name is the profile's name for the field, a built-in name, or the
	// platform's own identifier when the profile does not name the field.
	Name string
	// Label is the platform's name for the field.
	Label    string
	Kind     string
	Required bool
	Options  []Option
}

// Option is one allowed value of a select field.
type Option struct {
	ID   string
	Name string
}

// Comment is one existing comment, shown read-only.
type Comment struct {
	Author  string
	Created time.Time
	// Body is Markdown, in the form Ticket.Body describes.
	Body string
	// Kind is a word for a comment that not everyone with the ticket can
	// read, such as "internal". It is empty for any other comment.
	Kind string
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
	// Fields are the values the platform asks for with the transition.
	Fields []FieldSpec
}

// User is a person a ticket can be assigned to.
type User struct {
	ID   string
	Name string
}
