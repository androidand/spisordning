package persistence

import (
	"context"
	"testing"
	"time"
)

func TestShoppingList_CreateAndGet(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "shopping_list_item", "shopping_list")

	owner := "p-001"
	l, err := s.CreateShoppingList(ctx, ShoppingList{
		OwnerPersonID: &owner,
		Name:          "Vecka 30",
		Status:        "active",
	})
	if err != nil {
		t.Fatalf("CreateShoppingList: %v", err)
	}
	got, err := s.GetShoppingList(ctx, l)
	if err != nil {
		t.Fatalf("GetShoppingList: %v", err)
	}
	if got.Name != "Vecka 30" || got.Status != "active" {
		t.Errorf("got %+v", got)
	}
	if got.OwnerPersonID == nil || *got.OwnerPersonID != owner {
		t.Errorf("owner_person_id = %v, want %q", got.OwnerPersonID, owner)
	}
}

func TestShoppingList_ListAndStatusUpdate(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "shopping_list_item", "shopping_list")

	_, err := s.CreateShoppingList(ctx, ShoppingList{Name: "List A", Status: "active"})
	if err != nil {
		t.Fatalf("CreateShoppingList A: %v", err)
	}
	_, err = s.CreateShoppingList(ctx, ShoppingList{Name: "List B", Status: "active"})
	if err != nil {
		t.Fatalf("CreateShoppingList B: %v", err)
	}

	alls, err := s.ListShoppingLists(ctx)
	if err != nil {
		t.Fatalf("ListShoppingLists: %v", err)
	}
	if len(alls) != 2 {
		t.Fatalf("expected 2 lists, got %d", len(alls))
	}
	// Order is by created_at DESC so B comes first.
	if alls[0].Name != "List B" {
		t.Errorf("expected B first, got %s", alls[0].Name)
	}

	if err := s.UpdateShoppingListStatus(ctx, alls[1].ID, "archived"); err != nil {
		t.Fatalf("UpdateShoppingListStatus: %v", err)
	}
	got, err := s.GetShoppingList(ctx, alls[1].ID)
	if err != nil {
		t.Fatalf("GetShoppingList: %v", err)
	}
	if got.Status != "archived" {
		t.Errorf("status = %q, want archived", got.Status)
	}
}

func TestShoppingListItem_RoundTrip(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "shopping_list_item", "shopping_list")

	listID, err := s.CreateShoppingList(ctx, ShoppingList{Name: "Test list"})
	if err != nil {
		t.Fatalf("CreateShoppingList: %v", err)
	}

	// Seed an ingredient for the FK to resolve.
	if err := s.UpsertIngredient(ctx, Ingredient{ID: "köttfärs", Display: "Köttfärs"}); err != nil {
		t.Fatalf("UpsertIngredient: %v", err)
	}

	ing := "köttfärs"
	item, err := s.CreateShoppingListItem(ctx, ShoppingListItem{
		ShoppingListID: listID,
		IngredientID:   &ing,
		Label:          nil,
		Quantity:       400,
		Unit:           "g",
		Checked:        false,
	})
	if err != nil {
		t.Fatalf("CreateShoppingListItem: %v", err)
	}

	items, err := s.ListShoppingListItems(ctx, listID)
	if err != nil {
		t.Fatalf("ListShoppingListItems: %v", err)
	}
	if len(items) != 1 || items[0].Quantity != 400 || items[0].Unit != "g" {
		t.Errorf("items = %+v", items)
	}

	// Toggle check-off.
	if err := s.UpdateShoppingListItemChecked(ctx, item, true); err != nil {
		t.Fatalf("UpdateShoppingListItemChecked: %v", err)
	}
	items, err = s.ListShoppingListItems(ctx, listID)
	if err != nil {
		t.Fatalf("ListShoppingListItems after check: %v", err)
	}
	if !items[0].Checked {
		t.Errorf("expected checked=true after toggle")
	}

	// Delete.
	if err := s.DeleteShoppingListItem(ctx, item); err != nil {
		t.Fatalf("DeleteShoppingListItem: %v", err)
	}
	items, err = s.ListShoppingListItems(ctx, listID)
	if err != nil {
		t.Fatalf("ListShoppingListItems after delete: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items after delete, got %d", len(items))
	}
}

func TestShoppingListItem_LabelOnly(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "shopping_list_item", "shopping_list")

	listID, err := s.CreateShoppingList(ctx, ShoppingList{Name: "Pantry"})
	if err != nil {
		t.Fatalf("CreateShoppingList: %v", err)
	}

	label := "Paper towels"
	item, err := s.CreateShoppingListItem(ctx, ShoppingListItem{
		ShoppingListID: listID,
		Label:          &label,
		Quantity:       1,
		Unit:           "pack",
	})
	if err != nil {
		t.Fatalf("CreateShoppingListItem: %v", err)
	}
	items, err := s.ListShoppingListItems(ctx, listID)
	if err != nil {
		t.Fatalf("ListShoppingListItems: %v", err)
	}
	if len(items) != 1 || items[0].Label == nil || *items[0].Label != "Paper towels" {
		t.Errorf("unexpected item: %+v", items[0])
	}
	_ = item
}

