-- +goose Up
-- Store geo: latitude/longitude for the store locator.
--
-- The store table (000013_price_intelligence.sql) carried only identity
-- (id, retailer_id, name). The store-locator use case — "closest store",
-- "cheapest store near me" — needs a position per store. Both columns are
-- nullable: a store may be known (and carry prices) without a mapped
-- location, and the locator simply skips geo-less stores when ranking by
-- distance.
--
-- WGS84 decimal degrees. No spatial index yet — the household's store set is
-- tiny (dozens, not thousands), so a plain scan + in-process haversine is the
-- right tool; a PostGIS index would be over-engineering at this scale.

ALTER TABLE store
    ADD COLUMN IF NOT EXISTS latitude  DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS longitude DOUBLE PRECISION;

-- +goose Down
ALTER TABLE store
    DROP COLUMN IF EXISTS latitude,
    DROP COLUMN IF EXISTS longitude;
