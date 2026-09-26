package main

// The mutation surface of the SYNCED backlog: every command builds
// one VerbRequest and calls the engine verb whose authority rules
// decide — the CLI adds no policy of its own. On a checkout still
// carrying the legacy ledger these commands do not apply; the
// legacy wrappers own that world untouched.

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/counselor"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalrevision"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/metrics"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

type goalRecoveryPolicy struct {
	dispatchcore.GoalRecoveryPolicy
	root string
}

func (p goalRecoveryPolicy) ParkBranchCheck(endpoint goal.Endpoint) func(string, string) (string, error) {
	return goalParkBranchCheck(p.root, endpoint)
}

// The park branch check and its reader types moved into internal/goal/branch,
// where the interface's own park reaches them too (R-125-m1u); these are the
// command edge's one-line names for them.
func goalParkBranchCheck(root string, endpoint goal.Endpoint) func(string, string) (string, error) {
	return goalbranch.ParkCheck(root, endpoint)
}

type parkLocalTipReader = goalbranch.ParkLocalTipReader
type parkEndpointTipReader = goalbranch.ParkEndpointTipReader
type parkOriginTipReader = goalbranch.ParkOriginTipReader

func goalParkBranchCheckWithReaders(root string, endpoint goal.Endpoint, localTip parkLocalTipReader, endpointTipReader parkEndpointTipReader, originTipReader parkOriginTipReader) func(string, string) (string, error) {
	return goalbranch.ParkCheckWithReaders(root, endpoint, localTip, endpointTipReader, originTipReader)
}

func bindHandoverTargetRoot(request *goal.VerbRequest, targetRoot string) {
	request.HandoverTargetRoot = targetRoot
}

func configureCarriedCounselor(endpoint *goal.Endpoint) {
	endpoint.ConfigureCarriedCounselorAppend(func(root, _ string, row goal.HistoryLine, _ time.Time) error {
		line, err := counselor.CarriedLandingLine(row)
		if err != nil {
			return err
		}
		return counselor.AppendCarriedLanding(root, line)
	})
}

func goalHandoverTargetLiveness(root, targetMachine, targetLineage string, targetEpoch int64) (identity.Liveness, error) {
	return goalHandoverTargetLivenessWithReads(root, targetMachine, targetLineage, targetEpoch, goal.ResolveMachine, identity.KernelProber{})
}

func goalHandoverTargetLivenessWithReads(root, targetMachine, targetLineage string, targetEpoch int64, resolveMachine func(string) (string, error), prober identity.Prober) (identity.Liveness, error) {
	machine, err := resolveMachine(root)
	if err != nil || machine != targetMachine {
		return identity.Unknown, fmt.Errorf("target checkout is machine %q, not target %q", machine, targetMachine)
	}
	holder, err := lease.CurrentHolder(root)
	if err != nil {
		return identity.Unknown, err
	}
	if holder.OwnerLineage != targetLineage || holder.ClaimEpoch != targetEpoch {
		return identity.Unknown, fmt.Errorf("landing holder is %s at epoch %d, not %s at epoch %d", holder.OwnerLineage, holder.ClaimEpoch, targetLineage, targetEpoch)
	}
	for _, announcement := range lease.AnnouncementsFor(root, holder.Pid) {
		lineage := announcement.OwnerLineage
		if lineage == "" {
			lineage = announcement.MainId
		}
		if announcement.MainId == holder.MainId && lineage == targetLineage {
			ref := identity.Ref{Pid: announcement.Pid, StartedAtSec: announcement.PidStartedAt,
				StartTicks: announcement.PidStartTicks, BootID: announcement.BootID}
			return identity.AliveRef(prober, ref), nil
		}
	}
	return identity.Unknown, nil
}

func goalHandoverAuthenticationRoot(seatRoot, targetRoot string) (string, error) {
	if targetRoot != "" {
		return targetRoot, nil
	}
	now, err := goalCommandNow(seatRoot)
	if err != nil {
		return "", err
	}
	landing, err := config.ResolveBatchLanding(filepath.Join(seatRoot, "metasystem.conf"), seatRoot, func() time.Time { return now })
	return landing.Root, err
}

func runGoalHandoverMutation(args []string) int {
	flags := flag.NewFlagSet("goal handover", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "seat checkout root")
	id := flags.String("id", "", "goal id")
	lineage := flags.String("lineage", "", "current holder lineage")
	targetMachine := flags.String("target-machine", "", "target machine")
	targetLineage := flags.String("target-lineage", "", "target lineage")
	targetEpoch := flags.Int64("target-claim-epoch", 0, "target claim epoch")
	batch := flags.String("batch", "", "landing batch id")
	targetRoot := flags.String("target-root", "", "checkout root that authenticates a returning target")
	if flags.Parse(args) != nil {
		return 2
	}
	if *id == "" || *targetMachine == "" || *targetLineage == "" || *targetEpoch < 1 || *batch == "" {
		fmt.Fprintln(os.Stderr, "goal handover needs --id, --target-machine, --target-lineage, --target-claim-epoch, and --batch")
		return 2
	}
	if !converted(*root) {
		fmt.Fprintln(os.Stderr, "goal handover works the synced backlog; this checkout still carries the legacy ledger")
		return 1
	}
	req, err := syncReq("handover", *root, "", *lineage)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	bindHandoverTargetRoot(&req, *targetRoot)
	liveness := func() (identity.Liveness, error) {
		if req.Actor.Machine == *targetMachine && req.Actor.Lineage == *targetLineage {
			if req.ClaimEpoch != *targetEpoch || req.ClaimEpoch < 1 {
				return identity.Unknown, fmt.Errorf("the current target pair is not authenticated at epoch %d", *targetEpoch)
			}
			return identity.Alive, nil
		}
		authRoot, err := goalHandoverAuthenticationRoot(*root, *targetRoot)
		if err != nil {
			return identity.Unknown, err
		}
		return goalHandoverTargetLiveness(authRoot, *targetMachine, *targetLineage, *targetEpoch)
	}
	res, err := goal.Handover(req, *id, *targetMachine, *targetLineage, *targetEpoch, *batch, liveness)
	return printSyncResult(res, err)
}

