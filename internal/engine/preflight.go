package engine

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/nektos/act/pkg/model"
)

// JobScope reports whether a job can actually be debugged locally.
type JobScope struct {
	JobID  string
	RunsOn string
	// Reason is empty when the job is runnable; otherwise it explains why not.
	Reason string
}

// Runnable reports whether this job can run under fermata.
func (j JobScope) Runnable() bool { return j.Reason == "" }

// Preflight reads the workflow and classifies each job for the given event,
// so out-of-scope workflows fail with an explanation up front instead of act
// silently skipping every job and exiting successfully (which reads as "it did
// nothing"). This is the scope check the plan calls for: v0.1 supports Linux
// (ubuntu-*) jobs only, because Windows and macOS jobs cannot run in Linux
// containers.
func Preflight(workflowFile, eventName string, platforms map[string]string) ([]JobScope, error) {
	f, err := os.Open(workflowFile)
	if err != nil {
		return nil, fmt.Errorf("read workflow: %w", err)
	}
	defer f.Close()

	wf, err := model.ReadWorkflow(f, false)
	if err != nil {
		return nil, fmt.Errorf("parse workflow: %w", err)
	}

	ids := wf.GetJobIDs()
	sort.Strings(ids)

	scopes := make([]JobScope, 0, len(ids))
	for _, id := range ids {
		job := wf.GetJob(id)
		if job == nil {
			continue
		}
		scopes = append(scopes, classifyJob(id, job, platforms))
	}
	return scopes, nil
}

func classifyJob(id string, job *model.Job, platforms map[string]string) JobScope {
	labels := job.RunsOn()
	runsOn := strings.Join(labels, ", ")
	scope := JobScope{JobID: id, RunsOn: runsOn}

	// A reusable workflow job (`uses:` at job level) is out of scope for v0.1.
	if job.Uses != "" {
		scope.Reason = "calls a reusable workflow (not supported yet)"
		return scope
	}

	if len(labels) == 0 {
		scope.Reason = "no runs-on specified"
		return scope
	}

	for _, l := range labels {
		lower := strings.ToLower(l)
		if _, mapped := platforms[lower]; mapped {
			return scope // runnable: we have an image for this label
		}
		if strings.HasPrefix(lower, "windows") {
			scope.Reason = fmt.Sprintf("runs-on %q — Windows jobs can't run in Linux containers", l)
			return scope
		}
		if strings.HasPrefix(lower, "macos") || strings.HasPrefix(lower, "mac-") {
			scope.Reason = fmt.Sprintf("runs-on %q — macOS jobs can't run in Linux containers", l)
			return scope
		}
	}

	scope.Reason = fmt.Sprintf("runs-on %q — no container image mapped for this label", runsOn)
	return scope
}

// ScopeReport renders a human-readable summary, and reports whether anything
// is runnable at all.
func ScopeReport(scopes []JobScope) (report string, anyRunnable bool) {
	var b strings.Builder
	for _, s := range scopes {
		if s.Runnable() {
			anyRunnable = true
			continue
		}
		fmt.Fprintf(&b, "  skipping job %q: %s\n", s.JobID, s.Reason)
	}
	return b.String(), anyRunnable
}
