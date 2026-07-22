package controller

import (
	"encoding/json"
	"io"
	"sync"
	"time"
)

// Event is one machine-readable record of what fermata did. The stream exists
// so fermata can be wrapped: a CI job, a Slack bot, or an editor plugin can
// consume it without parsing human-facing output, which is free to change.
//
// One JSON object per line (JSONL), written to a caller-chosen sink.
type Event struct {
	Time  time.Time `json:"time"`
	Kind  string    `json:"kind"`
	Step  string    `json:"step,omitempty"`
	Job   string    `json:"job,omitempty"`
	Error string    `json:"error,omitempty"`
	// Container is the live job container's name, so a consumer can reach it
	// with plain docker commands.
	Container string `json:"container,omitempty"`
	// Reason explains a pause (step failure, breakpoint hit).
	Reason string `json:"reason,omitempty"`
	// Detail carries command-specific extras (e.g. retry outcome).
	Detail string `json:"detail,omitempty"`
}

// Event kinds. Kept stable: consumers match on these strings.
const (
	EventPaused   = "paused"
	EventResumed  = "resumed"
	EventRetried  = "retried"
	EventSkipped  = "skipped"
	EventQuit     = "quit"
	EventShellIn  = "shell_opened"
	EventShellOut = "shell_closed"
)

// eventSink writes events as JSONL. A nil sink is valid and does nothing, so
// callers never need to check whether --json was requested.
type eventSink struct {
	mu  sync.Mutex
	w   io.Writer
	enc *json.Encoder
}

func newEventSink(w io.Writer) *eventSink {
	if w == nil {
		return nil
	}
	return &eventSink{w: w, enc: json.NewEncoder(w)}
}

// emit writes one event. Failures are ignored: a broken event consumer must
// never take down a debugging session.
func (s *eventSink) emit(e Event) {
	if s == nil {
		return
	}
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.enc.Encode(e)
}
