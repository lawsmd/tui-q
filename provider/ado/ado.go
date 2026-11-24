package ado

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
const Kind = "ado"

// batchSize is the most work items one batch read returns.
const batchSize = 200

// readers is how many batch reads run at once.
const readers = 4

// The provider adds comments.
var _ provider.Commenter = (*Provider)(nil)

func init() {
	provider.Register(Kind, func(cfg provider.Config) (provider.Provider, error) {
		return New(cfg)
	})
}

// Fields read for every work item, whatever the profile asks for.
const (
	fieldTitle    = "System.Title"
	fieldState    = "System.State"
	fieldType     = "System.WorkItemType"
	fieldAssignee = "System.AssignedTo"
	fieldCreated  = "System.CreatedDate"
	fieldChanged  = "System.ChangedDate"
	fieldBody     = "System.Description"
	fieldTags     = "System.Tags"
)

var baseFields = []string{"System.Id", fieldTitle, fieldState, fieldType, fieldAssignee, fieldCreated, fieldChanged}

// Provider talks to one project of an Azure DevOps organisation.
type Provider struct {
	client  *Client
	project string
	// fields maps a profile field name to a field reference name.
	fields map[string]string
	meta   meta
}

// New builds a Provider from a profile's [provider] section. base_url is
// the organisation URL and default_project the project. With token_env set
// the provider signs in with the personal access token in that variable;
// without it, with the Azure CLI's sign-in.
func New(cfg provider.Config) (*Provider, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("ado: [provider] base_url is required")
	}
	if cfg.DefaultProject == "" {
		return nil, fmt.Errorf("ado: [provider] default_project is required")
	}
	auth := AzureCLI()
	if cfg.TokenEnv != "" {
		token := os.Getenv(cfg.TokenEnv)
		if token == "" {
			return nil, fmt.Errorf("ado: environment variable %s is not set", cfg.TokenEnv)
		}
		auth = PAT(token)
	}
	// Azure DevOps has one kind of comment, so a profile can turn on none.
	if _, err := provider.PickKinds(cfg.CommentKinds); err != nil {
		return nil, fmt.Errorf("ado: %w", err)
	}
	return NewProvider(NewClient(cfg.BaseURL, auth), cfg.DefaultProject, cfg.Fields), nil
}

// NewProvider builds a Provider around an existing Client.
func NewProvider(client *Client, project string, fields map[string]string) *Provider {
	return &Provider{client: client, project: project, fields: fields}
}

// Client returns the HTTP client, for instance code that needs endpoints
// beyond the Provider interface.
func (p *Provider) Client() *Client { return p.client }

// Project is the project the provider reads.
func (p *Provider) Project() string { return p.project }

func (p *Provider) projectPath() string { return "/" + url.PathEscape(p.project) }

// Resolve reads a work item number from a number or a link to a work item.
func (p *Provider) Resolve(input string) (string, error) {
	return provider.NumberRef(input)
}

