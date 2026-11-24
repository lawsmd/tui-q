package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
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
	// project is the project a bare issue number is looked up in.
	project string
	// kinds are the kinds of comment the profile turned on.
	kinds []provider.CommentKind

	// meta remembers, per issue and Jira field ID, what the edit and
	// transition screens said about each field.
	// edit marks the issues whose edit screen has been read.
	// media remembers, per attachment ID, the ID the media store knows the
	// file by, and an empty one for an attachment that has none.
	mu    sync.Mutex
	meta  map[string]map[string]ticket.FieldSpec
	edit  map[string]bool
	media map[string]string
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
	p := NewProvider(NewClient(cfg.BaseURL, user, token), cfg.Fields)
	p.project = cfg.DefaultProject
	if err := p.SetCommentKinds(cfg.CommentKinds); err != nil {
		return nil, err
	}
	return p, nil
}

// Resolve reads an issue key from a number, a key or a link to an issue.
// A bare number is taken to be in the profile's default project.
func (p *Provider) Resolve(input string) (string, error) {
	return provider.KeyRef(input, p.project)
}

// NewProvider builds a Provider around an existing Client.
func NewProvider(client *Client, fields map[string]string) *Provider {
	return &Provider{client: client, fields: fields, meta: map[string]map[string]ticket.FieldSpec{}, edit: map[string]bool{}, media: map[string]string{}}
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
			out = append(out, p.toTicket(is, nil))
		}
		token = page.NextPageToken
		if token == "" || len(page.Issues) == 0 || len(out) >= maxResults {
			return out, nil
		}
	}
}

// SearchPage runs a JQL query for one page. The cursor is Jira's page
// token. The total is Jira's approximate count, and -1 when Jira does not
// give one.
func (p *Provider) SearchPage(ctx context.Context, query, cursor string, limit int) (provider.Page, error) {
	body := map[string]any{
		"jql":        query,
		"fields":     p.fieldIDs(),
		"maxResults": max(limit, 1),
	}
	if cursor != "" {
		body["nextPageToken"] = cursor
	}
	// The count is a request of its own, made while the page is read.
	var count struct {
		Count *int `json:"count"`
	}
	var countErr error
	counted := make(chan struct{})
	go func() {
		defer close(counted)
		countErr = p.client.Do(ctx, http.MethodPost, "/rest/api/3/search/approximate-count", map[string]any{"jql": query}, &count)
	}()

	var resp struct {
		Issues        []issue `json:"issues"`
		NextPageToken string  `json:"nextPageToken"`
	}
	err := p.client.Do(ctx, http.MethodPost, "/rest/api/3/search/jql", body, &resp)
	<-counted
	if err != nil {
		return provider.Page{}, err
	}
	page := provider.Page{Next: resp.NextPageToken, Total: -1}
	for _, is := range resp.Issues {
		page.Tickets = append(page.Tickets, p.toTicket(is, nil))
	}
	if countErr == nil && count.Count != nil {
		page.Total = *count.Count
	}
	return page, nil
}

// Get loads one issue with its description, its comments, its attachments,
// its links to other issues, and the fields of its edit screen.
func (p *Provider) Get(ctx context.Context, id string) (ticket.Ticket, error) {
	path := "/rest/api/3/issue/" + url.PathEscape(id) +
		"?expand=editmeta&fields=" + url.QueryEscape(strings.Join(p.fieldIDs("description", "comment", "attachment", "issuelinks"), ","))
	var is issue
	if err := p.client.Do(ctx, http.MethodGet, path, nil, &is); err != nil {
		return ticket.Ticket{}, err
	}
	t := p.toTicket(is, p.mediaIDs(ctx, is))
	t.Editable = p.specs(t.ID, is.EditMeta.Fields, true)
	return t, nil
}

// attachments reads the list of files on an issue.
func attachments(raw json.RawMessage) []ticket.Attachment {
	var files []struct {
		ID       string `json:"id"`
		Filename string `json:"filename"`
		MimeType string `json:"mimeType"`
		Size     int64  `json:"size"`
	}
	if json.Unmarshal(raw, &files) != nil {
		return nil
	}
	out := make([]ticket.Attachment, len(files))
	for i, a := range files {
		out[i] = ticket.Attachment{ID: a.ID, Name: a.Filename, MIME: a.MimeType, Size: a.Size}
	}
	return out
}

