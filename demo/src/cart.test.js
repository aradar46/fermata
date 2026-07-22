const assert = require('node:assert');
const { subtotal, applyDiscount } = require('./cart');

const items = [
  { price: 20, qty: 2 },  // 40
  { price: 10, qty: 1 },  // 10
];

assert.strictEqual(subtotal(items), 50);

// 10% off 50 = 45
assert.strictEqual(applyDiscount(items, 10), 45,
  `expected 45, got ${applyDiscount(items, 10)}`);

console.log('all tests passed');
