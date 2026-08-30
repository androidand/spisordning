-- +goose Up
-- Apple Notes outbound sync: item-level match key + resolved timestamp.
--
-- Backs the notes-bridge write-back (openspec/changes/expose-shopping-price-
-- and-notes-bridge, group 7 / design D6-D7). When a from-checklist list is
-- pushed to a retailer wishlist, the items that resolved confidently are
-- stamped resolved_at so the Mac-local bridge can check them off on the
-- household's Apple Note.
--
--   note_match_key  the normalized label (lowercased, trimmed, whitespace-
--                   collapsed) captured at ingestion. It mirrors the notes-
--                   sync bridge's normalizeKey so the outbound write finds the
--                   same checklist line again by a plain equality — no fuzzy
--                   matching of the live note. NULL for items not ingested
--                   from a checklist.
--   resolved_at     when the item was last confidently resolved and pushed to
--                   a retailer wishlist (NULL = not yet resolved).
--
-- Both columns are additive and nullable; no existing row changes.

ALTER TABLE shopping_list_item
    ADD COLUMN IF NOT EXISTS note_match_key TEXT,
    ADD COLUMN IF NOT EXISTS resolved_at TIMESTAMPTZ;

-- Cheap "resolved since last sync" reads for the polling bridge.
CREATE INDEX IF NOT EXISTS idx_shopping_list_item_resolved
    ON shopping_list_item (shopping_list_id, resolved_at)
    WHERE resolved_at IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_shopping_list_item_resolved;
ALTER TABLE shopping_list_item
    DROP COLUMN IF EXISTS note_match_key,
    DROP COLUMN IF EXISTS resolved_at;
