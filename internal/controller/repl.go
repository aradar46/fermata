package controller

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/aradar46/fermata/internal/engine"
	"golang.org/x/term"
)

// replAction is what the REPL decided when it returned control to the pipeline.
type replAction int

const (
	actionContinue replAction = iota // resume the job
	actionQuit                       // abort the whole run
)

// repl drives the interactive prompt at a paused step. It blocks act's
// synchronous pipeline (holding the container open) until the user resumes.
type repl struct {
	stdin *stdinMux // single owner of stdin: REPL prompt and shell share it
	out   io.Writer
	gate  *gatedWriter // act's log output, quiesced while we own the terminal
	// workflowFile is re-read on retry so the user's edits take effect.
	workflowFile string
	// bound and workdir let retry warn when the container holds a copy of the
	// repo rather than a bind mount, so host edits cannot reach it.
	bound   bool
	workdir string
	// pausedAt is when this pause began; files modified after it are edits the
	// user made while paused.
	pausedAt time.Time
	// hist persists commands across sessions.
	hist *history
	// events, when set, records what happened in machine-readable form.
	events *eventSink
}

func newREPL(c *Controller) *repl {
	return &repl{
		stdin:        newStdinMux(os.Stdin),
		out:          os.Stderr,
		gate:         c.gate,
		workflowFile: c.workflowFile,
		bound:        c.bound,
		workdir:      c.workdir,
		pausedAt:     time.Now(),
		hist:         loadHistory(),
		events:       c.events,
	}
}

// run takes over the terminal at a paused step and loops until continue/quit.
// It returns the chosen action. It also installs a SIGINT handler so that
// Ctrl-C while paused asks for confirmation instead of killing the session and
// losing all state (plan F11/F17).
func (r *repl) run(ev engine.StepEvent, reason string) replAction {
	// Quiesce act's streaming output for as long as the REPL is up.
	if r.gate != nil {
		r.gate.Close()
		defer r.gate.Open()
	}

	// While paused, intercept SIGINT ourselves: a stray Ctrl-C must not tear
	// down the still-alive container. We turn it into a "really quit?" prompt.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	// Exactly one goroutine reads stdin for the whole paused session (see
	// stdinMux). Spawning a reader per prompt leaked goroutines that stayed
	// blocked on stdin and then raced the live prompt (and the quit
	// confirmation) for the user's keystrokes, which is why Ctrl-C appeared
	// not to work.
	defer r.stdin.Close()
	lineCh := r.stdin.Lines()

	r.emit(ev, Event{Kind: EventPaused, Reason: reason})
	r.banner(ev, reason)

	for {
		fmt.Fprint(r.out, "fermata> ")

		var line string
		select {
		case <-sigCh:
			fmt.Fprint(r.out, "\n")
			if r.confirmVia(lineCh, sigCh, "Quit? The job container and all session state will be lost.") {
				return actionQuit
			}
			continue
		case l, ok := <-lineCh:
			if !ok {
				return actionContinue
			}
			line = l
		}

		// The mux emits "" (no newline) only at EOF, e.g. piped or closed
		// stdin. Resume rather than spin on an empty prompt forever.
		if line == "" {
			fmt.Fprint(r.out, "\n")
			return actionContinue
		}

		cmd := strings.TrimSpace(line)
		if cmd != "" {
			r.hist.add(cmd)
			r.hist.save()
		}

		switch cmd {
		case "help", "h", "?":
			r.help()
		case "history":
			r.printHistory()
		case "":
			// bare Enter: just reprompt
		case "continue", "c":
			r.emit(ev, Event{Kind: EventResumed})
			return actionContinue
		case "quit", "q":
			r.emit(ev, Event{Kind: EventQuit})
			return actionQuit
		case "env":
			r.printEnv(ev)
		case "shell", "sh":
			r.openShell(ev)
		case "retry", "r":
			r.retryStep(ev)
		case "skip", "s":
			if r.skipStep(ev) {
				return actionContinue
			}
		default:
			fmt.Fprintf(r.out, "  unknown command %q — type 'help'\n", cmd)
			// Typing a shell command at the fermata prompt is a natural
			// mistake; point at the command that does what they meant.
			if looksLikeShellCommand(cmd) {
				fmt.Fprintln(r.out, "  (to run commands in the container, type `shell` first)")
			}
		}
	}
}