// FieldIDs is every field reference name a list read asks for: the base
// set, the profile's own, and any extras.
func (p *Provider) FieldIDs(extra ...string) []string {
	ids := append([]string(nil), baseFields...)
	custom := make([]string, 0, len(p.fields))
	for _, id := range p.fields {
		custom = append(custom, id)
	}
	sort.Strings(custom)
	seen := map[string]bool{}
	var out []string
	for _, id := range append(append(ids, custom...), extra...) {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// WorkItem is one work item as the REST API returns it.
type WorkItem struct {
	ID     int                        `json:"id"`
	Fields map[string]json.RawMessage `json:"fields"`
}

// MaxQuery is the most IDs one WIQL query returns. A query that matches
// more is cut off there; Azure DevOps refuses it outright without the
// limit.
const MaxQuery = 20000

// Query runs a WIQL query and returns the IDs it matches, in its order, at
// most MaxQuery of them.
func (p *Provider) Query(ctx context.Context, wiql string) ([]int, error) {
	var resp struct {
		WorkItems []struct {
			ID int `json:"id"`
		} `json:"workItems"`
	}
	path := p.projectPath() + "/_apis/wit/wiql?$top=" + strconv.Itoa(MaxQuery)
	if err := p.client.Do(ctx, http.MethodPost, path, map[string]string{"query": wiql}, &resp); err != nil {
		return nil, err
	}
	ids := make([]int, len(resp.WorkItems))
	for i, w := range resp.WorkItems {
		ids[i] = w.ID
	}
	return ids, nil
}

// Read loads the given fields of the work items, in the order of ids. A
// work item that no longer exists is left out.
func (p *Provider) Read(ctx context.Context, ids []int, fields []string) ([]WorkItem, error) {
	var (
		mu       sync.Mutex
		byID     = map[int]WorkItem{}
		firstErr error
		wg       sync.WaitGroup
		slots    = make(chan struct{}, readers)
	)
	for start := 0; start < len(ids); start += batchSize {
		chunk := ids[start:min(start+batchSize, len(ids))]
		wg.Add(1)
		slots <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			var resp struct {
				Value []WorkItem `json:"value"`
			}
			body := map[string]any{"ids": chunk, "fields": fields, "errorPolicy": "omit"}
			err := p.client.Do(ctx, http.MethodPost, p.projectPath()+"/_apis/wit/workitemsbatch", body, &resp)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			for _, w := range resp.Value {
				// errorPolicy omit gives null for a missing item.
				if w.ID != 0 {
					byID[w.ID] = w
				}
			}
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	out := make([]WorkItem, 0, len(byID))
	for _, id := range ids {
		if w, ok := byID[id]; ok {
			out = append(out, w)
		}
	}
	return out, nil
}

// Search runs a WIQL query and loads every work item it matches.
func (p *Provider) Search(ctx context.Context, query string) ([]ticket.Ticket, error) {
	ids, err := p.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	items, err := p.Read(ctx, ids, p.FieldIDs())
	if err != nil {
		return nil, err
	}
	out := make([]ticket.Ticket, len(items))
	for i, w := range items {
		out[i] = p.ToTicket(w)
	}
	return out, nil
}

// SearchPage runs a WIQL query and loads one page of it. The cursor is the
// position of the page's first work item.
func (p *Provider) SearchPage(ctx context.Context, query, cursor string, limit int) (provider.Page, error) {
	start := 0
	if cursor != "" {
		n, err := strconv.Atoi(cursor)
		if err != nil || n < 0 {
			return provider.Page{}, fmt.Errorf("ado: bad page cursor %q", cursor)
		}
		start = n
	}
	ids, err := p.Query(ctx, query)
	if err != nil {
		return provider.Page{}, err
	}
	page := provider.Page{Total: len(ids)}
	if start >= len(ids) {
		return page, nil
	}
	end := min(start+max(limit, 1), len(ids))
	items, err := p.Read(ctx, ids[start:end], p.FieldIDs())
	if err != nil {
		return provider.Page{}, err
	}
	for _, w := range items {
		page.Tickets = append(page.Tickets, p.ToTicket(w))
	}
	if end < len(ids) {
		page.Next = strconv.Itoa(end)
	}
	return page, nil
}

// Get loads one work item with its description and comments. The work
// item and its comments are two requests, made at the same time.
func (p *Provider) Get(ctx context.Context, id string) (ticket.Ticket, error) {
	if _, err := strconv.Atoi(id); err != nil {
		return ticket.Ticket{}, fmt.Errorf("ado: %q is not a work item number", id)
	}
	var comments struct {
		Comments []struct {
			Text string `json:"text"`
			// RenderedText is the HTML Azure DevOps shows, also for a
			// comment written in Markdown, with each mention as a name.
			RenderedText string `json:"renderedText"`
			CreatedBy    struct {
				DisplayName string `json:"displayName"`
			} `json:"createdBy"`
			CreatedDate json.RawMessage `json:"createdDate"`
		} `json:"comments"`
	}
	var commentsErr error
	read := make(chan struct{})
	go func() {
		defer close(read)
		path := p.projectPath() + "/_apis/wit/workItems/" + id + "/comments?order=asc&$expand=renderedText&api-version=" + apiVersion + "-preview.4"
		commentsErr = p.client.Do(ctx, http.MethodGet, path, nil, &comments)
	}()

	var (
		w        WorkItem
		specs    []ticket.FieldSpec
		specsErr error
	)
	err := p.client.Do(ctx, http.MethodGet, p.projectPath()+"/_apis/wit/workitems/"+id, nil, &w)
	if err == nil {
		// The fields of a type are read the first time the type is seen,
		// while the comments are still being read.
		specs, specsErr = p.editable(ctx, decode(w.Fields[fieldType]).Text)
	}
	<-read
	if err = cmp.Or(err, commentsErr); err != nil {
		return ticket.Ticket{}, err
	}
	t := p.ToTicket(w)
	// The attachments are the pictures the description and the comments
	// show, each listed once.
	text := func(html string) string {
		md, found := convert(html)
		for _, a := range found {
			if _, ok := t.Attachment(a.ID); !ok {
				t.Attachments = append(t.Attachments, a)
			}
		}
		return md
	}
	t.Body = text(decode(w.Fields[fieldBody]).Text)
	// The ticket is shown without the fields that can be edited when Azure
	// DevOps does not describe them.
	if specsErr == nil {
		t.Editable = specs
	}
	for _, c := range comments.Comments {
		t.Comments = append(t.Comments, ticket.Comment{
			Author:  c.CreatedBy.DisplayName,
			Created: parseTime(c.CreatedDate),
			Body:    text(cmp.Or(c.RenderedText, c.Text)),
		})
	}
	return t, nil
}

// The provider reads attachments.
var _ provider.AttachmentReader = (*Provider)(nil)

// ReadAttachment reads a file by its attachment ID.
func (p *Provider) ReadAttachment(ctx context.Context, a ticket.Attachment) (io.ReadCloser, error) {
	if !attachmentURL.MatchString("/_apis/wit/attachments/" + a.ID) {
		return nil, fmt.Errorf("ado: %q is not an attachment ID", a.ID)
	}
	resp, err := p.client.Raw(ctx, http.MethodGet, p.projectPath()+"/_apis/wit/attachments/"+a.ID, nil, nil)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// Me is the signed-in user. Its ID is the account's sign-in name, which is
// the uniqueName of an identity field such as System.AssignedTo.
func (p *Provider) Me(ctx context.Context) (ticket.User, error) {
	var resp struct {
		AuthenticatedUser struct {
			ProviderDisplayName string `json:"providerDisplayName"`
			Properties          struct {
				Account struct {
					Value string `json:"$value"`
				} `json:"Account"`
			} `json:"properties"`
		} `json:"authenticatedUser"`
	}
	if err := p.client.Do(ctx, http.MethodGet, "/_apis/connectionData?api-version="+apiVersion+"-preview", nil, &resp); err != nil {
		return ticket.User{}, err
	}
	u := resp.AuthenticatedUser
	return ticket.User{ID: u.Properties.Account.Value, Name: u.ProviderDisplayName}, nil
}

// FieldLabels gives the platform's name for each field of the profile. The
// names come from the description of the project's fields, which is read
// once and kept for the fields that can be edited too.
func (p *Provider) FieldLabels(ctx context.Context) (map[string]string, error) {
	defs, err := p.fieldDefs(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for name, id := range p.fields {
		if def, ok := defs[id]; ok {
			out[name] = def.Name
		}
	}
	return out, nil
}

// URL is the web page of a work item.
func (p *Provider) URL(id int) string {
	return p.client.BaseURL() + p.projectPath() + "/_workitems/edit/" + strconv.Itoa(id)
}

// ToTicket maps one work item into the neutral model.
func (p *Provider) ToTicket(w WorkItem) ticket.Ticket {
	f := w.Fields
	id := strconv.Itoa(w.ID)
	t := ticket.Ticket{
		ID:       id,
		Key:      id,
		Title:    decode(f[fieldTitle]).Text,
		Status:   decode(f[fieldState]).Text,
		Type:     decode(f[fieldType]).Text,
		Assignee: decode(f[fieldAssignee]).Text,
		URL:      p.URL(w.ID),
		Created:  parseTime(f[fieldCreated]),
		Updated:  parseTime(f[fieldChanged]),
		Fields:   map[string]ticket.Value{},
	}
	for name, ref := range p.fields {
		raw, ok := f[ref]
		if !ok {
			continue
		}
		v := decode(raw)
		if ref == fieldTags {
			v = ticket.Value{Items: SplitTags(v.Text)}
		}
		t.Fields[name] = v
		if name == ticket.FieldPriority {
			t.Priority = v.String()
		}
	}
	return t
}

// SplitTags reads the tags of a work item, which Azure DevOps writes as one
// string with "; " between them.
func SplitTags(s string) []string {
	var out []string
	for _, tag := range strings.Split(s, ";") {
		if tag = strings.TrimSpace(tag); tag != "" {
			out = append(out, tag)
		}
	}
	return out
}

// decode turns a field value into a Value. Fields come as strings,
// numbers, booleans, or identities.
func decode(raw json.RawMessage) ticket.Value {
	if len(raw) == 0 {
		return ticket.Value{}
	}
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return ticket.Value{}
	}
	switch v := v.(type) {
	case string:
		return ticket.Text(v)
	case float64:
		return ticket.Text(strconv.FormatFloat(v, 'f', -1, 64))
	case bool:
		return ticket.Text(strconv.FormatBool(v))
	case map[string]any:
		name, _ := v["displayName"].(string)
		unique, _ := v["uniqueName"].(string)
		return ticket.Value{Text: name, ID: unique}
	}
	return ticket.Value{}
}

// Decode turns a field value into a Value, for instance code that reads
// fields the profile does not name.
func Decode(raw json.RawMessage) ticket.Value { return decode(raw) }

func parseTime(raw json.RawMessage) time.Time {
	var s string
	if json.Unmarshal(raw, &s) != nil || s == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}

// ParseTime reads a date field, for instance code.
func ParseTime(raw json.RawMessage) time.Time { return parseTime(raw) }
