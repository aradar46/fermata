package engine

import (
	"fmt"
	"os"

	"github.com/nektos/act/pkg/model"
)

// ReloadStep re-reads the workflow file from disk and returns the step that
// corresponds to the given job/step, so a retry picks up the user's edits.
//
// Matching follows the plan's F7 ordering: by step id first, then by name, and
// never by bare index alone, since indices shift when the user edits the YAML
// mid-session. If the step can no longer be found, the caller is told rather
// than silently retrying something else.
func ReloadStep(workflowFile, jobID, stepID, stepName string) (*model.Step, error) {
	f, err := os.Open(workflowFile)
	if err != nil {
		return nil, fmt.Errorf("re-read workflow: %w", err)
	}
	defer f.Close()

	wf, err := model.ReadWorkflow(f, false)
	if err != nil {
		return nil, fmt.Errorf("parse workflow: %w", err)
	}

	job := wf.GetJob(jobID)
	if job == nil {
		return nil, fmt.Errorf("job %q no longer exists in the workflow", jobID)
	}

	// act assigns positional ids when a step has none, so a step whose id is
	// just its index must be matched by name instead.
	for _, s := range job.Steps {
		if s == nil {
			continue
		}
		if s.ID != "" && stepID != "" && s.ID == stepID {
			return s, nil
		}
	}
	for _, s := range job.Steps {
		if s == nil {
			continue
		}
		if s.Name != "" && stepName != "" && s.Name == stepName {
			return s, nil
		}
	}

	return nil, fmt.Errorf("step %q (id %q) no longer exists in job %q — "+
		"rename or removal mid-session is not auto-matched", stepName, stepID, jobID)
}