func printCarryMutation(res goal.PublishResult, detail string, err error) int {
	if err != nil {
		var ask *goal.CarryAskError
		if errors.As(err, &ask) {
			fmt.Fprintln(os.Stderr, ask.Error())
			return 3
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if detail != "" {
		fmt.Printf("%s ledger=%s\n", detail, res.Tip)
	} else {
		printJSON(map[string]any{"outcome": res.Outcome, "tip": res.Tip, "detail": res.Detail})
	}
	if res.Outcome != goal.OutcomeConfirmed {
		return 1
	}
	return 0
}

func runGoalCarry(args []string) int {
	for _, arg := range args {
		if arg == "--to" || strings.HasPrefix(arg, "--to=") {
			return runGoalCarryAbandoned(args)
		}
	}
	return runGoalCarryLanding(args)
}

func runGoalCarryLanding(args []string) int {
	flags := flag.NewFlagSet("goal carry", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	id := flags.String("id", "", "goal id")
	by := flags.String("by", "", "the directing human")
	lineage := flags.String("lineage", "", "coordinator lineage")
	tree := flags.String("tree", "", "full whole-project tree id")
	past := flags.String("past", "", "one named refusal code or testing group")
	why := flags.String("why", "", "human reason for carrying the landing")
	expires := flags.Duration("expires", 2*time.Hour, "word lifetime, at most four hours")
	supersede := flags.String("supersede", "", "unconsumed carry word replaced by this word")
	transfer := flags.Bool("transfer", false, "allow superseding a word from another seat")
	raiseFormat := flags.Bool("raise-format", false, "raise ledger format 1 to format 2 in this transaction")
	fixtureAuthority := flags.Bool("fixture-human-authority", false, "fixture-only enrolled-human proof")
	temporary := flags.String("temporary-human-word", "", "not accepted by carry")
	reviewBy := flags.String("review-by", "", "not accepted by carry")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *id == "" || *by == "" || *tree == "" || *past == "" || strings.TrimSpace(*why) == "" {
		fmt.Fprintln(os.Stderr, "usage: metasystem goal carry --root ROOT --id GOAL --by NAME --tree SHA40 --past NAME --why TEXT [--expires 2h] [--supersede OPID] [--transfer] [--raise-format]")
		return 2
	}
	if *temporary != "" || *reviewBy != "" {
		fmt.Fprintln(os.Stderr, "carry takes no relayed word")
		return 2
	}
	if strings.HasPrefix(*by, "human:") {
		fmt.Fprintln(os.Stderr, "goal carry --by takes the human name without the human: actor prefix")
		return 2
	}
	if *expires <= 0 || *expires > 4*time.Hour {
		fmt.Fprintln(os.Stderr, "the carry expiry must be positive and no more than the four-hour ceiling")
		return 3
	}
	if *transfer && *supersede == "" {
		fmt.Fprintln(os.Stderr, "--transfer requires --supersede")
		return 2
	}
	if len(*tree) != 40 || !allLowerHex(*tree) || gitObjectType(*root, *tree) != "tree" {
		fmt.Fprintln(os.Stderr, "carry asks for the full 40-digit tree id of a Git tree")
		return 3
	}
	projected, err := landing.ProjectWorkspaceTree(*root, *tree)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	f := &syncFlags{root: *root, id: *id, by: *by, lineage: *lineage, fixtureHumanAuthority: *fixtureAuthority}
	classification, err := classifyGoalAuthorityFirst("carry", f)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	proof, err := proveGoalHumanAuthority("carry", f, proveEnrolledGoalHumanAuthority)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	req, err := syncReqClassified(*root, *by, *lineage, &proof, classification)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	opid := goal.Opid(req.Ulid, req.Actor.Machine, req.Actor.Lineage)
	result, err := goal.Carry(req, goal.CarryArgs{Goal: *id, Workspace: projected, Past: *past, Why: *why, Supersede: *supersede, Expires: req.Now.Add(*expires), Transfer: *transfer, RaiseFormat: *raiseFormat}, &proof)
	if err != nil || result.Outcome != goal.OutcomeConfirmed {
		return printCarryMutation(result, "", err)
	}
	if err := humanauthority.RecordCarryProof(*root, opid, proof); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	projection, err := goal.Project(req.Endpoint, false, req.Now)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	file := projection.Tree.Live[*id]
	if file == nil {
		fmt.Fprintf(os.Stderr, "goal %s vanished after its carry word was confirmed\n", *id)
		return 1
	}
	if file.Risk == nil {
		fmt.Println("risk: unanswered")
	} else {
		fmt.Printf("tier: %d risk: severity=%d novelty=%d exposure=%d accumulation=%d\n", file.Tier, file.Risk.Severity, file.Risk.Novelty, file.Risk.Exposure, file.Risk.Accumulation)
	}
	reviewRounds := int64(0)
	if file.Budget != nil {
		reviewRounds = file.Budget.ReviewRoundLimit
	}
	fmt.Printf("review rounds: %d skipped by human carry\n", reviewRounds)
	codeTip := "refs/remotes/origin/main"
	if req.Endpoint.LocalMode() {
		codeTip = "refs/heads/main"
	} else if result.Tip != "" {
		// The confirmed word itself is anchored on the accepted tip even
		// though Publish deliberately leaves the checkout's tracking ref alone.
		codeTip = result.Tip
	}
	open, err := goal.OpenCarryWords(*root, projection.Tree, codeTip, req.Actor.Machine, req.Now)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	counts, err := goal.CountCarries(*root, projection.Tree, codeTip, req.Now)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("open carries: %d on seat %s\n", len(open), req.Actor.Machine)
	fmt.Printf("carry debt: obligations=%d inflight=%d\n", counts.Debt, counts.Inflight)
	fmt.Printf("expires: %s\n", req.Now.Add(*expires).UTC().Format(time.RFC3339))
	fmt.Printf("ledger format: %s\n", projection.Tree.Root.FormatVersion)
	fmt.Printf("carry=%s workspace=%s past=%s ledger=%s\n", opid, projected, *past, result.Tip)
	return 0
}

func runGoalCarrying(args []string) int {
	flags := flag.NewFlagSet("goal carrying", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	id := flags.String("id", "", "goal id")
	ref := flags.String("ref", "", "carry word operation id")
	carrying := flags.String("carrying", "", "fleet reservation row operation id")
	commit := flags.String("commit", "", "carried commit id for the local intent form")
	tree := flags.String("tree", "", "whole-project tree")
	workspace := flags.String("workspace", "", "workspace projection for the local intent form")
	past := flags.String("past", "", "named refusal")
	battery := flags.String("battery", "", "green or red testing battery")
	missing := flags.String("missing", "", "comma-separated missing groups")
	failing := flags.String("failing", "", "comma-separated failing groups")
	judge := flags.String("judge", "", "live or base")
	judgeTree := flags.String("judge-tree", "", "base judge tree")
	judgeDigest := flags.String("judge-digest", "", "judge SHA-256 digest")
	liveFailure := flags.String("live-failure", "", "live judge failure")
	ledger := flags.String("ledger", "", "accepted ledger tip")
	by := flags.String("by", "", "human actor; reservations default to the carry word's actor")
	ownerPID := flags.Int64("owner-pid", 0, "live ancestor process that owns the local intent")
	abandon := flags.String("abandon", "", "reservation row to close")
	why := flags.String("why", "landing is not continuing", "reason for abandoning the reservation")
	lineage := flags.String("lineage", "", "coordinator lineage")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *id == "" {
		return 2
	}
	req, err := syncReq("carrying", *root, "", *lineage)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *abandon != "" {
		result, err := goal.AbandonCarrying(req, *id, *abandon, *why)
		return printCarryMutation(result, "", err)
	}
	if *ref == "" || *tree == "" || len(*tree) != 40 || gitObjectType(*root, *tree) != "tree" {
		fmt.Fprintln(os.Stderr, "goal carrying needs --id, --ref, and a full --tree")
		return 2
	}
	projected, err := landing.ProjectWorkspaceTree(*root, *tree)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *commit != "" {
		if *workspace == "" {
			*workspace = projected
		}
		if *workspace != projected {
			fmt.Fprintf(os.Stderr, "goal carrying --commit workspace differs: supplied=%s projected=%s\n", *workspace, projected)
			return 1
		}
		if *carrying == "" || len(*commit) != 40 || gitObjectType(*root, *commit) != "commit" || *past == "" || (*battery != "green" && *battery != "red") || *judge == "" || *judgeDigest == "" || *ledger == "" || *by == "" || *ownerPID < 1 {
			fmt.Fprintln(os.Stderr, "goal carrying --commit needs the reservation, carried fields, and a live --owner-pid")
			return 2
		}
	}
	result, row, err := goal.Carrying(req, goal.CarryingArgs{Goal: *id, ApprovedRef: *ref, Carrying: *carrying, Commit: *commit, Project: *tree, Workspace: projected, Past: *past, Battery: *battery, Missing: *missing, Failing: *failing, Judge: *judge, JudgeTree: *judgeTree, JudgeDigest: *judgeDigest, LiveFailure: *liveFailure, Ledger: *ledger, By: *by, OwnerPID: *ownerPID})
	if err != nil && *ownerPID > 0 && strings.Contains(err.Error(), "owner must be a live ancestor") {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	detail := ""
	if row != "" {
		detail = "carrying=" + row
	}
	return printCarryMutation(result, detail, err)
}

func runGoalCarried(args []string) int {
	flags := flag.NewFlagSet("goal carried", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	entry := flags.String("entry", "", "created carried journal entry")
	rebuild := flags.String("rebuild-from-commit", "", "landed commit whose carried trailers rebuild the record")
	repair := flags.Bool("repair-counselor", false, "repair the counselor line from the carried row")
	ref := flags.String("ref", "", "carry word operation id")
	id := flags.String("id", "", "goal id for a rebuilt record")
	lineage := flags.String("lineage", "", "coordinator lineage")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	req, err := syncReq("carried", *root, "", *lineage)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	configureCarriedCounselor(&req.Endpoint)
	selected := 0
	if *entry != "" {
		selected++
	}
	if *rebuild != "" {
		selected++
	}
	if *repair {
		selected++
	}
	if selected != 1 {
		fmt.Fprintln(os.Stderr, "goal carried needs exactly one of --entry, --rebuild-from-commit, or --repair-counselor")
		return 2
	}
	if *repair {
		if *ref == "" {
			return 2
		}
		if err := goal.RepairCarriedCounselor(req.Endpoint, *ref, req.Now); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	if *entry != "" {
		result, err := goal.Carried(req, *entry)
		return printCarryMutation(result, "", err)
	}
	if *id == "" || *ref == "" {
		fmt.Fprintln(os.Stderr, "goal carried --rebuild-from-commit needs --id and --ref")
		return 2
	}
	carriedArgs, err := carriedArgsFromCommit(*root, *id, *ref, *rebuild)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	result, err := goal.CarriedFromCommit(req, carriedArgs)
	return printCarryMutation(result, "", err)
}

func allLowerHex(value string) bool {
	for _, character := range value {
		if character < '0' || character > '9' {
			if character < 'a' || character > 'f' {
				return false
			}
		}
	}
	return true
}

func gitObjectType(root, object string) string {
	output, err := exec.Command("git", "-C", root, "cat-file", "-t", object).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func carriedArgsFromCommit(root, goalID, ref, commit string) (goal.CarriedArgs, error) {
	if len(commit) != 40 || !allLowerHex(commit) || gitObjectType(root, commit) != "commit" {
		return goal.CarriedArgs{}, fmt.Errorf("rebuild requires a full carried commit id")
	}
	message, err := exec.Command("git", "-C", root, "log", "-1", "--format=%B", commit).Output()
	if err != nil {
		return goal.CarriedArgs{}, err
	}
	trailer := func(key string) (string, error) {
		var matches []string
		for _, line := range strings.Split(strings.ReplaceAll(string(message), "\r\n", "\n"), "\n") {
			if strings.HasPrefix(line, key+": ") {
				matches = append(matches, strings.TrimPrefix(line, key+": "))
			}
		}
		if len(matches) != 1 || matches[0] == "" {
			return "", fmt.Errorf("carried commit %s requires exactly one %s trailer", commit, key)
		}
		return matches[0], nil
	}
	values := map[string]string{}
	for _, key := range []string{"Carry", "Carried-By", "Carried-Tree", "Carried-Past", "Carried-Battery", "Carried-Judge", "Carried-Ledger", "Landing-Provenance"} {
		values[key], err = trailer(key)
		if err != nil {
			return goal.CarriedArgs{}, err
		}
	}
	if values["Carry"] != ref {
		return goal.CarriedArgs{}, fmt.Errorf("carry trailer is %s, not %s", values["Carry"], ref)
	}
	fields := func(value string) (string, map[string]string) {
		parts := strings.Fields(value)
		first := ""
		keyed := map[string]string{}
		if len(parts) > 0 {
			first = parts[0]
		}
		for _, part := range parts {
			if k, v, ok := strings.Cut(part, "="); ok {
				keyed[k] = v
			}
		}
		return first, keyed
	}
	_, treeValues := fields(values["Carried-Tree"])
	battery, batteryValues := fields(values["Carried-Battery"])
	judge, judgeValues := fields(values["Carried-Judge"])
	if treeValues["workspace"] == "" || treeValues["project"] == "" || (battery != "green" && battery != "red") || (judge != "live" && judge != "base") || judgeValues["sha256"] == "" {
		return goal.CarriedArgs{}, fmt.Errorf("carried commit %s has malformed Carried-Tree, Carried-Battery, or Carried-Judge trailer", commit)
	}
	return goal.CarriedArgs{Goal: goalID, ApprovedRef: ref, Commit: commit, Workspace: treeValues["workspace"], Project: treeValues["project"], Past: values["Carried-Past"], Battery: battery, Missing: batteryValues["missing"], Failing: batteryValues["failing"], Judge: judge, JudgeTree: judgeValues["tree"], JudgeDigest: judgeValues["sha256"], LiveFailure: judgeValues["live-failure"], Ledger: values["Carried-Ledger"], By: values["Carried-By"], Outcome: "landed"}, nil
}

// syncReq assembles the one request every synced verb consumes. A
// mutation without an identity refuses: the silent "session"
// default minted claims under a generic lineage, and steal and
// succession then judged the wrong owner.
func syncReq(verb, root, by, lineageFlag string) (goal.VerbRequest, error) {
	return syncReqWithProof(verb, root, by, lineageFlag, nil)
}

func syncStoppingReq(verb, root, by, lineageFlag string) (goal.VerbRequest, error) {
	return syncStoppingReqWithProof(verb, root, by, lineageFlag, nil)
}

func syncStoppingReqWithProof(verb, root, by, lineageFlag string, observedProof *humanauthority.Proof) (goal.VerbRequest, error) {
	return syncStoppingReqWithProofWithDependencies(verb, root, by, lineageFlag, observedProof, goalCommandNow, defaultSyncRequestDependencies())
}

func syncStoppingReqWithProofWithDependencies(verb, root, by, lineageFlag string, observedProof *humanauthority.Proof, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies) (goal.VerbRequest, error) {
	classification, classifyErr := brainHumanWordClassificationWithFacts(verb, root, by, observedProof, dependencies.authorityFacts)
	if classifyErr != nil {
		return goal.VerbRequest{}, classifyErr
	}
	return syncReqClassifiedWithTerminalGradeAtWithDependencies(root, by, lineageFlag, observedProof, classification, true, commandNow, dependencies)
}

var (
	proveSyncReqHumanAuthority    = humanauthority.Prove
	proveSyncReqTerminalAuthority = humanauthority.ProveTerminal
	dischargeReviewObligation     = goal.DischargeReviewObligation
)

type syncRequestDependencies struct {
	authorityFacts goalAuthorityReadFacts
	endpoint       func(string) (goal.Endpoint, error)
	machine        func(string) (string, error)
	ensureGuard    func(string) error
	ownerLineage   func() string
	proveHuman     func(string, int64, humanauthority.Reader, time.Time) (humanauthority.Proof, error)
	proveTerminal  func(string, int64, humanauthority.Reader, time.Time) (humanauthority.Proof, error)
	presence       func(string, goal.Endpoint) (seat.Copy, error)
	// report, when set, receives the owner's typed outcome instead of the
	// printed one; the public intent commands render it themselves.
	report *ownerReport
}

func defaultSyncRequestDependencies() syncRequestDependencies {
	return syncRequestDependencies{
		authorityFacts: defaultGoalAuthorityReadFacts(),
		endpoint:       goal.ResolveEndpoint,
		machine:        goal.ResolveMachine,
		ensureGuard:    ensureGuardEnrolled,
		ownerLineage:   func() string { return os.Getenv("METASYSTEM_OWNER_LINEAGE") },
		proveHuman:     proveSyncReqHumanAuthority,
		proveTerminal:  proveSyncReqTerminalAuthority,
		presence:       readTickPresence,
	}
}

func terminalEnrollmentLineage(enrollment humanauthority.Enrollment) string {
	return terminalAuthorityLineage(enrollment.TerminalID, enrollment.Generation)
}

func terminalAuthorityLineage(observedTerminalID string, generation uint64) string {
	terminalID := []byte(observedTerminalID)
	for index, character := range terminalID {
		if !('A' <= character && character <= 'Z') && !('a' <= character && character <= 'z') &&
			!('0' <= character && character <= '9') && character != '-' {
			terminalID[index] = '-'
		}
	}
	return fmt.Sprintf("terminal-%s-%d", terminalID, generation)
}

func syncReqWithProof(verb, root, by, lineageFlag string, observedProof *humanauthority.Proof) (goal.VerbRequest, error) {
	return syncReqWithProofAt(verb, root, by, lineageFlag, observedProof, goalCommandNow)
}

func syncReqWithProofAt(verb, root, by, lineageFlag string, observedProof *humanauthority.Proof, commandNow func(string) (time.Time, error)) (goal.VerbRequest, error) {
	return syncReqWithProofAtWithDependencies(verb, root, by, lineageFlag, observedProof, commandNow, defaultSyncRequestDependencies())
}

func syncReqWithProofAtWithDependencies(verb, root, by, lineageFlag string, observedProof *humanauthority.Proof, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies) (goal.VerbRequest, error) {
	classification, classifyErr := brainHumanWordClassificationWithFacts(verb, root, by, observedProof, dependencies.authorityFacts)
	if classifyErr != nil {
		return goal.VerbRequest{}, classifyErr
	}
	return syncReqClassifiedWithTerminalGradeAtWithDependencies(root, by, lineageFlag, observedProof, classification, false, commandNow, dependencies)
}

func syncReqClassified(root, by, lineageFlag string, observedProof *humanauthority.Proof, classification lease.ClassifyResult) (goal.VerbRequest, error) {
	return syncReqClassifiedAt(root, by, lineageFlag, observedProof, classification, goalCommandNow)
}

func syncReqClassifiedAt(root, by, lineageFlag string, observedProof *humanauthority.Proof, classification lease.ClassifyResult, commandNow func(string) (time.Time, error)) (goal.VerbRequest, error) {
	return syncReqClassifiedWithTerminalGradeAt(root, by, lineageFlag, observedProof, classification, false, commandNow)
}

func syncReqClassifiedWithTerminalGradeAt(root, by, lineageFlag string, observedProof *humanauthority.Proof, classification lease.ClassifyResult, allowTerminal bool, commandNow func(string) (time.Time, error)) (goal.VerbRequest, error) {
	return syncReqClassifiedWithTerminalGradeAtWithDependencies(root, by, lineageFlag, observedProof, classification, allowTerminal, commandNow, defaultSyncRequestDependencies())
}

func syncReqClassifiedWithTerminalGradeAtWithDependencies(root, by, lineageFlag string, observedProof *humanauthority.Proof, classification lease.ClassifyResult, allowTerminal bool, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies) (goal.VerbRequest, error) {
	if err := dependencies.ensureGuard(root); err != nil {
		return goal.VerbRequest{}, err
	}
	e, err := dependencies.endpoint(root)
	if err != nil {
		return goal.VerbRequest{}, err
	}
	configureCarriedCounselor(&e)
	machine, err := dependencies.machine(root)
	if err != nil {
		return goal.VerbRequest{}, err
	}
	authority := observedProof
	if allowTerminal && by != "" && authority == nil {
		now, nowErr := commandNow(root)
		if nowErr != nil {
			return goal.VerbRequest{}, nowErr
		}
		fullProof, fullErr := dependencies.proveHuman(root, int64(os.Getppid()), nil, now)
		if fullErr == nil && fullProof.ValidFor(root) {
			authority = &fullProof
		} else {
			if fullProof.Outcome != humanauthority.OutcomeTerminalMissing && fullProof.Outcome != humanauthority.OutcomeNotEnrolled {
				if fullErr == nil {
					fullErr = fmt.Errorf("proof outcome %s was not a valid enrolled-grade proof", fullProof.Outcome)
				}
				return goal.VerbRequest{}, fmt.Errorf("a human stopping act could not prove enrolled human ancestry: %w", fullErr)
			}
			terminalProof, terminalErr := dependencies.proveTerminal(root, int64(os.Getppid()), nil, now)
			if terminalErr != nil {
				return goal.VerbRequest{}, fmt.Errorf("a human stopping act could not prove terminal human ancestry: %w", terminalErr)
			}
			if !terminalProof.TerminalValidFor(root) || terminalProof.AuthorityGrade() != humanauthority.GradeTerminal {
				return goal.VerbRequest{}, fmt.Errorf("a human stopping act did not produce a valid terminal-grade proof")
			}
			authority = &terminalProof
		}
	}
	lineage := lineageFlag
	if lineage == "" {
		lineage = dependencies.ownerLineage()
	}
	if lineage == "" {
		if by == "" {
			return goal.VerbRequest{}, fmt.Errorf("mutations carry their coordinator's identity: export METASYSTEM_OWNER_LINEAGE or pass --lineage")
		}
		if allowTerminal && authority != nil && authority.TerminalValidFor(root) {
			terminalID := authority.ObservedTerminalID()
			if terminalID == "" {
				return goal.VerbRequest{}, fmt.Errorf("a human stopping act did not retain its observed terminal identity")
			}
			lineage = terminalAuthorityLineage(terminalID, authority.TerminalGeneration)
		}
	}
	if lineage == "" {
		enrollment, err := humanauthority.ReadEnrollment(root)
		if err != nil {
			return goal.VerbRequest{}, fmt.Errorf("a human act derives its lineage from the enrolled terminal, and this checkout has none: run metasystem goal enroll-terminal here once, or pass --lineage: %w", err)
		}
		now, nowErr := commandNow(root)
		if nowErr != nil {
			return goal.VerbRequest{}, nowErr
		}
		proof := humanauthority.Proof{}
		var proofErr error
		if authority != nil {
			proof = *authority
		} else {
			proof, proofErr = dependencies.proveHuman(root, int64(os.Getppid()), nil, now)
		}
		if proofErr != nil || proof.Outcome != humanauthority.OutcomeProven || !proof.ValidFor(root) {
			outcome := proof.Outcome
			if outcome == "" {
				outcome = humanauthority.OutcomeUnreadable
			} else if outcome == humanauthority.OutcomeProven {
				outcome = humanauthority.OutcomeChanged
			}
			return goal.VerbRequest{}, fmt.Errorf("a human act derives its lineage only at the enrolled terminal: this shell does not descend from it (%s); run the act at the terminal, or pass --lineage", outcome)
		}
		if !proof.FixtureOnly && (proof.TerminalGeneration != enrollment.Generation || proof.TerminalRef != enrollment.TerminalRef) {
			return goal.VerbRequest{}, fmt.Errorf("a human act derives its lineage only at the enrolled terminal: this shell does not descend from it (%s); run the act at the terminal, or pass --lineage", humanauthority.OutcomeChanged)
		}
		lineage = terminalEnrollmentLineage(enrollment)
		authority = &proof
	}
	ulid, err := goalUlid()
	if err != nil {
		return goal.VerbRequest{}, err
	}
	now, err := commandNow(root)
	if err != nil {
		return goal.VerbRequest{}, err
	}
	req := goal.VerbRequest{
		Endpoint: e, Actor: goal.Actor{Machine: machine, Lineage: lineage, Human: by},
		Authority: authority, Ulid: ulid, Now: now, CallerClass: classification.Class,
	}
	if classification.Holder && classification.ClaimEpoch != nil {
		req.EpochAuthority = goal.EpochAuthorityHolder
	}
	if classification.ClaimEpoch != nil && (classification.Holder || by != "" || classification.Class == lease.ClassHuman) {
		req.ClaimEpoch = *classification.ClaimEpoch
	} else if classification.Class == lease.ClassHuman {
		req.ClaimEpoch = 1
	}
	return req, nil
}

type goalAuthorityReadFacts struct {
	repositoryTop  func(string) (string, error)
	ledgerIdentity func(string) string
}

func defaultGoalAuthorityReadFacts() goalAuthorityReadFacts {
	return goalAuthorityReadFacts{
		repositoryTop:  stateroot.RepositoryTop,
		ledgerIdentity: goal.ExistingLedgerIdentity,
	}
}

func brainHumanWordClassification(verb, root, by string, observedProof *humanauthority.Proof) (lease.ClassifyResult, error) {
	return brainHumanWordClassificationWithFacts(verb, root, by, observedProof, defaultGoalAuthorityReadFacts())
}

func brainHumanWordClassificationWithFacts(verb, root, by string, observedProof *humanauthority.Proof, facts goalAuthorityReadFacts) (lease.ClassifyResult, error) {
	classification, classifyErr := classifyVerbCallerWith(root, int64(os.Getppid()), facts.repositoryTop)
	brainState := brain.Read(root, facts.ledgerIdentity(root))
	if brainState.State != brain.Undeclared {
		command := fmt.Sprintf("metasystem goal %s --root <checkout> --id <id> --by <name> <the verb's own flags>", verb)
		if classifyErr != nil {
			return lease.ClassifyResult{}, fmt.Errorf("this checkout is declared the brain and the caller could not be classified (%v); an unclassified caller carries no human's word here. Wido runs, from an agent-free terminal: %s", classifyErr, command)
		}
		if brainState.State == brain.Corrupt {
			return lease.ClassifyResult{}, fmt.Errorf("%s", brain.RemedialRefusal(brainState.Reason, root))
		}
		fixtureProof := observedProof != nil && observedProof.FixtureOnly
		if by != "" && classification.Class != lease.ClassHuman && !fixtureProof {
			return lease.ClassifyResult{}, fmt.Errorf("this checkout is declared the brain; the brain never carries a human's word into goal %s, not as --by, not as a relayed word, not as a channel reference. Wido runs, from an agent-free terminal: %s", verb, command)
		}
	}
	return classification, nil
}

func classifyGoalAuthorityFirst(verb string, f *syncFlags) (lease.ClassifyResult, error) {
	return classifyGoalAuthorityFirstWithFacts(verb, f, defaultGoalAuthorityReadFacts())
}

func classifyGoalAuthorityFirstWithFacts(verb string, f *syncFlags, facts goalAuthorityReadFacts) (lease.ClassifyResult, error) {
	var fixtureProof *humanauthority.Proof
	var fixtureErr error
	if f.fixtureHumanAuthority {
		proof, err := proveFixtureGoalAuthority(verb, f)
		if err != nil {
			fixtureErr = err
		} else {
			fixtureProof = &proof
		}
	}
	classification, err := brainHumanWordClassificationWithFacts(verb, f.root, f.by, fixtureProof, facts)
	if err != nil {
		return lease.ClassifyResult{}, err
	}
	if fixtureErr != nil {
		return lease.ClassifyResult{}, fixtureErr
	}
	return classification, nil
}

func printSyncResult(res goal.PublishResult, err error) int {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	printJSON(map[string]any{"outcome": res.Outcome, "tip": res.Tip, "detail": res.Detail})
	if res.Outcome != goal.OutcomeConfirmed {
		return 1
	}
	return 0
}

func runGoalReadItems(args []string) int {
	return runGoalReadItemsWithInputs(args, goalCommandNow, defaultSyncRequestDependencies(), nil)
}

func runGoalReadItemsWithInputs(args []string, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies, resolveCodeCommit func(root, ref string) (string, error)) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: metasystem goal read-items add|close|list [flags]")
		return 2
	}
	switch args[0] {
	case "add":
		return runGoalReadItemsAddWithInputs(args[1:], commandNow, dependencies)
	case "close":
		return runGoalReadItemsCloseWithInputs(args[1:], commandNow, dependencies, resolveCodeCommit)
	case "list":
		return runGoalReadItemsListWithInputs(args[1:], commandNow, dependencies.endpoint)
	default:
		fmt.Fprintf(os.Stderr, "unknown goal read-items verb %q\n", args[0])
		return 2
	}
}

// readItemMutationRequestWithProof builds a read-item request carrying the
// person's proof. A --by without an observed proof is proven here at the
// enrolled terminal; a name alone never becomes the request's authority.
func readItemMutationRequestWithProof(verb, root, by, lineage string, observed *humanauthority.Proof, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies) (goal.VerbRequest, bool) {
	if !converted(root) {
		dependencies.complain("goal read-items works the synced backlog; this checkout still carries the legacy ledger")
		return goal.VerbRequest{}, false
	}
	if by != "" && observed == nil {
		now, err := commandNow(root)
		if err != nil {
			dependencies.complain(err)
			return goal.VerbRequest{}, false
		}
		proof, err := dependencies.proveHuman(root, int64(os.Getppid()), nil, now)
		if err == nil && !proof.ValidFor(root) {
			err = fmt.Errorf("proof outcome %s is not a valid enrolled-human proof", proof.Outcome)
		}
		if err != nil {
			dependencies.complain(fmt.Errorf("goal %s --by %s could not prove enrolled human ancestry: %w", verb, by, err))
			return goal.VerbRequest{}, false
		}
		observed = &proof
	}
	req, err := syncReqWithProofAtWithDependencies(verb, root, by, lineage, observed, commandNow, dependencies)
	if err != nil {
		dependencies.complain(err)
		return goal.VerbRequest{}, false
	}
	return req, true
}

func runGoalReadItemsAddWithInputs(args []string, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies) int {
	return runGoalReadItemsAddWithProof(args, nil, commandNow, dependencies)
}

// runGoalReadItemsAddWithProof is read-items add with the person's observed
// proof, when a caller has already proven it.
func runGoalReadItemsAddWithProof(args []string, observed *humanauthority.Proof, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies) int {
	flags := flag.NewFlagSet("goal read-items add", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	id := flags.String("id", "", "goal id")
	read := flags.String("read", "", "read label")
	by := flags.String("by", "", "the directing human")
	lineage := flags.String("lineage", "", "coordinator lineage")
	itemsFile := flags.String("items-file", "", "one item per line")
	var items repeatedStrings
	flags.Var(&items, "item", "read item (repeatable)")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *id == "" || *read == "" || (len(items) == 0) == (*itemsFile == "") {
		dependencies.complain("usage: metasystem goal read-items add --id GOAL --read LABEL (--item TEXT ... | --items-file PATH)")
		return 2
	}
	texts := []string(items)
	if *itemsFile != "" {
		var err error
		texts, err = readItemsFile(*itemsFile)
		if err != nil {
			dependencies.complain(err)
			return 1
		}
		if len(texts) == 0 {
			dependencies.complain("--items-file contains no read items")
			return 2
		}
	}
	req, ok := readItemMutationRequestWithProof("read-items add", *root, *by, *lineage, observed, commandNow, dependencies)
	if !ok {
		return 1
	}
	result, err := goal.AddReadItems(req, *id, *read, texts)
	return dependencies.publish(result, err)
}

func readItemsFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var items []string
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			items = append(items, line)
		}
	}
	return items, nil
}

func runGoalReadItemsCloseWithInputs(args []string, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies, resolveCodeCommit func(root, ref string) (string, error)) int {
	return runGoalReadItemsCloseWithProof(args, nil, commandNow, dependencies, resolveCodeCommit)
}

// runGoalReadItemsCloseWithProof is read-items close with the person's
// observed proof, when a caller has already proven it.
func runGoalReadItemsCloseWithProof(args []string, observed *humanauthority.Proof, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies, resolveCodeCommit func(root, ref string) (string, error)) int {
	flags := flag.NewFlagSet("goal read-items close", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	id := flags.String("id", "", "goal id")
	item := flags.String("item", "", "read item id")
	by := flags.String("by", "", "the directing human")
	lineage := flags.String("lineage", "", "coordinator lineage")
	fixed := flags.String("fixed", "", "commit that fixed the item")
	moved := flags.String("moved", "", "open goal receiving the item")
	accepted := flags.String("accepted", "", "reason this is not a defect")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *id == "" || *item == "" {
		dependencies.complain("usage: metasystem goal read-items close --id GOAL --item ITEM (--fixed COMMIT | --moved GOAL | --accepted REASON)")
		return 2
	}
	closure := goal.ReadItemClosure{}
	flags.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "fixed":
			closure.Fixed = fixed
		case "moved":
			closure.Moved = moved
		case "accepted":
			closure.Accepted = accepted
		}
	})
	req, ok := readItemMutationRequestWithProof("read-items close", *root, *by, *lineage, observed, commandNow, dependencies)
	if !ok {
		return 1
	}
	var result goal.PublishResult
	var err error
	if resolveCodeCommit == nil {
		result, err = goal.CloseReadItem(req, *id, *item, closure)
	} else {
		result, err = goal.CloseReadItemWithResolver(req, *id, *item, closure, resolveCodeCommit)
	}
	return dependencies.publish(result, err)
}

type readItemsGoalJSON struct {
	Goal  string          `json:"goal"`
	State string          `json:"state"`
	Items []goal.ReadItem `json:"items"`
}

func runGoalReadItemsListWithInputs(args []string, commandNow func(string) (time.Time, error), resolve func(string) (goal.Endpoint, error)) int {
	flags := flag.NewFlagSet("goal read-items list", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	id := flags.String("id", "", "one goal id")
	openOnly := flags.Bool("open", false, "only open items")
	jsonOutput := flags.Bool("json", false, "machine-readable output")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	if !converted(*root) {
		fmt.Fprintln(os.Stderr, "goal read-items list reads the synced backlog")
		return 1
	}
	endpoint, err := resolve(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	now, err := commandNow(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	projection, err := goal.Project(endpoint, false, now)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	all := map[string]*goal.GoalFile{}
	for goalID, file := range projection.Tree.Live {
		all[goalID] = file
	}
	for goalID, file := range projection.Tree.Done {
		all[goalID] = file
	}
	for goalID, file := range projection.Tree.Abandoned {
		all[goalID] = file
	}
	ids := make([]string, 0, len(all))
	if *id != "" {
		if all[*id] == nil {
			fmt.Fprintf(os.Stderr, "no goal %q on the accepted tree\n", *id)
			return 1
		}
		ids = append(ids, *id)
	} else {
		for goalID, file := range all {
			if len(file.ReadItems) > 0 {
				ids = append(ids, goalID)
			}
		}
		sort.Strings(ids)
	}
	rows := make([]readItemsGoalJSON, 0, len(ids))
	for _, goalID := range ids {
		file := all[goalID]
		items := make([]goal.ReadItem, 0, len(file.ReadItems))
		for _, readItem := range file.ReadItems {
			if !*openOnly || readItem.State == goal.ReadItemOpen {
				items = append(items, readItem)
			}
		}
		rows = append(rows, readItemsGoalJSON{Goal: goalID, State: file.State, Items: items})
	}
	if *jsonOutput {
		printJSON(map[string]any{"goals": rows, "tip": projection.Tip})
		return 0
	}
	for _, row := range rows {
		for _, readItem := range row.Items {
			fmt.Printf("goal=%s state=%s read=%s item=%s itemState=%s text=%s", row.Goal, row.State, readItem.Read, readItem.ID, readItem.State, strconv.Quote(readItem.Text))
			if readItem.ClosingReference != "" {
				fmt.Printf(" closingReference=%s", strconv.Quote(readItem.ClosingReference))
			}
			fmt.Println()
		}
	}
	return 0
}

// syncFlags is the shared flag surface; each verb reads the fields
// it consumes and ignores the rest.
type syncFlags struct {
	root, by, id, intent, next, origin, because, conclude, arc, pin, members string
	goal, branch, to                                                         string
	blocker, under, tiers, verbs, expires, verified                          string
	lineage, digest, elapsedLimit, approvedRef, temporaryWord, reviewBy      string
	budgetBox, confirm, risk, basis, evidence                                string
	finding, chain, why, test, implementationChain, artifact, result, critic string
	attemptLimit, reservedJobMinutesLimit, activeJobLimit, reviewRoundLimit  int64
	tier                                                                     uint
	labels, unlabels, ids, blocks, blockedBy                                 repeatedStrings
	claim, refreshOnly, sweep, fixtureHumanAuthority                         bool
	keep                                                                     int
}

type repeatedStrings []string

func (v *repeatedStrings) String() string { return fmt.Sprint([]string(*v)) }
func (v *repeatedStrings) Set(value string) error {
	*v = append(*v, value)
	return nil
}

func parseSyncFlags(name string, args []string) (*syncFlags, bool) {
	f, err := parseSyncFlagValues(name, args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return nil, false
	}
	return f, true
}

func parseHumanSyncFlags(values *humanVerbValues, name string, args []string) (*syncFlags, bool) {
	f, err := parseSyncFlagValues(name, args)
	if err == nil {
		values.bindSyncFlags(f)
		return f, true
	}
	message := strings.TrimPrefix(err.Error(), "goal "+name+" ")
	drop := humanFlagDrop(err.Error())
	remedy := humanVerbRemedy{words: "remove the unrecognized flag and retry"}
	if len(drop) > 0 {
		remedy = humanVerbRemedy{command: values.sameCommandWithout(drop...)}
	}
	refuseHumanVerb(values, 2, message, remedy)
	return nil, false
}

func humanFlagDrop(message string) []string {
	for _, name := range []string{"label", "unlabel", "tier", "members", "approved-ref"} {
		if strings.Contains(message, "--"+name) || strings.Contains(message, "-"+name) {
			return []string{name}
		}
	}
	if strings.Contains(message, "--risk, --basis, or --evidence") {
		return []string{"risk", "basis", "evidence"}
	}
	if strings.Contains(message, "budget flags") {
		return []string{"elapsed-limit", "attempt-limit", "reserved-job-minutes-limit", "active-job-limit", "review-round-limit"}
	}
	return nil
}

func parseSyncFlagValues(name string, args []string) (*syncFlags, error) {
	return parseSyncFlagValuesWithOutput(name, args, io.Discard)
}

func parseSyncFlagValuesWithOutput(name string, args []string, output io.Writer) (*syncFlags, error) {
	fs := flag.NewFlagSet("goal "+name, flag.ContinueOnError)
	fs.SetOutput(output)
	f := &syncFlags{}
	pathFlagVar(fs, &f.root, "root", ".", "checkout root")
	fs.StringVar(&f.by, "by", "", "the directing human (a human act carries its name)")
	if name == "approve" {
		fs.Var(&f.ids, "id", "goal id (repeatable)")
	} else {
		fs.StringVar(&f.id, "id", "", "goal id")
	}
	fs.StringVar(&f.intent, "intent", "", "one-line intent")
	fs.StringVar(&f.next, "next", "", "the next step")
	fs.StringVar(&f.origin, "origin", "main", "creation provenance: human|main")
	fs.Var(&f.blocks, "blocks", "the live goals this open unblocks (repeatable, or comma-separated): each parks with this blocker in the same publish and returns when every blocker it waits for is done (a seat's open, origin main, requires it, and reaches only the goal it holds)")
	fs.Var(&f.blockedBy, "blocked-by", "the goals this open waits for (repeatable, or comma-separated): it parks at once unless every one of them is already done")
	fs.StringVar(&f.blocker, "blocker", "", "block and unblock: the goal on the other end of the edge (--id names the goal that waits)")
	fs.StringVar(&f.under, "under", "", "act under a recorded power of attorney entry (approve, set-budget and unpark): the seat's own act, no --by and no proof")
	if name == "unpark" {
		fs.StringVar(&f.verified, "verified", "", "with --under: what the seat verified holds now, one line, recorded beside the park's reason")
	}
	fs.StringVar(&f.tiers, "tiers", "", "grant: the tiers the power of attorney covers (1 in this build)")
	fs.StringVar(&f.verbs, "verbs", "", "grant: the verbs the power of attorney covers, from approve,set-budget,unpark")
	fs.StringVar(&f.expires, "expires", "", "grant: the last day the power of attorney covers, YYYY-MM-DD, at most seven days out")
	fs.StringVar(&f.because, "because", "", "the park's reason")
	fs.StringVar(&f.conclude, "conclude", "", "the conclusion")
	fs.StringVar(&f.arc, "arc", "", "the destination arc")
	fs.StringVar(&f.pin, "pin", "", "the machine nickname a goal is pinned to (\"-\" clears)")
	fs.StringVar(&f.members, "members", "", "split member draft path")
	fs.StringVar(&f.goal, "goal", "", "fix goal for a trunk-red entry")
	fs.StringVar(&f.branch, "branch", "", "fix branch for a trunk-red entry")
	fs.StringVar(&f.to, "to", "", "machine receiving a human-assigned trunk-red entry")
	fs.StringVar(&f.finding, "finding", "", "finding identifier")
	fs.StringVar(&f.chain, "chain", "", "critic chain root")
	fs.StringVar(&f.why, "why", "", "decision reason")
	fs.StringVar(&f.risk, "risk", "", "four risk answers: severity=,novelty=,exposure=,accumulation=")
	fs.StringVar(&f.basis, "basis", "", "plain-English basis for the four risk answers")
	fs.StringVar(&f.evidence, "evidence", "", "misclassification evidence reference")
	fs.StringVar(&f.test, "test", "", "test citation")
	fs.StringVar(&f.implementationChain, "implementation-chain", "", "implementation chain carrying the fix")
	fs.StringVar(&f.artifact, "artifact", "", "changed artifact")
	fs.StringVar(&f.result, "result", "", "retained governed test result")
	fs.StringVar(&f.critic, "critic", "", "clean code-critic root")
	fs.StringVar(&f.lineage, "lineage", "", "this coordinator's lineage (or export METASYSTEM_OWNER_LINEAGE)")
	fs.StringVar(&f.digest, "digest", "", "the declaration's freshness digest (declare-free)")
	fs.StringVar(&f.approvedRef, "approved-ref", "", "recorded human approval reference for an over-norm goal budget")
	fs.UintVar(&f.tier, "tier", 0, "goal rigor tier: 1, 2, or 3")
	fs.StringVar(&f.elapsedLimit, "elapsed-limit", "", "positive elapsed duration, for example 4h")
	fs.Int64Var(&f.attemptLimit, "attempt-limit", 0, "positive reservation-attempt limit")
	fs.Int64Var(&f.reservedJobMinutesLimit, "reserved-job-minutes-limit", 0, "positive reserved job-minute limit")
	fs.Int64Var(&f.activeJobLimit, "active-job-limit", 0, "positive concurrent-job limit")
	fs.Int64Var(&f.reviewRoundLimit, "review-round-limit", -1, "non-negative critic review-round limit")
	if name == "approve" {
		fs.StringVar(&f.budgetBox, "budget", "", "standing budget box: box")
		fs.BoolVar(&f.sweep, "sweep", false, "preview or confirm the grandfather approval sweep")
		fs.StringVar(&f.confirm, "confirm", "", "sha256 from the exact sweep listing")
	}
	if name == "budget" || name == "resume" || name == "approve" || name == "unapprove" || name == "set-budget" || name == "accept-risk" || name == "open" || name == "edit" || name == "grant" || name == "revoke" || name == "unblock" {
		fs.StringVar(&f.temporaryWord, "temporary-human-word", "", "recorded relayed words presented as the human's; provenance is not verified; resumes TEMPORARILY")
		fs.StringVar(&f.reviewBy, "review-by", "", "recorded re-approval date supplied with the relay (required with --temporary-human-word)")
	}
	if name == "budget" || name == "resume" || name == "approve" || name == "unapprove" || name == "set-budget" || name == "accept-risk" || name == "open" || name == "edit" || name == "grant" || name == "revoke" || name == "park" || name == "unpark" || name == "reopen" || name == "unblock" || name == "done" {
		fs.BoolVar(&f.fixtureHumanAuthority, "fixture-human-authority", false, "fixture-only enrolled-human proof; accepted only for an exact fake-runtime root")
	}
	fs.Var(&f.labels, "label", "label token (repeatable)")
	fs.Var(&f.unlabels, "unlabel", "label token to remove (repeatable; edit only)")
	fs.BoolVar(&f.claim, "claim", false, "claim on open")
	fs.BoolVar(&f.refreshOnly, "refresh-only", false, "complete a died refresh")
	fs.IntVar(&f.keep, "keep", 10, "archive entries to keep")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if name != "open" && (len(f.blocks) > 0 || len(f.blockedBy) > 0) {
		return nil, fmt.Errorf("goal %s does not take --blocks or --blocked-by", name)
	}
	if name != "block" && name != "unblock" && f.blocker != "" {
		return nil, fmt.Errorf("goal %s does not take --blocker; the edge verbs are goal block and goal unblock", name)
	}
	if name != "approve" && name != "set-budget" && name != "unpark" && f.under != "" {
		return nil, fmt.Errorf("goal %s does not take --under; a power of attorney covers approve, set-budget and unpark", name)
	}
	if name != "grant" && (f.tiers != "" || f.verbs != "" || f.expires != "") {
		return nil, fmt.Errorf("goal %s does not take --tiers, --verbs or --expires", name)
	}
	if name != "open" && name != "edit" {
		if len(f.labels) > 0 {
			return nil, fmt.Errorf("goal %s does not take --label", name)
		}
		if len(f.unlabels) > 0 {
			return nil, fmt.Errorf("goal %s does not take --unlabel", name)
		}
		if f.tier != 0 {
			return nil, fmt.Errorf("goal %s does not take --tier", name)
		}
		if f.risk != "" || f.basis != "" || f.evidence != "" {
			return nil, fmt.Errorf("goal %s does not take --risk, --basis, or --evidence", name)
		}
	}
	if name != "open" && name != "claim" && name != "budget" && name != "set-budget" && name != "resume" && name != "approve" && f.hasAnyBudgetFlag() {
		return nil, fmt.Errorf("goal %s does not take budget flags", name)
	}
	if f.approvedRef != "" && name != "budget" && name != "set-budget" && name != "resume" && name != "approve" {
		return nil, fmt.Errorf("goal %s does not take --approved-ref", name)
	}
	if f.members != "" && name != "split" {
		return nil, fmt.Errorf("goal %s does not take --members", name)
	}
	if name != "trunk-red own" && (f.goal != "" || f.branch != "" || f.to != "") {
		return nil, fmt.Errorf("goal %s does not take --goal, --branch, or --to", name)
	}
	if name == "trunk-red own" && f.to != "" && f.by == "" {
		return nil, fmt.Errorf("goal trunk-red own takes --to only with --by")
	}
	return f, nil
}

func runGoalDischargeReviewObligation(args []string) int {
	return runGoalDischargeReviewObligationWithOwners(args, syncReq, dischargeReviewObligation)
}

func runGoalDischargeReviewObligationWithOwners(args []string, requestBuilder func(verb, root, by, lineage string) (goal.VerbRequest, error), discharge func(goal.VerbRequest, string, string, string, string, string, ...goal.DischargeEvidence) (goal.PublishResult, error)) int {
	return runGoalDischargeReviewObligationWithDependencies(args, requestBuilder, discharge, defaultSyncRequestDependencies())
}

func runGoalDischargeReviewObligationWithDependencies(args []string, requestBuilder func(verb, root, by, lineage string) (goal.VerbRequest, error), discharge func(goal.VerbRequest, string, string, string, string, string, ...goal.DischargeEvidence) (goal.PublishResult, error), dependencies syncRequestDependencies) int {
	f, ok := dependencies.parseSyncFlags("discharge-review-obligation", args)
	if !ok || f.id == "" || f.finding == "" || f.chain == "" || f.by == "" || f.test == "" && (f.chain == goal.HumanCarriedChain || f.implementationChain == "" || f.artifact == "" || f.result == "" || f.critic == "") {
		dependencies.complain("goal discharge-review-obligation needs --id, --finding, --chain, and --by; non-fixture obligations also need --test, while fixture obligations need --implementation-chain, --artifact, --result, and --critic")
		return 2
	}
	if f.chain == goal.HumanCarriedChain {
		commit, commitErr := humanCarriedFindingCommit(f.finding)
		if commitErr != nil {
			dependencies.complain(commitErr)
			return 1
		}
		if err := dispatchcore.ValidateHumanCarriedCritic(f.root, f.test, commit); err != nil {
			dependencies.complain(err)
			return 1
		}
	}
	req, err := requestBuilder("discharge-review-obligation", f.root, f.by, f.lineage)
	if err != nil {
		dependencies.complain(err)
		return 1
	}
	if req.CallerClass == lease.ClassHuman {
		req.Actor.Human = f.by
	}
	evidence := goal.DischargeEvidence{Root: f.root, ImplementationChain: f.implementationChain, Artifact: f.artifact, ResultRunID: f.result, CriticRoot: f.critic}
	res, err := discharge(req, f.id, f.finding, f.chain, f.by, f.test, evidence)
	return dependencies.publish(res, err)
}

func runGoalAcceptRisk(args []string) int {
	return runGoalAcceptRiskWithAuthority(args, humanauthority.ProveOrTemporaryGoalAuthority)
}

func runGoalAcceptRiskWithAuthority(args []string, prove goalAuthorityProver) int {
	return runGoalAcceptRiskWithInputs(args, prove, goalCommandNow, defaultSyncRequestDependencies())
}

func runGoalAcceptRiskWithInputs(args []string, prove goalAuthorityProver, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies) int {
	return runGoalAcceptRiskWithFacts(args, prove, commandNow, dependencies, nil)
}

func runGoalAcceptRiskWithFacts(args []string, prove goalAuthorityProver, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies, commitMessage func(root, commit string) ([]byte, error)) int {
	values := newHumanVerbValues("accept-risk", args)
	values.report = dependencies.report
	f, ok := parseHumanSyncFlags(values, "accept-risk", args)
	if !ok {
		return 2
	}
	if f.id == "" || f.finding == "" || f.chain == "" || strings.TrimSpace(f.why) == "" {
		return refuseHumanVerb(values, 2, "needs --id, --finding, --chain, and --why", humanVerbRemedy{words: "add the missing decision value named in the refusal"})
	}
	f.why = strings.TrimSpace(f.why)
	if (f.temporaryWord == "") != (f.reviewBy == "") {
		return refuseHumanVerb(values, 2, "--temporary-human-word and --review-by travel together", humanVerbRemedy{command: values.sameCommandWithout("temporary-human-word", "review-by")})
	}
	classification, err := classifyGoalAuthorityFirstWithFacts("accept-risk", f, dependencies.authorityFacts)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "choose an open severe or unproven finding from the named critique register"})
	}
	var finding dispatchcore.CritiqueDecisionFinding
	if f.chain != goal.HumanCarriedChain {
		finding, err = dispatchcore.CritiqueRegisterDecisionFinding(f.root, f.chain, f.finding, f.id)
		if err != nil {
			dependencies.complain(err)
			return 1
		}
	}
	proof, err := proveGoalHumanAuthorityAt("accept-risk", f, prove, commandNow)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanProofRemedy(values, f.fixtureHumanAuthority, f.temporaryWord, f.reviewBy))
	}
	if err := resolveGoalHuman(f, proof); err != nil {
		return refuseHumanVerb(values, 2, err.Error(), humanVerbRemedy{words: "re-enroll with goal enroll-terminal --by <your name>, or add --by <your name> to this command"})
	}
	values.by = f.by
	req, err := syncReqClassifiedWithTerminalGradeAtWithDependencies(f.root, f.by, f.lineage, &proof, classification, false, commandNow, dependencies)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "repair the named checkout identity fact before retrying"})
	}
	opid := goal.Opid(req.Ulid, req.Actor.Machine, req.Actor.Lineage)
	var carriedRisk counselor.CarriedAcceptedRiskAppend
	if f.chain == goal.HumanCarriedChain {
		commit, commitErr := humanCarriedFindingCommit(f.finding)
		if commitErr != nil {
			dependencies.complain(commitErr)
			return 1
		}
		carriedRisk = counselor.CarriedAcceptedRiskAppend{Goal: f.id, Finding: f.finding, By: f.by, Why: f.why, OpID: opid, Commit: commit, RecordedAt: req.Now, CommitMessage: commitMessage}
		if err := counselor.ValidateCarriedAcceptedRisk(f.root, carriedRisk); err != nil {
			dependencies.complain(err)
			return 1
		}
	}
	res, err := goal.AcceptedRiskDecision(req, f.id, f.finding, f.chain, f.by, f.why, &proof)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "read the current goal and critique finding before retrying"})
	}
	if res.Outcome != goal.OutcomeConfirmed {
		dependencies.outcomeBeforeRefusal(res)
		return refuseHumanVerb(values, 1, res.Detail, humanVerbRemedy{words: "read the current goal and critique finding before retrying"})
	}
	// The goal act has landed; a later refusal reports it as partial.
	dependencies.landed(res)
	opid, err = goal.AcceptedRiskDecisionOpIDWithResolver(f.root, f.id, f.finding, f.chain, req.Now, dependencies.endpoint)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "the goal act landed, but its accepted-risk record needs repair"})
	}
	if f.chain == goal.HumanCarriedChain {
		carriedRisk.OpID = opid
		if err := counselor.AppendCarriedAcceptedRisk(f.root, carriedRisk); err != nil {
			return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "the goal act landed, but its carried accepted-risk record needs repair"})
		}
	} else {
		if err := counselor.AppendAcceptedRisk(f.root, counselor.AcceptedRiskAppend{Goal: f.id, RootJob: f.chain, FindingID: f.finding, Class: finding.RigorClass, Title: finding.Title, Claim: finding.Claim, Evidence: finding.Evidence, Why: f.why, OpID: opid, RecordedAt: req.Now}); err != nil {
			return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "the goal act landed, but its accepted-risk record needs repair"})
		}
		if err := dispatchcore.CritiqueRegisterAcceptRisk(f.root, f.chain, f.finding, opid); err != nil {
			return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "the goal act landed, but the critique register needs repair"})
		}
	}
	if err := recordGoalApprovalProof(f.root, opid, "goal accept-risk", proof); err != nil {
		return refuseHumanVerb(values, 1, "the act landed at tip "+res.Tip+", but its authority proof did not: "+err.Error(), humanVerbRemedy{words: "the act already landed; do not run it again"})
	}
	return dependencies.publish(res, nil)
}

