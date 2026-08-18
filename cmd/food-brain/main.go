// Command food-brain is the Food Brain CLI.
//
//	food-brain demo   — in-memory demonstration of the scoring pipe (no services)
//	food-brain plan   — live weekly plan: Mealie → scorer (+Skolmaten, +Olla) →
//	                    shopping requirements → willys-adapter (optional wishlist)
//	food-brain serve  — HTTP server (api/openapi.yaml); /health today, more routes
//	                    as the contract is implemented (tasks 3.3+).
//
// Running with no arguments is equivalent to `demo`.
package main

import (
	"fmt"
	"os"

	"github.com/androidand/spisordning/internal/httpapi"
)

func main() {
	cmd := "demo"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "demo":
		runDemo()
	case "plan":
		if err := runPlan(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "❌", err)
			os.Exit(1)
		}
	case "serve":
		addr := os.Getenv("SPISORNING_ADDR")
		if addr == "" {
			addr = ":8080"
		}
		if err := httpapi.Serve(addr); err != nil {
			fmt.Fprintln(os.Stderr, "❌", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q (want: demo, plan, serve)\n", cmd)
		os.Exit(2)
	}
}
