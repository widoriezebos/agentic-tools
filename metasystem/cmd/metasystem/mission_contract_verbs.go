package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/contract"
)

// The mission contract-measure verb is the per-cycle reading the mission runner
// records: it runs the contract's gate and guards against the current candidate,
// classifies the gate metrics against the prior cycle's reading, and prints the
// measurement as JSON.

func runMissionContractMeasure(args []string) int {
	flags := flag.NewFlagSet("mission contract-measure", flag.ContinueOnError)
	file := flags.String("file", "", "mission contract path")
	previous := flags.String("previous", "", "prior per-metric values as name=decimal[,name=decimal...]; empty measures against the sealed baseline")
	if flags.Parse(args) != nil {
		return 2
	}
	if *file == "" {
		fmt.Fprintln(os.Stderr, "--file is required")
		return 2
	}
	prior, err := parsePreviousMetrics(*previous)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	result, err := contract.Measure(*file, prior)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(string(encoded))
	return 0
}

// parsePreviousMetrics reads a comma-separated name=decimal list into a map. An
// empty argument yields a nil map, which measurement reads as "no prior cycle;
// measure against the sealed baseline".
func parsePreviousMetrics(list string) (map[string]string, error) {
	if strings.TrimSpace(list) == "" {
		return nil, nil
	}
	values := map[string]string{}
	for _, item := range strings.Split(list, ",") {
		name, value, found := strings.Cut(item, "=")
		if !found || name == "" || value == "" {
			return nil, fmt.Errorf("--previous entry is not name=value: %q", item)
		}
		values[name] = value
	}
	return values, nil
}

// runMissionContractEnvelopeAllows exits 0 when the mission's signed contract
// carries the exact runtime:model pair in envelope.dispatch-allow.
func runMissionContractEnvelopeAllows(args []string) int {
	flags := flag.NewFlagSet("mission contract-envelope-allows", flag.ContinueOnError)
	root := flags.String("root", "", "project root")
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
