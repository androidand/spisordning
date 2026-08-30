// Hand-maintained mirror of the Spisordning API contract.
//
// Source of truth: api/openapi.yaml (the design-first OpenAPI contract). The Go
// server types are generated from that file into internal/openapi/types.gen.go;
// this file is the frontend's equivalent, kept in lockstep by hand.
//
// Why hand-written instead of openapi-typescript: openapi-typescript generates
// types by calling the TypeScript compiler API (ts.factory.*), which the
// TypeScript 7 (native/Go) compiler does not expose as a JS module. Upstream
// issue openapi-ts/openapi-typescript#2841 tracks this; TS 7 support is
// deferred to TS 7.1. Until then the frontend is compiled with tsc 7 and this
// file is transcribed from the spec. When the spec changes, re-transcribe the
// affected schemas here (and check internal/openapi/types.gen.go for the
// authoritative wire shape).
//
// Note on `parameters`: OpenAPI allows parameters at both the path-item level
// and the operation level. openapi-typescript merges them onto each operation.
// We do the same here by inlining the path-level parameters into every
// operation's `parameters` key, so openapi-fetch's ParamsOption resolves the
// full param set (including `path`) per operation.

export type paths = {
  "/health": {
    get: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["Health"] };
        };
      };
    };
  };
  "/people": {
    get: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["Person"][] };
        };
      };
    };
    post: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      requestBody: {
        content: { "application/json": components["schemas"]["PersonNew"] };
      };
      responses: {
        201: {
          content: { "application/json": components["schemas"]["Person"] };
        };
      };
    };
  };
  "/people/{id}": {
    get: {
      parameters: { query?: never; header?: never; path: { id: string }; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["Person"] };
        };
        404: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
    patch: {
      parameters: { query?: never; header?: never; path: { id: string }; cookie?: never };
      requestBody: {
        content: { "application/json": components["schemas"]["PersonUpdate"] };
      };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["Person"] };
        };
        400: {
          content: { "application/json": components["schemas"]["Error"] };
        };
        404: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/preferences": {
    get: {
      parameters: { query?: { personId?: string }; header?: never; path?: never; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["PersonPreference"][] };
        };
      };
    };
    post: {
      requestBody: {
        content: { "application/json": components["schemas"]["PersonPreferenceNew"] };
      };
      responses: {
        201: {
          content: { "application/json": components["schemas"]["PersonPreference"] };
        };
        400: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/recipes": {
    get: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["RecipeRef"][] };
        };
      };
    };
  };
  "/recipes/{id}": {
    get: {
      parameters: { query?: never; header?: never; path: { id: string }; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["RecipeRef"] };
        };
        404: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/meals": {
    get: {
      parameters: {
        query?: { mealieRecipeId?: string | null; servedOn?: string | null };
        header?: never;
        path?: never;
        cookie?: never;
      };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["MealEvent"][] };
        };
      };
    };
    post: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      requestBody: {
        content: { "application/json": components["schemas"]["MealEventNew"] };
      };
      responses: {
        201: {
          content: { "application/json": components["schemas"]["MealEvent"] };
        };
      };
    };
  };
  "/meals/{id}": {
    get: {
      parameters: { query?: never; header?: never; path: { id: number }; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["MealEvent"] };
        };
        404: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/tonight": {
    get: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["TonightView"] };
        };
        404: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/reactions": {
    post: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      requestBody: {
        content: { "application/json": components["schemas"]["ReactionNew"] };
      };
      responses: {
        201: {
          content: { "application/json": components["schemas"]["MealReaction"] };
        };
      };
    };
  };
  "/plans": {
    get: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["MealPlan"][] };
        };
      };
    };
    post: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      requestBody: {
        content: { "application/json": components["schemas"]["MealPlanNew"] };
      };
      responses: {
        201: {
          content: { "application/json": components["schemas"]["MealPlan"] };
        };
      };
    };
  };
  "/plans/run": {
    post: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      requestBody?: {
        content: { "application/json": components["schemas"]["PlanRunInput"] };
      };
      responses: {
        202: {
          content: { "application/json": components["schemas"]["PlanRunResult"] };
        };
        500: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/plans/{planId}": {
    get: {
      parameters: { query?: never; header?: never; path: { planId: number }; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["MealPlanView"] };
        };
        404: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
    patch: {
      parameters: { query?: never; header?: never; path: { planId: number }; cookie?: never };
      requestBody: {
        content: { "application/json": components["schemas"]["MealPlanUpdate"] };
      };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["MealPlan"] };
        };
        409: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/plans/{planId}/decisions": {
    post: {
      parameters: { query?: never; header?: never; path: { planId: number }; cookie?: never };
      requestBody: {
        content: {
          "application/json": components["schemas"]["MealPlanDecisionInput"][];
        };
      };
      responses: {
        200: {
          content: {
            "application/json": components["schemas"]["MealPlanDecision"][];
          };
        };
        409: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/plans/{planId}/candidates": {
    get: {
      parameters: { query?: never; header?: never; path: { planId: number }; cookie?: never };
      responses: {
        200: {
          content: {
            "application/json": components["schemas"]["MealPlanCandidate"][];
          };
        };
      };
    };
  };
  "/plans/{planId}/shopping-requirements": {
    get: {
      parameters: { query?: never; header?: never; path: { planId: number }; cookie?: never };
      responses: {
        200: {
          content: {
            "application/json": components["schemas"]["ShoppingRequirement"][];
          };
        };
      };
    };
  };
  "/effort-profiles": {
    get: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["EffortProfile"][] };
        };
      };
    };
    post: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      requestBody: {
        content: { "application/json": components["schemas"]["EffortProfile"] };
      };
      responses: {
        201: { content?: never };
      };
    };
  };
  "/constraints": {
    get: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      responses: {
        200: {
          content: {
            "application/json": components["schemas"]["PlanningConstraint"][];
          };
        };
      };
    };
    post: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      requestBody: {
        content: { "application/json": components["schemas"]["PlanningConstraintNew"] };
      };
      responses: {
        201: {
          content: { "application/json": components["schemas"]["PlanningConstraint"] };
        };
      };
    };
  };
  "/ingredients/search": {
    get: {
      parameters: { query: { q: string; limit?: number }; header?: never; path?: never; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["Ingredient"][] };
        };
      };
    };
  };
  "/ingredients/{ingredientId}/nutrition": {
    get: {
      parameters: { query?: never; header?: never; path: { ingredientId: string }; cookie?: never };
      responses: {
        200: {
          content: {
            "application/json": components["schemas"]["IngredientNutrient"][];
          };
        };
      };
    };
  };
  "/ingredients/nutrition/{nummer}": {
    get: {
      parameters: { query?: never; header?: never; path: { nummer: number }; cookie?: never };
      responses: {
        200: {
          content: {
            "application/json": components["schemas"]["IngredientNutrient"][];
          };
        };
      };
    };
  };
  "/ingredients/dabas/search": {
    get: {
      parameters: { query: { q: string }; header?: never; path?: never; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["Ingredient"][] };
        };
      };
    };
  };
  "/ingredients/matpriskollen/search": {
    get: {
      parameters: { query: { q: string }; header?: never; path?: never; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["Ingredient"][] };
        };
      };
    };
  };
  "/stores": {
    get: {
      parameters: {
        query?: { latitude?: number; longitude?: number };
        header?: never;
        path?: never;
        cookie?: never;
      };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["Store"][] };
        };
        400: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/stores/{id}/offers": {
    get: {
      parameters: { query?: never; header?: never; path: { id: string }; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["StoreOffer"][] };
        };
      };
    };
  };
  "/products/search": {
    get: {
      parameters: { query: { q: string }; header?: never; path?: never; cookie?: never };
      responses: {
        200: {
          content: {
            "application/json": components["schemas"]["IngredientProduct"][];
          };
        };
      };
    };
  };
  "/products/by-gtin": {
    get: {
      parameters: { query: { gtin: string }; header?: never; path?: never; cookie?: never };
      responses: {
        200: {
          content: {
            "application/json": components["schemas"]["IngredientProduct"][];
          };
        };
      };
    };
  };
  "/ingredient-mappings/{mealieFoodId}": {
    get: {
      parameters: { query?: never; header?: never; path: { mealieFoodId: string }; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["IngredientMapping"] };
        };
        404: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
    patch: {
      parameters: { query?: never; header?: never; path: { mealieFoodId: string }; cookie?: never };
      requestBody: {
        content: {
          "application/json": components["schemas"]["IngredientMappingResolve"];
        };
      };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["IngredientMapping"] };
        };
        404: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/shopping-lists": {
    get: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["ShoppingList"][] };
        };
      };
    };
    post: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      requestBody: {
        content: { "application/json": components["schemas"]["ShoppingListNew"] };
      };
      responses: {
        201: {
          content: { "application/json": components["schemas"]["ShoppingList"] };
        };
      };
    };
  };
  "/shopping-lists/from-checklist": {
    post: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      requestBody: {
        content: { "application/json": components["schemas"]["ShoppingListFromChecklistNew"] };
      };
      responses: {
        201: {
          content: { "application/json": components["schemas"]["ShoppingListFromChecklist"] };
        };
      };
    };
  };
  "/shopping-lists/{listId}": {
    get: {
      parameters: { query?: never; header?: never; path: { listId: number }; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["ShoppingList"] };
        };
        404: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
    delete: {
      parameters: { query?: never; header?: never; path: { listId: number }; cookie?: never };
      responses: {
        200: { content?: never };
        404: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/shopping-lists/{listId}/items": {
    get: {
      parameters: { query?: never; header?: never; path: { listId: number }; cookie?: never };
      responses: {
        200: {
          content: {
            "application/json": components["schemas"]["ShoppingListItem"][];
          };
        };
      };
    };
    post: {
      parameters: { query?: never; header?: never; path: { listId: number }; cookie?: never };
      requestBody: {
        content: { "application/json": components["schemas"]["ShoppingListItemNew"] };
      };
      responses: {
        201: {
          content: { "application/json": components["schemas"]["ShoppingListItem"] };
        };
      };
    };
  };
  "/shopping-lists/{listId}/items/{itemId}": {
    patch: {
      parameters: {
        query?: never;
        header?: never;
        path: { listId: number; itemId: number };
        cookie?: never;
      };
      requestBody: {
        content: { "application/json": { checked: boolean } };
      };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["ShoppingListItem"] };
        };
        404: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
    delete: {
      parameters: {
        query?: never;
        header?: never;
        path: { listId: number; itemId: number };
        cookie?: never;
      };
      responses: {
        204: { content?: never };
        404: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/shopping-lists/{listId}/push": {
    post: {
      parameters: {
        query?: { retailer?: string };
        header?: never;
        path: { listId: number };
        cookie?: never;
      };
      responses: {
        200: {
          content: {
            "application/json": components["schemas"]["RetailerListBinding"];
          };
        };
        404: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/shopping-lists/{listId}/carts": {
    get: {
      parameters: { query?: never; header?: never; path: { listId: number }; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["ShoppingCart"][] };
        };
      };
    };
  };
  "/shopping-lists/{listId}/push/to-cart": {
    post: {
      parameters: {
        query?: { retailer?: string };
        header?: never;
        path: { listId: number };
        cookie?: never;
      };
      responses: {
        201: {
          content: { "application/json": components["schemas"]["ShoppingCart"] };
        };
        404: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/orders": {
    get: {
      parameters: {
        query?: { retailer?: string | null; cartId?: number | null };
        header?: never;
        path?: never;
        cookie?: never;
      };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["Order"][] };
        };
      };
    };
  };
  "/orders/{orderId}": {
    get: {
      parameters: { query?: never; header?: never; path: { orderId: number }; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["OrderView"] };
        };
        404: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/orders/{orderId}/items": {
    get: {
      parameters: { query?: never; header?: never; path: { orderId: number }; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["OrderItem"][] };
        };
      };
    };
  };
  "/pantry/locations": {
    get: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["InventoryLocation"][] };
        };
      };
    };
    post: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      requestBody: {
        content: { "application/json": components["schemas"]["InventoryLocationNew"] };
      };
      responses: {
        201: {
          content: { "application/json": components["schemas"]["InventoryLocation"] };
        };
      };
    };
  };
  "/pantry/locations/{id}/lots": {
    get: {
      parameters: { query?: never; header?: never; path: { id: string }; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["InventoryLot"][] };
        };
      };
    };
  };
  "/pantry/lots/purchase": {
    post: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      requestBody: {
        content: { "application/json": components["schemas"]["PurchaseNew"] };
      };
      responses: {
        201: {
          content: { "application/json": components["schemas"]["InventoryLot"] };
        };
      };
    };
  };
  "/pantry/lots/{id}/consume": {
    post: {
      parameters: { query?: never; header?: never; path: { id: string }; cookie?: never };
      requestBody: {
        content: { "application/json": components["schemas"]["ConsumeNew"] };
      };
      responses: {
        200: {
          content: { "application/json": { status: string } };
        };
        404: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/pantry/lots/{id}/discard": {
    post: {
      parameters: { query?: never; header?: never; path: { id: string }; cookie?: never };
      requestBody: {
        content: { "application/json": components["schemas"]["PantryDiscardNew"] };
      };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["PantryLot"] };
        };
        400: {
          content: { "application/json": components["schemas"]["Error"] };
        };
        404: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/pantry/lots/{id}/adjust": {
    post: {
      parameters: { query?: never; header?: never; path: { id: string }; cookie?: never };
      requestBody: {
        content: { "application/json": components["schemas"]["PantryAdjustNew"] };
      };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["PantryLot"] };
        };
        400: {
          content: { "application/json": components["schemas"]["Error"] };
        };
        404: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/pantry/lots/{id}/mark-empty": {
    post: {
      parameters: { query?: never; header?: never; path: { id: string }; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["PantryLot"] };
        };
        404: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/pantry/lots/{id}/open": {
    post: {
      parameters: { query?: never; header?: never; path: { id: string }; cookie?: never };
      requestBody: {
        content: { "application/json": components["schemas"]["PantryOpenNew"] };
      };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["PantryLot"] };
        };
        400: {
          content: { "application/json": components["schemas"]["Error"] };
        };
        404: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/pantry/lots/{id}/transfer": {
    post: {
      parameters: { query?: never; header?: never; path: { id: string }; cookie?: never };
      requestBody: {
        content: { "application/json": components["schemas"]["PantryTransferNew"] };
      };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["PantryLot"] };
        };
        400: {
          content: { "application/json": components["schemas"]["Error"] };
        };
        404: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/compare": {
    post: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      requestBody: {
        content: { "application/json": components["schemas"]["CompareInput"] };
      };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["PriceComparison"] };
        };
        400: {
          content: { "application/json": components["schemas"]["Error"] };
        };
        500: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/recipe-families": {
    get: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["RecipeFamily"][] };
        };
      };
    };
    post: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      requestBody: {
        content: { "application/json": components["schemas"]["RecipeFamilyNew"] };
      };
      responses: {
        201: {
          content: { "application/json": components["schemas"]["RecipeFamily"] };
        };
      };
    };
  };
  "/recipe-families/{familyId}": {
    get: {
      parameters: { query?: never; header?: never; path: { familyId: string }; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["RecipeFamily"] };
        };
        404: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/recipe-families/{familyId}/variants": {
    get: {
      parameters: { query?: never; header?: never; path: { familyId: string }; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["RecipeVariant"][] };
        };
        404: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
    post: {
      parameters: { query?: never; header?: never; path: { familyId: string }; cookie?: never };
      requestBody: {
        content: { "application/json": components["schemas"]["RecipeVariantNew"] };
      };
      responses: {
        201: {
          content: { "application/json": components["schemas"]["RecipeVariant"] };
        };
        404: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/recipe-families/{familyId}/variants/{variantId}/revisions": {
    get: {
      parameters: {
        query?: never;
        header?: never;
        path: { familyId: string; variantId: string };
        cookie?: never;
      };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["RecipeRevision"][] };
        };
        404: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
    post: {
      parameters: {
        query?: never;
        header?: never;
        path: { familyId: string; variantId: string };
        cookie?: never;
      };
      requestBody: {
        content: { "application/json": components["schemas"]["RecipeRevisionNew"] };
      };
      responses: {
        201: {
          content: { "application/json": components["schemas"]["RecipeRevision"] };
        };
        404: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/recipe-families/{familyId}/variants/{variantId}/revisions/{revisionId}": {
    get: {
      parameters: {
        query?: never;
        header?: never;
        path: { familyId: string; variantId: string; revisionId: number };
        cookie?: never;
      };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["RecipeRevision"] };
        };
        404: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/recipe-families/{familyId}/variants/{variantId}/default": {
    post: {
      parameters: {
        query?: never;
        header?: never;
        path: { familyId: string; variantId: string };
        cookie?: never;
      };
      responses: {
        200: {
          content: { "application/json": { status: string } };
        };
        404: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/recipes/{id}/favorites": {
    get: {
      parameters: { query?: never; header?: never; path: { id: string }; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["Favorite"][] };
        };
      };
    };
    post: {
      parameters: { query?: never; header?: never; path: { id: string }; cookie?: never };
      requestBody: {
        content: { "application/json": components["schemas"]["FavoriteNew"] };
      };
      responses: {
        200: {
          content: { "application/json": { status: string } };
        };
      };
    };
    delete: {
      parameters: { query?: never; header?: never; path: { id: string }; cookie?: never };
      requestBody: {
        content: { "application/json": components["schemas"]["FavoriteNew"] };
      };
      responses: {
        200: {
          content: { "application/json": { status: string } };
        };
      };
    };
  };
  "/recipes/{id}/rating": {
    get: {
      parameters: { query?: never; header?: never; path: { id: string }; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["RecipeRating"] };
        };
      };
    };
  };
  "/prices": {
    get: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["ProductPriceGroup"][] };
        };
      };
    };
  };
  "/widgets/dashboard": {
    get: {
      parameters: {
        query?: { householdId?: string };
        header?: never;
        path?: never;
        cookie?: never;
      };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["Dashboard"] };
        };
      };
    };
  };
  "/inspiration": {
    get: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["InspirationSuggestion"][] };
        };
      };
    };
  };
  "/grocy/status": {
    get: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["GrocyStatus"] };
        };
      };
    };
  };
  "/grocy/products": {
    get: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["GrocyProduct"][] };
        };
        502: {
          content: { "application/json": components["schemas"]["Error"] };
        };
        503: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/grocy/stock": {
    get: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["GrocyStockEntry"][] };
        };
        502: {
          content: { "application/json": components["schemas"]["Error"] };
        };
        503: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/grocy/shopping-list": {
    get: {
      parameters: { query?: never; header?: never; path?: never; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["GrocyShoppingItem"][] };
        };
        502: {
          content: { "application/json": components["schemas"]["Error"] };
        };
        503: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/grocy/stock/add": {
    post: {
      requestBody: {
        content: {
          "application/json": {
            product_id: number;
            amount: number;
            best_before?: string;
          };
        };
      };
      responses: {
        200: { content: { "application/json": { status: string } } };
        400: { content: { "application/json": components["schemas"]["Error"] } };
        502: { content: { "application/json": components["schemas"]["Error"] } };
        503: { content: { "application/json": components["schemas"]["Error"] } };
      };
    };
  };
  "/grocy/stock/consume": {
    post: {
      requestBody: {
        content: {
          "application/json": {
            product_id: number;
            amount: number;
          };
        };
      };
      responses: {
        200: { content: { "application/json": { status: string } } };
        400: { content: { "application/json": components["schemas"]["Error"] } };
        502: { content: { "application/json": components["schemas"]["Error"] } };
        503: { content: { "application/json": components["schemas"]["Error"] } };
      };
    };
  };
  "/grocy/shopping-list/items": {
    post: {
      requestBody: {
        content: {
          "application/json": {
            product_id?: number;
            note?: string;
            amount: number;
          };
        };
      };
      responses: {
        200: { content: { "application/json": { status: string } } };
        400: { content: { "application/json": components["schemas"]["Error"] } };
        502: { content: { "application/json": components["schemas"]["Error"] } };
        503: { content: { "application/json": components["schemas"]["Error"] } };
      };
    };
  };
  "/ingredient-aliases": {
    get: {
      parameters: {
        query?: { householdId?: string };
        header?: never;
        path?: never;
        cookie?: never;
      };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["IngredientAlias"][] };
        };
      };
    };
    post: {
      requestBody: {
        content: { "application/json": components["schemas"]["IngredientAliasNew"] };
      };
      responses: {
        201: {
          content: { "application/json": components["schemas"]["IngredientAlias"] };
        };
        400: {
          content: { "application/json": components["schemas"]["Error"] };
        };
      };
    };
  };
  "/ingredient-aliases/resolve/{alias}": {
    get: {
      parameters: {
        query?: { householdId?: string };
        header?: never;
        path: { alias: string };
        cookie?: never;
      };
      responses: {
        200: {
          content: { "application/json": { ingredient_id: string } };
        };
      };
    };
  };
  "/ingredient-aliases/{alias}": {
    delete: {
      parameters: {
        query?: { householdId?: string };
        header?: never;
        path: { alias: string };
        cookie?: never;
      };
      responses: {
        200: {
          content: { "application/json": { status: string } };
        };
      };
    };
  };
  "/pantry/expiring": {
    get: {
      parameters: { query?: { withinHours?: number }; header?: never; path?: never; cookie?: never };
      responses: {
        200: {
          content: { "application/json": components["schemas"]["InventoryLot"][] };
        };
      };
    };
  };
};

