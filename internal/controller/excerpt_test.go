package controller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFailureExcerpt_StripsActPrefixAndShowsStepOutput(t *testing.T) {
	tail := strings.Join([]string{
		`[CI/test] ⭐ Run Main Run tests`,
		`[CI/test]   🐳  docker exec cmd=[node src/cart.test.js]`,
		`[CI/test]   | AssertionError [ERR_ASSERTION]: expected 45, got 49.8`,
		`[CI/test]   | 49.8 !== 45`,
		`[CI/test]   ❌  Failure - Main Run tests [95ms]`,
		`[CI/test] exitcode '1': failure`,
	}, "\n")

	got := failureExcerpt(tail)

	if !strings.Contains(got, "expected 45, got 49.8") {
		t.Errorf("excerpt should contain the assertion, got:\n%s", got)
	}
	// act's decoration and status chatter should not survive.
	for _, unwanted := range []string{"[CI/test]", "docker exec", "Failure - Main", "exitcode '1'"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("excerpt should not contain %q, got:\n%s", unwanted, got)
		}
	}
}

// act repeats git warnings when the workflow isn't in a repo; they would push
// the real error out of a short excerpt.
func TestFailureExcerpt_DropsGitNoise(t *testing.T) {
	var lines []string
	for i := 0; i < 10; i++ {
		lines = append(lines, `[x] unable to get git ref: repository does not exist`)
	}
	lines = append(lines, `[x]   | real error here`)

	got := failureExcerpt(strings.Join(lines, "\n"))

	if !strings.Contains(got, "real error here") {
		t.Errorf("real error was crowded out:\n%s", got)
	}
	if strings.Contains(got, "unable to get git ref") {
		t.Errorf("git noise should be filtered:\n%s", got)
	}
}

func TestFailureExcerpt_LimitsLineCount(t *testing.T) {
	var lines []string
	for i := 0; i < 50; i++ {
		lines = append(lines, "[x]   | line")
	}
	got := failureExcerpt(strings.Join(lines, "\n"))

	if n := strings.Count(got, "│"); n > excerptLines {
		t.Errorf("excerpt has %d lines, limit is %d", n, excerptLines)
	}
}

func TestFailureExcerpt_EmptyWhenNothingUseful(t *testing.T) {
	for _, tail := range []string{"", "   ", "[x] ⭐ Run Main thing\n[x] ✅  Success - Main thing"} {
		if got := failureExcerpt(tail); got != "" {
			t.Errorf("expected no excerpt for %q, got %q", tail, got)
		}
	}
}

func TestEditedSince_DetectsEditAfterPause(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "cart.js")
	if err := os.WriteFile(src, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	pausedAt := time.Now()
	time.Sleep(20 * time.Millisecond)
	if err := os.WriteFile(src, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}

	path, ok := editedSince(dir, pausedAt)
	if !ok {
		t.Fatal("expected the edit to be detected")
	}
	if filepath.Base(path) != "cart.js" {
		t.Errorf("detected %q, want cart.js", path)
	}
}

func TestEditedSince_QuietWhenNothingChanged(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.js"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	time.Sleep(20 * time.Millisecond)
	if _, ok := editedSince(dir, time.Now()); ok {
		t.Error("no edit happened after the pause; should not warn")
	}
}

// Dependency and VCS churn must not be mistaken for the user editing source.
func TestEditedSince_IgnoresNoisyDirectories(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "node_modules", "pkg")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	pausedAt := time.Now()
	time.Sleep(20 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(nested, "index.js"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if path, ok := editedSince(dir, pausedAt); ok {
		t.Errorf("node_modules churn should be ignored, flagged %q", path)
	}
}

func TestEditedSince_EmptyRootIsSafe(t *testing.T) {
	if _, ok := editedSince("", time.Now()); ok {
		t.Error("an empty root should not report an edit")
	}
}
