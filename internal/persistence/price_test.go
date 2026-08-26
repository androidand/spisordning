package persistence

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/androidand/spisordning/internal/domain"
)

func TestPrice_RetailerAndStore(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "price_observation", "store_product_offer", "retailer_product", "product", "store", "retailer")

	// Create retailer.
	retailer := domain.Retailer{ID: "r-willys", Name: "Willys"}
	if err := s.CreateRetailer(ctx, retailer); err != nil {
		t.Fatalf("CreateRetailer: %v", err)
	}
	got, err := s.GetRetailer(ctx, "r-willys")
	if err != nil {
		t.Fatalf("GetRetailer: %v", err)
	}
	if got.Name != "Willys" {
		t.Errorf("GetRetailer = %+v", got)
	}

	// Create store.
	store := domain.Store{ID: "s-lindhagen", RetailerID: "r-willys", Name: "Willys Lindhagen"}
	if err := s.CreateStore(ctx, store); err != nil {
		t.Fatalf("CreateStore: %v", err)
	}
	gotStore, err := s.GetStore(ctx, "s-lindhagen")
	if err != nil {
		t.Fatalf("GetStore: %v", err)
	}
	if gotStore.RetailerID != "r-willys" || gotStore.Name != "Willys Lindhagen" {
		t.Errorf("GetStore = %+v", gotStore)
	}

	// List stores for retailer.
	stores, err := s.ListStores(ctx, "r-willys")
	if err != nil {
		t.Fatalf("ListStores: %v", err)
	}
	if len(stores) != 1 || stores[0].ID != "s-lindhagen" {
		t.Errorf("ListStores = %v", stores)
	}

	// List retailers.
	retailers, err := s.ListRetailers(ctx)
	if err != nil {
		t.Fatalf("ListRetailers: %v", err)
	}
	if len(retailers) != 1 || retailers[0].ID != "r-willys" {
		t.Errorf("ListRetailers = %v", retailers)
	}
}

func TestPrice_RetailerProduct(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "price_observation", "store_product_offer", "retailer_product", "product", "store", "retailer")

	// Seed retailer + store + product.
	if err := s.CreateRetailer(ctx, domain.Retailer{ID: "r-willys", Name: "Willys"}); err != nil {
		t.Fatalf("CreateRetailer: %v", err)
	}
	if err := s.CreateStore(ctx, domain.Store{ID: "s-lind", RetailerID: "r-willys", Name: "Lindhagen"}); err != nil {
		t.Fatalf("CreateStore: %v", err)
	}
	if err := s.CreateProduct(ctx, Product{ID: "p-milk", Name: "Arla Standardmjölk 1L"}); err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}

	// Create unmapped retailer product.
	rp := domain.RetailerProduct{ID: "rp-1", RetailerID: "r-willys", RetailerSKU: "123456", DisplayName: "Arla mjölk"}
	if err := s.UpsertRetailerProduct(ctx, rp); err != nil {
		t.Fatalf("UpsertRetailerProduct: %v", err)
	}
	got, err := s.GetRetailerProduct(ctx, "rp-1")
	if err != nil {
		t.Fatalf("GetRetailerProduct: %v", err)
	}
	if got.ProductID != "" {
		t.Errorf("expected unmapped product_id, got %q", got.ProductID)
	}

	// Map it to a product.
	rp.ProductID = "p-milk"
	if err := s.UpsertRetailerProduct(ctx, rp); err != nil {
		t.Fatalf("UpsertRetailerProduct (mapped): %v", err)
	}
	got, err = s.GetRetailerProduct(ctx, "rp-1")
	if err != nil {
		t.Fatalf("GetRetailerProduct (mapped): %v", err)
	}
	if got.ProductID != "p-milk" {
		t.Errorf("expected mapped product_id = p-milk, got %q", got.ProductID)
	}

	// List retailer products.
	rps, err := s.ListRetailerProducts(ctx, "r-willys")
	if err != nil {
		t.Fatalf("ListRetailerProducts: %v", err)
	}
	if len(rps) != 1 || rps[0].RetailerSKU != "123456" {
		t.Errorf("ListRetailerProducts = %v", rps)
	}
}

