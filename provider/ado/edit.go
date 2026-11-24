package ado

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lawsmd/tui-q/provider"
	"github.com/lawsmd/tui-q/ticket"
)

// The provider searches the tags of the project.
var _ provider.Suggester = (*Provider)(nil)

// The provider searches people to mention apart from people to assign.
var _ provider.Mentioner = (*Provider)(nil)

// tagsFor is how long the list of the project's tags is kept before it is
// read again.
const tagsFor = 5 * time.Minute

// suggestLimit is the most values Suggest returns.
const suggestLimit = 30

// meta is what the provider remembers of the project's fields, read once
// and kept for the life of the provider.
type meta struct {
	mu sync.Mutex
	// fields describes every field of the project, by reference name.
	fields map[string]fieldDef
	// types holds the fields of each work item type, by type name.
	types map[string][]typeField
	// states holds the states of each work item type, in the order Azure
	// DevOps lists them.
	states map[string][]string
	tags   []string
	tagsAt time.Time
}

// fieldDef is a field of the project as /_apis/wit/fields describes it.
type fieldDef struct {
	Name          string `json:"name"`
	ReferenceName string `json:"referenceName"`
	Type          string `json:"type"`
	ReadOnly      bool   `json:"readOnly"`
	IsIdentity    bool   `json:"isIdentity"`
}

// typeField is a field of one work item type, with the values it allows.
type typeField struct {
	ReferenceName  string   `json:"referenceName"`
	AlwaysRequired bool     `json:"alwaysRequired"`
	AllowedValues  []string `json:"allowedValues"`
}

// builtIn maps the fields of every work item that tui-q has names for to
// those names.
var builtIn = map[string]string{
	fieldTitle:    ticket.FieldTitle,
	fieldBody:     ticket.FieldBody,
	fieldAssignee: ticket.FieldAssignee,
	fieldState:    ticket.FieldStatus,
	fieldType:     ticket.FieldType,
}

// ref is the reference name of a field named by the profile or built in,
// or the name itself.
func (p *Provider) ref(name string) string {
	if ref, ok := p.fields[name]; ok {
		return ref
	}
	for ref, n := range builtIn {
		if n == name {
			return ref
		}
	}
	return name
}

// nameOf is the profile's name for a reference name, a built-in name, or
// the reference name itself.
func (p *Provider) nameOf(ref string) string {
	for name, r := range p.fields {
		if r == ref {
			return name
		}
	}
	if name, ok := builtIn[ref]; ok {
		return name
	}
	return ref
}

// fieldDefs reads the description of every field of the project.
func (p *Provider) fieldDefs(ctx context.Context) (map[string]fieldDef, error) {
	p.meta.mu.Lock()
	defs := p.meta.fields
	p.meta.mu.Unlock()
	if defs != nil {
		return defs, nil
	}
	var resp struct {
		Value []fieldDef `json:"value"`
	}
	if err := p.client.Do(ctx, http.MethodGet, p.projectPath()+"/_apis/wit/fields", nil, &resp); err != nil {
		return nil, err
	}
	defs = make(map[string]fieldDef, len(resp.Value))
	for _, f := range resp.Value {
		defs[f.ReferenceName] = f
	}
	p.meta.mu.Lock()
	p.meta.fields = defs
	p.meta.mu.Unlock()
	return defs, nil
}

// typeFields reads the fields of one work item type.
func (p *Provider) typeFields(ctx context.Context, typ string) ([]typeField, error) {
	p.meta.mu.Lock()
	fields, ok := p.meta.types[typ]
	p.meta.mu.Unlock()
	if ok {
		return fields, nil
	}
	var resp struct {
		Value []typeField `json:"value"`
	}
	path := p.projectPath() + "/_apis/wit/workitemtypes/" + url.PathEscape(typ) + "/fields?$expand=allowedValues"
	if err := p.client.Do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	p.meta.mu.Lock()
	if p.meta.types == nil {
		p.meta.types = map[string][]typeField{}
	}
	p.meta.types[typ] = resp.Value
	p.meta.mu.Unlock()
	return resp.Value, nil
}

