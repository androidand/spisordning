package persistence

import (
	"context"
	"testing"

	"github.com/androidand/spisordning/internal/domain"
)

func TestCatalog_HouseholdProductIdentifierMapping(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "product_ingredient_mapping", "product_identifier", "product", "household")

	hhID := domain.NewHouseholdID()
	if err := s.CreateHousehold(ctx, Household{ID: hhID, Name: "Test Household"}); err != nil {
		t.Fatalf("CreateHousehold: %v", err)
	}
	gotH, err := s.GetHousehold(ctx, hhID)
	if err != nil {
		t.Fatalf("GetHousehold: %v", err)
	}
	if gotH.Name != "Test Household" {
		t.Errorf("household name = %q, want %q", gotH.Name, "Test Household")
	}

	milkID := domain.IngredientIDForName("milk")
	if err := s.UpsertIngredient(ctx, Ingredient{ID: milkID, Display: "Milk"}); err != nil {
		t.Fatalf("UpsertIngredient: %v", err)
	}

	p := Product{ID: domain.NewProductID(), Name: "Arla Standardmjölk", Brand: "Arla", PackageSize: "1L"}
	if err := s.CreateProduct(ctx, p); err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	gotP, err := s.GetProduct(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetProduct: %v", err)
	}
	if gotP.Name != p.Name || gotP.Brand != p.Brand || gotP.PackageSize != p.PackageSize {
		t.Errorf("GetProduct = %+v, want %+v", gotP, p)
	}

	// A product without a resolved mapping is still valid (ingredient-catalog spec scenario).
	unmapped, err := s.ListProductsForIngredient(ctx, milkID)
	if err != nil {
		t.Fatalf("ListProductsForIngredient (pre-mapping): %v", err)
	}
	if len(unmapped) != 0 {
		t.Errorf("expected no products mapped yet, got %v", unmapped)
	}

	if err := s.SetProductIngredientMapping(ctx, ProductIngredientMapping{ProductID: p.ID, IngredientID: milkID}); err != nil {
		t.Fatalf("SetProductIngredientMapping: %v", err)
	}
	mapped, err := s.ListProductsForIngredient(ctx, milkID)
	if err != nil {
		t.Fatalf("ListProductsForIngredient (post-mapping): %v", err)
	}
	if len(mapped) != 1 || mapped[0].ID != p.ID {
		t.Errorf("ListProductsForIngredient = %v, want [%s]", mapped, p.ID)
	}

	// GTIN resolution: unknown GTIN resolves to zero-value, no error; known GTIN resolves to the product.
	unknown, err := s.LookupProductByGTIN(ctx, "07300400176353")
	if err != nil {
		t.Fatalf("LookupProductByGTIN (unknown): %v", err)
	}
	if unknown != (domain.ProductID{}) {
		t.Errorf("expected empty product id for unknown GTIN, got %q", unknown)
	}

	if err := s.UpsertProductIdentifier(ctx, p.ID, "07300400176353"); err != nil {
		t.Fatalf("UpsertProductIdentifier: %v", err)
	}
	resolved, err := s.LookupProductByGTIN(ctx, "07300400176353")
	if err != nil {
		t.Fatalf("LookupProductByGTIN (known): %v", err)
	}
	if resolved != p.ID {
		t.Errorf("LookupProductByGTIN = %q, want %q", resolved, p.ID)
	}

	// Re-linking the same GTIN to a different product overwrites the link (D6: latest confirmed
	// match wins).
	p2 := Product{ID: domain.NewProductID(), Name: "Arla Standardmjölk (relabel)"}
	if err := s.CreateProduct(ctx, p2); err != nil {
		t.Fatalf("CreateProduct (p2): %v", err)
	}
	if err := s.UpsertProductIdentifier(ctx, p2.ID, "07300400176353"); err != nil {
		t.Fatalf("UpsertProductIdentifier (relink): %v", err)
	}
	relinked, err := s.LookupProductByGTIN(ctx, "07300400176353")
	if err != nil {
		t.Fatalf("LookupProductByGTIN (relinked): %v", err)
	}
	if relinked != p2.ID {
		t.Errorf("LookupProductByGTIN after relink = %q, want %q", relinked, p2.ID)
	}
}
