package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// runHooksCheck structurally verifies a selected host's live lifecycle
// settings. The historical two-positional-argument form remains Claude.
func runHooksCheck(args []string) int {
	flags := flag.NewFlagSet("hooks check", flag.ContinueOnError)
	runtime := flags.String("runtime", "", "runtime whose live lifecycle settings to verify (default: claude)")
	if flags.Parse(args) != nil {
		return 2
	}
	rest := flags.Args()
	if len(rest) != 2 {
		fmt.Fprintln(os.Stderr, "usage: metasystem hooks check [--runtime R] <live settings> <shipped hooks>")
		return 2
	}
	if *runtime == "" {
		*runtime = "claude"
	}
	declaration, ok := runtimes.Lookup(*runtime)
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown runtime: %s\n", *runtime)
		return 1
	}
	if !declaration.Adoptable || declaration.ShippedEnforcementConfig == "" {
		fmt.Fprintf(os.Stderr, "no host hook configuration declared for %s\n", *runtime)
		return 1
	}
	layout, err := stateroot.ResolveLayout(rest[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	live, err := os.ReadFile(rest[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	shipped, err := os.ReadFile(rest[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := hooks.CheckSettings(live, shipped, *runtime, layout.InstallationRel, layout.RepositoryRoot == layout.InstallationRoot); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