func TestPrice_StoreProductOffer(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "price_observation", "store_product_offer", "retailer_product", "product", "store", "retailer")

	if err := s.CreateRetailer(ctx, domain.Retailer{ID: "r-willys", Name: "Willys"}); err != nil {
		t.Fatalf("CreateRetailer: %v", err)
	}
	if err := s.CreateStore(ctx, domain.Store{ID: "s-lind", RetailerID: "r-willys", Name: "Lindhagen"}); err != nil {
		t.Fatalf("CreateStore: %v", err)
	}
	if err := s.CreateProduct(ctx, Product{ID: "p-milk", Name: "Arla Standardmjölk 1L"}); err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	if err := s.UpsertRetailerProduct(ctx, domain.RetailerProduct{ID: "rp-1", RetailerID: "r-willys", RetailerSKU: "123456"}); err != nil {
		t.Fatalf("UpsertRetailerProduct: %v", err)
	}

	// Upsert offer (carried).
	offerID, err := s.UpsertStoreProductOffer(ctx, domain.StoreProductOffer{StoreID: "s-lind", RetailerProductID: "rp-1", CurrentlyCarried: true})
	if err != nil {
		t.Fatalf("UpsertStoreProductOffer: %v", err)
	}
	got, err := s.GetStoreProductOffer(ctx, offerID)
	if err != nil {
		t.Fatalf("GetStoreProductOffer: %v", err)
	}
	if !got.CurrentlyCarried {
		t.Errorf("expected carried=true")
	}

	// Mark as not carried (upsert on same row).
	if _, err := s.UpsertStoreProductOffer(ctx, domain.StoreProductOffer{StoreID: "s-lind", RetailerProductID: "rp-1", CurrentlyCarried: false}); err != nil {
		t.Fatalf("UpsertStoreProductOffer (not carried): %v", err)
	}
	got, err = s.GetStoreProductOffer(ctx, offerID)
	if err != nil {
		t.Fatalf("GetStoreProductOffer (not carried): %v", err)
	}
	if got.CurrentlyCarried {
		t.Errorf("expected carried=false")
	}

	// List offers.
	offers, err := s.ListStoreProductOffers(ctx, "s-lind")
	if err != nil {
		t.Fatalf("ListStoreProductOffers: %v", err)
	}
	if len(offers) != 1 || offers[0].CurrentlyCarried {
		t.Errorf("ListStoreProductOffers = %v", offers)
	}
}

func TestPrice_StoreProductOfferMultiple(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "price_observation", "store_product_offer", "retailer_product", "product", "store", "retailer")

	if err := s.CreateRetailer(ctx, domain.Retailer{ID: "r-willys", Name: "Willys"}); err != nil {
		t.Fatalf("CreateRetailer: %v", err)
	}
	if err := s.CreateStore(ctx, domain.Store{ID: "s-lind", RetailerID: "r-willys", Name: "Lindhagen"}); err != nil {
		t.Fatalf("CreateStore: %v", err)
	}
	if err := s.CreateProduct(ctx, Product{ID: "p-milk", Name: "Arla Standardmjölk 1L"}); err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	if err := s.CreateProduct(ctx, Product{ID: "p-bread", Name: "Rugbrød"}); err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	if err := s.UpsertRetailerProduct(ctx, domain.RetailerProduct{ID: "rp-milk", RetailerID: "r-willys", ProductID: "p-milk", RetailerSKU: "111"}); err != nil {
		t.Fatalf("UpsertRetailerProduct milk: %v", err)
	}
	if err := s.UpsertRetailerProduct(ctx, domain.RetailerProduct{ID: "rp-bread", RetailerID: "r-willys", ProductID: "p-bread", RetailerSKU: "222"}); err != nil {
		t.Fatalf("UpsertRetailerProduct bread: %v", err)
	}

	// Two different retailer products at the same store — each gets its own
	// BIGSERIAL id; neither should collide.
	offerID1, err := s.UpsertStoreProductOffer(ctx, domain.StoreProductOffer{StoreID: "s-lind", RetailerProductID: "rp-milk", CurrentlyCarried: true})
	if err != nil {
		t.Fatalf("UpsertStoreProductOffer milk: %v", err)
	}
	offerID2, err := s.UpsertStoreProductOffer(ctx, domain.StoreProductOffer{StoreID: "s-lind", RetailerProductID: "rp-bread", CurrentlyCarried: true})
	if err != nil {
		t.Fatalf("UpsertStoreProductOffer bread: %v", err)
	}
	if offerID1 == offerID2 {
		t.Errorf("expected different ids for different offers, got %d", offerID1)
	}
	if offerID1 == 0 || offerID2 == 0 {
		t.Errorf("expected non-zero ids, got %d and %d", offerID1, offerID2)
	}

	offers, err := s.ListStoreProductOffers(ctx, "s-lind")
	if err != nil {
		t.Fatalf("ListStoreProductOffers: %v", err)
	}
	if len(offers) != 2 {
		t.Fatalf("expected 2 offers, got %d", len(offers))
	}

	// Each offer should still be independently updatable.
	if _, err := s.UpsertStoreProductOffer(ctx, domain.StoreProductOffer{StoreID: "s-lind", RetailerProductID: "rp-milk", CurrentlyCarried: false}); err != nil {
		t.Fatalf("UpsertStoreProductOffer milk not carried: %v", err)
	}
	got, err := s.GetStoreProductOffer(ctx, offerID1)
	if err != nil {
		t.Fatalf("GetStoreProductOffer: %v", err)
	}
	if got.CurrentlyCarried {
		t.Errorf("expected milk not carried")
	}
	got2, err := s.GetStoreProductOffer(ctx, offerID2)
	if err != nil {
		t.Fatalf("GetStoreProductOffer bread: %v", err)
	}
	if !got2.CurrentlyCarried {
		t.Errorf("expected bread still carried")
	}
}

