package main

import (
	"flag"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
)

func runLandingObserve(args []string) int {
	flags := flag.NewFlagSet("landing observe", flag.ContinueOnError)
	root := flags.String("root", "", "project checkout root")
	tree := flags.String("tree", "", "prospective project tree")
	chain := flags.String("chain", "", "closed implementation chain root")
	directFix := flags.String("direct-fix", "", "typed direct-fix class; register-carriage may accompany --chain")
	revertOf := flags.String("revert-of", "", "commit inverted by exact-revert")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	observation := landing.Observe(landing.ObserveParams{
		RepoRoot: *root, CandidateTree: *tree, Chain: *chain,
		DirectFix: *directFix, RevertOf: *revertOf,
	})
	printJSON(observation)
	return 0
}
