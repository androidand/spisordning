package dto

import "errors"

// ErrNotFound is the contract-layer sentinel services return when a resource
// does not exist. Transports (httpapi, mcp) map it to their own not-found
// responses without services importing transport packages.
var ErrNotFound = errors.New("not found")

// ErrInvalid is the contract-layer sentinel services return when request input
// is invalid. Transports map it to their own bad-request responses without
// services importing transport packages.
var ErrInvalid = errors.New("invalid input")
