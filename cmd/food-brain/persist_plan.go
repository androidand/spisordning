// Package main is the composition root. This file opens the Postgres store from
// the environment when one is configured (task 2.3). The plan-persistence
// pipeline itself lives in the planning service (internal/service/planweek.go);
// the composition root only decides whether a database is available and hands
// the connection to the service and the catalog path.
//
// When no database is configured (POSTGRES_PASSWORD/DATABASE_URL unset) the
// pipeline stays in-memory exactly as before; this keeps `food-brain plan`
// runnable without Postgres and keeps plan_test.go green in CI's no-DB job.
package main

import (
	"context"
	"os"

	"github.com/androidand/spisordning/internal/persistence"
)

// openStore opens the Postgres store from the environment when one is configured.
// Returns (nil, nil) when no database is configured (in-memory path), and a
// non-nil error only when a database IS configured but unreachable.
func openStore(ctx context.Context) (*persistence.Store, error) {
	cfg, err := persistence.FromEnv(os.Getenv)
	if err != nil {
		return nil, nil // no database configured -> stay in-memory
	}
	store, err := persistence.New(ctx, cfg)
	if err != nil {
		return nil, err // configured but unreachable
	}
	return store, nil
}
