# Makefile — common tasks for food-brain. Mirrors what the CI workflow runs.
#
# The OpenAPI-to-Go codegen tool (oapi-codegen v2) is pinned in tools/tools.go
# (build-tagged `tools`) so `go mod tidy` records it; run it via `go run` (uses
# the version in go.mod) or the `generate` target below. See task 3.2.

GO        ?= go
OAPIC     ?= $(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen
SQLC      ?= $(GO) run github.com/sqlc-dev/sqlc/cmd/sqlc
OAPI_SPEC := api/openapi.yaml
GEN_OUT   := internal/openapi/types.gen.go
SQLC_OUT  := internal/persistence/sqlc

.PHONY: help generate verify-codegen verify build vet test tidy

help: ## show targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-16s\033m %s\n", $$1, $$2}'

generate: generate-openapi generate-sqlc ## regenerate all generated code

generate-openapi: ## regenerate Go types from api/openapi.yaml
	$(OAPIC) -generate types -package openapi -o $(GEN_OUT) $(OAPI_SPEC)

generate-sqlc: ## regenerate sqlc query code from db/migrations + db/queries
	$(SQLC) generate

verify-codegen: generate ## fail if committed generated code is out of sync
	git diff --exit-code -- $(GEN_OUT) $(SQLC_OUT)

build: ## build all packages
	$(GO) build ./...

vet: ## static analysis
	$(GO) vet ./...

test: ## build + vet + all tests (incl. architecture-enforcement gate)
	$(GO) build ./...
	$(GO) vet ./...
	$(GO) test -count=1 ./...

tidy: ## normalize modules
	$(GO) mod tidy
