-- name: ListRetailers :many
SELECT id, slug, name, created_at
FROM retailer
ORDER BY name;

-- name: ListStores :many
SELECT id, slug, retailer_id, name, created_at
FROM store
WHERE retailer_id = $1
ORDER BY name;

-- name: ListAllStores :many
SELECT id, slug, retailer_id, name, created_at
FROM store
ORDER BY retailer_id, name;

-- name: ListRetailerProducts :many
SELECT id, slug, retailer_id, product_id, retailer_sku, display_name, created_at
FROM retailer_product
WHERE retailer_id = $1
ORDER BY display_name;

-- name: ListStoreProductOffers :many
SELECT id, store_id, retailer_product_id, currently_carried, created_at, updated_at
FROM store_product_offer
WHERE store_id = $1
ORDER BY retailer_product_id;

-- name: ListCurrentPrices :many
SELECT spo.id, spo.store_id, spo.retailer_product_id, spo.currently_carried,
       rp.display_name AS product_name,
       s.name AS store_name,
       r.name AS retailer_name,
       po.price, po.price_kind, po.observed_at
FROM store_product_offer spo
JOIN retailer_product rp ON spo.retailer_product_id = rp.id
JOIN store s ON spo.store_id = s.id
JOIN retailer r ON s.retailer_id = r.id
LEFT JOIN price_observation po ON po.store_product_offer_id = spo.id
    AND po.observed_at = (SELECT MAX(observed_at) FROM price_observation WHERE store_product_offer_id = spo.id)
WHERE spo.currently_carried
ORDER BY rp.display_name, s.name;

-- name: PriceObservationsForProduct :many
SELECT po.id, po.store_product_offer_id, po.observed_at, po.price, po.price_kind, po.source
FROM price_observation po
JOIN store_product_offer spo ON po.store_product_offer_id = spo.id
WHERE spo.retailer_product_id = $1
ORDER BY po.observed_at DESC;

-- name: PriceObservationsForStore :many
SELECT po.id, po.store_product_offer_id, po.observed_at, po.price, po.price_kind, po.source
FROM price_observation po
WHERE po.store_product_offer_id IN (
    SELECT id FROM store_product_offer WHERE store_id = $1
)
ORDER BY po.observed_at DESC;
