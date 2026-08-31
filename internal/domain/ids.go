package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// Typed ID types for non-recipe domain entities. Each wraps uuid.UUID so that
// a repository call passing the wrong entity's id type does not compile.
// See design.md D2.

type PersonID uuid.UUID
type IngredientID uuid.UUID
type HouseholdID uuid.UUID
type ProductID uuid.UUID
type InventoryLocationID uuid.UUID
type MealEventID uuid.UUID
type MealPlanID uuid.UUID
type MealPlanCandidateID uuid.UUID
type ShoppingRequirementID uuid.UUID
type RecipeImportCandidateID uuid.UUID
type ShoppingListID uuid.UUID
type ShoppingListItemID uuid.UUID
type ShoppingCartID uuid.UUID
type OrderID uuid.UUID
type OrderItemID uuid.UUID
type InventoryLotID uuid.UUID
type InventoryEventID uuid.UUID
type PreferenceObservationID uuid.UUID
type PlanningConstraintID uuid.UUID
type IngredientAliasID uuid.UUID
type RetailerID uuid.UUID
type StoreID uuid.UUID
type RetailerProductID uuid.UUID
type StoreProductOfferID uuid.UUID
type PriceObservationID uuid.UUID
type AccountID uuid.UUID
type RecipeRefID uuid.UUID
type RecipeFamilyID uuid.UUID
type RecipeVariantID uuid.UUID
type RecipeRevisionID uuid.UUID
type RecipeSourceRefID uuid.UUID

// newUUIDv7 generates a UUIDv7 (time-ordered, per design D1). It panics on
// error, which is effectively impossible: uuid.NewV7 only fails if the system
// clock is broken, and a broken clock would break far more than identity.
func newUUIDv7() uuid.UUID {
	u, err := uuid.NewV7()
	if err != nil {
		panic("domain: generate UUIDv7: " + err.Error())
	}
	return u
}

// ── Constructors ─────────────────────────────────────────────────────────────

func NewPersonID() PersonID { return PersonID(newUUIDv7()) }
func NewIngredientID() IngredientID { return IngredientID(newUUIDv7()) }
func NewHouseholdID() HouseholdID { return HouseholdID(newUUIDv7()) }
func NewProductID() ProductID { return ProductID(newUUIDv7()) }
func NewInventoryLocationID() InventoryLocationID { return InventoryLocationID(newUUIDv7()) }
func NewMealEventID() MealEventID { return MealEventID(newUUIDv7()) }
func NewMealPlanID() MealPlanID { return MealPlanID(newUUIDv7()) }
func NewMealPlanCandidateID() MealPlanCandidateID { return MealPlanCandidateID(newUUIDv7()) }
func NewShoppingRequirementID() ShoppingRequirementID { return ShoppingRequirementID(newUUIDv7()) }
func NewRecipeImportCandidateID() RecipeImportCandidateID { return RecipeImportCandidateID(newUUIDv7()) }
func NewShoppingListID() ShoppingListID { return ShoppingListID(newUUIDv7()) }
func NewShoppingListItemID() ShoppingListItemID { return ShoppingListItemID(newUUIDv7()) }
func NewShoppingCartID() ShoppingCartID { return ShoppingCartID(newUUIDv7()) }
func NewOrderID() OrderID { return OrderID(newUUIDv7()) }
func NewOrderItemID() OrderItemID { return OrderItemID(newUUIDv7()) }
func NewInventoryLotID() InventoryLotID { return InventoryLotID(newUUIDv7()) }
func NewInventoryEventID() InventoryEventID { return InventoryEventID(newUUIDv7()) }
func NewPreferenceObservationID() PreferenceObservationID { return PreferenceObservationID(newUUIDv7()) }
func NewPlanningConstraintID() PlanningConstraintID { return PlanningConstraintID(newUUIDv7()) }
func NewIngredientAliasID() IngredientAliasID { return IngredientAliasID(newUUIDv7()) }
func NewRetailerID() RetailerID { return RetailerID(newUUIDv7()) }
func NewStoreID() StoreID { return StoreID(newUUIDv7()) }
func NewRetailerProductID() RetailerProductID { return RetailerProductID(newUUIDv7()) }
func NewStoreProductOfferID() StoreProductOfferID { return StoreProductOfferID(newUUIDv7()) }
func NewPriceObservationID() PriceObservationID { return PriceObservationID(newUUIDv7()) }
func NewAccountID() AccountID { return AccountID(newUUIDv7()) }
func NewRecipeRefID() RecipeRefID { return RecipeRefID(newUUIDv7()) }
func NewRecipeFamilyID() RecipeFamilyID { return RecipeFamilyID(newUUIDv7()) }
func NewRecipeVariantID() RecipeVariantID { return RecipeVariantID(newUUIDv7()) }
func NewRecipeRevisionID() RecipeRevisionID { return RecipeRevisionID(newUUIDv7()) }
func NewRecipeSourceRefID() RecipeSourceRefID { return RecipeSourceRefID(newUUIDv7()) }

