-- +goose Up
-- Order (a completed, fidelity-preserving purchase record).
--
-- This migration adds the tables that record spisordning's completed purchase records
-- (see openspec/changes/implement-shopping-and-commerce/design.md, "Order (v1)"):
--   "order"       a completed purchase: which cart (if any), which retailer, source, total
--   order_item    the actual items purchased: product, quantity, prices, substitutions
--
-- An order is a fidelity-preserving record (design D4): it stores the ACTUAL quantity, price,
-- retailer product, and substitutions, per PLAN.md's "Preserve actual: quantity, price,
-- retailer product, substitutions." It is NOT a projection of the cart's checkpoint.
--
-- order.source is an explicit enum ('manual' | 'retailer_api' | 'receipt_import') because no
-- retailer in scope has a real order-history API today (D4). Most orders start as 'manual'
-- (a person confirms what they actually bought) until a retailer API or receipt pipeline exists.
--
-- FORWARD DEPENDENCY (task 4.3): order.id and order_item.id are stable, referenceable ids from
-- day one so a future implement-pantry-inventory change (Epic D) can add an
-- inventory_event(kind='PURCHASE', order_item_id REFERENCES order_item(id)) WITHOUT a migration
-- that touches these tables. This change does not create any inventory_* table itself.
--
-- Invariants enforced here (schema-level):
--   * An order may reference a cart (shopping_cart_id FK, SET NULL) or none (manual/receipt).
--   * source is 'manual', 'retailer_api', or 'receipt_import' (CHECK).
--   * An order item belongs to exactly one order (order_id FK, CASCADE).
--   * An order item references a retailer product (retailer_product_id NOT NULL).
--   * substituted_for_item_id is a self-reference to order_item(id) (nullable).
-- Invariants enforced in the application layer, NOT here:
--   * Manual confirmation pre-fills from the most recent shopping_cart checkpoint and marks the
--     cart 'confirmed' (design "Manual order-confirmation flow"; task 4.2).


-- ── Orders (completed, fidelity-preserving purchase records, v1) ───────────

-- A completed purchase. `shopping_cart_id` is nullable (an order may be confirmed from a cart
-- checkpoint, or entered manually with no preceding cart, e.g. a receipt import). `source` is
-- explicit (D4): 'manual' is the common case until a retailer API or receipt pipeline exists.
CREATE TABLE "order" (
    id UUID PRIMARY KEY,
    shopping_cart_id UUID REFERENCES shopping_cart(id) ON DELETE SET NULL,
    retailer TEXT NOT NULL,
    source TEXT NOT NULL CHECK (source IN ('manual','retailer_api','receipt_import')),
    ordered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    total_price_minor BIGINT,
    currency CHAR(3) NOT NULL DEFAULT 'SEK'
);

-- The actual items purchased (D4): product, quantity, prices, and substitutions.
-- `substituted_for_item_id` is a self-reference set when the retailer substituted a product at
-- fulfillment (the referenced item is the originally-intended product).
CREATE TABLE order_item (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL REFERENCES "order"(id) ON DELETE CASCADE,
    retailer_product_id TEXT NOT NULL,
    quantity numeric(12,3) NOT NULL,
    unit_price numeric(12,3),
    total_price_minor BIGINT,
    currency CHAR(3) NOT NULL DEFAULT 'SEK',
    substituted_for_item_id UUID REFERENCES order_item(id)
);
CREATE INDEX ON order_item (order_id);

-- +goose Down
DROP TABLE IF EXISTS order_item CASCADE;
DROP TABLE IF EXISTS "order" CASCADE;
