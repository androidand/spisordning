package persistence

import (
	"context"
	"testing"

	"github.com/androidand/spisordning/internal/domain"
)

// pantryFixture sets up a household, an ingredient, and a product mapped to
// it, returning their ids. Shared setup for every test in this file.
func pantryFixture(t *testing.T, ctx context.Context, s *Store, suffix string) (householdID domain.HouseholdID, ingredientID domain.IngredientID, productID domain.ProductID) {
	t.Helper()
	householdID = domain.NewHouseholdID()
	ingredientID = domain.NewIngredientID()
	productID = domain.NewProductID()

	if err := s.CreateHousehold(ctx, Household{ID: householdID, Name: "Pantry Test Household"}); err != nil {
		t.Fatalf("CreateHousehold: %v", err)
	}
	if err := s.UpsertIngredient(ctx, Ingredient{ID: ingredientID, Display: "mjölk"}); err != nil {
		t.Fatalf("UpsertIngredient: %v", err)
	}
	if err := s.CreateProduct(ctx, Product{ID: productID, Name: "Arla Mjölk 1L"}); err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	return householdID, ingredientID, productID
}

func TestPantry_GraduatedSpecificity(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "inventory_event", "inventory_lot", "inventory_location", "product", "household")
	householdID, ingredientID, productID := pantryFixture(t, ctx, s, "specificity")

	loc := InventoryLocation{ID: domain.NewInventoryLocationID(), HouseholdID: householdID, Name: "Fridge"}
	if err := s.CreateInventoryLocation(ctx, loc); err != nil {
		t.Fatalf("CreateInventoryLocation: %v", err)
	}

	// Manual quick entry: no productID, ingredient-only lot.
	lotID, err := s.RecordPurchase(ctx, ingredientID, nil, loc.ID, 1, "l", nil, "manual_count")
	if err != nil {
		t.Fatalf("RecordPurchase (ingredient-only): %v", err)
	}
	lot, err := s.GetInventoryLot(ctx, lotID)
	if err != nil {
		t.Fatalf("GetInventoryLot: %v", err)
	}
	if lot.ProductID != nil {
		t.Errorf("expected ingredient-only lot to have no product, got %q", lot.ProductID.String())
	}
	if lot.Confidence != domain.ConfidenceExact {
		t.Errorf("expected EXACT confidence for a counted purchase, got %s", lot.Confidence)
	}

	// Refine to a specific product; quantity/location/confidence unchanged.
	if err := s.RefineLotProduct(ctx, lotID, productID); err != nil {
		t.Fatalf("RefineLotProduct: %v", err)
	}
	refined, err := s.GetInventoryLot(ctx, lotID)
	if err != nil {
		t.Fatalf("GetInventoryLot after refine: %v", err)
	}
	if refined.ProductID == nil || *refined.ProductID != productID {
		t.Errorf("expected refined lot to reference product %q, got %v", productID, refined.ProductID)
	}
	if refined.Quantity != lot.Quantity || refined.LocationID != lot.LocationID || refined.Confidence != lot.Confidence {
		t.Errorf("RefineLotProduct changed quantity/location/confidence: before %+v after %+v", lot, refined)
	}

	// shopping_order source requires a productID.
	if _, err := s.RecordPurchase(ctx, ingredientID, nil, loc.ID, 1, "l", nil, sourceShoppingOrder); err == nil {
		t.Error("expected RecordPurchase to reject shopping_order source without a productID")
	}
	orderLotID, err := s.RecordPurchase(ctx, ingredientID, &productID, loc.ID, 1, "l", nil, sourceShoppingOrder)
	if err != nil {
		t.Fatalf("RecordPurchase (shopping_order): %v", err)
	}
	orderLot, err := s.GetInventoryLot(ctx, orderLotID)
	if err != nil {
		t.Fatalf("GetInventoryLot (order lot): %v", err)
	}
	if orderLot.ProductID == nil || *orderLot.ProductID != productID {
		t.Errorf("expected shopping_order lot to carry productID immediately, got %v", orderLot.ProductID)
	}

	// home_prepared source creates an ingredient-only lot, same as manual quick entry.
	homeLotID, err := s.RecordPurchase(ctx, ingredientID, nil, loc.ID, 1, "portion", nil, "home_prepared")
	if err != nil {
		t.Fatalf("RecordPurchase (home_prepared): %v", err)
	}
	homeLot, err := s.GetInventoryLot(ctx, homeLotID)
	if err != nil {
		t.Fatalf("GetInventoryLot (home lot): %v", err)
	}
	if homeLot.ProductID != nil {
		t.Errorf("expected home_prepared lot to have no product, got %q", homeLot.ProductID.String())
	}
	if homeLot.Confidence != domain.ConfidenceExact {
		t.Errorf("expected EXACT confidence for home_prepared purchase, got %s", homeLot.Confidence)
	}
}

