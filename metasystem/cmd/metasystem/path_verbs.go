package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathclass"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

var resolvePathClass = pathclass.ResolvePath

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

func runPathClass(args []string) int {
	flags := flag.NewFlagSet("path class", flag.ContinueOnError)
	explain := flags.Bool("explain", false, "print the matching row and resolution namespace")
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: metasystem path class [--explain] <path>")
		return 2
	}
	answer, err := resolvePathClass(flags.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *explain {
		fmt.Printf("%s row=%s key=%s:%s mode=%s\n", answer.Class, answer.Row, answer.Namespace, answer.Key, answer.Mode)
	} else {
		fmt.Println(answer.Class)
	}
	if answer.Class == pathclass.Unclassified {
		fmt.Fprintln(os.Stderr, pathclass.RefusalText(answer.Key))
		return 1
	}
	if answer.Class == pathclass.Outside {
		return 1
	}
	return 0
}
