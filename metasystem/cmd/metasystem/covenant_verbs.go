package main

// The covenant family: the thin, non-mutating structural check the
// inception interview runs after authoring covenant.json — a relay to
// the one landed reader, never an authoring surface and never an
// executor. Shape validity is all it can prove: whether the covenant's
// proofs actually guard the stated intent is adequacy, which no parser
// establishes.

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/covenant"
)

// runCovenantValidate loads and validates a covenant document's shape.
// Exit 0 shape-valid (with the honesty line), 1 refused, 2 usage.
func runCovenantValidate(args []string) int {
	flags := flag.NewFlagSet("covenant validate", flag.ContinueOnError)
	root := flags.String("root", ".", "repository root; the covenant's one home is <root>/covenant.json")
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem covenant validate [--root DIR]")
		return 2
	}
	c, err := covenant.Load(filepath.Join(*root, covenant.Filename))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("covenant shape valid: %s (%d requirement(s), battery %q on %s %s, %d budget(s), %d guard(s), net %v); adequacy not established — shape says the rows parse, never that the proofs guard the intent\n",
		c.Identity.Name, len(c.Requirements), c.Battery.Command,
		c.Battery.Metric, c.Battery.Threshold, len(c.Budgets), len(c.Guards),
		c.GuardrailSet.Entries())
	return 0
}
