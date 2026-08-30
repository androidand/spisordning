// `food-brain sync prices` fetches current offers from a retailer adapter and
// persists them into the price-intelligence tables (task 9.4): it upserts the
// retailer and store identity rows, upserts one retailer_product per offer SKU
// and one store_product_offer per (store, SKU), then appends a price_observation
// per offer. Price observations are append-only — re-running the sync appends a
// new reading, never mutating history.
package main

import (
	"context"
	"flag"
	"fmt"
	"strconv"

	"github.com/androidand/spisordning/internal/config"
	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/retailer"
)

func runSyncPrices(args []string) error {
	fs := flag.NewFlagSet("sync prices", flag.ExitOnError)
	retailerFlag := fs.String("retailer", "ica", "retailer backend: willys or ica")
	storeID := fs.String("store", "", "store id to sync (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *storeID == "" {
		return fmt.Errorf("sync prices: -store is required")
	}

	ctx := context.Background()
	store, err := openStore(ctx)
	if err != nil {
		return err
	}
	if store == nil {
		return fmt.Errorf("sync prices: no database configured (set POSTGRES_PASSWORD or DATABASE_URL)")
	}

	cfg := config.Load()
	kind := retailer.RetailerKind(*retailerFlag)
	rc, err := retailer.NewFromKind(kind, cfg.AdapterURL, cfg.ICAAdapterURL, cfg.HemkopAdapterURL)
	if err != nil {
		return fmt.Errorf("sync prices: %w", err)
	}
	rc.WithAuthFile(cfg.ICAAuthFile)

	offers, err := rc.SyncOffers(ctx, *storeID)
	if err != nil {
		return fmt.Errorf("sync prices: fetch offers: %w", err)
	}
	fmt.Printf("fetched %d offer(s) from %s-adapter (store=%s)\n", len(offers), kind, *storeID)

	// Identity rows: one retailer per chain, one store per location.
	retailerID, err := domain.ParseRetailerID(string(kind))
	if err != nil {
		return fmt.Errorf("sync prices: parse retailer id: %w", err)
	}
	if err := store.UpsertRetailer(ctx, domain.Retailer{ID: retailerID, Name: string(kind)}); err != nil {
		return fmt.Errorf("sync prices: upsert retailer: %w", err)
	}
	storeUUID, err := domain.ParseStoreID(*storeID)
	if err != nil {
		return fmt.Errorf("sync prices: parse store id: %w", err)
	}
	if err := store.UpsertStore(ctx, domain.Store{ID: storeUUID, RetailerID: retailerID, Name: *storeID}); err != nil {
		return fmt.Errorf("sync prices: upsert store: %w", err)
	}

	source := string(kind) + "_adapter"
	for _, o := range offers {
		sku := strconv.Itoa(o.ArticleID)
		rpUUID := domain.NewRetailerProductID()
		rp := domain.RetailerProduct{
			ID:           rpUUID,
			RetailerID:   retailerID,
			RetailerSKU:  sku,
			DisplayName:  o.Name,
		}
		if err := store.UpsertRetailerProduct(ctx, rp); err != nil {
			return fmt.Errorf("sync prices: upsert retailer_product %s: %w", sku, err)
		}
		offerID, err := store.UpsertStoreProductOffer(ctx, domain.StoreProductOffer{
			StoreID:           storeUUID,
			RetailerProductID: rpUUID,
			CurrentlyCarried:  true,
		})
		if err != nil {
			return fmt.Errorf("sync prices: upsert store_product_offer %s: %w", sku, err)
		}
		if err := store.InsertPriceObservation(ctx, domain.PriceObservation{
			StoreProductOfferID: offerID,
			Price:               o.Price,
			PriceKind:           domain.PriceKindRegular,
			Source:              source,
		}); err != nil {
			return fmt.Errorf("sync prices: insert price_observation %s: %w", sku, err)
		}
	}

	fmt.Printf("✅ recorded %d price observation(s) for store %s\n", len(offers), *storeID)
	return nil
}
