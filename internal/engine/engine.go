// Package engine wraps act's runner: it builds an execution plan from a
// workflow file and runs it, wiring fermata's step-boundary hook into act's
// Config so the controller is notified at each step. It deliberately owns as
// little logic as possible: the differentiating behavior lives in the
// controller; this is the thin seam onto the (forked) engine.
package engine

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/nektos/act/pkg/model"
	"github.com/nektos/act/pkg/runner"
)

// StepEvent is what the controller receives at each step boundary. It is a
// narrow view over act's internals so the rest of fermata does not depend on
// act's RunContext shape directly.
type StepEvent struct {
	Ctx  context.Context
	RC   *runner.RunContext
	Step *model.Step
	Err  error // non-nil if this step failed
}

// Options configures a single workflow run.
type Options struct {
	// WorkflowFile is the path to the .yml/.yaml workflow to run.
	WorkflowFile string
	// EventName is the GitHub event to plan for (e.g. "push").
	EventName string
	// Platform maps a runs-on label to a container image. fermata v0.1 targets
	// ubuntu-* only (plan R5); default filled in by the CLI.
	Platforms map[string]string
	// OnStep is invoked after every step's main executor, before teardown.
	// Blocking inside it pauses the job with the container alive.
	OnStep func(StepEvent)
	// LogTo, if set, is where act's streaming log output is routed. Callers
	// pass a writer they can gate (quiesce during the REPL, plan F17). If nil,
	// act writes to os.Stdout as usual.
	LogTo io.Writer
	// OnCancel receives the run's cancel function once the run starts, so the
	// controller can abort the pipeline (e.g. on `quit`).
	OnCancel func(context.CancelFunc)
	// Secrets are passed to the workflow as ${{ secrets.NAME }}. Values are
	// masked in fermata's own output (they cannot be masked inside the
	// in-container shell: see the README).
	Secrets map[string]string
	// Notify, if set, receives fermata's own user-facing messages (as opposed
	// to act's streamed job output): e.g. the scope preflight report.
	Notify func(string)
	// Reuse keeps the job container alive after the run so tool caches built
	// inside it (Gradle, Maven, pip, ...) survive to the next run.
	Reuse bool
	// Matrix restricts which matrix legs run, e.g. {"python": {"3.12": true}}.
	// Debugging is a single-cell activity; without this a matrix job fans out
	// into every combination at once.
	Matrix map[string]map[string]bool
	// Bind mounts the working directory into the container instead of copying
	// it. Without this, act snapshots the repo at checkout, so edits you make
	// on the host while paused are invisible to a retried step: you could fix
	// the bug and watch retry fail on the old code.
	Bind bool
}

// Run plans and executes the workflow, returning the workflow's result error
// (non-nil if any job failed) or a setup error.
func Run(ctx context.Context, opts Options) error {
	if opts.WorkflowFile == "" {
		return fmt.Errorf("no workflow file given")
	}
	if opts.EventName == "" {
		opts.EventName = "push"
	}

	wfPath, err := filepath.Abs(opts.WorkflowFile)
	if err != nil {
		return fmt.Errorf("resolve workflow path: %w", err)
	}

	planner, err := model.NewWorkflowPlanner(wfPath, false, false)
	if err != nil {
		return fmt.Errorf("read workflow %s: %w", wfPath, err)
	}
	plan, err := planner.PlanEvent(opts.EventName)
	if err != nil {
		return fmt.Errorf("plan event %q: %w", opts.EventName, err)
	}
	if plan == nil || len(plan.Stages) == 0 {
		return noJobsError(wfPath, opts.EventName, planner.GetEvents())
	}

	// Scope check: tell the user up front which jobs can't be debugged locally
	// and why. Without this, act silently skips e.g. macOS jobs and exits 0,
	// which looks like the tool did nothing at all.
	if scopes, perr := Preflight(wfPath, opts.EventName, opts.Platforms); perr == nil {
		report, anyRunnable := ScopeReport(scopes)
		if report != "" && opts.Notify != nil {
			opts.Notify(strings.TrimRight(report, "\n"))
		}
		if !anyRunnable {
			return fmt.Errorf("no job in %s can run locally — see the reasons above.\n"+
				"  fermata v0.1 supports Linux jobs (runs-on: ubuntu-*)",
				filepath.Base(wfPath))
		}
	}

	// Workdir: the repository root is the workflow file's dir climbing out of
	// .github/workflows. For v0.1 we use the directory two levels up when the
	// path looks like .../.github/workflows/x.yml, else the file's dir.
	workdir := deriveWorkdir(wfPath)

	cfg := buildConfig(opts, workdir)

	if opts.OnStep != nil {
		cfg.StepHook = func(ctx context.Context, rc *runner.RunContext, step *model.Step, stepErr error) {
			opts.OnStep(StepEvent{Ctx: ctx, RC: rc, Step: step, Err: stepErr})
		}
	}

	r, err := runner.New(cfg)
	if err != nil {
		return fmt.Errorf("create runner: %w", err)
	}

	// Derive a cancelable context so the controller can abort on `quit`.
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if opts.OnCancel != nil {
		opts.OnCancel(cancel)
	}

	return r.NewPlanExecutor(plan)(runCtx)
}

// buildConfig assembles act's runner.Config for a run.
//
// The defaults here mirror what act's own CLI applies. They are not optional
// polish: GitHubInstance is interpolated directly into action clone URLs
// (fmt.Sprintf("https://%s", instance)), so an empty value yields
// "https:///actions/checkout" and every `uses:` step fails during setup,
// before any step runs, and therefore before fermata can pause on anything.
func buildConfig(opts Options, workdir string) *runner.Config {
	return &runner.Config{
		Workdir:   workdir,
		EventName: opts.EventName,
		Platforms: opts.Platforms,
		LogOutput: true,
		LogWriter: opts.LogTo, // nil -> act defaults to os.Stdout

		// With Reuse, the job container survives the run, so anything a build
		// caches inside it (Gradle distributions and ~/.gradle, ~/.m2, pip and
		// friends) is still there next time instead of being re-downloaded.
		ReuseContainers: opts.Reuse,
		AutoRemove:      !opts.Reuse,

		GitHubInstance: "github.com",
		RemoteName:     "origin",
		Actor:          "nektos/act",
		UseGitIgnore:   true,
		BindWorkdir:    opts.Bind,

		Secrets: opts.Secrets,
		Matrix:  opts.Matrix,
	}
}

// WorkdirFor returns the directory act will use as the working directory for
// the given workflow file, resolving it the same way Run does.
func WorkdirFor(workflowFile string) string {
	abs, err := filepath.Abs(workflowFile)
	if err != nil {
		return ""
	}
	return deriveWorkdir(abs)
}

// deriveWorkdir returns the repo root for a workflow path. For
// <root>/.github/workflows/file.yml it returns <root>; otherwise the file's
// directory.
func deriveWorkdir(wfPath string) string {
	dir := filepath.Dir(wfPath) // .../.github/workflows
	if filepath.Base(dir) == "workflows" {
		parent := filepath.Dir(dir) // .../.github
		if filepath.Base(parent) == ".github" {
			return filepath.Dir(parent) // .../<root>
		}
	}
	return dir
}
