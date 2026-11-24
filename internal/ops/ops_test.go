package ops

import (
	"context"
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
)

type result string

func TestReadDoesNotLock(t *testing.T) {
	r := New()
	cmd := r.Start("load", Background, func(context.Context) (tea.Msg, error) { return result("ok"), nil })
	if !r.Busy() || r.Locked() {
		t.Fatalf("Busy=%v Locked=%v, want busy and unlocked", r.Busy(), r.Locked())
	}
	done := cmd().(DoneMsg)
	if done.Err != nil || done.Msg != result("ok") {
		t.Fatalf("done = %+v", done)
	}
	r.Finish(done)
	if r.Busy() {
		t.Fatal("still busy after Finish")
	}
	if _, failed := r.Failed(); failed {
		t.Fatal("a success was recorded as a failure")
	}
}

func TestWriteLocksAndFailureIsKept(t *testing.T) {
	r := New()
	r.Start("load", Background, func(context.Context) (tea.Msg, error) { return nil, nil })
	cmd := r.Start("move", Write, func(context.Context) (tea.Msg, error) { return nil, errors.New("refused") })
	if !r.Locked() {
		t.Fatal("a running write did not lock")
	}
	if cur, _ := r.Current(); cur.Label != "move" {
		t.Fatalf("Current = %q, want the write", cur.Label)
	}
	r.Finish(cmd().(DoneMsg))
	if r.Locked() {
		t.Fatal("still locked after the write ended")
	}
	failed, ok := r.Failed()
	if !ok || failed.Label != "move" || failed.Err == nil {
		t.Fatalf("Failed = %+v, %v", failed, ok)
	}
	r.Dismiss()
	if _, ok := r.Failed(); ok {
		t.Fatal("failure survived Dismiss")
	}
	if len(r.Log()) != 1 {
		t.Fatalf("log has %d entries, want 1", len(r.Log()))
	}
}

func TestStepsAreReportedWhileTheOperationRuns(t *testing.T) {
	r := New()
	cmd := r.StartSteps("copy", Write, 0, func(_ context.Context, report func(string)) (tea.Msg, error) {
		report("first")
		report("second")
		return nil, nil
	})
	if cur, _ := r.Current(); cur.Step() != "" {
		t.Fatalf("step before the run = %q", cur.Step())
	}
	done := cmd().(DoneMsg)
	if cur, _ := r.Current(); cur.Step() != "second" {
		t.Fatalf("step = %q, want the last one reported", cur.Step())
	}
	r.Finish(done)
}

func TestAReadTheUserWaitsOnLocks(t *testing.T) {
	r := New()
	r.Start("names", Background, func(context.Context) (tea.Msg, error) { return nil, nil })
	if r.Locked() {
		t.Fatal("a read in the background locked")
	}
	cmd := r.Start("open", Wait, func(context.Context) (tea.Msg, error) { return nil, nil })
	if !r.Locked() || r.Count() != 2 || r.Last() != 2 {
		t.Fatalf("Locked=%v Count=%d Last=%d, want a lock, 2 and 2", r.Locked(), r.Count(), r.Last())
	}
	if cur, _ := r.Current(); cur.Label != "open" || !cur.Locks() {
		t.Fatalf("Current = %+v, want the read that locks", cur)
	}
	r.Finish(cmd().(DoneMsg))
	if r.Locked() {
		t.Fatal("still locked after the read ended")
	}
}