export type webhooks = Record<string, never>;

export interface components {
  schemas: {
    /** Error — resource not found / state conflict body. */
    Error: {
      message: string;
    };

    /** Health — liveness/ready probe. */
    Health: {
      status?: "ok";
    };

    /** Person — a household member. */
    Person: {
      /** canonical id */
      id: string;
      name: string;
      weight: number;
      created_at: string;
    };

    /** PersonNew — create a person. */
    PersonNew: {
      name: string;
      weight?: number;
    };

    /** PersonUpdate — request body for updating a person. */
    PersonUpdate: {
      name: string;
      /** 0/omitted leaves the existing weight unchanged */
      weight?: number;
    };

    /** PersonPreference — a learned preference for a person. */
    PersonPreference: {
      person_id: string;
      tag: string;
      /** -2 (strongly dislike) to 2 (love) */
      sentiment: number;
      confidence: number;
      updated_at: string;
    };

    /** PersonPreferenceNew — request body for setting a preference. */
    PersonPreferenceNew: {
      person_id: string;
      tag: string;
      /** -2 (strongly dislike) to 2 (love) */
      sentiment: number;
      confidence: number;
    };

    /** RecipeRef — a known recipe (Mealie-backed). */
    RecipeRef: {
      mealie_recipe_id: string;
      title: string;
      tags: string[];
      /** 1 (quick) to 3 (intense) */
      effort: number;
      last_synced_at: string;
    };

    /** Ingredient — a canonical food/ingredient. */
    Ingredient: {
      /** canonical id, e.g. slv-12345 */
      id: string;
      /** human-friendly name */
      display: string;
      /** upstream source (slv, dabas, matpriskollen) */
      source?: string;
      /** SLV nummer when source is slv */
      slv_nummer?: number;
      gtin?: string;
      brand?: string;
    };

    /** IngredientNutrient — a nutrient value per 100 g edible portion. */
    IngredientNutrient: {
      /** nutrient name, e.g. Energi */
      name: string;
      /** value per 100 g edible portion */
      value: number;
      /** unit, e.g. kJ, g, mg */
      unit: string;
    };

    /** IngredientProduct — a retail product (Matpriskollen). */
    IngredientProduct: {
      key: string;
      gtin?: string;
      name: string;
      brand?: string;
      description?: string;
      amount?: string;
      image_url?: string;
    };

    /** Store — a physical store for a retailer, with optional position + distance. */
    Store: {
      id: string;
      retailer_id: string;
      retailer_name?: string;
      name: string;
      latitude?: number | null;
      longitude?: number | null;
      /** Great-circle distance from the request origin in km; null when unmapped. */
      distance_km?: number | null;
    };

    /** Dashboard — aggregate read model: tonight, pantry summary, expiring. */
    Dashboard: {
      tonight?: components["schemas"]["DashboardTonight"] | null;
      pantry: components["schemas"]["DashboardPantry"];
      expiring: components["schemas"]["DashboardExpiringLot"][];
    };

    /** DashboardTonight — tonight's meal (null when nothing is planned). */
    DashboardTonight: {
      served_on: string;
      recipe: components["schemas"]["RecipeRef"];
    };

    /** DashboardPantry — pantry summary counts. */
    DashboardPantry: {
      locations: number;
      lots: number;
      expiring: number;
    };

    /** DashboardExpiringLot — one pantry lot that is expired or expiring soon. */
    DashboardExpiringLot: {
      ingredient_id: string;
      quantity: number;
      unit: string;
      best_before?: string | null;
    };

    /** GrocyStatus — whether a Grocy instance is configured and reachable. */
    GrocyStatus: {
      configured: boolean;
      reachable: boolean;
      version?: string;
      base_url?: string;
    };

    /** GrocyProduct — a product in the Grocy catalog. */
    GrocyProduct: {
      id: number;
      name: string;
      barcode?: string;
      location_id?: number;
      qu_id_stock?: number;
      qu_id_purchase?: number;
      min_stock_amount?: number;
    };

    /** GrocyStockEntry — one Grocy stock lot (zero-amount lots excluded). */
    GrocyStockEntry: {
      id: number;
      product_id: number;
      product_name: string;
      amount: number;
      qu_id: number;
      location_id: number;
      best_before?: string;
    };

    /** GrocyShoppingItem — one line of the Grocy shopping list. */
    GrocyShoppingItem: {
      id: number;
      /** 0 for a free-text item */
      product_id: number;
      note: string;
      amount: number;
      qu_id: number;
      done: boolean;
    };

    /** InspirationSuggestion — a recipe ranked by pantry coverage. */
    InspirationSuggestion: {
      mealie_recipe_id: string;
      title: string;
      tags: string[];
      effort: number;
      total_ingredients: number;
      /** recipe ingredients already in the pantry */
      matched_ingredient_ids: string[];
      /** recipe ingredients still needed */
      missing_ingredient_ids: string[];
      /** 1 = fully cookable from pantry */
      match_ratio: number;
    };

    /** IngredientAlias — a household nickname → canonical ingredient mapping. */
    IngredientAlias: {
      id: number;
      household_id?: string | null;
      alias: string;
      ingredient_id: string;
      created_at: string;
    };

    /** IngredientAliasNew — request body for adding a nickname mapping. */
    IngredientAliasNew: {
      household_id?: string;
      alias: string;
      ingredient_id: string;
    };

    /** StoreOffer — whether a store carries a retailer product. */
    StoreOffer: {
      id: number;
      store_id: string;
      retailer_product_id: string;
      currently_carried: boolean;
      updated_at: string;
    };

    /** IngredientMapping — maps a Mealie food to a canonical ingredient. */
    IngredientMapping: {
      mealie_food_id: string;
      /** canonical ingredient id (see Ingredient.id) */
      ingredient_id: string;
      source_name: string;
      external_id?: string | null;
      needs_review: boolean;
      updated_at: string;
    };

    /** IngredientMappingResolve — resolve a needs_review mapping. */
    IngredientMappingResolve: {
      ingredient_id: string;
      acceptable_forms?: string[];
      preferred_form?: string | null;
    };

    /** MealEvent — a served meal. */
    MealEvent: {
      id: number;
      mealie_recipe_id: string;
      served_on: string;
      created_at: string;
      reactions: components["schemas"]["MealReaction"][];
    };

    /** MealReaction — one person's reaction to a meal. */
    MealReaction: {
      person_id: string;
      sentiment: number;
    };

    /** MealEventNew — record a served meal. */
    MealEventNew: {
      mealie_recipe_id: string;
      served_on: string;
      reactions?: components["schemas"]["MealReaction"][];
    };

    /** MealPlan — a weekly meal plan. */
    MealPlan: {
      id: number;
      /** Monday of the planned week */
      week_start: string;
      status: "draft" | "approved" | "archived";
      created_at: string;
    };

    /** MealPlanNew — create a plan for a week. */
    MealPlanNew: {
      week_start?: string;
    };

    /** MealPlanUpdate — update a plan's status. */
    MealPlanUpdate: {
      status?: "draft" | "approved" | "archived";
    };

    /** MealPlanCandidate — a ranked candidate for a plan slot. */
    MealPlanCandidate: {
      id: number;
      recipe: components["schemas"]["RecipeRef"];
      slot_date: string;
      score: number;
      /** per-signal score breakdown (pref, effort, repeat, school, campaign, familiarity, novelty) */
      breakdown: Record<string, number>;
      feasible: boolean;
      rank: number;
    };

    /** MealPlanDecision — a committed slot decision. */
    MealPlanDecision: {
      plan_id: number;
      slot_date: string;
      mealie_recipe_id: string;
      decided_at?: string | null;
    };

    /** MealPlanDecisionInput — request body for POST /plans/{planId}/decisions. */
    MealPlanDecisionInput: {
      slot_date: string;
      mealie_recipe_id: string;
    };

    /** MealPlanView — a plan with its ranked candidates. */
    MealPlanView: {
      plan: components["schemas"]["MealPlan"];
      candidates: components["schemas"]["MealPlanCandidate"][];
      decisions?: components["schemas"]["MealPlanDecision"][];
    };

    /** ShoppingRequirement — a canonical shopping requirement. */
    ShoppingRequirement: {
      id: number;
      ingredient_id: string;
      quantity: number;
      unit: string;
      acceptable_forms: string[];
      preferred_form?: string | null;
    };

    /** EffortProfile — kitchen energy per weekday. */
    EffortProfile: {
      /** 0=Sunday .. 6=Saturday */
      weekday: number;
      kitchen_energy: number;
    };

    /** TonightView — tonight's meal + reactions so far. */
    TonightView: {
      /** today's date */
      served_on: string;
      recipe: components["schemas"]["RecipeRef"];
      /** reactions already recorded for today's meal event */
      reactions: components["schemas"]["MealReaction"][];
    };

    /** PlanningConstraint — a planning constraint. */
    PlanningConstraint: {
      id: number;
      /** constraint kind (e.g. avoid_tag, max_effort) */
      kind: string;
      /** constraint value */
      value: string;
      /** whether the constraint is active */
      active: boolean;
    };

    /** PlanningConstraintNew — create a planning constraint. */
    PlanningConstraintNew: {
      kind: string;
      value: string;
      active: boolean;
    };

    /** ReactionNew — one-tap reaction to a meal. */
    ReactionNew: {
      /** who is reacting */
      person_id: string;
      /** -2 (hate) .. 2 (love) */
      sentiment: number;
      /** optional free-text note */
      note?: string | null;
    };

    /** PlanRunInput — run the weekly planner. */
    PlanRunInput: {
      /** ISO week, e.g. 2026-W31 (default: next week) */
      week?: string;
      /** number of dinners (default: 7) */
      days?: number;
      /** resolve products and create Willys wishlist */
      create_wishlist?: boolean;
    };

    /** PlanRunResult — result of a plan run. */
    PlanRunResult: {
      /** accepted = queued, failed = error */
      status: "accepted" | "failed";
      /** human-readable result summary */
      message: string;
      /** Monday of the planned week */
      week_start?: string | null;
    };

    /**
     * PlanProgress — one SSE progress event for a running plan
     * (POST /plans/run/stream). Payload shape not finalized until the
     * frontend's SSE consumer needs it (task 3.4).
     */
    PlanProgress: {
      /** the phase the run has reached */
      phase: "started" | "planning" | "resolving" | "wishlist" | "done";
      /** human-readable progress detail */
      message: string;
      /** when the phase was reached */
      at: string;
    };

    /** ShoppingList — a shopping list. */
    ShoppingList: {
      id: number;
      /** person who owns the list; null = shared */
      owner_person_id?: string | null;
      name: string;
      status: "active" | "archived";
      created_at: string;
    };

    /** ShoppingListNew — create a shopping list. */
    ShoppingListNew: {
      name: string;
      owner_person_id?: string | null;
    };

    /** ShoppingListItem — an item on a shopping list. */
    ShoppingListItem: {
      id: number;
      shopping_list_id: number;
      shopping_requirement_id?: number | null;
      ingredient_id?: string | null;
      label?: string | null;
      quantity: number;
      unit: string;
      checked: boolean;
      added_at: string;
    };

    /** ShoppingListItemNew — add an item to a shopping list. */
    ShoppingListItemNew: {
      shopping_requirement_id?: number | null;
      ingredient_id?: string | null;
      label?: string | null;
      quantity: number;
      unit: string;
    };

    /** RetailerListBinding — a list bound to a retailer. */
    RetailerListBinding: {
      id: number;
      shopping_list_id: number;
      retailer: string;
      external_list_id: string;
      sync_direction: "outbound";
      last_pushed_at?: string | null;
      last_push_status?: "success" | "error" | null;
    };

    /** ShoppingCart — a cart checkpoint. */
    ShoppingCart: {
      id: number;
      retailer_list_binding_id: number;
      created_at: string;
      status: "created" | "confirmed" | "abandoned";
    };

    /** ShoppingCartItem — an item in a cart. */
    ShoppingCartItem: {
      id: number;
      shopping_cart_id: number;
      retailer_product_id: string;
      quantity: number;
      unit: string;
      resolved_price?: number | null;
    };

    /** Order — a placed order. */
    Order: {
      id: number;
      shopping_cart_id?: number | null;
      retailer: string;
      source: "manual" | "retailer_api" | "receipt_import";
      ordered_at: string;
      total_price?: number | null;
    };

    /** OrderView — an order with its items. */
    OrderView: {
      order: components["schemas"]["Order"];
      items: components["schemas"]["OrderItem"][];
    };

    /** OrderItem — an item on an order. */
    OrderItem: {
      id: number;
      order_id: number;
      retailer_product_id: string;
      quantity: number;
      unit_price?: number | null;
      total_price?: number | null;
      substituted_for_item_id?: number | null;
    };

    /** InventoryLocation — a pantry location (fridge, freezer, cupboard). */
    InventoryLocation: {
      id: string;
      household_id: string;
      name: string;
      location_type: string;
      parent_location_id: string;
      archived_at?: string;
    };

    /** InventoryLocationNew — create a pantry location. */
    InventoryLocationNew: {
      household_id: string;
      name: string;
      location_type: string;
      parent_location_id: string;
    };

    /** InventoryLot — a quantity of an ingredient at a location. */
    InventoryLot: {
      id: string;
      ingredient_id: string;
      product_id?: string | null;
      location_id: string;
      quantity: number;
      unit: string;
      confidence: string;
      best_before?: string;
      opened_at?: string;
      created_at: string;
      updated_at: string;
    };

    /** PantryLot — pantry lot returned by the pantry event ledger. */
    PantryLot: {
      id: string;
      ingredient_id: string;
      product_id?: string | null;
      location_id: string;
      quantity: number;
      unit: string;
      confidence: string;
      best_before?: string;
      opened_at?: string;
      created_at: string;
      updated_at: string;
    };

    /** PurchaseNew — record a purchase into the pantry. */
    PurchaseNew: {
      ingredient_id: string;
      product_id: string;
      location_id: string;
      quantity: number;
      unit: string;
      best_before?: string;
      source: string;
    };

    /** ConsumeNew — consume quantity from a lot. */
    ConsumeNew: {
      quantity: number;
      estimated: boolean;
      source: string;
    };

    /** PantryDiscardNew — discard part or all of a pantry lot. */
    PantryDiscardNew: {
      quantity: number;
      estimated?: boolean;
      reason?: string;
      source: string;
    };

    /** PantryAdjustNew — correct the observed quantity of a pantry lot. */
    PantryAdjustNew: {
      quantity: number;
      estimated?: boolean;
      reason?: string;
      source: string;
    };

    /** PantryOpenNew — record that a sealed pantry lot was opened. */
    PantryOpenNew: {
      source: string;
    };

    /** PantryTransferNew — move part or all of a pantry lot. */
    PantryTransferNew: {
      location_id: string;
      quantity: number;
      source: string;
    };

    /** ShoppingListFromChecklistNew — ingest a checklist (Apple Notes bridge). */
    ShoppingListFromChecklistNew: {
      name: string;
      items: { label: string; quantity: number; unit: string }[];
    };

    /** ShoppingListFromChecklist — a list created from a checklist + items. */
    ShoppingListFromChecklist: {
      id: number;
      name: string;
      status: "active" | "archived";
      created_at: string;
      items: components["schemas"]["ShoppingListItem"][];
    };

    /** CompareRequirement — one canonical line to compare across retailers. */
    CompareRequirement: {
      ingredient: string;
      quantity: number;
      unit: string;
      acceptable_forms?: string[];
      preferred_form?: string;
    };

    /** CompareInput — request body for POST /compare. */
    CompareInput: {
      requirements: components["schemas"]["CompareRequirement"][];
    };

    /** RetailerPriceResult — one retailer's outcome for a requirement. */
    RetailerPriceResult: {
      retailer: string;
      available: boolean;
      product_id?: string | null;
      product_name?: string | null;
      price_value?: number | null;
      price?: string | null;
      error?: string;
    };

    /** ItemComparison — cross-retailer comparison for one requirement. */
    ItemComparison: {
      ingredient: string;
      results: components["schemas"]["RetailerPriceResult"][];
      cheapest?: components["schemas"]["RetailerPriceResult"] | null;
      unresolved: boolean;
    };

    /** PriceComparison — response body for POST /compare. */
    PriceComparison: {
      items: components["schemas"]["ItemComparison"][];
    };

    /** RecipeFamily — a conceptual dish (git-like hierarchy root). */
    RecipeFamily: {
      id: string;
      name: string;
      description?: string | null;
      default_variant_id?: string | null;
      archived: boolean;
      created_at: string;
    };

    /** RecipeFamilyNew — create a recipe family. */
    RecipeFamilyNew: {
      id?: string;
      name: string;
      description?: string;
    };

    /** RecipeVariant — one recognizable fork of a family. */
    RecipeVariant: {
      id: string;
      family_id: string;
      title: string;
      source_attribution?: string | null;
      archived: boolean;
      created_at: string;
    };

    /** RecipeVariantNew — create a variant of a family. */
    RecipeVariantNew: {
      id?: string;
      title: string;
      source_attribution?: string;
    };

    /** RecipeIngredient — one structured ingredient line of a revision. */
    RecipeIngredient: {
      ingredient_id: string;
      quantity: number;
      unit: string;
      raw_text?: string;
      acceptable_forms?: string[];
      preferred_form?: string;
    };

    /** RecipeRevision — one immutable snapshot of a variant's content. */
    RecipeRevision: {
      id: number;
      variant_id: string;
      servings?: number | null;
      description?: string | null;
      ingredients: components["schemas"]["RecipeIngredient"][];
      steps: string[];
      parents?: number[];
      created_at: string;
    };

    /** RecipeRevisionNew — create a revision of a variant. */
    RecipeRevisionNew: {
      servings?: number;
      description?: string;
      ingredients: components["schemas"]["RecipeIngredient"][];
      steps: string[];
      parent_revision_id?: number | null;
    };

    /** Favorite — an explicit favorite marker on a recipe. */
    Favorite: {
      id: number;
      person_id?: string | null;
      household_id?: string | null;
      mealie_recipe_id: string;
      created_at: string;
    };

    /** FavoriteNew — mark/remove a favorite (exactly one scope set). */
    FavoriteNew: {
      person_id?: string;
      household_id?: string;
    };

    /** RecipeRating — weighted average rating for a recipe. */
    RecipeRating: {
      mealie_recipe_id: string;
      average: number;
      review_count: number;
    };

    /** StorePrice — current price of a retailer product at one store. */
    StorePrice: {
      store_id: string;
      store_name?: string;
      retailer_id: string;
      retailer_name?: string;
      price_kind: "regular" | "member" | "campaign";
      price: number;
      observed_at: string;
      source: string;
    };

    /** ProductPriceGroup — current prices for one retailer product across stores. */
    ProductPriceGroup: {
      retailer_product_id: string;
      product_id?: string | null;
      display_name?: string | null;
      retailer_id: string;
      retailer_name?: string;
      prices: components["schemas"]["StorePrice"][];
      cheapest?: components["schemas"]["StorePrice"] | null;
    };
  };
  responses: never;
  parameters: never;
  requestBodies: never;
  headers: never;
};

export type $defs = Record<string, never>;