// editable describes the fields of a work item type that the profile names
// or that tui-q has names for, and that Azure DevOps lets a user write.
// The state is left out: Transitions offers it.
func (p *Provider) editable(ctx context.Context, typ string) ([]ticket.FieldSpec, error) {
	if typ == "" {
		return nil, errors.New("ado: the work item has no type")
	}
	defs, err := p.fieldDefs(ctx)
	if err != nil {
		return nil, err
	}
	fields, err := p.typeFields(ctx, typ)
	if err != nil {
		return nil, err
	}
	var out []ticket.FieldSpec
	for _, tf := range fields {
		ref := tf.ReferenceName
		name := p.nameOf(ref)
		def, ok := defs[ref]
		if !ok || def.ReadOnly || name == ref || ref == fieldState || ref == fieldType {
			continue
		}
		out = append(out, specOf(name, def, tf))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out, nil
}

// specOf describes how tui-q edits one field.
func specOf(name string, def fieldDef, tf typeField) ticket.FieldSpec {
	spec := ticket.FieldSpec{Name: name, Label: def.Name, Required: tf.AlwaysRequired}
	switch {
	case def.IsIdentity:
		spec.Kind = ticket.KindUser
	case def.ReferenceName == fieldTags:
		spec.Kind = ticket.KindTags
	case len(tf.AllowedValues) > 0:
		spec.Kind = ticket.KindSelect
		for _, v := range tf.AllowedValues {
			spec.Options = append(spec.Options, ticket.Option{ID: v, Name: v})
		}
	case def.Type == "boolean":
		spec.Kind = ticket.KindSelect
		spec.Options = []ticket.Option{{ID: "true", Name: "true"}, {ID: "false", Name: "false"}}
	case def.Type == "integer" || def.Type == "double":
		spec.Kind = ticket.KindNumber
	case def.Type == "string" || def.Type == "plainText":
		spec.Kind = ticket.KindText
	case def.Type == "html":
		spec.Kind = ticket.KindRichText
	}
	return spec
}

// spec describes one field of a work item, by its reference name.
func (p *Provider) spec(ctx context.Context, id, ref string) (ticket.FieldSpec, error) {
	typ, _, err := p.typeAndState(ctx, id)
	if err != nil {
		return ticket.FieldSpec{}, err
	}
	defs, err := p.fieldDefs(ctx)
	if err != nil {
		return ticket.FieldSpec{}, err
	}
	fields, err := p.typeFields(ctx, typ)
	if err != nil {
		return ticket.FieldSpec{}, err
	}
	def, ok := defs[ref]
	i := slices.IndexFunc(fields, func(f typeField) bool { return f.ReferenceName == ref })
	if !ok || i < 0 {
		return ticket.FieldSpec{}, fmt.Errorf("ado: a %s has no field %s", typ, ref)
	}
	if def.ReadOnly {
		return ticket.FieldSpec{}, fmt.Errorf("ado: %s cannot be changed", def.Name)
	}
	return specOf(p.nameOf(ref), def, fields[i]), nil
}

// typeAndState reads the type and the state of a work item.
func (p *Provider) typeAndState(ctx context.Context, id string) (string, string, error) {
	if _, err := strconv.Atoi(id); err != nil {
		return "", "", fmt.Errorf("ado: %q is not a work item number", id)
	}
	var w WorkItem
	path := p.projectPath() + "/_apis/wit/workitems/" + id + "?fields=" + fieldType + "," + fieldState
	if err := p.client.Do(ctx, http.MethodGet, path, nil, &w); err != nil {
		return "", "", err
	}
	return decode(w.Fields[fieldType]).Text, decode(w.Fields[fieldState]).Text, nil
}

// states reads the states of a work item type.
func (p *Provider) states(ctx context.Context, typ string) ([]string, error) {
	p.meta.mu.Lock()
	states, ok := p.meta.states[typ]
	p.meta.mu.Unlock()
	if ok {
		return states, nil
	}
	var resp struct {
		Value []struct {
			Name string `json:"name"`
		} `json:"value"`
	}
	path := p.projectPath() + "/_apis/wit/workitemtypes/" + url.PathEscape(typ) + "/states"
	if err := p.client.Do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	for _, s := range resp.Value {
		states = append(states, s.Name)
	}
	p.meta.mu.Lock()
	if p.meta.states == nil {
		p.meta.states = map[string][]string{}
	}
	p.meta.states[typ] = states
	p.meta.mu.Unlock()
	return states, nil
}

// Transitions lists every state of the work item's type but the one it is
// in. Azure DevOps lets a work item go from any state to any other unless
// the process has rules against it, which it applies when the move is
// made.
func (p *Provider) Transitions(ctx context.Context, id string) ([]ticket.Transition, error) {
	typ, current, err := p.typeAndState(ctx, id)
	if err != nil {
		return nil, err
	}
	return p.transitions(ctx, typ, current)
}

// The provider reads a work item and its states together.
var _ provider.Loader = (*Provider)(nil)

// Load reads a work item and the states open to it. The states of its
// type are kept after the first read, so the type and state that Get has
// just read are all it takes, and the second read of the work item that
// Transitions makes is saved.
func (p *Provider) Load(ctx context.Context, id string) (ticket.Ticket, []ticket.Transition, error) {
	t, err := p.Get(ctx, id)
	if err != nil {
		return t, nil, err
	}
	// A work item whose states cannot be read is still worth showing.
	trs, _ := p.transitions(ctx, t.Type, t.Status)
	return t, trs, nil
}

// transitions lists every state of a type but the current one.
func (p *Provider) transitions(ctx context.Context, typ, current string) ([]ticket.Transition, error) {
	states, err := p.states(ctx, typ)
	if err != nil {
		return nil, err
	}
	var out []ticket.Transition
	for _, s := range states {
		if !strings.EqualFold(s, current) {
			out = append(out, ticket.Transition{ID: s, Name: s, To: s})
		}
	}
	return out, nil
}

// Transition sets the state of a work item, with any other fields in the
// same change.
func (p *Provider) Transition(ctx context.Context, id string, tr ticket.Transition, fields map[string]ticket.Value) error {
	ops := []PatchOp{{Op: "add", Path: "/fields/" + fieldState, Value: tr.To}}
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		op, err := p.op(ctx, id, name, fields[name])
		if err != nil {
			return err
		}
		ops = append(ops, op)
	}
	return p.patch(ctx, id, ops)
}

