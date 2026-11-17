package jira

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lawsmd/tui-q/provider"
	"github.com/lawsmd/tui-q/ticket"
)

const issueJSON = `{
  "id": "10001",
  "key": "DEMO-7",
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
		case r.URL.Path == "/rest/api/3/issue/DEMO-7" && r.Method == http.MethodGet:
			io.WriteString(w, issueJSON)
		case r.URL.Path == "/rest/api/3/issue/DEMO-7/transitions" && r.Method == http.MethodGet:
			io.WriteString(w, `{"transitions": [{"id": "11", "name": "Start work", "to": {"name": "In progress"}}]}`)
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
	if len(trs) != 1 || trs[0].To != "In progress" {
		t.Fatalf("transitions = %+v", trs)
	}
	if err := p.Transition(ctx, "DEMO-7", trs[0], nil); err != nil {
		t.Fatal(err)
	}
	if err := p.Assign(ctx, "DEMO-7", "abc"); err != nil {
		t.Fatal(err)
	}
	if err := p.Assign(ctx, "DEMO-7", ""); err != nil {
		t.Fatal(err)
	}
	if err := p.UpdateField(ctx, "DEMO-7", "labels", ticket.List("a", "b")); err != nil {
		t.Fatal(err)
	}
	if err := p.UpdateField(ctx, "DEMO-7", ticket.FieldTitle, ticket.Text("New title")); err != nil {
		t.Fatal(err)
	}

	want := []struct{ request, body string }{
		{"GET /rest/api/3/issue/DEMO-7/transitions", ""},
		{"POST /rest/api/3/issue/DEMO-7/transitions", `{"transition":{"id":"11"}}`},
		{"PUT /rest/api/3/issue/DEMO-7/assignee", `{"accountId":"abc"}`},
		{"PUT /rest/api/3/issue/DEMO-7/assignee", `{"accountId":null}`},
		{"PUT /rest/api/3/issue/DEMO-7", `{"fields":{"labels":["a","b"]}}`},
		{"PUT /rest/api/3/issue/DEMO-7", `{"fields":{"summary":"New title"}}`},
	}
	if len(s.requests) != len(want) {
		t.Fatalf("requests = %v", s.requests)
	}
	for i, w := range want {
		if s.requests[i] != w.request || s.bodies[i] != w.body {
			t.Errorf("request %d = %s %s, want %s %s", i, s.requests[i], s.bodies[i], w.request, w.body)
		}
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
