package main

import (
	"flag"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
)

func runLandingObserve(args []string) int {
	flags := flag.NewFlagSet("landing observe", flag.ContinueOnError)
	root := flags.String("root", "", "project checkout root")
	tree := flags.String("tree", "", "prospective project tree")
	chain := flags.String("chain", "", "closed implementation chain root")
	directFix := flags.String("direct-fix", "", "typed direct-fix class; register-carriage may accompany --chain")
	revertOf := flags.String("revert-of", "", "commit inverted by exact-revert")
	goal := flags.String("goal", "", "goal item carried by the landing")
	actor := flags.String("actor", "", "wrapper actor as machine+lineage")
	rootJob := flags.String("root-job", "", "tier-1 root implementer job")
	testReceipt := flags.String("test-receipt", "", "tier-1 test receipt path")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	observation := landing.Observe(landing.ObserveParams{
		RepoRoot: *root, CandidateTree: *tree, Chain: *chain,
		DirectFix: *directFix, RevertOf: *revertOf, Goal: *goal, Actor: *actor,
		RootJob: *rootJob, TestReceipt: *testReceipt,
	})
	printJSON(observation)
	return 0
}

func runLandingTestReceipt(args []string) int {
	flags := flag.NewFlagSet("landing test-receipt", flag.ContinueOnError)
	root := flags.String("root", "", "project checkout root")
	tree := flags.String("tree", "", "candidate project tree")
	command := flags.String("command", "", "test command to run from the project root")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	receipt, err := landing.CreateTestReceipt(*root, *tree, *command, os.Stdout, os.Stderr)
	if err != nil {
		return recordExit(err)
	}
	printJSON(receipt)
	return receipt.ExitStatus
}