func humanCarriedFindingCommit(finding string) (string, error) {
	value := strings.TrimPrefix(finding, "carried:")
	value = strings.TrimSuffix(value, ":battery-red")
	if value == finding || len(value) != 40 || !allLowerHex(value) || finding != "carried:"+value && finding != "carried:"+value+":battery-red" {
		return "", fmt.Errorf("human-carried finding must be carried:<sha40> or carried:<sha40>:battery-red")
	}
	return value, nil
}

func (f *syncFlags) hasAnyBudgetFlag() bool {
	return f.elapsedLimit != "" || f.attemptLimit != 0 || f.reservedJobMinutesLimit != 0 || f.activeJobLimit != 0 || f.reviewRoundLimit >= 0
}

func (f *syncFlags) budgetTuple(required bool) (*goal.Budget, error) {
	if !f.hasAnyBudgetFlag() {
		if required {
			return nil, fmt.Errorf("the complete budget tuple is required: --elapsed-limit, --attempt-limit, --reserved-job-minutes-limit, --active-job-limit, and --review-round-limit")
		}
		return nil, nil
	}
	if f.elapsedLimit == "" || f.attemptLimit == 0 || f.reservedJobMinutesLimit == 0 || f.activeJobLimit == 0 || f.reviewRoundLimit < 0 {
		return nil, fmt.Errorf("budget flags are all-or-nothing: supply --elapsed-limit, --attempt-limit, --reserved-job-minutes-limit, --active-job-limit, and --review-round-limit")
	}
	budget, err := goal.NewBudget(f.elapsedLimit, f.attemptLimit, f.reservedJobMinutesLimit, f.activeJobLimit, f.reviewRoundLimit)
	if err != nil {
		return nil, err
	}
	return &budget, nil
}

func (f *syncFlags) approvalBudget() (*goal.Budget, error) {
	if f.budgetBox != "" && f.hasAnyBudgetFlag() {
		return nil, fmt.Errorf("--budget and the five explicit budget limits are mutually exclusive")
	}
	if f.budgetBox != "" && f.budgetBox != "box" {
		return nil, fmt.Errorf("--budget is box")
	}
	return f.budgetTuple(false)
}

func extractGoalBudgetBox(args []string) ([]string, string, error) {
	cleaned := make([]string, 0, len(args))
	box := ""
	for index := 0; index < len(args); index++ {
		token := args[index]
		if strings.HasPrefix(token, "--") {
			cleaned = append(cleaned, token)
			nameValue := strings.TrimPrefix(token, "--")
			name, _, joined := strings.Cut(nameValue, "=")
			if joined || goalBooleanFlag(name) || index+1 >= len(args) {
				continue
			}
			index++
			cleaned = append(cleaned, args[index])
			continue
		}
		if token == "norm" || token == "keep" || strings.Contains(token, "/") {
			if box != "" {
				return nil, "", fmt.Errorf("takes one box, not both %q and %q", box, token)
			}
			box = token
			continue
		}
		return nil, "", fmt.Errorf("does not recognize positional value %q as a box", token)
	}
	return cleaned, box, nil
}

func completeBudgetBox(value string, fallback goal.Budget, standing *goal.Budget, reviewRoundMax uint64) (goal.Budget, string, bool) {
	parts := strings.Split(value, "/")
	if len(parts) > 5 {
		firstFive := strings.Join(parts[:5], "/")
		completed, err := goalbudget.ParseBox(firstFive, standing, reviewRoundMax)
		return completed, firstFive, err == nil
	}
	base := strings.Split(goalbudget.FormatBox(fallback), "/")
	for len(parts) < 5 {
		parts = append(parts, "")
	}
	valid := []func(string) bool{
		func(part string) bool { _, ok := goalbudget.ParseWorkingDuration(part); return ok },
		func(part string) bool { value, err := strconv.ParseUint(part, 10, 64); return err == nil && value > 0 },
		func(part string) bool {
			value, err := strconv.ParseUint(strings.TrimSuffix(part, "m"), 10, 64)
			return strings.HasSuffix(part, "m") && err == nil && value > 0
		},
		func(part string) bool { value, err := strconv.ParseUint(part, 10, 64); return err == nil && value > 0 },
		func(part string) bool { value, err := strconv.ParseInt(part, 10, 64); return err == nil && value >= 0 },
	}
	for index := range parts {
		if !valid[index](parts[index]) {
			parts[index] = base[index]
		}
	}
	completed, err := goalbudget.ParseBox(strings.Join(parts, "/"), nil, reviewRoundMax)
	if err != nil {
		return goal.Budget{}, "", false
	}
	return completed, goalbudget.FormatBox(completed), true
}

func completedBudgetRemedy(values *humanVerbValues, file *goal.GoalFile, completed goal.Budget, box string, horizon goal.ApprovalHorizon) humanVerbRemedy {
	if file.Budget == nil || *file.Budget != completed {
		return humanVerbRemedy{command: values.budgetCommand(box)}
	}
	switch file.State {
	case goal.StateClaimed:
		if file.StopFence != nil {
			return humanVerbRemedy{command: values.budgetCommandWithoutApprovedRef("keep")}
		}
		if values.approvedRef == "" {
			return humanVerbRemedy{words: "the goal already carries that box, so there is no new act to record"}
		}
	case goal.StateApproved, goal.StateParked:
		expired, _ := file.ApprovalExpired(horizon)
		if file.Approved != nil && file.Approved.Authority == goal.ApprovalAuthorityProven &&
			values.temporaryWord == "" && values.reviewBy == "" && !expired {
			return humanVerbRemedy{words: "the goal already carries that box, so there is no new act to record"}
		}
	}
	return humanVerbRemedy{command: values.budgetCommand(box)}
}

func malformedBudgetRemedy(values *humanVerbValues, file *goal.GoalFile, value string, fallback goal.Budget, reviewRoundMax uint64, parseErr error, horizon goal.ApprovalHorizon) humanVerbRemedy {
	if strings.Contains(parseErr.Error(), "exceeds configured maximum") {
		return humanVerbRemedy{words: parseErr.Error()}
	}
	completed, box, ok := completeBudgetBox(value, fallback, file.Budget, reviewRoundMax)
	if !ok {
		if len(strings.Split(value, "/")) > 5 {
			return humanVerbRemedy{words: "the first five compact-box members do not form a valid box"}
		}
		return humanVerbRemedy{words: "the compact box cannot be completed from the goal's standing limits"}
	}
	values.box = &completed
	return completedBudgetRemedy(values, file, completed, box, horizon)
}

