# Multi-stage build for the spisordning services. Produces two binaries:
#   - food-brain  (CLI + HTTP API)
#   - mcp-server  (MCP tool server: Streamable HTTP + stdio)
#
# Build:  docker build -t spisordning/food-brain .
# Run:    food-brain serve  (default)  /  mcp-server  (MCP server, :8081)
FROM golang:1.26-alpine AS build
RUN apk add --no-cache ca-certificates git
WORKDIR /src
# Pin the toolchain version (matches .go-version / go.mod go directive).
ENV GOFLAGS=-mod=mod
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" ./cmd/food-brain
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" ./cmd/mcp-server

FROM gcr.io/distroless/static-debian12
COPY --from=build /src/food-brain /usr/local/bin/food-brain
COPY --from=build /src/mcp-server /usr/local/bin/mcp-server
# Runtime needs the migrations to apply on boot (see 2.4).
COPY migrations /migrations
ENTRYPOINT ["food-brain"]
CMD ["serve"]
