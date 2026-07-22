package engine

import "testing"

// Regression: an empty GitHubInstance is interpolated straight into action
// clone URLs by act (fmt.Sprintf("https://%s", instance)), producing
// "https:///actions/setup-node", so every `uses:` step fails during setup,
// before any step runs and before fermata can pause. These defaults mirror
// act's own CLI and must not be dropped.
func TestBuildConfig_SetsActCLIDefaults(t *testing.T) {
	cfg := buildConfig(Options{
		WorkflowFile: "/tmp/x/.github/workflows/w.yml",
		EventName:    "push",
	}, "/tmp/x")

	if cfg.GitHubInstance != "github.com" {
		t.Errorf("GitHubInstance = %q, want %q (empty breaks every uses: step)",
			cfg.GitHubInstance, "github.com")
	}
	if cfg.RemoteName != "origin" {
		t.Errorf("RemoteName = %q, want %q", cfg.RemoteName, "origin")
	}
	if cfg.Actor == "" {
		t.Error("Actor must not be empty")
	}
	if !cfg.UseGitIgnore {
		t.Error("UseGitIgnore should default true, as in act's CLI")
	}
}

func TestBuildConfig_CarriesCallerValues(t *testing.T) {
	platforms := map[string]string{"ubuntu-latest": "img"}
	cfg := buildConfig(Options{
		WorkflowFile: "/tmp/x/.github/workflows/w.yml",
		EventName:    "workflow_dispatch",
		Platforms:    platforms,
	}, "/tmp/x")

	if cfg.EventName != "workflow_dispatch" {
		t.Errorf("EventName = %q", cfg.EventName)
	}
	if cfg.Workdir != "/tmp/x" {
		t.Errorf("Workdir = %q", cfg.Workdir)
	}
	if cfg.Platforms["ubuntu-latest"] != "img" {
		t.Errorf("Platforms not carried through: %v", cfg.Platforms)
	}
}
