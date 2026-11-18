package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lawsmd/tui-q/provider"
	"github.com/lawsmd/tui-q/ticket"
)

// Kind is the profile value that selects this provider.
const Kind = "jira"

// Search limits. A queue holding more than maxResults tickets is cut off
// there; a narrower query is the fix.
const (
	pageSize   = 100
	maxResults = 500
)

func init() {
	provider.Register(Kind, func(cfg provider.Config) (provider.Provider, error) {
		return New(cfg)
	})
}

// Fields Jira returns for every ticket, whatever the profile asks for.
var baseFields = []string{"summary", "status", "issuetype", "assignee", "priority", "created", "updated"}

// Provider talks to one Jira Cloud site.
type Provider struct {
	client *Client
	// fields maps a profile field name to a Jira field ID.
	fields map[string]string

	// meta remembers, per issue and Jira field ID, what the edit and
	// transition screens said about each field.
	// edit marks the issues whose edit screen has been read.
	mu   sync.Mutex
	meta map[string]map[string]ticket.FieldSpec
	edit map[string]bool
}

// New builds a Provider from a profile's [provider] section. The
// credentials are read from the environment variables the section names.
func New(cfg provider.Config) (*Provider, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("jira: [provider] base_url is required")
	}
	if cfg.UserEnv == "" || cfg.TokenEnv == "" {
		return nil, fmt.Errorf("jira: [provider] user_env and token_env are required")
	}
	user, token := os.Getenv(cfg.UserEnv), os.Getenv(cfg.TokenEnv)
	if user == "" {
		return nil, fmt.Errorf("jira: environment variable %s is not set", cfg.UserEnv)
	}
	if token == "" {
		return nil, fmt.Errorf("jira: environment variable %s is not set", cfg.TokenEnv)
	}
	return NewProvider(NewClient(cfg.BaseURL, user, token), cfg.Fields), nil
}

// NewProvider builds a Provider around an existing Client.
func NewProvider(client *Client, fields map[string]string) *Provider {
	return &Provider{client: client, fields: fields, meta: map[string]map[string]ticket.FieldSpec{}, edit: map[string]bool{}}
}

// Client returns the HTTP client, for instance code that needs endpoints
// beyond the Provider interface.
func (p *Provider) Client() *Client { return p.client }

// fieldIDs is every field to request: the base set, the profile's own, and
// any extras.
func (p *Provider) fieldIDs(extra ...string) []string {
	ids := append([]string(nil), baseFields...)
	custom := make([]string, 0, len(p.fields))
	for _, id := range p.fields {
		custom = append(custom, id)
	}
	sort.Strings(custom)
	return append(append(ids, custom...), extra...)
}

type issue struct {
	ID     string                     `json:"id"`
	Key    string                     `json:"key"`
	Fields map[string]json.RawMessage `json:"fields"`
	// EditMeta is present when the request asks for it.
	EditMeta struct {
		Fields map[string]rawMeta `json:"fields"`
	} `json:"editmeta"`
}

// Search runs a JQL query and follows the page tokens.
func (p *Provider) Search(ctx context.Context, query string) ([]ticket.Ticket, error) {
	var out []ticket.Ticket
	token := ""
	for {
		body := map[string]any{
			"jql":        query,
			"fields":     p.fieldIDs(),
			"maxResults": pageSize,
		}
		if token != "" {
			body["nextPageToken"] = token
		}
		var page struct {
			Issues        []issue `json:"issues"`
			NextPageToken string  `json:"nextPageToken"`
		}
		if err := p.client.Do(ctx, http.MethodPost, "/rest/api/3/search/jql", body, &page); err != nil {
			return nil, err
		}
		for _, is := range page.Issues {
			out = append(out, p.toTicket(is))
		}
		token = page.NextPageToken
		if token == "" || len(page.Issues) == 0 || len(out) >= maxResults {
			return out, nil
		}
	}
}

// Get loads one issue with its description, its comments, and the fields
// of its edit screen.
func (p *Provider) Get(ctx context.Context, id string) (ticket.Ticket, error) {
	path := "/rest/api/3/issue/" + url.PathEscape(id) +
		"?expand=editmeta&fields=" + url.QueryEscape(strings.Join(p.fieldIDs("description", "comment"), ","))
	var is issue
	if err := p.client.Do(ctx, http.MethodGet, path, nil, &is); err != nil {
		return ticket.Ticket{}, err
	}
	t := p.toTicket(is)
	t.Editable = p.specs(t.ID, is.EditMeta.Fields, true)
	return t, nil
}

// Transitions lists the workflow transitions open to an issue, each with
// the fields of its screen.
func (p *Provider) Transitions(ctx context.Context, id string) ([]ticket.Transition, error) {
	var resp struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			To   struct {
				Name string `json:"name"`
			} `json:"to"`
			Fields map[string]rawMeta `json:"fields"`
		} `json:"transitions"`
	}
	path := "/rest/api/3/issue/" + url.PathEscape(id) + "/transitions?expand=transitions.fields"
	if err := p.client.Do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	out := make([]ticket.Transition, len(resp.Transitions))
	for i, tr := range resp.Transitions {
		out[i] = ticket.Transition{ID: tr.ID, Name: tr.Name, To: tr.To.Name}
		if len(tr.Fields) > 0 {
			out[i].Fields = p.specs(id, tr.Fields, false)
		}
	}
	return out, nil
}

