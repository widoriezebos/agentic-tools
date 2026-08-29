package main

import (
	"flag"
	"fmt"
	"os"
	"time"

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
	root := flags.String("root", "", "checkout root")
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
	root := flags.String("root", "", "checkout root")
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

func runLeaseClassify(args []string) int {
	flags := flag.NewFlagSet("lease classify", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	metasystemRoot := flags.String("metasystem-root", "", "installed metasystem root (defaults to checkout root)")
	caller := flags.Int64("caller-pid", 0, "caller pid")
	if flags.Parse(args) != nil {
		return 2
	}
	if *metasystemRoot == "" {
		*metasystemRoot = *root
	}
	out, err := lease.ClassifyVerbAt(*root, *metasystemRoot, *caller)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	printJSON(out)
	return 0
}

func runLeaseRequireHolder(args []string) int {
	flags := flag.NewFlagSet("lease require-holder", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
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

func runLeaseRenew(args []string) int {
	flags := flag.NewFlagSet("lease renew", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	caller := flags.Int64("caller-pid", 0, "caller pid")
	if flags.Parse(args) != nil {
		return 2
	}
	out, err := lease.Renew(*root, *caller)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	printJSON(out)
	return 0
}

func runLeaseRunHeld(args []string) int {
	flags := flag.NewFlagSet("lease run-held", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	caller := flags.Int64("caller-pid", 0, "caller pid")
	epoch := flags.Int64("expected-epoch", 0, "expected claim epoch (optional)")
	if flags.Parse(args) != nil {
		return 2
	}
	argv := flags.Args()
	if len(argv) == 0 {
		fmt.Fprintln(os.Stderr, "run-held requires a command")
		return 2
	}
	code, err := lease.RunHeld(*root, *caller, optionalEpoch(flags, epoch), argv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return code
}

func runLeaseProtocolGrowth(args []string) int {
	flags := flag.NewFlagSet("lease protocol-growth", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	mainID := flags.String("main-id", "", "main id")
	if flags.Parse(args) != nil {
		return 2
	}
	out, err := lease.ProtocolGrowth(*root, *mainID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	printJSON(out)
	return 0
}

func runLeaseProtocolAdvance(args []string) int {
	flags := flag.NewFlagSet("lease protocol-advance", flag.ContinueOnError)
	root := flags.String("root", "", "checkout root")
	mainID := flags.String("main-id", "", "main id")
	caller := flags.Int64("caller-pid", 0, "caller pid")
	counts := flags.String("counts", "", "JSON object of protocol-error counts")
	if flags.Parse(args) != nil {
		return 2
	}
	if err := lease.ProtocolAdvance(*root, *mainID, *caller, *counts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runLeaseCommitToken atomically writes the live wrapper token that the
// pre-commit guard verifies: the wrapper's pid and kernel start second, a
// fresh nonce, and the creation time.
func runLeaseCommitToken(args []string) int {
	flags := flag.NewFlagSet("lease commit-token", flag.ContinueOnError)
	path := flags.String("path", "", "token file path")
	pid := flags.Int64("pid", 0, "wrapper pid")
	start := flags.Int64("start", 0, "wrapper start epoch seconds")
	nonce := flags.String("nonce", "", "wrapper nonce")
	if flags.Parse(args) != nil {
		return 2
	}
	if *path == "" || *pid < 1 || *start < 1 || *nonce == "" {
		fmt.Fprintln(os.Stderr, "lease commit-token: --path, --pid, --start, and --nonce are required")
		return 2
	}
	value := map[string]any{
		"wrapperPid": *pid, "wrapperPidStartedAt": *start, "nonce": *nonce,
		"createdAt": time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}
	if err := writeIdentityJSON(*path, value); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
