package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/authority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

// runCensusRun computes a fixture-driven census verdict and writes it to
// --output, printing the inventory and diagnostic lines for the run.
func runCensusRun(args []string) int {
	flags := flag.NewFlagSet("proc census", flag.ContinueOnError)
	repo := flags.String("repo", "", "checkout root")
	root := flags.String("root", "", "metasystem root (defaults to --repo)")
	fp := flags.String("fingerprint", "", "fingerprint to stamp")
	interval := flags.Int("interval", 60, "interval seconds")
	output := flags.String("output", "", "verdict output path")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "proc census: --repo and --output are required")
		return 2
	}
	metasystemRoot := *root
	if metasystemRoot == "" {
		metasystemRoot = *repo
	}
	// With a recorded process table the verdict is fixture-driven and uses a
	// fixed clock for a deterministic result; otherwise it scans the live
	// process table with the real clock.
	var (
		verdict census.Verdict
		err     error
	)
	if processFile := os.Getenv("METASYSTEM_CENSUS_PROCESS_FILE"); processFile != "" {
		verdict, err = census.RunFixtureCensus(metasystemRoot, *repo, processFile, *fp, *interval, time.Unix(1786000000, 0))
	} else {
		verdict, err = census.RunProductionCensus(metasystemRoot, *repo, *fp, *interval, time.Now().UTC())
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "proc census:", err)
		return 1
	}
	encoded, err := json.MarshalIndent(verdict, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "proc census:", err)
		return 1
	}
	if err := os.WriteFile(*output, append(encoded, '\n'), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "proc census:", err)
		return 1
	}
	return 0
}

// runProcClassify relays `proc classify`: the shell liveness ladder's
// four-way verdict — live, stale, dead, unknown — from
// internal/identity. Callers on kill-capable paths DEFER on unknown;
// indeterminacy never acts.
func runProcClassify(args []string) int {
	flags := flag.NewFlagSet("proc classify", flag.ContinueOnError)
	pid := flags.Int64("pid", 0, "process id")
	tag := flags.String("tag", "", "recorded instance tag the argv must carry")
	if flags.Parse(args) != nil {
		return 2
	}
	if *pid < 1 {
		fmt.Fprintln(os.Stderr, "proc classify: --pid is required")
		return 2
	}
	fmt.Println(identity.TagState(identity.KernelProber{}, *pid, *tag))
	return 0
}

// runProcAcknowledge records that one exact untracked process (by the
// pid the human saw in the watchdog line) is known-harmless, so the
// end-of-turn report stops nagging about it — while a new untracked
// process, or the same pid reused by a different one, still shouts.
// The caller is classified and authorized holder-only (the human is
// sovereign; a holder main passes; machinery is refused), so an
// untracked agent cannot acknowledge itself through the verb — a
// same-user direct file write remains outside what local state can
// refuse; the posture is cooperative tamper evidence, not an
// adversarial boundary.
func runProcAcknowledge(args []string) int {
	flags := flag.NewFlagSet("proc acknowledge", flag.ContinueOnError)
	pid := flags.Int64("pid", 0, "pid of the untracked process to acknowledge")
	reason := flags.String("reason", "", "the human's reason this process is harmless")
	root := flags.String("root", "", "checkout root (required; the fixture authority binds to it)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *pid < 1 || *reason == "" || *root == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem proc acknowledge --pid P --reason R --root ROOT")
		return 2
	}
	// The caller is ALWAYS the real parent process — this verb takes no
	// --caller-pid: a caller-supplied pid
	// (e.g. -1) classifies HUMAN by fallthrough and launders holder-only
	// authorization. A human types this command; wrappers that need
	// identity forwarding have no business acknowledging.
	parent := int64(os.Getppid())
	view, err := lease.ClassifyVerb(*root, parent)
	if err != nil {
		fmt.Fprintln(os.Stderr, "acknowledge refused: caller classification failed:", err)
		return 1
	}
	if err := authority.Authorize("holder-only",
		map[string]any{"class": view.Class, "holder": view.Holder}, ""); err != nil {
		fmt.Fprintln(os.Stderr, "acknowledge refused:", err)
		return 1
	}
	// The classifier's ancestry walk checks the exact caller node for
	// announcements but starts machinery checks at its parent — so an
	// agent binary SPAWNING this verb directly would be judged by its
	// own ancestors, not by what it is. Close the seam here against
	// EVERY installed adapter's signature — not only configured
	// runtimes, since adoption retains all adapters while configuring
	// one. Fail closed: an unprovable invoker refuses too.
	runtime, isAgent, invErr := lease.DirectAgentInvoker(*root, parent)
	if invErr != nil {
		fmt.Fprintln(os.Stderr, "acknowledge refused:", invErr)
		return 1
	}
	if isAgent {
		fmt.Fprintf(os.Stderr, "acknowledge refused: the direct invoker is an agent process (%s); an agent cannot acknowledge on its own behalf\n", runtime)
		return 1
	}
	authorization, err := fixtureauth.New(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	entry, err := supervise.Acknowledge(*root, *pid, *reason, time.Now(), authorization.Identity())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("acknowledged pid %d (start %d): %s — the end-of-turn report will stay silent about this exact process\n",
		entry.Pid, entry.PidStartedAt, entry.Reason)
	return 0
}

func runCensusAlive(args []string) int {
	flags := flag.NewFlagSet("proc alive", flag.ContinueOnError)
	pid := flags.Int64("pid", 0, "process id")
	start := flags.Int64("start-time", 0, "expected start epoch seconds")
	startTicks := flags.Int64("start-ticks", 0, "expected start ticks (clock-step-immune pair; 0 = seconds only)")
	bootID := flags.String("boot-id", "", "expected boot id (clock-step-immune pair)")
	root := flags.String("root", "", "checkout root (required; the fixture authority binds to it)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *root == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem proc alive --pid P --start-time S [--start-ticks T --boot-id B] --root R")
		return 2
	}
	if (*startTicks > 0) != (*bootID != "") {
		fmt.Fprintln(os.Stderr, "proc alive: --start-ticks and --boot-id are both-or-neither")
		return 2
	}
	authorization, err := fixtureauth.New(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if census.AlivePair(*pid, *start, *startTicks, *bootID, authorization.Identity()) {
		return 0
	}
	return 1
}

func runCensusSignatureCheck(args []string) int {
	flags := flag.NewFlagSet("proc signature-check", flag.ContinueOnError)
	adapter := flags.String("adapter", "", "adapter path")
	positive := flags.String("positive", "", "argv that must classify")
	lookalike := flags.String("lookalike", "", "argv that must NOT classify")
	if flags.Parse(args) != nil {
		return 2
	}
	if err := census.SignatureCheck(*adapter, *positive, *lookalike); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runCensusFindAncestor walks the live process tree from --pid and prints the
// first signature-matched agent ancestor as compact JSON.
func runCensusFindAncestor(args []string) int {
	flags := flag.NewFlagSet("proc find-ancestor", flag.ContinueOnError)
	repo := flags.String("repo", "", "metasystem root")
	pid := flags.Int64("pid", 0, "process id to walk up from")
	runtime := flags.String("runtime", "", "restrict to one runtime (optional)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" || *pid == 0 {
		fmt.Fprintln(os.Stderr, "proc find-ancestor: --repo and --pid are required")
		return 2
	}
	ancestor, err := census.FindAncestorProduction(*repo, *pid, *runtime)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	encoded, _ := json.Marshal(ancestor)
	fmt.Println(string(encoded))
	return 0
}
