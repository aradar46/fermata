#!/usr/bin/env bash
# Fidelity test (plan F3): a step re-executed via `retry` must see exactly the
# environment it would have seen in a clean run — accumulated GITHUB_ENV,
# GITHUB_PATH, job- and step-level env, and prior step outputs.
#
# This is the trust-critical gate for retry: if RunSingleStep composed env even
# slightly differently, retried steps would lie. Run this after any rebase of
# the act fork.
#
# Usage: ./scripts/fidelity-test.sh   (requires Docker + a built ./fermata)
set -euo pipefail

FERMATA="${FERMATA:-./fermata}"
[ -x "$FERMATA" ] || { echo "build fermata first: go build -o fermata ."; exit 1; }

WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT
mkdir -p "$WORKDIR/.github/workflows"
WF="$WORKDIR/.github/workflows/fid.yml"

cat > "$WF" <<'YML'
name: fid
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    env:
      JOB_LEVEL: job-value
    steps:
      - name: set env and path
        run: |
          echo "ACCUM=first" >> $GITHUB_ENV
          echo "/opt/custom/bin" >> $GITHUB_PATH
      - name: emit output
        id: emitter
        run: echo "outkey=outval" >> $GITHUB_OUTPUT
      - name: observe
        env:
          STEP_LEVEL: step-value
        run: |
          echo "OBS ACCUM=$ACCUM"
          echo "OBS JOB_LEVEL=$JOB_LEVEL"
          echo "OBS STEP_LEVEL=$STEP_LEVEL"
          echo "OBS OUT=${{ steps.emitter.outputs.outkey }}"
          echo "OBS PATHHAS=$(echo $PATH | grep -c /opt/custom/bin)"
YML

echo "==> clean run (baseline)"
"$FERMATA" run -W "$WF" --no-break-on-failure 2>&1 \
  | grep -aE "OBS " | sed 's/.*| //' | sort > "$WORKDIR/clean.txt"
cat "$WORKDIR/clean.txt"

echo "==> breakpointed run, retrying the observed step"
FIFO="$WORKDIR/in.fifo"; mkfifo "$FIFO"
LOG="$WORKDIR/log.txt"
( "$FERMATA" run -W "$WF" --break "observe" < "$FIFO" > "$LOG" 2>&1 || true ) &
RUN_PID=$!
exec 9>"$FIFO"
for _ in $(seq 1 180); do grep -q "fermata paused" "$LOG" 2>/dev/null && break; sleep 1; done
echo "retry" >&9
sleep 12
echo "continue" >&9
sleep 12
exec 9>&-
wait "$RUN_PID" 2>/dev/null || true

awk '/re-running step/{f=1} f' "$LOG" | grep -aE "OBS " | sed 's/.*| //' | sort > "$WORKDIR/retried.txt"
cat "$WORKDIR/retried.txt"

echo "==> diff"
if diff "$WORKDIR/clean.txt" "$WORKDIR/retried.txt"; then
  echo "PASS: retried step env is identical to a clean run (F3 holds)"
else
  echo "FAIL: retry env differs from a clean run — fidelity broken"
  exit 1
fi
