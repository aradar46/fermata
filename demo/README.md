# Demo project

A deliberately broken project used to demonstrate Fermata's pause → fix →
retry loop. See [SCRIPT.md](SCRIPT.md) for the walkthrough.

## The bug

`src/cart.js` applies a percentage discount to the **item count** instead of
the **subtotal**:

```js
return subtotal(items) - items.length * (percentOff / 100);
```

So 10% off a £50 cart returns `49.8` instead of `45`. `src/cart.test.js`
catches it, which fails the `Run tests` step of
[.github/workflows/ci.yml](.github/workflows/ci.yml).

The fix is one line:

```js
return subtotal(items) * (1 - percentOff / 100);
```

## Running it

```sh
fermata run -W .github/workflows/ci.yml --bind
```

`--bind` matters: without it act copies the repo into the container at
checkout, so edits you make on the host while paused never reach the retried
step, and `retry` re-runs the old code.

## Resetting between takes

```sh
sed -i 's|\* (1 - percentOff / 100)|- items.length * (percentOff / 100)|' src/cart.js
```

## Why the workflow sleeps

`Install dependencies` and `Build` sleep 12s and 10s so the demo has realistic
setup time ahead of the failing step. That's the point being demonstrated:
~24s to reach the failure, ~1s to retry it, with nothing before it re-run.