// emit records an event, filling in the step/job/container context so
// consumers get a complete record without reconstructing it.
func (r *repl) emit(ev engine.StepEvent, e Event) {
	if r.events == nil {
		return
	}
	e.Step = stepDisplayName(ev)
	if ev.RC != nil {
		if ev.RC.Run != nil {
			e.Job = ev.RC.Run.JobID
		}
		e.Container = ev.RC.JobContainerName()
	}
	if e.Error == "" && ev.Err != nil && e.Kind == EventPaused {
		e.Error = ev.Err.Error()
	}
	r.events.emit(e)
}

func (r *repl) banner(ev engine.StepEvent, reason string) {
	name := stepDisplayName(ev)
	fmt.Fprintf(r.out, "\n⏸  fermata paused at step %q — %s\n", name, reason)

	if ev.Err != nil && r.gate != nil {
		tail := r.gate.Tail()

		// Show what the step actually printed. By the time we pause, the real
		// error has usually scrolled past the setup noise, and "exitcode 1"
		// alone tells the user nothing they can act on.
		if excerpt := failureExcerpt(tail); excerpt != "" {
			fmt.Fprintf(r.out, "\n%s\n", excerpt)
		}

		// If the failure is a known local-vs-GitHub gap rather than a bug in
		// the user's workflow, say so. That is the difference between a
		// useful pause and a confusing one.
		if d, ok := diagnose(tail); ok {
			fmt.Fprintf(r.out, "\n   likely cause: %s\n", d.Cause)
			fmt.Fprintf(r.out, "   %s\n", d.Hint)
		}
	}

	// Name the container so the user is never trapped: if fermata dies, they
	// can still `docker exec` into it and `docker rm -f` it themselves.
	if ev.RC != nil {
		if name := ev.RC.JobContainerName(); name != "" {
			fmt.Fprintf(r.out, "\n   container: %s\n", name)
		}
	}

	fmt.Fprintf(r.out, "\n   commands: continue  quit  env  shell  retry  skip  (help)\n\n")
}

func (r *repl) help() {
	fmt.Fprint(r.out, `  continue (c)  resume the job from here
  quit (q)      abort the whole run (container is torn down)
  env           print this step's environment (secrets masked)
  shell (sh)    open a shell in the job container
  retry (r)     re-run just this step after you fix it
  skip (s)      mark this step skipped and resume the job
  history       recent commands, including from previous sessions
`)
}

