package controller

import (
	"fmt"
	"io"
	"time"

	"github.com/aradar46/fermata/internal/engine"
)

// holdMode keeps a failed step's container alive without an interactive
// prompt, printing how to get into it and how to clean it up.
//
// This exists for the case an interactive REPL cannot serve: a run with no
// terminal attached, such as a CI job, a script, or a wrapper. There the
// prompt would be read as EOF, so hold the container, print how to reach it,
// and let the user attach on their own time.
//
// It is deliberately not a daemon or a network protocol. It hands off to
// `docker exec`, which anyone running containers already has.
type holdMode struct {
	out      io.Writer
	duration time.Duration
	events   *eventSink
}

// hold blocks for the configured duration (or until the process is killed),
// keeping the job container alive and reachable.
func (h *holdMode) hold(ev engine.StepEvent, reason string) {
	name := ""
	if ev.RC != nil {
		name = ev.RC.JobContainerName()
	}

	fmt.Fprintf(h.out, "\n⏸  fermata holding step %q — %s\n", stepDisplayName(ev), reason)

	if name != "" {
		fmt.Fprintf(h.out, "\n   container: %s\n", name)
		fmt.Fprintf(h.out, "   get a shell:  docker exec -it %s bash\n", name)
		fmt.Fprintf(h.out, "   clean up:     docker rm -f %s\n", name)
	} else {
		fmt.Fprintln(h.out, "\n   (no live container to attach to)")
	}

	fmt.Fprintf(h.out, "\n   holding for %s, then the job continues. Ctrl-C to stop now.\n\n",
		h.duration)

	if h.events != nil {
		h.events.emit(Event{
			Kind:      EventPaused,
			Step:      stepDisplayName(ev),
			Reason:    reason,
			Container: name,
			Detail:    fmt.Sprintf("held for %s", h.duration),
		})
	}

	time.Sleep(h.duration)

	fmt.Fprintf(h.out, "   hold elapsed; continuing\n")
	if h.events != nil {
		h.events.emit(Event{Kind: EventResumed, Step: stepDisplayName(ev), Container: name})
	}
}
