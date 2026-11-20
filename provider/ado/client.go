// Package ado is the provider for Azure DevOps Boards, over the REST API.
package ado

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// apiVersion is the REST API version every request names.
const apiVersion = "7.1"

// resourceID is the Entra ID application of Azure DevOps, the audience of
// a token for it.
const resourceID = "499b84ac-1321-427f-aa17-267ca6975798"

// Auth signs one request.
type Auth func(ctx context.Context, req *http.Request) error

// PAT signs requests with a personal access token.
func PAT(token string) Auth {
	return func(_ context.Context, req *http.Request) error {
		req.SetBasicAuth("", token)
		return nil
	}
}

// AzureCLI signs requests with a token from the Azure CLI's sign-in. It
// runs az once and again shortly before the token expires. The az program
// is taken from AZURE_CLI_PATH when that is set, and from PATH otherwise.
func AzureCLI() Auth {
	var (
		mu      sync.Mutex
		token   string
		expires time.Time
	)
	return func(ctx context.Context, req *http.Request) error {
		mu.Lock()
		defer mu.Unlock()
		if token == "" || time.Now().After(expires.Add(-5*time.Minute)) {
			t, exp, err := azToken(ctx)
			if err != nil {
				return err
			}
			token, expires = t, exp
		}
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	}
}

func azProgram() string {
	name := "az"
	if runtime.GOOS == "windows" {
		name = "az.cmd"
	}
	if dir := os.Getenv("AZURE_CLI_PATH"); dir != "" {
		return filepath.Join(dir, name)
	}
	return name
}

func azToken(ctx context.Context) (string, time.Time, error) {
	cmd := exec.CommandContext(ctx, azProgram(), "account", "get-access-token", "--resource", resourceID, "--output", "json")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if i := strings.IndexByte(msg, '\n'); i > 0 {
			msg = msg[:i]
		}
		return "", time.Time{}, fmt.Errorf("ado: az account get-access-token: %v: %s", err, msg)
	}
	var resp struct {
		AccessToken string `json:"accessToken"`
		ExpiresOn   int64  `json:"expires_on"`
	}
	if err := json.Unmarshal(out, &resp); err != nil || resp.AccessToken == "" {
		return "", time.Time{}, fmt.Errorf("ado: az account get-access-token gave no token")
	}
	exp := time.Now().Add(30 * time.Minute)
	if resp.ExpiresOn > 0 {
		exp = time.Unix(resp.ExpiresOn, 0)
	}
	return resp.AccessToken, exp, nil
}

// Client is a thin HTTP client for one Azure DevOps organisation. It is
// exported so an instance module can call endpoints the provider does not
// cover.
type Client struct {
	base string
	auth Auth
	http *http.Client
}

// NewClient returns a Client for the organisation at base, such as
// https://dev.azure.com/example.
func NewClient(base string, auth Auth) *Client {
	return &Client{
		base: strings.TrimRight(base, "/"),
		auth: auth,
		http: &http.Client{Timeout: 60 * time.Second},
	}
}

// BaseURL is the organisation URL with no trailing slash.
func (c *Client) BaseURL() string { return c.base }

// Error is a response with a status outside 200-299.
type Error struct {
	Method  string
	Path    string
	Status  int
	Message string
}

func (e *Error) Error() string {
	detail := e.Message
	if detail == "" {
		detail = http.StatusText(e.Status)
	}
	return fmt.Sprintf("%s %s: %d %s", e.Method, e.Path, e.Status, detail)
}

// Do sends one request. path starts with "/" and is appended to the
// organisation URL; the api-version parameter is added when path has none.
// A non-nil body is sent as JSON. A non-nil out receives the decoded JSON
// response.
func (c *Client) Do(ctx context.Context, method, path string, body, out any) error {
	if !strings.Contains(path, "api-version=") {
		sep := "?"
		if strings.Contains(path, "?") {
			sep = "&"
		}
		path += sep + "api-version=" + apiVersion
	}
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
	if err := c.auth(ctx, req); err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return fmt.Errorf("%s %s: read response: %w", method, path, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &Error{Method: method, Path: path, Status: resp.StatusCode, Message: errorMessage(data)}
	}
	// A sign-in page instead of JSON means the credentials were refused.
	if ct := resp.Header.Get("Content-Type"); strings.HasPrefix(ct, "text/html") {
		return &Error{Method: method, Path: path, Status: http.StatusUnauthorized, Message: "the credentials were refused"}
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
// such as a file. path starts with "/" and is appended to the organisation
// URL; the api-version parameter is added when path has none. The caller
// reads and closes the body of the response. A status outside 200-299 is
// returned as an *Error.
func (c *Client) Raw(ctx context.Context, method, path string, header http.Header, body io.Reader) (*http.Response, error) {
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("%s %s: not a path on %s", method, path, c.base)
	}
	if !strings.Contains(path, "api-version=") {
		sep := "?"
		if strings.Contains(path, "?") {
			sep = "&"
		}
		path += sep + "api-version=" + apiVersion
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return nil, err
	}
	for name, values := range header {
		req.Header[name] = values
	}
	if err := c.auth(ctx, req); err != nil {
		return nil, err
	}
	// A file can take longer than the limit Do works under; the context
	// bounds this request.
	client := *c.http
	client.Timeout = 0
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		return nil, &Error{Method: method, Path: path, Status: resp.StatusCode, Message: errorMessage(data)}
	}
	// A sign-in page instead of the file means the credentials were refused.
	if ct := resp.Header.Get("Content-Type"); strings.HasPrefix(ct, "text/html") {
		resp.Body.Close()
		return nil, &Error{Method: method, Path: path, Status: http.StatusUnauthorized, Message: "the credentials were refused"}
	}
	return resp, nil
}

func errorMessage(data []byte) string {
	var body struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(data, &body) != nil {
		return ""
	}
	return body.Message
}
