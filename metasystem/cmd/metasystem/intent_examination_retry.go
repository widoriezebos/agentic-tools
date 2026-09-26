package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
)

// runJobExaminationRetry is the dispatch follow-up's eligibility question for
// a critic round that ended without a return: exit 0 admits one fresh
// examination round, exit 1 refuses with the reason on standard error.
func runJobExaminationRetry(args []string) int {
	flags := flag.NewFlagSet("job examination-retry", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	root := pathFlag(flags, "root", "", "repository root")
	record := pathFlag(flags, "record", "", "the chain's newest job record")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *root == "" || *record == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem job examination-retry --root ROOT --record FILE")
		return 2
	}
	latest, err := dispatchcore.ReadRecordObject(*record)
	if err == nil {
		err = dispatchcore.ExaminationRetryAdmissibleWith(*root, latest, dispatchcore.CustodyDeathDependencies{MatchesTag: positionedJobTag})
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