// Transition moves an issue through one transition.
func (p *Provider) Transition(ctx context.Context, id string, tr ticket.Transition, fields map[string]ticket.Value) error {
	body := map[string]any{"transition": map[string]string{"id": tr.ID}}
	if len(fields) > 0 {
		encoded := map[string]any{}
		for name, v := range fields {
			fid, value, err := p.encodeField(ctx, id, name, v)
			if err != nil {
				return err
			}
			encoded[fid] = value
		}
		body["fields"] = encoded
	}
	path := "/rest/api/3/issue/" + url.PathEscape(id) + "/transitions"
	return p.client.Do(ctx, http.MethodPost, path, body, nil)
}

// Assign sets the assignee by account ID. An empty ID clears it.
func (p *Provider) Assign(ctx context.Context, id, userID string) error {
	var account any
	if userID != "" {
		account = userID
	}
	path := "/rest/api/3/issue/" + url.PathEscape(id) + "/assignee"
	return p.client.Do(ctx, http.MethodPut, path, map[string]any{"accountId": account}, nil)
}

// UpdateField writes one field, shaped the way the issue's edit screen
// describes it.
func (p *Provider) UpdateField(ctx context.Context, id, field string, v ticket.Value) error {
	if field == ticket.FieldAssignee {
		return p.Assign(ctx, id, v.ID)
	}
	fid, value, err := p.encodeField(ctx, id, field, v)
	if err != nil {
		return err
	}
	body := map[string]any{"fields": map[string]any{fid: value}}
	return p.client.Do(ctx, http.MethodPut, "/rest/api/3/issue/"+url.PathEscape(id), body, nil)
}

// fieldID maps a profile field name to the Jira field ID.
func (p *Provider) fieldID(name string) string {
	switch name {
	case ticket.FieldTitle:
		return "summary"
	case ticket.FieldBody:
		return "description"
	case ticket.FieldType:
		return "issuetype"
	}
	if id, ok := p.fields[name]; ok {
		return id
	}
	return name
}

func encode(v ticket.Value) any {
	if v.Items != nil {
		return v.Items
	}
	return v.Text
}

// toTicket maps one issue into the neutral model.
func (p *Provider) toTicket(is issue) ticket.Ticket {
	f := is.Fields
	t := ticket.Ticket{
		ID:       is.Key,
		Key:      is.Key,
		Title:    decode(f["summary"]).String(),
		Status:   decode(f["status"]).String(),
		Type:     decode(f["issuetype"]).String(),
		Assignee: decode(f["assignee"]).String(),
		Priority: decode(f["priority"]).String(),
		URL:      p.client.BaseURL() + "/browse/" + is.Key,
		Created:  parseTime(f["created"]),
		Updated:  parseTime(f["updated"]),
		Body:     adfToMarkdown(f["description"]),
		Fields:   map[string]ticket.Value{},
	}
	for name, id := range p.fields {
		if raw, ok := f[id]; ok {
			t.Fields[name] = decode(raw)
		}
	}
	if raw, ok := f["comment"]; ok {
		var block struct {
			Comments []struct {
				Author struct {
					DisplayName string `json:"displayName"`
				} `json:"author"`
				Created json.RawMessage `json:"created"`
				Body    json.RawMessage `json:"body"`
			} `json:"comments"`
		}
		if json.Unmarshal(raw, &block) == nil {
			for _, c := range block.Comments {
				t.Comments = append(t.Comments, ticket.Comment{
					Author:  c.Author.DisplayName,
					Created: parseTime(c.Created),
					Body:    adfToMarkdown(c.Body),
				})
			}
		}
	}
	return t
}

// decode turns any Jira field value into a Value. Jira fields come as
// strings, numbers, objects naming themselves in one of a few keys, rich
// text documents, or arrays of any of those.
func decode(raw json.RawMessage) ticket.Value {
	if len(raw) == 0 {
		return ticket.Value{}
	}
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return ticket.Value{}
	}
	switch v := v.(type) {
	case []any:
		items := make([]string, 0, len(v))
		for _, item := range v {
			if s := scalar(item); s != "" {
				items = append(items, s)
			}
		}
		return ticket.Value{Items: items}
	case map[string]any:
		if v["type"] == "doc" {
			return ticket.Text(adfToMarkdown(raw))
		}
	}
	return ticket.Text(scalar(v))
}

// nameKeys are the keys an object-valued field uses for its readable name,
// most specific first.
var nameKeys = []string{"displayName", "value", "name", "key"}

func scalar(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	case map[string]any:
		for _, key := range nameKeys {
			if s, ok := v[key].(string); ok && s != "" {
				// A cascading option carries its child inside it.
				if child, ok := v["child"].(map[string]any); ok {
					if cs := scalar(child); cs != "" {
						return s + " / " + cs
					}
				}
				return s
			}
		}
	}
	return ""
}

// Jira writes timestamps with a numeric zone and no colon in it.
const timeLayout = "2006-01-02T15:04:05.000-0700"

func parseTime(raw json.RawMessage) time.Time {
	var s string
	if json.Unmarshal(raw, &s) != nil || s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(timeLayout, s); err == nil {
		return t
	}
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
