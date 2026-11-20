// Package people keeps the people a user has mentioned in comments, per
// site, so that the list after "@" can offer them without asking the
// platform. They are stored per machine, outside every profile, in the
// user's configuration directory.
package people

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/lawsmd/tui-q/ticket"
)

// Limit is how many people a store keeps. The ones mentioned longest ago
// are dropped first.
const Limit = 50

// person is one entry of the file.
type person struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Store is the people mentioned on one site, the most recent first. A
// Store with no path keeps them in memory only. It is safe to use from
// more than one goroutine: the editor asks for matches from its own.
type Store struct {
	mu   sync.Mutex
	path string
	list []person
}

// Path is the file for a site, named by a hash of key so that the file
// name says nothing about the site.
func Path(key string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(dir, "tui-q", "people", hex.EncodeToString(sum[:6])+".json"), nil
}

// Open reads the people at path. A missing or unreadable file gives an
// empty store; an empty path gives a store that is never saved.
func Open(path string) *Store {
	s := &Store{path: path}
	if path != "" {
		if data, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(data, &s.list)
		}
	}
	return s
}

// Match returns the people whose names contain query, whatever its case,
// the most recently mentioned first. A blank query matches everyone.
func (s *Store) Match(query string) []ticket.User {
	query = strings.ToLower(strings.TrimSpace(query))
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []ticket.User
	for _, p := range s.list {
		if strings.Contains(strings.ToLower(p.Name), query) {
			out = append(out, ticket.User{ID: p.ID, Name: p.Name})
		}
	}
	return out
}

// Note records that users were mentioned, putting them first, and saves
// the file. A user with no ID or no name is left out.
func (s *Store) Note(users ...ticket.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	for i := len(users) - 1; i >= 0; i-- {
		u := users[i]
		if u.ID == "" || strings.TrimSpace(u.Name) == "" {
			continue
		}
		list := []person{{ID: u.ID, Name: u.Name}}
		for _, p := range s.list {
			if p.ID != u.ID {
				list = append(list, p)
			}
		}
		s.list = list[:min(len(list), Limit)]
		changed = true
	}
	if !changed || s.path == "" {
		return nil
	}
	data, err := json.MarshalIndent(s.list, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(s.path, append(data, '\n'), 0o600)
}

// link matches a mention as the comment box writes it in Markdown:
// [@Name](mention:ID), with the ID escaped as part of a URL.
var link = regexp.MustCompile(`\[@([^\]\n]*)\]\(mention:([^)\s]+)\)`)

// Links returns the people mentioned in the Markdown of a comment, each
// once, in the order they first appear.
func Links(markdown string) []ticket.User {
	var out []ticket.User
	seen := map[string]bool{}
	for _, m := range link.FindAllStringSubmatch(markdown, -1) {
		id, err := url.PathUnescape(m[2])
		if err != nil {
			id = m[2]
		}
		name := strings.TrimSpace(strings.ReplaceAll(m[1], `\`, ""))
		if id == "" || name == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, ticket.User{ID: id, Name: name})
	}
	return out
}
