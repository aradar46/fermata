package controller

import (
	"io"
	"sync"
)

// gatedWriter wraps an underlying writer (act's log destination) so fermata can
// quiesce act's streaming output while the REPL owns the terminal (plan F17).
//
// While open, writes pass through immediately. While closed (paused), writes
// are buffered and flushed on the next Open, so nothing is lost; it just doesn't
// interleave with the prompt.
type gatedWriter struct {
	mu   sync.Mutex
	w    io.Writer
	open bool
	buf  []byte
	// tail keeps the most recent output so the REPL can diagnose a failure
	// from what the step actually printed (see diagnose).
	tail []byte
}

// tailLimit caps the retained output. A Gradle failure prints its diagnosis
// well within this.
const tailLimit = 16 << 10

func newGatedWriter(w io.Writer) *gatedWriter {
	return &gatedWriter{w: w, open: true}
}

func (g *gatedWriter) Write(p []byte) (int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.tail = append(g.tail, p...)
	if len(g.tail) > tailLimit {
		g.tail = g.tail[len(g.tail)-tailLimit:]
	}

	if g.open {
		return g.w.Write(p)
	}
	// Paused: buffer for later. Report full length so callers don't error.
	g.buf = append(g.buf, p...)
	return len(p), nil
}

// Close quiesces output: subsequent writes are buffered until Open.
func (g *gatedWriter) Close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.open = false
}

// Tail returns the most recent output act produced.
func (g *gatedWriter) Tail() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return string(g.tail)
}

// Open resumes output, flushing anything buffered while closed.
func (g *gatedWriter) Open() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.buf) > 0 {
		_, _ = g.w.Write(g.buf)
		g.buf = nil
	}
	g.open = true
}
