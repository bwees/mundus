// Package pending reports an optimistic value after a command has been issued,
// so a caller sees the intended result before the device has caught up, without
// the intent outliving the evidence.
package pending

import (
	"sync"
	"time"
)

// Value holds the result a just-issued command is expected to produce. Resolve
// reports it in place of the observed value until the device leaves the value
// the command was issued from, or the window elapses. The zero value of T is
// the assumed starting point until the first Resolve.
type Value[T comparable] struct {
	window time.Duration

	mu     sync.Mutex
	last   T
	want   T
	from   T
	at     time.Time
	active bool
}

func New[T comparable](window time.Duration) *Value[T] {
	return &Value[T]{window: window}
}

// Expect records the value a command should produce, measured against whatever
// Resolve last observed.
func (v *Value[T]) Expect(want T) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.want, v.from, v.at, v.active = want, v.last, time.Now(), true
}

// Resolve takes a freshly observed value and returns the value to report.
func (v *Value[T]) Resolve(actual T) T {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.last = actual
	if !v.active {
		return actual
	}
	if actual != v.from || time.Since(v.at) > v.window {
		v.active = false
		return actual
	}
	return v.want
}
