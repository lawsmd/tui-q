package jira

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lawsmd/tui-q/provider"
	"github.com/lawsmd/tui-q/ticket"
)

const editMetaJSON = `{"fields": {
  "summary": {"required": true, "name": "Summary", "schema": {"type": "string", "system": "summary"}},
  "assignee": {"required": false, "name": "Assignee", "schema": {"type": "user", "system": "assignee"}},
  "priority": {"required": false, "name": "Priority", "schema": {"type": "priority", "system": "priority"},
    "allowedValues": [{"id": "1", "name": "High"}, {"id": "3", "name": "Low"}]},
  "labels": {"required": false, "name": "Labels", "schema": {"type": "array", "items": "string", "system": "labels"}},
  "field_43": {"required": false, "name": "Sales regions", "schema": {"type": "array", "items": "option", "custom": "x:multiselect"},
    "allowedValues": [{"id": "201", "value": "North"}, {"id": "202", "value": "South"}]},
  "field_44": {"required": false, "name": "Seat count", "schema": {"type": "number", "custom": "x:float"}},
  "field_45": {"required": false, "name": "Notes for the next shift", "schema": {"type": "string", "custom": "x:textarea"}},
  "issuetype": {"required": true, "name": "Issue Type", "schema": {"type": "issuetype", "system": "issuetype"}}
}}`

const issueJSON = `{
  "id": "10001",
  "key": "DEMO-7",
  "editmeta": ` + editMetaJSON + `,
  "fields": {
    "summary": "Export drops rows",
    "status": {"name": "Open"},
    "issuetype": {"name": "Bug"},
    "assignee": {"displayName": "Sam Example", "accountId": "abc"},
    "priority": {"name": "High"},
    "created": "2026-03-04T09:15:00.000+0100",
    "updated": "2026-03-05T10:00:00.000+0100",
    "labels": ["export", "csv"],
    "field_42": {"value": "Reports", "child": {"value": "Monthly"}},
    "field_43": [{"value": "North"}, {"value": "South"}],
    "field_44": 3,
    "field_46": {"requestType": {"id": "9", "name": "Report a fault"}, "currentStatus": {"status": "Open"}},
    "description": {"type": "doc", "version": 1, "content": [
      {"type": "paragraph", "content": [{"type": "text", "text": "Rows vanish."}]}
    ]},
    "comment": {"comments": [
      {"author": {"displayName": "Alex Example"}, "created": "2026-03-04T11:00:00.000+0100",
       "body": {"type": "doc", "version": 1, "content": [
         {"type": "paragraph", "content": [{"type": "text", "text": "Seen it too.", "marks": [{"type": "strong"}]}]}
       ]}}
    ]}
  }
}`

var testFields = map[string]string{
	"labels":   "labels",
	"category": "field_42",
	"regions":  "field_43",
	"count":    "field_44",
	"notes":    "field_45",
	"request":  "field_46",
}

// server answers the endpoints the provider uses and records what it was
// sent.
type server struct {
	*httptest.Server
	requests []string
	bodies   []string
	auth     string
}

func newServer(t *testing.T) *server {
	t.Helper()
	s := &server{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.requests = append(s.requests, r.Method+" "+r.URL.Path)
		s.bodies = append(s.bodies, string(body))
		s.auth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/3/search/jql":
			var req struct {
				NextPageToken string `json:"nextPageToken"`
			}
			_ = json.Unmarshal(body, &req)
			if req.NextPageToken == "" {
				io.WriteString(w, `{"issues": [`+issueJSON+`], "nextPageToken": "page2"}`)
			} else {
				io.WriteString(w, `{"issues": [`+strings.Replace(issueJSON, "DEMO-7", "DEMO-8", 1)+`]}`)
			}
		case r.URL.Path == "/rest/api/3/search/approximate-count":
			io.WriteString(w, `{"count": 412}`)
		case r.URL.Path == "/rest/api/3/issue/DEMO-7" && r.Method == http.MethodGet:
			io.WriteString(w, issueJSON)
		case r.URL.Path == "/rest/api/3/issue/DEMO-7/transitions" && r.Method == http.MethodGet:
			io.WriteString(w, `{"transitions": [
				{"id": "11", "name": "Start work", "to": {"name": "In progress"}},
				{"id": "21", "name": "Close", "to": {"name": "Done"}, "fields": {
					"resolution": {"required": true, "name": "Resolution", "schema": {"type": "resolution", "system": "resolution"},
						"allowedValues": [{"id": "5", "name": "Fixed"}, {"id": "6", "name": "Duplicate"}]}}}
			]}`)
		case r.URL.Path == "/rest/api/3/issue/DEMO-7/editmeta":
			io.WriteString(w, editMetaJSON)
		case r.URL.Path == "/rest/api/3/myself":
			io.WriteString(w, `{"accountId": "me-1", "displayName": "Sam Example"}`)
		case r.URL.Path == "/rest/api/3/user/assignable/search":
			io.WriteString(w, `[{"accountId": "u-2", "displayName": "Alex Example"}]`)
		case r.URL.Path == "/rest/api/3/field":
			io.WriteString(w, `[{"id": "field_43", "name": "Sales regions"}, {"id": "field_99", "name": "Unused"}, {"id": "labels", "name": "Labels"}]`)
		case r.URL.Path == "/rest/api/3/issue/MISSING-1":
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, `{"errorMessages": ["Issue does not exist"], "errors": {}}`)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	t.Cleanup(s.Close)
	return s
}

