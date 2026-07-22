package engine

import (
	"os"
	"path/filepath"
	"testing"
)

const sampleWorkflow = `name: t
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - name: first
        run: echo one
      - name: second
        id: sec
        run: echo two
`

func writeWorkflow(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "wf.yml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
	return path
}

func TestReloadStep_MatchesByID(t *testing.T) {
	path := writeWorkflow(t, sampleWorkflow)

	step, err := ReloadStep(path, "build", "sec", "second")
	if err != nil {
		t.Fatalf("ReloadStep: %v", err)
	}
	if step.ID != "sec" {
		t.Errorf("matched wrong step: id=%q name=%q", step.ID, step.Name)
	}
}

func TestReloadStep_MatchesByNameWhenIDAbsent(t *testing.T) {
	path := writeWorkflow(t, sampleWorkflow)

	// act assigns positional ids, so a step without an explicit id must match
	// on name instead.
	step, err := ReloadStep(path, "build", "0", "first")
	if err != nil {
		t.Fatalf("ReloadStep: %v", err)
	}
	if step.Name != "first" {
		t.Errorf("matched wrong step: id=%q name=%q", step.ID, step.Name)
	}
}

func TestReloadStep_PicksUpEdits(t *testing.T) {
	path := writeWorkflow(t, sampleWorkflow)

	edited := `name: t
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - name: first
        run: echo one
      - name: second
        id: sec
        run: echo FIXED
`
	if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	step, err := ReloadStep(path, "build", "sec", "second")
	if err != nil {
		t.Fatalf("ReloadStep: %v", err)
	}
	if step.Run != "echo FIXED" {
		t.Errorf("edit not picked up, got run=%q", step.Run)
	}
}

func TestReloadStep_ErrorsWhenStepGone(t *testing.T) {
	path := writeWorkflow(t, sampleWorkflow)

	if _, err := ReloadStep(path, "build", "missing", "nope"); err == nil {
		t.Error("expected an error when the step no longer exists")
	}
}

func TestReloadStep_ErrorsWhenJobGone(t *testing.T) {
	path := writeWorkflow(t, sampleWorkflow)

	if _, err := ReloadStep(path, "no-such-job", "sec", "second"); err == nil {
		t.Error("expected an error when the job no longer exists")
	}
}

func TestReloadStep_ErrorsOnMissingFile(t *testing.T) {
	if _, err := ReloadStep(filepath.Join(t.TempDir(), "nope.yml"), "build", "x", "y"); err == nil {
		t.Error("expected an error for a missing workflow file")
	}
}
