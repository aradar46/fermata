package runner

import (
	"context"
	"errors"

	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
)

// RunSingleStep is a fermata addition: it re-executes exactly one step against
// this RunContext, reusing the engine's own machinery.
//
// Fidelity is the whole point (and the reason this lives inside pkg/runner
// rather than in an embedder): the step is built by the same stepFactory a
// clean run uses, and executed via step.main(), which wraps itself in
// runStepExecutor -> setupEnv. That means accumulated GITHUB_ENV/GITHUB_PATH,
// prior step outputs, `if:` evaluation, masking, and step results are all
// composed by act's own code paths, not re-implemented here. A retried step
// therefore sees the same environment it would have seen in a clean run.
//
// The job container and every earlier step's filesystem/env state must still be
// live: this is intended to be called from a step-boundary hook (see
// Config.StepHook), while the job is paused and before teardown.
//
// The passed step model may come from a freshly re-read workflow file, so the
// user's edits to the step are picked up.
func (rc *RunContext) RunSingleStep(ctx context.Context, stepModel *model.Step) error {
	if stepModel == nil {
		return errNilStep
	}
	// act assigns positional IDs when a step has none; preserve that behavior so
	// StepResults keys stay consistent with the original run.
	if stepModel.ID == "" {
		stepModel.ID = rc.CurrentStep
	}

	sf := &stepFactoryImpl{}
	step, err := sf.newStep(stepModel, rc)
	if err != nil {
		return err
	}

	// Same logger/log-writer wiring the job executor uses for a step.
	err = useStepLogger(rc, stepModel, stepStageMain, step.main())(ctx)

	// A clean run records a step failure as the job error. When a retry
	// succeeds, the job should no longer be considered failed — otherwise
	// fixing the broken step still reports a failed run. Clearing is only safe
	// when the caller's ctx carries act's job-error container (i.e. the retry
	// was driven from a step hook inside a running job).
	if common.JobError(ctx) != nil {
		if err == nil {
			common.SetJobError(ctx, nil)
		} else {
			common.SetJobError(ctx, err)
		}
	}

	return err
}

// errNilStep is returned when RunSingleStep is called without a step.
var errNilStep = errors.New("fermata: RunSingleStep called with a nil step")

// JobContainerName exposes the name act gave this job's container.
//
// A debugger that owns the only handle to a live container is a trap: if the
// tool dies, the user is left with an orphan they cannot find or clean up.
// Printing the name means `docker exec` and `docker rm` still work without us.
func (rc *RunContext) JobContainerName() string {
	return rc.jobContainerName()
}

// SkipCurrentStep is a fermata addition: it marks the step that just failed as
// skipped and clears the job error, so the run continues and finishes green
// rather than being reported as failed by a step the user chose to skip.
//
// This mirrors what act records for a step disabled by an `if:` expression:
// outcome and conclusion both "skipped". It is intended to be called from a
// step hook while the job is paused.
func (rc *RunContext) SkipCurrentStep(ctx context.Context, stepModel *model.Step) {
	id := rc.CurrentStep
	if stepModel != nil && stepModel.ID != "" {
		id = stepModel.ID
	}
	if id != "" {
		if rc.StepResults == nil {
			rc.StepResults = map[string]*model.StepResult{}
		}
		rc.StepResults[id] = &model.StepResult{
			Outputs:    map[string]string{},
			Outcome:    model.StepStatusSkipped,
			Conclusion: model.StepStatusSkipped,
		}
	}

	// Drop the failure this step recorded, or the job still ends up failed.
	if common.JobError(ctx) != nil {
		common.SetJobError(ctx, nil)
	}
}
