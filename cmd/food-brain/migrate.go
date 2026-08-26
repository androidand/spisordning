package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/androidand/spisordning/internal/persistence"
)

// runMigrate handles `food-brain migrate up [--seed]` and `food-brain migrate
// status`. It is the only supported schema-mutation path (see
// establish-migration-and-postgres-19); serve refuses to run while migrations
// are pending.
func runMigrate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: food-brain migrate <up [--seed] | status>")
	}
	sub, rest := args[0], args[1:]
	cfg, err := persistence.FromEnv(os.Getenv)
	if err != nil {
		return err
	}
	ctx := context.Background()
	switch sub {
	case "up":
		fs := flag.NewFlagSet("up", flag.ContinueOnError)
		seed := fs.Bool("seed", false, "apply db/seeds idempotently after migrations")
		if err := fs.Parse(rest); err != nil {
			return err
		}
		if err := persistence.MigrateUp(ctx, cfg); err != nil {
			return err
		}
		fmt.Println("✓ migrations applied")
		if *seed {
			if err := persistence.Seed(ctx, cfg); err != nil {
				return err
			}
			fmt.Println("✓ seeds applied")
		}
		return nil
	case "status":
		return persistence.MigrateStatus(ctx, cfg)
	default:
		return fmt.Errorf("unknown migrate subcommand %q (want: up, status)", sub)
	}
}
