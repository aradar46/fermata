# The vendored act fork

[`third_party/act-fork/`](../../third_party/act-fork/) is a copy of
[nektos/act](https://github.com/nektos/act) at tag **v0.2.89**, plus a small
fermata patch series. It is vendored (flattened into this repo) so a fresh clone
builds with no extra steps; `go.mod` points at it via a `replace` directive.

## The patches

Three small changes on top of clean `v0.2.89`:

1. **`Config.StepHook`** (`pkg/runner/runner.go`, `pkg/runner/job_executor.go`)
   A step-boundary callback fired synchronously after each step's main
   executor, before job teardown, with the container still alive. This is
   fermata's pause point.
2. **`Config.LogWriter`** (`pkg/runner/runner.go`, `pkg/runner/logger.go`):
   redirects act's job log output to a writer fermata controls, so log streaming
   can be quiesced while the REPL owns the terminal, while keeping act's own
   log formatting.
3. **`Container.ExecInteractive`** (`pkg/container/container_types.go`,
   `pkg/container/docker_run.go`, `pkg/container/host_environment.go`). Like
   act's `Exec`, but attaches a caller-supplied stdin and a TTY so fermata can
   open an interactive shell inside the paused container, going through act's
   own container abstraction rather than shelling out to the docker CLI.
4. **`RunContext.RunSingleStep` / `SkipCurrentStep`** (`pkg/runner/fermata_single_step.go`, new file)
   Re-executes exactly one step against a live `RunContext`. It builds the
   step with act's own `stepFactory` and runs `step.main()`, which wraps itself
   in `runStepExecutor` → `setupEnv`. Env composition, `if:` evaluation, step
   results and masking therefore all come from act's own code paths rather than
   a re-implementation, which is what makes retry trustworthy (plan F3). It
   also clears the job error when a retry succeeds, so fixing a broken step no
   longer leaves the run reported as failed.

The full series is kept as a standalone patch file:
[`fermata-act-patches.patch`](fermata-act-patches.patch), verified to apply cleanly
to a fresh `v0.2.89` checkout.

## Reproducing the fork from upstream

If you ever need to rebuild the fork from a clean checkout (e.g. to rebase onto
a newer act tag):

```sh
git clone --branch v0.2.89 https://github.com/nektos/act.git third_party/act-fork
cd third_party/act-fork
git apply ../../docs/fork/fermata-act-patches.patch
rm -rf .git   # vendored copy carries no history
```

Then resolve any conflicts, run act's own `pkg/runner` tests plus fermata's
end-to-end pause test, and bump the pinned tag.

## Licensing

act is MIT-licensed. The vendored copy retains act's `LICENSE` and copyright
notice unchanged (MIT's only hard requirement). fermata itself is MIT.
