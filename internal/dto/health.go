package dto

// Health is the response body for GET /health (matches api/openapi.yaml).
type Health struct {
	Status string `json:"status"`
}