func bindGoalTierView(values *humanVerbValues, root string, file *goal.GoalFile) error {
	tier := file.Tier
	if tier == 0 {
		tier = 3
	}
	tierBox, err := config.TierBox(filepath.Join(root, "metasystem.conf"), tier)
	if err != nil {
		return err
	}
	values.bindGoalView(file, tierBox)
	return nil
}

func resolveGoalHuman(flags *syncFlags, proof humanauthority.Proof) error {
	if flags.by != "" {
		return nil
	}
	if !proof.EnrolledTerminalFor(flags.root) && !(proof.FixtureOnly && proof.ValidFor(flags.root)) {
		return fmt.Errorf("the enrolled terminal has no recorded name")
	}
	enrollment, err := humanauthority.ReadEnrollment(flags.root)
	if err != nil || strings.TrimSpace(enrollment.Human) == "" {
		return fmt.Errorf("the enrolled terminal has no recorded name")
	}
	if proof.EnrolledTerminalFor(flags.root) &&
		(proof.TerminalGeneration != enrollment.Generation || proof.TerminalRef != enrollment.TerminalRef) {
		return fmt.Errorf("the enrolled terminal has no recorded name")
	}
	flags.by = enrollment.Human
	return nil
}

func runGoalBudget(args []string) int {
	return runGoalBudgetWithAuthority(args, humanauthority.ProveOrTemporaryGoalAuthority)
}

func runGoalBudgetWithAuthority(args []string, prove goalAuthorityProver) int {
	return runGoalBudgetWithAuthorityAt(args, prove, goalCommandNow)
}

func runGoalBudgetWithAuthorityAt(args []string, prove goalAuthorityProver, commandNow func(string) (time.Time, error)) int {
	return runGoalBudgetWithInputs(args, prove, commandNow, defaultSyncRequestDependencies(), dispatchcore.ResolveGoalBinding)
}

type goalBindingResolver func(string, string, time.Time) (dispatchcore.GoalBinding, error)

func runGoalBudgetWithInputs(args []string, prove goalAuthorityProver, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies, binding goalBindingResolver) int {
	values := newHumanVerbValues("budget", args)
	values.report = dependencies.report
	cleaned, box, err := extractGoalBudgetBox(args)
	if err != nil {
		return refuseHumanVerb(values, 2, err.Error(), humanVerbRemedy{words: "give exactly one compact box after or before the flags"})
	}
	flags, ok := parseHumanSyncFlags(values, "budget", cleaned)
	if !ok {
		return 2
	}
	values.boxTyped = box
	return runGoalBudgetPreparedWithInputs(values, flags, box, prove, commandNow, dependencies, binding)
}

func runGoalBudgetPreparedWithInputs(values *humanVerbValues, flags *syncFlags, boxToken string, prove goalAuthorityProver, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies, binding goalBindingResolver) int {
	values.bindSyncFlags(flags)
	values.boxTyped = boxToken
	values.report = dependencies.report
	if !converted(flags.root) {
		return refuseHumanVerb(values, 1, "works only with the synced backlog", humanVerbRemedy{words: "migrate this checkout to the synced backlog first"})
	}
	if flags.id == "" {
		return refuseHumanVerb(values, 2, "needs --id", humanVerbRemedy{words: "add the live goal's identifier with --id"})
	}
	if dependencies.endpoint == nil {
		return refuseHumanVerb(values, 1, "goal endpoint reader is missing", humanVerbRemedy{words: "repair the checkout's goal synchronization endpoint"})
	}
	endpoint, err := dependencies.endpoint(flags.root)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "repair the checkout's goal synchronization endpoint"})
	}
	if commandNow == nil {
		return refuseHumanVerb(values, 1, "goal command clock is missing", humanVerbRemedy{words: "provide a valid goal command clock"})
	}
	now, err := commandNow(flags.root)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "provide a valid goal command clock"})
	}
	projection, err := goal.Project(endpoint, false, now)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "repair the accepted goal projection before retrying"})
	}
	file := projection.Tree.Live[flags.id]
	if file == nil {
		if projection.Tree.Done[flags.id] != nil {
			return refuseHumanVerb(values, 1, "the goal is done and its archived budget is read-only", humanVerbRemedy{words: "reopen the goal before giving its live revision a box"})
		}
		return refuseHumanVerb(values, 1, "the goal identifier is unknown", humanVerbRemedy{words: "choose an identifier from metasystem goal list"})
	}
	tier := file.Tier
	if tier == 0 {
		tier = 3
	}
	tierBox, err := config.TierBox(filepath.Join(flags.root, "metasystem.conf"), tier)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "repair the configured tier box"})
	}
	reviewRoundMax, err := config.ReviewRoundMax(filepath.Join(flags.root, "metasystem.conf"))
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "repair the configured review-round limit"})
	}
	values.bindGoalView(file, tierBox)
	approvalHorizon := goal.ApprovalHorizon{Now: now}
	var budget goal.Budget
	switch {
	case boxToken != "" && flags.hasAnyBudgetFlag():
		parsed, parseErr := goalbudget.ParseBox(boxToken, file.Budget, reviewRoundMax)
		if parseErr != nil {
			fallback := tierBox
			if file.Budget != nil {
				fallback = *file.Budget
			}
			return refuseHumanVerb(values, 2, "a compact box and the five long-form limits cannot be combined", malformedBudgetRemedy(values, file, boxToken, fallback, reviewRoundMax, parseErr, approvalHorizon))
		}
		values.box = &parsed
		return refuseHumanVerb(values, 2, "a compact box and the five long-form limits cannot be combined", completedBudgetRemedy(values, file, parsed, goalbudget.FormatBox(parsed), approvalHorizon))
	case boxToken == "norm":
		budget = tierBox
	case boxToken == "keep":
		if file.Budget == nil {
			return refuseHumanVerb(values, 2, "keep needs a standing budget", humanVerbRemedy{command: values.budgetCommand(goalbudget.FormatBox(tierBox))})
		}
		budget = *file.Budget
	case boxToken != "":
		budget, err = goalbudget.ParseBox(boxToken, file.Budget, reviewRoundMax)
		if err != nil {
			fallback := tierBox
			if file.Budget != nil {
				fallback = *file.Budget
			}
			return refuseHumanVerb(values, 2, err.Error(), malformedBudgetRemedy(values, file, boxToken, fallback, reviewRoundMax, err, approvalHorizon))
		}
	default:
		budgetPointer, budgetErr := flags.budgetTuple(true)
		if budgetErr != nil {
			fallback := tierBox
			if file.Budget != nil {
				fallback = *file.Budget
			}
			values.box = &fallback
			return refuseHumanVerb(values, 2, budgetErr.Error(), completedBudgetRemedy(values, file, fallback, goalbudget.FormatBox(fallback), approvalHorizon))
		}
		budget = *budgetPointer
		if budgetErr := budget.Validate(reviewRoundMax); budgetErr != nil {
			return refuseHumanVerb(values, 2, budgetErr.Error(), humanVerbRemedy{words: budgetErr.Error()})
		}
	}
	values.box = &budget
	if file.StopFence != nil && file.Budget != nil && budget != *file.Budget {
		return refuseHumanVerb(values, 1, "a breach-stopped goal resumes under its standing box before a new box is recorded", humanVerbRemedy{command: values.budgetCommandWithoutApprovedRef("keep")})
	}
	if _, err := classifyGoalAuthorityFirstWithFacts(values.verb, flags, dependencies.authorityFacts); err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "run this at the enrolled terminal"})
	}
	proof, err := proveGoalHumanAuthorityAt("budget", flags, prove, commandNow)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanProofRemedy(values, flags.fixtureHumanAuthority, flags.temporaryWord, flags.reviewBy))
	}
	if err := resolveGoalHuman(flags, proof); err != nil {
		return refuseHumanVerb(values, 2, err.Error(), humanVerbRemedy{words: "re-enroll with goal enroll-terminal --by <your name>, or add --by <your name> to this command"})
	}
	values.by = flags.by
	req, err := syncReqWithProofAtWithDependencies("budget", flags.root, flags.by, flags.lineage, &proof, commandNow, dependencies)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "repair the named checkout identity fact before retrying"})
	}
	req.ApprovedRef = flags.approvedRef
	var result goal.PublishResult
	action := ""
	switch file.State {
	case goal.StateQueued, goal.StateApproved, goal.StateParked:
		result, err = goal.Approve(req, []string{flags.id}, &budget, &proof)
		action = "goal approve"
	case goal.StateClaimed:
		if file.StopFence == nil {
			result, err = goal.SetBudgetApproved(req, flags.id, budget, &proof)
			action = "goal set-budget"
		} else {
			if binding == nil {
				return refuseHumanVerb(values, 1, "stopped goal binding reader is missing", humanVerbRemedy{words: "repair the stopped goal binding before retrying"})
			}
			binding, bindingErr := binding(flags.root, flags.id, req.Now)
			if bindingErr != nil {
				return refuseHumanVerb(values, 1, bindingErr.Error(), humanVerbRemedy{words: "repair the stopped goal binding before retrying"})
			}
			held, lockErr := goalrevision.Acquire(flags.root, flags.id, binding.Revision, "goal-resume")
			if lockErr != nil {
				return refuseHumanVerb(values, 1, lockErr.Error(), humanVerbRemedy{words: "wait for the goal-revision lock holder to finish"})
			}
			defer held.Release()
			result, err = goal.Resume(goal.ResumeRequest{VerbRequest: req, GoalID: flags.id, Budget: budget, Authority: &proof})
			action = "goal resume"
		}
	default:
		return refuseHumanVerb(values, 1, "the live goal state cannot receive a budget", humanVerbRemedy{words: "repair the goal's state before retrying"})
	}
	if err != nil || result.Outcome != goal.OutcomeConfirmed {
		if err == nil {
			dependencies.showOutcome(result)
		}
		detail := result.Detail
		if err != nil {
			detail = err.Error()
		}
		if file.StopFence != nil && flags.approvedRef != "" && strings.Contains(detail, "--approved-ref") {
			return refuseHumanVerb(values, 1, detail, humanVerbRemedy{command: values.budgetCommandWithoutApprovedRef("keep")})
		}
		if flags.approvedRef != "" && strings.Contains(detail, "--approved-ref") {
			return refuseHumanVerb(values, 1, detail, humanVerbRemedy{words: "the approved reference must cover this exact goal revision and box"})
		}
		if strings.Contains(detail, "GOAL_NORM_REFUSED") && !proof.EnrolledTerminalFor(flags.root) {
			return refuseHumanVerb(values, 1, detail, humanVerbRemedy{words: "run the over-norm box at a real enrolled terminal, or pass a recorded --approved-ref"})
		}
		if result.Outcome == goal.OutcomeAbandoned {
			return refuseHumanVerb(values, 1, detail, humanVerbRemedy{words: "the goal already has that box, so there is no new act to record"})
		}
		return refuseHumanVerb(values, 1, detail, budgetRemedyAfterRefusal(values, endpoint, now))
	}
	dependencies.landed(result)
	operationID := goal.Opid(req.Ulid, req.Actor.Machine, req.Actor.Lineage)
	if action == "goal resume" {
		err = humanauthority.RecordResumeProof(flags.root, operationID, proof)
	} else {
		err = recordGoalApprovalProof(flags.root, operationID, action, proof)
	}
	if err != nil {
		return refuseHumanVerb(values, 1, "the act landed at tip "+result.Tip+", but its authority proof did not: "+err.Error(), humanVerbRemedy{words: "the act already landed; do not run it again"})
	}
	if budgetExceedsCommandBox(budget, tierBox) {
		count := file.BudgetExceptions
		if action == "goal set-budget" && count < ^uint16(0) {
			count++
		}
		if after, readErr := goal.Project(endpoint, false, now); readErr == nil && after.Tree.Live[flags.id] != nil {
			count = after.Tree.Live[flags.id].BudgetExceptions
		}
		dependencies.note(true, fmt.Sprintf("goal budget: %s exceeds tier %d box %s; exception count %d", goalbudget.FormatBox(budget), tier, goalbudget.FormatBox(tierBox), count))
	}
	return dependencies.publish(result, nil)
}

func budgetExceedsCommandBox(budget, box goal.Budget) bool {
	return budget.ElapsedDuration() > box.ElapsedDuration() || budget.AttemptLimit > box.AttemptLimit ||
		budget.ReservedJobMinutesLimit > box.ReservedJobMinutesLimit || budget.ActiveJobLimit > box.ActiveJobLimit ||
		budget.ReviewRoundLimit > box.ReviewRoundLimit
}

func budgetRemedyAfterRefusal(values *humanVerbValues, endpoint goal.Endpoint, now time.Time) humanVerbRemedy {
	projection, err := goal.Project(endpoint, false, now)
	if err != nil {
		return humanVerbRemedy{words: "repair the accepted goal projection before retrying"}
	}
	file := projection.Tree.Live[values.id]
	if file == nil {
		if projection.Tree.Done[values.id] != nil {
			return humanVerbRemedy{words: "reopen the goal before giving its live revision a box"}
		}
		return humanVerbRemedy{words: "choose an identifier from metasystem goal list"}
	}
	if file.StopFence != nil {
		return humanVerbRemedy{command: values.budgetCommandWithoutApprovedRef("keep")}
	}
	box := values.suggestedBudgetBox()
	if box == "" && file.Budget != nil {
		box = goalbudget.FormatBox(*file.Budget)
	}
	if box == "" {
		return humanVerbRemedy{words: "choose a complete box for the live goal"}
	}
	return humanVerbRemedy{command: values.budgetCommand(box)}
}

// trySyncMutation intercepts a legacy mutation command on a
// CONVERTED checkout and routes it to the engine verb. Returns
// handled=false on a legacy checkout so the caller proceeds
// unchanged.
func trySyncMutation(name string, args []string) (int, bool) {
	return trySyncMutationWithDependencies(name, args, goalCommandNow, defaultSyncRequestDependencies(), goalParkBranchCheck)
}

func trySyncMutationWithDependencies(name string, args []string, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies, parkBranchCheck func(string, goal.Endpoint) func(string, string) (string, error)) (int, bool) {
	return trySyncMutationWithCompletion(name, args, commandNow, dependencies, parkBranchCheck, completionInputs{})
}

type completionInputs struct {
	localTip    func(repo, ref string) (string, bool, error)
	endpointTip func(string, goal.Endpoint) (string, error)
	reporter    func(metrics.Options) (metrics.Result, error)
}

func trySyncMutationWithCompletion(name string, args []string, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies, parkBranchCheck func(string, goal.Endpoint) func(string, string) (string, error), completion completionInputs) (int, bool) {
	f, ok := dependencies.parseSyncFlags(name, args)
	if !ok {
		return 2, true
	}
	if !converted(f.root) {
		return 0, false
	}
	if name == "reopen" {
		if f.id == "" {
			dependencies.complain("goal reopen needs --id")
			return 2, true
		}
		endpoint, endpointErr := dependencies.endpoint(f.root)
		if endpointErr != nil {
			dependencies.complain(endpointErr)
			return 1, true
		}
		now, nowErr := commandNow(f.root)
		if nowErr != nil {
			dependencies.complain(nowErr)
			return 1, true
		}
		projection, projectErr := goal.Project(endpoint, false, now)
		if projectErr != nil {
			dependencies.complain(projectErr)
			return 1, true
		}
		if projection.Tree.Abandoned[f.id] != nil {
			if f.by == "" {
				dependencies.complain("goal reopen from abandoned needs --by at the enrolled terminal")
				return 2, true
			}
			proof, proofErr := proveGoalHumanAuthorityAt("reopen", f, proveEnrolledGoalHumanAuthority, commandNow)
			if proofErr != nil {
				dependencies.complain(proofErr)
				return 1, true
			}
			provenRequest, requestErr := syncReqWithProofAtWithDependencies("reopen", f.root, f.by, f.lineage, &proof, commandNow, dependencies)
			if requestErr != nil {
				dependencies.complain(requestErr)
				return 1, true
			}
			res, runErr := goal.ReopenAbandoned(provenRequest, f.id, &proof)
			return dependencies.publish(res, runErr), true
		}
	}
	var req goal.VerbRequest
	var err error
	// A stopping act by a person takes the terminal grade (the enrollment
	// walk without the write). An unpark under a power of attorney is the
	// seat's own act and takes no proof: it is dispatched below to
	// runGoalUnderAttorneyWithInputs, whose refusals (a --by beside --under, a proof
	// beside it) must be reached, so the walk is not run in front of it.
	if ((name == "park" || name == "unpark") && f.under == "") || (name == "done" && f.by != "") {
		var proof *humanauthority.Proof
		if f.fixtureHumanAuthority {
			observed, proofErr := proveFixtureGoalAuthority(name, f)
			if proofErr != nil {
				return dependencies.fail(1, proofErr), true
			}
			proof = &observed
		}
		req, err = syncStoppingReqWithProofWithDependencies(name, f.root, f.by, f.lineage, proof, commandNow, dependencies)
	} else {
		req, err = syncReqWithProofAtWithDependencies(name, f.root, f.by, f.lineage, nil, commandNow, dependencies)
	}
	if err != nil {
		return dependencies.fail(1, err), true
	}
	req.ApprovedRef = f.approvedRef
	need := func(val, flagName string) bool {
		if val == "" {
			dependencies.fail(2, fmt.Errorf("goal %s needs --%s", name, flagName))
			return false
		}
		return true
	}
	switch name {
	case "open":
		if len(f.unlabels) > 0 {
			dependencies.complain("goal open does not accept --unlabel; remove labels with goal edit")
			return 2, true
		}
		if f.risk == "" || strings.TrimSpace(f.basis) == "" {
			dependencies.complain("answer the four questions: --risk severity=,novelty=,exposure=,accumulation= --basis")
			return 2, true
		}
		if !need(f.id, "id") || !need(f.intent, "intent") || !need(f.next, "next") || f.tier > 3 {
			return 2, true
		}
		risk, riskErr := goal.ParseRiskRecord(f.risk, f.basis)
		if riskErr != nil {
			dependencies.complain(riskErr)
			return 2, true
		}
		// The open's own hand is proved whenever the command claims a person,
		// not only when it lowers a tier: the engine decides whose open this
		// is from the proof, and --origin human is a word anyone can type.
		var proof *humanauthority.Proof
		if claimsAHuman(f) || f.tier != 0 && uint8(f.tier) < risk.DerivedTier() {
			proven, proofErr := proveGoalHumanAuthority("open", f, humanauthority.ProveOrTemporaryGoalAuthority)
			if proofErr != nil {
				dependencies.complain(proofErr)
				return 1, true
			}
			proof = &proven
		}
		if f.claim {
			budget, budgetErr := f.budgetTuple(true)
			if budgetErr != nil {
				dependencies.complain(budgetErr)
				return 2, true
			}
			if detail := brain.Fence(f.root, "claim", goal.ExistingLedgerIdentity(f.root)); detail != "" {
				dependencies.complain(detail)
				return 1, true
			}
			res, err := goal.OpenClaim(req, f.id, f.intent, f.origin, f.next, *budget, f.labels...)
			code := dependencies.publish(res, err)
			hintDesignIsARecord(dependencies.noteWriter(), res.Outcome, f.id)
			return code, true
		}
		budget, budgetErr := f.budgetTuple(false)
		if budgetErr != nil {
			dependencies.complain(budgetErr)
			return 2, true
		}
		res, err := goal.OpenRisked(req, f.id, f.intent, f.origin, f.next, f.blocks, f.blockedBy, risk, uint8(f.tier), f.why, budget, proof, f.labels...)
		code := dependencies.publish(res, err)
		hintDesignIsARecord(dependencies.noteWriter(), res.Outcome, f.id)
		return code, true
	case "park":
		if !need(f.id, "id") || !need(f.because, "because") {
			return 2, true
		}
		if f.arc != "" {
			res, err := goal.ParkArc(req, f.id, f.because)
			return dependencies.publish(res, err), true
		}
		req.ParkBranchCheck = parkBranchCheck(f.root, req.Endpoint)
		res, err := goal.Park(req, f.id, f.because)
		return dependencies.publish(res, err), true
	case "unpark":
		if !need(f.id, "id") {
			return 2, true
		}
		if f.under != "" {
			if f.arc != "" {
				dependencies.complain("goal unpark --under lifts one goal's park; --arc is not taken")
				return 2, true
			}
			return runGoalUnderAttorneyWithInputs("unpark", f, commandNow, dependencies), true
		}
		if f.verified != "" {
			return dependencies.fail(2, fmt.Errorf("goal unpark --verified belongs to an unpark under a power of attorney: add --under <entry>")), true
		}
		if f.arc != "" {
			res, err := goal.UnparkArc(req, f.id)
			return dependencies.publish(res, err), true
		}
		res, err := goal.Unpark(req, f.id)
		return dependencies.publish(res, err), true
	case "done":
		if !need(f.id, "id") || !need(f.conclude, "conclude") {
			return 2, true
		}
		req.SweepBranch = func(goalID string) error {
			projection, err := goal.Project(req.Endpoint, false, req.Now)
			if err != nil {
				return err
			}
			dropped := ""
			if file := projection.Tree.Done[goalID]; file != nil {
				dropped = file.NextStep
			}
			var shouldSweep bool
			if completion.localTip == nil {
				shouldSweep, err = goalbranch.ShouldSweep(f.root, goalID, dropped)
			} else {
				shouldSweep, err = goalbranch.ShouldSweepWithLocalTip(f.root, goalID, dropped, completion.localTip)
			}
			if err != nil || !shouldSweep {
				return err
			}
			readEndpointTip := completion.endpointTip
			if readEndpointTip == nil {
				readEndpointTip = goalBranchEndpointTip
			}
			endpointTip, err := readEndpointTip(f.root, req.Endpoint)
			if err != nil {
				return err
			}
			transport := ""
			if _, remoteErr := goalBranchGit(f.root, "remote", "get-url", "transport"); remoteErr == nil {
				transport = "transport"
			}
			_, err = goalbranch.Sweep(goalbranch.SweepRequest{Repo: f.root, Remote: req.Endpoint.Remote, Transport: transport,
				EndpointTip: endpointTip, GoalID: goalID, Dropped: dropped, CheckClaim: func() error { return nil }})
			return err
		}
		res, err := goal.Done(req, f.id, f.conclude)
		code := dependencies.publish(res, err)
		if dependencies.report != nil {
			if code == 0 {
				if metricsErr := concludedGoalMetrics(f.root, f.id, completion.reporter); metricsErr != nil {
					dependencies.report.secondary = append(dependencies.report.secondary, metricsErr)
					dependencies.report.repair = []string{"metasystem", "metrics", "report", "--goal", f.id}
					dependencies.report.repairReason = "write the goal's metrics report; run it from " + f.root
				}
			}
			return code, true
		}
		if completion.reporter != nil {
			return reportAfterConfirmedDoneWithReporter(code, f.root, f.id, dependencies.noteWriter(), completion.reporter), true
		}
		return reportAfterConfirmedDone(code, f.root, f.id, dependencies.noteWriter()), true
	case "reopen":
		res, err := goal.Reopen(req, f.id)
		return dependencies.publish(res, err), true
	case "prune":
		res, err := goal.Prune(req, f.keep)
		return dependencies.publish(res, err), true
	case "set-next":
		if !need(f.id, "id") || !need(f.next, "next") {
			return 2, true
		}
		res, err := goal.Edit(req, f.id, goal.EditFields{NextStep: &f.next})
		return dependencies.publish(res, err), true
	case "promote":
		dependencies.complain("the synced backlog has no Current slot to promote into; claim the goal instead (goal claim --id ...)")
		return 1, true
	case "declare-free":
		if !need(f.digest, "digest") {
			return 2, true
		}
		res, err := goal.DeclareFree(req, f.origin, f.digest)
		return dependencies.publish(res, err), true
	case "reconcile":
		if f.refreshOnly {
			skipped, err := goal.RefreshOnly(f.root)
			if err != nil {
				dependencies.complain(err)
				return 1, true
			}
			printJSON(map[string]any{"outcome": "confirmed", "skipped": skipped})
			return 0, true
		}
		// Reconcile is the hand-edit path and always names its human, but
		// most of what it republishes needs no proof: an intent reworded, a
		// next step rewritten. So the proof is taken where it can be taken
		// and the session runs either way; the edits that do need one — a
		// blocker removed before it is done — ask for it themselves and name
		// the edge when it is missing.
		if proven, _, proofErr := provenGoalRequest("reconcile", f, humanauthority.ProveOrTemporaryGoalAuthority); proofErr == nil {
			req = proven
		}
		res, err := goal.Reconcile(req)
		if err != nil {
			dependencies.complain(err)
			return 1, true
		}
		printJSON(map[string]any{"outcome": res.Publish.Outcome, "tip": res.Publish.Tip, "rows": len(res.Rows), "skipped": res.Skipped})
		if res.Publish.Outcome != goal.OutcomeConfirmed && len(res.Rows) > 0 {
			return 1, true
		}
		return 0, true
	}
	dependencies.complainf("goal %s has no synced-world route\n", name)
	return 1, true
}