// ── UUID() accessors ─────────────────────────────────────────────────────────

func (id PersonID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id IngredientID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id HouseholdID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id ProductID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id InventoryLocationID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id MealEventID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id MealPlanID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id MealPlanCandidateID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id ShoppingRequirementID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id RecipeImportCandidateID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id ShoppingListID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id ShoppingListItemID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id ShoppingCartID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id OrderID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id OrderItemID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id InventoryLotID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id InventoryEventID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id PreferenceObservationID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id PlanningConstraintID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id IngredientAliasID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id RetailerID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id StoreID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id RetailerProductID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id StoreProductOfferID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id PriceObservationID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id AccountID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id RecipeRefID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id RecipeFamilyID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id RecipeVariantID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id RecipeRevisionID) UUID() uuid.UUID { return uuid.UUID(id) }
func (id RecipeSourceRefID) UUID() uuid.UUID { return uuid.UUID(id) }

// ── String() ─────────────────────────────────────────────────────────────────

func (id PersonID) String() string { return uuid.UUID(id).String() }
func (id IngredientID) String() string { return uuid.UUID(id).String() }
func (id HouseholdID) String() string { return uuid.UUID(id).String() }
func (id ProductID) String() string { return uuid.UUID(id).String() }
func (id InventoryLocationID) String() string { return uuid.UUID(id).String() }
func (id MealEventID) String() string { return uuid.UUID(id).String() }
func (id MealPlanID) String() string { return uuid.UUID(id).String() }
func (id MealPlanCandidateID) String() string { return uuid.UUID(id).String() }
func (id ShoppingRequirementID) String() string { return uuid.UUID(id).String() }
func (id RecipeImportCandidateID) String() string { return uuid.UUID(id).String() }
func (id ShoppingListID) String() string { return uuid.UUID(id).String() }
func (id ShoppingListItemID) String() string { return uuid.UUID(id).String() }
func (id ShoppingCartID) String() string { return uuid.UUID(id).String() }
func (id OrderID) String() string { return uuid.UUID(id).String() }
func (id OrderItemID) String() string { return uuid.UUID(id).String() }
func (id InventoryLotID) String() string { return uuid.UUID(id).String() }
func (id InventoryEventID) String() string { return uuid.UUID(id).String() }
func (id PreferenceObservationID) String() string { return uuid.UUID(id).String() }
func (id PlanningConstraintID) String() string { return uuid.UUID(id).String() }
func (id IngredientAliasID) String() string { return uuid.UUID(id).String() }
func (id RetailerID) String() string { return uuid.UUID(id).String() }
func (id StoreID) String() string { return uuid.UUID(id).String() }
func (id RetailerProductID) String() string { return uuid.UUID(id).String() }
func (id StoreProductOfferID) String() string { return uuid.UUID(id).String() }
func (id PriceObservationID) String() string { return uuid.UUID(id).String() }
func (id AccountID) String() string { return uuid.UUID(id).String() }
func (id RecipeRefID) String() string { return uuid.UUID(id).String() }
func (id RecipeFamilyID) String() string { return uuid.UUID(id).String() }
func (id RecipeVariantID) String() string { return uuid.UUID(id).String() }
func (id RecipeRevisionID) String() string { return uuid.UUID(id).String() }
func (id RecipeSourceRefID) String() string { return uuid.UUID(id).String() }

