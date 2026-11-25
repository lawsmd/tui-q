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

// found is an issue as a search for parents returns it.
func found(key, summary string) string {
	return `{"key": "` + key + `", "fields": {"summary": "` + summary + `", "status": {"name": "To Do"}, "issuetype": {"name": "Epic"}}}`
}

// parentSite answers the requests made for the parent and the labels of
// an issue, and records the queries, the suggestions asked for and the
// writes it was sent.
type parentSite struct {
	*httptest.Server
	mu      sync.Mutex
	jql     []string
	asked   []string
	written []string
}

func newParentSite(t *testing.T) *parentSite {
	t.Helper()
	s := &parentSite{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		defer s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/rest/api/3/issue/DEMO-7" && r.Method == http.MethodGet:
			io.WriteString(w, `{"key": "DEMO-7",
				"editmeta": {"fields": {
					"parent": {"required": false, "name": "Parent", "schema": {"type": "issuelink", "system": "parent"}},
					"field_50": {"required": false, "name": "Linked", "schema": {"type": "issuelink", "custom": "x:link"}}}},
				"fields": {
					"summary": "Export drops rows",
					"issuetype": {"name": "Task", "hierarchyLevel": 0},
					"parent": {"id": "10003", "key": "DEMO-3", "fields": {"summary": "Exports", "status": {"name": "To Do"}}}}}`)
		case r.URL.Path == "/rest/api/3/issue/DEMO-7" && r.Method == http.MethodPut:
			s.written = append(s.written, string(body))
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/rest/api/3/issue/DEMO-8":
			io.WriteString(w, `{"key": "DEMO-8", "fields": {"issuetype": {"name": "Sub-task", "hierarchyLevel": -1}}}`)
		case r.URL.Path == "/rest/api/3/search/jql":
			var req struct {
				JQL string `json:"jql"`
			}
			_ = json.Unmarshal(body, &req)
			s.jql = append(s.jql, req.JQL)
			switch {
			case strings.Contains(req.JQL, `key = "DEMO-3"`):
				io.WriteString(w, `{"issues": [`+found("DEMO-3", "Exports")+`]}`)
			case strings.Contains(req.JQL, "key = "):
				w.WriteHeader(http.StatusBadRequest)
				io.WriteString(w, `{"errorMessages": ["An issue with that key does not exist for field 'key'."]}`)
			case strings.Contains(req.JQL, "summary ~"):
				io.WriteString(w, `{"issues": [`+strings.Join([]string{
					found("OTHER-1", "Billing exports"), found("DEMO-4", "Billing"),
					found("OTHER-2", "Billing imports"), found("DEMO-5", "Billing reports"),
				}, ",")+`]}`)
			default:
				io.WriteString(w, `{"issues": [`+found("DEMO-4", "Billing")+`]}`)
			}
		case r.URL.Path == "/rest/api/3/jql/autocompletedata/suggestions":
			s.asked = append(s.asked, r.URL.Query().Get("fieldName")+" "+r.URL.Query().Get("fieldValue"))
			io.WriteString(w, `{"results": [{"value": "export", "displayName": "<b>ex</b>port"}, {"value": "extra", "displayName": "<b>ex</b>tra"}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, `{"errorMessages": ["no such page"]}`)
		}
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *parentSite) provider() *Provider {
	fields := map[string]string{"parent": "parent", "labels": "labels", "areas": "customfield_77", "linked": "field_50"}
	return NewProvider(NewClient(s.URL, "user@example.com", "secret"), fields)
}

func keysOf(tickets []ticket.Ticket) string {
	keys := make([]string, len(tickets))
	for i, t := range tickets {
		keys[i] = t.Key
	}
	return strings.Join(keys, ",")
}

func TestParentIsReadAndWrittenAsATicket(t *testing.T) {
	s := newParentSite(t)
	p := s.provider()
	ctx := context.Background()
	got, err := p.Get(ctx, "DEMO-7")
	if err != nil {
		t.Fatal(err)
	}
	if v := got.Field("parent"); v.Text != "DEMO-3  Exports" || v.ID != "DEMO-3" {
		t.Errorf("parent = %+v", v)
	}
	if spec, ok := got.Spec("parent"); !ok || spec.Kind != ticket.KindTicket {
		t.Errorf("parent spec = %+v (found %v)", spec, ok)
	}
	// Any other field that links to an issue cannot be set from here.
	if spec, _ := got.Spec("linked"); spec.Kind != ticket.KindNone {
		t.Errorf("a link that is no parent has kind %q", spec.Kind)
	}

	for _, v := range []ticket.Value{
		{Text: "DEMO-4", ID: "DEMO-4"},
		ticket.Text("DEMO-5  Billing reports"),
		{},
	} {
		if err := p.UpdateField(ctx, "DEMO-7", "parent", v); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{
		`{"fields":{"parent":{"key":"DEMO-4"}}}`,
		`{"fields":{"parent":{"key":"DEMO-5"}}}`,
		`{"fields":{"parent":null}}`,
	}
	if strings.Join(s.written, "\n") != strings.Join(want, "\n") {
		t.Errorf("wrote %v, want %v", s.written, want)
	}
}

func TestFindSearchesTheLevelAbove(t *testing.T) {
	s := newParentSite(t)
	p := s.provider()
	ctx := context.Background()
	if _, err := p.Get(ctx, "DEMO-7"); err != nil {
		t.Fatal(err)
	}

	steps := []struct {
		id, query, keys string
		jql             []string
	}{
		// Nothing typed: the open issues of the project, one level up.
		{"DEMO-7", "", "DEMO-4", []string{
			`hierarchyLevel = 1 AND project = "DEMO" AND statusCategory != Done ORDER BY updated DESC`}},
		// A key, a link and a bare number each name one issue.
		{"DEMO-7", "demo-3", "DEMO-3", []string{`hierarchyLevel = 1 AND key = "DEMO-3"`}},
		{"DEMO-7", "https://example.com/browse/DEMO-3", "DEMO-3", []string{`hierarchyLevel = 1 AND key = "DEMO-3"`}},
		{"DEMO-7", "3", "DEMO-3", []string{`hierarchyLevel = 1 AND key = "DEMO-3"`}},
		// Words find summaries, the issue's own project first. Nothing
		// typed can break out of the quotes.
		{"DEMO-7", `bill" OR x-ray`, "DEMO-4,DEMO-5,OTHER-1,OTHER-2", []string{
			`hierarchyLevel = 1 AND summary ~ "bill* OR* x* ray*" AND statusCategory != Done ORDER BY updated DESC`}},
		// A key that names no issue is searched for as words.
		{"DEMO-7", "demo-99", "DEMO-4,DEMO-5,OTHER-1,OTHER-2", []string{
			`hierarchyLevel = 1 AND key = "DEMO-99"`,
			`hierarchyLevel = 1 AND summary ~ "demo* 99*" AND statusCategory != Done ORDER BY updated DESC`}},
		// An issue that was never loaded has its level read first.
		{"DEMO-8", "", "DEMO-4", []string{
			`hierarchyLevel = 0 AND project = "DEMO" AND statusCategory != Done ORDER BY updated DESC`}},
	}
	for _, step := range steps {
		s.jql = nil
		got, err := p.Find(ctx, step.id, "parent", step.query)
		if err != nil {
			t.Fatalf("%q: %v", step.query, err)
		}
		if keysOf(got) != step.keys {
			t.Errorf("%q found %s, want %s", step.query, keysOf(got), step.keys)
		}
		if strings.Join(s.jql, "\n") != strings.Join(step.jql, "\n") {
			t.Errorf("%q asked\n%s\nwant\n%s", step.query, strings.Join(s.jql, "\n"), strings.Join(step.jql, "\n"))
		}
	}
	if got, _ := p.Find(ctx, "DEMO-7", "parent", "3"); len(got) != 1 || got[0].Title != "Exports" || got[0].Status != "To Do" || got[0].Type != "Epic" {
		t.Errorf("found %+v", got)
	}
	// No other field is searched.
	s.jql = nil
	if got, err := p.Find(ctx, "DEMO-7", "labels", "x"); err != nil || len(got) != 0 || len(s.jql) != 0 {
		t.Errorf("labels: found %v, err %v, asked %v", got, err, s.jql)
	}
}

func TestSuggestSearchesLabels(t *testing.T) {
	s := newParentSite(t)
	p := s.provider()
	ctx := context.Background()
	got, err := p.Suggest(ctx, "DEMO-7", "labels", "ex")
	if err != nil || strings.Join(got, ",") != "export,extra" {
		t.Fatalf("labels: %v, %v", got, err)
	}
	if _, err := p.Suggest(ctx, "DEMO-7", "areas", "no rth"); err != nil {
		t.Fatal(err)
	}
	if got, err := p.Suggest(ctx, "DEMO-7", "parent", "ex"); err != nil || len(got) != 0 {
		t.Errorf("parent: %v, %v", got, err)
	}
	if want := "labels ex\ncf[77] no rth"; strings.Join(s.asked, "\n") != want {
		t.Errorf("asked %q, want %q", s.asked, want)
	}
}