func newProvider(s *server) *Provider {
	return NewProvider(NewClient(s.URL+"/", "user@example.com", "secret"), testFields)
}

func TestSearchFollowsPages(t *testing.T) {
	s := newServer(t)
	got, err := newProvider(s).Search(context.Background(), "project = DEMO")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Key != "DEMO-7" || got[1].Key != "DEMO-8" {
		t.Fatalf("got %d tickets: %+v", len(got), got)
	}
	if len(s.requests) != 2 {
		t.Fatalf("made %d requests, want 2", len(s.requests))
	}
	var first struct {
		JQL    string   `json:"jql"`
		Fields []string `json:"fields"`
	}
	if err := json.Unmarshal([]byte(s.bodies[0]), &first); err != nil {
		t.Fatal(err)
	}
	if first.JQL != "project = DEMO" {
		t.Errorf("jql = %q", first.JQL)
	}
	if joined := strings.Join(first.Fields, ","); !strings.Contains(joined, "summary") || !strings.Contains(joined, "field_42") {
		t.Errorf("fields requested: %s", joined)
	}
	if !strings.Contains(s.bodies[1], `"nextPageToken":"page2"`) {
		t.Errorf("second request did not carry the page token: %s", s.bodies[1])
	}
	if !strings.HasPrefix(s.auth, "Basic ") {
		t.Errorf("Authorization = %q", s.auth)
	}
}

func TestSearchPageAsksForOnePage(t *testing.T) {
	s := newServer(t)
	p := newProvider(s)
	first, err := p.SearchPage(context.Background(), "project = DEMO", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Tickets) != 1 || first.Tickets[0].Key != "DEMO-7" || first.Next != "page2" || first.Total != 412 {
		t.Fatalf("first page: %+v", first)
	}
	if !strings.Contains(s.bodies[0], `"maxResults":10`) || strings.Contains(s.bodies[0], "nextPageToken") {
		t.Errorf("first request: %s", s.bodies[0])
	}
	if len(s.requests) != 2 || !strings.HasSuffix(s.requests[1], "/search/approximate-count") {
		t.Fatalf("requests: %v", s.requests)
	}
	second, err := p.SearchPage(context.Background(), "project = DEMO", first.Next, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Tickets) != 1 || second.Tickets[0].Key != "DEMO-8" || second.Next != "" {
		t.Fatalf("second page: %+v", second)
	}
	if !strings.Contains(s.bodies[2], `"nextPageToken":"page2"`) {
		t.Errorf("second request: %s", s.bodies[2])
	}
}

func TestGetMapsFields(t *testing.T) {
	s := newServer(t)
	got, err := newProvider(s).Get(context.Background(), "DEMO-7")
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string]string{
		ticket.FieldKey:      "DEMO-7",
		ticket.FieldTitle:    "Export drops rows",
		ticket.FieldStatus:   "Open",
		ticket.FieldType:     "Bug",
		ticket.FieldAssignee: "Sam Example",
		ticket.FieldPriority: "High",
		ticket.FieldBody:     "Rows vanish.",
		"labels":             "export, csv",
		"category":           "Reports / Monthly",
		"regions":            "North, South",
		"count":              "3",
		"request":            "Report a fault",
	}
	for name, want := range checks {
		if got := got.Field(name).String(); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if want := s.URL + "/browse/DEMO-7"; got.URL != want {
		t.Errorf("URL = %q, want %q", got.URL, want)
	}
	if got.Created.IsZero() || got.Created.UTC().Hour() != 8 {
		t.Errorf("Created = %v", got.Created)
	}
	if len(got.Comments) != 1 || got.Comments[0].Author != "Alex Example" || got.Comments[0].Body != "**Seen it too.**" {
		t.Errorf("Comments = %+v", got.Comments)
	}
}

