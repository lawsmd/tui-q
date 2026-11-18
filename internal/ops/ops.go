// Package ops runs provider calls off the UI goroutine and tracks them for
// the status bar.
//
// A read leaves the interface usable while it runs. A write locks input
// until it finishes, which rules out a second action landing on a ticket
// whose state is about to change.
package ops

import (
	"context"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
)

// DefaultTimeout bounds every operation.
const DefaultTimeout = 30 * time.Second

const logSize = 50

// Func is the work of one operation. The message it returns is delivered
// to the UI when the operation succeeds.
type Func func(ctx context.Context) (tea.Msg, error)

// StepFunc is the work of an operation with several steps. It calls report
// with the name of each step as it starts, for the status bar.
type StepFunc func(ctx context.Context, report func(step string)) (tea.Msg, error)

// DoneMsg reports that an operation ended. The UI passes it to Finish.
type DoneMsg struct {
	ID  int
	Err error
	// Msg is the result to deliver. It is nil when Err is set.
	Msg tea.Msg
}

// Entry is one operation, running or finished.
type Entry struct {
	ID      int
	Label   string
	Write   bool
	Started time.Time
	Ended   time.Time
	Err     error

	// step is written by the operation's goroutine and read by the UI.
	step *atomic.Pointer[string]
}

// Step is the step a running operation last reported, or "".
func (e Entry) Step() string {
	if e.step == nil {
		return ""
	}
	if s := e.step.Load(); s != nil {
		return *s
	}
	return ""
}

// Runner tracks operations. It is owned by the UI model and touched only
// from Update.
type Runner struct {
	Timeout time.Duration

	next   int
	active []Entry
	log    []Entry
	err    *Entry
}

// New returns a Runner with the default timeout.
func New() *Runner { return &Runner{Timeout: DefaultTimeout} }

// Start records an operation and returns the command that runs it.
func (r *Runner) Start(label string, write bool, fn Func) tea.Cmd {
	return r.StartSteps(label, write, 0, func(ctx context.Context, _ func(string)) (tea.Msg, error) {
		return fn(ctx)
	})
}

// StartSteps is Start for an operation that reports its steps. A timeout
// of zero means the runner's own.
func (r *Runner) StartSteps(label string, write bool, timeout time.Duration, fn StepFunc) tea.Cmd {
	r.next++
	id := r.next
	step := &atomic.Pointer[string]{}
	r.active = append(r.active, Entry{ID: id, Label: label, Write: write, Started: time.Now(), step: step})
	if timeout <= 0 {
		timeout = r.Timeout
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		msg, err := fn(ctx, func(s string) { step.Store(&s) })
		if err != nil {
			return DoneMsg{ID: id, Err: err}
		}
		return DoneMsg{ID: id, Msg: msg}
	}
}

// Finish records the end of an operation and returns its entry. The entry
// is empty when the operation is not known.
func (r *Runner) Finish(d DoneMsg) Entry {
	for i, e := range r.active {
		if e.ID != d.ID {
			continue
		}
		r.active = append(r.active[:i], r.active[i+1:]...)
		e.Ended = time.Now()
		e.Err = d.Err
		r.log = append(r.log, e)
		if len(r.log) > logSize {
			r.log = r.log[len(r.log)-logSize:]
		}
		if d.Err != nil {
			failed := e
			r.err = &failed
		}
		return e
	}
	return Entry{}
}

// Busy reports whether any operation is running.
func (r *Runner) Busy() bool { return len(r.active) > 0 }

// Locked reports whether a write is running, which locks input.
func (r *Runner) Locked() bool {
	for _, e := range r.active {
		if e.Write {
			return true
		}
	}
	return false
}

// Current returns the operation to show in the status bar: the running
// write when there is one, and the oldest running read otherwise.
func (r *Runner) Current() (Entry, bool) {
	for _, e := range r.active {
		if e.Write {
			return e, true
		}
	}
	if len(r.active) > 0 {
		return r.active[0], true
	}
	return Entry{}, false
}

// Failed returns the most recent failure that has not been dismissed.
func (r *Runner) Failed() (Entry, bool) {
	if r.err == nil {
		return Entry{}, false
	}
	return *r.err, true
}

// Dismiss clears the failure shown in the status bar.
func (r *Runner) Dismiss() { r.err = nil }

// Log returns finished operations, oldest first.
func (r *Runner) Log() []Entry { return r.log }
