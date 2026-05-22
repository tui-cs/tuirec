package pointer

import (
	"io"
	"sync"
)

// SyncWriter wraps an io.Writer with a mutex for concurrent access.
type SyncWriter struct {
	mu     sync.Mutex
	writer io.Writer
}

// NewSyncWriter creates a synchronized writer.
func NewSyncWriter(w io.Writer) *SyncWriter {
	return &SyncWriter{writer: w}
}

// Write implements io.Writer with mutex protection.
func (sw *SyncWriter) Write(p []byte) (int, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.writer.Write(p)
}

// WriteString writes a string under the mutex.
func (sw *SyncWriter) WriteString(s string) (int, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return io.WriteString(sw.writer, s)
}
