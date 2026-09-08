package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
)

type landingRepeatedStrings []string

func (values *landingRepeatedStrings) String() string { return fmt.Sprint([]string(*values)) }
func (values *landingRepeatedStrings) Set(value string) error {
	*values = append(*values, value)
	return nil
}

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
	testReceipt := flags.String("test-receipt", "", "candidate test receipt path")
	recertification := flags.String("recertification", "", "canonical recertification record path")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	observation := landing.Observe(landing.ObserveParams{
		RepoRoot: *root, CandidateTree: *tree, Chain: *chain,
		DirectFix: *directFix, RevertOf: *revertOf, Goal: *goal, Actor: *actor,
		RootJob: *rootJob, TestReceipt: *testReceipt, Recertification: *recertification,
	})
	printJSON(observation)
	return 0
}

func runLandingTestReceipt(args []string) int {
	flags := flag.NewFlagSet("landing test-receipt", flag.ContinueOnError)
	root := flags.String("root", "", "project checkout root")
	tree := flags.String("tree", "", "candidate project tree")
	command := flags.String("command", "", "test command to run from the isolated candidate workspace")
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

func runLandingPark(args []string) int {
	flags := flag.NewFlagSet("landing park", flag.ContinueOnError)
	root := flags.String("root", "", "integration project root")
	chain := flags.String("chain", "", "root implementation chain")
	target := flags.String("target", "", "frozen target commit")
	reason := flags.String("reason", "", "original landing refusal code")
	detail := flags.String("detail", "", "specific refusal explanation")
	recertification := flags.String("recertification", "", "diagnostic recertification record path")
	candidate := flags.String("candidate-commit", "", "retained local landing commit")
	var refs landingRepeatedStrings
	flags.Var(&refs, "recovery-ref", "retained source/result anchor ref (repeatable)")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	result, err := landing.Park(landing.ParkParams{
		Root: *root, Chain: *chain, TargetCommit: *target, Reason: *reason, Detail: *detail,
		Recertification: *recertification, CandidateCommit: *candidate, RecoveryRefs: refs,
		CallerPID: int64(os.Getppid()),
	})
	if err != nil {
		var failure *landing.ParkFailure
		if errors.As(err, &failure) {
			fmt.Fprintf(os.Stderr, "reason=chain-recertification-park-failed cause=%s error=%v\n", failure.Cause, failure.Err)
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		return 1
	}
	fmt.Printf("state=%s\nreason=%s\nparkRecord=%s\n", result.State, result.Reason, result.ParkRecord)
	return 0
}