// openShell drops the user into an interactive shell inside the paused job
// container, at the step's working directory with the step's env. Output
// cannot mask secrets the user chooses to print (documented, not pretended
// away, per plan F16).
func (r *repl) openShell(ev engine.StepEvent) {
	if ev.RC == nil || ev.RC.JobContainer == nil {
		fmt.Fprintln(r.out, "  shell: no live container at this step")
		return
	}

	r.emit(ev, Event{Kind: EventShellIn})
	fmt.Fprintln(r.out, "  opening a shell in the job container (exit to return to fermata)")
	fmt.Fprintln(r.out, "  note: secrets are NOT masked inside the shell")

	env := ev.RC.GetEnv()

	// Prefer bash, fall back to sh via a tiny bootstrap so it works on images
	// without bash. The shell runs interactively with stdin/TTY attached.
	// Interactive (-i) is what makes bash print a prompt and echo input; without
	// it the session looks dead. But `bash -i` fed from a pipe never sees EOF
	// and hangs forever, so only ask for it when we really have a terminal.
	// stderr is deliberately not redirected, so errors from user commands show.
	interactive := term.IsTerminal(int(os.Stdin.Fd()))
	bootstrap := "command -v bash >/dev/null && exec bash || exec sh"
	if interactive {
		bootstrap = "command -v bash >/dev/null && exec bash -i || exec sh -i"
	}
	shellCmd := []string{"sh", "-c", bootstrap}

	// Put the local terminal in raw mode so keystrokes reach the container
	// shell directly (arrows, tab, in-shell Ctrl-C).
	var restore func()
	if interactive {
		if oldState, err := term.MakeRaw(int(os.Stdin.Fd())); err == nil {
			restore = func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }
		}
	}

	// Hand stdin to the shell for the duration. r.stdin is the single owner of
	// the real stdin for the whole session; borrowing it here (rather than
	// reading os.Stdin separately) is what stops the REPL's reader and the
	// shell from stealing each other's bytes.
	shellIn := r.stdin.borrowStdin()

	// Fresh context: the shell is a side-session, independent of any run
	// cancellation, so a container-side Ctrl-C ends the shell, not the job.
	exec := ev.RC.JobContainer.ExecInteractive(shellCmd, env, "", "", shellIn, os.Stdout)
	err := exec(context.Background())

	r.stdin.release()
	if restore != nil {
		restore()
	}
	if err != nil {
		fmt.Fprintf(r.out, "\r\n  shell exited: %v\r\n", err)
	} else {
		fmt.Fprintln(r.out, "\r\n  shell closed")
	}
	r.emit(ev, Event{Kind: EventShellOut})
}

// retryStep re-reads the workflow from disk (picking up the user's fix),
// re-matches the paused step, and re-executes just that step against the live
// container: no earlier step is re-run.
//
// The retry runs against whatever state the failed step left behind; act does
// not roll anything back. We say so rather than pretending otherwise (plan F2).
func (r *repl) retryStep(ev engine.StepEvent) {
	if ev.RC == nil {
		fmt.Fprintln(r.out, "  retry: no run context at this step")
		return
	}
	if r.workflowFile == "" {
		fmt.Fprintln(r.out, "  retry: workflow file unknown")
		return
	}

	stepID, stepName := "", ""
	if ev.Step != nil {
		stepID, stepName = ev.Step.ID, ev.Step.Name
	}
	jobID := ev.RC.Run.JobID

	reloaded, err := engine.ReloadStep(r.workflowFile, jobID, stepID, stepName)
	if err != nil {
		fmt.Fprintf(r.out, "  retry: %v\n", err)
		return
	}

	// Without a bind mount the container has a copy of the repo from checkout.
	// If the user edited files while paused, those edits cannot reach the
	// retried step: warn instead of re-running stale code and failing with the
	// same error, which looks like fermata is broken.
	if !r.bound {
		if edited, ok := editedSince(r.workdir, r.pausedAt); ok {
			fmt.Fprintf(r.out, "\n  ⚠  %s changed on your machine, but this container has a copy\n",
				filepath.Base(edited))
			fmt.Fprintln(r.out, "     of the repo from checkout — the retry will run the OLD code.")
			fmt.Fprintln(r.out, "     Re-run fermata with --bind so edits are live in the container.")
			fmt.Fprintln(r.out, "     (workflow-file edits are always picked up; source edits are not)")
			fmt.Fprint(r.out, "\n")
		}
	}

	fmt.Fprintf(r.out, "  re-running step %q (container state is post-failure; nothing was rolled back)\n",
		stepDisplayName(ev))

	// Run in the step's own context so the retry uses act's job logger (native
	// formatting) and can clear the job error when it succeeds. Let act's
	// output stream again for the duration, as in a normal run.
	retryCtx := ev.Ctx
	if retryCtx == nil {
		retryCtx = context.Background()
	}
	if r.gate != nil {
		r.gate.Open()
	}
	retryErr := ev.RC.RunSingleStep(retryCtx, reloaded)
	if r.gate != nil {
		r.gate.Close()
	}

	if retryErr != nil {
		r.emit(ev, Event{Kind: EventRetried, Detail: "failed", Error: retryErr.Error()})
		fmt.Fprintf(r.out, "\n  retry failed: %v\n", retryErr)
		fmt.Fprintln(r.out, "  fix the step and `retry` again, or `continue` / `quit`")
		return
	}
	r.emit(ev, Event{Kind: EventRetried, Detail: "succeeded"})
	fmt.Fprintln(r.out, "\n  retry succeeded — `continue` to resume the rest of the job")
}