func TestPantry_ListCandidateProductsForIngredient(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "product_ingredient_mapping", "product", "household")
	_, ingredientID, productID := pantryFixture(t, ctx, s, "candidates")

	// No mapping yet, but the name matches — falls back to name-match search.
	unrelated := Product{ID: domain.NewProductID(), Name: "Chips"}
	if err := s.CreateProduct(ctx, unrelated); err != nil {
		t.Fatalf("CreateProduct (unrelated): %v", err)
	}
	byName, err := s.ListCandidateProductsForIngredient(ctx, ingredientID)
	if err != nil {
		t.Fatalf("ListCandidateProductsForIngredient (name-match): %v", err)
	}
	foundByName := false
	for _, p := range byName {
		if p.ID == unrelated.ID {
			t.Errorf("unrelated product %q should not match ingredient %q by name", unrelated.ID, ingredientID)
		}
		if p.ID == productID {
			foundByName = true
		}
	}
	if !foundByName {
		t.Errorf("expected name-match fallback to find %q (name contains ingredient display), got %v", productID.String(), byName)
	}

	// Once mapped, the mapped set takes priority (and excludes the unrelated product).
	if err := s.SetProductIngredientMapping(ctx, ProductIngredientMapping{ProductID: productID, IngredientID: ingredientID}); err != nil {
		t.Fatalf("SetProductIngredientMapping: %v", err)
	}
	mapped, err := s.ListCandidateProductsForIngredient(ctx, ingredientID)
	if err != nil {
		t.Fatalf("ListCandidateProductsForIngredient (mapped): %v", err)
	}
	if len(mapped) != 1 || mapped[0].ID != productID {
		t.Errorf("ListCandidateProductsForIngredient (mapped) = %v, want only [%s]", mapped, productID)
	}
}

