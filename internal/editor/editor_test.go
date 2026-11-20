package editor

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lawsmd/tui-q/provider"
)

func TestOpenRefusesAFormatItCannotWrite(t *testing.T) {
	if _, err := Open(Config{Format: "html"}); err == nil || !strings.Contains(err.Error(), "html") {
		t.Fatalf("err = %v", err)
	}
}

// TestRealEditor opens the real editor, which starts a browser, so it runs
// only when asked for:
//
//	TQ_EDITOR_TEST=1 go test ./internal/editor
func TestRealEditor(t *testing.T) {
	if os.Getenv("TQ_EDITOR_TEST") == "" {
		t.Skip("set TQ_EDITOR_TEST=1 to open the real editor")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	const draft = "Fixed in **1.2**.\n\n- one\n- two"

	md, err := Open(Config{Format: provider.CommentMarkdown, Markdown: draft, Title: "comment on DEMO-101"})
	if err != nil {
		t.Fatal(err)
	}
	if got := md.Text(); got != draft {
		t.Errorf("text = %q", got)
	}
	out, err := md.Export(ctx)
	if err != nil || !strings.Contains(out, "**1.2**") {
		t.Errorf("markdown export = %q, %v", out, err)
	}
	md.Close()
	if msg := md.Changes()(); msg != nil {
		t.Errorf("a closed box reported %#v", msg)
	}

	jira, err := Open(Config{Format: provider.CommentJira, Markdown: draft})
	if err != nil {
		t.Fatal(err)
	}
	defer jira.Close()
	out, err = jira.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Type    string            `json:"type"`
		Content []json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil || doc.Type != "doc" || len(doc.Content) != 2 {
		t.Errorf("jira export = %s (%v)", out, err)
	}
}