// Assign sets the assignee to the person with the sign-in name userID, or
// clears it when userID is empty.
func (p *Provider) Assign(ctx context.Context, id, userID string) error {
	return p.patch(ctx, id, []PatchOp{{Op: "add", Path: "/fields/" + fieldAssignee, Value: userID}})
}

// UpdateField writes one field, shaped by the way Azure DevOps describes
// it for the work item's type.
func (p *Provider) UpdateField(ctx context.Context, id, field string, v ticket.Value) error {
	if field == ticket.FieldAssignee {
		return p.Assign(ctx, id, cmp.Or(v.ID, v.Text))
	}
	op, err := p.op(ctx, id, field, v)
	if err != nil {
		return err
	}
	return p.patch(ctx, id, []PatchOp{op})
}

// op is the patch operation that writes one field.
func (p *Provider) op(ctx context.Context, id, name string, v ticket.Value) (PatchOp, error) {
	ref := p.ref(name)
	spec, err := p.spec(ctx, id, ref)
	if err != nil {
		return PatchOp{}, err
	}
	value, err := encodeAs(spec, v)
	if err != nil {
		return PatchOp{}, fmt.Errorf("%s: %w", spec.Label, err)
	}
	if value == nil {
		return PatchOp{Op: "remove", Path: "/fields/" + ref}, nil
	}
	return PatchOp{Op: "add", Path: "/fields/" + ref, Value: value}, nil
}

// encodeAs shapes a value for a field of a known kind. nil clears the
// field.
func encodeAs(spec ticket.FieldSpec, v ticket.Value) (any, error) {
	text := strings.TrimSpace(v.Text)
	switch spec.Kind {
	case ticket.KindText:
		if text == "" {
			return nil, nil
		}
		return v.Text, nil
	case ticket.KindNumber:
		if text == "" {
			return nil, nil
		}
		n, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not a number", v.Text)
		}
		return n, nil
	case ticket.KindSelect:
		if text == "" {
			return nil, nil
		}
		for _, o := range spec.Options {
			if strings.EqualFold(o.Name, text) {
				if o.ID == "true" || o.ID == "false" {
					return o.ID == "true", nil
				}
				return o.ID, nil
			}
		}
		return nil, fmt.Errorf("%q is not one of its values", text)
	case ticket.KindTags:
		items := v.Items
		if items == nil && text != "" {
			items = SplitTags(text)
		}
		var out []string
		for _, item := range items {
			if item = strings.TrimSpace(item); item == "" {
				continue
			}
			if strings.Contains(item, ";") {
				return nil, fmt.Errorf("%q has a semicolon in it", item)
			}
			if !slices.ContainsFunc(out, func(s string) bool { return strings.EqualFold(s, item) }) {
				out = append(out, item)
			}
		}
		// An empty string clears the tags; remove fails on a work item
		// that has none.
		return strings.Join(out, "; "), nil
	case ticket.KindUser:
		return cmp.Or(v.ID, text), nil
	}
	return nil, errors.New("this kind of field cannot be edited here")
}