// runSyncOnly wraps the verbs that exist ONLY in the synced world.
func runSyncOnly(name string, run func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error), required ...string) func([]string) int {
	return runSyncOnlyWithRequest(name, run, nil, required...)
}

func runSyncOnlyWithRequest(name string, run func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error), requestBuilder func(string, string, string, string) (goal.VerbRequest, error), required ...string) func([]string) int {
	return runSyncOnlyWithDependencies(name, run, requestBuilder, defaultSyncRequestDependencies(), required...)
}

func runSyncOnlyWithDependencies(name string, run func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error), requestBuilder func(string, string, string, string) (goal.VerbRequest, error), dependencies syncRequestDependencies, required ...string) func([]string) int {
	return func(args []string) int {
		f, ok := dependencies.parseSyncFlags(name, args)
		if !ok {
			return 2
		}
		if name == "restamp" && f.by != "" {
			dependencies.complain("goal restamp acts only for the caller's own pair and does not accept --by")
			return 2
		}
		if !converted(f.root) {
			dependencies.complainf("goal %s works the synced backlog; this checkout still carries the legacy ledger\n", name)
			return 1
		}
		for _, r := range required {
			if r == "id" && f.id == "" {
				dependencies.complainf("goal %s needs --id\n", name)
				return 2
			}
			if r == "arc" && f.arc == "" {
				dependencies.complainf("goal %s needs --arc\n", name)
				return 2
			}
			if r == "pin" && f.pin == "" {
				dependencies.complainf("goal %s needs --pin (a machine nickname, or - to clear)\n", name)
				return 2
			}
			if r == "goal" && f.goal == "" {
				dependencies.complainf("goal %s needs --goal\n", name)
				return 2
			}
			if r == "by" && f.by == "" {
				dependencies.complainf("goal %s needs --by\n", name)
				return 2
			}
			if r == "why" && strings.TrimSpace(f.why) == "" {
				dependencies.complainf("goal %s needs --why\n", name)
				return 2
			}
		}
		builder := requestBuilder
		if builder == nil {
			builder = syncReq
			if name == "release" {
				builder = syncStoppingReq
			}
		}
		req, err := builder(name, f.root, f.by, f.lineage)
		if err != nil {
			dependencies.complain(err)
			return 1
		}
		req.ApprovedRef = f.approvedRef
		res, runErr := run(req, f)
		code := dependencies.publish(res, runErr)
		if name == "claim" {
			hintDesignIsARecord(dependencies.noteWriter(), res.Outcome, f.id)
		}
		return code
	}
}

func runGoalReleaseWithDependencies(args []string, requestBuilder func(string, string, string, string) (goal.VerbRequest, error), dependencies syncRequestDependencies) int {
	return runSyncOnlyWithDependencies("release", func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		if f.arc != "" {
			return goal.ReleaseArc(req, f.id)
		}
		return goal.Release(req, f.id)
	}, requestBuilder, dependencies, "id")(args)
}

func runGoalReleaseWithRequest(args []string, requestBuilder func(string, string, string, string) (goal.VerbRequest, error)) int {
	return runGoalReleaseWithDependencies(args, requestBuilder, defaultSyncRequestDependencies())
}

func runGoalTrunkRed(args []string) int {
	return runGoalTrunkRedWithRequest(args, nil, nil)
}

func runGoalTrunkRedWithRequest(args []string, requestBuilder func(string, string, string, string) (goal.VerbRequest, error), branchRead func(string, ...string) (string, error)) int {
	return runGoalTrunkRedWithDependencies(args, requestBuilder, branchRead, defaultSyncRequestDependencies())
}

func runGoalTrunkRedWithDependencies(args []string, requestBuilder func(string, string, string, string) (goal.VerbRequest, error), branchRead func(string, ...string) (string, error), dependencies syncRequestDependencies) int {
	if len(args) == 0 || args[0] != "own" && args[0] != "close" {
		dependencies.complain("goal trunk-red needs one of:\n  own --id <entry> --goal <fix-goal> [--branch <name>] [--by <human> [--to <machine>]]\n  close --id <entry> --by <human> --why <text>")
		return 2
	}
	sub := args[0]
	if sub == "own" {
		run := runSyncOnlyWithDependencies("trunk-red own", func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
			branchCommit := ""
			if f.branch != "" {
				commit, err := resolveTrunkRedBranchWithRead(req.Endpoint, f.branch, branchRead)
				if err != nil {
					return goal.PublishResult{}, err
				}
				branchCommit = commit
			}
			return goal.OwnTrunkRed(req, goal.TrunkRedOwnArgs{Entry: f.id, Goal: f.goal, Branch: f.branch,
				BranchCommit: branchCommit, To: f.to, By: f.by})
		}, requestBuilder, dependencies, "id", "goal")
		return run(args[1:])
	}
	run := runSyncOnlyWithDependencies("trunk-red close", func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		return goal.CloseTrunkRed(req, goal.TrunkRedCloseArgs{Entry: f.id, By: f.by, Why: strings.TrimSpace(f.why)})
	}, requestBuilder, dependencies, "id", "why")
	return run(args[1:])
}

func resolveTrunkRedBranchWithRead(endpoint goal.Endpoint, name string, read func(string, ...string) (string, error)) (string, error) {
	if read == nil {
		read = goalBranchGit
	}
	for _, ref := range []string{"refs/heads/" + name, "refs/remotes/" + endpoint.Remote + "/" + name} {
		if commit, err := read(endpoint.Root, "rev-parse", "--verify", "--quiet", ref); err == nil {
			return commit, nil
		}
	}
	return "", fmt.Errorf("branch %s is not in this checkout", name)
}

type goalAuthorityProver func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error)

func proveFixtureGoalAuthority(name string, f *syncFlags) (humanauthority.Proof, error) {
	return proveFixtureGoalAuthorityAt(name, f, goalCommandNow)
}

func proveFixtureGoalAuthorityAt(name string, f *syncFlags, commandNow func(string) (time.Time, error)) (humanauthority.Proof, error) {
	ancestryNow, err := commandNow(f.root)
	if err != nil {
		return humanauthority.Proof{}, err
	}
	authorization, err := fixtureauth.New(f.root)
	if err != nil {
		return humanauthority.Proof{}, err
	}
	proof, err := humanauthority.FixtureGoalProof(f.root, authorization.GoalHumanAuthority(), ancestryNow)
	if err != nil {
		return humanauthority.Proof{}, fmt.Errorf("goal %s could not prove fixture human authority: %w", name, err)
	}
	return proof, nil
}

func proveGoalHumanAuthority(name string, f *syncFlags, prove goalAuthorityProver) (humanauthority.Proof, error) {
	return proveGoalHumanAuthorityAt(name, f, prove, goalCommandNow)
}

func proveGoalHumanAuthorityAt(name string, f *syncFlags, prove goalAuthorityProver, commandNow func(string) (time.Time, error)) (humanauthority.Proof, error) {
	if f.fixtureHumanAuthority {
		if f.temporaryWord != "" || f.reviewBy != "" {
			return humanauthority.Proof{}, fmt.Errorf("goal %s fixture authority does not combine with a temporary human word or review date", name)
		}
		return proveFixtureGoalAuthorityAt(name, f, commandNow)
	}
	ancestryNow, err := commandNow(f.root)
	if err != nil {
		return humanauthority.Proof{}, err
	}
	proof, err := prove(f.root, int64(os.Getppid()), nil, f.temporaryWord, f.reviewBy, ancestryNow)
	if err != nil {
		if f.temporaryWord == "" && f.reviewBy == "" {
			return humanauthority.Proof{}, fmt.Errorf("goal %s could not prove enrolled human ancestry: %w", name, err)
		}
		return humanauthority.Proof{}, fmt.Errorf("goal %s could not bind its temporary recorded relay: %w", name, err)
	}
	return proof, nil
}

func proveEnrolledGoalHumanAuthority(root string, pid int64, reader humanauthority.Reader, _, _ string, now time.Time) (humanauthority.Proof, error) {
	return humanauthority.Prove(root, pid, reader, now)
}

// runGoalBlock writes one edge: goal block --id X --blocker G says X waits
// for G. It is not a human-only verb - a seat records the edge on the goal it
// holds, exactly as its open already does - so it takes the ordinary request
// and lets the engine judge the actor.
// claimsAHuman reports that this command is presenting itself as a person's
// act. A --by alone is exactly that presentation and not the act, so a
// command that claims one proves one: the engine reads the proof and never
// the name, and a name that could not be proved is refused here rather than
// being carried in as a seat with a person's word attached to it.
func claimsAHuman(f *syncFlags) bool {
	return f.by != "" || f.fixtureHumanAuthority || f.temporaryWord != ""
}

// provenGoalRequest assembles the request a goal verb runs under, with the
// human proof where the command claims a person and without one otherwise.
func provenGoalRequest(name string, f *syncFlags, prove goalAuthorityProver) (goal.VerbRequest, *humanauthority.Proof, error) {
	return provenGoalRequestWithInputs(name, f, prove, goalCommandNow, defaultSyncRequestDependencies())
}

func provenGoalRequestWithInputs(name string, f *syncFlags, prove goalAuthorityProver, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies) (goal.VerbRequest, *humanauthority.Proof, error) {
	if !claimsAHuman(f) {
		request, err := syncReqWithProofAtWithDependencies(name, f.root, f.by, f.lineage, nil, commandNow, dependencies)
		return request, nil, err
	}
	classification, err := classifyGoalAuthorityFirstWithFacts(name, f, dependencies.authorityFacts)
	if err != nil {
		return goal.VerbRequest{}, nil, err
	}
	proof, err := proveGoalHumanAuthorityAt(name, f, prove, commandNow)
	if err != nil {
		return goal.VerbRequest{}, nil, err
	}
	if err := resolveGoalHuman(f, proof); err != nil {
		return goal.VerbRequest{}, nil, err
	}
	request, err := syncReqClassifiedWithTerminalGradeAtWithDependencies(f.root, f.by, f.lineage, &proof, classification, false, commandNow, dependencies)
	return request, &proof, err
}

func runGoalBlock(args []string) int {
	return runGoalBlockWithAuthority(args, humanauthority.ProveOrTemporaryGoalAuthority)
}

// runGoalBlockWithAuthority writes one edge. It is not a human-only verb — a
// seat records an edge on the goal it holds, exactly as its open already does
// — but a --by beside it is a person's claim, and a person's claim is proved
// before the engine sees it.
func runGoalBlockWithAuthority(args []string, prove goalAuthorityProver) int {
	return runGoalBlockWithInputs(args, prove, goalCommandNow, defaultSyncRequestDependencies())
}

func runGoalBlockWithInputs(args []string, prove goalAuthorityProver, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies) int {
	f, ok := dependencies.parseSyncFlags("block", args)
	if !ok {
		return 2
	}
	if !converted(f.root) {
		dependencies.complain("goal block works the synced backlog; this checkout still carries the legacy ledger")
		return 1
	}
	if f.id == "" || f.blocker == "" {
		dependencies.complain("goal block needs --id <the goal that waits> and --blocker <the goal it waits for>")
		return 2
	}
	req, proof, err := provenGoalRequestWithInputs("block", f, prove, commandNow, dependencies)
	if err != nil {
		dependencies.complain(err)
		return 1
	}
	res, runErr := goal.Block(req, f.id, f.blocker, proof)
	dependencies.landed(res)
	if runErr == nil && res.Outcome == goal.OutcomeConfirmed && proof != nil {
		operation := goal.Opid(req.Ulid, req.Actor.Machine, req.Actor.Lineage)
		if recordErr := recordGoalApprovalProof(f.root, operation, "goal block", *proof); recordErr != nil {
			dependencies.complain("the act landed at tip " + res.Tip + ", but its authority proof did not: " + recordErr.Error() + "; do not run it again")
			return 1
		}
	}
	return dependencies.publish(res, runErr)
}

func runGoalUnblock(args []string) int {
	return runGoalUnblockWithAuthority(args, humanauthority.ProveOrTemporaryGoalAuthority)
}

// runGoalUnblockWithAuthority removes one edge. Removing an edge whose
// blocker is not done is an early lift and a person's act, so a --by is
// proved before the verb runs and the proof travels with it; without one the
// engine refuses the early case by name and takes the ordinary one, which is
// an edge a completed goal no longer needs.
func runGoalUnblockWithAuthority(args []string, prove goalAuthorityProver) int {
	return runGoalUnblockWithInputs(args, prove, goalCommandNow, defaultSyncRequestDependencies())
}

func runGoalUnblockWithInputs(args []string, prove goalAuthorityProver, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies) int {
	f, ok := dependencies.parseSyncFlags("unblock", args)
	if !ok {
		return 2
	}
	if !converted(f.root) {
		dependencies.complain("goal unblock works the synced backlog; this checkout still carries the legacy ledger")
		return 1
	}
	if f.id == "" || f.blocker == "" {
		dependencies.complain("goal unblock needs --id <the goal that waits> and --blocker <the goal it no longer waits for>")
		return 2
	}
	req, proof, err := provenGoalRequestWithInputs("unblock", f, prove, commandNow, dependencies)
	if err != nil {
		dependencies.complain(err)
		return 1
	}
	res, runErr := goal.Unblock(req, f.id, f.blocker, proof)
	dependencies.landed(res)
	if runErr == nil && res.Outcome == goal.OutcomeConfirmed && proof != nil {
		operation := goal.Opid(req.Ulid, req.Actor.Machine, req.Actor.Lineage)
		if recordErr := recordGoalApprovalProof(f.root, operation, "goal unblock", *proof); recordErr != nil {
			dependencies.complain("the act landed at tip " + res.Tip + ", but its authority proof did not: " + recordErr.Error() + "; do not run it again")
			return 1
		}
	}
	return dependencies.publish(res, runErr)
}

func runGoalSetPriority(args []string) int {
	return runGoalSetPriorityWithAuthority(args, proveEnrolledGoalHumanAuthority)
}

func configureAbandonFleetFloor(request *goal.VerbRequest) {
	request.ConfigureAbandon(
		func() string { return supervise.BuildStamp },
		func(root, ancestor, descendant string) (bool, error) {
			return goal.IsAncestor(root, ancestor, descendant)
		},
		func(floor string, isAncestor func(a, b string) (bool, error), now time.Time) ([]string, error) {
			return supervise.EngineFloorProblems(floor, isAncestor, identity.KernelProber{}, now)
		},
	)
}

func runGoalAbandon(args []string) int {
	return runGoalAbandonWithInputs(args, proveEnrolledGoalHumanAuthority, goalCommandNow, defaultSyncRequestDependencies())
}

func runGoalAbandonWithInputs(args []string, prove goalAuthorityProver, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies) int {
	flags := flag.NewFlagSet("goal abandon", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	id := flags.String("id", "", "goal id")
	by := flags.String("by", "", "the directing human")
	because := flags.String("because", "", "one-line reason the goal will not be worked")
	carried := flags.String("carried", "", "live successor goal")
	lineage := flags.String("lineage", "", "this coordinator's lineage")
	fixtureAuthority := flags.Bool("fixture-human-authority", false, "fixture-only enrolled-human proof; accepted only for an exact fake-runtime root")
	var waive, also repeatedStrings
	flags.Var(&waive, "waive", "dependent=reason (repeatable)")
	flags.Var(&also, "also", "live dependent to abandon in the same transaction (repeatable)")
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 {
		dependencies.complain("goal abandon takes no positional arguments")
		return 2
	}
	if !converted(*root) {
		dependencies.complain("goal abandon works only with the synced backlog; migrate this checkout first")
		return 1
	}
	shared := &syncFlags{root: *root, id: *id, by: *by, lineage: *lineage, fixtureHumanAuthority: *fixtureAuthority}
	proof, err := proveGoalHumanAuthorityAt("abandon", shared, prove, commandNow)
	if err != nil {
		dependencies.complain(err)
		return 1
	}
	request, err := syncReqWithProofAtWithDependencies("abandon", *root, *by, *lineage, &proof, commandNow, dependencies)
	if err != nil {
		dependencies.complain(err)
		return 1
	}
	configureAbandonFleetFloor(&request)
	result, err := goal.Abandon(request, *id, goal.AbandonSpec{Because: *because, Carried: *carried, Waive: waive, Also: also}, &proof)
	return dependencies.publish(result, err)
}

func runGoalCarryAbandoned(args []string) int {
	return runGoalCarryAbandonedWithInputs(args, proveEnrolledGoalHumanAuthority, goalCommandNow, defaultSyncRequestDependencies())
}

func runGoalCarryAbandonedWithInputs(args []string, prove goalAuthorityProver, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies) int {
	flags := flag.NewFlagSet("goal carry", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	id := flags.String("id", "", "abandoned goal id")
	successor := flags.String("to", "", "live successor goal")
	by := flags.String("by", "", "the directing human")
	lineage := flags.String("lineage", "", "this coordinator's lineage")
	fixtureAuthority := flags.Bool("fixture-human-authority", false, "fixture-only enrolled-human proof; accepted only for an exact fake-runtime root")
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 {
		dependencies.complain("goal carry takes no positional arguments")
		return 2
	}
	if !converted(*root) {
		dependencies.complain("goal carry works only with the synced backlog; migrate this checkout first")
		return 1
	}
	shared := &syncFlags{root: *root, id: *id, by: *by, lineage: *lineage, fixtureHumanAuthority: *fixtureAuthority}
	proof, err := proveGoalHumanAuthorityAt("carry", shared, prove, commandNow)
	if err != nil {
		dependencies.complain(err)
		return 1
	}
	request, err := syncReqWithProofAtWithDependencies("carry", *root, *by, *lineage, &proof, commandNow, dependencies)
	if err != nil {
		dependencies.complain(err)
		return 1
	}
	result, err := goal.CarryAbandoned(request, *id, *successor, &proof)
	return dependencies.publish(result, err)
}

func runGoalEngineFloor(args []string) int {
	flags := flag.NewFlagSet("goal engine-floor", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	commit := flags.String("commit", "", "40-character lowercase engine commit")
	by := flags.String("by", "", "the directing human")
	lineage := flags.String("lineage", "", "this coordinator's lineage")
	fixtureAuthority := flags.Bool("fixture-human-authority", false, "fixture-only enrolled-human proof; accepted only for an exact fake-runtime root")
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "goal engine-floor takes no positional arguments")
		return 2
	}
	if !converted(*root) {
		fmt.Fprintln(os.Stderr, "goal engine-floor works only with the synced backlog; migrate this checkout first")
		return 1
	}
	shared := &syncFlags{root: *root, by: *by, lineage: *lineage, fixtureHumanAuthority: *fixtureAuthority}
	proof, err := proveGoalHumanAuthority("engine-floor", shared, proveEnrolledGoalHumanAuthority)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	request, err := syncReqWithProof("engine-floor", *root, *by, *lineage, &proof)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	result, err := goal.EngineFloor(request, *commit, &proof)
	return printSyncResult(result, err)
}

func runGoalSetPriorityWithAuthority(args []string, prove goalAuthorityProver) int {
	return runGoalSetPriorityWithAuthorityAndInputs(args, prove, defaultSyncRequestDependencies())
}

func runGoalSetPriorityWithAuthorityAndInputs(args []string, prove goalAuthorityProver, dependencies syncRequestDependencies) int {
	flags := flag.NewFlagSet("goal set-priority", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	id := flags.String("id", "", "goal id")
	by := flags.String("by", "", "the directing human")
	lineage := flags.String("lineage", "", "this coordinator's lineage")
	fixtureAuthority := flags.Bool("fixture-human-authority", false, "fixture-only enrolled-human proof; accepted only for an exact fake-runtime root")
	priorityRaw, sequenceRaw := "", ""
	prioritySet, sequenceSet := false, false
	flags.Func("priority", "priority 1, 2, or 3", func(value string) error {
		prioritySet = true
		priorityRaw = value
		return nil
	})
	flags.Func("sequence", "one-based position within the priority", func(value string) error {
		sequenceSet = true
		sequenceRaw = value
		return nil
	})
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 {
		dependencies.complain("goal set-priority takes no positional arguments")
		return 2
	}
	if *id == "" || *by == "" || !prioritySet {
		dependencies.complain("goal set-priority needs --id, --by, and --priority")
		return 2
	}
	priorityValue, err := parseGoalRankDecimal("priority", priorityRaw, 8)
	if err != nil || priorityValue < 1 || priorityValue > 3 {
		dependencies.complainf("goal set-priority priority %q is not 1, 2, or 3\n", priorityRaw)
		return 2
	}
	var sequence *uint64
	if sequenceSet {
		sequenceValue, parseErr := parseGoalRankDecimal("sequence", sequenceRaw, 64)
		if parseErr != nil || sequenceValue == 0 {
			dependencies.complainf("goal set-priority sequence %q is not a positive unsigned 64-bit integer\n", sequenceRaw)
			return 2
		}
		sequence = &sequenceValue
	}
	if !converted(*root) {
		dependencies.complain("goal set-priority works only with the synced backlog; migrate this checkout first")
		return 1
	}
	shared := &syncFlags{root: *root, id: *id, by: *by, lineage: *lineage, fixtureHumanAuthority: *fixtureAuthority}
	proof, err := proveGoalHumanAuthority("set-priority", shared, prove)
	if err != nil {
		dependencies.complain(err)
		return 1
	}
	request, err := syncReqWithProofAtWithDependencies("set-priority", *root, *by, *lineage, &proof, goalCommandNow, dependencies)
	if err != nil {
		dependencies.complain(err)
		return 1
	}
	result, err := goal.SetPriority(request, *id, uint8(priorityValue), sequence, &proof)
	return dependencies.publish(result, err)
}

func parseGoalRankDecimal(name, value string, bitSize int) (uint64, error) {
	if value == "" {
		return 0, fmt.Errorf("%s is empty", name)
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return 0, fmt.Errorf("%s contains a non-decimal character", name)
		}
	}
	return strconv.ParseUint(value, 10, bitSize)
}

// recordGoalApprovalProof keeps the proof file's action bound to the exact
// human verb. The authority package's generic writer accepts proven ancestry;
// the relayed form is validated through the same authorization predicate the
// core consumed and then serialized with the same closed envelope.
func recordGoalApprovalProof(root, operationID, action string, proof humanauthority.Proof) error {
	if proof.ValidFor(root) {
		return humanauthority.RecordProof(root, operationID, action, proof)
	}
	if !proof.AuthorizesResume(root) || !proof.TemporaryResumeFor(root) || operationID == "" || filepath.Base(operationID) != operationID {
		return fmt.Errorf("cannot record an incomplete human authority proof")
	}
	record := struct {
		Schema      int                  `json:"schema"`
		OperationID string               `json:"operationId"`
		Action      string               `json:"action"`
		Proof       humanauthority.Proof `json:"proof"`
	}{Schema: 1, OperationID: operationID, Action: action, Proof: proof}
	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(root, "artifacts", "agents", "authority", "proofs", operationID+".json")
	durable, err := atomicfile.WriteText(path, string(encoded)+"\n", root)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("human authority proof was written but its durability is unknown")
	}
	return nil
}