// ── driver.Valuer / sql.Scanner ──────────────────────────────────────────────

func (id PersonID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *PersonID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = PersonID(u); return nil }
func (id IngredientID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *IngredientID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = IngredientID(u); return nil }
func (id HouseholdID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *HouseholdID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = HouseholdID(u); return nil }
func (id ProductID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *ProductID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = ProductID(u); return nil }
func (id InventoryLocationID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *InventoryLocationID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = InventoryLocationID(u); return nil }
func (id MealEventID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *MealEventID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = MealEventID(u); return nil }
func (id MealPlanID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *MealPlanID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = MealPlanID(u); return nil }
func (id MealPlanCandidateID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *MealPlanCandidateID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = MealPlanCandidateID(u); return nil }
func (id ShoppingRequirementID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *ShoppingRequirementID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = ShoppingRequirementID(u); return nil }
func (id RecipeImportCandidateID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *RecipeImportCandidateID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = RecipeImportCandidateID(u); return nil }
func (id ShoppingListID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *ShoppingListID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = ShoppingListID(u); return nil }
func (id ShoppingListItemID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *ShoppingListItemID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = ShoppingListItemID(u); return nil }
func (id ShoppingCartID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *ShoppingCartID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = ShoppingCartID(u); return nil }
func (id OrderID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *OrderID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = OrderID(u); return nil }
func (id OrderItemID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *OrderItemID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = OrderItemID(u); return nil }
func (id InventoryLotID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *InventoryLotID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = InventoryLotID(u); return nil }
func (id InventoryEventID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *InventoryEventID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = InventoryEventID(u); return nil }
func (id PreferenceObservationID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *PreferenceObservationID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = PreferenceObservationID(u); return nil }
func (id PlanningConstraintID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *PlanningConstraintID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = PlanningConstraintID(u); return nil }
func (id IngredientAliasID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *IngredientAliasID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = IngredientAliasID(u); return nil }
func (id RetailerID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *RetailerID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = RetailerID(u); return nil }
func (id StoreID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *StoreID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = StoreID(u); return nil }
func (id RetailerProductID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *RetailerProductID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = RetailerProductID(u); return nil }
func (id StoreProductOfferID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *StoreProductOfferID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = StoreProductOfferID(u); return nil }
func (id PriceObservationID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *PriceObservationID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = PriceObservationID(u); return nil }
func (id AccountID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *AccountID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = AccountID(u); return nil }
func (id RecipeRefID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *RecipeRefID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = RecipeRefID(u); return nil }
func (id RecipeFamilyID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *RecipeFamilyID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = RecipeFamilyID(u); return nil }
func (id RecipeVariantID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *RecipeVariantID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = RecipeVariantID(u); return nil }
func (id RecipeRevisionID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *RecipeRevisionID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = RecipeRevisionID(u); return nil }
func (id RecipeSourceRefID) Value() (driver.Value, error) { return id.UUID().Value() }
func (id *RecipeSourceRefID) Scan(src any) error { var u uuid.UUID; if err := u.Scan(src); err != nil { return err }; *id = RecipeSourceRefID(u); return nil }

// ── Parse helpers ────────────────────────────────────────────────────────────

