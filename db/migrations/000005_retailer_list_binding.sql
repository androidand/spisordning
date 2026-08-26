-- +goose Up
-- Retailer list binding (a shopping_list projected onto an external retailer list).
--
-- This migration adds the table that records spisordning's outbound projection of a
-- shopping_list onto a retailer's own list (see
-- openspec/changes/implement-shopping-and-commerce/design.md, "Retailer list binding (v1)"):
--   retailer_list_binding  which external list (e.g. a Willys wishlist id) a shopping_list
--                          maps to, plus the last push time/status so staleness is inspectable
--
-- The spisordning shopping_list is authoritative for intent; the retailer's list is a
-- synchronized projection (design D2). v1 is outbound-only: sync_direction is 'outbound' and
-- the CHECK encodes that until a future two-way change widens it. The adapter's push is
-- additive (re-push increments quantities), NOT idempotent — see design.md "Push semantics".
--
-- Invariants enforced here (schema-level):
--   * A binding belongs to exactly one shopping_list (shopping_list_id FK, CASCADE).
--   * A list has at most one binding per retailer (UNIQUE (shopping_list_id, retailer)).
--   * sync_direction is 'outbound' in v1 (CHECK).
--   * last_push_status, when set, is 'success' or 'error' (CHECK).
-- Invariants enforced in the application layer, NOT here:
--   * Pushing calls the adapter's POST /shopping-lists and records the returned wishlistId
--     and outcome (design D2; task 2.2).


-- ── Retailer list bindings (outbound projection, v1) ────────────────────────

-- Records that a shopping_list has been projected onto a specific external retailer
-- list. `external_list_id` is the adapter's wishlistId (a string). `last_pushed_at`
-- is NULL until the first successful push; `last_push_status` is NULL until the first
-- attempt. One binding per (list, retailer); re-pushing updates the same row.
CREATE TABLE retailer_list_binding (
    id               BIGSERIAL PRIMARY KEY,
    shopping_list_id BIGINT NOT NULL REFERENCES shopping_list(id) ON DELETE CASCADE,
    retailer         TEXT NOT NULL,
    external_list_id TEXT NOT NULL,
    -- v1 is outbound-only (D2); a future two-way change widens this CHECK.
    sync_direction   TEXT NOT NULL DEFAULT 'outbound' CHECK (sync_direction IN ('outbound')),
    last_pushed_at   TIMESTAMPTZ,
    last_push_status TEXT CHECK (last_push_status IN ('success','error')),
    UNIQUE (shopping_list_id, retailer)
);

-- +goose Down
DROP TABLE IF EXISTS retailer_list_binding CASCADE;
