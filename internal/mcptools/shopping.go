package mcptools

import (
	"context"
	"fmt"
	"strings"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ── Shopping tool inputs ────────────────────────────────────────────────────

// CreateShoppingListInput is the input for the create_shopping_list tool.
type CreateShoppingListInput struct {
	// Name is the display name for the new shopping list.
	Name string `json:"name"`
	// Items are the shopping lines to add. Each is a canonical requirement
	// (ingredient + amount per line).
	Items []ShoppingRequirement `json:"items"`
}

// ComparePricesInput is the input for the compare_shopping_prices tool.
type ComparePricesInput struct {
	// Requirements are the canonical lines to compare across retailers.
	Requirements []ShoppingRequirement `json:"requirements"`
}

// JottedListInput is the input for the resolve_jotted_list tool: free-text
// shopping lines the person jotted by hand (no recipe to derive them from).
type JottedListInput struct {
	// Items are the free-text lines to price. Each carries a human label, a
	// quantity, and a unit separately so the amount is forwarded to the
	// comparison rather than swallowed into the name.
	Items []JottedListItem `json:"items"`
}

// JottedListItem is one free-text line a person jotted for shopping.
type JottedListItem struct {
	// Item is the human-written label, e.g. "kycklingfilé". It is normalized
	// to a canonical ingredient name before the comparison runs.
	Item string `json:"item"`
	// Quantity is the amount, forwarded to the comparison as-is.
	Quantity float64 `json:"quantity"`
	// Unit is the unit of quantity, forwarded to the comparison as-is.
	Unit string `json:"unit"`
}

// PushWishlistItem is one resolved line to push to a retailer wishlist.
type PushWishlistItem struct {
	// ProductCode is the retailer product id (EAN) to add.
	ProductCode string `json:"product_code"`
	// Quantity is the number of packages to add.
	Quantity int `json:"quantity"`
}

// PushWishlistInput is the input for the push_shopping_wishlist tool.
type PushWishlistInput struct {
	// Retailer is the backend: "willys" or "ica".
	Retailer string `json:"retailer"`
	// ListName is the display name for the retailer wishlist.
	ListName string `json:"list_name"`
	// Items are the resolved lines to push.
	Items []PushWishlistItem `json:"items"`
	// ShoppingListID is the spisordning shopping_list to bind the wishlist to.
	// Optional: when omitted, the wishlist is created but no binding is recorded.
	ShoppingListID *string `json:"shopping_list_id,omitempty"`
}

// ── Shopping tool outputs ───────────────────────────────────────────────────

// CreateShoppingListResult is the output of the create_shopping_list tool.
type CreateShoppingListResult struct {
	ListID string `json:"list_id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Items  int    `json:"items"`
}

// RetailerPriceResult is one retailer's outcome for a single requirement.
type RetailerPriceResult struct {
	Retailer   string   `json:"retailer"`
	Available  bool     `json:"available"`
	ProductID  *string  `json:"product_id,omitempty"`
	ProductName *string `json:"product_name,omitempty"`
	PriceValue *float64 `json:"price_value,omitempty"`
	Price      *string  `json:"price,omitempty"`
	// Error is set when the retailer's resolve call failed entirely (e.g. a
	// stale session) so the caller can say why it is unavailable.
	Error string `json:"error,omitempty"`
}

// ItemComparison is the cross-retailer comparison for one requirement.
type ItemComparison struct {
	Ingredient string                `json:"ingredient"`
	// Label echoes the original free-text line that produced this comparison.
	Label    string                `json:"label,omitempty"`
	Results    []RetailerPriceResult `json:"results"`
	Cheapest   *RetailerPriceResult  `json:"cheapest,omitempty"`
	Unresolved bool                  `json:"unresolved"`
}

// PriceComparison is the output of the compare_shopping_prices tool.
type PriceComparison struct {
	Items []ItemComparison `json:"items"`
}

// PushWishlistResult is the output of the push_shopping_wishlist tool.
type PushWishlistResult struct {
	Retailer       string `json:"retailer"`
	WishlistID     string `json:"wishlist_id"`
	ListName       string `json:"list_name"`
	Items          int    `json:"items"`
	ShoppingListID *string `json:"shopping_list_id,omitempty"`
}

// ── Shopping service interfaces ─────────────────────────────────────────────

// ShoppingListService creates a spisordning shopping list from requirements.
type ShoppingListService interface {
	CreateShoppingList(ctx context.Context, in CreateShoppingListInput) (CreateShoppingListResult, error)
}

// CoverageService checks whether a filled shopping list satisfies the
// ingredient needs of a meal plan (see the check_shopping_coverage tool). The
// MCP surface returns the MCP CoverageReport shape; the application-layer
// implementation translates the pure coverage.Report into it.
type CoverageService interface {
	CheckCoverage(ctx context.Context, in CheckCoverageInput) (CoverageReport, error)
}

// PriceComparisonService compares prices across retailers for a set of
// requirements. A stale or unavailable retailer degrades to available:false
// instead of failing the call.
type PriceComparisonService interface {
	ComparePrices(ctx context.Context, reqs []ShoppingRequirement) (PriceComparison, error)
}

// WishlistService pushes resolved lines to a retailer wishlist. It stops at
// the wishlist — it never fills a cart, checks out, or books a delivery slot.
type WishlistService interface {
	PushToWishlist(ctx context.Context, in PushWishlistInput) (PushWishlistResult, error)
}

// ── Shopping tool handlers ──────────────────────────────────────────────────

func createShoppingListHandler(s ShoppingListService) mcp.ToolHandlerFor[CreateShoppingListInput, CreateShoppingListResult] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in CreateShoppingListInput) (*mcp.CallToolResult, CreateShoppingListResult, error) {
		name := strings.TrimSpace(in.Name)
		if name == "" {
			return nil, CreateShoppingListResult{}, fmt.Errorf("create_shopping_list: name is required")
		}
		if len(in.Items) == 0 {
			return nil, CreateShoppingListResult{}, fmt.Errorf("create_shopping_list: at least one item is required")
		}
		res, err := s.CreateShoppingList(ctx, in)
		if err != nil {
			return nil, CreateShoppingListResult{}, err
		}
		return nil, res, nil
	}
}

func comparePricesHandler(s PriceComparisonService) mcp.ToolHandlerFor[ComparePricesInput, PriceComparison] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ComparePricesInput) (*mcp.CallToolResult, PriceComparison, error) {
		if len(in.Requirements) == 0 {
			return nil, PriceComparison{}, fmt.Errorf("compare_shopping_prices: at least one requirement is required")
		}
		res, err := s.ComparePrices(ctx, in.Requirements)
		if err != nil {
			return nil, PriceComparison{}, err
		}
		return nil, res, nil
	}
}

// resolveJottedListInput is the MCP wire shape for resolve_jotted_list; the
// handler maps each free-text item onto a canonical compare requirement.
func resolveJottedListHandler(s PriceComparisonService) mcp.ToolHandlerFor[JottedListInput, PriceComparison] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in JottedListInput) (*mcp.CallToolResult, PriceComparison, error) {
		if len(in.Items) == 0 {
			return nil, PriceComparison{}, fmt.Errorf("resolve_jotted_list: at least one item is required")
		}
		reqs := make([]ShoppingRequirement, 0, len(in.Items))
		for _, it := range in.Items {
			reqs = append(reqs, ShoppingRequirement{
				Ingredient: domain.CanonicalIngredientID(it.Item),
				Quantity:   it.Quantity,
				Unit:       it.Unit,
			})
		}
		res, err := s.ComparePrices(ctx, reqs)
		if err != nil {
			return nil, PriceComparison{}, err
		}
		labels := make([]string, len(in.Items))
		for i, it := range in.Items {
			labels[i] = it.Item
		}
		echoMCPLabels(res.Items, labels)
		return nil, res, nil
	}
}

// echoMCPLabels sets each result item's Label from the corresponding input
// free-text line. Mirrors the REST surface's echoLabels.
func echoMCPLabels(items []ItemComparison, labels []string) {
	for i := range items {
		if i < len(labels) {
			items[i].Label = labels[i]
		}
	}
}

func pushWishlistHandler(s WishlistService) mcp.ToolHandlerFor[PushWishlistInput, PushWishlistResult] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in PushWishlistInput) (*mcp.CallToolResult, PushWishlistResult, error) {
		if in.Retailer != "willys" && in.Retailer != "ica" {
			return nil, PushWishlistResult{}, fmt.Errorf("push_shopping_wishlist: retailer must be \"willys\" or \"ica\", got %q", in.Retailer)
		}
		if strings.TrimSpace(in.ListName) == "" {
			return nil, PushWishlistResult{}, fmt.Errorf("push_shopping_wishlist: list_name is required")
		}
		if len(in.Items) == 0 {
			return nil, PushWishlistResult{}, fmt.Errorf("push_shopping_wishlist: at least one item is required")
		}
		for _, it := range in.Items {
			if strings.TrimSpace(it.ProductCode) == "" {
				return nil, PushWishlistResult{}, fmt.Errorf("push_shopping_wishlist: every item needs a product_code")
			}
		}
		res, err := s.PushToWishlist(ctx, in)
		if err != nil {
			return nil, PushWishlistResult{}, err
		}
		return nil, res, nil
	}
}

// CheckCoverageInput is the input for the check_shopping_coverage tool.
type CheckCoverageInput struct {
	ShoppingListID string `json:"shopping_list_id"`
	PlanID         string `json:"plan_id"`
}

// CoverageLine is the verdict for one required line. Status is one of
// "covered", "short", or "missing". Shortfall is positive when supplied <
// required and zero when covered.
type CoverageLine struct {
	IngredientID   string  `json:"ingredient_id"`
	IngredientName string  `json:"ingredient_name"`
	Unit           string  `json:"unit"`
	Status         string  `json:"status"`
	Required       float64 `json:"required"`
	Supplied       float64 `json:"supplied"`
	Shortfall      float64 `json:"shortfall"`
}

// CoverageReport is the per-ingredient verdict for a list vs. a plan.
type CoverageReport struct {
	ShortCount     int           `json:"short_count"`
	MissingCount   int           `json:"missing_count"`
	NotPlanDerived int           `json:"not_plan_derived"`
	Lines          []CoverageLine `json:"lines"`
}

func checkCoverageHandler(s CoverageService) mcp.ToolHandlerFor[CheckCoverageInput, CoverageReport] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in CheckCoverageInput) (*mcp.CallToolResult, CoverageReport, error) {
		if strings.TrimSpace(in.ShoppingListID) == "" || strings.TrimSpace(in.PlanID) == "" {
			return nil, CoverageReport{}, fmt.Errorf("check_shopping_coverage: shopping_list_id and plan_id are both required")
		}
		res, err := s.CheckCoverage(ctx, in)
		if err != nil {
			return nil, CoverageReport{}, err
		}
		return nil, res, nil
	}
}
