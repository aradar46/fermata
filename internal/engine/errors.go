package engine

import (
	"fmt"
	"path/filepath"
	"strings"
)

// noJobsError explains why nothing ran and how to fix it. A workflow that only
// triggers on workflow_dispatch or pull_request produces no jobs for the
// default "push" event, and simply saying "no jobs" leaves the user guessing,
// so name the events the file actually declares and show the flag to use.
func noJobsError(workflowPath, requestedEvent string, available []string) error {
	base := filepath.Base(workflowPath)

	if len(available) == 0 {
		return fmt.Errorf("%s declares no events (`on:`), so there is nothing to run", base)
	}

	// If the workflow does declare events, the user just picked the wrong one.
	var b strings.Builder
	fmt.Fprintf(&b, "%s has no jobs for event %q.\n", base, requestedEvent)
	fmt.Fprintf(&b, "  it triggers on: %s\n", strings.Join(available, ", "))
	fmt.Fprintf(&b, "  try: fermata run -W %s --event %s",
		workflowPath, available[0])
	return fmt.Errorf("%s", b.String())
}
