package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"

	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
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

func runMissionLedgerAppend(args []string) int {
	flags := flag.NewFlagSet("mission ledger-append", flag.ContinueOnError)
	file := flags.String("file", "", "ledger path")
	cycle := flags.Int("cycle", 0, "cycle number (must be next)")
	classification := flags.String("classification", "", "cycle classification")
	sha := flags.String("candidate-sha", "", "resolved candidate git sha")
	observed := flags.String("observed", "", "observed measurement")
	best := flags.String("best", "", "new-best marker (yes|no; omit for a marker-less line)")
	if flags.Parse(args) != nil {
		return 2
	}
	if _, err := mission.AppendCycle(*file, *cycle, *classification, *sha, *observed, *best); err != nil {
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

// requireHumanStateCaller gates the state family's mutating verbs:
// the lease-holding runner is the only
// ordinary writer and it writes in-process, so every CLI caller that
// classifies as an announced agent — MAIN, DELEGATE, supervision,
// adapter — refuses. The posture stays the design's cooperative
// tamper-evident tier: classification is ancestry-based, and the human
// remains the out-of-band authority.
func requireHumanStateCaller(statePath, verb string) int {
	root := stateVerbRoot(statePath)
	if root == "" {
		if wd, err := os.Getwd(); err == nil {
			root = wd
		}
	}
	view, err := lease.Classify(root, int64(os.Getpid()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s refused: caller classification failed: %v\n", verb, err)
		return 3
	}
	if view.Class != lease.ClassHuman {
		fmt.Fprintf(os.Stderr, "%s refused: mission state is runner-owned; this caller classifies %s\n", verb, view.Class)
		return 3
	}
	return 0
}

func runMissionStateWrite(args []string) int {
	flags := flag.NewFlagSet("mission state-write", flag.ContinueOnError)
	state := flags.String("state", "", "state path")
	source := flags.String("source", "", "proposed next state path")
	expect := flags.String("expect", "", "expected current state hash")
	if flags.Parse(args) != nil {
		return 2
	}
	if code := requireHumanStateCaller(*state, "state-write"); code != 0 {
		return code
	}
	// The writer additionally refuses resolution-shaped transitions
	// inside WriteState itself: resolutions and segment moves land
	// only through resolve-taint's verified path.
	if err := mission.WriteState(*state, *source, *expect); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func runMissionStateVerify(args []string) int {
	flags := flag.NewFlagSet("mission state-verify", flag.ContinueOnError)
	state := flags.String("state", "", "state path")
	repo := flags.String("repo", "", "repository (with --ledger, verifies the anchor)")
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

func runMissionStateAnchor(args []string) int {
	flags := flag.NewFlagSet("mission state-anchor", flag.ContinueOnError)
	state := flags.String("state", "", "state path")
	repo := flags.String("repo", "", "repository")
	ledger := flags.String("ledger", "", "ledger path")
	if flags.Parse(args) != nil {
		return 2
	}
	if code := requireHumanStateCaller(*state, "state-anchor"); code != 0 {
		return code
	}
	if err := mission.Anchor(*state, *repo, *ledger); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func runMissionStateReconcile(args []string) int {
	flags := flag.NewFlagSet("mission state-reconcile", flag.ContinueOnError)
	state := flags.String("state", "", "state path")
	repo := flags.String("repo", "", "repository")
	ledger := flags.String("ledger", "", "ledger path")
	if flags.Parse(args) != nil {
		return 2
	}
	if gate := requireHumanStateCaller(*state, "state-reconcile"); gate != 0 {
		return gate
	}
	code, err := mission.Reconcile(*state, *repo, *ledger)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		if code == 0 {
			code = 1
		}
	}
	return code
}

// The mission-fence family owns the lifecycle fences, cap authority, and usage.

func runMissionFenceReserve(name string, reserve bool) func([]string) int {
	return func(args []string) int {
		flags := flag.NewFlagSet("mission "+name, flag.ContinueOnError)
		repo := flags.String("repo", "", "repository")
		missionID := flags.String("mission", "", "mission id")
		job := flags.String("job", "", "job id")
		capMin := flags.Int("cap-min", 0, "per-job cap in minutes")
		if flags.Parse(args) != nil {
			return 2
		}
		if !missionIDRe.MatchString(*missionID) {
			fmt.Fprintln(os.Stderr, "invalid mission id")
			return 2
		}
		if !missionIDRe.MatchString(*job) || *capMin < 1 {
			fmt.Fprintln(os.Stderr, "invalid mission job reservation")
			return 2
		}
		if err := mission.CheckOrReserve(*repo, *missionID, *job, *capMin, reserve); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
}

func runMissionFenceReserveCycle(args []string) int {
	flags := flag.NewFlagSet("mission fence-reserve-cycle", flag.ContinueOnError)
	repo := flags.String("repo", "", "repository")
	missionID := flags.String("mission", "", "mission id")
	if flags.Parse(args) != nil {
		return 2
	}
	if !missionIDRe.MatchString(*missionID) {
		fmt.Fprintln(os.Stderr, "invalid mission id")
		return 2
	}
	if err := mission.ReserveCycle(*repo, *missionID); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func runMissionFenceAuthorizeCap(args []string) int {
	flags := flag.NewFlagSet("mission fence-authorize-cap", flag.ContinueOnError)
	repo := flags.String("repo", "", "repository")
	missionID := flags.String("mission", "", "mission id")
	job := flags.String("job", "", "job id")
	runtime := flags.String("runtime", "", "runtime")
	model := flags.String("model", "", "canonical model key")
	requested := flags.Int("requested", 0, "requested cap in minutes (optional)")
	if flags.Parse(args) != nil {
		return 2
	}
	var requestedPtr *int
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "requested" {
			v := *requested
			requestedPtr = &v
		}
	})
	if !missionIDRe.MatchString(*missionID) {
		fmt.Fprintln(os.Stderr, "invalid mission id")
		return 2
	}
	if !missionIDRe.MatchString(*job) || !missionIDRe.MatchString(*runtime) ||
		*model == "" || *model != config.CanonicalModel(*model) ||
		(requestedPtr != nil && *requestedPtr < 1) {
		fmt.Fprintln(os.Stderr, "invalid mission cap authorization request")
		return 2
	}
	result, err := mission.AuthorizeCap(*repo, *missionID, *job, *runtime, *model, requestedPtr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	encoded, _ := json.Marshal(result)
	fmt.Println(string(encoded))
	return 0
}

func runMissionFenceAggregateUsage(args []string) int {
	flags := flag.NewFlagSet("mission fence-aggregate-usage", flag.ContinueOnError)
	repo := flags.String("repo", "", "repository")
	missionID := flags.String("mission", "", "mission id")
	if flags.Parse(args) != nil {
		return 2
	}
	if !missionIDRe.MatchString(*missionID) {
		fmt.Fprintln(os.Stderr, "invalid mission id")
		return 2
	}
	if err := mission.AggregateUsage(*repo, *missionID); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func runMissionFenceReleaseJob(args []string) int {
	flags := flag.NewFlagSet("mission fence-release-job", flag.ContinueOnError)
	repo := flags.String("repo", "", "repository")
	missionID := flags.String("mission", "", "mission id")
	job := flags.String("job", "", "job id whose reservation to release")
	if flags.Parse(args) != nil {
		return 2
	}
	if !missionIDRe.MatchString(*missionID) || *job == "" {
		fmt.Fprintln(os.Stderr, "fence-release-job requires --repo, --mission and --job")
		return 2
	}
	if err := mission.ReleaseJob(*repo, *missionID, *job); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func runMissionFenceRefuse(args []string) int {
	flags := flag.NewFlagSet("mission fence-refuse", flag.ContinueOnError)
	repo := flags.String("repo", "", "repository")
	missionID := flags.String("mission", "", "mission id")
	reason := flags.String("reason", "", "fence refusal reason")
	if flags.Parse(args) != nil {
		return 2
	}
	if !missionIDRe.MatchString(*missionID) {
		fmt.Fprintln(os.Stderr, "invalid mission id")
		return 2
	}
	ask, err := mission.Refuse(*repo, *missionID, *reason)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(ask)
	return 0
}
