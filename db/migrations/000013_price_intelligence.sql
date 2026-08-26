-- +goose Up
-- Price intelligence: retailer/store identity, retailer product SKUs, store
-- offers, and an append-only price-observation series.
--
-- implement-price-intelligence (openspec/changes/implement-price-intelligence).
-- Extends migrations/0001_init.sql's style. All tables use CREATE TABLE IF NOT EXISTS
-- for idempotency; the view uses CREATE OR REPLACE VIEW.
--
-- Design decisions:
--   - retailer, store, retailer_product: identity layer, no price data
--   - store_product_offer: mutable assortment fact ("does this store carry this SKU?")
--   - price_observation: append-only, never updated; price is always a derived read
--   - current_store_product_price: read-optimized view, not the source of truth


-- ── 1. Retailer ───────────────────────────────────────────────────────────────
-- A retail chain (ICA, Willys, Coop, ...). One row per chain, not per store.
CREATE TABLE IF NOT EXISTS retailer (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ── 2. Store ──────────────────────────────────────────────────────────────────
-- A specific store within a retailer. Assortment and prices may differ per store.
CREATE TABLE IF NOT EXISTS store (
    id           TEXT PRIMARY KEY,
    retailer_id  TEXT NOT NULL REFERENCES retailer(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_store_retailer ON store (retailer_id);

-- ── 3. Retailer product ───────────────────────────────────────────────────────
-- A retailer's SKU for a canonical Product. Distinct from Product — this is the
-- retailer's view of the product, not the product itself. References Product
-- (owned by establish-household-and-catalog); product_id is nullable because
-- a retailer may list a SKU before the canonical mapping is resolved.
CREATE TABLE IF NOT EXISTS retailer_product (
    id               TEXT PRIMARY KEY,
    retailer_id      TEXT NOT NULL REFERENCES retailer(id) ON DELETE CASCADE,
    product_id       TEXT REFERENCES product(id) ON DELETE SET NULL,
    retailer_sku     TEXT NOT NULL,          -- article number / EAN / SKU as returned by the retailer
    display_name     TEXT,                   -- retailer's display name for this SKU
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (retailer_id, retailer_sku)
);
CREATE INDEX IF NOT EXISTS idx_retailer_product_product ON retailer_product (product_id);
CREATE INDEX IF NOT EXISTS idx_retailer_product_retailer ON retailer_product (retailer_id);

-- ── 4. Store product offer ────────────────────────────────────────────────────
-- Whether a specific store currently carries a specific retailer product.
-- Mutable: assortment genuinely changes (a store may stop carrying an item).
-- currently_carried is the source of truth for "is this available now?"; price
-- history lives separately in price_observation.
CREATE TABLE IF NOT EXISTS store_product_offer (
    id                    BIGSERIAL PRIMARY KEY,
    store_id              TEXT NOT NULL REFERENCES store(id) ON DELETE CASCADE,
    retailer_product_id   TEXT NOT NULL REFERENCES retailer_product(id) ON DELETE CASCADE,
    currently_carried     BOOLEAN NOT NULL DEFAULT true,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (store_id, retailer_product_id)
);
CREATE INDEX IF NOT EXISTS idx_store_product_offer_store ON store_product_offer (store_id);
CREATE INDEX IF NOT EXISTS idx_store_product_offer_product ON store_product_offer (retailer_product_id);

-- ── 5. Price observation ──────────────────────────────────────────────────────
-- Append-only ledger of price readings. Never UPDATEd or DELETEd. Each row is
-- a timestamped, sourced observation of one offer's price at one point in time.
-- "Current price" is derived by reading the latest observation per (offer,
-- price_kind) — see the view below.
CREATE TABLE IF NOT EXISTS price_observation (
    id                    BIGSERIAL PRIMARY KEY,
    store_product_offer_id BIGINT NOT NULL REFERENCES store_product_offer(id) ON DELETE CASCADE,
    observed_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    price                 DOUBLE PRECISION NOT NULL,
    price_kind            TEXT NOT NULL CHECK (price_kind IN ('regular', 'member', 'campaign')),
    source                TEXT NOT NULL,      -- e.g. 'willys_adapter', 'primat', 'matpriskollen', ...
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_price_observation_offer ON price_observation (store_product_offer_id, observed_at DESC);
CREATE INDEX IF NOT EXISTS idx_price_observation_source ON price_observation (source);

-- ── 6. Current price view ─────────────────────────────────────────────────────
-- Read-optimized view exposing the latest observation per (offer, price_kind).
-- Callers query this instead of hand-rolling the latest-per-group logic.
-- Overlapping observations from different sources at the same time are all
-- preserved; the view picks the latest observed_at per (offer, price_kind).
CREATE OR REPLACE VIEW current_store_product_price AS
SELECT DISTINCT ON (spo.id, po.price_kind)
    spo.id            AS offer_id,
    spo.store_id,
    spo.retailer_product_id,
    po.price_kind,
    po.price,
    po.observed_at,
    po.source
FROM store_product_offer spo
JOIN price_observation po ON po.store_product_offer_id = spo.id
ORDER BY spo.id, po.price_kind, po.observed_at DESC, po.id DESC;

-- +goose Down
DROP VIEW IF EXISTS current_store_product_price;
DROP TABLE IF EXISTS price_observation CASCADE;
DROP TABLE IF EXISTS store_product_offer CASCADE;
DROP TABLE IF EXISTS retailer_product CASCADE;
DROP TABLE IF EXISTS store CASCADE;
DROP TABLE IF EXISTS retailer CASCADE;