func TestPantry_EventLifecycle(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "inventory_event", "inventory_lot", "inventory_location", "product", "household")
	householdID, ingredientID, _ := pantryFixture(t, ctx, s, "events")

	loc := InventoryLocation{ID: domain.NewInventoryLocationID(), HouseholdID: householdID, Name: "Fridge"}
	if err := s.CreateInventoryLocation(ctx, loc); err != nil {
		t.Fatalf("CreateInventoryLocation: %v", err)
	}

	lotID, err := s.RecordPurchase(ctx, ingredientID, nil, loc.ID, 10, "portion", nil, "manual_count")
	if err != nil {
		t.Fatalf("RecordPurchase: %v", err)
	}

	if err := s.RecordConsume(ctx, lotID, 3, false, "manual_count"); err != nil {
		t.Fatalf("RecordConsume: %v", err)
	}
	afterConsume, err := s.GetInventoryLot(ctx, lotID)
	if err != nil {
		t.Fatalf("GetInventoryLot after consume: %v", err)
	}
	if afterConsume.Quantity != 7 {
		t.Errorf("quantity after consuming 3 of 10 = %v, want 7", afterConsume.Quantity)
	}
	if afterConsume.Confidence != domain.ConfidenceExact {
		t.Errorf("expected EXACT confidence for counted consume, got %s", afterConsume.Confidence)
	}

	if err := s.RecordAdjust(ctx, lotID, 5, true, "recount", "manual_count"); err != nil {
		t.Fatalf("RecordAdjust: %v", err)
	}
	afterAdjust, err := s.GetInventoryLot(ctx, lotID)
	if err != nil {
		t.Fatalf("GetInventoryLot after adjust: %v", err)
	}
	if afterAdjust.Quantity != 5 {
		t.Errorf("quantity after adjust to 5 = %v, want 5", afterAdjust.Quantity)
	}
	if afterAdjust.Confidence != domain.ConfidenceEstimated {
		t.Errorf("expected ESTIMATED confidence for estimated adjust, got %s", afterAdjust.Confidence)
	}

	if err := s.RecordOpen(ctx, lotID, "manual_count"); err != nil {
		t.Fatalf("RecordOpen: %v", err)
	}
	afterOpen, err := s.GetInventoryLot(ctx, lotID)
	if err != nil {
		t.Fatalf("GetInventoryLot after open: %v", err)
	}
	if afterOpen.OpenedAt == nil {
		t.Error("expected opened_at to be set after RecordOpen")
	}
	if afterOpen.Quantity != afterAdjust.Quantity || afterOpen.Confidence != afterAdjust.Confidence {
		t.Errorf("RecordOpen changed quantity/confidence: before %+v after %+v", afterAdjust, afterOpen)
	}

	if err := s.RecordMarkEmpty(ctx, lotID); err != nil {
		t.Fatalf("RecordMarkEmpty: %v", err)
	}
	afterEmpty, err := s.GetInventoryLot(ctx, lotID)
	if err != nil {
		t.Fatalf("GetInventoryLot after mark-empty: %v", err)
	}
	if afterEmpty.Quantity != 0 {
		t.Errorf("quantity after mark-empty = %v, want 0", afterEmpty.Quantity)
	}
	if afterEmpty.Confidence != domain.ConfidenceExact {
		t.Errorf("expected EXACT confidence after mark-empty, got %s", afterEmpty.Confidence)
	}

	// A second lot for DISCARD, independent of the one above.
	discardLotID, err := s.RecordPurchase(ctx, ingredientID, nil, loc.ID, 4, "portion", nil, "manual_count")
	if err != nil {
		t.Fatalf("RecordPurchase (discard lot): %v", err)
	}
	if err := s.RecordDiscard(ctx, discardLotID, 4, false, "spoiled", "manual_count"); err != nil {
		t.Fatalf("RecordDiscard: %v", err)
	}
	discarded, err := s.GetInventoryLot(ctx, discardLotID)
	if err != nil {
		t.Fatalf("GetInventoryLot after discard: %v", err)
	}
	if discarded.Quantity != 0 {
		t.Errorf("quantity after discarding all 4 = %v, want 0", discarded.Quantity)
	}
}

func TestPantry_Transfer(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "inventory_event", "inventory_lot", "inventory_location", "product", "household")
	householdID, ingredientID, _ := pantryFixture(t, ctx, s, "transfer")

	fridge := InventoryLocation{ID: domain.NewInventoryLocationID(), HouseholdID: householdID, Name: "Fridge"}
	freezer := InventoryLocation{ID: domain.NewInventoryLocationID(), HouseholdID: householdID, Name: "Freezer"}
	if err := s.CreateInventoryLocation(ctx, fridge); err != nil {
		t.Fatalf("CreateInventoryLocation (fridge): %v", err)
	}
	if err := s.CreateInventoryLocation(ctx, freezer); err != nil {
		t.Fatalf("CreateInventoryLocation (freezer): %v", err)
	}

	lotID, err := s.RecordPurchase(ctx, ingredientID, nil, fridge.ID, 10, "portion", nil, "manual_count")
	if err != nil {
		t.Fatalf("RecordPurchase: %v", err)
	}

	// Partial transfer: source lot decremented, a distinct destination lot created (not merged).
	newLotID, err := s.RecordTransfer(ctx, lotID, freezer.ID, 4, "manual_count")
	if err != nil {
		t.Fatalf("RecordTransfer (partial): %v", err)
	}
	if newLotID == lotID {
		t.Error("expected a partial transfer to create a distinct destination lot")
	}
	source, err := s.GetInventoryLot(ctx, lotID)
	if err != nil {
		t.Fatalf("GetInventoryLot (source after partial transfer): %v", err)
	}
	if source.Quantity != 6 || source.LocationID != fridge.ID {
		t.Errorf("source lot after partial transfer = %+v, want quantity 6 still in fridge", source)
	}
	dest, err := s.GetInventoryLot(ctx, newLotID)
	if err != nil {
		t.Fatalf("GetInventoryLot (destination): %v", err)
	}
	if dest.Quantity != 4 || dest.LocationID != freezer.ID {
		t.Errorf("destination lot after partial transfer = %+v, want quantity 4 in freezer", dest)
	}

	// Full transfer: the lot itself moves, no new row.
	movedLotID, err := s.RecordTransfer(ctx, newLotID, fridge.ID, 4, "manual_count")
	if err != nil {
		t.Fatalf("RecordTransfer (full): %v", err)
	}
	if movedLotID != newLotID {
		t.Errorf("expected a full transfer to move the same lot id, got new id %s (was %s)", movedLotID, newLotID)
	}
	moved, err := s.GetInventoryLot(ctx, newLotID)
	if err != nil {
		t.Fatalf("GetInventoryLot (moved): %v", err)
	}
	if moved.LocationID != fridge.ID || moved.Quantity != 4 {
		t.Errorf("moved lot = %+v, want quantity 4 in fridge", moved)
	}
}