// links reads the links of an issue to other issues. Jira names each kind
// of link from both ends, and gives the issue at the other end as the
// outward or the inward one: the name for that end is the one that reads
// from this issue. base is the address of the site.
func links(raw json.RawMessage, base string) []ticket.Link {
	type other struct {
		Key    string `json:"key"`
		Fields struct {
			Summary string `json:"summary"`
			Status  struct {
				Name string `json:"name"`
			} `json:"status"`
			Type struct {
				Name string `json:"name"`
			} `json:"issuetype"`
		} `json:"fields"`
	}
	var entries []struct {
		Type struct {
			Inward  string `json:"inward"`
			Outward string `json:"outward"`
		} `json:"type"`
		Inward  *other `json:"inwardIssue"`
		Outward *other `json:"outwardIssue"`
	}
	if json.Unmarshal(raw, &entries) != nil {
		return nil
	}
	var out []ticket.Link
	for _, e := range entries {
		is, relation := e.Outward, e.Type.Outward
		if is == nil {
			is, relation = e.Inward, e.Type.Inward
		}
		if is == nil || is.Key == "" {
			continue
		}
		out = append(out, ticket.Link{
			Relation: relation,
			ID:       is.Key,
			Key:      is.Key,
			Title:    is.Fields.Summary,
			Status:   is.Fields.Status.Name,
			Type:     is.Fields.Type.Name,
			URL:      base + "/browse/" + is.Key,
		})
	}
	return out
}

// mediaID finds the ID of the media store in the address Jira sends a
// reader of an attachment on to.
var mediaID = regexp.MustCompile(`/file/([0-9a-fA-F]{8}(?:-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12})/`)

// mediaReaders is how many attachments are asked about at once.
const mediaReaders = 4

// mediaIDs gives the attachments of an issue by the ID the media store
// knows each by, which is how the description and the comments name the
// files they show. Jira's list of attachments leaves that ID out. It is
// read from the address Jira answers a request for the file with, one
// request for each attachment and none for the file itself, the first time
// the attachment is seen. An issue whose text shows no file costs nothing.
// An attachment Jira does not answer for is left out, and is still matched
// by its name where the text gives one.
func (p *Provider) mediaIDs(ctx context.Context, is issue) map[string]ticket.Attachment {
	atts := attachments(is.Fields["attachment"])
	shown := bytes.Contains(is.Fields["description"], []byte(`"media`)) || bytes.Contains(is.Fields["comment"], []byte(`"media`))
	if len(atts) == 0 || !shown {
		return nil
	}
	var wg sync.WaitGroup
	slots := make(chan struct{}, mediaReaders)
	for _, a := range atts {
		p.mu.Lock()
		_, known := p.media[a.ID]
		p.mu.Unlock()
		if known {
			continue
		}
		wg.Add(1)
		slots <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			to, err := p.client.Location(ctx, "/rest/api/3/attachment/content/"+url.PathEscape(a.ID))
			if err != nil {
				return
			}
			id := ""
			if u, err := url.Parse(to); err == nil {
				if m := mediaID.FindStringSubmatch(u.Path); m != nil {
					id = strings.ToLower(m[1])
				}
			}
			p.mu.Lock()
			p.media[a.ID] = id
			p.mu.Unlock()
		}()
	}
	wg.Wait()
	out := map[string]ticket.Attachment{}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, a := range atts {
		if id := p.media[a.ID]; id != "" {
			out[id] = a
		}
	}
	return out
}

// Transitions lists the workflow transitions open to an issue, each with
// the fields of its screen.
func (p *Provider) Transitions(ctx context.Context, id string) ([]ticket.Transition, error) {
	list, err := p.readTransitions(ctx, id)
	if err != nil {
		return nil, err
	}
	return p.toTransitions(id, list), nil
}

// transitionList is Jira's answer to a request for an issue's transitions.
type transitionList struct {
	Transitions []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		To   struct {
			Name string `json:"name"`
		} `json:"to"`
		Fields map[string]rawMeta `json:"fields"`
	} `json:"transitions"`
}

func (p *Provider) readTransitions(ctx context.Context, id string) (transitionList, error) {
	var list transitionList
	path := "/rest/api/3/issue/" + url.PathEscape(id) + "/transitions?expand=transitions.fields"
	err := p.client.Do(ctx, http.MethodGet, path, nil, &list)
	return list, err
}

// toTransitions maps Jira's transitions into the neutral model, and
// remembers what each screen said about its fields for the issue.
func (p *Provider) toTransitions(issueID string, list transitionList) []ticket.Transition {
	out := make([]ticket.Transition, len(list.Transitions))
	for i, tr := range list.Transitions {
		out[i] = ticket.Transition{ID: tr.ID, Name: tr.Name, To: tr.To.Name}
		if len(tr.Fields) > 0 {
			out[i].Fields = p.specs(issueID, tr.Fields, false)
		}
	}
	return out
}