func TestErrorCarriesJiraMessage(t *testing.T) {
	s := newServer(t)
	_, err := newProvider(s).Get(context.Background(), "MISSING-1")
	if err == nil || !strings.Contains(err.Error(), "404") || !strings.Contains(err.Error(), "Issue does not exist") {
		t.Fatalf("err = %v", err)
	}
}

func TestWrites(t *testing.T) {
	s := newServer(t)
	p := newProvider(s)
	ctx := context.Background()

	trs, err := p.Transitions(ctx, "DEMO-7")
	if err != nil {
		t.Fatal(err)
	}
	if len(trs) != 2 || trs[0].To != "In progress" || len(trs[0].Fields) != 0 {
		t.Fatalf("transitions = %+v", trs)
	}
	if f := trs[1].Fields; len(f) != 1 || f[0].Name != "resolution" || !f[0].Required || f[0].Kind != ticket.KindSelect || len(f[0].Options) != 2 {
		t.Fatalf("fields of the second transition = %+v", f)
	}
	steps := []func() error{
		func() error { return p.Transition(ctx, "DEMO-7", trs[0], nil) },
		func() error {
			return p.Transition(ctx, "DEMO-7", trs[1], map[string]ticket.Value{"resolution": ticket.Text("duplicate")})
		},
		func() error { return p.Assign(ctx, "DEMO-7", "abc") },
		func() error { return p.Assign(ctx, "DEMO-7", "") },
		// The first field edit loads the edit screen; the rest reuse it.
		func() error { return p.UpdateField(ctx, "DEMO-7", "labels", ticket.List("a", "b")) },
		func() error { return p.UpdateField(ctx, "DEMO-7", ticket.FieldTitle, ticket.Text("New title")) },
		func() error { return p.UpdateField(ctx, "DEMO-7", ticket.FieldPriority, ticket.Text("low")) },
		func() error { return p.UpdateField(ctx, "DEMO-7", "regions", ticket.List("South", "North")) },
		func() error { return p.UpdateField(ctx, "DEMO-7", "regions", ticket.Text("North")) },
		func() error { return p.UpdateField(ctx, "DEMO-7", "count", ticket.Text("12.5")) },
		func() error { return p.UpdateField(ctx, "DEMO-7", "count", ticket.Text("")) },
		func() error { return p.UpdateField(ctx, "DEMO-7", "notes", ticket.Text("One\ntwo\n\nThree")) },
		func() error {
			return p.UpdateField(ctx, "DEMO-7", ticket.FieldAssignee, ticket.Value{Text: "Alex", ID: "u-2"})
		},
		// A field the edit screen does not describe is sent as it is.
		func() error { return p.UpdateField(ctx, "DEMO-7", "category", ticket.Text("Plain")) },
	}
	for i, step := range steps {
		if err := step(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}

	want := []struct{ request, body string }{
		{"GET /rest/api/3/issue/DEMO-7/transitions", ""},
		{"POST /rest/api/3/issue/DEMO-7/transitions", `{"transition":{"id":"11"}}`},
		{"POST /rest/api/3/issue/DEMO-7/transitions", `{"fields":{"resolution":{"id":"6"}},"transition":{"id":"21"}}`},
		{"PUT /rest/api/3/issue/DEMO-7/assignee", `{"accountId":"abc"}`},
		{"PUT /rest/api/3/issue/DEMO-7/assignee", `{"accountId":null}`},
		{"GET /rest/api/3/issue/DEMO-7/editmeta", ""},
		{"PUT /rest/api/3/issue/DEMO-7", `{"fields":{"labels":["a","b"]}}`},
		{"PUT /rest/api/3/issue/DEMO-7", `{"fields":{"summary":"New title"}}`},
		{"PUT /rest/api/3/issue/DEMO-7", `{"fields":{"priority":{"id":"3"}}}`},
		{"PUT /rest/api/3/issue/DEMO-7", `{"fields":{"field_43":[{"id":"202"},{"id":"201"}]}}`},
		{"PUT /rest/api/3/issue/DEMO-7", `{"fields":{"field_43":[{"id":"201"}]}}`},
		{"PUT /rest/api/3/issue/DEMO-7", `{"fields":{"field_44":12.5}}`},
		{"PUT /rest/api/3/issue/DEMO-7", `{"fields":{"field_44":null}}`},
		{"PUT /rest/api/3/issue/DEMO-7", `{"fields":{"field_45":{"content":[` +
			`{"content":[{"text":"One","type":"text"},{"type":"hardBreak"},{"text":"two","type":"text"}],"type":"paragraph"},` +
			`{"content":[{"text":"Three","type":"text"}],"type":"paragraph"}],"type":"doc","version":1}}}`},
		{"PUT /rest/api/3/issue/DEMO-7/assignee", `{"accountId":"u-2"}`},
		{"PUT /rest/api/3/issue/DEMO-7", `{"fields":{"field_42":"Plain"}}`},
	}
	if len(s.requests) != len(want) {
		t.Fatalf("requests = %v", s.requests)
	}
	for i, w := range want {
		if s.requests[i] != w.request || s.bodies[i] != w.body {
			t.Errorf("request %d = %s %s, want %s %s", i, s.requests[i], s.bodies[i], w.request, w.body)
		}
	}

	// A value that is not one of the options is refused before any request.
	sent := len(s.requests)
	err = p.UpdateField(ctx, "DEMO-7", ticket.FieldPriority, ticket.Text("Urgent"))
	if err == nil || !strings.Contains(err.Error(), "Priority") || len(s.requests) != sent {
		t.Fatalf("bad option: err=%v, requests made=%d", err, len(s.requests)-sent)
	}
	if err := p.UpdateField(ctx, "DEMO-7", "labels", ticket.List("two words")); err == nil {
		t.Fatal("a tag with a space in it was accepted")
	}
}

func TestGetDescribesEditableFields(t *testing.T) {
	s := newServer(t)
	got, err := newProvider(s).Get(context.Background(), "DEMO-7")
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{
		ticket.FieldTitle:    ticket.KindText,
		ticket.FieldAssignee: ticket.KindUser,
		ticket.FieldPriority: ticket.KindSelect,
		ticket.FieldType:     ticket.KindNone,
		"labels":             ticket.KindTags,
		"regions":            ticket.KindMulti,
		"count":              ticket.KindNumber,
		"notes":              ticket.KindRichText,
	}
	if len(got.Editable) != len(kinds) {
		t.Fatalf("%d editable fields, want %d: %+v", len(got.Editable), len(kinds), got.Editable)
	}
	for name, kind := range kinds {
		spec, ok := got.Spec(name)
		if !ok || spec.Kind != kind {
			t.Errorf("%s: kind %q (found %v), want %q", name, spec.Kind, ok, kind)
		}
	}
	if spec, _ := got.Spec("regions"); spec.Label != "Sales regions" || len(spec.Options) != 2 || spec.Options[1] != (ticket.Option{ID: "202", Name: "South"}) {
		t.Errorf("regions spec = %+v", spec)
	}
	if _, ok := got.Spec("category"); ok {
		t.Error("a field the edit screen lacks has a spec")
	}
}

func TestAccountUsersAndFieldLabels(t *testing.T) {
	s := newServer(t)
	p := newProvider(s)
	ctx := context.Background()
	me, err := p.Me(ctx)
	if err != nil || me != (ticket.User{ID: "me-1", Name: "Sam Example"}) {
		t.Fatalf("Me = %+v, %v", me, err)
	}
	users, err := p.Users(ctx, "DEMO-7", "al ex")
	if err != nil || len(users) != 1 || users[0].ID != "u-2" {
		t.Fatalf("Users = %+v, %v", users, err)
	}
	labels, err := p.FieldLabels(ctx)
	if err != nil || len(labels) != 2 || labels["regions"] != "Sales regions" || labels["labels"] != "Labels" {
		t.Fatalf("FieldLabels = %v, %v", labels, err)
	}
}

func TestNewReadsCredentialsFromEnvironment(t *testing.T) {
	cfg := provider.Config{Kind: Kind, BaseURL: "https://example.com", UserEnv: "TQ_TEST_USER", TokenEnv: "TQ_TEST_TOKEN"}
	t.Setenv("TQ_TEST_USER", "")
	t.Setenv("TQ_TEST_TOKEN", "")
	if _, err := New(cfg); err == nil || !strings.Contains(err.Error(), "TQ_TEST_USER") {
		t.Fatalf("err = %v, want it to name the unset variable", err)
	}
	t.Setenv("TQ_TEST_USER", "user@example.com")
	t.Setenv("TQ_TEST_TOKEN", "secret")
	if _, err := provider.New(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestRawSendsCredentialsOnlyToItsOwnSite(t *testing.T) {
	var got *http.Request
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		if r.URL.Path == "/missing" {
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, `{"errorMessages": ["No such file"]}`)
			return
		}
		io.WriteString(w, "file body")
	}))
	defer site.Close()
	c := NewClient(site.URL, "sam@example.com", "token")
	ctx := context.Background()

	for _, target := range []string{"/files/1", site.URL + "/files/1"} {
		resp, err := c.Raw(ctx, http.MethodGet, target, http.Header{"Accept": {"*/*"}}, nil)
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		user, _, ok := got.BasicAuth()
		if string(body) != "file body" || !ok || user != "sam@example.com" || got.Header.Get("Accept") != "*/*" {
			t.Errorf("%s: body=%q user=%q accept=%q", target, body, user, got.Header.Get("Accept"))
		}
	}

	got = nil
	if _, err := c.Raw(ctx, http.MethodGet, "https://elsewhere.example.com/files/1", nil, nil); err == nil || got != nil {
		t.Fatalf("a URL on another host: err=%v, request sent=%v", err, got != nil)
	}
	var e *Error
	if _, err := c.Raw(ctx, http.MethodGet, "/missing", nil, nil); !errors.As(err, &e) || e.Status != 404 || !strings.Contains(err.Error(), "No such file") {
		t.Fatalf("missing file: %v", err)
	}
}

