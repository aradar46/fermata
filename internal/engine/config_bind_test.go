package engine

import "testing"

// Without BindWorkdir act copies the repo into the container at checkout, so
// the container holds a snapshot. Edits made on the host while paused are then
// invisible to a retried step: you fix the bug and watch retry fail on the old
// code, which is the most confusing possible outcome. --bind is what makes
// "fix it and retry" work for source changes rather than only YAML changes.
func TestBuildConfig_BindWorkdirFollowsOption(t *testing.T) {
	base := Options{WorkflowFile: "/r/.github/workflows/w.yml", EventName: "push"}

	if cfg := buildConfig(base, "/r"); cfg.BindWorkdir {
		t.Error("BindWorkdir should default to false (act's copy-in behavior)")
	}

	withBind := base
	withBind.Bind = true
	if cfg := buildConfig(withBind, "/r"); !cfg.BindWorkdir {
		t.Error("--bind must set BindWorkdir so host edits reach a retried step")
	}
}

// --reuse keeps the container so tool caches survive; AutoRemove must be its
// inverse or the container is deleted anyway and the flag does nothing.
func TestBuildConfig_ReuseControlsContainerLifetime(t *testing.T) {
	base := Options{WorkflowFile: "/r/.github/workflows/w.yml", EventName: "push"}

	cfg := buildConfig(base, "/r")
	if cfg.ReuseContainers || !cfg.AutoRemove {
		t.Error("by default the container should be removed after the run")
	}

	withReuse := base
	withReuse.Reuse = true
	cfg = buildConfig(withReuse, "/r")
	if !cfg.ReuseContainers || cfg.AutoRemove {
		t.Error("--reuse must keep the container (ReuseContainers on, AutoRemove off)")
	}
}
