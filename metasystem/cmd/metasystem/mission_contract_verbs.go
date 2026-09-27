package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/contract"
)

// The mission contract-measure verb is the per-cycle reading the mission runner
// records: it runs the contract's gate and guards against the current candidate,
// classifies the gate metrics against the prior cycle's reading, and prints the
// measurement as JSON.

// runMissionContractEnvelopeAllows exits 0 when the mission's signed contract
// carries the exact runtime:model pair in envelope.dispatch-allow.
func runMissionContractEnvelopeAllows(args []string) int {
	flags := flag.NewFlagSet("mission contract-envelope-allows", flag.ContinueOnError)
	root := pathFlag(flags, "root", "", "project root")
	missionID := flags.String("mission", "", "mission id")
	pair := flags.String("pair", "", "exact runtime:model pair")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || *missionID == "" || *pair == "" {
		fmt.Fprintln(os.Stderr, "mission contract-envelope-allows: --root, --mission, and --pair are required")
		return 2
	}
	if err := contract.DispatchEnvelopeAllows(*root, *missionID, *pair); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
