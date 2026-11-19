package tuiq

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lawsmd/tui-q/profile"
	"github.com/lawsmd/tui-q/provider/fake"
)

// orderProfile has a table with a value bucket and a query bucket, a queue
// kept out of the order, and a board.
const orderProfile = `
[provider]
kind = "fake"

[[queue]]
name = "Mine"
query = ""

[[queue.bucket]]
name = "Active"
values = ["in progress", "Waiting"]

[[queue.bucket]]
name = "Backlog"
query = "status=Open"

[[queue]]
name = "Overview"
query = ""
order = false

[[queue]]
name = "Board"
query = ""
layout = "board"
lanes = ["Open", "Blocked", "Done"]
`

// runOrder runs "tq order" against the fake provider and returns what it
// printed.
func runOrder(t *testing.T, text string, args ...string) (string, error) {
	t.Helper()
	prof, err := profile.Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err = order(&out, prof, fake.New(0), args)
	return out.String(), err
}

// inOrder fails the test unless the parts appear in the text one after the
// other.
func inOrder(t *testing.T, text string, parts ...string) {
	t.Helper()
	at := 0
	for _, part := range parts {
		i := strings.Index(text[at:], part)
		if i < 0 {
			t.Fatalf("%q is missing, or out of place, in:\n%s", part, text)
		}
		at += i + len(part)
	}
}

func TestOrderListsQueuesBucketsAndTickets(t *testing.T) {
	out, err := runOrder(t, orderProfile)
	if err != nil {
		t.Fatal(err)
	}
	inOrder(t, out,
		"Working order from tq (fake)",
		"This list gives order only.",
		"Queue 1: Mine (table)",
		"  Active (5)",
		"     1. DEMO-104  [In progress]  Webhook retries stop after the first failure",
		"     5. DEMO-110  [In progress]",
		"  Backlog (5)",
		"     6. DEMO-101  [Open]",
		"    10. DEMO-109  [Open]",
		"  Other (7)",
		"    11. DEMO-101  [Open]",
		"    17. DEMO-112  [Done]",
		"Queue 2: Board (board, lanes by status)",
		"  Open (5)",
		"     1. DEMO-101",
		"  Blocked (0)",
		"  Done (2)",
		"  In progress (3)",
		"  Waiting (2)",
		"    12. DEMO-107",
	)
	if strings.Contains(out, "Overview") {
		t.Errorf("a queue with order = false was printed:\n%s", out)
	}
}

func TestOrderPrintsKeyStatusAndTitleOnly(t *testing.T) {
	out, err := runOrder(t, orderProfile)
	if err != nil {
		t.Fatal(err)
	}
	// The assignee, priority, label and link of DEMO-104 in the fixtures.
	for _, leak := range []string{"sam", "High", "webhooks", "example.com"} {
		if strings.Contains(out, leak) {
			t.Errorf("the output holds %q, which is not a key, a status or a title", leak)
		}
	}
}

func TestOrderLimitCutsEachSection(t *testing.T) {
	out, err := runOrder(t, orderProfile, "--limit", "2")
	if err != nil {
		t.Fatal(err)
	}
	inOrder(t, out,
		"  Active (2 of 5 shown)",
		"    1. DEMO-104",
		"    2. DEMO-105",
		"  Backlog (2 of 5 shown)",
		"    3. DEMO-101",
		"    4. DEMO-102",
		"  Other (2 of 7 shown)",
		"    6. DEMO-102",
		"  Open (2 of 5 shown)",
	)
	for _, bad := range []string{"0", "101"} {
		if _, err := runOrder(t, orderProfile, "--limit", bad); err == nil {
			t.Errorf("--limit %s was accepted", bad)
		}
	}
}

func TestOrderQueueFlag(t *testing.T) {
	out, err := runOrder(t, orderProfile, "--queue", "board")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Queue 1: Board") || strings.Contains(out, "Mine") {
		t.Errorf("--queue board printed:\n%s", out)
	}
	// Naming a queue prints it even when the profile keeps it out.
	out, err = runOrder(t, orderProfile, "--queue", "Overview")
	if err != nil {
		t.Fatal(err)
	}
	inOrder(t, out, "Queue 1: Overview (table, 12 tickets)", "   1. DEMO-101", "  12. DEMO-112")

	_, err = runOrder(t, orderProfile, "--queue", "nope")
	if err == nil || !strings.Contains(err.Error(), "Mine, Overview, Board") {
		t.Errorf("an unknown queue gave %v", err)
	}
}

func TestOrderHideUnmatched(t *testing.T) {
	text := strings.Replace(orderProfile, `query = ""`, "query = \"\"\nhide_unmatched = true", 1)
	out, err := runOrder(t, text, "--queue", "Mine")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "Other") {
		t.Errorf("hide_unmatched left the Other section in:\n%s", out)
	}
}

func TestOrderReportsAQueueThatFails(t *testing.T) {
	text := strings.Replace(orderProfile, `query = "status=Open"`, `query = "not a query"`, 1)
	out, err := runOrder(t, text)
	if err == nil {
		t.Fatalf("a bad query gave no error:\n%s", out)
	}
	inOrder(t, out, "Queue 1: Mine (table)", "could not be read:", "Queue 2: Board")
}
