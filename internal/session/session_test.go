package session

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

func TestPutIsListedAndCloseRemovesIt(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "run", "site")
	w := Open(dir)
	if got, err := List(dir); err != nil || len(got) != 0 {
		t.Fatalf("before a Put: %v, %v", got, err)
	}

	tabs := []Tab{
		{Key: "DEMO-104", Status: "In progress", Title: "Webhook retries stop", Links: []Link{{Relation: "blocks", Key: "DEMO-102", Status: "Open"}}},
		{Key: "DEMO-102", Status: "Open", Title: "Export drops rows", Shown: true},
	}
	if err := w.Put(1, tabs); err != nil {
		t.Fatal(err)
	}
	got, err := List(dir)
	if err != nil || len(got) != 1 {
		t.Fatalf("List = %+v, %v, want this process", got, err)
	}
	s := got[0]
	if s.PID != os.Getpid() || len(s.Tabs) != 2 || !s.Tabs[1].Shown || s.Tabs[0].Shown {
		t.Fatalf("state = %+v", s)
	}
	if l := s.Tabs[0].Links; len(l) != 1 || l[0] != tabs[0].Links[0] {
		t.Fatalf("links = %+v", l)
	}

	// A later change replaces the file, and one that arrives late is dropped.
	if err := w.Put(3, tabs[:1]); err != nil {
		t.Fatal(err)
	}
	if err := w.Put(2, tabs); err != nil {
		t.Fatal(err)
	}
	if got, _ = List(dir); len(got) != 1 || len(got[0].Tabs) != 1 {
		t.Fatalf("after an older Put: %+v, want the newer tabs", got)
	}
	if parts, _ := filepath.Glob(filepath.Join(dir, "*.part")); len(parts) != 0 {
		t.Errorf("part files left behind: %v", parts)
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Put(4, tabs); err != nil {
		t.Fatal(err)
	}
	if got, _ = List(dir); len(got) != 0 {
		t.Fatalf("after Close: %+v", got)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("a second Close: %v", err)
	}
}

func TestPutWithNoTabsWritesAnEmptyList(t *testing.T) {
	dir := t.TempDir()
	w := Open(dir)
	defer w.Close()
	if err := w.Put(1, nil); err != nil {
		t.Fatal(err)
	}
	got, err := List(dir)
	if err != nil || len(got) != 1 || got[0].Tabs == nil || len(got[0].Tabs) != 0 {
		t.Fatalf("List = %+v, %v", got, err)
	}
}

func TestListDropsAProcessThatHasEnded(t *testing.T) {
	// A process that has run and been waited for is gone.
	cmd := exec.Command(os.Args[0], "-test.run", "^$")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	dead := cmd.Process.Pid

	dir := t.TempDir()
	data, _ := json.Marshal(State{PID: dead, Tabs: []Tab{{Key: "DEMO-1"}}})
	stale := filepath.Join(dir, strconv.Itoa(dead)+".json")
	if err := os.WriteFile(stale, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "junk.json"), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	w := Open(dir)
	defer w.Close()
	if err := w.Put(1, []Tab{{Key: "DEMO-2"}}); err != nil {
		t.Fatal(err)
	}

	got, err := List(dir)
	if err != nil || len(got) != 1 || got[0].PID != os.Getpid() {
		t.Fatalf("List = %+v, %v, want this process alone", got, err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("the file of the ended process is still there: %v", err)
	}
}
