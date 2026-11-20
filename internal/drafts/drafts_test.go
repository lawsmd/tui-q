package drafts

import (
	"os"
	"testing"
)

func TestDraftsAreKeptPerTicketAndRemovedWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir)
	if got := s.Get("DEMO-101"); got != "" {
		t.Fatalf("a ticket with no draft gave %q", got)
	}
	if err := s.Set("DEMO-101", "First **draft**"); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("DEMO-102", "Another"); err != nil {
		t.Fatal(err)
	}

	// A new store reads what the first one wrote.
	again := Open(dir)
	if got := again.Get("DEMO-101"); got != "First **draft**" {
		t.Fatalf("draft read back = %q", got)
	}
	if got := again.Get("DEMO-102"); got != "Another" {
		t.Fatalf("second draft read back = %q", got)
	}

	if err := again.Set("DEMO-101", " \n"); err != nil {
		t.Fatal(err)
	}
	if got := Open(dir).Get("DEMO-101"); got != "" {
		t.Fatalf("a removed draft gave %q", got)
	}
	if files, _ := os.ReadDir(dir); len(files) != 1 {
		t.Fatalf("%d files left, want the one draft that remains", len(files))
	}
	// Removing a draft that was never written is not an error.
	if err := again.Set("DEMO-103", ""); err != nil {
		t.Fatal(err)
	}
}

func TestDraftsWithNoDirectoryStayInMemory(t *testing.T) {
	s := Open("")
	if err := s.Set("DEMO-101", "text"); err != nil {
		t.Fatal(err)
	}
	if got := s.Get("DEMO-101"); got != "text" {
		t.Fatalf("draft = %q", got)
	}
}
