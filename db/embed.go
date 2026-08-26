// Package db embeds the migration and seed SQL so the food-brain binary can
// apply them without shipping separate files on disk (see
// establish-migration-and-postgres-19).
package db

import "embed"

// FS holds the embedded db/migrations and db/seeds trees. Goose reads the
// migrations from the "migrations" root (dirpath "migrations"); the seed runner
// globs "seeds/*.sql".
//
//go:embed migrations/*.sql seeds/*.sql
var FS embed.FS
