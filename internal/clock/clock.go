// Package clock abstracts time so time-dependent logic can be tested.
package clock

import (
	"slices"
	"sync"
	"time"
)

type Clock interface {
	Now() time.Time
	// AfterFunc calls f in its own goroutine once d has passed.
	AfterFunc(d time.Duration, f func()) Timer
}

// Timer is a pending AfterFunc call.
type Timer interface {
	// Stop prevents the call if it has not happened yet; it reports whether
	// it did so.
	Stop() bool
}

// Real is the wall clock.
type Real struct{}

func (Real) Now() time.Time { return time.Now() }

func (Real) AfterFunc(d time.Duration, f func()) Timer { return time.AfterFunc(d, f) }

// Fake is a manually advanced clock for tests.
type Fake struct {
	mu      sync.Mutex
	now     time.Time
	pending []*fakeTimer
}

func NewFake(start time.Time) *Fake { return &Fake{now: start} }

func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Advance moves the clock forward. AfterFunc calls that have become due run
// before Advance returns, in the order they were due, so their effects are
// visible to the test right away.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	f.now = f.now.Add(d)
	var due []*fakeTimer
	f.pending = slices.DeleteFunc(f.pending, func(t *fakeTimer) bool {
		if t.at.After(f.now) {
			return false
		}
		due = append(due, t)
		return true
	})
	f.mu.Unlock()
	slices.SortStableFunc(due, func(a, b *fakeTimer) int { return a.at.Compare(b.at) })
	for _, t := range due {
		t.f()
	}
}

func (f *Fake) AfterFunc(d time.Duration, fn func()) Timer {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := &fakeTimer{clock: f, at: f.now.Add(d), f: fn}
	f.pending = append(f.pending, t)
	return t
}

type fakeTimer struct {
	clock *Fake
	at    time.Time
	f     func()
}

func (t *fakeTimer) Stop() bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	n := len(t.clock.pending)
	t.clock.pending = slices.DeleteFunc(t.clock.pending, func(p *fakeTimer) bool { return p == t })
	return len(t.clock.pending) != n
}
