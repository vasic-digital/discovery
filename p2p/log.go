package p2p

import (
	"fmt"
	"io"
	"sync"
)

// NopLogger discards all log lines. It is the default when Config.Logger is nil.
type NopLogger struct{}

// Logf implements Logger and does nothing.
func (NopLogger) Logf(string, ...any) {}

// StdLogger is a concurrency-safe Logger that writes timestamped-free, prefixed
// lines to an io.Writer and (optionally) captures them for later inspection by
// tests producing evidence.
type StdLogger struct {
	mu     sync.Mutex
	w      io.Writer
	prefix string
	lines  []string
	keep   bool
}

// NewStdLogger returns a StdLogger writing to w with the given prefix. When
// keep is true, every line is also retained in memory and retrievable via
// Lines (used to attach discovery/pubsub logs to per-run evidence).
func NewStdLogger(w io.Writer, prefix string, keep bool) *StdLogger {
	return &StdLogger{w: w, prefix: prefix, keep: keep}
}

// Logf implements Logger.
func (l *StdLogger) Logf(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	full := l.prefix + line
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.w != nil {
		fmt.Fprintln(l.w, full)
	}
	if l.keep {
		l.lines = append(l.lines, full)
	}
}

// Lines returns a copy of the captured log lines (empty unless keep was set).
func (l *StdLogger) Lines() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]string, len(l.lines))
	copy(out, l.lines)
	return out
}
