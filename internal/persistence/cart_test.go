package persistence

import (
	"context"
	"testing"
	"time"
)

func TestShoppingCart_CreateAndGet(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "shopping_cart_item", "shopping_cart", "retailer_list_binding", "shopping_list_item", "shopping_list")

	listID, err := s.CreateShoppingList(ctx, ShoppingList{Name: "Vecka 30"})
	if err != nil {
		t.Fatalf("CreateShoppingList: %v", err)
	}
	if err := s.CreateOrUpdateRetailerListBinding(ctx, RetailerListBinding{
		ShoppingListID: listID,
		Retailer:       "willys",
		ExternalListID: "9639791045159",
		SyncDirection:  "outbound",
	}); err != nil {
		t.Fatalf("CreateOrUpdateRetailerListBinding: %v", err)
	}

	cart, err := s.CreateShoppingCart(ctx, ShoppingCart{RetailerListBindingID: listID})
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

	cartID, err := s.CreateShoppingCart(ctx, ShoppingCart{RetailerListBindingID: listID})
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

	cartID, err := s.CreateShoppingCart(ctx, ShoppingCart{RetailerListBindingID: listID})
	if err != nil {
		t.Fatalf("CreateShoppingCart: %v", err)
	}

	price := 49.5
	item, err := s.CreateShoppingCartItem(ctx, ShoppingCartItem{
		ShoppingCartID:    cartID,
		RetailerProductID: "willys-123",
		Quantity:          1,
		Unit:              "pcs",
		ResolvedPrice:     &price,
	})
	if err != nil {
		t.Fatalf("CreateShoppingCartItem: %v", err)
	}
	items, err := s.ListShoppingCartItems(ctx, cartID)
	if err != nil {
		t.Fatalf("ListShoppingCartItems: %v", err)
	}
	if len(items) != 1 || items[0].RetailerProductID != "willys-123" || items[0].Quantity != 1 {
		t.Errorf("items = %+v", items)
	}
	if items[0].ResolvedPrice == nil || *items[0].ResolvedPrice != 49.5 {
		t.Errorf("resolved_price = %v, want 49.5", items[0].ResolvedPrice)
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

	cartID, err := s.CreateShoppingCart(ctx, ShoppingCart{RetailerListBindingID: listID})
	if err != nil {
		t.Fatalf("CreateShoppingCart: %v", err)
	}

	// No price — checkpoint doesn't always have price info.
	_, err = s.CreateShoppingCartItem(ctx, ShoppingCartItem{
		ShoppingCartID:    cartID,
		RetailerProductID: "willys-456",
		Quantity:          2,
		Unit:              "pcs",
		ResolvedPrice:     nil,
	})
	if err != nil {
		t.Fatalf("CreateShoppingCartItem (no price): %v", err)
	}
	items, err := s.ListShoppingCartItems(ctx, cartID)
	if err != nil {
		t.Fatalf("ListShoppingCartItems: %v", err)
	}
	if len(items) != 1 || items[0].ResolvedPrice != nil {
		t.Errorf("expected null resolved_price, got %+v", items[0])
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
	if err := s.CreateOrUpdateRetailerListBinding(ctx, RetailerListBinding{
		ShoppingListID: listID,
		Retailer:       "willys",
		ExternalListID: "9639791045159",
	}); err != nil {
		t.Fatalf("CreateOrUpdateRetailerListBinding: %v", err)
	}

	cartID, err := s.CreateShoppingCart(ctx, ShoppingCart{RetailerListBindingID: listID})
	if err != nil {
		t.Fatalf("CreateShoppingCart: %v", err)
	}
	_, err = s.CreateShoppingCartItem(ctx, ShoppingCartItem{
		ShoppingCartID:    cartID,
		RetailerProductID: "willys-123",
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
	if err == nil || len(items) != 0 {
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

	_, err = s.CreateShoppingCart(ctx, ShoppingCart{RetailerListBindingID: listID})
	if err != nil {
		t.Fatalf("CreateShoppingCart 1: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	_, err = s.CreateShoppingCart(ctx, ShoppingCart{RetailerListBindingID: listID})
	if err != nil {
		t.Fatalf("CreateShoppingCart 2: %v", err)
	}

	carts, err := s.ListShoppingCarts(ctx, listID)
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
