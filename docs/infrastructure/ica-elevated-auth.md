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
a headless server (e.g. the `deploy-food-brain-to-proxmox` target) — a browser window cannot be
shown or interacted with on a headless Proxmox LXC.

## Decided approach: login on the Mac, upload to spisordning (not a raw file copy)

`ica-client` already supports importing cookies from any source (`EcomAuth.importCookies`/
`importCookiesFromFile` — not Playwright-only), so this is purely a topology/config question, not
a "needs new client capability" one. Chosen approach (2026-08-28), once `ica-adapter` is deployed
on Proxmox:

1. When the elevated session goes stale, run
   `apps/ica-adapter/upload-elevated-credential.ts --apply` on the Mac. It calls
   `client.ecomAuth.ensureLogin()` (opens a real, local browser window only if the cached session
   no longer validates live — same unchanged library behavior as always), then POSTs the resulting
   cookie JSON straight to spisordning's `POST /retailers/ica/elevated-credential`. Dry-run by
   default (matching `spisordning-bridge.ts`'s convention); `SPISORDNING_URL`/`--url` picks the
   target.
2. spisordning stores it (`retailer_credential` table — one row per retailer, uploading again
   overwrites the previous credential) and serves it back via `GET
   /retailers/ica/elevated-credential`.
3. `ica-adapter` (task 2.8, **not yet built**) would fetch that on startup/refresh and write it to
   its own local `cookieCachePath` (`ICA_ELEVATED_CREDENTIAL_PATH`, below) before calling
   `ensureLogin()` itself — so a Proxmox-deployed adapter actually picks up what got uploaded,
   without the Mac and Proxmox ever needing to share a filesystem or agree on a path. Until that
   lands, a human can still fetch the credential from spisordning's GET endpoint and place it
   manually — the upload half is a complete, independently useful piece on its own.

This keeps `ica-adapter` centralized on Proxmox (unlike `willys-adapter`, which currently stays
Mac-resident) at the cost of an occasional manual upload step whenever the session goes stale
(unpredictable — anywhere from hours to months). Alternatives considered and rejected: keeping
`ica-adapter` Mac-resident too (simpler, but inconsistent with wanting ICA centralized); a
remote-viewable headless browser via Xvfb+VNC (real new engineering + a login-capable
remote-desktop exposure to secure, more mechanism than this project has favored elsewhere); a raw
`scp`/file-copy sync (would need the Mac and Proxmox to agree on a shared path — the HTTP
upload/fetch through spisordning avoids that entirely and gives the credential a real, queryable
home instead of a file nobody tracks the age of).

## Where the credential lives

Three places now, each with a distinct role:

1. **The Mac**, transiently: `apps/ica-adapter/upload-elevated-credential.ts` caches to its own
   `.ica-elevated-cookies.json` (next to the script, not cwd-relative) purely so it has something
   to read back and upload — not meant to be treated as a durable store.
2. **spisordning's Postgres** (`retailer_credential` table), the actual source of truth once
   uploaded — durable, queryable (`uploaded_at` lets you tell how stale it is), retailer-scoped.
3. **Wherever `ica-adapter` itself runs**, via `cookieCachePath`/`ICA_ELEVATED_CREDENTIAL_PATH` —
   today (run locally) this is `ica-client`'s own default, `.ecom-cookies.json` relative to the
   process's working directory (confirmed live:
   `~/dev/store-clients/ica-client/.ecom-cookies.json` exists because `ica-adapter` is launched
   from that directory). Once task 2.8 lands, this becomes the local write-target for what
   `ica-adapter` fetches from spisordning, rather than only ever being populated by its own
   `ensureLogin()` calls.

**Now wired end-to-end (2026-08-28)**: `IcaClientOptions.cookieCachePath` (added to
`ica-client/src/index.ts`, plumbed through to `EcomAuth`) + `ica-adapter/server.ts` reading
`process.env.ICA_ELEVATED_CREDENTIAL_PATH` and passing it through. Unset (today's default)
preserves current behavior exactly. `internal/config.Config.ICAElevatedCredentialPath` on the Go
side documents the same env var name for consistency, though Go itself still never reads the
file's contents — `internal/retailer`'s 401/403 staleness detection (`ErrElevatedAuthStale`) works
from the adapter's live HTTP response, independent of this file. Verified: constructing `IcaClient`
with a custom `cookieCachePath` succeeds at runtime (`npx tsx` smoke check); `ica-client`'s own
test suite run alongside this change for regressions.
