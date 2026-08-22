# Multi-stage build for the food-brain service. Built first as a CLI (current
# state) and joined to docker-compose once it grows an HTTP server — see
# openspec/changes/establish-enforced-go-architecture.
#
# Build:  docker build -t spisordning/food-brain .
# Run:    food-brain serve  (CLI)  /  food-brain plan  (batch)
FROM golang:1.26-alpine AS build
RUN apk add --no-cache ca-certificates git
WORKDIR /src
# Pin the toolchain version (matches .go-version / go.mod go directive).
ENV GOFLAGS=-mod=mod
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" ./cmd/food-brain

FROM gcr.io/distroless/static-debian12
COPY --from=build /src/food-brain /usr/local/bin/food-brain
# Runtime needs the migrations to apply on boot (see 2.4).
COPY migrations /migrations
ENTRYPOINT ["food-brain"]
CMD ["serve"]