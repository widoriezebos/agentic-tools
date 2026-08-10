package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/returnschema"
)

// runSchemaMaterialize ports return-schema.py: write a role's return schema at
// the requested version to an output file.
func runSchemaMaterialize(args []string) int {
	flags := flag.NewFlagSet("schema materialize", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	role := flags.String("role", "", "role name")
	version := flags.Int("version", 0, "schema version (1 or 2)")
	output := flags.String("output", "", "output path")
	if flags.Parse(args) != nil {
		return 2
	}
	if !returnschema.Roles[*role] {
		fmt.Fprintf(os.Stderr, "unknown role %q\n", *role)
		return 2
	}
	if *version != 1 && *version != 2 {
		fmt.Fprintln(os.Stderr, "version must be 1 or 2")
		return 2
	}
	if *root == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem schema materialize --root R --role ROLE --version V --output O")
		return 2
	}
	if err := returnschema.Materialize(*root, *role, *version, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