func ParsePersonID(s string) (PersonID, error) { u, err := uuid.Parse(s); if err != nil { return PersonID{}, fmt.Errorf("domain: parse person id %q: %w", s, err) }; return PersonID(u), nil }
func ParseIngredientID(s string) (IngredientID, error) { u, err := uuid.Parse(s); if err != nil { return IngredientID{}, fmt.Errorf("domain: parse ingredient id %q: %w", s, err) }; return IngredientID(u), nil }
func ParseHouseholdID(s string) (HouseholdID, error) { u, err := uuid.Parse(s); if err != nil { return HouseholdID{}, fmt.Errorf("domain: parse household id %q: %w", s, err) }; return HouseholdID(u), nil }
func ParseProductID(s string) (ProductID, error) { u, err := uuid.Parse(s); if err != nil { return ProductID{}, fmt.Errorf("domain: parse product id %q: %w", s, err) }; return ProductID(u), nil }
func ParseShoppingListID(s string) (ShoppingListID, error) { u, err := uuid.Parse(s); if err != nil { return ShoppingListID{}, fmt.Errorf("domain: parse shopping list id %q: %w", s, err) }; return ShoppingListID(u), nil }
func ParseOrderID(s string) (OrderID, error) { u, err := uuid.Parse(s); if err != nil { return OrderID{}, fmt.Errorf("domain: parse order id %q: %w", s, err) }; return OrderID(u), nil }
func ParseMealEventID(s string) (MealEventID, error) { u, err := uuid.Parse(s); if err != nil { return MealEventID{}, fmt.Errorf("domain: parse meal event id %q: %w", s, err) }; return MealEventID(u), nil }
func ParseMealPlanID(s string) (MealPlanID, error) { u, err := uuid.Parse(s); if err != nil { return MealPlanID{}, fmt.Errorf("domain: parse meal plan id %q: %w", s, err) }; return MealPlanID(u), nil }
func ParseShoppingCartID(s string) (ShoppingCartID, error) { u, err := uuid.Parse(s); if err != nil { return ShoppingCartID{}, fmt.Errorf("domain: parse shopping cart id %q: %w", s, err) }; return ShoppingCartID(u), nil }
func ParseInventoryLotID(s string) (InventoryLotID, error) { u, err := uuid.Parse(s); if err != nil { return InventoryLotID{}, fmt.Errorf("domain: parse inventory lot id %q: %w", s, err) }; return InventoryLotID(u), nil }
func ParseInventoryLocationID(s string) (InventoryLocationID, error) { u, err := uuid.Parse(s); if err != nil { return InventoryLocationID{}, fmt.Errorf("domain: parse inventory location id %q: %w", s, err) }; return InventoryLocationID(u), nil }
func ParseShoppingRequirementID(s string) (ShoppingRequirementID, error) { u, err := uuid.Parse(s); if err != nil { return ShoppingRequirementID{}, fmt.Errorf("domain: parse shopping requirement id %q: %w", s, err) }; return ShoppingRequirementID(u), nil }
func ParseShoppingListItemID(s string) (ShoppingListItemID, error) { u, err := uuid.Parse(s); if err != nil { return ShoppingListItemID{}, fmt.Errorf("domain: parse shopping list item id %q: %w", s, err) }; return ShoppingListItemID(u), nil }
func ParseOrderItemID(s string) (OrderItemID, error) { u, err := uuid.Parse(s); if err != nil { return OrderItemID{}, fmt.Errorf("domain: parse order item id %q: %w", s, err) }; return OrderItemID(u), nil }
func ParseRecipeImportCandidateID(s string) (RecipeImportCandidateID, error) { u, err := uuid.Parse(s); if err != nil { return RecipeImportCandidateID{}, fmt.Errorf("domain: parse recipe import candidate id %q: %w", s, err) }; return RecipeImportCandidateID(u), nil }
func ParsePreferenceObservationID(s string) (PreferenceObservationID, error) { u, err := uuid.Parse(s); if err != nil { return PreferenceObservationID{}, fmt.Errorf("domain: parse preference observation id %q: %w", s, err) }; return PreferenceObservationID(u), nil }
func ParsePlanningConstraintID(s string) (PlanningConstraintID, error) { u, err := uuid.Parse(s); if err != nil { return PlanningConstraintID{}, fmt.Errorf("domain: parse planning constraint id %q: %w", s, err) }; return PlanningConstraintID(u), nil }
func ParseIngredientAliasID(s string) (IngredientAliasID, error) { u, err := uuid.Parse(s); if err != nil { return IngredientAliasID{}, fmt.Errorf("domain: parse ingredient alias id %q: %w", s, err) }; return IngredientAliasID(u), nil }
func ParseRetailerID(s string) (RetailerID, error) { u, err := uuid.Parse(s); if err != nil { return RetailerID{}, fmt.Errorf("domain: parse retailer id %q: %w", s, err) }; return RetailerID(u), nil }
func ParseStoreID(s string) (StoreID, error) { u, err := uuid.Parse(s); if err != nil { return StoreID{}, fmt.Errorf("domain: parse store id %q: %w", s, err) }; return StoreID(u), nil }
func ParseRetailerProductID(s string) (RetailerProductID, error) { u, err := uuid.Parse(s); if err != nil { return RetailerProductID{}, fmt.Errorf("domain: parse retailer product id %q: %w", s, err) }; return RetailerProductID(u), nil }
func ParseStoreProductOfferID(s string) (StoreProductOfferID, error) { u, err := uuid.Parse(s); if err != nil { return StoreProductOfferID{}, fmt.Errorf("domain: parse store product offer id %q: %w", s, err) }; return StoreProductOfferID(u), nil }
func ParsePriceObservationID(s string) (PriceObservationID, error) { u, err := uuid.Parse(s); if err != nil { return PriceObservationID{}, fmt.Errorf("domain: parse price observation id %q: %w", s, err) }; return PriceObservationID(u), nil }
func ParseAccountID(s string) (AccountID, error) { u, err := uuid.Parse(s); if err != nil { return AccountID{}, fmt.Errorf("domain: parse account id %q: %w", s, err) }; return AccountID(u), nil }
func ParseRecipeRefID(s string) (RecipeRefID, error) { u, err := uuid.Parse(s); if err != nil { return RecipeRefID{}, fmt.Errorf("domain: parse recipe ref id %q: %w", s, err) }; return RecipeRefID(u), nil }
func ParseRecipeFamilyID(s string) (RecipeFamilyID, error) { u, err := uuid.Parse(s); if err != nil { return RecipeFamilyID{}, fmt.Errorf("domain: parse recipe family id %q: %w", s, err) }; return RecipeFamilyID(u), nil }
func ParseRecipeVariantID(s string) (RecipeVariantID, error) { u, err := uuid.Parse(s); if err != nil { return RecipeVariantID{}, fmt.Errorf("domain: parse recipe variant id %q: %w", s, err) }; return RecipeVariantID(u), nil }
func ParseRecipeRevisionID(s string) (RecipeRevisionID, error) { u, err := uuid.Parse(s); if err != nil { return RecipeRevisionID{}, fmt.Errorf("domain: parse recipe revision id %q: %w", s, err) }; return RecipeRevisionID(u), nil }
func ParseRecipeSourceRefID(s string) (RecipeSourceRefID, error) { u, err := uuid.Parse(s); if err != nil { return RecipeSourceRefID{}, fmt.Errorf("domain: parse recipe source ref id %q: %w", s, err) }; return RecipeSourceRefID(u), nil }
func ParseMealPlanCandidateID(s string) (MealPlanCandidateID, error) { u, err := uuid.Parse(s); if err != nil { return MealPlanCandidateID{}, fmt.Errorf("domain: parse meal plan candidate id %q: %w", s, err) }; return MealPlanCandidateID(u), nil }
func ParseInventoryEventID(s string) (InventoryEventID, error) { u, err := uuid.Parse(s); if err != nil { return InventoryEventID{}, fmt.Errorf("domain: parse inventory event id %q: %w", s, err) }; return InventoryEventID(u), nil }

