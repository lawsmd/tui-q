// Package session keeps the tickets a running interface has open in tabs
// where another process of the same user can read them. "tq tabs" prints
// them for an LLM agent. Each running interface has one file, in the user's
// cache directory, which it removes when it exits.
package session

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"syscall"
)

// Link is a ticket that an open ticket is linked to.
type Link struct {
	Relation string `json:"relation"`
	Key      string `json:"key"`
	Status   string `json:"status,omitempty"`
}

// Tab is a ticket open in a tab.
type Tab struct {
	Key    string `json:"key"`
	Status string `json:"status,omitempty"`
	Title  string `json:"title,omitempty"`
	// Shown is true for the ticket on screen.
	Shown bool   `json:"shown,omitempty"`
	Links []Link `json:"links,omitempty"`
}

// State is the tabs of one running interface.
type State struct {
	PID  int   `json:"pid"`
	Tabs []Tab `json:"tabs"`
}

// Dir is the directory for the files of the interfaces running on a site,
// named by a hash of key so that the name says nothing about the site.
func Dir(key string) (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(dir, "tui-q", "run", hex.EncodeToString(sum[:6])), nil
}

// Writer keeps the file of this process.
type Writer struct {
	mu     sync.Mutex
	path   string
	last   int
	closed bool
}

// Open returns the writer of this process's file in dir. Nothing is
// written until the first Put.
func Open(dir string) *Writer {
	return &Writer{path: filepath.Join(dir, strconv.Itoa(os.Getpid())+".json")}
}

// Put writes the tabs as they are at change number n. Puts may arrive out
// of order, since each is made on a goroutine of its own: one numbered
// lower than the last written is dropped. The file is replaced whole, so a
// reader sees the old tabs or the new ones.
func (w *Writer) Put(n int, tabs []Tab) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed || n <= w.last {
		return nil
	}
	if tabs == nil {
		tabs = []Tab{}
	}
	data, err := json.MarshalIndent(State{PID: os.Getpid(), Tabs: tabs}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(w.path), 0o700); err != nil {
		return err
	}
	part := w.path + ".part"
	if err := os.WriteFile(part, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(part, w.path); err != nil {
		os.Remove(part)
		return err
	}
	w.last = n
	return nil
}

// Close removes the file. A Put that arrives later writes nothing.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	if err := os.Remove(w.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// List returns the tabs of every interface running on the site whose
// directory is dir, in order of process ID. A file left by a process that
// has ended without removing it is deleted and left out.
func List(dir string) ([]State, error) {
	names, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	var out []State
	for _, name := range names {
		data, err := os.ReadFile(name)
		if err != nil {
			continue
		}
		var s State
		if json.Unmarshal(data, &s) != nil || s.PID <= 0 {
			continue
		}
		if !alive(s.PID) {
			os.Remove(name)
			continue
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PID < out[j].PID })
	return out, nil
}

// alive reports whether a process with the given ID is running.
func alive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	defer p.Release()
	if runtime.GOOS == "windows" {
		// Finding a process opens it here, which fails when there is none.
		return true
	}
	// Elsewhere finding always succeeds, and signal 0 tests for the
	// process without sending it anything.
	return p.Signal(syscall.Signal(0)) == nil
}
