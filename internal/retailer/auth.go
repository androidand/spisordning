package retailer

import (
	"errors"
	"net/http"

	"github.com/androidand/spisordning/internal/httpclient"
)

// AuthTier identifies the authentication level a retailer operation requires.
//
// AuthBasic operations work against the retailer's anonymous public surface
// (e.g. ecom product search) and never go stale. AuthElevated operations
// require a live, human-obtained session credential (e.g. an OAuth2 session
// from a manual web login) that can go stale; callers must detect staleness
// (see IsElevatedStale) and surface it rather than silently degrading.
//
// The tier concept is generic across retailers; ICA is the first real
// tiered instance (see TierFor). Willys and Hemköp are single-tier (always
// AuthBasic) and are unaffected.
type AuthTier string

const (
	// AuthBasic requires no credential — the anonymous public surface.
	AuthBasic AuthTier = "basic"
	// AuthElevated requires a live elevated credential (e.g. an OAuth2 session
	// obtained by a manual web login). The credential can go stale.
	AuthElevated AuthTier = "elevated"
)

// Operation identifies a discrete retailer operation whose auth tier can be
// declared. The set covers the operations the Client performs.
type Operation string

const (
	OpResolve      Operation = "resolve"
	OpSearch       Operation = "search"
	OpCreateList   Operation = "create-shopping-list"
	OpSyncList     Operation = "sync-shopping-list"
	OpToCart       Operation = "to-cart"
	OpBarcode      Operation = "barcode"
	OpBonus        Operation = "bonus"
	OpOffers       Operation = "offers"
)

// TierFor returns the auth tier this client's retailer requires for the given
// operation.
//
// Willys and Hemköp are single-tier: every operation is AuthBasic. ICA is
// tiered: its anonymous ecom surface (resolve/search/barcode/offers) is
// AuthBasic and never stale, while its account-bound writes (create/sync
// shopping list, to-cart, bonus) are AuthElevated and can go stale. This
// matches the verified ICA finding that product search is anonymous ecom
// (never stale) while the wishlist push needs the OAuth2 session.
func (c *Client) TierFor(op Operation) AuthTier {
	if c.kind != RetailerICA {
		return AuthBasic
	}
	switch op {
	case OpCreateList, OpSyncList, OpToCart, OpBonus:
		return AuthElevated
	default:
		return AuthBasic
	}
}

// ErrElevatedStale is the sentinel error signaling that an elevated-tier
// operation failed because the retailer's session credential is stale. Callers
// surface this as "re-run the manual web login" and degrade the affected
// retailer to unavailable rather than blocking the others.
var ErrElevatedStale = errors.New("retailer: elevated credential is stale — re-run the manual web login")

// IsStaleCredential reports whether the given HTTP status code is the canonical
// signal that a retailer's elevated-tier session credential is stale. Per the
// ICA adapter's verified failure mode the canonical stale signal is HTTP 401
// (and 403); detection is keyed off the status code, NOT off "did the call
// throw", because the dangerous stale case is a silent false-success and the
// catchable case is an opaque parse error — only the status code is reliable.
func IsStaleCredential(statusCode int) bool {
	return statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden
}

// IsElevatedStale reports whether err (or any error it wraps) signals that an
// elevated-tier credential is stale. It matches ErrElevatedStale when a caller
// has already wrapped an error with it, and otherwise detects the canonical
// 401/403 status carried by an httpclient.StatusError.
func IsElevatedStale(err error) bool {
	if errors.Is(err, ErrElevatedStale) {
		return true
	}
	var se *httpclient.StatusError
	if errors.As(err, &se) {
		return IsStaleCredential(se.StatusCode)
	}
	return false
}
