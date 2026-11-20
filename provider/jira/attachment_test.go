package jira

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/lawsmd/tui-q/ticket"
)

func TestMediaBecomesAReferenceToItsAttachment(t *testing.T) {
	atts := []ticket.Attachment{
		{ID: "10001", Name: "shot.png", MIME: "image/png"},
		{ID: "10002", Name: "log.txt", MIME: "text/plain"},
		{ID: "10003", Name: "shot.png", MIME: "image/png"},
	}
	adf := `{"type":"doc","content":[
		{"type":"paragraph","content":[{"type":"text","text":"See:"}]},
		{"type":"mediaSingle","content":[{"type":"media","attrs":{"id":"m-1","type":"file","alt":"shot.png"}}]},
		{"type":"mediaGroup","content":[
			{"type":"media","attrs":{"id":"m-2","type":"file","alt":"log.txt"}},
			{"type":"media","attrs":{"id":"m-3","type":"file","alt":"gone.png"}}]},
		{"type":"paragraph","content":[{"type":"text","text":"inline "},{"type":"mediaInline","attrs":{"id":"m-4"}}]},
		{"type":"mediaSingle","content":[{"type":"media","attrs":{"id":"m-5","type":"file"}}]},
		{"type":"mediaSingle","content":[{"type":"media","attrs":{"id":"m-5","type":"file","alt":"log.txt"}}]}
	]}`
	// Of two files with one name the newer is taken. The ID of the media
	// store, where it is known, counts for more than the name.
	want := "See:\n\n![shot.png](attachment:10003)\n\n![log.txt](attachment:10002)\n\n*[attachment]*\n\ninline *[attachment]*" +
		"\n\n![shot.png](attachment:10001)\n\n![shot.png](attachment:10001)"
	if got := adfToMarkdown(json.RawMessage(adf), files{all: atts, byMedia: map[string]ticket.Attachment{"m-5": atts[0]}}); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestGetListsAttachmentsAndReadsOne(t *testing.T) {
	const stored = "0a1b2c3d-1111-2222-3333-444455556666"
	var (
		asked string
		mu    sync.Mutex
		sent  int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/rest/api/3/issue/DEMO-101":
			asked = r.URL.Query().Get("fields")
			// The description names the file by its ID in the media store,
			// and the comment by its name.
			io.WriteString(w, `{"id":"1","key":"DEMO-101","fields":{
				"summary":"Title",
				"attachment":[{"id":"10001","filename":"shot.png","mimeType":"image/png","size":7}],
				"description":{"type":"doc","content":[{"type":"mediaSingle","content":[{"type":"media","attrs":{"id":"`+stored+`","type":"file"}}]}]},
				"comment":{"comments":[{"body":{"type":"doc","content":[{"type":"mediaSingle","content":[{"type":"media","attrs":{"id":"other","alt":"shot.png"}}]}]}}]}
			}}`)
		case r.URL.Path == "/rest/api/3/attachment/content/10001":
			if _, _, ok := r.BasicAuth(); !ok {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			mu.Lock()
			sent++
			mu.Unlock()
			http.Redirect(w, r, "/file/"+stored+"/binary?token=t", http.StatusSeeOther)
		case r.URL.Path == "/file/"+stored+"/binary":
			io.WriteString(w, "picture")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	p := NewProvider(NewClient(srv.URL, "sam@example.com", "token"), nil)

	tk, err := p.Get(context.Background(), "DEMO-101")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(asked, "attachment") {
		t.Errorf("fields asked for: %q", asked)
	}
	want := ticket.Attachment{ID: "10001", Name: "shot.png", MIME: "image/png", Size: 7}
	if len(tk.Attachments) != 1 || tk.Attachments[0] != want {
		t.Fatalf("attachments %+v", tk.Attachments)
	}
	if tk.Body != want.Ref() || tk.Comments[0].Body != want.Ref() {
		t.Errorf("body %q, comment %q", tk.Body, tk.Comments[0].Body)
	}
	// The ID of the media store is asked for once for each attachment.
	if _, err := p.Get(context.Background(), "DEMO-101"); err != nil || sent != 1 {
		t.Fatalf("second Get: err %v, %d requests for the attachment", err, sent)
	}
	body, err := p.ReadAttachment(context.Background(), want)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	if data, _ := io.ReadAll(body); string(data) != "picture" {
		t.Errorf("read %q", data)
	}
}
