// Package engine is the daemon's only door to the data plane (EasyTier).
package engine

import "context"

// Engine drives the virtual network. The production implementation will
// manage an easytier-core child process; tests use enginetest.Fake.
type Engine interface {
	// Cleanup removes leftovers (processes, virtual NICs, routes) from a
	// previous abnormal exit. It is called once when the daemon starts.
	Cleanup(ctx context.Context) error
}

// Noop is a placeholder until the EasyTier engine exists.
type Noop struct{}

func (Noop) Cleanup(context.Context) error { return nil }