// IngredientIDForName derives a deterministic UUIDv5 from a canonical food
// name. Until the ingredient_mapping table refines ids, the canonical id of an
// unmapped food is its own normalized name — this function turns that name into
// a stable UUID so the same food always resolves to the same ingredient id.
func IngredientIDForName(canonicalName string) IngredientID {
	u := uuid.NewSHA1(uuid.NameSpaceURL, []byte("spisordning/ingredient/"+canonicalName))
	return IngredientID(u)
}

// ── JSON marshaling ──────────────────────────────────────────────────────────
// Each typed ID marshals to and from a UUID string in JSON.

func (id PersonID) MarshalJSON() ([]byte, error)           { return json.Marshal(uuid.UUID(id)) }
func (id *PersonID) UnmarshalJSON(b []byte) error          { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = PersonID(u); return nil }
func (id IngredientID) MarshalJSON() ([]byte, error)       { return json.Marshal(uuid.UUID(id)) }
func (id *IngredientID) UnmarshalJSON(b []byte) error      { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = IngredientID(u); return nil }
func (id HouseholdID) MarshalJSON() ([]byte, error)        { return json.Marshal(uuid.UUID(id)) }
func (id *HouseholdID) UnmarshalJSON(b []byte) error       { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = HouseholdID(u); return nil }
func (id ProductID) MarshalJSON() ([]byte, error)          { return json.Marshal(uuid.UUID(id)) }
func (id *ProductID) UnmarshalJSON(b []byte) error         { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = ProductID(u); return nil }
func (id InventoryLocationID) MarshalJSON() ([]byte, error) { return json.Marshal(uuid.UUID(id)) }
func (id *InventoryLocationID) UnmarshalJSON(b []byte) error { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = InventoryLocationID(u); return nil }
func (id MealEventID) MarshalJSON() ([]byte, error)        { return json.Marshal(uuid.UUID(id)) }
func (id *MealEventID) UnmarshalJSON(b []byte) error       { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = MealEventID(u); return nil }
func (id MealPlanID) MarshalJSON() ([]byte, error)         { return json.Marshal(uuid.UUID(id)) }
func (id *MealPlanID) UnmarshalJSON(b []byte) error        { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = MealPlanID(u); return nil }
func (id MealPlanCandidateID) MarshalJSON() ([]byte, error) { return json.Marshal(uuid.UUID(id)) }
func (id *MealPlanCandidateID) UnmarshalJSON(b []byte) error { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = MealPlanCandidateID(u); return nil }
func (id ShoppingRequirementID) MarshalJSON() ([]byte, error) { return json.Marshal(uuid.UUID(id)) }
func (id *ShoppingRequirementID) UnmarshalJSON(b []byte) error { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = ShoppingRequirementID(u); return nil }
func (id RecipeImportCandidateID) MarshalJSON() ([]byte, error) { return json.Marshal(uuid.UUID(id)) }
func (id *RecipeImportCandidateID) UnmarshalJSON(b []byte) error { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = RecipeImportCandidateID(u); return nil }
func (id ShoppingListID) MarshalJSON() ([]byte, error)     { return json.Marshal(uuid.UUID(id)) }
func (id *ShoppingListID) UnmarshalJSON(b []byte) error    { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = ShoppingListID(u); return nil }
func (id ShoppingListItemID) MarshalJSON() ([]byte, error) { return json.Marshal(uuid.UUID(id)) }
func (id *ShoppingListItemID) UnmarshalJSON(b []byte) error { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = ShoppingListItemID(u); return nil }
func (id ShoppingCartID) MarshalJSON() ([]byte, error)     { return json.Marshal(uuid.UUID(id)) }
func (id *ShoppingCartID) UnmarshalJSON(b []byte) error    { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = ShoppingCartID(u); return nil }
func (id OrderID) MarshalJSON() ([]byte, error)            { return json.Marshal(uuid.UUID(id)) }
func (id *OrderID) UnmarshalJSON(b []byte) error           { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = OrderID(u); return nil }
func (id OrderItemID) MarshalJSON() ([]byte, error)        { return json.Marshal(uuid.UUID(id)) }
func (id *OrderItemID) UnmarshalJSON(b []byte) error       { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = OrderItemID(u); return nil }
func (id InventoryLotID) MarshalJSON() ([]byte, error)     { return json.Marshal(uuid.UUID(id)) }
func (id *InventoryLotID) UnmarshalJSON(b []byte) error    { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = InventoryLotID(u); return nil }
func (id InventoryEventID) MarshalJSON() ([]byte, error)   { return json.Marshal(uuid.UUID(id)) }
func (id *InventoryEventID) UnmarshalJSON(b []byte) error  { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = InventoryEventID(u); return nil }
func (id PreferenceObservationID) MarshalJSON() ([]byte, error) { return json.Marshal(uuid.UUID(id)) }
func (id *PreferenceObservationID) UnmarshalJSON(b []byte) error { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = PreferenceObservationID(u); return nil }
func (id PlanningConstraintID) MarshalJSON() ([]byte, error) { return json.Marshal(uuid.UUID(id)) }
func (id *PlanningConstraintID) UnmarshalJSON(b []byte) error { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = PlanningConstraintID(u); return nil }
func (id IngredientAliasID) MarshalJSON() ([]byte, error)  { return json.Marshal(uuid.UUID(id)) }
func (id *IngredientAliasID) UnmarshalJSON(b []byte) error { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = IngredientAliasID(u); return nil }
func (id RetailerID) MarshalJSON() ([]byte, error)         { return json.Marshal(uuid.UUID(id)) }
func (id *RetailerID) UnmarshalJSON(b []byte) error        { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = RetailerID(u); return nil }
func (id StoreID) MarshalJSON() ([]byte, error)            { return json.Marshal(uuid.UUID(id)) }
func (id *StoreID) UnmarshalJSON(b []byte) error           { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = StoreID(u); return nil }
func (id RetailerProductID) MarshalJSON() ([]byte, error)  { return json.Marshal(uuid.UUID(id)) }
func (id *RetailerProductID) UnmarshalJSON(b []byte) error { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = RetailerProductID(u); return nil }
func (id StoreProductOfferID) MarshalJSON() ([]byte, error) { return json.Marshal(uuid.UUID(id)) }
func (id *StoreProductOfferID) UnmarshalJSON(b []byte) error { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = StoreProductOfferID(u); return nil }
func (id PriceObservationID) MarshalJSON() ([]byte, error) { return json.Marshal(uuid.UUID(id)) }
func (id *PriceObservationID) UnmarshalJSON(b []byte) error { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = PriceObservationID(u); return nil }
func (id AccountID) MarshalJSON() ([]byte, error)          { return json.Marshal(uuid.UUID(id)) }
func (id *AccountID) UnmarshalJSON(b []byte) error          { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = AccountID(u); return nil }
func (id RecipeRefID) MarshalJSON() ([]byte, error)         { return json.Marshal(uuid.UUID(id)) }
func (id *RecipeRefID) UnmarshalJSON(b []byte) error         { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = RecipeRefID(u); return nil }
func (id RecipeFamilyID) MarshalJSON() ([]byte, error)      { return json.Marshal(uuid.UUID(id)) }
func (id *RecipeFamilyID) UnmarshalJSON(b []byte) error      { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = RecipeFamilyID(u); return nil }
func (id RecipeVariantID) MarshalJSON() ([]byte, error)     { return json.Marshal(uuid.UUID(id)) }
func (id *RecipeVariantID) UnmarshalJSON(b []byte) error     { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = RecipeVariantID(u); return nil }
func (id RecipeRevisionID) MarshalJSON() ([]byte, error)    { return json.Marshal(uuid.UUID(id)) }
func (id *RecipeRevisionID) UnmarshalJSON(b []byte) error    { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = RecipeRevisionID(u); return nil }
func (id RecipeSourceRefID) MarshalJSON() ([]byte, error)   { return json.Marshal(uuid.UUID(id)) }
func (id *RecipeSourceRefID) UnmarshalJSON(b []byte) error   { var u uuid.UUID; if err := json.Unmarshal(b, &u); err != nil { return err }; *id = RecipeSourceRefID(u); return nil }
