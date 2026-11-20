// Package profile loads the TOML file that tells tui-q which platform to
// talk to, which queues to show, and how tickets are laid out.
package profile

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/lawsmd/tui-q/icon"
	"github.com/lawsmd/tui-q/provider"
	"github.com/lawsmd/tui-q/ticket"
)

// EnvOverride names the environment variable that, when set, points at a
// profile file to load in place of the one compiled into the binary.
const EnvOverride = "TQ_PROFILE"

// Layouts a queue can use.
const (
	LayoutTable = "table"
	LayoutBoard = "board"
)

// Profile is one whole profile file.
type Profile struct {
	Provider provider.Config  `toml:"provider"`
	Queues   []Queue          `toml:"queue"`
	Ticket   TicketView       `toml:"ticket"`
	Fields   map[string]Field `toml:"fields"`
}

// Queue is one top-level tab.
type Queue struct {
	Name string `toml:"name"`
	// Query is written in the platform's own query language.
	Query  string `toml:"query"`
	Layout string `toml:"layout"`
	// Columns are the fields shown in a table, most important first. When
	// the pane is narrow the last columns are dropped first.
	Columns []string `toml:"columns"`
	// GroupBy is the field a board's lanes come from, and Lanes fixes their
	// order. Card lists the fields drawn on each card.
	GroupBy string   `toml:"group_by"`
	Lanes   []string `toml:"lanes"`
	Card    []string `toml:"card"`
	// Buckets split a table into titled sections, drawn from top to
	// bottom. A ticket goes in the first bucket it matches, and a ticket
	// that matches none goes in a last section named Other.
	Buckets []Bucket `toml:"bucket"`
	// HideUnmatched leaves out the tickets that match no bucket, so the
	// table has no Other section.
	HideUnmatched bool `toml:"hide_unmatched"`
	// Refresh is how often the queue reloads on its own. Zero means only
	// on request.
	Refresh Duration `toml:"refresh"`
	// Order set to false keeps the queue out of "tq order". Read it
	// through InOrder.
	Order *bool `toml:"order"`
}

// InOrder reports whether "tq order" lists the queue. It does unless the
// profile says order = false.
func (q *Queue) InOrder() bool {
	return q.Order == nil || *q.Order
}

// DefaultPageSize is the page size of a bucket that has its own query and
// names no size.
const DefaultPageSize = 10

// Bucket is one section of a table. It holds the tickets of the queue
// whose field has one of the values, or the tickets of its own query.
type Bucket struct {
	Name string `toml:"name"`
	// Field defaults to the status.
	Field  string   `toml:"field"`
	Values []string `toml:"values"`
	// Query, when set, is run for this bucket alone, one page at a time,
	// and Field and Values are not used. It suits a bucket too large to
	// load whole.
	Query string `toml:"query"`
	// PageSize is how many tickets the bucket shows at once. Zero shows
	// them all, which a bucket with its own query cannot do.
	PageSize int `toml:"page_size"`
}

// OwnQueries reports whether every bucket of the queue has its own query,
// so that the query of the queue itself is not needed.
func (q *Queue) OwnQueries() bool {
	for _, b := range q.Buckets {
		if b.Query == "" {
			return false
		}
	}
	return len(q.Buckets) > 0
}

// TicketView lists the fields of the ticket view, in display order.
type TicketView struct {
	Main    []string `toml:"main"`
	Sidebar []string `toml:"sidebar"`
	// Comments shows the comment section when true.
	Comments bool `toml:"comments"`
}

// Field describes a field beyond the built-in ones.
type Field struct {
	// ID is the platform's identifier for the field.
	ID string `toml:"id"`
	// Type is "text", "select", "list" or "user".
	Type  string `toml:"type"`
	Label string `toml:"label"`
	// Icons maps a value of the field to the name of an icon in package
	// icon. The key "*" names the icon for every other value. A table
	// shows the icon in place of the value.
	Icons map[string]string `toml:"icons"`
}

// AnyValue is the key in Icons for the values that have no entry.
const AnyValue = "*"

// Icon returns the glyph for a value of the field. It reports false when
// the field has no icons or the value is empty.
func (f Field) Icon(value string) (string, bool) {
	if len(f.Icons) == 0 || value == "" {
		return "", false
	}
	name, ok := f.Icons[value]
	if !ok {
		for v, n := range f.Icons {
			if strings.EqualFold(v, value) {
				name, ok = n, true
				break
			}
		}
	}
	if !ok {
		name, ok = f.Icons[AnyValue]
	}
	if !ok {
		return icon.Other, true
	}
	g, _ := icon.Glyph(name)
	return g, true
}

