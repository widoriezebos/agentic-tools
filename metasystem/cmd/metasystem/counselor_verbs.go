package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/counselor"
)

var buildCounselorBrief = counselor.Build
var renderCounselorBrief = counselor.Render
var resolveCounselorRoot = upMetasystemRoot

func runCounselorBrief(args []string) int {
	flags := flag.NewFlagSet("counselor brief", flag.ContinueOnError)
	dryRun := flags.Bool("dry-run", false, "print the current advisory brief without writing state")
	metasystemRoot := flags.String("metasystem-root", "", "metasystem checkout root (defaults to installed binary root)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "counselor brief takes no positional arguments")
		return 2
	}
	if !*dryRun {
		fmt.Fprintln(os.Stderr, "counselor brief currently requires --dry-run; steward carriage is outside this slice")
		return 2
	}
	root, err := resolveCounselorRoot(*metasystemRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "counselor brief root: %v\n", err)
		return 1
	}
	brief := buildCounselorBrief(counselor.Options{Root: root})
	if err := renderCounselorBrief(os.Stdout, brief); err != nil {
		fmt.Fprintf(os.Stderr, "counselor brief output: %v\n", err)
		return 1
	}
	return 0
}