func TestPrice_PriceObservationAppendOnly(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "price_observation", "store_product_offer", "retailer_product", "product", "store", "retailer")

	if err := s.CreateRetailer(ctx, domain.Retailer{ID: "r-willys", Name: "Willys"}); err != nil {
		t.Fatalf("CreateRetailer: %v", err)
	}
	if err := s.CreateStore(ctx, domain.Store{ID: "s-lind", RetailerID: "r-willys", Name: "Lindhagen"}); err != nil {
		t.Fatalf("CreateStore: %v", err)
	}
	if err := s.CreateProduct(ctx, Product{ID: "p-milk", Name: "Arla Standardmjölk 1L"}); err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	if err := s.UpsertRetailerProduct(ctx, domain.RetailerProduct{ID: "rp-1", RetailerID: "r-willys", RetailerSKU: "123456"}); err != nil {
		t.Fatalf("UpsertRetailerProduct: %v", err)
	}
	offerID, err := s.UpsertStoreProductOffer(ctx, domain.StoreProductOffer{StoreID: "s-lind", RetailerProductID: "rp-1", CurrentlyCarried: true})
	if err != nil {
		t.Fatalf("UpsertStoreProductOffer: %v", err)
	}

	now := time.Now().Truncate(time.Second)

	// Insert first observation (regular price).
	obs1 := domain.PriceObservation{StoreProductOfferID: offerID, ObservedAt: now, Price: 24.90, PriceKind: domain.PriceKindRegular, Source: "willys_adapter"}
	if err := s.InsertPriceObservation(ctx, obs1); err != nil {
		t.Fatalf("InsertPriceObservation 1: %v", err)
	}

	// Insert second observation (price dropped).
	obs2 := domain.PriceObservation{StoreProductOfferID: offerID, ObservedAt: now.Add(time.Hour), Price: 19.90, PriceKind: domain.PriceKindRegular, Source: "willys_adapter"}
	if err := s.InsertPriceObservation(ctx, obs2); err != nil {
		t.Fatalf("InsertPriceObservation 2: %v", err)
	}

	// Insert member price at same time as obs2.
	obs3 := domain.PriceObservation{StoreProductOfferID: offerID, ObservedAt: now.Add(time.Hour), Price: 17.90, PriceKind: domain.PriceKindMember, Source: "willys_adapter"}
	if err := s.InsertPriceObservation(ctx, obs3); err != nil {
		t.Fatalf("InsertPriceObservation 3: %v", err)
	}

	// All three rows should exist (append-only, never updated).
	all, err := s.ListPriceObservations(ctx, offerID)
	if err != nil {
		t.Fatalf("ListPriceObservations: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 observations, got %d", len(all))
	}

	// Latest regular price.
	latest, err := s.GetLatestPriceObservation(ctx, offerID, domain.PriceKindRegular)
	if err != nil {
		t.Fatalf("GetLatestPriceObservation: %v", err)
	}
	if latest.Price != 19.90 {
		t.Errorf("expected latest regular price 19.90, got %v", latest.Price)
	}

	// Latest member price.
	latestMember, err := s.GetLatestPriceObservation(ctx, offerID, domain.PriceKindMember)
	if err != nil {
		t.Fatalf("GetLatestPriceObservation member: %v", err)
	}
	if latestMember.Price != 17.90 {
		t.Errorf("expected latest member price 17.90, got %v", latestMember.Price)
	}

	// No campaign price exists.
	_, err = s.GetLatestPriceObservation(ctx, offerID, domain.PriceKindCampaign)
	if err == nil {
		t.Error("expected error for missing campaign price")
	}
}

