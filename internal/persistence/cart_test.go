package persistence

import (
	"context"
	"testing"
	"time"

	"github.com/androidand/spisordning/internal/domain"
)

// createBindingForList creates a retailer_list_binding for the given shopping
// list and returns the binding (which shopping_cart references via
// shopping_list_id + retailer — not a separate binding id).
func createBindingForList(t *testing.T, s *Store, ctx context.Context, listID domain.ShoppingListID) {
	t.Helper()
	if err := s.CreateOrUpdateRetailerListBinding(ctx, RetailerListBinding{
		ShoppingListID: listID,
		Retailer:       "willys",
		ExternalListID: "9639791045159",
		SyncDirection:  "outbound",
	}); err != nil {
		t.Fatalf("CreateOrUpdateRetailerListBinding: %v", err)
	}
	_, err := s.GetRetailerListBinding(ctx, listID, "willys")
	if err != nil {
		t.Fatalf("GetRetailerListBinding: %v", err)
	}
}

func TestShoppingCart_CreateAndGet(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "shopping_cart_item", "shopping_cart", "retailer_list_binding", "shopping_list_item", "shopping_list")

	listID, err := s.CreateShoppingList(ctx, ShoppingList{Name: "Vecka 30"})
	if err != nil {
		t.Fatalf("CreateShoppingList: %v", err)
	}
	createBindingForList(t, s, ctx, listID)

	cart, err := s.CreateShoppingCart(ctx, ShoppingCart{ShoppingListID: listID, Retailer: "willys"})
	if err != nil {
		t.Fatalf("CreateShoppingCart: %v", err)
	}
	got, err := s.GetShoppingCart(ctx, cart)
	if err != nil {
		t.Fatalf("GetShoppingCart: %v", err)
	}
	if got.Status != "created" {
		t.Errorf("status = %q, want created", got.Status)
	}
}

func TestShoppingCart_StatusTransitions(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "shopping_cart_item", "shopping_cart", "retailer_list_binding", "shopping_list_item", "shopping_list")

	listID, err := s.CreateShoppingList(ctx, ShoppingList{Name: "Vecka 30"})
	if err != nil {
		t.Fatalf("CreateShoppingList: %v", err)
	}
	createBindingForList(t, s, ctx, listID)

	cartID, err := s.CreateShoppingCart(ctx, ShoppingCart{ShoppingListID: listID, Retailer: "willys"})
	if err != nil {
		t.Fatalf("CreateShoppingCart: %v", err)
	}
	if err := s.UpdateShoppingCartStatus(ctx, cartID, "confirmed"); err != nil {
		t.Fatalf("UpdateShoppingCartStatus: %v", err)
	}
	got, err := s.GetShoppingCart(ctx, cartID)
	if err != nil {
		t.Fatalf("GetShoppingCart: %v", err)
	}
	if got.Status != "confirmed" {
		t.Errorf("status = %q, want confirmed", got.Status)
	}
}

func TestShoppingCartItem_RoundTrip(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "shopping_cart_item", "shopping_cart", "retailer_list_binding", "shopping_list_item", "shopping_list")

	listID, err := s.CreateShoppingList(ctx, ShoppingList{Name: "Vecka 30"})
	if err != nil {
		t.Fatalf("CreateShoppingList: %v", err)
	}
	createBindingForList(t, s, ctx, listID)

	cartID, err := s.CreateShoppingCart(ctx, ShoppingCart{ShoppingListID: listID, Retailer: "willys"})
	if err != nil {
		t.Fatalf("CreateShoppingCart: %v", err)
	}

	priceMinor := int64(4950)
	rpID := domain.NewRetailerProductID()
	item, err := s.CreateShoppingCartItem(ctx, ShoppingCartItem{
		ShoppingCartID:     cartID,
		LineNo:             1,
		RetailerProductID:  rpID,
		Quantity:           1,
		Unit:               "pcs",
		ResolvedPriceMinor: &priceMinor,
		Currency:           "SEK",
	})
	if err != nil {
		t.Fatalf("CreateShoppingCartItem: %v", err)
	}
	items, err := s.ListShoppingCartItems(ctx, cartID)
	if err != nil {
		t.Fatalf("ListShoppingCartItems: %v", err)
	}
	if len(items) != 1 || items[0].RetailerProductID != rpID || items[0].Quantity != 1 {
		t.Errorf("items = %+v", items)
	}
	if items[0].ResolvedPriceMinor == nil || *items[0].ResolvedPriceMinor != 4950 {
		t.Errorf("resolved_price_minor = %v, want 4950", items[0].ResolvedPriceMinor)
	}
	_ = item
}

