package httpapi

// FoodSearchResult is the response shape for ingredient/fusion lookups.
// Shaped to match the OpenAPI spec's expected output for /ingredients/search.
type FoodSearchResult struct {
	ID       string `json:"id"`
	Display  string `json:"display"`
	Source   string `json:"source"`
	SlvNummer int   `json:"slv_nummer,omitempty"`
}
