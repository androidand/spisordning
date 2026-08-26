-- +goose Up
-- Shopping cart (a checkpoint record of a to-cart call, not the retailer's live cart).
--
-- This migration adds the tables that record spisordning's checkpoint of a to-cart call
-- (see openspec/changes/implement-shopping-and-commerce/design.md, "Cart checkpoint (v1)"):
--   shopping_cart        that a to-cart call was made, for which binding, with what status
--   shopping_cart_item   the resolved items (product codes + quantities + prices) at that moment
--
-- A shopping_cart is a checkpoint, NOT a mirror of the retailer's live cart state (design D3).
-- Willys owns the actual cart server-side; spisordning has no visibility into it beyond having
-- triggered the POST /shopping-lists/:id/to-cart call. The cart records the resolved items and
-- quantities at the moment of the to-cart call — a snapshot for later order reconciliation.
--
-- The to-cart call is the LAST automated step in this change. No code path in this change (or
-- the adapter it calls) can trigger checkout, payment, or slot booking; those remain human
-- actions in the retailer's own app/site (task 3.3).
--
-- Invariants enforced here (schema-level):
--   * A cart belongs to exactly one retailer_list_binding (retailer_list_binding_id FK, CASCADE).
--   * A cart item belongs to exactly one cart (shopping_cart_id FK, CASCADE).
--   * status is 'created', 'confirmed', or 'abandoned' (CHECK).
--   * A cart item references a resolved retailer product (retailer_product_id NOT NULL).
-- Invariants enforced in the application layer, NOT here:
--   * Cart creation calls the adapter's POST /shopping-lists/:id/to-cart and records the
--     resolved items as the checkpoint (design D3; task 3.2).


-- ── Shopping carts (to-cart call checkpoint, v1) ────────────────────────────

-- Records that a to-cart call was made for a binding. `status` tracks the cart's
-- lifecycle: 'created' (to-cart call made) -> 'confirmed' (an order was confirmed
-- from it) or 'abandoned' (no order).
CREATE TABLE shopping_cart (
    id BIGSERIAL PRIMARY KEY,
    retailer_list_binding_id BIGINT NOT NULL REFERENCES retailer_list_binding(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    status TEXT NOT NULL DEFAULT 'created' CHECK (status IN ('created','confirmed','abandoned'))
);

-- The resolved items at the moment of the to-cart call — a snapshot for order
-- reconciliation, not live cart state. `retailer_product_id` is the adapter's
-- product code; the cart is retailer-specific (unlike the retailer-independent
-- shopping_list). `resolved_price` is the price at that moment (NULL if unknown).
CREATE TABLE shopping_cart_item (
    id BIGSERIAL PRIMARY KEY,
    shopping_cart_id BIGINT NOT NULL REFERENCES shopping_cart(id) ON DELETE CASCADE,
    retailer_product_id TEXT NOT NULL,
    quantity DOUBLE PRECISION NOT NULL,
    unit TEXT NOT NULL,
    resolved_price DOUBLE PRECISION
);
CREATE INDEX ON shopping_cart_item (shopping_cart_id);

-- +goose Down
DROP TABLE IF EXISTS shopping_cart_item CASCADE;
DROP TABLE IF EXISTS shopping_cart CASCADE;
