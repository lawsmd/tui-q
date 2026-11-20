// Package attach keeps the files read from tickets, so that each is read
// from the platform once. They are stored per machine in the user's cache
// directory, and can be deleted at any time.
package attach

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/lawsmd/tui-q/ticket"
)

// MaxSize is the largest file the store keeps, in bytes.
const MaxSize = 64 << 20

// Store is the files of one site, by attachment ID.
type Store struct {
	dir string
}

// Dir is the directory for the files of a site, named by a hash of key so
// that the name says nothing about the site.
func Dir(key string) (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tui-q", "attachments", hash(key)), nil
}

func hash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:6])
}

// Open returns the files kept in dir. With an empty dir the store keeps
// nothing, and Save fails.
func Open(dir string) *Store {
	return &Store{dir: dir}
}

// path is where the file of an attachment is kept. The name is a hash of
// the ID, with the extension of a picture when the attachment is one and
// none otherwise: what the platform calls the file is never part of it.
func (s *Store) path(a ticket.Attachment) string {
	return filepath.Join(s.dir, hash(a.ID)+a.Ext())
}

// Path returns the file of an attachment when the store has it.
func (s *Store) Path(a ticket.Attachment) (string, bool) {
	if s.dir == "" {
		return "", false
	}
	p := s.path(a)
	if info, err := os.Stat(p); err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	return p, true
}

// Save writes the bytes of an attachment to its file and returns the path.
// A file larger than MaxSize is refused.
func (s *Store) Save(a ticket.Attachment, r io.Reader) (string, error) {
	if s.dir == "" {
		return "", errors.New("there is no directory to keep attachments in")
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return "", err
	}
	// Written under another name first, so that a read that stops halfway
	// leaves no file that looks whole.
	tmp, err := os.CreateTemp(s.dir, "part-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	n, err := io.Copy(tmp, io.LimitReader(r, MaxSize+1))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", fmt.Errorf("%s: %w", a.Name, err)
	}
	if n > MaxSize {
		return "", fmt.Errorf("%s is larger than %d MB", a.Name, MaxSize>>20)
	}
	p := s.path(a)
	if err := os.Rename(tmp.Name(), p); err != nil {
		return "", err
	}
	return p, nil
}
