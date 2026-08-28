package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func runPathOwner(args []string) int {
	flags := flag.NewFlagSet("path owner", flag.ContinueOnError)
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: metasystem path owner <path>")
		return 2
	}
	ownership, mode, err := stateroot.Owner(flags.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		if ownership == stateroot.OwnerOutside && mode != "" {
			fmt.Printf("%s %s\n", ownership, mode)
		}
		return 1
	}
	fmt.Printf("%s %s\n", ownership, mode)
	return 0
}
