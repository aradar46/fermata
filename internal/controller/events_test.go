package controller

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEventSink_NilIsSafe(t *testing.T) {
	var s *eventSink
	// A nil sink must be usable so callers never branch on whether --json was
	// passed. This would panic if emit didn't guard.
	s.emit(Event{Kind: EventPaused})
}

func TestEventSink_WritesOneJSONObjectPerLine(t *testing.T) {
	var buf bytes.Buffer
	s := newEventSink(&buf)

	s.emit(Event{Kind: EventPaused, Step: "run tests", Reason: "step failed"})
	s.emit(Event{Kind: EventResumed, Step: "run tests"})

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 JSONL lines, got %d:\n%s", len(lines), buf.String())
	}

	var first Event
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("line 1 is not valid JSON: %v", err)
	}
	if first.Kind != EventPaused || first.Step != "run tests" {
		t.Errorf("unexpected event: %+v", first)
	}
	if first.Time.IsZero() {
		t.Error("emit should stamp a time so consumers can order events")
	}
}

func TestEventSink_OmitsEmptyFields(t *testing.T) {
	var buf bytes.Buffer
	newEventSink(&buf).emit(Event{Kind: EventResumed})

	// Consumers parse this; absent fields should not appear as empty strings.
	for _, field := range []string{"\"error\"", "\"container\"", "\"reason\"", "\"detail\""} {
		if strings.Contains(buf.String(), field) {
			t.Errorf("empty field %s should be omitted, got: %s", field, buf.String())
		}
	}
}

func TestHistory_AddSkipsBlanksAndRepeats(t *testing.T) {
	h := &history{}
	h.add("shell")
	h.add("shell") // immediate repeat
	h.add("")
	h.add("   ")
	h.add("retry")

	if got := h.recent(10); len(got) != 2 || got[0] != "shell" || got[1] != "retry" {
		t.Errorf("got %v, want [shell retry]", got)
	}
}

func TestHistory_IsBounded(t *testing.T) {
	h := &history{}
	for i := 0; i < historyLimit+50; i++ {
		h.add(string(rune('a'+i%26)) + string(rune('0'+i%10)) + "x")
	}
	if len(h.items) > historyLimit {
		t.Errorf("history grew to %d, limit is %d", len(h.items), historyLimit)
	}
}

func TestHistory_RoundTripsThroughDisk(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	h := loadHistory()
	h.add("shell")
	h.add("retry")
	h.save()

	// A fresh session should see the previous session's commands.
	reloaded := loadHistory()
	got := reloaded.recent(10)
	if len(got) != 2 || got[0] != "shell" || got[1] != "retry" {
		t.Errorf("history did not survive a reload, got %v", got)
	}

	if _, err := os.Stat(filepath.Join(dir, "fermata", "history")); err != nil {
		t.Errorf("history file not written: %v", err)
	}
}

// Failing to persist history must never interfere with debugging.
func TestHistory_SaveIsSilentWhenPathUnavailable(t *testing.T) {
	h := &history{path: ""}
	h.add("shell")
	h.save() // must not panic
}
