# Changelog

## v0.1.0

First release. Fermata pauses a failing GitHub Actions workflow locally, lets
you inspect and fix it, and re-runs only the broken step.

### The debug loop

- **Pause** happens automatically when a step fails, or at a chosen step with
  `--break`. The job container stays alive, with the state earlier steps left
  behind.
- **`shell`** opens an interactive shell inside the paused container, with the
  step's environment, through act's own container abstraction.
- **`retry`** re-runs *only* the failed step after your fix, re-reading the
  workflow from disk. Measured on the bundled demo: 24s to reach the failure,
  ~1s to retry it. Nothing before the failing step runs again.
- **`skip`** marks a step skipped and continues, for steps that cannot work
  locally at all.
- **`env`** prints the step's environment, with known secret values masked.
- **`continue`** / **`quit`** / **`history`**.

### Correctness

- A retried step sees exactly the environment a clean run would: accumulated
  `GITHUB_ENV` and `GITHUB_PATH`, job- and step-level env, and prior step
  outputs. Proved by `scripts/fidelity-test.sh`, which is part of the release
  criteria rather than an aspiration.
- A successful retry clears the job failure, so fixing a step makes the run
  finish green.

### Working with real workflows

- `-P label=image` uses your own container image, or map a self-hosted
  `runs-on` label so the job can run at all.
- `--matrix key:value` debugs one matrix leg instead of fanning out.
- `--secret` / `--secret-file`, with values masked in fermata's own output.
- `--bind` bind-mounts the working directory so edits made while paused reach
  the retried step. Fermata detects the case where they would not and warns
  before running stale code.
- `--reuse` keeps the container between runs so tool caches (Gradle, Maven,
  pip) survive.
- `--event`, for workflows that do not trigger on `push`.

### Not sitting at a terminal

- `--hold <duration>` skips the interactive prompt: it keeps the failed step's container
  alive, print how to `docker exec` into it, then continue. For CI jobs and
  wrapper scripts.
- `--json` emits a JSONL event stream on stdout (job logs move to stderr), so other
  tools can consume a run without parsing human-facing output.

### Telling you what went wrong

- A scope preflight reports which jobs cannot run locally, and why, before
  starting, instead of silently skipping them and exiting 0.
- The failing step's output is quoted at the pause, leading with the error
  rather than trailing stack frames.
- Known local-vs-GitHub gaps (missing Android SDK, unreachable cache service,
  TLS failures, out of disk, OIDC) are named at the pause rather than left as a
  stack trace to decode.
- Errors for unsupported events list the events the workflow actually declares,
  with the command to use.
- The container name is printed at every pause, so you are never trapped if
  fermata exits.

### Known limits

Stated in the README rather than discovered: retry runs against dirty state,
session state is lost on exit, reusable workflows (`workflow_call`) are not
supported, steps needing GitHub's services cannot run locally, and the
in-container shell cannot mask secrets.

### Engine

Built on [act](https://github.com/nektos/act) v0.2.89, vendored with a 211-line
patch across 7 files (`docs/fork/`). CI verifies on every push that the patch
still applies to a clean upstream checkout.
