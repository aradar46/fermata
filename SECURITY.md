# Security

## Reporting a vulnerability

Email **hello@aradar.top** with "fermata security" in the subject. Please
include what you did, what happened, and what you expected. You will get a
reply within a week.

Do not open a public issue for a vulnerability.

This is a solo, unfunded project: there is no bounty, and fixes ship when they
ship. That is stated plainly so you can calibrate your expectations rather than
guess.

## What fermata does, so you can judge the risk yourself

Fermata runs your GitHub Actions workflow on your machine, in Docker. That
means it **executes the code in that workflow, and any action the workflow
uses, on your computer with your Docker daemon.**

This is the same trust decision as running `npm install` on a branch you did
not write. It is not a sandbox. Specifically:

- **Untrusted branches.** Debugging a pull request from a stranger runs their
  code locally. Read the workflow first, exactly as you would read a
  `package.json` postinstall script.
- **Secrets.** Values passed via `--secret` / `--secret-file` are masked in
  fermata's own output, but **cannot be masked inside the container shell**:
  once you have a shell, `echo $MY_TOKEN` prints it. Do not screen-share a
  fermata shell session with real credentials loaded.
- **Secret files.** `--secret-file` reads plaintext. Keep it out of version
  control; `.secrets` and `*.secrets` are gitignored by default here.
- **Docker access.** Fermata talks to your Docker daemon. It does not add
  `--privileged` and does not mount the Docker socket into job containers, and
  it will not gain convenience features that do.

## Scope

In scope: anything that lets a workflow escape the boundaries above, for
example leaking a masked secret through fermata's own output, or fermata
granting a container more access than act would.

Out of scope: the fact that running a workflow runs its code. That is the
purpose of the tool.