func TestRetailerListBinding_CreateAndGet(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "retailer_list_binding", "shopping_list_item", "shopping_list")

	listID, err := s.CreateShoppingList(ctx, ShoppingList{Name: "Vecka 30"})
	if err != nil {
		t.Fatalf("CreateShoppingList: %v", err)
	}

	now := time.Now()
	status := "success"
	b := RetailerListBinding{
		ShoppingListID: listID,
		Retailer:       "willys",
		ExternalListID: "9639791045159",
		SyncDirection:  "outbound",
		LastPushedAt:   &now,
		LastPushStatus: &status,
	}
	if err := s.CreateOrUpdateRetailerListBinding(ctx, b); err != nil {
		t.Fatalf("CreateOrUpdateRetailerListBinding: %v", err)
	}

	got, err := s.GetRetailerListBinding(ctx, listID, "willys")
	if err != nil {
		t.Fatalf("GetRetailerListBinding: %v", err)
	}
	if got.ExternalListID != "9639791045159" || got.Retailer != "willys" {
		t.Errorf("got %+v", got)
	}
	if got.LastPushStatus == nil || *got.LastPushStatus != "success" {
		t.Errorf("last_push_status = %v", got.LastPushStatus)
	}
}

func TestRetailerListBinding_Upsert(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "retailer_list_binding", "shopping_list_item", "shopping_list")

	listID, err := s.CreateShoppingList(ctx, ShoppingList{Name: "Vecka 30"})
	if err != nil {
		t.Fatalf("CreateShoppingList: %v", err)
	}

	status1 := "success"
	b1 := RetailerListBinding{
		ShoppingListID: listID,
		Retailer:       "willys",
		ExternalListID: "list-1",
		SyncDirection:  "outbound",
		LastPushedAt:   ptrTime(date(t, "2026-07-20").UTC()),
		LastPushStatus: &status1,
	}
	if err := s.CreateOrUpdateRetailerListBinding(ctx, b1); err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	// Re-push: updates the same row, not a new one.
	status2 := "error"
	b2 := RetailerListBinding{
		ShoppingListID: listID,
		Retailer:       "willys",
		ExternalListID: "list-2", // changed
		SyncDirection:  "outbound",
		LastPushedAt:   ptrTime(date(t, "2026-07-21").UTC()),
		LastPushStatus: &status2,
	}
	if err := s.CreateOrUpdateRetailerListBinding(ctx, b2); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	got, err := s.GetRetailerListBinding(ctx, listID, "willys")
	if err != nil {
		t.Fatalf("GetRetailerListBinding: %v", err)
	}
	if got.ExternalListID != "list-2" {
		t.Errorf("external_list_id = %q, want list-2", got.ExternalListID)
	}
	if got.LastPushStatus == nil || *got.LastPushStatus != "error" {
		t.Errorf("last_push_status = %v, want error", got.LastPushStatus)
	}

	// A different retailer gets its own row.
	willysStatus := "success"
	willysBinding := RetailerListBinding{
		ShoppingListID: listID,
		Retailer:       "ica",
		ExternalListID: "ica-list-1",
		SyncDirection:  "outbound",
		LastPushedAt:   ptrTime(date(t, "2026-07-21").UTC()),
		LastPushStatus: &willysStatus,
	}
	if err := s.CreateOrUpdateRetailerListBinding(ctx, willysBinding); err != nil {
		t.Fatalf("ica upsert: %v", err)
	}
	bindings, err := s.ListRetailerListBindings(ctx, listID)
	if err != nil {
		t.Fatalf("ListRetailerListBindings: %v", err)
	}
	if len(bindings) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(bindings))
	}
}

func TestShoppingRequirement_Get(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "shopping_requirement", "meal_plan")

	weekStart := date(t, "2043-01-18")
	planID, err := s.CreateMealPlan(ctx, weekStart)
	if err != nil {
		t.Fatalf("CreateMealPlan: %v", err)
	}

	pref := "fresh"
	req := ShoppingRequirement{
		PlanID:          planID,
		IngredientID:    "köttfärs",
		Quantity:        400,
		Unit:            "g",
		AcceptableForms: []string{"400 g", "500 g"},
		PreferredForm:   &pref,
	}
	if err := s.InsertShoppingRequirement(ctx, req); err != nil {
		t.Fatalf("InsertShoppingRequirement: %v", err)
	}

	got, err := s.GetShoppingRequirement(ctx, req.ID)
	if err != nil {
		t.Fatalf("GetShoppingRequirement: %v", err)
	}
	if got.IngredientID != "köttfärs" || got.Quantity != 400 || got.Unit != "g" {
		t.Errorf("got %+v", got)
	}
	if got.PreferredForm == nil || *got.PreferredForm != "fresh" {
		t.Errorf("preferred_form = %v", got.PreferredForm)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
