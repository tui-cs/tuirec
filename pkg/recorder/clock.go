package recorder

import (
	"sync"
	"time"
)

// Clock reports elapsed recording time.
type Clock interface {
	Elapsed() time.Duration
}

// WallClock measures elapsed time using the system clock.
type WallClock struct {
	start time.Time
}

// NewWallClock creates a wall-clock recorder clock starting now.
func NewWallClock() WallClock {
	return WallClock{start: time.Now()}
}

// Elapsed returns the duration since the wall clock was created.
func (c WallClock) Elapsed() time.Duration {
	return time.Since(c.start)
}

// ScriptedClock is a deterministic clock for reproducible tests and recordings.
type ScriptedClock struct {
	mu      sync.Mutex
	elapsed time.Duration
}

// NewScriptedClock creates a deterministic clock starting at zero.
func NewScriptedClock() *ScriptedClock {
	return &ScriptedClock{}
}

// Advance moves the scripted clock forward by d.
func (c *ScriptedClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.elapsed += d
}

// Elapsed returns the scripted elapsed time.
func (c *ScriptedClock) Elapsed() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.elapsed
}
