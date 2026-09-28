package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/landpath"
)

// runPreCommitEntry is the `pre-commit` entry the enrolled git hook runs: the
// guard body (landpath.Guard) for the installation at --root, judging the
// commit git is making in the current work tree.
func runPreCommitEntry(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("pre-commit", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := pathFlag(flags, "root", "", "metasystem installation root")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *root == "" {
		fmt.Fprintln(stderr, "usage: metasystem internal pre-commit --root INSTALLATION")
		return 2
	}
	workTree, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "pre-commit guard:", err)
		return 1
	}
	return landpath.Guard(landingGuardOwners(), *root, workTree, stdout, stderr)
}