func TestPantry_LocationHierarchy(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "inventory_event", "inventory_lot", "inventory_location", "product", "household")
	householdID, ingredientID, _ := pantryFixture(t, ctx, s, "hierarchy")

	basement := InventoryLocation{ID: domain.NewInventoryLocationID(), HouseholdID: householdID, Name: "Basement", LocationType: "BASEMENT"}
	if err := s.CreateInventoryLocation(ctx, basement); err != nil {
		t.Fatalf("CreateInventoryLocation (basement): %v", err)
	}
	chestFreezer := InventoryLocation{ID: domain.NewInventoryLocationID(), HouseholdID: householdID, Name: "Chest Freezer", LocationType: "FREEZER", ParentLocationID: &basement.ID}
	if err := s.CreateInventoryLocation(ctx, chestFreezer); err != nil {
		t.Fatalf("CreateInventoryLocation (chest freezer): %v", err)
	}

	lotID, err := s.RecordPurchase(ctx, ingredientID, nil, chestFreezer.ID, 2, "portion", nil, "home_prepared")
	if err != nil {
		t.Fatalf("RecordPurchase: %v", err)
	}

	underBasement, err := s.ListLotsUnderLocation(ctx, basement.ID)
	if err != nil {
		t.Fatalf("ListLotsUnderLocation (basement): %v", err)
	}
	found := false
	for _, l := range underBasement {
		if l.ID == lotID {
			found = true
		}
	}
	if !found {
		t.Errorf("expected lot %s (recorded directly against the nested chest freezer) to appear under basement", lotID)
	}

	// Self-parent is rejected.
	selfID := domain.NewInventoryLocationID()
	if err := s.CreateInventoryLocation(ctx, InventoryLocation{ID: selfID, HouseholdID: householdID, Name: "Self", ParentLocationID: &selfID}); err == nil {
		t.Error("expected creating a location with itself as parent to be rejected")
	}

	// Descendant-as-parent is rejected: basement's parent cannot become the chest freezer (its
	// own descendant) — this would need an UPDATE in a real "move" API; here we simulate the
	// same cycle check by attempting to create a new location whose id collides with an
	// existing ancestor is out of scope, so instead verify directly against the domain function
	// using the persisted ancestor chain, mirroring what CreateInventoryLocation does internally.
	ancestorsOfChestFreezer, err := s.locationAncestors(ctx, chestFreezer.ID)
	if err != nil {
		t.Fatalf("locationAncestors: %v", err)
	}
	if !domain.WouldCreateLocationCycle(basement.ID, chestFreezer.ID, ancestorsOfChestFreezer) {
		t.Error("expected making basement a child of its own descendant (chest freezer) to be flagged as a cycle")
	}
}