func TestShoppingCartItem_NullPrice(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "shopping_cart_item", "shopping_cart", "retailer_list_binding", "shopping_list_item", "shopping_list")

	listID, err := s.CreateShoppingList(ctx, ShoppingList{Name: "Vecka 30"})
	if err != nil {
		t.Fatalf("CreateShoppingList: %v", err)
	}
	createBindingForList(t, s, ctx, listID)

	cartID, err := s.CreateShoppingCart(ctx, ShoppingCart{ShoppingListID: listID, Retailer: "willys"})
	if err != nil {
		t.Fatalf("CreateShoppingCart: %v", err)
	}

	// No price — checkpoint doesn't always have price info.
	_, err = s.CreateShoppingCartItem(ctx, ShoppingCartItem{
		ShoppingCartID:     cartID,
		LineNo:             1,
		RetailerProductID:  domain.NewRetailerProductID(),
		Quantity:           2,
		Unit:               "pcs",
		ResolvedPriceMinor: nil,
		Currency:           "SEK",
	})
	if err != nil {
		t.Fatalf("CreateShoppingCartItem (no price): %v", err)
	}
	items, err := s.ListShoppingCartItems(ctx, cartID)
	if err != nil {
		t.Fatalf("ListShoppingCartItems: %v", err)
	}
	if len(items) != 1 || items[0].ResolvedPriceMinor != nil {
		t.Errorf("expected null resolved_price_minor, got %+v", items[0])
	}
}

func TestShoppingCart_CASCADEDeletesWithBinding(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "shopping_cart_item", "shopping_cart", "retailer_list_binding", "shopping_list_item", "shopping_list")

	listID, err := s.CreateShoppingList(ctx, ShoppingList{Name: "Vecka 30"})
	if err != nil {
		t.Fatalf("CreateShoppingList: %v", err)
	}
	createBindingForList(t, s, ctx, listID)

	cartID, err := s.CreateShoppingCart(ctx, ShoppingCart{ShoppingListID: listID, Retailer: "willys"})
	if err != nil {
		t.Fatalf("CreateShoppingCart: %v", err)
	}
	_, err = s.CreateShoppingCartItem(ctx, ShoppingCartItem{
		ShoppingCartID:    cartID,
		LineNo:            1,
		RetailerProductID: domain.NewRetailerProductID(),
		Quantity:          1,
		Unit:              "pcs",
	})
	if err != nil {
		t.Fatalf("CreateShoppingCartItem: %v", err)
	}

	// Deleting the binding cascades to cart and cart items.
	if _, err := s.db.Exec(ctx, "DELETE FROM retailer_list_binding WHERE shopping_list_id = $1", listID); err != nil {
		t.Fatalf("delete binding: %v", err)
	}
	_, err = s.GetShoppingCart(ctx, cartID)
	if err == nil {
		t.Error("expected cart to be deleted via CASCADE")
	}
	items, err := s.ListShoppingCartItems(ctx, cartID)
	if err != nil {
		t.Fatalf("ListShoppingCartItems after CASCADE: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 cart items after CASCADE, got %d", len(items))
	}
}

func TestListShoppingCarts_Order(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "shopping_cart_item", "shopping_cart", "retailer_list_binding", "shopping_list_item", "shopping_list")

	listID, err := s.CreateShoppingList(ctx, ShoppingList{Name: "Vecka 30"})
	if err != nil {
		t.Fatalf("CreateShoppingList: %v", err)
	}
	createBindingForList(t, s, ctx, listID)

	_, err = s.CreateShoppingCart(ctx, ShoppingCart{ShoppingListID: listID, Retailer: "willys"})
	if err != nil {
		t.Fatalf("CreateShoppingCart 1: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	_, err = s.CreateShoppingCart(ctx, ShoppingCart{ShoppingListID: listID, Retailer: "willys"})
	if err != nil {
		t.Fatalf("CreateShoppingCart 2: %v", err)
	}

	carts, err := s.ListShoppingCarts(ctx, listID, "willys")
	if err != nil {
		t.Fatalf("ListShoppingCarts: %v", err)
	}
	if len(carts) != 2 {
		t.Fatalf("expected 2 carts, got %d", len(carts))
	}
	// Latest first (ORDER BY created_at DESC).
	if carts[0].CreatedAt.Before(carts[1].CreatedAt) {
		t.Errorf("expected newest cart first")
	}
}
