package controller

import (
	"testing"

	"github.com/aradar46/fermata/internal/engine"
	"github.com/nektos/act/pkg/model"
)

func evFor(id, name string, err error) engine.StepEvent {
	return engine.StepEvent{Step: &model.Step{ID: id, Name: name}, Err: err}
}

func TestStepMatches(t *testing.T) {
	tests := []struct {
		name  string
		ev    engine.StepEvent
		token string
		want  bool
	}{
		{"match by id", evFor("build", "Build it", nil), "build", true},
		{"match by name", evFor("0", "Build it", nil), "Build it", true},
		{"id takes precedence but name still matches its own token", evFor("0", "Build it", nil), "0", true},
		{"no match", evFor("0", "Build it", nil), "nope", false},
		{"nil step never matches", engine.StepEvent{Step: nil}, "anything", false},
		{"empty token does not match empty id", evFor("", "Named", nil), "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stepMatches(tt.ev, tt.token); got != tt.want {
				t.Errorf("stepMatches token=%q = %v, want %v", tt.token, got, tt.want)
			}
		})
	}
}

func TestShouldPause(t *testing.T) {
	errStep := evFor("2", "deploy", errBoom{})
	okStep := evFor("2", "deploy", nil)

	t.Run("break on failure pauses failing step", func(t *testing.T) {
		c := New(nil, true)
		if ok, _ := c.shouldPause(errStep); !ok {
			t.Error("expected pause on failing step with breakOnFailure")
		}
	})
	t.Run("break on failure disabled does not pause failing step", func(t *testing.T) {
		c := New(nil, false)
		if ok, _ := c.shouldPause(errStep); ok {
			t.Error("did not expect pause when breakOnFailure is off")
		}
	})
	t.Run("breakpoint pauses matching passing step", func(t *testing.T) {
		c := New([]string{"deploy"}, false)
		if ok, _ := c.shouldPause(okStep); !ok {
			t.Error("expected pause on breakpoint match")
		}
	})
	t.Run("no breakpoint, passing step, does not pause", func(t *testing.T) {
		c := New(nil, false)
		if ok, _ := c.shouldPause(okStep); ok {
			t.Error("did not expect pause")
		}
	})
	t.Run("blank break specs are ignored", func(t *testing.T) {
		c := New([]string{"  ", ""}, false)
		if len(c.breakpoints) != 0 {
			t.Errorf("expected blank specs ignored, got %d breakpoints", len(c.breakpoints))
		}
	})
}

type errBoom struct{}

func (errBoom) Error() string { return "boom" }
