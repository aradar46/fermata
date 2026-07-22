package controller

import (
	"io"
	"sync"
	"time"
)

// stdinMux is the single owner of the process's stdin while a step is paused.
//
// Two consumers want stdin at a paused step: the REPL prompt (line at a time)
// and the in-container shell (raw byte stream). Letting both read stdin
// directly makes them steal each other's keystrokes, because the REPL's buffered
// reader swallows bytes the shell needs, and orphaned readers race the live
// prompt so typed input appears to be ignored. So exactly one goroutine reads
// stdin here and routes what it reads to whoever currently holds it.
type stdinMux struct {
	src io.Reader

	mu     sync.Mutex
	borrow *io.PipeWriter // non-nil while a borrower (the shell) holds stdin

	lines chan string // complete lines, delivered when nobody has borrowed
	done  chan struct{}
	once  sync.Once
}

func newStdinMux(src io.Reader) *stdinMux {
	m := &stdinMux{
		src:   src,
		lines: make(chan string),
		done:  make(chan struct{}),
	}
	go m.pump()
	return m
}

// pump is the only reader of the underlying stdin.
func (m *stdinMux) pump() {
	buf := make([]byte, 4096)
	var line []byte

	for {
		n, err := m.src.Read(buf)
		if n > 0 {
			chunk := buf[:n]

			m.mu.Lock()
			w := m.borrow
			m.mu.Unlock()

			if w != nil {
				// A borrower (the shell) owns stdin: forward raw bytes.
				if _, werr := w.Write(chunk); werr != nil {
					// Borrower went away; fall through to line mode next read.
					m.mu.Lock()
					if m.borrow == w {
						m.borrow = nil
					}
					m.mu.Unlock()
				}
				continue
			}

			// Line mode: accumulate and emit complete lines to the REPL.
			for i, b := range chunk {
				line = append(line, b)
				if b != '\n' {
					continue
				}
				switch m.emit(string(line)) {
				case emitStopped:
					return
				case emitBorrowed:
					// A borrower claimed stdin while we were offering this
					// line (e.g. the REPL ran `shell`). Everything from this
					// line onward belongs to them, not the prompt. Otherwise
					// those bytes are stranded and both sides deadlock.
					pending := append([]byte(nil), line...)
					pending = append(pending, chunk[i+1:]...)
					m.forward(pending)
					line = nil
					goto nextRead
				}
				line = nil
			}
		nextRead:
		}
		if err != nil {
			// Close any borrowed pipe first: a shell reading from it would
			// otherwise wait forever for input that can never arrive, hanging
			// the whole run when stdin is a pipe that has reached EOF.
			m.mu.Lock()
			w := m.borrow
			m.borrow = nil
			m.mu.Unlock()
			if w != nil {
				_ = w.CloseWithError(err)
				// The borrower owns the session; don't also push EOF at the
				// REPL, which isn't reading right now.
				return
			}

			// Flush any partial line, then signal EOF to the REPL.
			if len(line) > 0 {
				m.emit(string(line))
			}
			m.emit("")
			return
		}
	}
}

type emitResult int

const (
	emitDelivered emitResult = iota // the REPL took the line
	emitStopped                     // the mux was closed; stop pumping
	emitBorrowed                    // someone borrowed stdin; the line is theirs
)

// emit offers a line to the REPL. It gives up if the mux is closed, or if a
// borrower (the shell) claims stdin while we are waiting. The REPL is not
// reading in that case, so holding the line here would deadlock the session.
func (m *stdinMux) emit(s string) emitResult {
	for {
		select {
		case m.lines <- s:
			return emitDelivered
		case <-m.done:
			return emitStopped
		case <-time.After(20 * time.Millisecond):
			m.mu.Lock()
			borrowed := m.borrow != nil
			m.mu.Unlock()
			if borrowed {
				return emitBorrowed
			}
		}
	}
}

// forward hands raw bytes to the current borrower, if any.
func (m *stdinMux) forward(b []byte) {
	m.mu.Lock()
	w := m.borrow
	m.mu.Unlock()
	if w == nil || len(b) == 0 {
		return
	}
	if _, err := w.Write(b); err != nil {
		m.mu.Lock()
		if m.borrow == w {
			m.borrow = nil
		}
		m.mu.Unlock()
	}
}

// Lines returns the channel of complete input lines for the REPL prompt.
func (m *stdinMux) Lines() <-chan string { return m.lines }

// borrowReader hands raw stdin to a single consumer (the in-container shell)
// until release is called. Bytes read while borrowed never reach Lines().
func (m *stdinMux) borrowStdin() io.Reader {
	pr, pw := io.Pipe()
	m.mu.Lock()
	m.borrow = pw
	m.mu.Unlock()
	return pr
}

// release returns stdin to line mode.
func (m *stdinMux) release() {
	m.mu.Lock()
	w := m.borrow
	m.borrow = nil
	m.mu.Unlock()
	if w != nil {
		_ = w.Close()
	}
}

// Close stops the mux. The pump goroutine may remain blocked on a stdin read
// until the next keystroke, which is unavoidable without closing stdin itself;
// it exits as soon as that read returns.
func (m *stdinMux) Close() {
	m.once.Do(func() { close(m.done) })
	m.release()
}
