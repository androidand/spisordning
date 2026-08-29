-- +goose Up
-- establish-config-di-and-presentation-layer: store a retailer's manually-
-- refreshed elevated-auth credential so it can be uploaded from wherever the
-- login actually happens (e.g. a Mac, for ICA's Playwright-driven ecom login
-- — see docs/infrastructure/ica-elevated-auth.md) and fetched by the adapter
-- that actually needs it, without assuming they share a filesystem/volume.
--
-- One row per (retailer, tier) — uploading again overwrites the previous
-- credential (there is exactly one "current" elevated session per retailer,
-- not a history of them). payload is opaque JSON from spisordning's point of
-- view (e.g. ICA's ImportedCookie[] shape) — Go never inspects its contents,
-- only stores and serves it back.

CREATE TABLE retailer_credential (
    retailer    TEXT NOT NULL,
    tier        TEXT NOT NULL DEFAULT 'elevated' CHECK (tier IN ('elevated')),
    payload     JSONB NOT NULL,
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (retailer, tier)
);

-- +goose Down
DROP TABLE IF EXISTS retailer_credential;