func runGoalApprove(args []string) int {
	return runGoalApproveWithAuthority(args, humanauthority.ProveOrTemporaryGoalAuthority)
}

func runGoalClassifySweep(args []string) int {
	return runGoalClassifySweepWithAuthority(args, humanauthority.ProveOrTemporaryGoalAuthority)
}

func runGoalClassifySweepWithAuthority(args []string, prove goalAuthorityProver) int {
	return runGoalClassifySweepWithInputs(args, prove, goalCommandNow, defaultSyncRequestDependencies())
}

func runGoalClassifySweepWithInputs(args []string, prove goalAuthorityProver, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies) int {
	values := newHumanVerbValues("classify-sweep", args)
	fs := flag.NewFlagSet("goal classify-sweep", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := pathFlag(fs, "root", ".", "checkout root")
	draftPath := fs.String("draft", "", "classification draft file")
	preview := fs.Bool("preview", false, "print the normalized listing without mutation")
	confirm := fs.String("confirm", "", "sha256 of the normalized preview listing")
	by := fs.String("by", "", "the directing human")
	lineage := fs.String("lineage", "", "this coordinator's lineage")
	fixtureHumanAuthority := fs.Bool("fixture-human-authority", false, "fixture-only enrolled-human proof; accepted only for an exact fake-runtime root")
	if err := fs.Parse(args); err != nil {
		return refuseHumanVerb(values, 2, err.Error(), humanVerbRemedy{words: "remove the unrecognized flag and retry with the sweep fields"})
	}
	values.root, values.lineage, values.by = *root, *lineage, *by
	values.fixtureHumanAuthority = *fixtureHumanAuthority
	if !converted(*root) {
		return refuseHumanVerb(values, 2, "needs a synced backlog", humanVerbRemedy{words: "migrate this checkout to the synced backlog first"})
	}
	if *draftPath == "" {
		return refuseHumanVerb(values, 2, "needs --draft", humanVerbRemedy{words: "provide the classification draft path before choosing --preview or --confirm"})
	}
	if *preview == (*confirm != "") {
		return refuseHumanVerb(values, 2, "needs a synced backlog, --draft, and exactly one of --preview or --confirm", humanVerbRemedy{command: shellCommand([]string{"metasystem", "goal", "classify-sweep", "--root", *root, "--draft", *draftPath, "--preview"})})
	}
	draft, err := os.ReadFile(*draftPath)
	if err != nil {
		return refuseHumanVerb(values, 1, "could not read its draft: "+err.Error(), humanVerbRemedy{words: "repair the draft path before previewing again"})
	}
	endpoint, err := dependencies.endpoint(*root)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "repair the checkout's goal synchronization endpoint"})
	}
	now, err := commandNow(*root)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "provide a valid goal command clock"})
	}
	listing, err := goal.PreviewClassificationSweep(endpoint, draft, now)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{command: shellCommand([]string{"metasystem", "goal", "classify-sweep", "--root", *root, "--draft", *draftPath, "--preview"})})
	}
	if *preview {
		for _, line := range listing.Lines {
			fmt.Println(line)
		}
		fmt.Println("listing-digest " + listing.Digest)
		return 0
	}
	if listing.Digest != *confirm {
		return refuseHumanVerb(values, 1, fmt.Sprintf("SWEEP_LISTING_CHANGED: confirmation %s does not match current listing %s", *confirm, listing.Digest), humanVerbRemedy{command: shellCommand([]string{"metasystem", "goal", "classify-sweep", "--root", *root, "--draft", *draftPath, "--preview"})})
	}
	authorityFlags := &syncFlags{root: *root, by: *by, lineage: *lineage, fixtureHumanAuthority: *fixtureHumanAuthority}
	if _, err := classifyGoalAuthorityFirstWithFacts("classify-sweep", authorityFlags, dependencies.authorityFacts); err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "run this at the enrolled terminal"})
	}
	proof, err := proveGoalHumanAuthorityAt("classify-sweep", authorityFlags, prove, commandNow)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "run confirmation at the enrolled terminal"})
	}
	if err := resolveGoalHuman(authorityFlags, proof); err != nil {
		return refuseHumanVerb(values, 2, err.Error(), humanVerbRemedy{words: "re-enroll with goal enroll-terminal --by <your name>, or add --by <your name> to this command"})
	}
	*by = authorityFlags.by
	values.by = *by
	if len(listing.Proposals) == 0 {
		if listing.TierLawInstalled {
			printJSON(map[string]any{"outcome": goal.OutcomeConfirmed, "detail": "the tier law is already installed and no tierless goals remain"})
			return 0
		}
		req, reqErr := syncReqWithProofAtWithDependencies("classify-sweep", *root, *by, *lineage, &proof, commandNow, dependencies)
		if reqErr != nil {
			return refuseHumanVerb(values, 1, reqErr.Error(), humanVerbRemedy{words: "repair the named checkout identity fact before retrying"})
		}
		res, installErr := goal.InstallTierLaw(req)
		if installErr != nil || res.Outcome != goal.OutcomeConfirmed {
			detail := res.Detail
			if installErr != nil {
				detail = installErr.Error()
			}
			return refuseHumanVerb(values, 1, detail, humanVerbRemedy{command: shellCommand([]string{"metasystem", "goal", "classify-sweep", "--root", *root, "--draft", *draftPath, "--preview"})})
		}
		if proofErr := recordGoalApprovalProof(*root, goal.Opid(req.Ulid, req.Actor.Machine, req.Actor.Lineage), "goal classify-sweep", proof); proofErr != nil {
			return refuseHumanVerb(values, 1, "the act landed at tip "+res.Tip+", but its authority proof did not: "+proofErr.Error(), humanVerbRemedy{words: "the act already landed; do not run it again"})
		}
		printJSON(map[string]any{"outcome": goal.OutcomeConfirmed, "classified": 0, "listingDigest": listing.Digest})
		return 0
	}
	for index, proposal := range listing.Proposals {
		req, reqErr := syncReqWithProofAtWithDependencies("classify-sweep", *root, *by, *lineage, &proof, commandNow, dependencies)
		if reqErr != nil {
			return refuseHumanVerb(values, 1, reqErr.Error(), humanVerbRemedy{words: "repair the named checkout identity fact before retrying"})
		}
		res, classifyErr := goal.ClassifyTier(req, proposal, index == len(listing.Proposals)-1)
		if classifyErr != nil {
			return refuseHumanVerb(values, 1, classifyErr.Error(), humanVerbRemedy{command: shellCommand([]string{"metasystem", "goal", "classify-sweep", "--root", *root, "--draft", *draftPath, "--preview"})})
		}
		if res.Outcome != goal.OutcomeConfirmed {
			printJSON(map[string]any{"outcome": res.Outcome, "tip": res.Tip, "detail": res.Detail})
			return refuseHumanVerb(values, 1, res.Detail, humanVerbRemedy{command: shellCommand([]string{"metasystem", "goal", "classify-sweep", "--root", *root, "--draft", *draftPath, "--preview"})})
		}
		if proofErr := recordGoalApprovalProof(*root, goal.Opid(req.Ulid, req.Actor.Machine, req.Actor.Lineage), "goal classify-sweep", proof); proofErr != nil {
			return refuseHumanVerb(values, 1, "the act landed at tip "+res.Tip+", but its authority proof did not: "+proofErr.Error(), humanVerbRemedy{words: "the act already landed; do not run it again"})
		}
	}
	printJSON(map[string]any{"outcome": goal.OutcomeConfirmed, "classified": len(listing.Proposals), "listingDigest": listing.Digest})
	return 0
}

func runGoalApproveWithAuthority(args []string, prove goalAuthorityProver) int {
	return runGoalApproveWithInputs(args, prove, goalCommandNow, defaultSyncRequestDependencies(), dispatchcore.ResolveGoalBinding)
}

func runGoalApproveWithInputs(args []string, prove goalAuthorityProver, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies, binding goalBindingResolver) int {
	values := newHumanVerbValues("approve", args)
	values.report = dependencies.report
	f, ok := parseHumanSyncFlags(values, "approve", args)
	if !ok {
		return 2
	}
	if !converted(f.root) {
		return refuseHumanVerb(values, 1, "works only with the synced backlog", humanVerbRemedy{words: "migrate this checkout to the synced backlog first"})
	}
	if f.sweep && len(f.ids) != 0 || !f.sweep && f.confirm != "" {
		return refuseHumanVerb(values, 2, "uses either repeatable --id or --sweep; --confirm belongs only to the sweep", humanVerbRemedy{command: values.sameCommandWithout("sweep", "confirm")})
	}
	if f.sweep && f.hasAnyBudgetFlag() || f.sweep && f.budgetBox != "" || f.sweep && f.approvedRef != "" {
		return refuseHumanVerb(values, 2, "--sweep uses the tuples already listed and takes no budget or --approved-ref", humanVerbRemedy{command: values.sameCommandWithout("budget", "elapsed-limit", "attempt-limit", "reserved-job-minutes-limit", "active-job-limit", "review-round-limit", "approved-ref")})
	}
	if f.sweep && f.confirm == "" {
		e, err := dependencies.endpoint(f.root)
		if err != nil {
			return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "repair the checkout's goal synchronization endpoint"})
		}
		now, err := commandNow(f.root)
		if err != nil {
			return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "provide a valid goal command clock"})
		}
		listing, err := goal.PreviewApprovalSweep(e, now)
		if err != nil {
			return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "repair the approval sweep listing"})
		}
		for _, line := range listing.Lines {
			fmt.Println(line)
		}
		fmt.Println("listing-sha256=" + listing.Digest)
		if len(listing.Skipped) > 0 {
			fmt.Println("without-budget=" + strings.Join(listing.Skipped, ","))
		}
		return 0
	}
	if f.under != "" {
		if f.sweep || len(f.ids) == 0 {
			return dependencies.fail(2, fmt.Errorf("goal approve --under takes repeatable --id and no --sweep"))
		}
		return runGoalUnderAttorneyWithInputs("approve", f, commandNow, dependencies)
	}
	if !f.sweep && len(f.ids) == 0 {
		return refuseHumanVerb(values, 2, "needs either repeatable --id or --sweep", humanVerbRemedy{words: "add the live goal's identifier with --id, or choose --sweep"})
	}
	if !f.sweep && len(f.ids) == 1 && (f.budgetBox != "" || f.hasAnyBudgetFlag()) {
		f.id = f.ids[0]
		values.id = f.id
		routeBox := ""
		hintBox := ""
		if f.budgetBox != "" {
			if f.budgetBox != "box" {
				return refuseHumanVerb(values, 2, "--budget is box", humanVerbRemedy{command: values.budgetCommand("norm")})
			}
			routeBox = "norm"
			hintBox = "norm"
		} else if parsed, budgetErr := f.budgetTuple(true); budgetErr == nil {
			hintBox = goalbudget.FormatBox(*parsed)
		}
		dependencies.note(false, "hint: "+values.budgetCommand(hintBox))
		return runGoalBudgetPreparedWithInputs(values, f, routeBox, prove, commandNow, dependencies, binding)
	}
	budget, err := f.approvalBudget()
	if err != nil {
		return refuseHumanVerb(values, 2, err.Error(), humanVerbRemedy{words: "give each goal its box with goal budget"})
	}
	classification, err := classifyGoalAuthorityFirstWithFacts("approve", f, dependencies.authorityFacts)
	if err != nil {
		return dependencies.fail(1, err)
	}
	proof, err := proveGoalHumanAuthorityAt("approve", f, prove, commandNow)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanProofRemedy(values, f.fixtureHumanAuthority, f.temporaryWord, f.reviewBy))
	}
	if err := resolveGoalHuman(f, proof); err != nil {
		return refuseHumanVerb(values, 2, err.Error(), humanVerbRemedy{words: "re-enroll with goal enroll-terminal --by <your name>, or add --by <your name> to this command"})
	}
	values.by = f.by
	req, err := syncReqClassifiedWithTerminalGradeAtWithDependencies(f.root, f.by, f.lineage, &proof, classification, false, commandNow, dependencies)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "repair the named checkout identity fact before retrying"})
	}
	req.ApprovedRef = f.approvedRef
	var res goal.PublishResult
	if f.sweep {
		res, err = goal.ApproveSweep(req, f.confirm, &proof)
	} else {
		res, err = goal.Approve(req, f.ids, budget, &proof)
	}
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "read the current goal state and give each live goal its box with goal budget"})
	}
	if res.Outcome == goal.OutcomeConfirmed {
		dependencies.landed(res)
		action := "goal approve"
		if f.sweep {
			action = "goal approve --sweep"
		}
		if err := recordGoalApprovalProof(f.root, goal.Opid(req.Ulid, req.Actor.Machine, req.Actor.Lineage), action, proof); err != nil {
			return refuseHumanVerb(values, 1, "the act landed at tip "+res.Tip+", but its authority proof did not: "+err.Error(), humanVerbRemedy{words: "the act already landed; do not run it again"})
		}
		if proof.TemporaryResumeFor(f.root) {
			dependencies.note(true, fmt.Sprintf("goal approve: TEMPORARY authority under a recorded relayed word (human provenance not verified); re-approval due %s at an agent-free terminal", f.reviewBy))
		}
	}
	if res.Outcome != goal.OutcomeConfirmed {
		dependencies.showOutcome(res)
		if res.Outcome == goal.OutcomeAbandoned {
			return refuseHumanVerb(values, 1, res.Detail, humanVerbRemedy{words: "every target already has the same proven approval, so there is no new act to record"})
		}
		return refuseHumanVerb(values, 1, res.Detail, humanVerbRemedy{words: "read the current goal state and give each live goal its box with goal budget"})
	}
	return dependencies.publish(res, nil)
}

func runGoalUnapprove(args []string) int {
	return runGoalUnapproveWithAuthority(args, humanauthority.ProveOrTemporaryGoalAuthority)
}

func runGoalUnapproveWithAuthority(args []string, prove goalAuthorityProver) int {
	return runGoalUnapproveWithInputs(args, prove, goalCommandNow, defaultSyncRequestDependencies())
}

func runGoalUnapproveWithInputs(args []string, prove goalAuthorityProver, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies) int {
	values := newHumanVerbValues("unapprove", args)
	values.report = dependencies.report
	f, ok := parseHumanSyncFlags(values, "unapprove", args)
	if !ok {
		return 2
	}
	if !converted(f.root) {
		return refuseHumanVerb(values, 1, "works only with the synced backlog", humanVerbRemedy{words: "migrate this checkout to the synced backlog first"})
	}
	if f.id == "" || f.because == "" {
		return refuseHumanVerb(values, 2, "needs --id and --because", humanVerbRemedy{words: "add the missing goal identifier or reason, whose value was not supplied"})
	}
	classification, err := classifyGoalAuthorityFirstWithFacts("unapprove", f, dependencies.authorityFacts)
	if err != nil {
		dependencies.complain(err)
		return 1
	}
	proof, err := proveGoalHumanAuthorityAt("unapprove", f, prove, commandNow)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanProofRemedy(values, f.fixtureHumanAuthority, f.temporaryWord, f.reviewBy))
	}
	if err := resolveGoalHuman(f, proof); err != nil {
		return refuseHumanVerb(values, 2, err.Error(), humanVerbRemedy{words: "re-enroll with goal enroll-terminal --by <your name>, or add --by <your name> to this command"})
	}
	values.by = f.by
	req, err := syncReqClassifiedWithTerminalGradeAtWithDependencies(f.root, f.by, f.lineage, &proof, classification, false, commandNow, dependencies)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "repair the named checkout identity fact before retrying"})
	}
	res, err := goal.Unapprove(req, f.id, f.because, &proof)
	dependencies.landed(res)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "read the goal's current approval state before retrying"})
	}
	if res.Outcome == goal.OutcomeConfirmed {
		if err := recordGoalApprovalProof(f.root, goal.Opid(req.Ulid, req.Actor.Machine, req.Actor.Lineage), "goal unapprove", proof); err != nil {
			return refuseHumanVerb(values, 1, "the act landed at tip "+res.Tip+", but its authority proof did not: "+err.Error(), humanVerbRemedy{words: "the act already landed; do not run it again"})
		}
		if proof.TemporaryResumeFor(f.root) {
			dependencies.note(true, fmt.Sprintf("goal unapprove: TEMPORARY authority under a recorded relayed word (human provenance not verified); re-approval due %s at an agent-free terminal", f.reviewBy))
		}
	}
	if res.Outcome != goal.OutcomeConfirmed {
		dependencies.outcomeBeforeRefusal(res)
		return refuseHumanVerb(values, 1, res.Detail, humanVerbRemedy{words: "read the goal's current approval state before retrying"})
	}
	return dependencies.publish(res, nil)
}

func runGoalSetBudget(args []string) int {
	return runGoalSetBudgetWithAuthority(args, humanauthority.ProveOrTemporaryGoalAuthority)
}

func runGoalExtendBudget(args []string) int {
	return runGoalExtendBudgetWithInputs(args, goalCommandNow, defaultSyncRequestDependencies(), dispatchcore.ConcreteProofAdmissionReads())
}

func runGoalExtendBudgetWithInputs(args []string, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies, reads dispatchcore.ProofAdmissionReads) int {
	flags := flag.NewFlagSet("goal extend-budget", flag.ContinueOnError)
	root := pathFlag(flags, "root", ".", "checkout root")
	id := flags.String("id", "", "goal id")
	revision := flags.Uint64("revision", 0, "exact accepted goal revision")
	proposedCap := flags.Uint64("proposed-cap", 0, "reserved minutes proposed by this dispatch")
	role := flags.String("role", "", "role proposed by this dispatch")
	dispatchMode := flags.String("dispatch-mode", "", "fresh or follow-up")
	destructiveReach := flags.String("destructive-reach", "", "MECHANICAL, DESIGN-BEARING, or DESTRUCTIVE-REACH")
	lineage := flags.String("lineage", "", "this coordinator's lineage (or export METASYSTEM_OWNER_LINEAGE)")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *id == "" || *revision == 0 || *proposedCap == 0 || *role == "" || *dispatchMode == "" || *destructiveReach == "" {
		fmt.Fprintln(os.Stderr, "goal extend-budget needs --id, --revision, a positive --proposed-cap, --role, --dispatch-mode, and --destructive-reach")
		return 2
	}
	if !converted(*root) {
		fmt.Fprintln(os.Stderr, "goal extend-budget works the synced backlog; this checkout still carries the legacy ledger")
		return 1
	}
	req, err := syncReqWithProofAtWithDependencies("extend-budget", *root, "", *lineage, nil, commandNow, dependencies)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	held, err := goalrevision.Acquire(*root, *id, *revision, "goal-extend-budget")
	if err != nil {
		fmt.Fprintln(os.Stderr, "goal extend-budget could not acquire the goal-revision lock:", err)
		return 1
	}
	defer held.Release()
	verdict, err := dispatchcore.EvaluateGoalRevisionAdmissionForDispatchWithReads(*root, *id, *revision, *proposedCap, req.Now, *role, *dispatchMode, reads, dispatchcore.HazardClass(*destructiveReach))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !verdict.Refused() {
		fmt.Fprintf(os.Stderr, "goal %s revision %d is admitted; there is no budget refusal to extend\n", *id, *revision)
		return 1
	}
	if verdict.PolicyRefusal != "" {
		fmt.Fprintln(os.Stderr, verdict.PolicyRefusal)
		return 1
	}
	if verdict.LiveStopReason != "" || verdict.Extension == nil {
		for _, line := range dispatchcore.FormatGoalRevisionAdmission(verdict) {
			fmt.Fprintln(os.Stderr, line)
		}
		if verdict.LiveStopReason != "" {
			fmt.Fprintf(os.Stderr, "goal %s revision %d names a live stop, not an extendable exhaustion\n", *id, *revision)
		} else {
			fmt.Fprintf(os.Stderr, "goal %s revision %d has no consumption-earned budget extension offer\n", *id, *revision)
		}
		return 1
	}
	if err := verdict.Extension.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	res, err := goal.ExtendBudget(req, *id, verdict.Extension.GoalOffer())
	return printSyncResult(res, err)
}

