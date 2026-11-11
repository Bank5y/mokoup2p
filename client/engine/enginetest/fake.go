// Package enginetest provides a fake engine that records what the daemon
// asked of it.
package enginetest

import (
	"context"
	"sync"
)

type Fake struct {
	mu           sync.Mutex
	cleanupCalls int
}

func (f *Fake) Cleanup(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleanupCalls++
	return nil
}

// CleanupCalls reports how many times Cleanup was called.
func (f *Fake) CleanupCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cleanupCalls
}
