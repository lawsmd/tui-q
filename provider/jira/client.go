// Package jira is the provider for Jira Cloud, including Jira Service
// Management projects, over REST API v3.
package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Client is a thin HTTP client for one Jira Cloud site. It is exported so an
// instance module can call endpoints the provider does not cover.
type Client struct {
	base  string
	user  string
	token string
	http  *http.Client
}

// NewClient returns a Client for the site at base, signing requests with an
// account email and an API token.
func NewClient(base, user, token string) *Client {
	return &Client{
		base:  strings.TrimRight(base, "/"),
		user:  user,
		token: token,
		http:  &http.Client{Timeout: 60 * time.Second},
	}
}

// BaseURL is the site URL with no trailing slash.
func (c *Client) BaseURL() string { return c.base }

// Error is a response with a status outside 200-299.
type Error struct {
	Method   string
	Path     string
	Status   int
	Messages []string
}

func (e *Error) Error() string {
	detail := strings.Join(e.Messages, "; ")
	if detail == "" {
		detail = http.StatusText(e.Status)
	}
	return fmt.Sprintf("%s %s: %d %s", e.Method, e.Path, e.Status, detail)
}

// Do sends one request. path starts with "/" and is appended to the site
// URL. A non-nil body is sent as JSON. A non-nil out receives the decoded
// JSON response.
func (c *Client) Do(ctx context.Context, method, path string, body, out any) error {
	var payload io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode %s %s: %w", method, path, err)
		}
		payload = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, payload)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.user, c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return fmt.Errorf("%s %s: read response: %w", method, path, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &Error{Method: method, Path: path, Status: resp.StatusCode, Messages: errorMessages(data)}
	}
	if out == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("%s %s: decode response: %w", method, path, err)
	}
	return nil
}

// Raw sends a request whose body is not JSON, or whose response is not,
// such as a file. target is a path that starts with "/" or a full URL on
// this site; a URL on any other host is refused, since the request carries
// the credentials. The caller reads and closes the body of the response. A
// status outside 200-299 is returned as an *Error.
func (c *Client) Raw(ctx context.Context, method, target string, header http.Header, body io.Reader) (*http.Response, error) {
	full := target
	if strings.HasPrefix(target, "/") {
		full = c.base + target
	} else if !strings.HasPrefix(target, c.base+"/") {
		return nil, fmt.Errorf("%s %s: not a URL on %s", method, target, c.base)
	}
	req, err := http.NewRequestWithContext(ctx, method, full, body)
	if err != nil {
		return nil, err
	}
	for name, values := range header {
		req.Header[name] = values
	}
	req.SetBasicAuth(c.user, c.token)
	// A file can take longer than the limit Do works under; the context
	// bounds this request.
	client := *c.http
	client.Timeout = 0
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, target, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		return nil, &Error{Method: method, Path: target, Status: resp.StatusCode, Messages: errorMessages(data)}
	}
	return resp, nil
}

// errorMessages pulls the readable parts out of a Jira error body.
func errorMessages(data []byte) []string {
	var body struct {
		ErrorMessages []string          `json:"errorMessages"`
		Errors        map[string]string `json:"errors"`
		Message       string            `json:"message"`
	}
	if json.Unmarshal(data, &body) != nil {
		return nil
	}
	msgs := append([]string(nil), body.ErrorMessages...)
	fields := make([]string, 0, len(body.Errors))
	for field := range body.Errors {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	for _, field := range fields {
		msgs = append(msgs, field+": "+body.Errors[field])
	}
	if body.Message != "" {
		msgs = append(msgs, body.Message)
	}
	return msgs
}