func TestPrice_CurrentPriceView(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "price_observation", "store_product_offer", "retailer_product", "product", "store", "retailer")

	if err := s.CreateRetailer(ctx, domain.Retailer{ID: "r-willys", Name: "Willys"}); err != nil {
		t.Fatalf("CreateRetailer: %v", err)
	}
	if err := s.CreateStore(ctx, domain.Store{ID: "s-lind", RetailerID: "r-willys", Name: "Lindhagen"}); err != nil {
		t.Fatalf("CreateStore: %v", err)
	}
	if err := s.CreateProduct(ctx, Product{ID: "p-milk", Name: "Arla Standardmjölk 1L"}); err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	if err := s.UpsertRetailerProduct(ctx, domain.RetailerProduct{ID: "rp-1", RetailerID: "r-willys", RetailerSKU: "123456"}); err != nil {
		t.Fatalf("UpsertRetailerProduct: %v", err)
	}
	offerID, err := s.UpsertStoreProductOffer(ctx, domain.StoreProductOffer{StoreID: "s-lind", RetailerProductID: "rp-1", CurrentlyCarried: true})
	if err != nil {
		t.Fatalf("UpsertStoreProductOffer: %v", err)
	}

	now := time.Now().Truncate(time.Second)
	if err := s.InsertPriceObservation(ctx, domain.PriceObservation{StoreProductOfferID: offerID, ObservedAt: now, Price: 24.90, PriceKind: domain.PriceKindRegular, Source: "willys_adapter"}); err != nil {
		t.Fatalf("InsertPriceObservation: %v", err)
	}
	if err := s.InsertPriceObservation(ctx, domain.PriceObservation{StoreProductOfferID: offerID, ObservedAt: now, Price: 19.90, PriceKind: domain.PriceKindMember, Source: "willys_adapter"}); err != nil {
		t.Fatalf("InsertPriceObservation member: %v", err)
	}

	prices, err := s.ListCurrentPrices(ctx)
	if err != nil {
		t.Fatalf("ListCurrentPrices: %v", err)
	}
	if len(prices) != 2 {
		t.Fatalf("expected 2 current prices, got %d", len(prices))
	}
	// View returns one row per (offer, price_kind) with the latest observation.
	for _, p := range prices {
		if p.OfferID != offerID {
			t.Errorf("expected offer_id=%d, got %d", offerID, p.OfferID)
		}
	}
}

func TestPrice_PriceObservationNeverUpdated(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "price_observation", "store_product_offer", "retailer_product", "product", "store", "retailer")

	if err := s.CreateRetailer(ctx, domain.Retailer{ID: "r-willys", Name: "Willys"}); err != nil {
		t.Fatalf("CreateRetailer: %v", err)
	}
	if err := s.CreateStore(ctx, domain.Store{ID: "s-lind", RetailerID: "r-willys", Name: "Lindhagen"}); err != nil {
		t.Fatalf("CreateStore: %v", err)
	}
	if err := s.CreateProduct(ctx, Product{ID: "p-milk", Name: "Arla Standardmjölk 1L"}); err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	if err := s.UpsertRetailerProduct(ctx, domain.RetailerProduct{ID: "rp-1", RetailerID: "r-willys", RetailerSKU: "123456"}); err != nil {
		t.Fatalf("UpsertRetailerProduct: %v", err)
	}
	offerID, err := s.UpsertStoreProductOffer(ctx, domain.StoreProductOffer{StoreID: "s-lind", RetailerProductID: "rp-1", CurrentlyCarried: true})
	if err != nil {
		t.Fatalf("UpsertStoreProductOffer: %v", err)
	}

	now := time.Now().Truncate(time.Second)
	// Insert observation.
	if err := s.InsertPriceObservation(ctx, domain.PriceObservation{StoreProductOfferID: offerID, ObservedAt: now, Price: 24.90, PriceKind: domain.PriceKindRegular, Source: "willys_adapter"}); err != nil {
		t.Fatalf("InsertPriceObservation: %v", err)
	}

	// Verify the price is still the original — if any persistence method had
	// updated it, the price would have changed.
	obs, err := s.GetLatestPriceObservation(ctx, offerID, domain.PriceKindRegular)
	if err != nil {
		t.Fatalf("GetLatestPriceObservation: %v", err)
	}
	if obs.Price != 24.90 {
		t.Errorf("price was mutated: expected 24.90, got %v", obs.Price)
	}

	// The append-only invariant is enforced at the application layer:
	// no persistence method issues an UPDATE on price_observation.
	src, err := os.ReadFile("price.go")
	if err != nil {
		t.Fatalf("read price.go: %v", err)
	}
	srcStr := string(src)
	if strings.Contains(srcStr, "UPDATE price_observation") || strings.Contains(srcStr, "update price_observation") {
		t.Error("price.go contains an UPDATE on price_observation; the append-only invariant requires INSERT-only writes")
	}
}

