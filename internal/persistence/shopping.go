package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ShoppingList mirrors migrations/0004_shopping_list.sql.
type ShoppingList struct {
	ID              int64
	OwnerPersonID   *string
	Name            string
	Status          string // 'active' | 'archived'
	CreatedAt       time.Time
}

// CreateShoppingList inserts a new shopping_list and returns its id. An empty
// status defaults to "active" (the column's CHECK only allows active/archived,
// and the schema default does not apply when a value is supplied).
func (s *Store) CreateShoppingList(ctx context.Context, l ShoppingList) (int64, error) {
	status := l.Status
	if status == "" {
		status = "active"
	}
	const q = `INSERT INTO shopping_list (owner_person_id, name, status)
		VALUES ($1, $2, $3) RETURNING id`
	var id int64
	err := s.db.QueryRow(ctx, q, l.OwnerPersonID, l.Name, status).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("persistence: create shopping_list: %w", err)
	}
	return id, nil
}

// GetShoppingList fetches one shopping_list by id.
func (s *Store) GetShoppingList(ctx context.Context, id int64) (ShoppingList, error) {
	const q = `SELECT id, owner_person_id, name, status, created_at FROM shopping_list WHERE id = $1`
	var l ShoppingList
	if err := s.db.QueryRow(ctx, q, id).Scan(&l.ID, &l.OwnerPersonID, &l.Name, &l.Status, &l.CreatedAt); err != nil {
		return ShoppingList{}, fmt.Errorf("persistence: get shopping_list: %w", err)
	}
	return l, nil
}

// ListShoppingLists returns all shopping_lists ordered by created_at descending.
func (s *Store) ListShoppingLists(ctx context.Context) ([]ShoppingList, error) {
	rows, err := s.db.Query(ctx, `SELECT id, owner_person_id, name, status, created_at FROM shopping_list ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("persistence: list shopping_lists: %w", err)
	}
	defer rows.Close()
	return scanShoppingLists(rows)
}

func scanShoppingLists(rows pgx.Rows) ([]ShoppingList, error) {
	defer rows.Close()
	var out []ShoppingList
	for rows.Next() {
		var l ShoppingList
		if err := rows.Scan(&l.ID, &l.OwnerPersonID, &l.Name, &l.Status, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// UpdateShoppingListStatus updates the status of a shopping_list.
func (s *Store) UpdateShoppingListStatus(ctx context.Context, id int64, status string) error {
	const q = `UPDATE shopping_list SET status = $1 WHERE id = $2`
	if _, err := s.db.Exec(ctx, q, status, id); err != nil {
		return fmt.Errorf("persistence: update shopping_list status: %w", err)
	}
	return nil
}

// ── Shopping list items ──────────────────────────────────────────────────────

// ShoppingListItem mirrors migrations/0004_shopping_list.sql shopping_list_item.
type ShoppingListItem struct {
	ID                    int64
	ShoppingListID        int64
	ShoppingRequirementID *int64
	IngredientID          *string
	Label                 *string
	Quantity              float64
	Unit                  string
	Checked               bool
	AddedAt               time.Time
}

// CreateShoppingListItem inserts a new item and returns its id.
func (s *Store) CreateShoppingListItem(ctx context.Context, item ShoppingListItem) (int64, error) {
	const q = `INSERT INTO shopping_list_item
		(shopping_list_id, shopping_requirement_id, ingredient_id, label, quantity, unit, checked)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`
	var id int64
	err := s.db.QueryRow(ctx, q, item.ShoppingListID, item.ShoppingRequirementID, item.IngredientID,
		item.Label, item.Quantity, item.Unit, item.Checked).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("persistence: create shopping_list_item: %w", err)
	}
	return id, nil
}

// ListShoppingListItems returns all items for a list, ordered by added_at.
func (s *Store) ListShoppingListItems(ctx context.Context, listID int64) ([]ShoppingListItem, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, shopping_list_id, shopping_requirement_id, ingredient_id, label,
		       quantity, unit, checked, added_at
		FROM shopping_list_item WHERE shopping_list_id = $1 ORDER BY added_at`, listID)
	if err != nil {
		return nil, fmt.Errorf("persistence: list shopping_list_items: %w", err)
	}
	defer rows.Close()
	return scanShoppingListItems(rows)
}