func TestAddCommentPostsTheDocument(t *testing.T) {
	s := newServer(t)
	p := newProvider(s)
	var c provider.Commenter = p
	if c.CommentFormat() != provider.CommentJira {
		t.Fatalf("format = %q", c.CommentFormat())
	}
	doc := `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Done"}]}]}`
	if err := c.AddComment(context.Background(), "DEMO-7", "", doc); err != nil {
		t.Fatal(err)
	}
	if len(s.requests) != 1 || s.requests[0] != "POST /rest/api/3/issue/DEMO-7/comment" {
		t.Fatalf("requests = %v", s.requests)
	}
	if want := `{"body":` + doc + `}`; s.bodies[0] != want {
		t.Errorf("body = %s, want %s", s.bodies[0], want)
	}

	// Text that is not a document is refused before any request.
	if err := c.AddComment(context.Background(), "DEMO-7", "", "Done"); err == nil || len(s.requests) != 1 {
		t.Fatalf("a body that is not JSON: err=%v, requests=%v", err, s.requests)
	}
}

func TestAddCommentSendsTheKind(t *testing.T) {
	s := newServer(t)
	p := newProvider(s)
	if len(p.CommentKinds()) != 0 {
		t.Fatalf("kinds with none turned on: %v", p.CommentKinds())
	}
	if err := p.SetCommentKinds([]string{"internal", "public"}); err != nil {
		t.Fatal(err)
	}
	if got := p.CommentKinds(); len(got) != 2 || got[0].ID != provider.KindInternal || got[1].ID != provider.KindPublic {
		t.Fatalf("kinds = %v", got)
	}
	if err := p.SetCommentKinds([]string{"secret"}); err == nil {
		t.Fatal("an unknown kind was accepted")
	}

	doc := `{"type":"doc","version":1,"content":[]}`
	for kind, internal := range map[string]string{provider.KindInternal: "true", provider.KindPublic: "false"} {
		if err := p.AddComment(context.Background(), "DEMO-7", kind, doc); err != nil {
			t.Fatal(err)
		}
		want := `{"body":` + doc + `,"properties":[{"key":"sd.public.comment","value":{"internal":` + internal + `}}]}`
		if got := s.bodies[len(s.bodies)-1]; got != want {
			t.Errorf("%s: body = %s, want %s", kind, got, want)
		}
	}
	n := len(s.requests)
	if err := p.AddComment(context.Background(), "DEMO-7", "secret", doc); err == nil || len(s.requests) != n {
		t.Fatalf("an unknown kind: err=%v", err)
	}
}
