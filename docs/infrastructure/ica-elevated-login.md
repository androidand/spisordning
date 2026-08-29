# ICA elevated-login handoff

This is the documented handoff point for the one manual step in spisordning's ICA
integration: the elevated-auth credential the ica-adapter needs for account-bound
operations (wishlist push, barcode lookup, bonus, offers). Until now this step existed
inside `ica-adapter`/`ica-client` with **no documented location at all**; this file is that
location. It is the operational counterpart to the `AuthTier`/`ICA_AUTH_FILE` wiring in
`establish-config-di-and-presentation-layer` (section 2).

## What the elevated credential is

ICA's account-bound surface runs on a **mobile OAuth2/PKCE Bearer session** — the
"second-auth" session. It is obtained by a **Playwright-driven, human-in-the-loop login**
(personal ID + PIN against `ims.icagruppen.se`) that lives entirely in the TS
`ica-adapter`/`ica-client` code. Go never touches the session.

The **anonymous ecom surface** (product search, `/resolve`) needs **no** credential and is
**never stale** — so price comparison keeps working even when the elevated session is dead.
Only the account-bound writes (wishlist push, etc.) depend on it. The verified failure mode
lives in `openspec/changes/expose-shopping-price-and-notes-bridge/tasks.md` (task 1.2).

## Who runs the login

A human. The login is Playwright-driven and human-in-the-loop by nature — it completes an
HTML-form login with personal ID + PIN. The ica-adapter orchestrates it; the person
supervises the browser window and enters their credentials. It is not automated and is not
expected to be.

## Where the credential file goes

The login produces a credential file (the OAuth2 session state). Drop it at the path named
by the `ICA_AUTH_FILE` env var:

| Env var | Read by | Meaning |
|---|---|---|
| `ICA_AUTH_FILE` | `config.Load()` → `Config.ICAAuthFile` | Path to the ICA elevated-credential file. Empty = no elevated credential configured (ICA account-bound ops unavailable; anonymous ecom still works). |

- Single-user homelab box: file permissions `0600` are sufficient. No encryption at rest
  (see the change's open question — revisit if the deployment ever becomes multi-tenant).
- Do **not** commit the file (or its contents) to this repo. It is a secret.
- The ica-adapter reads the file itself; spisordning (Go) only knows the path so it can
  *report* staleness — it never uses the credential.

## How staleness is detected (and what it means)

The canonical stale signal is **HTTP 401/403** on an elevated-tier call — keyed off the
status code, **not** "did the call throw". The dangerous stale case is a *silent
false-success*: a 401 with a JSON body makes the adapter fabricate a local list and return
200/201, so the push never reached ICA and no error surfaces. That is why the ica-adapter's
shopping-list write path must carry an explicit `res.ok`/status guard keyed off 401/403 so
staleness becomes a typed error instead of a silent success.

Once that guard is in place, Go models and detects the condition:

- `retailer.AuthElevated` (vs `AuthBasic`) declares which operations need the session.
- `retailer.IsElevatedStale(err)` detects it (a `httpclient.StatusError` carrying 401/403,
  or a wrapped `retailer.ErrElevatedStale`).
- When detected, ICA degrades to "unavailable" for the affected operation (e.g., the price
  comparison reports Willys/Hemköp only and flags ICA as unavailable) rather than failing
  the whole request.

The fix is always the same: **re-run the Playwright login** and drop the fresh credential
file at `ICA_AUTH_FILE`.

## Quick reference

| Symptom | Cause | Fix |
|---|---|---|
| ICA wishlist push fails / ICA flagged unavailable in comparison | Elevated session stale (401/403) | Re-run the Playwright login; drop the new credential file at `ICA_AUTH_FILE`. |
| ICA price comparison works but wishlist push doesn't | Elevated session stale; the anonymous ecom surface is unaffected | Same as above. |
| `ICA_AUTH_FILE` unset | No elevated credential configured | Run the login once; set `ICA_AUTH_FILE` to the resulting file path. |
