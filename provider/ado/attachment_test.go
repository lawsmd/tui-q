package ado

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lawsmd/tui-q/ticket"
)

const guid = "0a1b2c3d-1111-2222-3333-444455556666"

func TestAnAttachedImageBecomesAReference(t *testing.T) {
	src := "https://dev.azure.com/example/9f8e7d6c-0000-0000-0000-000000000000/_apis/wit/attachments/" + guid + "?fileName=shot.png"
	html := `<div>Before <img src="` + src + `" alt="Image"> after</div>` +
		`<div><img src="` + src + `"></div>` +
		`<div><img src="https://example.com/logo.png" alt="logo"></div>`
	want := ticket.Attachment{ID: guid, Name: "shot.png", MIME: "image/png"}
	text, found := convert(html)
	if wantText := "Before \n\n" + want.Ref() + "\n\nafter\n\n" + want.Ref() + "\n\n[image: logo]"; text != wantText {
		t.Errorf("got  %q\nwant %q", text, wantText)
	}
	if len(found) != 1 || found[0] != want {
		t.Errorf("attachments %+v", found)
	}
	if HTMLToMarkdown(html) != text {
		t.Error("HTMLToMarkdown and convert differ")
	}
}

func TestReadAttachment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/Demo/_apis/wit/attachments/" + guid:
			if _, token, ok := r.BasicAuth(); !ok || token != "pat" || r.URL.Query().Get("api-version") == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			io.WriteString(w, "picture")
		default:
			// What Azure DevOps answers a request it will not sign in.
			w.Header().Set("Content-Type", "text/html")
			io.WriteString(w, "<html>Sign in</html>")
		}
	}))
	defer srv.Close()
	p := NewProvider(NewClient(srv.URL, PAT("pat")), "Demo", nil)
	ctx := context.Background()

	body, err := p.ReadAttachment(ctx, ticket.Attachment{ID: guid})
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	if data, _ := io.ReadAll(body); string(data) != "picture" {
		t.Errorf("read %q", data)
	}
	if _, err := p.ReadAttachment(ctx, ticket.Attachment{ID: "../../other"}); err == nil {
		t.Error("an ID that is no attachment's was sent")
	}
	if _, err := p.client.Raw(ctx, http.MethodGet, "/elsewhere", nil, nil); err == nil {
		t.Error("a sign-in page was taken for a file")
	}
}
