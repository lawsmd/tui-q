package attach

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lawsmd/tui-q/ticket"
)

func TestSaveKeepsAFileUnderAHashedName(t *testing.T) {
	s := Open(t.TempDir())
	a := ticket.Attachment{ID: "10001", Name: "run me.exe", MIME: "image/png"}
	if _, ok := s.Path(a); ok {
		t.Fatal("a file before any was saved")
	}
	p, err := s.Save(a, strings.NewReader("bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(p) != ".png" || strings.Contains(p, "run") {
		t.Fatalf("path %q: want a hashed name ending .png", p)
	}
	if got, ok := s.Path(a); !ok || got != p {
		t.Fatalf("Path = %q, %v; want %q", got, ok, p)
	}
	if data, _ := os.ReadFile(p); string(data) != "bytes" {
		t.Fatalf("file holds %q", data)
	}
	// Anything that is not a picture gets no extension.
	p, err = s.Save(ticket.Attachment{ID: "10002", Name: "tool.exe", MIME: "application/octet-stream"}, strings.NewReader("x"))
	if err != nil || filepath.Ext(p) != "" {
		t.Fatalf("path %q, err %v: want no extension", p, err)
	}
	entries, _ := os.ReadDir(filepath.Dir(p))
	if len(entries) != 2 {
		t.Fatalf("%d files left in the directory, want 2", len(entries))
	}
}

func TestSaveWithNoDirectoryFails(t *testing.T) {
	if _, err := Open("").Save(ticket.Attachment{ID: "1"}, strings.NewReader("x")); err == nil {
		t.Fatal("no error")
	}
}