// Duration lets TOML hold values like "90s".
type Duration struct{ time.Duration }

// UnmarshalText parses a Go duration string.
func (d *Duration) UnmarshalText(text []byte) error {
	v, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	d.Duration = v
	return nil
}

// Parse reads a profile from TOML text and checks it.
func Parse(text string) (*Profile, error) {
	var p Profile
	meta, err := toml.Decode(text, &p)
	if err != nil {
		return nil, fmt.Errorf("profile: %w", err)
	}
	if extra := meta.Undecoded(); len(extra) > 0 {
		return nil, fmt.Errorf("profile: unknown key %q", extra[0].String())
	}
	if err := p.finish(); err != nil {
		return nil, err
	}
	return &p, nil
}

// Load returns the profile from the file named by TQ_PROFILE when that is
// set, and from the embedded text otherwise.
func Load(embedded string) (*Profile, error) {
	if path := os.Getenv(EnvOverride); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("profile: %w", err)
		}
		return Parse(string(data))
	}
	return Parse(embedded)
}

func (p *Profile) finish() error {
	if p.Provider.Kind == "" {
		return fmt.Errorf("profile: [provider] kind is required")
	}
	if len(p.Queues) == 0 {
		return fmt.Errorf("profile: at least one [[queue]] is required")
	}
	p.Provider.Fields = map[string]string{}
	for name, f := range p.Fields {
		if f.ID != "" {
			p.Provider.Fields[name] = f.ID
		}
		for value, n := range f.Icons {
			if _, ok := icon.Glyph(n); !ok {
				return fmt.Errorf("profile: field %q: no icon named %q for %q", name, n, value)
			}
		}
	}
	for i := range p.Queues {
		q := &p.Queues[i]
		if q.Name == "" {
			return fmt.Errorf("profile: queue %d has no name", i+1)
		}
		switch q.Layout {
		case "":
			q.Layout = LayoutTable
		case LayoutTable, LayoutBoard:
		default:
			return fmt.Errorf("profile: queue %q: layout must be %q or %q", q.Name, LayoutTable, LayoutBoard)
		}
		if len(q.Columns) == 0 {
			q.Columns = []string{ticket.FieldKey, ticket.FieldTitle, ticket.FieldStatus, ticket.FieldAssignee}
		}
		if q.HideUnmatched && len(q.Buckets) == 0 {
			return fmt.Errorf("profile: queue %q: hide_unmatched needs buckets", q.Name)
		}
		if len(q.Buckets) > 0 && q.Layout != LayoutTable {
			return fmt.Errorf("profile: queue %q: only a table has buckets", q.Name)
		}
		for j := range q.Buckets {
			b := &q.Buckets[j]
			if b.Name == "" || (len(b.Values) == 0 && b.Query == "") {
				return fmt.Errorf("profile: queue %q: bucket %d needs a name, and values or a query", q.Name, j+1)
			}
			if b.PageSize < 0 {
				return fmt.Errorf("profile: queue %q: bucket %q: page_size is negative", q.Name, b.Name)
			}
			if b.Query != "" && b.PageSize == 0 {
				b.PageSize = DefaultPageSize
			}
			if b.Field == "" {
				b.Field = ticket.FieldStatus
			}
		}
		if q.Layout == LayoutBoard {
			if q.GroupBy == "" {
				q.GroupBy = ticket.FieldStatus
			}
			if len(q.Card) == 0 {
				q.Card = []string{ticket.FieldKey, ticket.FieldTitle, ticket.FieldAssignee}
			}
		}
	}
	if len(p.Ticket.Main) == 0 {
		p.Ticket.Main = []string{ticket.FieldTitle, ticket.FieldBody}
	}
	if len(p.Ticket.Sidebar) == 0 {
		p.Ticket.Sidebar = []string{ticket.FieldStatus, ticket.FieldType, ticket.FieldAssignee, ticket.FieldPriority}
	}
	return nil
}

// Label returns the heading for a field: the profile's label when it gives
// one, and the capitalised field name otherwise.
func (p *Profile) Label(name string) string {
	if f, ok := p.Fields[name]; ok && f.Label != "" {
		return f.Label
	}
	if name == "" {
		return ""
	}
	return strings.ToUpper(name[:1]) + name[1:]
}
