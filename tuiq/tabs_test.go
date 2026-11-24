package tuiq

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/lawsmd/tui-q/internal/session"
)

func TestTabsListsTheTicketsOfARunningInterface(t *testing.T) {
	dir := t.TempDir()
	var out strings.Builder
	if err := tabs(&out, dir, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "Tickets open in tq\n"+tabsNotice) || !strings.HasSuffix(out.String(), "\nNo tq is running.\n") {
		t.Fatalf("with nothing running:\n%s", out.String())
	}

	// This process stands for a running interface.
	w := session.Open(dir)
	defer w.Close()
	if err := w.Put(1, []session.Tab{
		{Key: "DEMO-104", Status: "In progress", Title: "Webhook retries stop after the first failure", Links: []session.Link{
			{Relation: "blocks", Key: "DEMO-102", Status: "Open"},
			{Relation: "relates to", Key: "OTHER-7", Status: "Done"},
		}},
		{Key: "DEMO-9", Status: "Open", Title: "Export drops rows", Shown: true},
	}); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := tabs(&out, dir, nil); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf(`
tq %d
  1. DEMO-104  [In progress]  Webhook retries stop after the first failure
       blocks      DEMO-102  [Open]
       relates to  OTHER-7   [Done]
* 2. DEMO-9    [Open]  Export drops rows
`, os.Getpid())
	if got := strings.TrimPrefix(out.String(), "Tickets open in tq\n"+tabsNotice); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}

	// An interface showing a queue has no ticket open.
	if err := w.Put(2, nil); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := tabs(&out, dir, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(out.String(), fmt.Sprintf("\ntq %d\n  no ticket is open\n", os.Getpid())) {
		t.Errorf("with no ticket open:\n%s", out.String())
	}
}

func TestTabsTakesNoArgument(t *testing.T) {
	var out strings.Builder
	if err := tabs(&out, t.TempDir(), []string{"DEMO-1"}); err == nil || out.Len() != 0 {
		t.Fatalf("err = %v, printed %q", err, out.String())
	}
}