func runGoalSetBudgetWithAuthority(args []string, prove goalAuthorityProver) int {
	return runGoalSetBudgetWithInputs(args, prove, goalCommandNow, defaultSyncRequestDependencies(), dispatchcore.ResolveGoalBinding)
}

func runGoalSetBudgetWithInputs(args []string, prove goalAuthorityProver, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies, binding goalBindingResolver) int {
	values := newHumanVerbValues("set-budget", args)
	f, ok := parseHumanSyncFlags(values, "set-budget", args)
	if !ok {
		return 2
	}
	if f.under != "" {
		if !converted(f.root) || f.id == "" {
			fmt.Fprintln(os.Stderr, "goal set-budget --under needs a synced backlog plus --id")
			return 2
		}
		return runGoalUnderAttorneyWithInputs("set-budget", f, commandNow, dependencies)
	}
	box := ""
	if parsed, err := f.budgetTuple(true); err == nil {
		box = goalbudget.FormatBox(*parsed)
	}
	fmt.Fprintln(os.Stderr, "hint:", values.budgetCommand(box))
	if box != "" && stoppedGoalForSetBudgetWithInputs(f, commandNow, dependencies.endpoint) {
		return runGoalStoppedSetBudgetWithInputs(values, f, prove, commandNow, dependencies, binding)
	}
	return runGoalBudgetPreparedWithInputs(values, f, "", prove, commandNow, dependencies, binding)
}

func stoppedGoalForSetBudgetWithInputs(flags *syncFlags, commandNow func(string) (time.Time, error), resolveEndpoint func(string) (goal.Endpoint, error)) bool {
	if !converted(flags.root) || flags.id == "" {
		return false
	}
	if resolveEndpoint == nil || commandNow == nil {
		return false
	}
	endpoint, err := resolveEndpoint(flags.root)
	if err != nil {
		return false
	}
	now, err := commandNow(flags.root)
	if err != nil {
		return false
	}
	projection, err := goal.Project(endpoint, false, now)
	if err != nil {
		return false
	}
	file := projection.Tree.Live[flags.id]
	return file != nil && file.StopFence != nil
}

func runGoalStoppedSetBudgetWithInputs(values *humanVerbValues, flags *syncFlags, prove goalAuthorityProver, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies, binding goalBindingResolver) int {
	budget, err := flags.budgetTuple(true)
	if err != nil {
		return runGoalBudgetPreparedWithInputs(values, flags, "", prove, commandNow, dependencies, binding)
	}
	values.box = budget
	classification, err := classifyGoalAuthorityFirstWithFacts("set-budget", flags, dependencies.authorityFacts)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "repair the goal authority classification before retrying"})
	}
	proof, err := proveGoalHumanAuthorityAt("set-budget", flags, prove, commandNow)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanProofRemedy(values, flags.fixtureHumanAuthority, flags.temporaryWord, flags.reviewBy))
	}
	if err := resolveGoalHuman(flags, proof); err != nil {
		return refuseHumanVerb(values, 2, err.Error(), humanVerbRemedy{words: "re-enroll with goal enroll-terminal --by <your name>, or add --by <your name> to this command"})
	}
	values.by = flags.by
	req, err := syncReqClassifiedWithTerminalGradeAtWithDependencies(flags.root, flags.by, flags.lineage, &proof, classification, false, commandNow, dependencies)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "repair the named checkout identity fact before retrying"})
	}
	req.ApprovedRef = flags.approvedRef
	result, err := goal.SetBudgetApproved(req, flags.id, *budget, &proof)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{command: values.budgetCommandWithoutApprovedRef("keep")})
	}
	if result.Outcome != goal.OutcomeConfirmed {
		printJSON(map[string]any{"outcome": result.Outcome, "tip": result.Tip, "detail": result.Detail})
		return refuseHumanVerb(values, 1, result.Detail, humanVerbRemedy{command: values.budgetCommandWithoutApprovedRef("keep")})
	}
	if err := recordGoalApprovalProof(flags.root, goal.Opid(req.Ulid, req.Actor.Machine, req.Actor.Lineage), "goal set-budget", proof); err != nil {
		return refuseHumanVerb(values, 1, "the act landed at tip "+result.Tip+", but its authority proof did not: "+err.Error(), humanVerbRemedy{words: "the act already landed; do not run it again"})
	}
	if proof.TemporaryResumeFor(flags.root) {
		fmt.Printf("goal set-budget: TEMPORARY authority under a recorded relayed word (human provenance not verified); re-approval due %s at an agent-free terminal\n", flags.reviewBy)
	}
	return printSyncResult(result, nil)
}

// runGoalUnderAttorneyWithInputs is the seat's own act under a recorded power
// of attorney. The clock and the ledger endpoint come from the caller so the
// attorney entry is read from the same accepted ledger the act publishes to.
func runGoalUnderAttorneyWithInputs(name string, f *syncFlags, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies) int {
	if f.by != "" || f.fixtureHumanAuthority || f.temporaryWord != "" || f.reviewBy != "" || f.approvedRef != "" {
		return dependencies.fail(2, fmt.Errorf("goal %s --under is the seat's own act: it combines with neither --by, a human proof, nor --approved-ref", name))
	}
	if brainState := brain.Read(f.root, goal.ExistingLedgerIdentity(f.root)); brainState.State != brain.Undeclared {
		return dependencies.fail(1, fmt.Errorf("this checkout is declared the brain; the brain never carries a human's word into goal %s, a power of attorney included. Wido runs, from an agent-free terminal: metasystem goal %s --root <checkout> --id <id> --by <name> <the verb's own flags>", name, name))
	}
	now, err := commandNow(f.root)
	if err != nil {
		return dependencies.fail(1, err)
	}
	if dependencies.endpoint == nil {
		return dependencies.fail(1, fmt.Errorf("goal endpoint reader is missing"))
	}
	endpoint, err := dependencies.endpoint(f.root)
	if err != nil {
		return dependencies.fail(1, err)
	}
	entry, err := goal.ResolveAttorneyForEndpoint(endpoint, f.under, name, now)
	if err != nil {
		return dependencies.fail(1, err)
	}
	req, err := syncReqWithProofAtWithDependencies(name, f.root, "", f.lineage, nil, commandNow, dependencies)
	if err != nil {
		return dependencies.fail(1, err)
	}
	req.Attorney = &entry
	var res goal.PublishResult
	switch name {
	case "approve":
		budget, budgetErr := f.approvalBudget()
		if budgetErr != nil {
			return dependencies.fail(2, budgetErr)
		}
		res, err = goal.Approve(req, f.ids, budget, nil)
	case "set-budget":
		budget, budgetErr := f.budgetTuple(true)
		if budgetErr != nil {
			return dependencies.fail(2, budgetErr)
		}
		res, err = goal.SetBudgetApproved(req, f.id, *budget, nil)
	case "unpark":
		if f.verified == "" {
			return dependencies.fail(2, fmt.Errorf("goal unpark --under says what the seat verified holds now: --verified <one line>"))
		}
		res, err = goal.UnparkUnderAttorney(req, f.id, f.verified)
	}
	return dependencies.publish(res, err)
}

func runGoalGrant(args []string) int {
	return runGoalGrantWithAuthority(args, humanauthority.ProveOrTemporaryGoalAuthority)
}

// runGoalGrantWithAuthority records a power of attorney under the human's
// own proof and prints the entry id the seat will name with --under.
func runGoalGrantWithAuthority(args []string, prove goalAuthorityProver) int {
	return runGoalGrantWithInputs(args, prove, goalCommandNow, defaultSyncRequestDependencies())
}

func runGoalGrantWithInputs(args []string, prove goalAuthorityProver, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies) int {
	f, ok := dependencies.parseSyncFlags("grant", args)
	if !ok {
		return 2
	}
	if !converted(f.root) || f.by == "" || f.tiers == "" || f.verbs == "" || f.expires == "" {
		dependencies.complain("goal grant needs a synced backlog plus --by, --tiers, --verbs and --expires")
		return 2
	}
	tiers, err := goal.ParseTiers(f.tiers)
	if err != nil {
		dependencies.complain(err)
		return 2
	}
	var verbs []string
	for _, verb := range strings.Split(f.verbs, ",") {
		if verb = strings.TrimSpace(verb); verb != "" {
			verbs = append(verbs, verb)
		}
	}
	classification, err := classifyGoalAuthorityFirstWithFacts("grant", f, dependencies.authorityFacts)
	if err != nil {
		dependencies.complain(err)
		return 1
	}
	proof, err := proveGoalHumanAuthorityAt("grant", f, prove, commandNow)
	if err != nil {
		dependencies.complain(err)
		return 1
	}
	req, err := syncReqClassifiedWithTerminalGradeAtWithDependencies(f.root, f.by, f.lineage, &proof, classification, false, commandNow, dependencies)
	if err != nil {
		dependencies.complain(err)
		return 1
	}
	res, err := goal.Grant(req, &proof, tiers, verbs, f.expires)
	dependencies.landed(res)
	if err != nil {
		return dependencies.publish(res, err)
	}
	opid := goal.Opid(req.Ulid, req.Actor.Machine, req.Actor.Lineage)
	if res.Outcome == goal.OutcomeConfirmed {
		if err := recordGoalApprovalProof(f.root, opid, "goal grant", proof); err != nil {
			dependencies.complain("goal grant confirmed but could not record its authority proof:", err)
			return 1
		}
	}
	return dependencies.publishGrant(res, opid)
}

func runGoalRevoke(args []string) int {
	return runGoalRevokeWithAuthority(args, humanauthority.ProveOrTemporaryGoalAuthority)
}

func runGoalRevokeWithAuthority(args []string, prove goalAuthorityProver) int {
	return runGoalRevokeWithInputs(args, prove, goalCommandNow, defaultSyncRequestDependencies())
}

func runGoalRevokeWithInputs(args []string, prove goalAuthorityProver, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies) int {
	f, ok := dependencies.parseSyncFlags("revoke", args)
	if !ok {
		return 2
	}
	if !converted(f.root) || f.by == "" || f.id == "" {
		dependencies.complain("goal revoke needs a synced backlog plus --by and --id <entry>")
		return 2
	}
	classification, err := classifyGoalAuthorityFirstWithFacts("revoke", f, dependencies.authorityFacts)
	if err != nil {
		dependencies.complain(err)
		return 1
	}
	proof, err := proveGoalHumanAuthorityAt("revoke", f, prove, commandNow)
	if err != nil {
		dependencies.complain(err)
		return 1
	}
	req, err := syncReqClassifiedWithTerminalGradeAtWithDependencies(f.root, f.by, f.lineage, &proof, classification, false, commandNow, dependencies)
	if err != nil {
		dependencies.complain(err)
		return 1
	}
	res, err := goal.Revoke(req, &proof, f.id)
	dependencies.landed(res)
	if err != nil {
		return dependencies.publish(res, err)
	}
	if res.Outcome == goal.OutcomeConfirmed {
		if err := recordGoalApprovalProof(f.root, goal.Opid(req.Ulid, req.Actor.Machine, req.Actor.Lineage), "goal revoke", proof); err != nil {
			dependencies.complain("goal revoke confirmed but could not record its authority proof:", err)
			return 1
		}
	}
	return dependencies.publish(res, nil)
}

func runGoalResume(args []string) int {
	return runGoalResumeWithAuthority(args, humanauthority.ProveOrTemporaryGoalAuthority)
}

// runGoalResumeWithAuthority gives package tests a direct, non-CLI seam for an
// already-granted proof. The shipped command always enters through
// runGoalResume, whose authority owner reads the real wall clock.
func runGoalResumeWithAuthority(args []string, prove goalAuthorityProver) int {
	return runGoalResumeWithAuthorityAt(args, prove, goalCommandNow)
}

func runGoalResumeWithAuthorityAt(args []string, prove goalAuthorityProver, commandNow func(string) (time.Time, error)) int {
	return runGoalResumeWithAuthorityFacts(args, prove, commandNow, defaultGoalAuthorityReadFacts())
}

func runGoalResumeWithAuthorityFacts(args []string, prove goalAuthorityProver, commandNow func(string) (time.Time, error), facts goalAuthorityReadFacts) int {
	dependencies := defaultSyncRequestDependencies()
	dependencies.authorityFacts = facts
	return runGoalResumeWithInputs(args, prove, commandNow, dependencies, dispatchcore.ResolveGoalBinding)
}

func runGoalResumeWithInputs(args []string, prove goalAuthorityProver, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies, binding goalBindingResolver) int {
	values := newHumanVerbValues("resume", args)
	values.report = dependencies.report
	f, ok := parseHumanSyncFlags(values, "resume", args)
	if !ok {
		return 2
	}
	if !converted(f.root) {
		return refuseHumanVerb(values, 1, "works the synced backlog; this checkout still carries the legacy ledger", humanVerbRemedy{words: "migrate this checkout to the synced backlog first"})
	}
	if f.id == "" {
		return refuseHumanVerb(values, 2, "needs --id", humanVerbRemedy{words: "add the stopped goal's identifier with --id"})
	}
	budget, err := f.budgetTuple(true)
	if err != nil {
		return refuseHumanVerb(values, 2, err.Error(), humanVerbRemedy{command: values.budgetCommand("keep")})
	}
	values.box = budget
	classification, err := classifyGoalAuthorityFirstWithFacts("resume", f, dependencies.authorityFacts)
	if err != nil {
		return dependencies.fail(1, err)
	}
	if commandNow == nil {
		return refuseHumanVerb(values, 1, "goal command clock is missing", humanVerbRemedy{words: "provide a valid goal command clock"})
	}
	ancestryNow, err := commandNow(f.root)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "provide a valid goal command clock"})
	}
	var proof humanauthority.Proof
	if f.approvedRef != "" {
		recorded, approvalErr := goal.AuthenticatedChannelApproval(f.root, f.id, f.approvedRef, goal.ResumeApprovalToken(f.id, *budget), ancestryNow)
		if approvalErr != nil {
			return refuseHumanVerb(values, 1, "could not validate --approved-ref: "+approvalErr.Error(), humanVerbRemedy{words: "record a channel answer for this exact standing budget, or run at the enrolled terminal"})
		}
		proof, err = humanauthority.AuthenticatedChannelProof(f.root, recorded, ancestryNow)
	} else {
		proof, err = proveGoalHumanAuthorityAt("resume", f, prove, commandNow)
	}
	if err != nil {
		code := 1
		message := "could not prove enrolled human ancestry: " + err.Error()
		if f.temporaryWord != "" || f.reviewBy != "" {
			code = 2
			message = "could not bind its temporary recorded relay: " + err.Error()
		}
		return refuseHumanVerb(values, code, message, humanProofRemedy(values, f.fixtureHumanAuthority, f.temporaryWord, f.reviewBy))
	}
	temporaryAuthority := proof.TemporaryResumeFor(f.root)
	if err := resolveGoalHuman(f, proof); err != nil {
		return refuseHumanVerb(values, 2, err.Error(), humanVerbRemedy{words: "re-enroll with goal enroll-terminal --by <your name>, or add --by <your name> to this command"})
	}
	values.by = f.by
	if dependencies.endpoint == nil {
		return refuseHumanVerb(values, 1, "goal endpoint reader is missing", humanVerbRemedy{words: "repair the checkout's goal synchronization endpoint"})
	}
	req, err := syncReqClassifiedWithTerminalGradeAtWithDependencies(f.root, f.by, f.lineage, &proof, classification, false, commandNow, dependencies)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "repair the named checkout identity fact before retrying"})
	}
	req.ApprovedRef = f.approvedRef
	projection, projectErr := goal.Project(req.Endpoint, false, req.Now)
	if projectErr == nil {
		if file := projection.Tree.Live[f.id]; file != nil && file.State == goal.StateClaimed && file.StopFence == nil && file.Budget == nil {
			return refuseHumanVerb(values, 1, "the claimed goal is not breach-stopped and has no standing budget", humanVerbRemedy{command: values.budgetCommand("norm")})
		}
	}
	if binding == nil {
		return refuseHumanVerb(values, 1, "goal binding reader is missing", humanVerbRemedy{words: "repair the goal's current claim binding before retrying"})
	}
	resolvedBinding, err := binding(f.root, f.id, req.Now)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "repair the goal's current claim binding before retrying"})
	}
	if resolvedBinding.Fence == nil {
		projection, projectErr := goal.Project(req.Endpoint, false, req.Now)
		if projectErr == nil {
			if file := projection.Tree.Live[f.id]; file != nil {
				if file.Budget == nil {
					return refuseHumanVerb(values, 1, fmt.Sprintf("revision %d is not breach-stopped", resolvedBinding.Revision), humanVerbRemedy{command: values.budgetCommand("norm")})
				}
				if viewErr := bindGoalTierView(values, f.root, file); viewErr != nil {
					return refuseHumanVerb(values, 1, viewErr.Error(), humanVerbRemedy{words: "repair the configured tier box"})
				}
				if *file.Budget != *budget {
					return refuseHumanVerb(values, 1, fmt.Sprintf("revision %d is not breach-stopped", resolvedBinding.Revision), humanVerbRemedy{command: values.budgetCommand(goalbudget.FormatBox(*budget))})
				}
			}
		}
		return refuseHumanVerb(values, 1, fmt.Sprintf("revision %d is not breach-stopped", resolvedBinding.Revision), humanVerbRemedy{words: "the goal is already live under that standing box"})
	}
	held, err := goalrevision.Acquire(f.root, f.id, resolvedBinding.Revision, "goal-resume")
	if err != nil {
		return refuseHumanVerb(values, 1, "could not acquire the goal-revision lock: "+err.Error(), humanVerbRemedy{words: "wait for the current goal revision operation to finish"})
	}
	defer held.Release()
	res, err := goal.Resume(goal.ResumeRequest{VerbRequest: req, GoalID: f.id, Budget: *budget, Authority: &proof})
	if err == nil && res.Outcome == goal.OutcomeConfirmed {
		dependencies.landed(res)
		if err := humanauthority.RecordResumeProof(f.root, goal.Opid(req.Ulid, req.Actor.Machine, req.Actor.Lineage), proof); err != nil {
			return refuseHumanVerb(values, 1, "the act landed at tip "+res.Tip+", but its authority proof did not: "+err.Error(), humanVerbRemedy{words: "the act already landed; do not run it again"})
		}
	}
	if err == nil && res.Outcome == goal.OutcomeConfirmed && temporaryAuthority {
		dependencies.note(true, fmt.Sprintf("goal resume: TEMPORARY authority under a recorded relayed word (human provenance not verified); re-approval due %s at an agent-free terminal", f.reviewBy))
	}
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "repair the stopped goal's recorded cancellation batch before retrying"})
	}
	if res.Outcome != goal.OutcomeConfirmed {
		dependencies.showOutcome(res)
		return refuseHumanVerb(values, 1, res.Detail, humanVerbRemedy{command: values.budgetCommand("keep")})
	}
	return dependencies.publish(res, err)
}

func runGoalSplit(args []string) int {
	return runGoalSplitWithInputs(args, goalCommandNow, defaultSyncRequestDependencies())
}

func runGoalSplitWithInputs(args []string, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies) int {
	f, ok := dependencies.parseSyncFlags("split", args)
	if !ok {
		return 2
	}
	if !converted(f.root) {
		dependencies.complain("goal split works the synced backlog; this checkout still carries the legacy ledger")
		return 1
	}
	if f.id == "" || f.members == "" {
		dependencies.complain("goal split needs --id and --members")
		return 2
	}
	draftBytes, err := os.ReadFile(f.members)
	if err != nil {
		dependencies.complain("goal split could not read its member draft:", err)
		return 1
	}
	members, err := goal.ParseMemberDraft(draftBytes, f.id)
	if err != nil {
		dependencies.complain("goal split draft refused:", err)
		return 1
	}
	req, err := syncReqWithProofAtWithDependencies("split", f.root, f.by, f.lineage, nil, commandNow, dependencies)
	if err != nil {
		dependencies.complain(err)
		return 1
	}
	projection, err := goal.Project(req.Endpoint, false, req.Now)
	if err != nil {
		dependencies.complain(err)
		return 1
	}
	parent := projection.Tree.Live[f.id]
	if parent == nil {
		if _, archived := projection.Tree.Archived(f.id); archived {
			dependencies.complainf("goal %s is in the archive; there is nothing to split\n", f.id)
		} else {
			dependencies.complainf("goal %s does not exist\n", f.id)
		}
		return 1
	}
	digest := goal.SplitDraftSHA256(f.id, members)
	var proof *humanauthority.Proof
	var ratification goal.SplitRatification
	if f.by != "" {
		observed, proofErr := humanauthority.Prove(f.root, int64(os.Getppid()), nil, time.Now().UTC())
		if proofErr != nil {
			dependencies.complain("SPLIT_RATIFY_REFUSED: goal split could not prove enrolled human ancestry:", proofErr)
			return 1
		}
		proof = &observed
		ratification = goal.SplitRatification{Tier: goal.RatifierHuman, By: f.by, DraftSHA256: digest}
	} else {
		classification, classErr := classifyVerbCaller(f.root, int64(os.Getppid()))
		if classErr != nil {
			dependencies.complain("SPLIT_RATIFY_REFUSED: caller classification failed:", classErr)
			return 1
		}
		if parent.Origin != goal.OriginMain {
			dependencies.complainf("SPLIT_RATIFY_REFUSED: goal %s's split draft must be ratified by its origin tier — use --by from the enrolled human terminal for human-origin work, or run from the MAIN checkout-lease holder for main-origin work\n", f.id)
			return 1
		}
		ratification, err = mainSplitRatification(f.id, digest, classification)
		if err != nil {
			dependencies.complain(err)
			return 1
		}
	}

	var held *goalrevision.Held
	if parent.State == goal.StateClaimed && parent.Claimed != nil && parent.Claimed.Machine == req.Actor.Machine && parent.Claimed.Lineage == req.Actor.Lineage {
		held, err = goalrevision.Acquire(f.root, f.id, parent.Claimed.Revision, "goal-split")
		if err != nil {
			dependencies.complain("goal split could not acquire the goal-revision lock:", err)
			return 1
		}
		defer held.Release()
		spend := dispatchcore.ProjectBudget(f.root, parent, req.Now)
		if spend.Status != dispatchcore.BudgetKnown {
			detail := "unknown spending evidence"
			if spend.Unknown != nil {
				detail = spend.Unknown.Record + ": " + spend.Unknown.Reason
			}
			dependencies.complainf("goal %s revision %d cannot prove zero work: %s\n", f.id, parent.Claimed.Revision, detail)
			return 1
		}
		if spend.Attempts != 0 || spend.ActiveJobs != 0 || spend.ReservedJobMinutes != 0 {
			dependencies.complainf("goal %s revision %d has recorded work (%d attempts, %d active jobs, %d reserved minutes); split is a before-slicing act — conclude the slice or take the parent to the human\n", f.id, parent.Claimed.Revision, spend.Attempts, spend.ActiveJobs, spend.ReservedJobMinutes)
			return 1
		}
	}
	if proof != nil {
		if err := humanauthority.RecordProof(f.root, goal.Opid(req.Ulid, req.Actor.Machine, req.Actor.Lineage), "goal split", *proof); err != nil {
			dependencies.complain("goal split could not record its authority proof:", err)
			return 1
		}
	}
	res, err := goal.Split(req, f.id, members, ratification, proof)
	return dependencies.publish(res, err)
}

