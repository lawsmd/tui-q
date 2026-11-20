package ado

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/lawsmd/tui-q/provider"
	"github.com/lawsmd/tui-q/ticket"
)

var testFields = map[string]string{
	"tags":     "System.Tags",
	"effort":   "Custom.Field42",
	"priority": "Microsoft.VSTS.Common.Priority",
}

func item(id int, title string, extra string) string {
	return `{"id": ` + itoa(id) + `, "fields": {
	  "System.Id": ` + itoa(id) + `,
	  "System.Title": "` + title + `",
	  "System.State": "New",
	  "System.WorkItemType": "Bug",
	  "System.AssignedTo": {"displayName": "Sam Example", "uniqueName": "sam@example.com"},
	  "System.CreatedDate": "2026-03-04T09:15:00.123Z",
	  "System.ChangedDate": "2026-03-05T10:00:00Z"` + extra + `}}`
}

func itoa(n int) string { return strconv.Itoa(n) }

// fakeServer answers the endpoints the provider calls and records the
// bodies of the batch reads.
type fakeServer struct {
	ids     []int
	items   map[int]string
	batches [][]int
	auth    string
	// posted holds the query and text of each comment added.
	posted []postedComment
}

type postedComment struct {
	query url.Values
	text  string
}

func (f *fakeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.auth = r.Header.Get("Authorization")
	if r.URL.Query().Get("api-version") == "" {
		http.Error(w, `{"message": "no api-version"}`, http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/DEMO/_apis/wit/wiql":
		var body struct{ Query string }
		json.NewDecoder(r.Body).Decode(&body)
		if !strings.HasPrefix(body.Query, "SELECT") {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `{"message": "TF51005: The query references a field that does not exist."}`)
			return
		}
		var refs []string
		for _, id := range f.ids {
			refs = append(refs, `{"id": `+itoa(id)+`}`)
		}
		io.WriteString(w, `{"workItems": [`+strings.Join(refs, ",")+`]}`)
	case r.Method == http.MethodPost && r.URL.Path == "/DEMO/_apis/wit/workitemsbatch":
		var body struct {
			IDs    []int    `json:"ids"`
			Fields []string `json:"fields"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		f.batches = append(f.batches, body.IDs)
		var out []string
		// Answer in reverse to check the provider restores the order.
		for i := len(body.IDs) - 1; i >= 0; i-- {
			if s, ok := f.items[body.IDs[i]]; ok {
				out = append(out, s)
			} else {
				out = append(out, "null")
			}
		}
		io.WriteString(w, `{"count": 1, "value": [`+strings.Join(out, ",")+`]}`)
	case r.Method == http.MethodGet && r.URL.Path == "/DEMO/_apis/wit/workitems/7":
		io.WriteString(w, item(7, "Export drops rows", `,
		  "System.Description": "<p>Rows <b>vanish</b>.</p>",
		  "System.Tags": "export; csv",
		  "Custom.Field42": 8.5`))
	case r.Method == http.MethodGet && r.URL.Path == "/DEMO/_apis/wit/workItems/7/comments":
		if r.URL.Query().Get("order") != "asc" {
			http.Error(w, "order not asc", http.StatusBadRequest)
			return
		}
		rendered := r.URL.Query().Get("$expand") == "renderedText"
		io.WriteString(w, `{"comments": [{"text": "<div>Seen it <i>too</i>.</div>",
		  "createdBy": {"displayName": "Alex Example"}, "createdDate": "2026-03-04T11:00:00Z"},
		  {"text": "Thanks @<0000-42>", "format": "markdown",`+map[bool]string{true: `
		  "renderedText": "<p>Thanks <a href=\"#\" data-vss-mention=\"version:2.0,0000-42\">@Sam Example</a></p>",`}[rendered]+`
		  "createdBy": {"displayName": "Alex Example"}, "createdDate": "2026-03-04T12:00:00Z"}]}`)
	case r.Method == http.MethodPost && r.URL.Path == "/DEMO/_apis/wit/workItems/7/comments":
		var body struct{ Text string }
		json.NewDecoder(r.Body).Decode(&body)
		f.posted = append(f.posted, postedComment{query: r.URL.Query(), text: body.Text})
		io.WriteString(w, `{"id": 1}`)
	case r.Method == http.MethodPost && r.URL.Path == "/_apis/IdentityPicker/Identities":
		var body struct {
			Query         string
			IdentityTypes []string
		}
		json.NewDecoder(r.Body).Decode(&body)
		if body.Query != "sam" || len(body.IdentityTypes) != 1 || body.IdentityTypes[0] != "user" {
			http.Error(w, `{"message": "bad search"}`, http.StatusBadRequest)
			return
		}
		io.WriteString(w, `{"results": [{"identities": [
		  {"entityType": "User", "localId": "0000-42", "displayName": "Sam Example"},
		  {"entityType": "User", "localId": null, "displayName": "Sam Elsewhere"}]}]}`)
	case r.Method == http.MethodGet && r.URL.Path == "/_apis/connectionData":
		io.WriteString(w, `{"authenticatedUser": {"providerDisplayName": "Sam Example",
		  "properties": {"Account": {"$type": "System.String", "$value": "sam@example.com"}}}}`)
	case r.Method == http.MethodGet && r.URL.Path == "/DEMO/_apis/wit/fields":
		io.WriteString(w, `{"value": [{"name": "Tags", "referenceName": "System.Tags"},
		  {"name": "Field 42", "referenceName": "Custom.Field42"}]}`)
	default:
		http.Error(w, `{"message": "not found"}`, http.StatusNotFound)
	}
}

func newTest(t *testing.T, f *fakeServer) *Provider {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return NewProvider(NewClient(srv.URL+"/", PAT("secret")), "DEMO", testFields)
}

func TestSearchKeepsQueryOrderAcrossBatches(t *testing.T) {
	f := &fakeServer{items: map[int]string{}}
	for i := 1; i <= 450; i++ {
		f.ids = append(f.ids, 1000-i)
		f.items[1000-i] = item(1000-i, "T", "")
	}
	// One work item has been deleted since the query ran.
	delete(f.items, 990)
	p := newTest(t, f)
	got, err := p.Search(context.Background(), "SELECT [System.Id] FROM WorkItems")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 449 {
		t.Fatalf("got %d tickets, want 449", len(got))
	}
	if got[0].ID != "999" || got[len(got)-1].ID != "550" {
		t.Errorf("order: first %s last %s", got[0].ID, got[len(got)-1].ID)
	}
	if len(f.batches) != 3 {
		t.Errorf("got %d batch reads, want 3", len(f.batches))
	}
	for _, b := range f.batches {
		if len(b) > batchSize {
			t.Errorf("batch of %d", len(b))
		}
	}
	if !strings.HasPrefix(f.auth, "Basic ") {
		t.Errorf("auth header %q", f.auth)
	}
}

func TestSearchPage(t *testing.T) {
	f := &fakeServer{items: map[int]string{}}
	for i := 1; i <= 25; i++ {
		f.ids = append(f.ids, i)
		f.items[i] = item(i, "T", "")
	}
	p := newTest(t, f)
	ctx := context.Background()
	page, err := p.SearchPage(ctx, "SELECT x", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 25 || len(page.Tickets) != 10 || page.Next != "10" || page.Tickets[0].ID != "1" {
		t.Fatalf("first page: total %d, %d tickets, next %q", page.Total, len(page.Tickets), page.Next)
	}
	page, err = p.SearchPage(ctx, "SELECT x", "20", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Tickets) != 5 || page.Next != "" || page.Tickets[0].ID != "21" {
		t.Fatalf("last page: %d tickets, next %q", len(page.Tickets), page.Next)
	}
	if _, err := p.SearchPage(ctx, "SELECT x", "nope", 10); err == nil {
		t.Error("bad cursor accepted")
	}
}

func TestGetMapsFieldsBodyAndComments(t *testing.T) {
	p := newTest(t, &fakeServer{})
	got, err := p.Get(context.Background(), "7")
	if err != nil {
		t.Fatal(err)
	}
	if got.Key != "7" || got.Title != "Export drops rows" || got.Status != "New" || got.Type != "Bug" || got.Assignee != "Sam Example" {
		t.Errorf("base fields: %+v", got)
	}
	if !strings.HasSuffix(got.URL, "/DEMO/_workitems/edit/7") {
		t.Errorf("URL %q", got.URL)
	}
	if got.Created.IsZero() || got.Created.Nanosecond() == 0 {
		t.Errorf("created %v", got.Created)
	}
	if got.Body != "Rows **vanish**." {
		t.Errorf("body %q", got.Body)
	}
	if tags := got.Fields["tags"].Items; len(tags) != 2 || tags[0] != "export" || tags[1] != "csv" {
		t.Errorf("tags %v", tags)
	}
	if got.Fields["effort"].Text != "8.5" {
		t.Errorf("effort %q", got.Fields["effort"].Text)
	}
	if len(got.Comments) != 2 || got.Comments[0].Author != "Alex Example" || got.Comments[0].Body != "Seen it *too*." ||
		got.Comments[1].Body != "Thanks [@Sam Example](#)" {
		t.Errorf("comments %+v", got.Comments)
	}
	if _, err := p.Get(context.Background(), "DEMO-7"); err == nil {
		t.Error("a key that is no number was accepted")
	}
}

func TestMeAndLabels(t *testing.T) {
	p := newTest(t, &fakeServer{})
	me, err := p.Me(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if me.ID != "sam@example.com" || me.Name != "Sam Example" {
		t.Errorf("me %+v", me)
	}
	labels, err := p.FieldLabels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if labels["effort"] != "Field 42" || labels["tags"] != "Tags" {
		t.Errorf("labels %v", labels)
	}
}

func TestErrorsAndWrites(t *testing.T) {
	p := newTest(t, &fakeServer{})
	_, err := p.Search(context.Background(), "nonsense")
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Status != 400 || !strings.Contains(apiErr.Error(), "TF51005") {
		t.Errorf("error %v", err)
	}
	if err := p.Assign(context.Background(), "7", "x"); !errors.Is(err, ErrReadOnly) {
		t.Errorf("assign: %v", err)
	}
	trs, err := p.Transitions(context.Background(), "7")
	if err != nil || len(trs) != 0 {
		t.Errorf("transitions %v %v", trs, err)
	}
}

func TestResolveAndRegistered(t *testing.T) {
	p := newTest(t, &fakeServer{})
	for in, want := range map[string]string{
		"17":  "17",
		"#17": "17",
		"https://dev.example.com/org/DEMO/_workitems/edit/17": "17",
	} {
		if got, err := p.Resolve(in); err != nil || got != want {
			t.Errorf("Resolve(%q) = %q, %v", in, got, err)
		}
	}
	if _, err := provider.New(provider.Config{Kind: Kind, BaseURL: "https://example.com/org"}); err == nil {
		t.Error("no project accepted")
	}
	t.Setenv("DEMO_ADO_TOKEN", "")
	if _, err := provider.New(provider.Config{Kind: Kind, BaseURL: "https://example.com/org", DefaultProject: "DEMO", TokenEnv: "DEMO_ADO_TOKEN"}); err == nil {
		t.Error("empty token accepted")
	}
}

func TestDecode(t *testing.T) {
	for raw, want := range map[string]ticket.Value{
		`"text"`: ticket.Text("text"),
		`3`:      ticket.Text("3"),
		`8.5`:    ticket.Text("8.5"),
		`true`:   ticket.Text("true"),
		`{"displayName": "Sam Example", "uniqueName": "sam@example.com"}`: {Text: "Sam Example", ID: "sam@example.com"},
		`null`: {},
	} {
		got := decode(json.RawMessage(raw))
		if got.Text != want.Text || got.ID != want.ID {
			t.Errorf("decode(%s) = %+v, want %+v", raw, got, want)
		}
	}
	if got := SplitTags("a; b ;;c"); strings.Join(got, ",") != "a,b,c" {
		t.Errorf("SplitTags %v", got)
	}
}

func TestAddComment(t *testing.T) {
	f := &fakeServer{}
	p := newTest(t, f)
	ctx := context.Background()
	if err := p.AddComment(ctx, "7", "", "Hi [@Sam Example](mention:0000-42), see **this**."); err != nil {
		t.Fatal(err)
	}
	if len(f.posted) != 1 {
		t.Fatalf("posted %d comments", len(f.posted))
	}
	got := f.posted[0]
	if got.query.Get("format") != "markdown" || got.query.Get("api-version") != "7.1-preview.4" {
		t.Errorf("query %v", got.query)
	}
	if got.text != "Hi @<0000-42>, see **this**." {
		t.Errorf("text %q", got.text)
	}
	if p.CommentFormat() != provider.CommentMarkdown || len(p.CommentKinds()) != 0 {
		t.Errorf("format %q kinds %v", p.CommentFormat(), p.CommentKinds())
	}
	for _, c := range []struct{ id, kind, body string }{
		{"DEMO-7", "", "x"},
		{"7", provider.KindInternal, "x"},
		{"7", "", "  \n"},
	} {
		if err := p.AddComment(ctx, c.id, c.kind, c.body); err == nil {
			t.Errorf("AddComment(%q, %q, %q) was accepted", c.id, c.kind, c.body)
		}
	}
	if len(f.posted) != 1 {
		t.Errorf("a refused comment was posted")
	}
}

func TestMentions(t *testing.T) {
	for in, want := range map[string]string{
		"[@Sam](mention:0000-42)":                                            "@<0000-42>",
		"a [@Sam Example](mention:id%2042) b":                                "a @<id 42> b",
		"`[@Sam](mention:1)` and [@Sam](mention:1)":                          "`[@Sam](mention:1)` and @<1>",
		"```\n[@Sam](mention:1)\n```\n[@Sam](mention:1)":                     "```\n[@Sam](mention:1)\n```\n@<1>",
		"~~~~md\n[@Sam](mention:1)\n~~~\nstill code [@Sam](mention:1)\n~~~~": "~~~~md\n[@Sam](mention:1)\n~~~\nstill code [@Sam](mention:1)\n~~~~",
		"[link](https://example.com)":                                        "[link](https://example.com)",
	} {
		if got := Mentions(in); got != want {
			t.Errorf("Mentions(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUsers(t *testing.T) {
	p := newTest(t, &fakeServer{})
	users, err := p.Users(context.Background(), "7", "sam")
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].ID != "0000-42" || users[0].Name != "Sam Example" {
		t.Errorf("users %+v", users)
	}
	if users, err := p.Users(context.Background(), "7", " "); err != nil || users != nil {
		t.Errorf("empty query: %v %v", users, err)
	}
}

func TestNewRefusesCommentKinds(t *testing.T) {
	t.Setenv("TQ_TEST_ADO_TOKEN", "secret")
	_, err := New(provider.Config{BaseURL: "https://example.com/org", DefaultProject: "DEMO", TokenEnv: "TQ_TEST_ADO_TOKEN", CommentKinds: []string{"internal"}})
	if err == nil || !strings.Contains(err.Error(), "comment_kinds") {
		t.Errorf("error %v", err)
	}
}
