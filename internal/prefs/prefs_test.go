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