func TestPrice_PriceObservationsForProduct(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "price_observation", "store_product_offer", "retailer_product", "product", "store", "retailer")

	if err := s.CreateRetailer(ctx, domain.Retailer{ID: "r-willys", Name: "Willys"}); err != nil {
		t.Fatalf("CreateRetailer: %v", err)
	}
	if err := s.CreateStore(ctx, domain.Store{ID: "s-lind", RetailerID: "r-willys", Name: "Lindhagen"}); err != nil {
		t.Fatalf("CreateStore: %v", err)
	}
	if err := s.CreateProduct(ctx, Product{ID: "p-milk", Name: "Arla Standardmjölk 1L"}); err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	if err := s.UpsertRetailerProduct(ctx, domain.RetailerProduct{ID: "rp-1", RetailerID: "r-willys", RetailerSKU: "123456"}); err != nil {
		t.Fatalf("UpsertRetailerProduct: %v", err)
	}
	offerID, err := s.UpsertStoreProductOffer(ctx, domain.StoreProductOffer{StoreID: "s-lind", RetailerProductID: "rp-1", CurrentlyCarried: true})
	if err != nil {
		t.Fatalf("UpsertStoreProductOffer: %v", err)
	}

	now := time.Now().Truncate(time.Second)
	if err := s.InsertPriceObservation(ctx, domain.PriceObservation{StoreProductOfferID: offerID, ObservedAt: now, Price: 24.90, PriceKind: domain.PriceKindRegular, Source: "willys_adapter"}); err != nil {
		t.Fatalf("InsertPriceObservation: %v", err)
	}
	if err := s.InsertPriceObservation(ctx, domain.PriceObservation{StoreProductOfferID: offerID, ObservedAt: now.Add(time.Hour), Price: 19.90, PriceKind: domain.PriceKindRegular, Source: "willys_adapter"}); err != nil {
		t.Fatalf("InsertPriceObservation 2: %v", err)
	}

	obs, err := s.PriceObservationsForProduct(ctx, "rp-1")
	if err != nil {
		t.Fatalf("PriceObservationsForProduct: %v", err)
	}
	if len(obs) != 2 {
		t.Fatalf("expected 2 observations, got %d", len(obs))
	}
	if obs[0].Price != 19.90 {
		t.Errorf("expected latest first, got %v", obs[0].Price)
	}
}

func TestPrice_PriceObservationsForStore(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "price_observation", "store_product_offer", "retailer_product", "product", "store", "retailer")

	if err := s.CreateRetailer(ctx, domain.Retailer{ID: "r-willys", Name: "Willys"}); err != nil {
		t.Fatalf("CreateRetailer: %v", err)
	}
	if err := s.CreateStore(ctx, domain.Store{ID: "s-lind", RetailerID: "r-willys", Name: "Lindhagen"}); err != nil {
		t.Fatalf("CreateStore: %v", err)
	}
	if err := s.CreateProduct(ctx, Product{ID: "p-milk", Name: "Arla Standardmjölk 1L"}); err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	if err := s.UpsertRetailerProduct(ctx, domain.RetailerProduct{ID: "rp-1", RetailerID: "r-willys", RetailerSKU: "123456"}); err != nil {
		t.Fatalf("UpsertRetailerProduct: %v", err)
	}
	offerID, err := s.UpsertStoreProductOffer(ctx, domain.StoreProductOffer{StoreID: "s-lind", RetailerProductID: "rp-1", CurrentlyCarried: true})
	if err != nil {
		t.Fatalf("UpsertStoreProductOffer: %v", err)
	}

	now := time.Now().Truncate(time.Second)
	if err := s.InsertPriceObservation(ctx, domain.PriceObservation{StoreProductOfferID: offerID, ObservedAt: now, Price: 24.90, PriceKind: domain.PriceKindRegular, Source: "willys_adapter"}); err != nil {
		t.Fatalf("InsertPriceObservation: %v", err)
	}

	obs, err := s.PriceObservationsForStore(ctx, "s-lind")
	if err != nil {
		t.Fatalf("PriceObservationsForStore: %v", err)
	}
	if len(obs) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(obs))
	}
	if obs[0].Price != 24.90 {
		t.Errorf("expected price 24.90, got %v", obs[0].Price)
	}
}
