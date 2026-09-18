package main

import (
	"flag"
	"fmt"
	"os"
	"syscall"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

var fixtureSurvivorSignal identity.SignalFunc = syscall.Kill

func runFixtureSurvivors(args []string) int {
	flags := flag.NewFlagSet("proc fixture-survivors", flag.ContinueOnError)
	root := pathFlag(flags, "root", "", "checkout root (required; fixture authority binds to it)")
	ownerText := flags.String("owner", "", "exact fixture owner reference")
	keyText := flags.String("key", "", "fixture key")
	reap := flags.Bool("reap", false, "kill certainly owned survivors")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" || flags.NArg() != 0 || *ownerText != "" && *keyText != "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem proc fixture-survivors [--owner REF | --key KEY] [--reap] --root R")
		return 2
	}

	selection := census.FixtureSurvivorSelection{}
	if *ownerText != "" {
		owner, err := identity.ParseRef(*ownerText)
		if err != nil {
			fmt.Fprintf(os.Stderr, "proc fixture-survivors: invalid owner %q: %v\n", *ownerText, err)
			return 2
		}
		selection.Owner = &owner
	}
	if *keyText != "" {
		key, err := identity.ParseKey(*keyText)
		if err != nil {
			fmt.Fprintf(os.Stderr, "proc fixture-survivors: invalid key %q: %v\n", *keyText, err)
			return 2
		}
		selection.Key = &key
	}

	prober, processes, configured, err := census.FixtureSurvivorSource(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "proc fixture-survivors: process table is unreadable: %v\n", err)
		return 2
	}
	if configured && *reap {
		fmt.Fprintln(os.Stderr, "proc fixture-survivors: --reap is refused with a configured process file")
		return 2
	}
	var survivors []identity.FixtureSurvivor
	if *reap {
		survivors, err = census.ReapFixtureSurvivors(prober, processes, selection, fixtureSurvivorSignal)
	} else {
		survivors, err = census.ScanFixtureSurvivors(prober, processes, selection)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "proc fixture-survivors: %v\n", err)
		return 2
	}
	certain := false
	for _, survivor := range survivors {
		fmt.Println(census.FixtureSurvivorLine(prober, survivor))
		certain = certain || survivor.Class == identity.FixtureSurvivorCertain
	}
	if certain {
		return 1
	}
	return 0
}
