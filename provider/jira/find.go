package jira

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"unicode"

	"github.com/lawsmd/tui-q/provider"
	"github.com/lawsmd/tui-q/ticket"
)

// parentField is the ID of the field that holds the parent of an issue.
const parentField = "parent"

// labelsField is the ID of the field that holds the labels of an issue.
const labelsField = "labels"

// Limits of Find. More issues are read than are returned, so that those of
// the issue's own project can be put first.
const (
	findLimit = 20
	findRead  = 50
)

// The provider searches the issues that can be a parent, and the labels.
var (
	_ provider.Finder    = (*Provider)(nil)
	_ provider.Suggester = (*Provider)(nil)
)

// issueRef reads a field whose value is an issue, as the parent is: the
// key of that issue, and its key and summary as the text to show.
func issueRef(v map[string]any) (ticket.Value, bool) {
	key, _ := v["key"].(string)
	fields, ok := v["fields"].(map[string]any)
	if key == "" || !ok {
		return ticket.Value{}, false
	}
	text := key
	if summary, _ := fields["summary"].(string); summary != "" {
		text += "  " + summary
	}
	return ticket.Value{Text: text, ID: key}, true
}

// hierarchyLevel reads the level of an issue type in Jira's hierarchy: -1
// for a subtask, 0 for a story, task or bug, 1 for an epic. A type that
// gives none counts as 0.
func hierarchyLevel(issueType json.RawMessage) int {
	var t struct {
		Level int `json:"hierarchyLevel"`
	}
	_ = json.Unmarshal(issueType, &t)
	return t.Level
}

// level is the hierarchy level of an issue, read from Jira the first time
// it is asked for an issue Get has not loaded.
func (p *Provider) level(ctx context.Context, id string) (int, error) {
	p.mu.Lock()
	level, ok := p.levels[id]
	p.mu.Unlock()
	if ok {
		return level, nil
	}
	var is issue
	path := "/rest/api/3/issue/" + url.PathEscape(id) + "?fields=issuetype"
	if err := p.client.Do(ctx, http.MethodGet, path, nil, &is); err != nil {
		return 0, err
	}
	level = hierarchyLevel(is.Fields["issuetype"])
	p.mu.Lock()
	p.levels[id] = level
	p.mu.Unlock()
	return level, nil
}

// projectOf is the project key of an issue key.
func projectOf(key string) string {
	if i := strings.LastIndexByte(key, '-'); i > 0 {
		return key[:i]
	}
	return ""
}

// searchWords splits what a user typed into the words of a text search,
// each matching any word it starts. Only letters and digits are kept, so
// nothing in the words has a meaning in JQL.
func searchWords(query string) []string {
	words := strings.FieldsFunc(query, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for i, w := range words {
		words[i] = w + "*"
	}
	return words
}

// Find searches the issues that can be the parent of an issue: those one
// level above it in Jira's hierarchy, which for a story, a task or a bug
// are the epics.
//
// Text that names an issue, by key, link or number, finds that issue, a
// bare number being taken to be in the project of the issue. Any other
// text finds the issues not yet done whose summaries have a word starting
// with each word typed, those of the issue's own project first. No text
// finds the issues of that project not yet done, the latest changed first.
func (p *Provider) Find(ctx context.Context, id, field, query string) ([]ticket.Ticket, error) {
	if p.fieldID(field) != parentField {
		return nil, nil
	}
	level, err := p.level(ctx, id)
	if err != nil {
		return nil, err
	}
	project := projectOf(id)
	above := fmt.Sprintf("hierarchyLevel = %d", level+1)
	const open = " AND statusCategory != Done ORDER BY updated DESC"

	if key, err := provider.KeyRef(query, project); err == nil {
		found, err := p.findIssues(ctx, above+` AND key = "`+key+`"`, 1)
		// Jira answers a key that names no issue with an error. The text
		// may be words that only look like a key, so the search goes on.
		var bad *Error
		if err != nil && !(errors.As(err, &bad) && bad.Status == http.StatusBadRequest) {
			return nil, err
		}
		if len(found) > 0 {
			return found, nil
		}
	}
	words := searchWords(query)
	if len(words) == 0 {
		return p.findIssues(ctx, above+` AND project = "`+project+`"`+open, findLimit)
	}
	found, err := p.findIssues(ctx, above+` AND summary ~ "`+strings.Join(words, " ")+`"`+open, findRead)
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(found, func(a, b ticket.Ticket) int {
		switch home, other := projectOf(a.Key) == project, projectOf(b.Key) == project; {
		case home && !other:
			return -1
		case other && !home:
			return 1
		}
		return 0
	})
	return found[:min(len(found), findLimit)], nil
}

// findIssues runs a JQL query for at most limit issues, with the fields
// Find returns.
func (p *Provider) findIssues(ctx context.Context, jql string, limit int) ([]ticket.Ticket, error) {
	body := map[string]any{
		"jql":        jql,
		"fields":     []string{"summary", "status", "issuetype"},
		"maxResults": limit,
	}
	var page struct {
		Issues []issue `json:"issues"`
	}
	if err := p.client.Do(ctx, http.MethodPost, "/rest/api/3/search/jql", body, &page); err != nil {
		return nil, err
	}
	out := make([]ticket.Ticket, len(page.Issues))
	for i, is := range page.Issues {
		out[i] = ticket.Ticket{
			ID:     is.Key,
			Key:    is.Key,
			Title:  decode(is.Fields["summary"]).String(),
			Status: decode(is.Fields["status"]).String(),
			Type:   decode(is.Fields["issuetype"]).String(),
			URL:    p.client.BaseURL() + "/browse/" + is.Key,
		}
	}
	return out, nil
}

// Suggest searches the labels of the site for the labels field, and the
// values in use for a custom field that holds labels. Jira has no such
// search for any other field, and none is found for one.
func (p *Provider) Suggest(ctx context.Context, _ string, field, query string) ([]string, error) {
	name := p.fieldID(field)
	switch number, custom := strings.CutPrefix(name, "customfield_"); {
	case custom:
		name = "cf[" + number + "]"
	case name != labelsField:
		return nil, nil
	}
	var resp struct {
		Results []struct {
			Value string `json:"value"`
		} `json:"results"`
	}
	path := "/rest/api/3/jql/autocompletedata/suggestions?fieldName=" + url.QueryEscape(name) +
		"&fieldValue=" + url.QueryEscape(query)
	if err := p.client.Do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(resp.Results))
	for _, r := range resp.Results {
		if r.Value != "" {
			out = append(out, r.Value)
		}
	}
	return out, nil
}
