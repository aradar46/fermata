# Contributing

Bug reports are the most useful thing you can send. Especially: a workflow that
fermata handles badly.

## Reporting a bug

Use the issue templates. They ask for your OS, architecture, Docker variant,
and the workflow file, because nearly every hard-to-reproduce report comes down
to one of those four.

If a step behaves differently under fermata than on GitHub's runners, please
use the **parity gap** template instead of the bug template. Those often are not
fermata bugs (see below), and separating them keeps both queues honest.

## Development

```sh
go build -o fermata .
go test ./internal/...        # fast, no Docker
./scripts/fidelity-test.sh    # needs Docker; proves retry env fidelity
```

Before opening a PR: `gofmt -l cmd internal main.go` must be empty, `go vet`
clean, tests passing. CI checks all three, plus that the vendored act patch
still applies to a clean upstream checkout.

If you touch retry or the engine, run `scripts/fidelity-test.sh`. It proves a
retried step sees exactly the environment a clean run would. That property is
the reason retry is trustworthy, and it is easy to break invisibly.

## What will be rejected

Saying this in writing beats saying it in twenty issue threads. Fermata is
deliberately one tool that does one thing:

- **A GUI, TUI, or web UI.** The CLI is the product. `--json` exists so you can
  build any interface you like on top of it, out of tree.
- **Other CI systems.** GitLab CI, Jenkins, CircleCI, Buildkite. Fermata is
  built on act, which implements GitHub Actions semantics. Supporting another
  system means writing another engine.
- **A config file.** Flags and act's own conventions only. Config files grow.
- **Anything that widens container privileges** for convenience:
  `--privileged` by default, mounting the Docker socket, disabling masking.
- **Speculative abstraction.** Plugin systems, extension points, interfaces
  with one implementation.

## What is genuinely wanted

- Workflows fermata handles badly, with the file attached.
- Reducing the vendored act patch (currently 211 lines / 7 files). Upstreaming
  a patch to act so fermata can drop it is the single most valuable
  contribution available.
- Documentation fixes, especially where an error message left you stuck.

## Rebasing the act fork

`third_party/act-fork/` is act v0.2.89 plus fermata's patches, vendored so a
clone builds with no extra steps. To move to a newer act, see
[docs/fork/](docs/fork/), which has the patch series and the reproduction steps.
Run the fidelity test afterwards.