// patch sends a change to a work item.
func (p *Provider) patch(ctx context.Context, id string, ops []PatchOp) error {
	if _, err := strconv.Atoi(id); err != nil {
		return fmt.Errorf("ado: %q is not a work item number", id)
	}
	return p.client.Patch(ctx, p.projectPath()+"/_apis/wit/workitems/"+id, ops, nil)
}

// Suggest searches the values the field takes, for a field with more than
// a menu can list. Only the tags have such a search: it returns the tags
// of the project that contain query, those that start with it first.
func (p *Provider) Suggest(ctx context.Context, _ string, field, query string) ([]string, error) {
	if p.ref(field) != fieldTags {
		return nil, nil
	}
	tags, err := p.projectTags(ctx)
	if err != nil {
		return nil, err
	}
	return matchTags(tags, query, suggestLimit), nil
}

// projectTags reads every tag of the project, or the copy read in the last
// tagsFor.
func (p *Provider) projectTags(ctx context.Context) ([]string, error) {
	p.meta.mu.Lock()
	tags, at := p.meta.tags, p.meta.tagsAt
	p.meta.mu.Unlock()
	if tags != nil && time.Since(at) < tagsFor {
		return tags, nil
	}
	var resp struct {
		Value []struct {
			Name string `json:"name"`
		} `json:"value"`
	}
	path := p.projectPath() + "/_apis/wit/tags?api-version=" + apiVersion + "-preview.1"
	if err := p.client.Do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	tags = make([]string, 0, len(resp.Value))
	for _, t := range resp.Value {
		tags = append(tags, t.Name)
	}
	p.meta.mu.Lock()
	p.meta.tags, p.meta.tagsAt = tags, time.Now()
	p.meta.mu.Unlock()
	return tags, nil
}

// matchTags returns at most limit of the tags that contain query, ignoring
// case: those that start with it first, each group in alphabetical order.
func matchTags(tags []string, query string, limit int) []string {
	q := strings.ToLower(strings.TrimSpace(query))
	var starts, contains []string
	for _, tag := range tags {
		low := strings.ToLower(tag)
		switch {
		case strings.HasPrefix(low, q):
			starts = append(starts, tag)
		case strings.Contains(low, q):
			contains = append(contains, tag)
		}
	}
	byName := func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) }
	slices.SortFunc(starts, byName)
	slices.SortFunc(contains, byName)
	out := append(starts, contains...)
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// identity is a person the identity picker found.
type identity struct {
	LocalID       string `json:"localId"`
	DisplayName   string `json:"displayName"`
	SignInAddress string `json:"signInAddress"`
}

// identities finds the people of the organisation whose name or sign-in
// address matches query.
func (p *Provider) identities(ctx context.Context, query string) ([]identity, error) {
	req := map[string]any{
		"query":           query,
		"identityTypes":   []string{"user"},
		"operationScopes": []string{"ims", "source"},
		"properties":      []string{"DisplayName", "SignInAddress"},
		"options":         map[string]int{"MinResults": 5, "MaxResults": 20},
	}
	var resp struct {
		Results []struct {
			Identities []identity `json:"identities"`
		} `json:"results"`
	}
	path := "/_apis/IdentityPicker/Identities?api-version=" + apiVersion + "-preview.1"
	if err := p.client.Do(ctx, http.MethodPost, path, req, &resp); err != nil {
		return nil, err
	}
	var out []identity
	for _, r := range resp.Results {
		out = append(out, r.Identities...)
	}
	return out, nil
}

// Users finds the people whose name or sign-in address matches query, for
// the assignee. A user's ID is the sign-in name, which is what Me returns
// and what System.AssignedTo takes. Azure DevOps refuses a person who is
// not in the organisation when the work item is saved.
func (p *Provider) Users(ctx context.Context, _ string, query string) ([]ticket.User, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	found, err := p.identities(ctx, query)
	if err != nil {
		return nil, err
	}
	var out []ticket.User
	for _, id := range found {
		if id.SignInAddress != "" {
			out = append(out, ticket.User{ID: id.SignInAddress, Name: id.DisplayName})
		}
	}
	return out, nil
}
