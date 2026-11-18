package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/lawsmd/tui-q/ticket"
)

// rawMeta is how Jira describes one field of an edit screen or of a
// transition screen.
type rawMeta struct {
	Required bool   `json:"required"`
	Name     string `json:"name"`
	Schema   struct {
		Type   string `json:"type"`
		Items  string `json:"items"`
		System string `json:"system"`
		Custom string `json:"custom"`
	} `json:"schema"`
	AllowedValues []json.RawMessage `json:"allowedValues"`
}

// kind maps a Jira field schema to the way tui-q edits it.
func (r rawMeta) kind() string {
	s := r.Schema
	switch s.Type {
	case "user":
		return ticket.KindUser
	case "option", "priority", "resolution":
		return ticket.KindSelect
	case "number":
		return ticket.KindNumber
	case "date":
		return ticket.KindText
	case "string":
		if strings.HasSuffix(s.Custom, ":textarea") || s.System == "description" || s.System == "environment" {
			return ticket.KindRichText
		}
		return ticket.KindText
	case "array":
		switch s.Items {
		case "string":
			return ticket.KindTags
		case "option", "component", "version":
			return ticket.KindMulti
		}
	}
	return ticket.KindNone
}

// options reads the allowed values of a select field.
func (r rawMeta) options() []ticket.Option {
	var out []ticket.Option
	for _, raw := range r.AllowedValues {
		var v map[string]any
		if json.Unmarshal(raw, &v) != nil {
			continue
		}
		id, _ := v["id"].(string)
		if name := scalar(v); id != "" && name != "" {
			out = append(out, ticket.Option{ID: id, Name: name})
		}
	}
	return out
}

// systemNames maps Jira's built-in field IDs to tui-q's built-in names.
var systemNames = map[string]string{
	"summary":     ticket.FieldTitle,
	"description": ticket.FieldBody,
	"issuetype":   ticket.FieldType,
	"assignee":    ticket.FieldAssignee,
	"priority":    ticket.FieldPriority,
}

// nameOf is the profile's name for a Jira field ID, a built-in name, or
// the ID itself.
func (p *Provider) nameOf(id string) string {
	for name, fid := range p.fields {
		if fid == id {
			return name
		}
	}
	if name, ok := systemNames[id]; ok {
		return name
	}
	return id
}

