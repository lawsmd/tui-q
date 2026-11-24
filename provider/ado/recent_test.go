package ado

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// up writes one update of a history: who made it, when, and the fields it
// changed as "ref": {oldValue, newValue} pairs.
func up(by, at string, fields ...string) string {
	all := append([]string{`"System.ChangedDate": {"newValue": "` + at + `"}`}, fields...)
	return `{"revisedBy": {"uniqueName": "` + by + `"}, "revisedDate": "9999-01-01T00:00:00Z",
	  "fields": {` + strings.Join(all, ",") + `}}`
}

func person(name string) string {
	if name == "" {
		return "null"
	}
	return `{"displayName": "` + name + `", "uniqueName": "` + strings.ToLower(name) + `@example.com"}`
}

func TestLastTouch(t *testing.T) {
	since := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		updates []string
		want    string
	}{
		{"state by me", []string{
			up("sam@example.com", "2026-03-02T10:00:00Z", `"System.State": {"oldValue": "New", "newValue": "Active"}`),
			up("alex@example.com", "2026-03-03T10:00:00Z", `"System.State": {"oldValue": "Active", "newValue": "Done"}`),
		}, "2026-03-02 New → Active"},
		{"assigned to me by someone else", []string{
			up("alex@example.com", "2026-03-04T10:00:00Z", `"System.AssignedTo": {"oldValue": `+person("Alex")+`, "newValue": `+person("Sam")+`}`),
		}, "2026-03-04 Assignee Alex → me"},
		{"taken from me", []string{
			up("alex@example.com", "2026-03-04T10:00:00Z", `"System.AssignedTo": {"oldValue": `+person("Sam")+`}`),
		}, "2026-03-04 Assignee me → –"},
		{"tags by me, latest wins", []string{
			up("sam@example.com", "2026-03-02T10:00:00Z", `"System.State": {"oldValue": "New", "newValue": "Active"}`),
			up("sam@example.com", "2026-03-05T10:00:00Z", `"System.Tags": {"oldValue": "a; b", "newValue": "b; c"}`),
		}, "2026-03-05 +c −a"},
		{"title by me does not count", []string{
			up("sam@example.com", "2026-03-05T10:00:00Z", `"System.Title": {"oldValue": "x", "newValue": "y"}`),
		}, ""},
		{"too old", []string{
			up("sam@example.com", "2026-02-20T10:00:00Z", `"System.State": {"oldValue": "New", "newValue": "Active"}`),
		}, ""},
	}
	for _, c := range cases {
		var ups []update
		if err := json.Unmarshal([]byte("["+strings.Join(c.updates, ",")+"]"), &ups); err != nil {
			t.Fatal(err)
		}
		got := ""
		if tc, ok := lastTouch(ups, "SAM@example.com", since); ok {
			got = tc.At.UTC().Format("2006-01-02") + " " + tc.What
		}
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// historyServer answers the WIQL query, the history of each work item and
// the batch read, and hands any other request to fakeServer.
type historyServer struct {
	*fakeServer
	histories map[string]string
	wiql      string
	// reads counts the reads of each history.
	reads map[string]int
}

func (h *historyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost && r.URL.Path == "/DEMO/_apis/wit/wiql" {
		var body struct{ Query string }
		data, _ := io.ReadAll(r.Body)
		json.Unmarshal(data, &body)
		h.wiql = body.Query
		r.Body = io.NopCloser(strings.NewReader(string(data)))
	}
	id, ok := strings.CutPrefix(r.URL.Path, "/DEMO/_apis/wit/workItems/")
	if id, ok2 := strings.CutSuffix(id, "/updates"); ok && ok2 {
		h.reads[id]++
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"value": [`+h.histories[id]+`]}`)
		return
	}
	h.fakeServer.ServeHTTP(w, r)
}

func TestRecent(t *testing.T) {
	f := &fakeServer{ids: []int{3, 2, 1}, items: map[int]string{
		1: item(1, "One", ""), 2: item(2, "Two", ""), 3: item(3, "Three", ""),
	}}
	now := time.Now().UTC()
	at := func(ago time.Duration) string { return now.Add(-ago).Format(time.RFC3339) }
	h := &historyServer{fakeServer: f, reads: map[string]int{}, histories: map[string]string{
		"1": up("sam@example.com", at(time.Hour), `"System.State": {"oldValue": "New", "newValue": "Active"}`),
		"2": up("alex@example.com", at(time.Minute), `"System.Title": {"oldValue": "x", "newValue": "Two"}`),
		"3": up("sam@example.com", at(2*time.Hour), `"System.Tags": {"newValue": "urgent"}`),
	}}
	p := newTestWith(t, h)
	got, err := p.Recent(context.Background(), now.AddDate(0, 0, -30))
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, tc := range got {
		keys = append(keys, tc.Ticket.Key+" "+tc.What)
	}
	if strings.Join(keys, "; ") != "3 +urgent; 1 New → Active" {
		t.Errorf("got %q", keys)
	}
	// A second look reads no history again while the work items have the
	// same revisions.
	if _, err := p.Recent(context.Background(), now.AddDate(0, 0, -30)); err != nil {
		t.Fatal(err)
	}
	if h.reads["1"] != 1 || h.reads["2"] != 1 || h.reads["3"] != 1 {
		t.Errorf("history reads %v", h.reads)
	}
	f.items[1] = item(1, "One", `, "System.Rev": 2`)
	if _, err := p.Recent(context.Background(), now.AddDate(0, 0, -30)); err != nil {
		t.Fatal(err)
	}
	if h.reads["1"] != 2 || h.reads["3"] != 1 {
		t.Errorf("after a change, history reads %v", h.reads)
	}
	for _, s := range []string{"EVER [System.ChangedBy] = @me", "EVER [System.AssignedTo] = @me", "[System.ChangedDate] >= '"} {
		if !strings.Contains(h.wiql, s) {
			t.Errorf("query lacks %q: %s", s, h.wiql)
		}
	}
}
