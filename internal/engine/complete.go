package engine

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nektos/act/pkg/model"
)

// WorkflowFiles lists workflow files under .github/workflows, relative to dir.
//
// Shell completion falls back to listing every file in the tree when a flag
// offers nothing, which for -W means offering source files that can never be
// valid. Narrowing it to the directory GitHub actually reads makes the
// suggestion list short enough to be worth reading.
func WorkflowFiles(dir string) []string {
	if dir == "" {
		dir = "."
	}
	var out []string
	for _, ext := range []string{"*.yml", "*.yaml"} {
		matches, err := filepath.Glob(filepath.Join(dir, ".github", "workflows", ext))
		if err != nil {
			continue
		}
		out = append(out, matches...)
	}
	sort.Strings(out)
	return out
}

// StepNames lists the steps of a workflow as completion candidates, each
// paired with the job it belongs to as a description.
//
// This is what --break wants: the flag takes a step id or name, and typing one
// by hand means reading the YAML and copying a string exactly, including its
// spaces. Steps with an explicit id are offered by id, because that is the
// form that survives the user renaming the step.
func StepNames(workflowFile string) []string {
	f, err := os.Open(workflowFile)
	if err != nil {
		return nil
	}
	defer f.Close()

	wf, err := model.ReadWorkflow(f, false)
	if err != nil {
		return nil
	}

	var out []string
	seen := map[string]bool{}
	for _, jobID := range wf.GetJobIDs() {
		job := wf.GetJob(jobID)
		if job == nil {
			continue
		}
		for _, s := range job.Steps {
			if s == nil {
				continue
			}
			// Prefer the id: it is stable across renames, and act generates a
			// positional one when the step has neither id nor name, which is
			// useless to complete.
			candidate := s.ID
			if candidate == "" || isPositional(candidate) {
				// Not Step.String(): that falls back to the `run:` body, which
				// can be a whole shell script and is never what --break wants.
				candidate = s.Name
				if candidate == "" {
					candidate = s.Uses
				}
			}
			if candidate == "" || seen[candidate] {
				continue
			}
			seen[candidate] = true
			out = append(out, candidate+"\t"+jobID)
		}
	}
	return out
}

// WorkflowEvents lists the events a workflow actually declares in its `on:`
// block, so --event offers the ones that will produce jobs instead of a
// generic list of every GitHub event.
func WorkflowEvents(workflowFile string) []string {
	f, err := os.Open(workflowFile)
	if err != nil {
		return nil
	}
	defer f.Close()

	wf, err := model.ReadWorkflow(f, false)
	if err != nil {
		return nil
	}
	return wf.On()
}

// isPositional reports whether act generated this id from the step's index
// rather than the user writing one.
func isPositional(id string) bool {
	if id == "" {
		return false
	}
	return strings.IndexFunc(id, func(r rune) bool { return r < '0' || r > '9' }) < 0
}
