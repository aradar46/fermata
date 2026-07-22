package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeWorkflowAt(t *testing.T, dir, name, body string) string {
	t.Helper()
	wfDir := filepath.Join(dir, ".github", "workflows")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(wfDir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const completionWorkflow = `name: CI
on: [push, workflow_dispatch]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Run tests
        run: npm test
      - id: build
        name: Build the thing
        run: make
      - run: echo no name and no id
`

func TestWorkflowFilesFindsOnlyWorkflows(t *testing.T) {
	dir := t.TempDir()
	writeWorkflowAt(t, dir, "ci.yml", completionWorkflow)
	writeWorkflowAt(t, dir, "release.yaml", completionWorkflow)
	// A file outside .github/workflows must not be offered.
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := WorkflowFiles(dir)
	if len(got) != 2 {
		t.Fatalf("want 2 workflows, got %d: %v", len(got), got)
	}
	for _, g := range got {
		if strings.Contains(g, "docker-compose") {
			t.Errorf("offered a non-workflow file: %s", g)
		}
	}
}

func TestWorkflowFilesEmptyWhenNoneExist(t *testing.T) {
	if got := WorkflowFiles(t.TempDir()); len(got) != 0 {
		t.Errorf("want none, got %v", got)
	}
}

func TestStepNamesPrefersIDThenName(t *testing.T) {
	dir := t.TempDir()
	wf := writeWorkflowAt(t, dir, "ci.yml", completionWorkflow)

	got := StepNames(wf)
	joined := strings.Join(got, "\n")

	// A step with an explicit id is offered by id, since that survives renames.
	if !strings.Contains(joined, "build\ttest") {
		t.Errorf("want the id 'build' offered, got:\n%s", joined)
	}
	// A step with only a name is offered by name.
	if !strings.Contains(joined, "Run tests\ttest") {
		t.Errorf("want 'Run tests' offered, got:\n%s", joined)
	}
	// A `uses:` step with neither falls back to the action reference.
	if !strings.Contains(joined, "actions/checkout@v4") {
		t.Errorf("want the checkout action offered, got:\n%s", joined)
	}
}

// The bare `run:` step has no id, no name, and no uses. act's Step.String()
// would return the shell script body, which is useless as something to type
// after --break, so it must be dropped rather than offered.
func TestStepNamesOmitsRunBodies(t *testing.T) {
	dir := t.TempDir()
	wf := writeWorkflowAt(t, dir, "ci.yml", completionWorkflow)

	for _, got := range StepNames(wf) {
		if strings.Contains(got, "echo no name and no id") {
			t.Errorf("offered a run: body as a completion candidate: %q", got)
		}
	}
}

func TestStepNamesToleratesBadInput(t *testing.T) {
	dir := t.TempDir()
	bad := writeWorkflowAt(t, dir, "bad.yml", "not: [valid: yaml")

	// Completion runs on every TAB keypress; it must never panic or block,
	// only return nothing.
	if got := StepNames(bad); len(got) != 0 {
		t.Errorf("want none for malformed yaml, got %v", got)
	}
	if got := StepNames(filepath.Join(dir, "does-not-exist.yml")); len(got) != 0 {
		t.Errorf("want none for a missing file, got %v", got)
	}
}

func TestWorkflowEventsReadsOnBlock(t *testing.T) {
	dir := t.TempDir()
	wf := writeWorkflowAt(t, dir, "ci.yml", completionWorkflow)

	got := WorkflowEvents(wf)
	want := map[string]bool{"push": true, "workflow_dispatch": true}
	if len(got) != len(want) {
		t.Fatalf("want %d events, got %v", len(want), got)
	}
	for _, g := range got {
		if !want[g] {
			t.Errorf("unexpected event %q", g)
		}
	}
}

func TestIsPositional(t *testing.T) {
	for _, tc := range []struct {
		id   string
		want bool
	}{
		{"0", true},
		{"12", true},
		{"build", false},
		{"step2", false},
		{"", false},
	} {
		if got := isPositional(tc.id); got != tc.want {
			t.Errorf("isPositional(%q) = %v, want %v", tc.id, got, tc.want)
		}
	}
}
