// Package controller decides when to pause a running workflow and drives the
// interactive REPL at a paused step. The REPL blocks act's synchronous
// pipeline (holding the job container open) until the user resumes; act's log
// streaming is quiesced while the prompt owns the terminal (plan F17). The
// shell and retry commands are stubbed here and land in later milestones.
package controller

import (
	"context"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aradar46/fermata/internal/engine"
)

// Breakpoint identifies where to pause. A bare step index is intentionally not
// supported: indices shift when the user edits the YAML mid-session (plan F7).
// Match is by step id first, then step name.
type Breakpoint struct {
	Step string
}

// Controller holds debug configuration and the runtime handles needed to pause
// and abort a run.
type Controller struct {
	breakpoints    []Breakpoint
	breakOnFailure bool

	gate   *gatedWriter       // act's log output, quiesced during the REPL
	cancel context.CancelFunc // aborts the run on `quit`

	// workflowFile is re-read on `retry` so the user's edits are picked up.
	workflowFile string
	// bound records whether the workdir is bind-mounted. When it is not, the
	// container holds a copy taken at checkout, so source edits made while
	// paused never reach a retried step, which is worth warning about rather than
	// letting retry silently re-run stale code.
	bound bool
	// workdir is the directory whose files the container copied.
	workdir string
	// events, when set, receives a machine-readable record of what happened.
	events *eventSink
	// holdFor, when non-zero, keeps a failed step's container alive for this
	// long instead of opening an interactive prompt. For runs with no terminal.
	holdFor time.Duration
}

// SetHold switches the pause from an interactive prompt to a timed hold: the
// container stays alive and reachable via docker exec, then the job continues.
// This is what makes fermata usable where there is no TTY.
func (c *Controller) SetHold(d time.Duration) { c.holdFor = d }

// SetEventSink routes machine-readable events (JSONL) to w. Pass nil to
// disable. This is what makes fermata wrappable by other tools.
func (c *Controller) SetEventSink(w io.Writer) {
	c.events = newEventSink(w)
}

// SetWorkdirMode records how the working directory reached the container, so
// retry can warn when host edits cannot possibly be visible to it.
func (c *Controller) SetWorkdirMode(workdir string, bound bool) {
	c.workdir = workdir
	c.bound = bound
}

// SetWorkflowFile tells the controller which file to re-read on retry.
func (c *Controller) SetWorkflowFile(path string) { c.workflowFile = path }

// New builds a Controller. breakSpecs are raw --break values (step id or name).
func New(breakSpecs []string, breakOnFailure bool) *Controller {
	bps := make([]Breakpoint, 0, len(breakSpecs))
	for _, s := range breakSpecs {
		if t := strings.TrimSpace(s); t != "" {
			bps = append(bps, Breakpoint{Step: t})
		}
	}
	return &Controller{breakpoints: bps, breakOnFailure: breakOnFailure}
}

// LogWriter wraps w (act's log destination) in a gate the REPL can close while
// it owns the terminal, and returns it. The engine passes the result to act's
// job-logger factory.
func (c *Controller) LogWriter(w interface{ Write([]byte) (int, error) }) *gatedWriter {
	c.gate = newGatedWriter(w)
	return c.gate
}

// SetCancel gives the controller the run's cancel function so `quit` can abort.
func (c *Controller) SetCancel(cancel context.CancelFunc) {
	c.cancel = cancel
}

// OnStep is the engine step-boundary callback. It decides whether to pause and,
// if so, runs the REPL.
func (c *Controller) OnStep(ev engine.StepEvent) {
	shouldPause, reason := c.shouldPause(ev)
	if !shouldPause {
		return
	}

	// --hold replaces the interactive prompt with a timed hold: keep the
	// container alive and reachable, then continue. This is the shape that
	// works where no human is attached (a CI job or a wrapper script), and
	// where a prompt would simply be read as EOF.
	if c.holdFor > 0 {
		(&holdMode{out: os.Stderr, duration: c.holdFor, events: c.events}).hold(ev, reason)
		return
	}

	action := newREPL(c).run(ev, reason)
	if action == actionQuit && c.cancel != nil {
		c.cancel()
	}
}

func (c *Controller) shouldPause(ev engine.StepEvent) (bool, string) {
	if ev.Err != nil && c.breakOnFailure {
		return true, "step failed: " + ev.Err.Error()
	}
	for _, bp := range c.breakpoints {
		if stepMatches(ev, bp.Step) {
			return true, "breakpoint " + strconv.Quote(bp.Step) + " hit"
		}
	}
	return false, ""
}

// stepMatches reports whether the executed step matches the breakpoint token,
// by id first then name (plan F7 ordering).
func stepMatches(ev engine.StepEvent, token string) bool {
	if ev.Step == nil {
		return false
	}
	if ev.Step.ID != "" && ev.Step.ID == token {
		return true
	}
	if ev.Step.Name != "" && ev.Step.Name == token {
		return true
	}
	return false
}