func scanShoppingListItems(rows pgx.Rows) ([]ShoppingListItem, error) {
	defer rows.Close()
	var out []ShoppingListItem
	for rows.Next() {
		var item ShoppingListItem
		if err := rows.Scan(&item.ID, &item.ShoppingListID, &item.ShoppingRequirementID,
			&item.IngredientID, &item.Label, &item.Quantity, &item.Unit,
			&item.Checked, &item.AddedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// UpdateShoppingListItemChecked toggles the checked flag on an item.
func (s *Store) UpdateShoppingListItemChecked(ctx context.Context, id int64, checked bool) error {
	const q = `UPDATE shopping_list_item SET checked = $1 WHERE id = $2`
	if _, err := s.db.Exec(ctx, q, checked, id); err != nil {
		return fmt.Errorf("persistence: update shopping_list_item checked: %w", err)
	}
	return nil
}

// DeleteShoppingListItem removes an item row (hard delete; requirement is unaffected).
func (s *Store) DeleteShoppingListItem(ctx context.Context, id int64) error {
	const q = `DELETE FROM shopping_list_item WHERE id = $1`
	if _, err := s.db.Exec(ctx, q, id); err != nil {
		return fmt.Errorf("persistence: delete shopping_list_item: %w", err)
	}
	return nil
}

// GetShoppingRequirement fetches one shopping_requirement by id.
func (s *Store) GetShoppingRequirement(ctx context.Context, id int64) (ShoppingRequirement, error) {
	const q = `SELECT id, plan_id, ingredient_id, quantity, unit, acceptable_forms, preferred_form
		FROM shopping_requirement WHERE id = $1`
	var r ShoppingRequirement
	if err := s.db.QueryRow(ctx, q, id).Scan(&r.ID, &r.PlanID, &r.IngredientID, &r.Quantity, &r.Unit,
		&r.AcceptableForms, &r.PreferredForm); err != nil {
		return ShoppingRequirement{}, fmt.Errorf("persistence: get shopping_requirement: %w", err)
	}
	return r, nil
}

// ── Retailer list bindings ───────────────────────────────────────────────────

// RetailerListBinding mirrors migrations/0005_retailer_list_binding.sql.
type RetailerListBinding struct {
	ID             int64
	ShoppingListID int64
	Retailer       string
	ExternalListID string
	SyncDirection  string // 'outbound' in v1
	LastPushedAt   *time.Time
	LastPushStatus *string // 'success' | 'error'
}

// CreateOrUpdateRetailerListBinding upserts the binding for a (list, retailer) pair.
// On first push this inserts; on re-push it updates the existing row, per the
// UNIQUE (shopping_list_id, retailer) constraint.
func (s *Store) CreateOrUpdateRetailerListBinding(ctx context.Context, b RetailerListBinding) error {
	const q = `INSERT INTO retailer_list_binding
		(shopping_list_id, retailer, external_list_id, sync_direction, last_pushed_at, last_push_status)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (shopping_list_id, retailer) DO UPDATE SET
			external_list_id = EXCLUDED.external_list_id,
			sync_direction   = EXCLUDED.sync_direction,
			last_pushed_at   = EXCLUDED.last_pushed_at,
			last_push_status = EXCLUDED.last_push_status`
	if _, err := s.db.Exec(ctx, q, b.ShoppingListID, b.Retailer, b.ExternalListID,
		b.SyncDirection, b.LastPushedAt, b.LastPushStatus); err != nil {
		return fmt.Errorf("persistence: upsert retailer_list_binding: %w", err)
	}
	return nil
}

// GetRetailerListBinding fetches the binding for a shopping_list + retailer pair.
func (s *Store) GetRetailerListBinding(ctx context.Context, shoppingListID int64, retailer string) (RetailerListBinding, error) {
	const q = `SELECT id, shopping_list_id, retailer, external_list_id, sync_direction,
		last_pushed_at, last_push_status
		FROM retailer_list_binding WHERE shopping_list_id = $1 AND retailer = $2`
	var b RetailerListBinding
	if err := s.db.QueryRow(ctx, q, shoppingListID, retailer).Scan(
		&b.ID, &b.ShoppingListID, &b.Retailer, &b.ExternalListID, &b.SyncDirection,
		&b.LastPushedAt, &b.LastPushStatus); err != nil {
		return RetailerListBinding{}, fmt.Errorf("persistence: get retailer_list_binding: %w", err)
	}
	return b, nil
}

// ListRetailerListBindings returns all bindings for a shopping_list.
func (s *Store) ListRetailerListBindings(ctx context.Context, shoppingListID int64) ([]RetailerListBinding, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, shopping_list_id, retailer, external_list_id, sync_direction,
		       last_pushed_at, last_push_status
		FROM retailer_list_binding WHERE shopping_list_id = $1`, shoppingListID)
	if err != nil {
		return nil, fmt.Errorf("persistence: list retailer_list_bindings: %w", err)
	}
	defer rows.Close()
	return scanRetailerListBindings(rows)
}

func scanRetailerListBindings(rows pgx.Rows) ([]RetailerListBinding, error) {
	defer rows.Close()
	var out []RetailerListBinding
	for rows.Next() {
		var b RetailerListBinding
		if err := rows.Scan(&b.ID, &b.ShoppingListID, &b.Retailer, &b.ExternalListID,
			&b.SyncDirection, &b.LastPushedAt, &b.LastPushStatus); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
