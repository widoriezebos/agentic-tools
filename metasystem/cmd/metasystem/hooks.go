package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

// runHooksCheck verifies this repository runs under the metasystem it
// ships. --runtime is REQUIRED and selects the registry's live
// self-check declaration (the vendored-entry marker); the live and
// shipped paths stay explicit arguments because the suite's
// nested-template case resolves live settings in the parent repo.
func runHooksCheck(args []string) int {
	flags := flag.NewFlagSet("hooks check", flag.ContinueOnError)
	runtime := flags.String("runtime", "", "runtime whose live self-check to verify (required)")
	if flags.Parse(args) != nil {
		return 2
	}
	rest := flags.Args()
	if *runtime == "" || len(rest) != 2 {
		fmt.Fprintln(os.Stderr, "usage: metasystem hooks check --runtime R <live settings> <shipped hooks>")
		return 2
	}
	declaration, ok := runtimes.Lookup(*runtime)
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown runtime: %s\n", *runtime)
		return 1
	}
	if declaration.SelfCheck == nil {
		fmt.Fprintf(os.Stderr, "no live self-check declared for %s\n", *runtime)
		return 1
	}
	if err := hooks.CheckOwnHooks(rest[0], rest[1], declaration.SelfCheck.VendoredMarker); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
