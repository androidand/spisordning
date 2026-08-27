## ADDED Requirements

### Requirement: food-brain and mcp-server images are published and pullable
CI SHALL push real, pullable `food-brain` and `mcp-server` images to GHCR on changes to `main`,
rather than only build-verifying them.

#### Scenario: A merge to main publishes a pullable image
- **WHEN** a commit merges to `main`
- **THEN** `ghcr.io/androidand/spisordning/food-brain:latest` is pullable from a clean environment
- **AND** its tag reflects the merged commit (sha tag alongside `:latest`)

### Requirement: The deployed stack is reachable from the household LAN
Once deployed to Tengil, spisordning's HTTP API and MCP server SHALL be reachable by name/address
from other devices on the household LAN (not only from the Proxmox host itself), so the Mac-local
Apple Notes bridge and an MCP-capable chat client can reach them.

#### Scenario: food-brain's HTTP API is reachable from the Mac
- **WHEN** a request is made from the user's Mac to the deployed `food-brain` HTTP API's health
  endpoint
- **THEN** it responds successfully

#### Scenario: mcp-server is reachable from an MCP client on the LAN
- **WHEN** an MCP-capable chat client on the LAN connects to the deployed `mcp-server` over
  Streamable HTTP
- **THEN** it can list and call tools (e.g. `list_recipe_candidates`) successfully

#### Scenario: Migrations apply cleanly on first deploy
- **WHEN** the one-shot `migrate` compose service runs against a fresh deployed Postgres
- **THEN** it completes successfully before `food-brain`/`mcp-server` start, per the existing
  `depends_on: service_completed_successfully` ordering in `docker-compose.yml`
