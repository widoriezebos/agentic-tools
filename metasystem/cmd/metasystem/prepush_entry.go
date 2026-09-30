package main

import (
	"fmt"
	"io"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// runPrePushEntry is the `pre-push` entry the landing lane checkout's git
// hook runs (lane.HookScript): git's remote name and URL follow the
// options, and the ref updates come on standard input. It admits the push
// only when it is exactly the one update a lane publication minted a token
// for (lane.AdmitPush).
func runPrePushEntry(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("pre-push", stdout, stderr)
	home := pathFlag(flags, "home", "", "the home the landing lane lives under")
	if flags.Parse(args) != nil || !requireFlags(flags, stderr, "home") || flags.NArg() != 2 || *home == "" {
		fmt.Fprintln(stderr, "usage: metasystem internal pre-push --home HOME REMOTE URL")
		return 2
	}
	return lane.RunPrePush(*home, flags.Arg(1), os.Stdin, stderr)
}
