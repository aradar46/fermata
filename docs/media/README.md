# Media

## retry-demo.svg

An animated SVG of the pause → fix → retry loop, embedded at the top of the
project README. Self-contained (SMIL animation, no JavaScript), so it renders
on GitHub.

Its content is taken from a **real** captured run of [`demo/`](../../demo/):
the step names, the assertion (`expected 45, got 49.8`), and the timings are
what actually happened, not mock-ups:

| | measured |
|---|---|
| Reach the failure | 24.7s |
| Retry the fixed step | ~1s |

It is a condensed re-enactment on a 27-second loop, not a frame-accurate
terminal capture; the waiting is compressed so the loop stays watchable.

### Recording a real terminal capture instead

The SVG is a stand-in for a proper recording. For the real thing (asciinema
cast or GIF), perform [`demo/SCRIPT.md`](../../demo/SCRIPT.md) at a terminal:

```sh
# one-off tooling
pipx install asciinema     # or: pip install --user asciinema

cd demo
asciinema rec ../docs/media/retry-demo.cast
#   ... perform the script ...
#   exit when done
```

That capture is worth doing before any public launch: a real recording of a
real session is more convincing than an illustration, however honest.