// printHistory lists recent commands from previous sessions too, so a paused
// step behaves like a shell rather than a fresh prompt each time.
func (r *repl) printHistory() {
	items := r.hist.recent(20)
	if len(items) == 0 {
		fmt.Fprintln(r.out, "  (no history yet)")
		return
	}
	for i, item := range items {
		fmt.Fprintf(r.out, "  %2d  %s\n", i+1, item)
	}
}

// skipStep marks the failed step as skipped and resumes the job. It reports
// whether the job should continue.
//
// Skipping is for steps that cannot work locally at all (an OIDC login, an
// upload to a release service), where the alternative is abandoning the run.
func (r *repl) skipStep(ev engine.StepEvent) bool {
	if ev.RC == nil {
		fmt.Fprintln(r.out, "  skip: no run context at this step")
		return false
	}

	ctx := ev.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	ev.RC.SkipCurrentStep(ctx, ev.Step)

	r.emit(ev, Event{Kind: EventSkipped})
	fmt.Fprintf(r.out, "  skipped %q — its failure will not fail the job\n", stepDisplayName(ev))
	if ev.Err != nil {
		fmt.Fprintln(r.out, "  note: later steps that depend on it may still fail")
	}
	return true
}

// printEnv prints the paused step's environment with secret values masked.
func (r *repl) printEnv(ev engine.StepEvent) {
	if ev.RC == nil {
		fmt.Fprintln(r.out, "  (no run context available)")
		return
	}
	env := ev.RC.GetEnv()
	masker := newMasker(ev.RC.Config.Secrets, ev.RC.Masks)

	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(r.out, "  %s=%s\n", k, masker.mask(env[k]))
	}
}

// confirmVia asks a yes/no question, defaulting to no. It consumes the shared
// stdin channel rather than reading stdin itself: a second concurrent reader
// would steal the answer from the prompt (and vice versa). A second Ctrl-C at
// the confirmation is taken as "yes, really quit".
func (r *repl) confirmVia(lineCh <-chan string, sigCh <-chan os.Signal, q string) bool {
	fmt.Fprintf(r.out, "%s [y/N] ", q)
	select {
	case <-sigCh:
		// Pressing Ctrl-C again means they mean it.
		fmt.Fprint(r.out, "\n")
		return true
	case line, ok := <-lineCh:
		if !ok {
			return false
		}
		line = strings.ToLower(strings.TrimSpace(line))
		return line == "y" || line == "yes"
	}
}

// looksLikeShellCommand reports whether input at the fermata prompt was
// probably meant for the container shell (e.g. "echo $ANDROID_HOME", "ls -la").
func looksLikeShellCommand(s string) bool {
	if strings.ContainsAny(s, "$|/&><") {
		return true
	}
	first, _, _ := strings.Cut(s, " ")
	switch first {
	case "echo", "ls", "cat", "cd", "pwd", "which", "printenv", "export",
		"grep", "find", "rm", "cp", "mv", "mkdir", "touch", "whoami", "id",
		"ps", "top", "df", "du", "curl", "wget", "git", "npm", "node",
		"python", "java", "gradle", "make", "vi", "vim", "nano", "less", "tail", "head":
		return true
	}
	return false
}

func stepDisplayName(ev engine.StepEvent) string {
	if ev.Step == nil {
		return "<step>"
	}
	if ev.Step.Name != "" {
		return ev.Step.Name
	}
	if ev.Step.ID != "" {
		return ev.Step.ID
	}
	return "<step>"
}
