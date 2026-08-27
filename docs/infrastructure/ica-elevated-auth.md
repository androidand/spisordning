# ICA's elevated (ecom) auth: what it is and how it gets refreshed

This documents a real operational handoff that had no documented location before
`establish-config-di-and-presentation-layer`'s task group 2 — not new complexity, just giving an
already-existing manual step somewhere written down.

## The two ICA auth tiers

Confirmed by reading `ica-client/src/auth/oauth2.ts` and `ica-client/src/auth/ecom-session.ts`
directly (also documented in `internal/retailer`'s `AuthTier` type):

- **Basic** (`oauth2.ts`): the mobile app's OAuth2/PKCE flow. Fully programmatic, ica-adapter
  handles it transparently. Backs product search/resolve — see
  `expose-shopping-price-and-notes-bridge`'s task 1.2 finding that this surface is never stale.
- **Elevated** (`ecom-session.ts`): the web-storefront (handlaprivatkund.ica.se) session. **Cannot
  be automated** — verified live in that file's own module docstring: the login step is gated
  behind Akamai Bot Manager + AWS WAF Bot Control device-fingerprinting that a plain HTTP client
  cannot pass. Backs wishlist creation/sync (`internal/retailer.CreateShoppingList`/
  `SyncShoppingList`).

## How the elevated session actually gets refreshed

There is no separate script to run. `EcomAuth.ensureLogin()` (called internally by `IcaClient`
whenever an elevated operation runs) does a live validation request against cached cookies first;
only if that fails does it **open a real, visible browser window** and wait (up to 10 minutes) for
a human to log in, then extract the resulting session cookies via Playwright's CDP-level cookie
API and cache them to disk.

**This means**: whoever is running `ica-adapter` needs to be at a machine with a display when the
elevated session goes stale, ready to complete a real browser login. Session lifetime is not well
characterized — `ecom-session.ts`'s own docstring notes one observed case of expiry after ~9 hours.
Expect this to happen sometimes, not rarely.

**Real deployment implication**: this is a genuine blocker for running `ica-adapter` unattended on
a headless server (e.g. the `deploy-food-brain-to-proxmox` target). A browser window cannot be
shown or interacted with on a headless Proxmox LXC. Until/unless this gets a remote-login-friendly
path (e.g. a forwarded browser session, or accepting cookies exported from a different machine),
`ica-adapter`'s elevated tier realistically needs to run somewhere a human can sit at, or wishlist
push for ICA specifically stays a manually-triggered, attended operation. Flagging this now rather
than discovering it mid-deploy.

## Where the credential lives today

`ica-client`'s `IcaClient` defaults `cookieCachePath` to `.ecom-cookies.json` relative to the
process's working directory — confirmed live: `~/dev/store-clients/ica-client/.ecom-cookies.json`
exists because `ica-adapter` is run from that directory (`server.ts` constructs `IcaClient`
without overriding `cookieCachePath`).

`internal/config.Config.ICAElevatedCredentialPath` (env `ICA_ELEVATED_CREDENTIAL_PATH`) exists on
the Go side as the documented, discoverable slot for this path — **but it is not yet wired to
anything on the `ica-adapter` side**: the TS `IcaClient` constructor in `server.ts` doesn't
currently accept an env var to override `cookieCachePath`. That's a small, real, cross-repo
follow-up (in `store-clients`, not spisordning) if a health check or explicit path override is
ever needed. Go doesn't need to read the file's contents either way — `internal/retailer`'s
401/403 staleness detection (`ErrElevatedAuthStale`) works from the adapter's live HTTP response,
independent of this file.
