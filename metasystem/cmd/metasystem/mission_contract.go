package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

// The mission-contract family owns the authored mission contract: parsing and
// type-checking it, sealing a reproducible baseline and priced exposure into
// it, and running the preflight a mission must pass before it may launch.

func runMissionContractValidate(args []string) int {
	flags := flag.NewFlagSet("mission-contract validate", flag.ContinueOnError)
	file := flags.String("file", "", "mission contract path")
	if flags.Parse(args) != nil {
		return 2
	}
	if *file == "" {
		fmt.Fprintln(os.Stderr, "--file is required")
		return 2
	}
	resolved, err := mission.ContractValidate(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("mission contract valid: %s\n", resolved)
	return 0
}

func runMissionContractSeal(args []string) int {
	flags := flag.NewFlagSet("mission-contract seal", flag.ContinueOnError)
	file := flags.String("file", "", "mission contract path")
	if flags.Parse(args) != nil {
		return 2
	}
	if *file == "" {
		fmt.Fprintln(os.Stderr, "--file is required")
		return 2
	}
	hash, err := mission.ContractSeal(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(hash)
	return 0
}

func runMissionContractPreflight(args []string) int {
	flags := flag.NewFlagSet("mission-contract preflight", flag.ContinueOnError)
	file := flags.String("file", "", "mission contract path")
	verifiedBytes := flags.String("verified-bytes-output", "", "on success, write the approved contract bytes here")
	if flags.Parse(args) != nil {
		return 2
	}
	if *file == "" {
		fmt.Fprintln(os.Stderr, "--file is required")
		return 2
	}
	missionID, rawSHA, err := mission.ContractPreflight(*file, *verifiedBytes)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("mission preflight passed: %s approvedContractSha256=%s\n", missionID, rawSHA)
	return 0
}