// specs turns Jira's field descriptions into specs, sorted by label, and
// remembers them for the issue so a later write knows each field's shape.
// edit says the fields are the issue's whole edit screen.
func (p *Provider) specs(issueID string, fields map[string]rawMeta, edit bool) []ticket.FieldSpec {
	out := make([]ticket.FieldSpec, 0, len(fields))
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.meta[issueID] == nil {
		p.meta[issueID] = map[string]ticket.FieldSpec{}
	}
	if edit {
		p.edit[issueID] = true
	}
	for id, raw := range fields {
		spec := ticket.FieldSpec{
			Name:     p.nameOf(id),
			Label:    raw.Name,
			Kind:     raw.kind(),
			Required: raw.Required,
			Options:  raw.options(),
		}
		p.meta[issueID][id] = spec
		out = append(out, spec)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out
}

// spec returns what is known about one field of an issue, loading the
// issue's edit screen when nothing is remembered for it.
func (p *Provider) spec(ctx context.Context, issueID, fieldID string) (ticket.FieldSpec, bool) {
	p.mu.Lock()
	spec, ok := p.meta[issueID][fieldID]
	loaded := p.edit[issueID]
	p.mu.Unlock()
	if ok || loaded {
		return spec, ok
	}
	var resp struct {
		Fields map[string]rawMeta `json:"fields"`
	}
	path := "/rest/api/3/issue/" + url.PathEscape(issueID) + "/editmeta"
	if p.client.Do(ctx, http.MethodGet, path, nil, &resp) != nil {
		return ticket.FieldSpec{}, false
	}
	p.specs(issueID, resp.Fields, true)
	p.mu.Lock()
	defer p.mu.Unlock()
	spec, ok = p.meta[issueID][fieldID]
	return spec, ok
}

// encodeField shapes a value the way Jira wants it for one field of one
// issue. A field Jira has not described is sent as plain text or a plain
// list.
func (p *Provider) encodeField(ctx context.Context, issueID, name string, v ticket.Value) (string, any, error) {
	id := p.fieldID(name)
	spec, ok := p.spec(ctx, issueID, id)
	if !ok {
		return id, encode(v), nil
	}
	out, err := encodeAs(spec, v)
	if err != nil {
		return id, nil, fmt.Errorf("%s: %w", spec.Label, err)
	}
	return id, out, nil
}

// encodeAs shapes a value for a field of a known kind. An empty value
// clears the field.
func encodeAs(spec ticket.FieldSpec, v ticket.Value) (any, error) {
	items := v.Items
	if items == nil && v.Text != "" {
		items = []string{v.Text}
	}
	optionID := func(name string) (string, error) {
		for _, o := range spec.Options {
			if strings.EqualFold(o.Name, name) {
				return o.ID, nil
			}
		}
		return "", fmt.Errorf("%q is not one of its options", name)
	}
	switch spec.Kind {
	case ticket.KindText:
		if v.Text == "" {
			return nil, nil
		}
		return v.Text, nil
	case ticket.KindRichText:
		if strings.TrimSpace(v.Text) == "" {
			return nil, nil
		}
		return textToADF(v.Text), nil
	case ticket.KindNumber:
		if strings.TrimSpace(v.Text) == "" {
			return nil, nil
		}
		n, err := strconv.ParseFloat(strings.TrimSpace(v.Text), 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not a number", v.Text)
		}
		return n, nil
	case ticket.KindSelect:
		if v.Text == "" {
			return nil, nil
		}
		id, err := optionID(v.Text)
		if err != nil {
			return nil, err
		}
		return map[string]string{"id": id}, nil
	case ticket.KindMulti:
		out := make([]map[string]string, 0, len(items))
		for _, item := range items {
			id, err := optionID(item)
			if err != nil {
				return nil, err
			}
			out = append(out, map[string]string{"id": id})
		}
		return out, nil
	case ticket.KindTags:
		out := make([]string, 0, len(items))
		for _, item := range items {
			if strings.ContainsAny(item, " \t") {
				return nil, fmt.Errorf("%q has a space in it", item)
			}
			out = append(out, item)
		}
		return out, nil
	case ticket.KindUser:
		if v.ID == "" {
			return nil, nil
		}
		return map[string]string{"accountId": v.ID}, nil
	}
	return nil, fmt.Errorf("this kind of field cannot be edited here")
}

// textToADF wraps plain text in a rich text document: a blank line starts
// a new paragraph and a single line break is kept as a break.
func textToADF(text string) map[string]any {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var paragraphs []any
	for _, para := range strings.Split(text, "\n\n") {
		para = strings.Trim(para, "\n")
		if strings.TrimSpace(para) == "" {
			continue
		}
		var content []any
		for i, line := range strings.Split(para, "\n") {
			if i > 0 {
				content = append(content, map[string]any{"type": "hardBreak"})
			}
			if line != "" {
				content = append(content, map[string]any{"type": "text", "text": line})
			}
		}
		paragraphs = append(paragraphs, map[string]any{"type": "paragraph", "content": content})
	}
	return map[string]any{"type": "doc", "version": 1, "content": paragraphs}
}

// Me is the account the credentials belong to.
func (p *Provider) Me(ctx context.Context) (ticket.User, error) {
	var resp struct {
		AccountID   string `json:"accountId"`
		DisplayName string `json:"displayName"`
	}
	if err := p.client.Do(ctx, http.MethodGet, "/rest/api/3/myself", nil, &resp); err != nil {
		return ticket.User{}, err
	}
	return ticket.User{ID: resp.AccountID, Name: resp.DisplayName}, nil
}

// userLimit is how many matching users a search returns.
const userLimit = 20

// Users lists the people an issue can be assigned to whose names match
// query.
func (p *Provider) Users(ctx context.Context, id, query string) ([]ticket.User, error) {
	var resp []struct {
		AccountID   string `json:"accountId"`
		DisplayName string `json:"displayName"`
	}
	path := "/rest/api/3/user/assignable/search?issueKey=" + url.QueryEscape(id) +
		"&query=" + url.QueryEscape(query) + "&maxResults=" + strconv.Itoa(userLimit)
	if err := p.client.Do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	out := make([]ticket.User, 0, len(resp))
	for _, u := range resp {
		out = append(out, ticket.User{ID: u.AccountID, Name: u.DisplayName})
	}
	return out, nil
}

// FieldLabels gives Jira's name for each custom field the profile maps.
func (p *Provider) FieldLabels(ctx context.Context) (map[string]string, error) {
	var resp []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := p.client.Do(ctx, http.MethodGet, "/rest/api/3/field", nil, &resp); err != nil {
		return nil, err
	}
	names := make(map[string]string, len(resp))
	for _, f := range resp {
		names[f.ID] = f.Name
	}
	out := map[string]string{}
	for name, id := range p.fields {
		if label := names[id]; label != "" {
			out[name] = label
		}
	}
	return out, nil
}
