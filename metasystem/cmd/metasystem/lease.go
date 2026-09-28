package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// The lease family is the checkout write-authority surface (internal/lease):
// announce/retire a main, classify a caller, gate a writer on holdership,
// renew the lease, run a command held, and track protocol-error cursors.

// optionalEpoch returns a pointer to the --expected-epoch value only when it
// was actually passed, so "absent" and "0" stay distinct.
func optionalEpoch(flags *flag.FlagSet, value *int64) *int64 {
	var out *int64
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "expected-epoch" {
			v := *value
			out = &v
		}
	})
	return out
}

func runLeaseAnnounce(args []string) int {
	flags := flag.NewFlagSet("lease announce", flag.ContinueOnError)
	root := pathFlag(flags, "root", "", "checkout root")
	session := flags.String("session", "", "session id")
	pid := flags.Int64("pid", 0, "main pid")
	start := flags.Int64("start", 0, "main start epoch seconds")
	tag := flags.String("tag", "", "instance tag")
	runtime := flags.String("runtime", "", "runtime name")
	lineage := flags.String("owner-lineage", "", "logical owner lineage (optional)")
	startTicks := flags.Int64("start-ticks", 0, "start ticks (clock-step-immune pair; 0 = seconds only)")
	bootID := flags.String("boot-id", "", "boot id (clock-step-immune pair)")
	if flags.Parse(args) != nil {
		return 2
	}
	path, err := lease.AnnounceWithPair(*root, *session, *pid, *start, *startTicks, *bootID, *tag, *runtime, *lineage)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(path)
	return 0
}

func runLeaseRetire(args []string) int {
	flags := flag.NewFlagSet("lease retire", flag.ContinueOnError)
	root := pathFlag(flags, "root", "", "checkout root")
	session := flags.String("session", "", "session id")
	pid := flags.Int64("pid", 0, "main pid")
	start := flags.Int64("start", 0, "main start epoch seconds")
	if flags.Parse(args) != nil {
		return 2
	}
	if err := lease.Retire(*root, *session, *pid, *start); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func runLeaseRequireHolder(args []string) int {
	flags := flag.NewFlagSet("lease require-holder", flag.ContinueOnError)
	root := pathFlag(flags, "root", "", "checkout root")
	caller := flags.Int64("caller-pid", 0, "caller pid")
	epoch := flags.Int64("expected-epoch", 0, "expected claim epoch (optional)")
	if flags.Parse(args) != nil {
		return 2
	}
	out, err := lease.RequireHolder(*root, *caller, optionalEpoch(flags, epoch))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	printJSON(out)
	return 0
}
