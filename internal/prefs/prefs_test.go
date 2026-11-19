package prefs

import (
	"path/filepath"
	"testing"
)

func TestSettingsAreSavedAndReadBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "site.json")
	s := Open(path)
	if s.Column("assignee").FirstName {
		t.Fatal("a new store has a setting")
	}
	if err := s.SetColumn("assignee", Column{FirstName: true}); err != nil {
		t.Fatal(err)
	}
	if !Open(path).Column("assignee").FirstName {
		t.Fatal("the setting was not read back")
	}
	if err := s.SetView("Open", View{Show: []string{"key", "title"}}); err != nil {
		t.Fatal(err)
	}
	if got := Open(path).View("Open").Show; len(got) != 2 || got[1] != "title" {
		t.Fatalf("view read back = %v", got)
	}
	if err := s.SetView("Open", View{Widths: map[string]int{"status": 9}}); err != nil {
		t.Fatal(err)
	}
	if got := Open(path).View("Open"); got.Widths["status"] != 9 || len(got.Show) != 0 {
		t.Fatalf("widths read back = %+v", got)
	}
	if err := s.SetCommentsOldestFirst(true); err != nil {
		t.Fatal(err)
	}
	if !Open(path).CommentsOldestFirst {
		t.Fatal("the comment order was not read back")
	}
	if err := s.SetColumn("assignee", Column{}); err != nil {
		t.Fatal(err)
	}
	if got := Open(path).Columns; len(got) != 0 {
		t.Fatalf("columns after clearing = %v", got)
	}
}

func TestStoreWithNoPathKeepsSettingsInMemory(t *testing.T) {
	s := Open("")
	if err := s.SetColumn("assignee", Column{FirstName: true}); err != nil {
		t.Fatal(err)
	}
	if !s.Column("assignee").FirstName {
		t.Fatal("the setting was lost")
	}
}
