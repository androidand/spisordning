package retailer

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/androidand/spisordning/internal/httpclient"
)

// AuthTier classifies how strong a session a retailer operation needs.
//
// Willys has no tiered auth today — willys-adapter's single OAuth-ish session
// (established once, refreshed transparently server-side) covers every
// operation, so every Willys call is effectively AuthBasic.
//
// ICA is the first retailer where the distinction is real: search/resolve
// runs over ica-client's anonymous ecom surface, which never goes stale
// (confirmed in expose-shopping-price-and-notes-bridge's task 1.2 research) —
// AuthBasic. Wishlist creation/sync needs the mobile OAuth2/PKCE session
// (ica-client/src/auth/oauth2.ts), which can go stale between the periodic
// manual web-login that refreshes ica-adapter's elevated credential —
// AuthElevated.
type AuthTier int

const (
	// AuthBasic operations work off a session that doesn't require manual
	// intervention to stay fresh. ResolveRequirements is AuthBasic for every
	// retailer.
	AuthBasic AuthTier = iota
	// AuthElevated operations need a session that can go stale and requires a
	// human to refresh it (for ICA: a Playwright-driven web login on the
	// ica-adapter side). CreateShoppingList and SyncShoppingList are
	// AuthElevated for ICA specifically.
	AuthElevated
)

// ErrElevatedAuthStale indicates an AuthElevated operation failed because the
// adapter's elevated session needs a fresh manual login — detected from the
// adapter's HTTP 401/403 response. Callers should use errors.Is to check for
// it rather than matching on the error string.
//
// This only catches the "catchable" ICA failure shape documented in
// expose-shopping-price-and-notes-bridge's task 1.2 research (an opaque 401
// that the adapter surfaces as a 4xx). That research also found a "dangerous"
// shape — a stale session that still returns 200/201 with fabricated data —
// which cannot be detected from this side of the HTTP boundary at all; fixing
// that requires an explicit res.ok/status guard inside ica-adapter itself
// (tracked in that same change, not here). This wrapper narrows the gap; it
// does not close it.
var ErrElevatedAuthStale = errors.New("retailer: elevated auth session is stale — manual re-login required")

// wrapElevatedAuthError checks whether err is an adapter 401/403 — the
// catchable signal that an AuthElevated operation's session has gone stale —
// and, if so, wraps it with ErrElevatedAuthStale so callers can detect it via
// errors.Is. Any other error (including nil) passes through unchanged.
func wrapElevatedAuthError(err error) error {
	if err == nil {
		return nil
	}
	var statusErr *httpclient.StatusError
	if errors.As(err, &statusErr) &&
		(statusErr.StatusCode == http.StatusUnauthorized || statusErr.StatusCode == http.StatusForbidden) {
		return fmt.Errorf("%w: %v", ErrElevatedAuthStale, err)
	}
	return err
}
