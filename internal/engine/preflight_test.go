package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeWF(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "wf.yml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

var linuxPlatforms = map[string]string{"ubuntu-latest": "img", "ubuntu-22.04": "img"}

func TestPreflight_UbuntuJobIsRunnable(t *testing.T) {
	path := writeWF(t, `name: t
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: echo hi
`)
	scopes, err := Preflight(path, "push", linuxPlatforms)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if len(scopes) != 1 {
		t.Fatalf("got %d jobs, want 1", len(scopes))
	}
	if !scopes[0].Runnable() {
		t.Errorf("ubuntu job should be runnable, got reason %q", scopes[0].Reason)
	}
}

// The ios-deploy.yml case: act silently skips macOS jobs and exits 0, which
// looks like the tool did nothing. Preflight must call it out.
func TestPreflight_MacOSJobIsNotRunnable(t *testing.T) {
	path := writeWF(t, `name: t
on: push
jobs:
  build_ios:
    runs-on: macos-15
    steps:
      - run: echo hi
`)
	scopes, err := Preflight(path, "push", linuxPlatforms)
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if scopes[0].Runnable() {
		t.Fatal("macOS job must not be reported as runnable")
	}
	if !strings.Contains(scopes[0].Reason, "macOS") {
		t.Errorf("reason should mention macOS, got %q", scopes[0].Reason)
	}

	report, anyRunnable := ScopeReport(scopes)
	if anyRunnable {
		t.Error("anyRunnable should be false")
	}
	if !strings.Contains(report, "build_ios") {
		t.Errorf("report should name the job, got %q", report)
	}
}

func TestPreflight_WindowsJobIsNotRunnable(t *testing.T) {
	path := writeWF(t, `name: t
on: push
jobs:
  win:
    runs-on: windows-latest
    steps:
      - run: echo hi
`)
	scopes, _ := Preflight(path, "push", linuxPlatforms)
	if scopes[0].Runnable() {
		t.Fatal("Windows job must not be runnable")
	}
	if !strings.Contains(scopes[0].Reason, "Windows") {
		t.Errorf("reason should mention Windows, got %q", scopes[0].Reason)
	}
}

func TestPreflight_MixedWorkflowStillHasRunnableJob(t *testing.T) {
	path := writeWF(t, `name: t
on: push
jobs:
  linux:
    runs-on: ubuntu-latest
    steps:
      - run: echo hi
  mac:
    runs-on: macos-14
    steps:
      - run: echo hi
`)
	scopes, _ := Preflight(path, "push", linuxPlatforms)
	report, anyRunnable := ScopeReport(scopes)
	if !anyRunnable {
		t.Error("a workflow with one ubuntu job must still be runnable")
	}
	if !strings.Contains(report, "mac") {
		t.Errorf("report should still explain the skipped mac job, got %q", report)
	}
}

func TestNoJobsError_ListsAvailableEvents(t *testing.T) {
	err := noJobsError("/x/.github/workflows/build-test.yml", "push",
		[]string{"pull_request", "workflow_dispatch"})
	msg := err.Error()

	for _, want := range []string{"pull_request", "workflow_dispatch", "--event"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message should mention %q, got:\n%s", want, msg)
		}
	}
}

func TestNoJobsError_HandlesWorkflowWithNoEvents(t *testing.T) {
	err := noJobsError("/x/wf.yml", "push", nil)
	if !strings.Contains(err.Error(), "no events") {
		t.Errorf("got %q", err.Error())
	}
}