func mainSplitRatification(id, digest string, classification lease.ClassifyResult) (goal.SplitRatification, error) {
	if classification.Class == lease.ClassHuman {
		// HUMAN classification proves terminal ancestry but deliberately
		// carries no person's name. The accepted tier=human token does name
		// its ratifier, so only explicit --by can mint it without fabricating
		// identity from an authentication class.
		return goal.SplitRatification{}, fmt.Errorf("SPLIT_RATIFY_REFUSED: caller is human-classified, but the accepted human ratification token must name its person; re-run goal split with --by from the enrolled human terminal")
	}
	if classification.Class != lease.ClassMain || !classification.Holder {
		return goal.SplitRatification{}, fmt.Errorf("SPLIT_RATIFY_REFUSED: goal %s is main-origin; its split draft is ratified by the coordinator — re-run goal split from the MAIN checkout-lease holder session (a human may also run it with --by)", id)
	}
	if classification.ClaimEpoch == nil {
		return goal.SplitRatification{}, fmt.Errorf("SPLIT_RATIFY_REFUSED: goal %s has a MAIN holder classification but no checkout lease epoch; run metasystem up to establish the authenticated lease, then retry", id)
	}
	return goal.SplitRatification{Tier: goal.RatifierMain, MainID: classification.MainId, ClaimEpoch: *classification.ClaimEpoch, DraftSHA256: digest}, nil
}

type goalTerminalEnroller func(string, int64, humanauthority.Reader, string, time.Time) (humanauthority.Enrollment, error)

func runGoalEnrollTerminal(args []string) int {
	return runGoalEnrollTerminalWith(args, humanauthority.Enroll)
}

func runGoalEnrollTerminalWith(args []string, enroll goalTerminalEnroller) int {
	return runGoalEnrollTerminalWithDependencies(args, enroll, goalCommandNow, defaultSyncRequestDependencies())
}

func runGoalEnrollTerminalWithDependencies(args []string, enroll goalTerminalEnroller, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies) int {
	values := newHumanVerbValues("enroll-terminal", args)
	values.report = dependencies.report
	flags := flag.NewFlagSet("goal enroll-terminal", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	root := pathFlag(flags, "root", ".", "checkout root")
	by := flags.String("by", "", "the human enrolling this terminal")
	lineage := flags.String("lineage", "", "coordinator lineage used to publish the fleet enrollment")
	if err := flags.Parse(args); err != nil {
		return refuseHumanVerb(values, 2, err.Error(), humanVerbRemedy{words: "use only --root, --lineage, and --by with terminal enrollment"})
	}
	values.root, values.lineage, values.by = *root, *lineage, *by
	if !converted(*root) {
		return refuseHumanVerb(values, 1, "requires the synced backlog so the first enrollment ends relayed approval fleet-wide", humanVerbRemedy{words: "migrate this checkout to the synced backlog first"})
	}
	if strings.TrimSpace(*by) == "" {
		return refuseHumanVerb(values, 2, "needs --by", humanVerbRemedy{words: "re-enroll with goal enroll-terminal --by <your name>"})
	}
	enrollment, err := enroll(*root, int64(os.Getppid()), nil, *by, time.Now().UTC())
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "run enrollment from a shell whose ancestry to its session leader is owned by you"})
	}
	if dependencies.report != nil {
		// The local enrollment is committed; what follows publishes it.
		dependencies.report.value = enrollment
	}
	req, err := syncReqWithProofAtWithDependencies("enroll-terminal", *root, "terminal", *lineage, nil, commandNow, dependencies)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "repair the named checkout identity fact before retrying"})
	}
	req.Now = enrollment.EnrolledAt
	res, err := goal.RecordFleetEnrollment(req, enrollment.Generation)
	// Another machine may already own the immutable fleet cutoff. That leaves
	// this machine's completed local enrollment valid and needs no root rewrite.
	if err != nil || (res.Outcome != goal.OutcomeConfirmed && res.Outcome != goal.OutcomeConfirmedLate && res.Outcome != goal.OutcomeAbandoned) {
		return refuseHumanVerb(values, 1, fmt.Sprint("the terminal enrolled locally but its fleet cutoff did not publish: ", err, " ", res.Detail), humanVerbRemedy{words: "the local enrollment stands; repair fleet synchronization without re-enrolling"})
	}
	if dependencies.report != nil {
		dependencies.report.result = &res
		return 0
	}
	printJSON(enrollment)
	return 0
}

func runGoalSetObligation(args []string) int {
	return runGoalSetObligationWithAuthority(args, humanauthority.ProveOrTemporaryGoalAuthority)
}

// runGoalSetObligationWithAuthority is the test-only entry seam paired with
// runGoalResumeWithAuthority; no flag, config, or environment value selects it.
func runGoalSetObligationWithAuthority(args []string, prove goalAuthorityProver) int {
	return runGoalSetObligationWithAuthorityFacts(args, prove, defaultGoalAuthorityReadFacts())
}

func runGoalSetObligationWithAuthorityFacts(args []string, prove goalAuthorityProver, facts goalAuthorityReadFacts) int {
	dependencies := defaultSyncRequestDependencies()
	dependencies.authorityFacts = facts
	return runGoalSetObligationWithAuthorityFactsAtWithDependencies(args, prove, goalCommandNow, dependencies)
}

func runGoalSetObligationWithAuthorityFactsAtWithDependencies(args []string, prove goalAuthorityProver, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies) int {
	values := newHumanVerbValues("set-obligation", args)
	values.report = dependencies.report
	flags := flag.NewFlagSet("goal set-obligation", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	root := pathFlag(flags, "root", ".", "checkout root")
	id := flags.String("id", "", "claimed goal id")
	by := flags.String("by", "", "directing human")
	lineage := flags.String("lineage", "", "coordinator lineage")
	state := flags.String("state", "", "DRAFT|OBSERVE|LIMITED|ENFORCED")
	owner := flags.String("owner", "", "person accountable for the obligation")
	recurrence := flags.String("recurrence", "", "single-experiment|standing-shared-process")
	platform := flags.String("platform", "", "authorized operating-system/architecture token")
	toolchain := flags.String("toolchain-identity", "", "authorized toolchain identity")
	surface := flags.String("surface-digest", "", "authorized behavior-surface digest")
	maxActiveJobs := flags.Uint64("max-active-jobs", 0, "greatest active-job observation permitted")
	timingEnvelope := flags.Uint64("timing-envelope-sec", 0, "maximum terminal duration")
	var effects repeatedStrings
	flags.Var(&effects, "effect", "governing effect (repeatable)")
	valueJudgment := flags.String("value-judgment", "", "yes|no|unknown")
	reversibility := flags.String("reversibility", "", "reversible|compensable|irreversible|unknown")
	severeHarm := flags.String("severe-harm", "", "yes|no|unknown")
	unfamiliarApproach := flags.String("unfamiliar-approach", "", "yes|no|unknown")
	testDiscrimination := flags.String("test-discrimination", "", "strong|weak|unknown")
	correlatedRisk := flags.String("correlated-assumption-risk", "", "yes|no|unknown")
	authorityScopeChange := flags.String("authority-scope-change", "", "yes|no|unknown")
	destructiveReach := flags.String("destructive-reach", "", "none|reversible-local|destructive|unknown")
	temporaryWord := flags.String("temporary-human-word", "", "recorded relayed words presented as the human's; provenance is not verified; authorizes set-obligation TEMPORARILY")
	reviewBy := flags.String("review-by", "", "recorded re-approval date supplied with the relay (required with --temporary-human-word)")
	approvedRef := flags.String("approved-ref", "", "authenticated channel answer operation on this goal")
	fixtureHumanAuthority := flags.Bool("fixture-human-authority", false, "fixture-only enrolled-human proof; accepted only for an exact fake-runtime root")
	if err := flags.Parse(args); err != nil {
		return refuseHumanVerb(values, 2, err.Error(), humanVerbRemedy{words: "remove the unrecognized flag and retry with the documented obligation fields"})
	}
	values.root, values.lineage, values.id, values.by = *root, *lineage, *id, *by
	values.approvedRef, values.temporaryWord, values.reviewBy = *approvedRef, *temporaryWord, *reviewBy
	values.fixtureHumanAuthority = *fixtureHumanAuthority
	if flags.NArg() != 0 {
		return refuseHumanVerb(values, 2, "does not take positional values", humanVerbRemedy{words: "remove the positional value and retry with the documented obligation fields"})
	}
	if *id == "" || *by == "" || *state == "" || *owner == "" || *recurrence == "" ||
		*platform == "" || *toolchain == "" || *surface == "" || *maxActiveJobs == 0 || *timingEnvelope == 0 ||
		len(effects) == 0 || *valueJudgment == "" || *reversibility == "" || *severeHarm == "" ||
		*unfamiliarApproach == "" || *testDiscrimination == "" || *correlatedRisk == "" || *authorityScopeChange == "" || *destructiveReach == "" {
		if *id == "" || *state == "" || *owner == "" || *recurrence == "" || *platform == "" || *toolchain == "" || *surface == "" || *maxActiveJobs == 0 || *timingEnvelope == 0 || len(effects) == 0 || *valueJudgment == "" || *reversibility == "" || *severeHarm == "" || *unfamiliarApproach == "" || *testDiscrimination == "" || *correlatedRisk == "" || *authorityScopeChange == "" || *destructiveReach == "" {
			return refuseHumanVerb(values, 2, "requires identity, recurrence, platform and toolchain and surface observations, active and timing ceilings, effects, and every typed review trigger", humanVerbRemedy{words: "supply every missing named flag; their values were not present"})
		}
	}
	if !converted(*root) {
		return refuseHumanVerb(values, 1, "works only with the synced backlog", humanVerbRemedy{words: "migrate this checkout to the synced backlog first"})
	}
	classification, err := brainHumanWordClassificationWithFacts("set-obligation", *root, *by, nil, dependencies.authorityFacts)
	if err != nil {
		dependencies.complain(err)
		return 1
	}
	ancestryNow, err := commandNow(*root)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "provide a valid goal command clock"})
	}
	var proof humanauthority.Proof
	if *approvedRef != "" {
		recorded, approvalErr := goal.AuthenticatedChannelApproval(*root, *id, *approvedRef, goal.SetObligationApprovalToken(*id, goal.ObligationState(*state), *owner), ancestryNow)
		if approvalErr != nil {
			return refuseHumanVerb(values, 1, "could not validate --approved-ref: "+approvalErr.Error(), humanVerbRemedy{words: "record a channel answer for this exact obligation, or run at the enrolled terminal"})
		}
		proof, err = humanauthority.AuthenticatedChannelProof(*root, recorded, ancestryNow)
	} else {
		authorityFlags := &syncFlags{root: *root, id: *id, by: *by, lineage: *lineage,
			temporaryWord: *temporaryWord, reviewBy: *reviewBy, fixtureHumanAuthority: *fixtureHumanAuthority}
		proof, err = proveGoalHumanAuthorityAt("set-obligation", authorityFlags, prove, commandNow)
	}
	if err != nil {
		code := 1
		message := "could not prove enrolled human ancestry: " + err.Error()
		if *temporaryWord != "" || *reviewBy != "" {
			code = 2
			message = "could not bind its temporary recorded relay: " + err.Error()
		}
		return refuseHumanVerb(values, code, message, humanProofRemedy(values, *fixtureHumanAuthority, *temporaryWord, *reviewBy))
	}
	temporaryAuthority := proof.TemporarySetObligationFor(*root)
	authorityFlags := &syncFlags{root: *root, by: *by}
	if err := resolveGoalHuman(authorityFlags, proof); err != nil {
		return refuseHumanVerb(values, 2, err.Error(), humanVerbRemedy{words: "re-enroll with goal enroll-terminal --by <your name>, or add --by <your name> to this command"})
	}
	*by = authorityFlags.by
	values.by = *by
	req, err := syncReqClassifiedWithTerminalGradeAtWithDependencies(*root, *by, *lineage, &proof, classification, false, commandNow, dependencies)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "repair the named checkout identity fact before retrying"})
	}
	operationID := goal.Opid(req.Ulid, req.Actor.Machine, req.Actor.Lineage)
	governingEffects := make([]goal.GoverningEffect, len(effects))
	for index, effect := range effects {
		governingEffects[index] = goal.GoverningEffect(effect)
	}
	obligation := goal.GovernedObligation{
		State: goal.ObligationState(*state), Owner: *owner, Effects: governingEffects,
		Assumptions: goal.ObligationAssumptions{Recurrence: goal.RecurrenceClass(*recurrence),
			Platform: *platform, ToolchainIdentity: *toolchain, SurfaceDigest: *surface,
			MaxActiveJobs: *maxActiveJobs, TimingEnvelopeSeconds: *timingEnvelope,
			ObservationSource: "run-terminal-record"},
		Triggers: goal.HumanReviewTriggers{ValueJudgment: *valueJudgment, Reversibility: *reversibility,
			SevereHarm: *severeHarm, UnfamiliarApproach: *unfamiliarApproach, TestDiscrimination: *testDiscrimination,
			CorrelatedAssumptionRisk: *correlatedRisk, AuthorityScopeChange: *authorityScopeChange, DestructiveReach: *destructiveReach},
	}
	res, err := goal.SetObligation(req, *id, obligation, &proof)
	dependencies.landed(res)
	if err != nil {
		return refuseHumanVerb(values, 1, err.Error(), humanVerbRemedy{words: "repair the goal's current claimed budget or obligation fields before retrying"})
	}
	if res.Outcome == goal.OutcomeConfirmed {
		if err := humanauthority.RecordSetObligationProof(*root, operationID, proof); err != nil {
			return refuseHumanVerb(values, 1, "the act landed at tip "+res.Tip+", but its authority proof did not: "+err.Error(), humanVerbRemedy{words: "the act already landed; do not run it again"})
		}
	}
	if res.Outcome == goal.OutcomeConfirmed && temporaryAuthority {
		dependencies.note(true, fmt.Sprintf("goal set-obligation: TEMPORARY authority under a recorded relayed word (human provenance not verified); re-approval due %s at an agent-free terminal", *reviewBy))
	}
	if res.Outcome != goal.OutcomeConfirmed {
		dependencies.outcomeBeforeRefusal(res)
		return refuseHumanVerb(values, 1, res.Detail, humanVerbRemedy{words: "repair the goal's current claimed budget or obligation fields before retrying"})
	}
	return dependencies.publish(res, nil)
}

func runGoalEditWithDependencies(args []string, commandNow func(string) (time.Time, error), dependencies syncRequestDependencies) int {
	requestBuilder := func(verb, root, by, lineage string) (goal.VerbRequest, error) {
		return syncReqWithProofAtWithDependencies(verb, root, by, lineage, nil, commandNow, dependencies)
	}
	run := runSyncOnlyWithRequest("edit", func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		return goalEditEffect(req, f, commandNow)
	}, requestBuilder, "id")
	return run(args)
}

func goalEditEffect(req goal.VerbRequest, f *syncFlags, commandNow func(string) (time.Time, error)) (goal.PublishResult, error) {
	return goalEditEffectAppending(req, f, commandNow, "")
}

// goalEditEffectAppending is the edit with text added to the next step
// inside the edit's own transaction.
func goalEditEffectAppending(req goal.VerbRequest, f *syncFlags, commandNow func(string) (time.Time, error), appendNext string) (goal.PublishResult, error) {
	fields := goal.EditFields{Why: f.why, Evidence: f.evidence}
	if appendNext != "" {
		fields.NextStepAppend = &appendNext
	}
	var beforeDerived uint8
	if f.tier != 0 && f.risk == "" {
		return goal.PublishResult{}, fmt.Errorf("answer the four questions: --risk severity=,novelty=,exposure=,accumulation= --basis")
	}
	if f.risk != "" {
		if strings.TrimSpace(f.basis) == "" {
			return goal.PublishResult{}, fmt.Errorf("goal edit --risk requires --basis")
		}
		risk, err := goal.ParseRiskRecord(f.risk, f.basis)
		if err != nil {
			return goal.PublishResult{}, err
		}
		fields.Risk = &risk
		projected, err := goal.Project(req.Endpoint, false, req.Now)
		if err != nil {
			return goal.PublishResult{}, err
		}
		current := projected.Tree.Live[f.id]
		if current == nil {
			return goal.PublishResult{}, fmt.Errorf("goal %s is not live", f.id)
		}
		beforeDerived = current.Tier
		if current.Risk != nil {
			beforeDerived = current.Risk.DerivedTier()
		}
		if current.Approved != nil && risk.DerivedTier() > beforeDerived {
			if err := dispatchcore.ValidateMisclassificationEvidence(f.root, f.id, f.evidence); err != nil {
				return goal.PublishResult{}, err
			}
		}
		// A lowering of an answer, the derivation, the width or the recorded
		// tier is the human's act: with --by the edge proves the human; without
		// it the verb refuses the lowering by name.
		tierLowered := f.tier != 0 && uint8(f.tier) < current.Tier
		if f.by != "" && (tierLowered || current.Risk != nil && (risk.Severity < current.Risk.Severity || risk.Novelty < current.Risk.Novelty || risk.Exposure < current.Risk.Exposure || risk.Accumulation < current.Risk.Accumulation || risk.DerivedTier() < beforeDerived || (current.Risk.GateWidth() == "full" && risk.GateWidth() == "area"))) {
			proof, proofErr := proveGoalHumanAuthorityAt("edit", f, humanauthority.ProveOrTemporaryGoalAuthority, commandNow)
			if proofErr != nil {
				return goal.PublishResult{}, proofErr
			}
			fields.Proof = &proof
		}
	}
	if f.intent != "" {
		fields.Intent = &f.intent
	}
	if f.next != "" {
		fields.NextStep = &f.next
	}
	if f.tier != 0 {
		tier := uint8(f.tier)
		fields.Tier = &tier
	}
	if len(f.labels) > 0 || len(f.unlabels) > 0 {
		p, err := goal.Project(req.Endpoint, false, time.Now())
		if err != nil {
			return goal.PublishResult{}, err
		}
		current, exists := p.Tree.Live[f.id]
		if !exists {
			return goal.PublishResult{}, fmt.Errorf("goal %s is not live; the archive edits through reopen", f.id)
		}
		labels, err := goal.ApplyLabelDelta(current.Labels, f.labels, f.unlabels)
		if err != nil {
			return goal.PublishResult{}, err
		}
		fields.Labels = &labels
	}
	res, err := goal.Edit(req, f.id, fields)
	if err == nil && res.Outcome == goal.OutcomeConfirmed && fields.Risk != nil && res.RiskRaised {
		opid := goal.Opid(req.Ulid, req.Actor.Machine, req.Actor.Lineage)
		if appendErr := counselor.AppendMisclassification(f.root, counselor.MisclassificationAppend{Goal: f.id, OpID: opid, From: int(beforeDerived), To: int(fields.Risk.DerivedTier()), Evidence: f.evidence, RecordedAt: req.Now}); appendErr != nil {
			return res, appendErr
		}
	}
	return res, err
}

var (
	runGoalClaim = runSyncOnly("claim", func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		budget, err := f.budgetTuple(false)
		if err != nil {
			return goal.PublishResult{}, err
		}
		if f.arc != "" {
			if budget != nil {
				return goal.ClaimArc(req, f.id, *budget)
			}
			return goal.ClaimArc(req, f.id)
		}
		if budget != nil {
			return goal.Claim(req, f.id, *budget)
		}
		return goal.Claim(req, f.id)
	}, "id")
	runGoalRestamp = runSyncOnly("restamp", func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		return goal.Restamp(req, f.id)
	}, "id")
	runGoalRelease = func(args []string) int { return runGoalReleaseWithRequest(args, nil) }
	runGoalSteal   = runSyncOnly("steal", func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		return goal.Steal(req, f.id)
	}, "id")
	runGoalLandReady = runSyncOnly("land-ready", func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		return goal.LandReady(req, f.id)
	}, "id")
	runGoalEdit = runSyncOnly("edit", func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		return goalEditEffect(req, f, goalCommandNow)
	}, "id")
	runGoalSetPin = runSyncOnly("set-pin", func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		return goal.SetPin(req, f.id, f.pin)
	}, "id", "pin")
	runGoalSetArc = runSyncOnly("set-arc", func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		return goal.SetArc(req, f.id, f.arc)
	}, "id", "arc")
	runGoalDetach = runSyncOnly("detach", func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		return goal.Detach(req, f.id)
	}, "id")
)
