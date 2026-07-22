# The 60-second demo: walkthrough script

You perform this live. Everything below is measured from real runs, not
estimated: reaching the failure takes **~24s**, the retry takes **~1s**.

That contrast is the whole pitch. Don't rush the setup steps. The audience
needs to *feel* the 24 seconds before they can appreciate the 1.

---

## Before you hit record

```sh
cd demo

# 1. Make sure the bug is present (this is the state the demo starts from)
git checkout src/cart.js        # or: sed -i 's|\* (1 - percentOff / 100)|- items.length * (percentOff / 100)|' src/cart.js
grep 'return subtotal' src/cart.js
#   -> return subtotal(items) - items.length * (percentOff / 100);

# 2. Warm the image and action cache so the demo isn't 3 minutes of docker pull
../fermata run -W .github/workflows/ci.yml --bind --no-break-on-failure >/dev/null 2>&1

# 3. Reset the bug again (step 2 doesn't change source, but be sure)
grep 'return subtotal' src/cart.js

# 4. Clean any leftover containers
docker rm -f $(docker ps -aq --filter name=act-) 2>/dev/null
```

Have your editor open on `demo/src/cart.js`, on a second screen or split pane.
Terminal font large enough to read on video.

**Do not skip `--bind`.** Without it the container gets a *copy* of the repo at
checkout, so your edit never reaches the retried step and the demo fails.

---

## The performance

### 0:00  Start the run

Say: *"This is a normal GitHub Actions workflow. I'm running it on my laptop."*

```sh
fermata run -W .github/workflows/ci.yml --bind
```

Steps scroll past: Checkout, Setup Node, Install dependencies, Build.

### 0:05–0:24  Let it work

Say, over the install/build steps: *"Install, build, the stuff you normally
wait on. About twenty-five seconds here; on a real pipeline it's minutes."*

Don't talk over the failure when it lands.

### 0:24  The failure

```
| AssertionError [ERR_ASSERTION]: expected 45, got 49.8
❌  Failure - Main Run tests

⏸  fermata paused at step "Run tests" — step failed: exitcode '1': failure
   commands: continue  quit  env  shell  retry  skip  (help)

fermata>
```

Say: *"Normally this is where the run dies and you push a fix and wait again.
Watch what happens instead. It stopped. The container is still alive."*

**Beat. Let the pause sit for two seconds.** This is the moment.

### 0:30  Look around inside (optional, cut if tight on time)

```
fermata> shell
```
```sh
node -e "const c=require('./src/cart');console.log(c.applyDiscount([{price:20,qty:2},{price:10,qty:1}],10))"
# 49.8
exit
```

Say: *"I'm inside the container that just failed. Same filesystem, same env."*

If you're cutting for length, drop this section; the retry is the point.

### 0:38  Fix the bug

Switch to the editor. In `src/cart.js`, change one line:

```js
// from
return subtotal(items) - items.length * (percentOff / 100);
// to
return subtotal(items) * (1 - percentOff / 100);
```

Save. Say: *"The discount was being applied to the item count instead of the
subtotal. One line."*

### 0:48  Retry

Back to the terminal:

```
fermata> retry
```

```
re-running step "Run tests" (container state is post-failure; nothing was rolled back)
| all tests passed
retry succeeded — `continue` to resume the rest of the job
```

Say, and land it clearly: **"One second. It did not re-run checkout, install,
or build. Those twenty-five seconds are still done."**

### 0:53  Finish the job

```
fermata> continue
```

```
| Published!
🏁  Job succeeded
```

Say: *"Green. Without pushing anything."*

### 0:58  Close

*"Breakpoints, a shell, and retry-from-the-broken-step, on the workflow file
you already have. That's Fermata."*

---

## Measured timings

| Phase | Time |
|---|---|
| Checkout + Setup Node | ~0.6s |
| Install dependencies | 12s |
| Build | 10s |
| Run tests (fails) | ~0.1s |
| **Reach the failure** | **~24s** |
| **Retry the fixed step** | **~1s** |
| Continue to green | ~1s |

The saving shown is 24s. Say that number, not an inflated one. The honesty is
part of the pitch, and on a real pipeline the same structure saves minutes.

## If something goes wrong on camera

- **Retry fails with the same error** → you forgot `--bind`, or didn't save the
  file. Restart.
- **First run is slow** → the image wasn't warmed. Do the pre-flight above.
- **`fermata>` doesn't accept `retry`** → you're inside the container shell;
  type `exit` first.
