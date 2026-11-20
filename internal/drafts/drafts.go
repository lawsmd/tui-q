// Package drafts keeps the comments a user has started and not sent. Each
// is Markdown in a file of its own, stored per machine in the user's
// configuration directory, beside the settings of package prefs.
package drafts

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Store is the drafts of one site, by ticket ID. A Store with no directory
// keeps them in memory only.
type Store struct {
	dir string
	// held is every draft read or written so far. An empty text says the
	// ticket has none.
	held map[string]string
}

// Dir is the directory for the drafts of a site, named by a hash of key so
// that the name says nothing about the site.
func Dir(key string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tui-q", "drafts", hash(key)), nil
}

func hash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:6])
}

// Open returns the drafts kept in dir. An empty dir gives drafts that last
// until the program exits.
func Open(dir string) *Store {
	return &Store{dir: dir, held: map[string]string{}}
}

func (s *Store) path(id string) string {
	return filepath.Join(s.dir, hash(id)+".md")
}

// Get returns the draft for a ticket, or an empty text when it has none.
func (s *Store) Get(id string) string {
	if text, ok := s.held[id]; ok {
		return text
	}
	text := ""
	if s.dir != "" {
		if data, err := os.ReadFile(s.path(id)); err == nil {
			text = string(data)
		}
	}
	s.held[id] = text
	return text
}

// Set keeps text as the draft for a ticket. Text that is empty or all
// spaces removes the draft.
func (s *Store) Set(id, text string) error {
	if strings.TrimSpace(text) == "" {
		text = ""
	}
	s.held[id] = text
	if s.dir == "" {
		return nil
	}
	if text == "" {
		if err := os.Remove(s.path(id)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(s.path(id), []byte(text), 0o600)
}
