package profile

import (
	"strings"
	"testing"
	"time"
)

func TestParseAppliesDefaults(t *testing.T) {
	p, err := Parse(`
[provider]
kind = "fake"

[[queue]]
name = "Mine"
query = "assignee=me"
refresh = "90s"

[[queue]]
name = "Board"
layout = "board"

[fields.labels]
id = "field_42"
label = "Tags"
`)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Queues[0].InOrder() {
		t.Error("a queue with no order key is out of the order")
	}
	if got := p.Queues[0].Layout; got != LayoutTable {
		t.Errorf("default layout = %q", got)
	}
	if got := p.Queues[0].Refresh.Duration; got != 90*time.Second {
		t.Errorf("refresh = %v", got)
	}
	if len(p.Queues[0].Columns) == 0 {
		t.Error("no default columns")
	}
	if p.Queues[1].GroupBy != "status" || len(p.Queues[1].Card) == 0 {
		t.Errorf("board defaults missing: %+v", p.Queues[1])
	}
	if len(p.Ticket.Main) == 0 || len(p.Ticket.Sidebar) == 0 {
		t.Error("no default ticket layout")
	}
	if p.Provider.Fields["labels"] != "field_42" {
		t.Errorf("field IDs not passed to the provider: %v", p.Provider.Fields)
	}
	if p.Label("labels") != "Tags" || p.Label("status") != "Status" {
		t.Errorf("labels: %q %q", p.Label("labels"), p.Label("status"))
	}
}

func TestFieldIcons(t *testing.T) {
	p, err := Parse("[provider]\nkind = \"fake\"\n[[queue]]\nname = \"A\"\n[fields.type.icons]\nBug = \"bug\"\n")
	if err != nil {
		t.Fatal(err)
	}
	f := p.Fields["type"]
	if g, ok := f.Icon("bug"); !ok || g != "¤" {
		t.Errorf("a mapped value gives %q, %v", g, ok)
	}
	if g, ok := f.Icon("Task"); !ok || g != "·" {
		t.Errorf("an unmapped value gives %q, %v", g, ok)
	}
	if _, ok := f.Icon(""); ok {
		t.Error("an empty value has an icon")
	}
	f.Icons[AnyValue] = "question"
	if g, _ := f.Icon("Task"); g != "?" {
		t.Errorf("with a default, an unmapped value gives %q", g)
	}
	if _, ok := p.Fields["status"].Icon("Open"); ok {
		t.Error("a field with no icons has one")
	}
}

func TestParseRejectsMistakes(t *testing.T) {
	cases := map[string]string{
		"unknown key":  "[provider]\nkind = \"fake\"\n[[queue]]\nname = \"A\"\ncolour = \"red\"\n",
		"no provider":  "[[queue]]\nname = \"A\"\n",
		"no queue":     "[provider]\nkind = \"fake\"\n",
		"bad layout":   "[provider]\nkind = \"fake\"\n[[queue]]\nname = \"A\"\nlayout = \"grid\"\n",
		"unnamed":      "[provider]\nkind = \"fake\"\n[[queue]]\nquery = \"x\"\n",
		"empty bucket": "[provider]\nkind = \"fake\"\n[[queue]]\nname = \"A\"\n[[queue.bucket]]\nname = \"B\"\n",
		"bad page":     "[provider]\nkind = \"fake\"\n[[queue]]\nname = \"A\"\n[[queue.bucket]]\nname = \"B\"\nquery = \"x\"\npage_size = -1\n",
		"bad icon":     "[provider]\nkind = \"fake\"\n[[queue]]\nname = \"A\"\n[fields.type.icons]\nBug = \"nonesuch\"\n",
		"bad duration": "[provider]\nkind = \"fake\"\n[[queue]]\nname = \"A\"\nrefresh = \"soon\"\n",
	}
	for name, text := range cases {
		if _, err := Parse(text); err == nil {
			t.Errorf("%s: no error", name)
		} else if !strings.Contains(err.Error(), "profile") && name != "bad duration" {
			t.Errorf("%s: error does not say where it came from: %v", name, err)
		}
	}
}

func TestOrderFalseKeepsAQueueOut(t *testing.T) {
	p, err := Parse(`
[provider]
kind = "fake"

[[queue]]
name = "Overview"
order = false

[[queue]]
name = "Mine"
order = true
`)
	if err != nil {
		t.Fatal(err)
	}
	if p.Queues[0].InOrder() || !p.Queues[1].InOrder() {
		t.Errorf("InOrder = %v, %v", p.Queues[0].InOrder(), p.Queues[1].InOrder())
	}
}

func TestRecentQueue(t *testing.T) {
	p, err := Parse(`
[provider]
kind = "fake"

[[queue]]
name = "Recent"
recent = true
`)
	if err != nil {
		t.Fatal(err)
	}
	q := p.Queues[0]
	if q.Days != DefaultRecentDays || q.PageSize != DefaultRecentPageSize || q.InOrder() {
		t.Errorf("days %d, page size %d, in order %v", q.Days, q.PageSize, q.InOrder())
	}
	if len(q.Buckets) != 1 || q.Buckets[0].PageSize != DefaultRecentPageSize || len(q.Buckets[0].Values) != 0 {
		t.Errorf("buckets %+v", q.Buckets)
	}
	if q.Columns[0] != "touched" {
		t.Errorf("columns %v", q.Columns)
	}

	for _, bad := range []string{
		"recent = true\nquery = \"x\"",
		"recent = true\nlayout = \"board\"",
		"recent = true\ndays = -1",
		"recent = true\n[[queue.bucket]]\nname = \"b\"\nvalues = [\"x\"]",
		"query = \"x\"\npage_size = 5",
	} {
		_, err := Parse("[provider]\nkind = \"fake\"\n[[queue]]\nname = \"Q\"\n" + bad)
		if err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}
