package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/contract"
)

// The mission-contract family owns the authored mission contract: parsing and
// type-checking it, sealing a reproducible baseline and priced exposure into
// it, and running the preflight a mission must pass before it may launch.

func runMissionContractValidate(args []string) int {
	flags := flag.NewFlagSet("mission contract-validate", flag.ContinueOnError)
	file := flags.String("file", "", "mission contract path")
	if flags.Parse(args) != nil {
		return 2
	}
	if *file == "" {
		fmt.Fprintln(os.Stderr, "--file is required")
		return 2
	}
	resolved, warnings, err := contract.Validate(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	// Calibration warnings never refuse a contract; they name what the
	// stop-loss design advises against so the human sizing the budget sees it.
	for _, warning := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}
	fmt.Printf("mission contract valid: %s\n", resolved)
	return 0
}

func runMissionContractSeal(args []string) int {
	flags := flag.NewFlagSet("mission contract-seal", flag.ContinueOnError)
	file := flags.String("file", "", "mission contract path")
	if flags.Parse(args) != nil {
		return 2
	}
	if *file == "" {
		fmt.Fprintln(os.Stderr, "--file is required")
		return 2
	}
	hash, err := contract.Seal(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(hash)
	return 0
}

func runMissionContractPreflight(args []string) int {
	flags := flag.NewFlagSet("mission contract-preflight", flag.ContinueOnError)
	file := flags.String("file", "", "mission contract path")
	verifiedBytes := flags.String("verified-bytes-output", "", "on success, write the approved contract bytes here")
	if flags.Parse(args) != nil {
		return 2
	}
	if *file == "" {
		fmt.Fprintln(os.Stderr, "--file is required")
		return 2
	}
	missionID, rawSHA, err := contract.Preflight(*file, *verifiedBytes)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("mission preflight passed: %s approvedContractSha256=%s\n", missionID, rawSHA)
	return 0
}

// runMissionContractHash prints the canonical signed-bytes digest of a
// contract file — the hash an approval records — without validating the
// authored grammar: envelope-only fixtures need the hash, not the full
// gate instruments.
func runMissionContractHash(args []string) int {
	flags := flag.NewFlagSet("mission contract-hash", flag.ContinueOnError)
	file := flags.String("file", "", "contract file")
	if flags.Parse(args) != nil {
		return 2
	}
	if *file == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem mission contract-hash --file FILE")
		return 2
	}
	data, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(contract.CanonicalContractHash(string(data)))
	return 0
}
