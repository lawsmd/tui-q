// Package prefs keeps the settings a user changes from inside the
// interface, such as how a column is shown. They are stored per machine,
// outside every profile, in the user's configuration directory.
package prefs

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

// Column is the settings of one table column.
type Column struct {
	// FirstName shows only the first name of a person.
	FirstName bool `json:"first_name,omitempty"`
}

// Store is the settings of one site. A Store with no path keeps them in
// memory only.
type Store struct {
	path    string
	Columns map[string]Column `json:"columns"`
}

// Path is the settings file for a site, named by a hash of key so that
// the file name says nothing about the site.
func Path(key string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(dir, "tui-q", hex.EncodeToString(sum[:6])+".json"), nil
}

// Open reads the settings at path. A missing or unreadable file gives
// empty settings; an empty path gives settings that are never saved.
func Open(path string) *Store {
	s := &Store{path: path}
	if path != "" {
		if data, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(data, s)
		}
	}
	if s.Columns == nil {
		s.Columns = map[string]Column{}
	}
	return s
}

// Column returns the settings of the column for a field.
func (s *Store) Column(field string) Column { return s.Columns[field] }

// SetColumn changes the settings of a column and saves the file.
func (s *Store) SetColumn(field string, c Column) error {
	if c == (Column{}) {
		delete(s.Columns, field)
	} else {
		s.Columns[field] = c
	}
	return s.save()
}

func (s *Store) save() error {
	if s.path == "" {
		return nil
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.path, append(data, '\n'), 0o644)
}
