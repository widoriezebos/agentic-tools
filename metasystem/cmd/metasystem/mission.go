package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"

	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

var missionIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
var treeIDRe = regexp.MustCompile(`^[0-9a-f]{40,64}$`)

// The mission-ledger family is the atomic owner of the stop-loss ledger
// (init, append, verify, count).

func runMissionLedgerInit(args []string) int {
	flags := flag.NewFlagSet("mission ledger-init", flag.ContinueOnError)
	file := flags.String("file", "", "ledger path")
	cycleBudget := flags.Int("cycle-budget", 0, "cycle budget")
	noGainBudget := flags.Int("no-gain-budget", 0, "no-gain budget")
	if flags.Parse(args) != nil {
		return 2
	}
	if err := mission.InitLedger(*file, *cycleBudget, *noGainBudget); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// The mission-state family owns the atomic, hash-chained mission state.

func runMissionStateInit(args []string) int {
	flags := flag.NewFlagSet("mission state-init", flag.ContinueOnError)
	state := flags.String("state", "", "state path")
	contract := flags.String("contract", "", "contract path")
	ledger := flags.String("ledger", "", "ledger path")
	lease := flags.String("lease", "", "runner lease reference")
	branch := flags.String("branch", "", "candidate branch override")
	baseline := flags.String("baseline", "", "admitted initial baseline tree id (required; every mission is born with its wall baseline)")
	if flags.Parse(args) != nil {
		return 2
	}
	root := stateVerbRoot(*state)
	if root == "" {
		fmt.Fprintln(os.Stderr, "mission state-init refused: the state path is not inside a mission layout, so the admission origins cannot be captured")
		return 1
	}
	match := regexp.MustCompile(`^mission-([a-z0-9][a-z0-9-]*)\.contract\.md$`).FindStringSubmatch(filepath.Base(*contract))
	if match == nil {
		fmt.Fprintln(os.Stderr, "mission state-init refused: contract filename is invalid")
		return 1
	}
	origins, err := mission.CaptureAdmissionOrigins(root, match[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	// The supplied baseline must BE the live filtered projection at this
	// instant — clean and human-sealed baselines both satisfy it, and a
	// freely invented tree cannot become E0.
	if err := mission.VerifyBaselineIsLive(root, match[1], *baseline); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := mission.InitStateWithBaseline(*state, *contract, *ledger, *lease, *branch, *baseline, origins); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// stateVerbRoot walks a mission state path back to the repository root
// (…/artifacts/agents/missions/<id>/state.json). Empty when the layout
// does not match — classification then runs against the working
// directory, where an unannounced caller classifies HUMAN.
func stateVerbRoot(statePath string) string {
	abs, err := filepath.Abs(statePath)
	if err != nil {
		return ""
	}
	// Symlinks resolve BEFORE the layout walk: an alias root symlinked
	// into the real mission directory must classify against the real
	// repository, not an empty decoy.
	if resolved, rerr := filepath.EvalSymlinks(abs); rerr == nil {
		abs = resolved
	}
	dir := filepath.Dir(abs)
	for i := 0; i < 3; i++ {
		dir = filepath.Dir(dir)
	}
	if filepath.Base(dir) != "artifacts" {
		return ""
	}
	return filepath.Dir(dir)
}

func runMissionStateVerify(args []string) int {
	flags := flag.NewFlagSet("mission state-verify", flag.ContinueOnError)
	state := flags.String("state", "", "state path")
	repo := pathFlag(flags, "repo", "", "repository (with --ledger, verifies the anchor)")
	ledger := flags.String("ledger", "", "ledger path (with --repo, verifies the anchor)")
	if flags.Parse(args) != nil {
		return 2
	}
	if (*repo == "") != (*ledger == "") {
		fmt.Fprintln(os.Stderr, "--repo and --ledger are required together for anchor verification")
		return 2
	}
	var (
		seq  int64
		hash string
		err  error
	)
	if *repo != "" {
		seq, hash, err = mission.VerifyStateWithAnchor(*state, *repo, *ledger)
	} else {
		seq, hash, err = mission.VerifyStateShape(*state)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("mission state valid: sequence=%d hash=%s\n", seq, hash)
	return 0
}

// The mission-fence family owns the lifecycle fences, cap authority, and usage.