// The provider reads an issue and its transitions together.
var _ provider.Loader = (*Provider)(nil)

// Load reads an issue and the transitions open to it, with the two
// requests made at the same time. What the transition screens say about a
// field is remembered after what the edit screen says, as it is when
// Transitions is called after Get.
func (p *Provider) Load(ctx context.Context, id string) (ticket.Ticket, []ticket.Transition, error) {
	var (
		list    transitionList
		listErr error
	)
	read := make(chan struct{})
	go func() {
		defer close(read)
		list, listErr = p.readTransitions(ctx, id)
	}()
	t, err := p.Get(ctx, id)
	<-read
	if err != nil {
		return t, nil, err
	}
	if listErr != nil {
		// An issue whose transitions cannot be read is still worth showing.
		return t, nil, nil
	}
	return t, p.toTransitions(t.ID, list), nil
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

// CommentFormat is Atlassian Document Format.
func (p *Provider) CommentFormat() string { return provider.CommentJira }

// commentKinds are the kinds of comment a service project has: a reply the
// customer sees and a note the customer does not.
var commentKinds = []provider.CommentKind{
	{ID: provider.KindPublic, Name: "public"},
	{ID: provider.KindInternal, Name: "internal"},
}

// publicProperty is the comment property a service project reads to tell
// a reply to the customer from an internal note.
const publicProperty = "sd.public.comment"

// CommentKinds are the kinds the profile turned on.
func (p *Provider) CommentKinds() []provider.CommentKind { return p.kinds }

// SetCommentKinds turns on the kinds of comment named, the default first.
func (p *Provider) SetCommentKinds(ids []string) error {
	kinds, err := provider.PickKinds(ids, commentKinds...)
	if err != nil {
		return fmt.Errorf("jira: %w", err)
	}
	p.kinds = kinds
	return nil
}

// AddComment adds a comment to an issue. body is an Atlassian Document
// Format document as JSON. A kind is sent as the property a service
// project reads; an issue of any other project keeps the property and
// shows the comment as it shows every other.
func (p *Provider) AddComment(ctx context.Context, id, kind, body string) error {
	if !json.Valid([]byte(body)) {
		return errors.New("the comment is not a valid document")
	}
	req := map[string]any{"body": json.RawMessage(body)}
	switch kind {
	case "":
	case provider.KindPublic, provider.KindInternal:
		req["properties"] = []map[string]any{{
			"key":   publicProperty,
			"value": map[string]bool{"internal": kind == provider.KindInternal},
		}}
	default:
		return fmt.Errorf("jira has no comment kind %q", kind)
	}
	path := "/rest/api/3/issue/" + url.PathEscape(id) + "/comment"
	return p.client.Do(ctx, http.MethodPost, path, req, nil)
}

// The provider reads attachments.
var _ provider.AttachmentReader = (*Provider)(nil)

// ReadAttachment reads a file by the ID the issue lists it under. Jira
// answers with a redirect to its media store, to an address that carries
// its own permission; the credentials are not sent there.
func (p *Provider) ReadAttachment(ctx context.Context, a ticket.Attachment) (io.ReadCloser, error) {
	resp, err := p.client.Raw(ctx, http.MethodGet, "/rest/api/3/attachment/content/"+url.PathEscape(a.ID), nil, nil)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
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

// toTicket maps one issue into the neutral model. byMedia is the issue's
// attachments by their ID in the media store, where Get has found them.
func (p *Provider) toTicket(is issue, byMedia map[string]ticket.Attachment) ticket.Ticket {
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
		Fields:   map[string]ticket.Value{},
	}
	t.Attachments = attachments(f["attachment"])
	t.Links = links(f["issuelinks"], p.client.BaseURL())
	shown := files{all: t.Attachments, byMedia: byMedia}
	t.Body = adfToMarkdown(f["description"], shown)
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
				// Public is false on an internal note of a service
				// project, and absent outside one.
				Public *bool `json:"jsdPublic"`
			} `json:"comments"`
		}
		if json.Unmarshal(raw, &block) == nil {
			for _, c := range block.Comments {
				kind := ""
				if c.Public != nil && !*c.Public {
					kind = "internal"
				}
				t.Comments = append(t.Comments, ticket.Comment{
					Author:  c.Author.DisplayName,
					Created: parseTime(c.Created),
					Body:    adfToMarkdown(c.Body, shown),
					Kind:    kind,
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
			return ticket.Text(adfToMarkdown(raw, files{}))
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
		// A service desk request names its type one level down.
		if rt, ok := v["requestType"].(map[string]any); ok {
			return scalar(rt)
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
