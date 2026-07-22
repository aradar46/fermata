// Shopping cart totals.
function subtotal(items) {
  return items.reduce((sum, item) => sum + item.price * item.qty, 0);
}

// BUG: applies the discount to the item count instead of the subtotal.
function applyDiscount(items, percentOff) {
  return subtotal(items) - items.length * (percentOff / 100);
}

module.exports = { subtotal, applyDiscount };
