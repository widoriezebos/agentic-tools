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
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/brain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/counselor"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalrevision"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

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
	flags := flag.NewFlagSet("goal carry", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
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
	root := flags.String("root", ".", "checkout root")
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
	root := flags.String("root", ".", "checkout root")
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

var proveSyncReqHumanAuthority = humanauthority.Prove

func terminalEnrollmentLineage(enrollment humanauthority.Enrollment) string {
	terminalID := []byte(enrollment.TerminalID)
	for index, character := range terminalID {
		if !('A' <= character && character <= 'Z') && !('a' <= character && character <= 'z') &&
			!('0' <= character && character <= '9') && character != '-' {
			terminalID[index] = '-'
		}
	}
	return fmt.Sprintf("terminal-%s-%d", terminalID, enrollment.Generation)
}

func syncReqWithProof(verb, root, by, lineageFlag string, observedProof *humanauthority.Proof) (goal.VerbRequest, error) {
	classification, classifyErr := brainHumanWordClassification(verb, root, by, observedProof)
	if classifyErr != nil {
		return goal.VerbRequest{}, classifyErr
	}
	return syncReqClassified(root, by, lineageFlag, observedProof, classification)
}

func syncReqClassified(root, by, lineageFlag string, observedProof *humanauthority.Proof, classification lease.ClassifyResult) (goal.VerbRequest, error) {
	if err := ensureGuardEnrolled(root); err != nil {
		return goal.VerbRequest{}, err
	}
	e, err := goal.ResolveEndpoint(root)
	if err != nil {
		return goal.VerbRequest{}, err
	}
	machine, err := goal.ResolveMachine(root)
	if err != nil {
		return goal.VerbRequest{}, err
	}
	lineage := lineageFlag
	if lineage == "" {
		lineage = os.Getenv("METASYSTEM_OWNER_LINEAGE")
	}
	if lineage == "" {
		if by == "" {
			return goal.VerbRequest{}, fmt.Errorf("mutations carry their coordinator's identity: export METASYSTEM_OWNER_LINEAGE or pass --lineage")
		}
		enrollment, err := humanauthority.ReadEnrollment(root)
		if err != nil {
			return goal.VerbRequest{}, fmt.Errorf("a human act derives its lineage from the enrolled terminal, and this checkout has none: run metasystem goal enroll-terminal here once, or pass --lineage: %w", err)
		}
		now, nowErr := goalCommandNow(root)
		if nowErr != nil {
			return goal.VerbRequest{}, nowErr
		}
		proof := humanauthority.Proof{}
		var proofErr error
		if observedProof != nil {
			proof = *observedProof
		} else {
			proof, proofErr = proveSyncReqHumanAuthority(root, int64(os.Getppid()), nil, now)
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
	}
	ulid, err := goalUlid()
	if err != nil {
		return goal.VerbRequest{}, err
	}
	now, err := goalCommandNow(root)
	if err != nil {
		return goal.VerbRequest{}, err
	}
	req := goal.VerbRequest{
		Endpoint: e, Actor: goal.Actor{Machine: machine, Lineage: lineage, Human: by},
		Ulid: ulid, Now: now, CallerClass: classification.Class,
	}
	if classification.ClaimEpoch != nil && (classification.Holder || by != "" || classification.Class == lease.ClassHuman) {
		req.ClaimEpoch = *classification.ClaimEpoch
	} else if classification.Class == lease.ClassHuman {
		req.ClaimEpoch = 1
	}
	return req, nil
}

func brainHumanWordClassification(verb, root, by string, observedProof *humanauthority.Proof) (lease.ClassifyResult, error) {
	classification, classifyErr := classifyVerbCaller(root, int64(os.Getppid()))
	brainState := brain.Read(root, goal.ExistingLedgerIdentity(root))
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
	classification, err := brainHumanWordClassification(verb, f.root, f.by, fixtureProof)
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

// syncFlags is the shared flag surface; each verb reads the fields
// it consumes and ignores the rest.
type syncFlags struct {
	root, by, id, intent, next, origin, because, conclude, arc, pin, members string
	blocks, under, tiers, verbs, expires                                     string
	lineage, digest, elapsedLimit, approvedRef, temporaryWord, reviewBy      string
	budgetBox, confirm, risk, basis, evidence                                string
	finding, chain, why, test                                                string
	attemptLimit, reservedJobMinutesLimit, activeJobLimit, reviewRoundLimit  int64
	tier                                                                     uint
	labels, unlabels, ids                                                    repeatedStrings
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
	fs := flag.NewFlagSet("goal "+name, flag.ContinueOnError)
	f := &syncFlags{}
	fs.StringVar(&f.root, "root", ".", "checkout root")
	fs.StringVar(&f.by, "by", "", "the directing human (a human act carries its name)")
	if name == "approve" {
		fs.Var(&f.ids, "id", "goal id (repeatable)")
	} else {
		fs.StringVar(&f.id, "id", "", "goal id")
	}
	fs.StringVar(&f.intent, "intent", "", "one-line intent")
	fs.StringVar(&f.next, "next", "", "the next step")
	fs.StringVar(&f.origin, "origin", "main", "creation provenance: human|main")
	fs.StringVar(&f.blocks, "blocks", "", "the live goal this open unblocks: it parks with this blocker in the same publish and returns when the blocker is done (a seat's open, origin main, requires it)")
	fs.StringVar(&f.under, "under", "", "act under a recorded power of attorney entry (approve and set-budget): the seat's own act, no --by and no proof")
	fs.StringVar(&f.tiers, "tiers", "", "grant: the tiers the power of attorney covers (1 in this build)")
	fs.StringVar(&f.verbs, "verbs", "", "grant: the verbs the power of attorney covers, from approve,set-budget")
	fs.StringVar(&f.expires, "expires", "", "grant: the last day the power of attorney covers, YYYY-MM-DD, at most seven days out")
	fs.StringVar(&f.because, "because", "", "the park's reason")
	fs.StringVar(&f.conclude, "conclude", "", "the conclusion")
	fs.StringVar(&f.arc, "arc", "", "the destination arc")
	fs.StringVar(&f.pin, "pin", "", "the machine nickname a goal is pinned to (\"-\" clears)")
	fs.StringVar(&f.members, "members", "", "split member draft path")
	fs.StringVar(&f.finding, "finding", "", "finding identifier")
	fs.StringVar(&f.chain, "chain", "", "critic chain root")
	fs.StringVar(&f.why, "why", "", "decision reason")
	fs.StringVar(&f.risk, "risk", "", "four risk answers: severity=,novelty=,exposure=,accumulation=")
	fs.StringVar(&f.basis, "basis", "", "plain-English basis for the four risk answers")
	fs.StringVar(&f.evidence, "evidence", "", "misclassification evidence reference")
	fs.StringVar(&f.test, "test", "", "test citation")
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
	if name == "resume" || name == "approve" || name == "unapprove" || name == "set-budget" || name == "accept-risk" || name == "open" || name == "edit" || name == "grant" || name == "revoke" {
		fs.StringVar(&f.temporaryWord, "temporary-human-word", "", "recorded relayed words presented as the human's; provenance is not verified; resumes TEMPORARILY")
		fs.StringVar(&f.reviewBy, "review-by", "", "recorded re-approval date supplied with the relay (required with --temporary-human-word)")
	}
	if name == "approve" || name == "set-budget" || name == "accept-risk" || name == "open" || name == "edit" || name == "grant" || name == "revoke" {
		fs.BoolVar(&f.fixtureHumanAuthority, "fixture-human-authority", false, "fixture-only enrolled-human proof; accepted only for an exact fake-runtime root")
	}
	fs.Var(&f.labels, "label", "label token (repeatable)")
	fs.Var(&f.unlabels, "unlabel", "label token to remove (repeatable; edit only)")
	fs.BoolVar(&f.claim, "claim", false, "claim on open")
	fs.BoolVar(&f.refreshOnly, "refresh-only", false, "complete a died refresh")
	fs.IntVar(&f.keep, "keep", 10, "archive entries to keep")
	if fs.Parse(args) != nil {
		return nil, false
	}
	if name != "open" && f.blocks != "" {
		fmt.Fprintf(os.Stderr, "goal %s does not take --blocks\n", name)
		return nil, false
	}
	if name != "approve" && name != "set-budget" && f.under != "" {
		fmt.Fprintf(os.Stderr, "goal %s does not take --under; a power of attorney covers approve and set-budget\n", name)
		return nil, false
	}
	if name != "grant" && (f.tiers != "" || f.verbs != "" || f.expires != "") {
		fmt.Fprintf(os.Stderr, "goal %s does not take --tiers, --verbs or --expires\n", name)
		return nil, false
	}
	if name != "open" && name != "edit" {
		if len(f.labels) > 0 {
			fmt.Fprintf(os.Stderr, "goal %s does not take --label\n", name)
			return nil, false
		}
		if len(f.unlabels) > 0 {
			fmt.Fprintf(os.Stderr, "goal %s does not take --unlabel\n", name)
			return nil, false
		}
		if f.tier != 0 {
			fmt.Fprintf(os.Stderr, "goal %s does not take --tier\n", name)
			return nil, false
		}
		if f.risk != "" || f.basis != "" || f.evidence != "" {
			fmt.Fprintf(os.Stderr, "goal %s does not take --risk, --basis, or --evidence\n", name)
			return nil, false
		}
	}
	if name != "open" && name != "claim" && name != "set-budget" && name != "resume" && name != "approve" && f.hasAnyBudgetFlag() {
		fmt.Fprintf(os.Stderr, "goal %s does not take budget flags\n", name)
		return nil, false
	}
	if f.approvedRef != "" && name != "set-budget" && name != "resume" && name != "approve" {
		fmt.Fprintf(os.Stderr, "goal %s does not take --approved-ref\n", name)
		return nil, false
	}
	if f.members != "" && name != "split" {
		fmt.Fprintf(os.Stderr, "goal %s does not take --members\n", name)
		return nil, false
	}
	return f, true
}

func runGoalDischargeReviewObligation(args []string) int {
	f, ok := parseSyncFlags("discharge-review-obligation", args)
	if !ok || f.id == "" || f.finding == "" || f.chain == "" || f.by == "" || f.test == "" {
		fmt.Fprintln(os.Stderr, "goal discharge-review-obligation needs --id, --finding, --chain, --by, and --test")
		return 2
	}
	if f.chain == goal.HumanCarriedChain {
		commit, commitErr := humanCarriedFindingCommit(f.finding)
		if commitErr != nil {
			fmt.Fprintln(os.Stderr, commitErr)
			return 1
		}
		if err := dispatchcore.ValidateHumanCarriedCritic(f.root, f.test, commit); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	req, err := syncReq("discharge-review-obligation", f.root, f.by, f.lineage)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if req.CallerClass == lease.ClassHuman {
		req.Actor.Human = f.by
	}
	res, err := goal.DischargeReviewObligation(req, f.id, f.finding, f.chain, f.by, f.test)
	return printSyncResult(res, err)
}

func runGoalAcceptRisk(args []string) int {
	return runGoalAcceptRiskWithAuthority(args, humanauthority.ProveOrTemporaryGoalAuthority)
}

func runGoalAcceptRiskWithAuthority(args []string, prove goalAuthorityProver) int {
	f, ok := parseSyncFlags("accept-risk", args)
	if ok {
		f.why = strings.TrimSpace(f.why)
	}
	if !ok || f.id == "" || f.finding == "" || f.chain == "" || f.by == "" || f.why == "" {
		fmt.Fprintln(os.Stderr, "goal accept-risk needs --id, --finding, --chain, --by, and --why")
		return 2
	}
	if (f.temporaryWord == "") != (f.reviewBy == "") {
		fmt.Fprintln(os.Stderr, "--temporary-human-word and --review-by travel together")
		return 2
	}
	classification, err := classifyGoalAuthorityFirst("accept-risk", f)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	var finding dispatchcore.CritiqueDecisionFinding
	if f.chain != goal.HumanCarriedChain {
		finding, err = dispatchcore.CritiqueRegisterDecisionFinding(f.root, f.chain, f.finding, f.id)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	proof, err := proveGoalHumanAuthority("accept-risk", f, prove)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	req, err := syncReqClassified(f.root, f.by, f.lineage, &proof, classification)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	opid := goal.Opid(req.Ulid, req.Actor.Machine, req.Actor.Lineage)
	var carriedRisk counselor.CarriedAcceptedRiskAppend
	if f.chain == goal.HumanCarriedChain {
		commit, commitErr := humanCarriedFindingCommit(f.finding)
		if commitErr != nil {
			fmt.Fprintln(os.Stderr, commitErr)
			return 1
		}
		carriedRisk = counselor.CarriedAcceptedRiskAppend{Goal: f.id, Finding: f.finding, By: f.by, Why: f.why, OpID: opid, Commit: commit, RecordedAt: req.Now}
		if err := counselor.ValidateCarriedAcceptedRisk(f.root, carriedRisk); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	res, err := goal.AcceptedRiskDecision(req, f.id, f.finding, f.chain, f.by, f.why, &proof)
	if err != nil {
		return printSyncResult(res, err)
	}
	opid, err = goal.AcceptedRiskDecisionOpID(f.root, f.id, f.finding, f.chain, req.Now)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if f.chain == goal.HumanCarriedChain {
		carriedRisk.OpID = opid
		if err := counselor.AppendCarriedAcceptedRisk(f.root, carriedRisk); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	} else {
		if err := counselor.AppendAcceptedRisk(f.root, counselor.AcceptedRiskAppend{Goal: f.id, RootJob: f.chain, FindingID: f.finding, Class: finding.RigorClass, Title: finding.Title, Claim: finding.Claim, Evidence: finding.Evidence, Why: f.why, OpID: opid, RecordedAt: req.Now}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if err := dispatchcore.CritiqueRegisterAcceptRisk(f.root, f.chain, f.finding, opid); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	if err := recordGoalApprovalProof(f.root, opid, "goal accept-risk", proof); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return printSyncResult(res, nil)
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

// trySyncMutation intercepts a legacy mutation command on a
// CONVERTED checkout and routes it to the engine verb. Returns
// handled=false on a legacy checkout so the caller proceeds
// unchanged.
func trySyncMutation(name string, args []string) (int, bool) {
	f, ok := parseSyncFlags(name, args)
	if !ok {
		return 2, true
	}
	if !converted(f.root) {
		return 0, false
	}
	req, err := syncReq(name, f.root, f.by, f.lineage)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1, true
	}
	req.ApprovedRef = f.approvedRef
	need := func(val, flagName string) bool {
		if val == "" {
			fmt.Fprintf(os.Stderr, "goal %s needs --%s\n", name, flagName)
			return false
		}
		return true
	}
	switch name {
	case "open":
		if len(f.unlabels) > 0 {
			fmt.Fprintln(os.Stderr, "goal open does not accept --unlabel; remove labels with goal edit")
			return 2, true
		}
		if f.risk == "" || strings.TrimSpace(f.basis) == "" {
			fmt.Fprintln(os.Stderr, "answer the four questions: --risk severity=,novelty=,exposure=,accumulation= --basis")
			return 2, true
		}
		if !need(f.id, "id") || !need(f.intent, "intent") || !need(f.next, "next") || f.tier > 3 {
			return 2, true
		}
		risk, riskErr := goal.ParseRiskRecord(f.risk, f.basis)
		if riskErr != nil {
			fmt.Fprintln(os.Stderr, riskErr)
			return 2, true
		}
		var proof *humanauthority.Proof
		if f.tier != 0 && uint8(f.tier) < risk.DerivedTier() {
			proven, proofErr := proveGoalHumanAuthority("open", f, humanauthority.ProveOrTemporaryGoalAuthority)
			if proofErr != nil {
				fmt.Fprintln(os.Stderr, proofErr)
				return 1, true
			}
			proof = &proven
		}
		if f.claim {
			budget, budgetErr := f.budgetTuple(true)
			if budgetErr != nil {
				fmt.Fprintln(os.Stderr, budgetErr)
				return 2, true
			}
			if detail := brain.Fence(f.root, "claim", goal.ExistingLedgerIdentity(f.root)); detail != "" {
				fmt.Fprintln(os.Stderr, detail)
				return 1, true
			}
			res, err := goal.OpenClaim(req, f.id, f.intent, f.origin, f.next, *budget, f.labels...)
			return printSyncResult(res, err), true
		}
		budget, budgetErr := f.budgetTuple(false)
		if budgetErr != nil {
			fmt.Fprintln(os.Stderr, budgetErr)
			return 2, true
		}
		res, err := goal.OpenRisked(req, f.id, f.intent, f.origin, f.next, f.blocks, risk, uint8(f.tier), f.why, budget, proof, f.labels...)
		return printSyncResult(res, err), true
	case "park":
		if !need(f.id, "id") || !need(f.because, "because") {
			return 2, true
		}
		if f.arc != "" {
			res, err := goal.ParkArc(req, f.id, f.because)
			return printSyncResult(res, err), true
		}
		res, err := goal.Park(req, f.id, f.because)
		return printSyncResult(res, err), true
	case "unpark":
		if !need(f.id, "id") {
			return 2, true
		}
		if f.arc != "" {
			res, err := goal.UnparkArc(req, f.id)
			return printSyncResult(res, err), true
		}
		res, err := goal.Unpark(req, f.id)
		return printSyncResult(res, err), true
	case "done":
		if !need(f.id, "id") || !need(f.conclude, "conclude") {
			return 2, true
		}
		res, err := goal.Done(req, f.id, f.conclude)
		code := printSyncResult(res, err)
		return reportAfterConfirmedDone(code, f.root, f.id, os.Stderr), true
	case "reopen":
		if !need(f.id, "id") {
			return 2, true
		}
		res, err := goal.Reopen(req, f.id)
		return printSyncResult(res, err), true
	case "prune":
		res, err := goal.Prune(req, f.keep)
		return printSyncResult(res, err), true
	case "set-next":
		if !need(f.id, "id") || !need(f.next, "next") {
			return 2, true
		}
		res, err := goal.Edit(req, f.id, goal.EditFields{NextStep: &f.next})
		return printSyncResult(res, err), true
	case "promote":
		fmt.Fprintln(os.Stderr, "the synced backlog has no Current slot to promote into; claim the goal instead (goal claim --id ...)")
		return 1, true
	case "declare-free":
		if !need(f.digest, "digest") {
			return 2, true
		}
		res, err := goal.DeclareFree(req, f.origin, f.digest)
		return printSyncResult(res, err), true
	case "reconcile":
		if f.refreshOnly {
			skipped, err := goal.RefreshOnly(f.root)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1, true
			}
			printJSON(map[string]any{"outcome": "confirmed", "skipped": skipped})
			return 0, true
		}
		res, err := goal.Reconcile(req)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1, true
		}
		printJSON(map[string]any{"outcome": res.Publish.Outcome, "tip": res.Publish.Tip, "rows": len(res.Rows), "skipped": res.Skipped})
		if res.Publish.Outcome != goal.OutcomeConfirmed && len(res.Rows) > 0 {
			return 1, true
		}
		return 0, true
	}
	fmt.Fprintf(os.Stderr, "goal %s has no synced-world route\n", name)
	return 1, true
}

// runSyncOnly wraps the verbs that exist ONLY in the synced world.
func runSyncOnly(name string, run func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error), required ...string) func([]string) int {
	return func(args []string) int {
		f, ok := parseSyncFlags(name, args)
		if !ok {
			return 2
		}
		if !converted(f.root) {
			fmt.Fprintf(os.Stderr, "goal %s works the synced backlog; this checkout still carries the legacy ledger\n", name)
			return 1
		}
		for _, r := range required {
			if r == "id" && f.id == "" {
				fmt.Fprintf(os.Stderr, "goal %s needs --id\n", name)
				return 2
			}
			if r == "arc" && f.arc == "" {
				fmt.Fprintf(os.Stderr, "goal %s needs --arc\n", name)
				return 2
			}
			if r == "pin" && f.pin == "" {
				fmt.Fprintf(os.Stderr, "goal %s needs --pin (a machine nickname, or - to clear)\n", name)
				return 2
			}
		}
		req, err := syncReq(name, f.root, f.by, f.lineage)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		req.ApprovedRef = f.approvedRef
		res, runErr := run(req, f)
		code := printSyncResult(res, runErr)
		return code
	}
}

type goalAuthorityProver func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error)

func proveFixtureGoalAuthority(name string, f *syncFlags) (humanauthority.Proof, error) {
	ancestryNow, err := goalCommandNow(f.root)
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
	if f.fixtureHumanAuthority {
		if f.temporaryWord != "" || f.reviewBy != "" {
			return humanauthority.Proof{}, fmt.Errorf("goal %s fixture authority does not combine with a temporary human word or review date", name)
		}
		return proveFixtureGoalAuthority(name, f)
	}
	ancestryNow, err := goalCommandNow(f.root)
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

func runGoalSetPriority(args []string) int {
	return runGoalSetPriorityWithAuthority(args, proveEnrolledGoalHumanAuthority)
}

func runGoalSetPriorityWithAuthority(args []string, prove goalAuthorityProver) int {
	flags := flag.NewFlagSet("goal set-priority", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
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
		fmt.Fprintln(os.Stderr, "goal set-priority takes no positional arguments")
		return 2
	}
	if *id == "" || *by == "" || !prioritySet {
		fmt.Fprintln(os.Stderr, "goal set-priority needs --id, --by, and --priority")
		return 2
	}
	priorityValue, err := parseGoalRankDecimal("priority", priorityRaw, 8)
	if err != nil || priorityValue < 1 || priorityValue > 3 {
		fmt.Fprintf(os.Stderr, "goal set-priority priority %q is not 1, 2, or 3\n", priorityRaw)
		return 2
	}
	var sequence *uint64
	if sequenceSet {
		sequenceValue, parseErr := parseGoalRankDecimal("sequence", sequenceRaw, 64)
		if parseErr != nil || sequenceValue == 0 {
			fmt.Fprintf(os.Stderr, "goal set-priority sequence %q is not a positive unsigned 64-bit integer\n", sequenceRaw)
			return 2
		}
		sequence = &sequenceValue
	}
	if !converted(*root) {
		fmt.Fprintln(os.Stderr, "goal set-priority works only with the synced backlog; migrate this checkout first")
		return 1
	}
	shared := &syncFlags{root: *root, id: *id, by: *by, lineage: *lineage, fixtureHumanAuthority: *fixtureAuthority}
	proof, err := proveGoalHumanAuthority("set-priority", shared, prove)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	request, err := syncReqWithProof("set-priority", *root, *by, *lineage, &proof)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	result, err := goal.SetPriority(request, *id, uint8(priorityValue), sequence, &proof)
	return printSyncResult(result, err)
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
	fs := flag.NewFlagSet("goal classify-sweep", flag.ContinueOnError)
	root := fs.String("root", ".", "checkout root")
	draftPath := fs.String("draft", "", "classification draft file")
	preview := fs.Bool("preview", false, "print the normalized listing without mutation")
	confirm := fs.String("confirm", "", "sha256 of the normalized preview listing")
	by := fs.String("by", "", "the directing human")
	lineage := fs.String("lineage", "", "this coordinator's lineage")
	if fs.Parse(args) != nil {
		return 2
	}
	if !converted(*root) || *draftPath == "" || (*preview == (*confirm != "")) || (!*preview && *by == "") {
		fmt.Fprintln(os.Stderr, "goal classify-sweep needs a synced backlog, --draft, and exactly one of --preview or --confirm; confirmation also needs --by")
		return 2
	}
	draft, err := os.ReadFile(*draftPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "goal classify-sweep could not read its draft:", err)
		return 1
	}
	endpoint, err := goal.ResolveEndpoint(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	now, err := goalCommandNow(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	listing, err := goal.PreviewClassificationSweep(endpoint, draft, now)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *preview {
		for _, line := range listing.Lines {
			fmt.Println(line)
		}
		fmt.Println("listing-digest " + listing.Digest)
		return 0
	}
	if listing.Digest != *confirm {
		fmt.Fprintf(os.Stderr, "SWEEP_LISTING_CHANGED: confirmation %s does not match current listing %s; preview again\n", *confirm, listing.Digest)
		return 1
	}
	if len(listing.Proposals) == 0 {
		if listing.TierLawInstalled {
			printJSON(map[string]any{"outcome": goal.OutcomeConfirmed, "detail": "the tier law is already installed and no tierless goals remain"})
			return 0
		}
		req, reqErr := syncReq("classify-sweep", *root, *by, *lineage)
		if reqErr != nil {
			fmt.Fprintln(os.Stderr, reqErr)
			return 1
		}
		res, installErr := goal.InstallTierLaw(req)
		if installErr != nil || res.Outcome != goal.OutcomeConfirmed {
			return printSyncResult(res, installErr)
		}
		printJSON(map[string]any{"outcome": goal.OutcomeConfirmed, "classified": 0, "listingDigest": listing.Digest})
		return 0
	}
	for index, proposal := range listing.Proposals {
		req, reqErr := syncReq("classify-sweep", *root, *by, *lineage)
		if reqErr != nil {
			fmt.Fprintln(os.Stderr, reqErr)
			return 1
		}
		res, classifyErr := goal.ClassifyTier(req, proposal, index == len(listing.Proposals)-1)
		if classifyErr != nil {
			return printSyncResult(res, classifyErr)
		}
		if res.Outcome != goal.OutcomeConfirmed {
			return printSyncResult(res, nil)
		}
	}
	printJSON(map[string]any{"outcome": goal.OutcomeConfirmed, "classified": len(listing.Proposals), "listingDigest": listing.Digest})
	return 0
}

func runGoalApproveWithAuthority(args []string, prove goalAuthorityProver) int {
	f, ok := parseSyncFlags("approve", args)
	if !ok {
		return 2
	}
	if !converted(f.root) {
		fmt.Fprintln(os.Stderr, "goal approve works only with the synced backlog")
		return 1
	}
	if f.sweep && len(f.ids) != 0 || !f.sweep && f.confirm != "" {
		fmt.Fprintln(os.Stderr, "goal approve uses either repeatable --id or --sweep; --confirm belongs only to the sweep")
		return 2
	}
	if f.sweep && f.hasAnyBudgetFlag() || f.sweep && f.budgetBox != "" || f.sweep && f.approvedRef != "" {
		fmt.Fprintln(os.Stderr, "goal approve --sweep uses the tuples already listed and takes no budget or --approved-ref")
		return 2
	}
	if f.sweep && f.confirm == "" {
		e, err := goal.ResolveEndpoint(f.root)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		now, err := goalCommandNow(f.root)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		listing, err := goal.PreviewApprovalSweep(e, now)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
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
			fmt.Fprintln(os.Stderr, "goal approve --under takes repeatable --id and no --sweep")
			return 2
		}
		return runGoalUnderAttorney("approve", f)
	}
	if f.by == "" || (!f.sweep && len(f.ids) == 0) {
		fmt.Fprintln(os.Stderr, "goal approve needs --by and either repeatable --id or --sweep")
		return 2
	}
	budget, err := f.approvalBudget()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	classification, err := classifyGoalAuthorityFirst("approve", f)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	proof, err := proveGoalHumanAuthority("approve", f, prove)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	req, err := syncReqClassified(f.root, f.by, f.lineage, &proof, classification)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	req.ApprovedRef = f.approvedRef
	var res goal.PublishResult
	if f.sweep {
		res, err = goal.ApproveSweep(req, f.confirm, &proof)
	} else {
		res, err = goal.Approve(req, f.ids, budget, &proof)
	}
	if err != nil {
		return printSyncResult(res, err)
	}
	if res.Outcome == goal.OutcomeConfirmed {
		action := "goal approve"
		if f.sweep {
			action = "goal approve --sweep"
		}
		if err := recordGoalApprovalProof(f.root, goal.Opid(req.Ulid, req.Actor.Machine, req.Actor.Lineage), action, proof); err != nil {
			fmt.Fprintln(os.Stderr, "goal approve confirmed but could not record its authority proof:", err)
			return 1
		}
		if proof.TemporaryResumeFor(f.root) {
			fmt.Printf("goal approve: TEMPORARY authority under a recorded relayed word (human provenance not verified); re-approval due %s at an agent-free terminal\n", f.reviewBy)
		}
	}
	return printSyncResult(res, nil)
}

func runGoalUnapprove(args []string) int {
	return runGoalUnapproveWithAuthority(args, humanauthority.ProveOrTemporaryGoalAuthority)
}

func runGoalUnapproveWithAuthority(args []string, prove goalAuthorityProver) int {
	f, ok := parseSyncFlags("unapprove", args)
	if !ok {
		return 2
	}
	if !converted(f.root) || f.id == "" || f.by == "" || f.because == "" {
		fmt.Fprintln(os.Stderr, "goal unapprove needs a synced backlog plus --id, --by, and --because")
		return 2
	}
	classification, err := classifyGoalAuthorityFirst("unapprove", f)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	proof, err := proveGoalHumanAuthority("unapprove", f, prove)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	req, err := syncReqClassified(f.root, f.by, f.lineage, &proof, classification)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	res, err := goal.Unapprove(req, f.id, f.because, &proof)
	if err != nil {
		return printSyncResult(res, err)
	}
	if res.Outcome == goal.OutcomeConfirmed {
		if err := recordGoalApprovalProof(f.root, goal.Opid(req.Ulid, req.Actor.Machine, req.Actor.Lineage), "goal unapprove", proof); err != nil {
			fmt.Fprintln(os.Stderr, "goal unapprove confirmed but could not record its authority proof:", err)
			return 1
		}
		if proof.TemporaryResumeFor(f.root) {
			fmt.Printf("goal unapprove: TEMPORARY authority under a recorded relayed word (human provenance not verified); re-approval due %s at an agent-free terminal\n", f.reviewBy)
		}
	}
	return printSyncResult(res, nil)
}

func runGoalSetBudget(args []string) int {
	return runGoalSetBudgetWithAuthority(args, humanauthority.ProveOrTemporaryGoalAuthority)
}

func runGoalSetBudgetWithAuthority(args []string, prove goalAuthorityProver) int {
	f, ok := parseSyncFlags("set-budget", args)
	if !ok {
		return 2
	}
	if f.under != "" {
		if !converted(f.root) || f.id == "" {
			fmt.Fprintln(os.Stderr, "goal set-budget --under needs a synced backlog plus --id")
			return 2
		}
		return runGoalUnderAttorney("set-budget", f)
	}
	if !converted(f.root) || f.id == "" || f.by == "" {
		fmt.Fprintln(os.Stderr, "goal set-budget needs a synced backlog plus --id and --by")
		return 2
	}
	budget, err := f.budgetTuple(true)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	classification, err := classifyGoalAuthorityFirst("set-budget", f)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	proof, err := proveGoalHumanAuthority("set-budget", f, prove)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	req, err := syncReqClassified(f.root, f.by, f.lineage, &proof, classification)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	req.ApprovedRef = f.approvedRef
	res, err := goal.SetBudgetApproved(req, f.id, *budget, &proof)
	if err != nil {
		return printSyncResult(res, err)
	}
	if res.Outcome == goal.OutcomeConfirmed {
		if err := recordGoalApprovalProof(f.root, goal.Opid(req.Ulid, req.Actor.Machine, req.Actor.Lineage), "goal set-budget", proof); err != nil {
			fmt.Fprintln(os.Stderr, "goal set-budget confirmed but could not record its authority proof:", err)
			return 1
		}
		if proof.TemporaryResumeFor(f.root) {
			fmt.Printf("goal set-budget: TEMPORARY authority under a recorded relayed word (human provenance not verified); re-approval due %s at an agent-free terminal\n", f.reviewBy)
		}
	}
	return printSyncResult(res, nil)
}

// runGoalUnderAttorney runs approve or set-budget as the seat's own act under
// a recorded power of attorney: no --by, no human proof, the entry resolved
// from the accepted tree and checked again inside the transaction.
func runGoalUnderAttorney(name string, f *syncFlags) int {
	if f.by != "" || f.fixtureHumanAuthority || f.temporaryWord != "" || f.reviewBy != "" || f.approvedRef != "" {
		fmt.Fprintf(os.Stderr, "goal %s --under is the seat's own act: it combines with neither --by, a human proof, nor --approved-ref\n", name)
		return 2
	}
	if brainState := brain.Read(f.root, goal.ExistingLedgerIdentity(f.root)); brainState.State != brain.Undeclared {
		fmt.Fprintf(os.Stderr, "this checkout is declared the brain; the brain never carries a human's word into goal %s, a power of attorney included. Wido runs, from an agent-free terminal: metasystem goal %s --root <checkout> --id <id> --by <name> <the verb's own flags>\n", name, name)
		return 1
	}
	now, err := goalCommandNow(f.root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	entry, err := goal.ResolveAttorney(f.root, f.under, name, now)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	req, err := syncReq(name, f.root, "", f.lineage)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	req.Attorney = &entry
	var res goal.PublishResult
	switch name {
	case "approve":
		budget, budgetErr := f.approvalBudget()
		if budgetErr != nil {
			fmt.Fprintln(os.Stderr, budgetErr)
			return 2
		}
		res, err = goal.Approve(req, f.ids, budget, nil)
	case "set-budget":
		budget, budgetErr := f.budgetTuple(true)
		if budgetErr != nil {
			fmt.Fprintln(os.Stderr, budgetErr)
			return 2
		}
		res, err = goal.SetBudgetApproved(req, f.id, *budget, nil)
	}
	return printSyncResult(res, err)
}

func runGoalGrant(args []string) int {
	return runGoalGrantWithAuthority(args, humanauthority.ProveOrTemporaryGoalAuthority)
}

// runGoalGrantWithAuthority records a power of attorney under the human's
// own proof and prints the entry id the seat will name with --under.
func runGoalGrantWithAuthority(args []string, prove goalAuthorityProver) int {
	f, ok := parseSyncFlags("grant", args)
	if !ok {
		return 2
	}
	if !converted(f.root) || f.by == "" || f.tiers == "" || f.verbs == "" || f.expires == "" {
		fmt.Fprintln(os.Stderr, "goal grant needs a synced backlog plus --by, --tiers, --verbs and --expires")
		return 2
	}
	tiers, err := goal.ParseTiers(f.tiers)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	var verbs []string
	for _, verb := range strings.Split(f.verbs, ",") {
		if verb = strings.TrimSpace(verb); verb != "" {
			verbs = append(verbs, verb)
		}
	}
	classification, err := classifyGoalAuthorityFirst("grant", f)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	proof, err := proveGoalHumanAuthority("grant", f, prove)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	req, err := syncReqClassified(f.root, f.by, f.lineage, &proof, classification)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	res, err := goal.Grant(req, &proof, tiers, verbs, f.expires)
	if err != nil {
		return printSyncResult(res, err)
	}
	opid := goal.Opid(req.Ulid, req.Actor.Machine, req.Actor.Lineage)
	if res.Outcome == goal.OutcomeConfirmed {
		if err := recordGoalApprovalProof(f.root, opid, "goal grant", proof); err != nil {
			fmt.Fprintln(os.Stderr, "goal grant confirmed but could not record its authority proof:", err)
			return 1
		}
	}
	printJSON(map[string]any{"outcome": res.Outcome, "tip": res.Tip, "entry": opid, "detail": res.Detail})
	if res.Outcome != goal.OutcomeConfirmed {
		return 1
	}
	return 0
}

func runGoalRevoke(args []string) int {
	return runGoalRevokeWithAuthority(args, humanauthority.ProveOrTemporaryGoalAuthority)
}

func runGoalRevokeWithAuthority(args []string, prove goalAuthorityProver) int {
	f, ok := parseSyncFlags("revoke", args)
	if !ok {
		return 2
	}
	if !converted(f.root) || f.by == "" || f.id == "" {
		fmt.Fprintln(os.Stderr, "goal revoke needs a synced backlog plus --by and --id <entry>")
		return 2
	}
	classification, err := classifyGoalAuthorityFirst("revoke", f)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	proof, err := proveGoalHumanAuthority("revoke", f, prove)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	req, err := syncReqClassified(f.root, f.by, f.lineage, &proof, classification)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	res, err := goal.Revoke(req, &proof, f.id)
	if err != nil {
		return printSyncResult(res, err)
	}
	if res.Outcome == goal.OutcomeConfirmed {
		if err := recordGoalApprovalProof(f.root, goal.Opid(req.Ulid, req.Actor.Machine, req.Actor.Lineage), "goal revoke", proof); err != nil {
			fmt.Fprintln(os.Stderr, "goal revoke confirmed but could not record its authority proof:", err)
			return 1
		}
	}
	return printSyncResult(res, nil)
}

func runGoalResume(args []string) int {
	return runGoalResumeWithAuthority(args, humanauthority.ProveOrTemporaryGoalAuthority)
}

// runGoalResumeWithAuthority gives package tests a direct, non-CLI seam for an
// already-granted proof. The shipped command always enters through
// runGoalResume, whose authority owner reads the real wall clock.
func runGoalResumeWithAuthority(args []string, prove goalAuthorityProver) int {
	f, ok := parseSyncFlags("resume", args)
	if !ok {
		return 2
	}
	if !converted(f.root) {
		fmt.Fprintln(os.Stderr, "goal resume works the synced backlog; this checkout still carries the legacy ledger")
		return 1
	}
	if f.id == "" || f.by == "" {
		fmt.Fprintln(os.Stderr, "goal resume needs --id and --by")
		return 2
	}
	budget, err := f.budgetTuple(true)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	classification, err := classifyGoalAuthorityFirst("resume", f)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ancestryNow, err := goalCommandNow(f.root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	var proof humanauthority.Proof
	if f.approvedRef != "" {
		recorded, approvalErr := goal.AuthenticatedChannelApproval(f.root, f.id, f.approvedRef, goal.ResumeApprovalToken(f.id, *budget), ancestryNow)
		if approvalErr != nil {
			fmt.Fprintln(os.Stderr, "goal resume could not validate --approved-ref:", approvalErr)
			return 1
		}
		proof, err = humanauthority.AuthenticatedChannelProof(f.root, recorded, ancestryNow)
	} else {
		proof, err = prove(f.root, int64(os.Getppid()), nil, f.temporaryWord, f.reviewBy, ancestryNow)
	}
	if err != nil {
		if f.temporaryWord == "" && f.reviewBy == "" {
			fmt.Fprintln(os.Stderr, "goal resume could not prove enrolled human ancestry:", err)
			return 1
		}
		fmt.Fprintln(os.Stderr, "goal resume could not bind its temporary recorded relay:", err)
		return 2
	}
	temporaryAuthority := proof.TemporaryResumeFor(f.root)
	req, err := syncReqClassified(f.root, f.by, f.lineage, &proof, classification)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	req.ApprovedRef = f.approvedRef
	binding, err := dispatchcore.ResolveGoalBinding(f.root, f.id, req.Now)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if binding.Fence == nil {
		fmt.Fprintf(os.Stderr, "goal %s revision %d is not breach-stopped\n", f.id, binding.Revision)
		return 1
	}
	held, err := goalrevision.Acquire(f.root, f.id, binding.Revision, "goal-resume")
	if err != nil {
		fmt.Fprintln(os.Stderr, "goal resume could not acquire the goal-revision lock:", err)
		return 1
	}
	defer held.Release()
	res, err := goal.Resume(goal.ResumeRequest{VerbRequest: req, GoalID: f.id, Budget: *budget, Authority: &proof})
	if err == nil && res.Outcome == goal.OutcomeConfirmed {
		if err := humanauthority.RecordResumeProof(f.root, goal.Opid(req.Ulid, req.Actor.Machine, req.Actor.Lineage), proof); err != nil {
			fmt.Fprintln(os.Stderr, "goal resume confirmed but could not record its authority proof:", err)
			return 1
		}
	}
	if err == nil && res.Outcome == goal.OutcomeConfirmed && temporaryAuthority {
		fmt.Printf("goal resume: TEMPORARY authority under a recorded relayed word (human provenance not verified); re-approval due %s at an agent-free terminal\n", f.reviewBy)
	}
	return printSyncResult(res, err)
}

func runGoalSplit(args []string) int {
	f, ok := parseSyncFlags("split", args)
	if !ok {
		return 2
	}
	if !converted(f.root) {
		fmt.Fprintln(os.Stderr, "goal split works the synced backlog; this checkout still carries the legacy ledger")
		return 1
	}
	if f.id == "" || f.members == "" {
		fmt.Fprintln(os.Stderr, "goal split needs --id and --members")
		return 2
	}
	draftBytes, err := os.ReadFile(f.members)
	if err != nil {
		fmt.Fprintln(os.Stderr, "goal split could not read its member draft:", err)
		return 1
	}
	members, err := goal.ParseMemberDraft(draftBytes, f.id)
	if err != nil {
		fmt.Fprintln(os.Stderr, "goal split draft refused:", err)
		return 1
	}
	req, err := syncReq("split", f.root, f.by, f.lineage)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	projection, err := goal.Project(req.Endpoint, false, req.Now)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	parent := projection.Tree.Live[f.id]
	if parent == nil {
		if projection.Tree.Done[f.id] != nil {
			fmt.Fprintf(os.Stderr, "goal %s is in the archive; there is nothing to split\n", f.id)
		} else {
			fmt.Fprintf(os.Stderr, "goal %s does not exist\n", f.id)
		}
		return 1
	}
	digest := goal.SplitDraftSHA256(f.id, members)
	var proof *humanauthority.Proof
	var ratification goal.SplitRatification
	if f.by != "" {
		observed, proofErr := humanauthority.Prove(f.root, int64(os.Getppid()), nil, time.Now().UTC())
		if proofErr != nil {
			fmt.Fprintln(os.Stderr, "SPLIT_RATIFY_REFUSED: goal split could not prove enrolled human ancestry:", proofErr)
			return 1
		}
		proof = &observed
		ratification = goal.SplitRatification{Tier: goal.RatifierHuman, By: f.by, DraftSHA256: digest}
	} else {
		classification, classErr := classifyVerbCaller(f.root, int64(os.Getppid()))
		if classErr != nil {
			fmt.Fprintln(os.Stderr, "SPLIT_RATIFY_REFUSED: caller classification failed:", classErr)
			return 1
		}
		if parent.Origin != goal.OriginMain {
			fmt.Fprintf(os.Stderr, "SPLIT_RATIFY_REFUSED: goal %s's split draft must be ratified by its origin tier — use --by from the enrolled human terminal for human-origin work, or run from the MAIN checkout-lease holder for main-origin work\n", f.id)
			return 1
		}
		ratification, err = mainSplitRatification(f.id, digest, classification)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}

	var held *goalrevision.Held
	if parent.State == goal.StateClaimed && parent.Claimed != nil && parent.Claimed.Machine == req.Actor.Machine && parent.Claimed.Lineage == req.Actor.Lineage {
		held, err = goalrevision.Acquire(f.root, f.id, parent.Claimed.Revision, "goal-split")
		if err != nil {
			fmt.Fprintln(os.Stderr, "goal split could not acquire the goal-revision lock:", err)
			return 1
		}
		defer held.Release()
		spend := dispatchcore.ProjectBudget(f.root, parent, req.Now)
		if spend.Status != dispatchcore.BudgetKnown {
			detail := "unknown spending evidence"
			if spend.Unknown != nil {
				detail = spend.Unknown.Record + ": " + spend.Unknown.Reason
			}
			fmt.Fprintf(os.Stderr, "goal %s revision %d cannot prove zero work: %s\n", f.id, parent.Claimed.Revision, detail)
			return 1
		}
		if spend.Attempts != 0 || spend.ActiveJobs != 0 || spend.ReservedJobMinutes != 0 {
			fmt.Fprintf(os.Stderr, "goal %s revision %d has recorded work (%d attempts, %d active jobs, %d reserved minutes); split is a before-slicing act — conclude the slice or take the parent to the human\n", f.id, parent.Claimed.Revision, spend.Attempts, spend.ActiveJobs, spend.ReservedJobMinutes)
			return 1
		}
	}
	if proof != nil {
		if err := humanauthority.RecordProof(f.root, goal.Opid(req.Ulid, req.Actor.Machine, req.Actor.Lineage), "goal split", *proof); err != nil {
			fmt.Fprintln(os.Stderr, "goal split could not record its authority proof:", err)
			return 1
		}
	}
	res, err := goal.Split(req, f.id, members, ratification, proof)
	return printSyncResult(res, err)
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

type goalTerminalEnroller func(string, int64, humanauthority.Reader, time.Time) (humanauthority.Enrollment, error)

func runGoalEnrollTerminal(args []string) int {
	return runGoalEnrollTerminalWith(args, humanauthority.Enroll)
}

func runGoalEnrollTerminalWith(args []string, enroll goalTerminalEnroller) int {
	flags := flag.NewFlagSet("goal enroll-terminal", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
	lineage := flags.String("lineage", "", "coordinator lineage used to publish the fleet enrollment")
	if flags.Parse(args) != nil {
		return 2
	}
	if !converted(*root) {
		fmt.Fprintln(os.Stderr, "goal enroll-terminal requires the synced backlog so the first enrollment ends relayed approval fleet-wide")
		return 1
	}
	enrollment, err := enroll(*root, int64(os.Getppid()), nil, time.Now().UTC())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	req, err := syncReq("enroll-terminal", *root, "terminal", *lineage)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	req.Now = enrollment.EnrolledAt
	res, err := goal.RecordFleetEnrollment(req, enrollment.Generation)
	// Another machine may already own the immutable fleet cutoff. That leaves
	// this machine's completed local enrollment valid and needs no root rewrite.
	if err != nil || (res.Outcome != goal.OutcomeConfirmed && res.Outcome != goal.OutcomeConfirmedLate && res.Outcome != goal.OutcomeAbandoned) {
		fmt.Fprintln(os.Stderr, "terminal enrolled locally but its fleet cutoff did not publish:", err, res.Detail)
		return 1
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
	flags := flag.NewFlagSet("goal set-obligation", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
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
	if flags.Parse(args) != nil {
		return 2
	}
	if flags.NArg() != 0 || *id == "" || *by == "" || *state == "" || *owner == "" || *recurrence == "" ||
		*platform == "" || *toolchain == "" || *surface == "" || *maxActiveJobs == 0 || *timingEnvelope == 0 ||
		len(effects) == 0 || *valueJudgment == "" || *reversibility == "" || *severeHarm == "" ||
		*unfamiliarApproach == "" || *testDiscrimination == "" || *correlatedRisk == "" || *authorityScopeChange == "" || *destructiveReach == "" {
		fmt.Fprintln(os.Stderr, "goal set-obligation requires identity, recurrence, platform/toolchain/surface observations, active/timing ceilings, effects, and every typed review trigger")
		return 2
	}
	if !converted(*root) {
		fmt.Fprintln(os.Stderr, "goal set-obligation works only with the synced backlog")
		return 1
	}
	classification, err := brainHumanWordClassification("set-obligation", *root, *by, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ancestryNow, err := goalCommandNow(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	var proof humanauthority.Proof
	if *approvedRef != "" {
		recorded, approvalErr := goal.AuthenticatedChannelApproval(*root, *id, *approvedRef, goal.SetObligationApprovalToken(*id, goal.ObligationState(*state), *owner), ancestryNow)
		if approvalErr != nil {
			fmt.Fprintln(os.Stderr, "goal set-obligation could not validate --approved-ref:", approvalErr)
			return 1
		}
		proof, err = humanauthority.AuthenticatedChannelProof(*root, recorded, ancestryNow)
	} else {
		proof, err = prove(*root, int64(os.Getppid()), nil, *temporaryWord, *reviewBy, ancestryNow)
	}
	if err != nil {
		if *temporaryWord == "" && *reviewBy == "" {
			fmt.Fprintln(os.Stderr, "goal set-obligation could not prove enrolled human ancestry:", err)
			return 1
		}
		fmt.Fprintln(os.Stderr, "goal set-obligation could not bind its temporary recorded relay:", err)
		return 2
	}
	temporaryAuthority := proof.TemporarySetObligationFor(*root)
	req, err := syncReqClassified(*root, *by, *lineage, &proof, classification)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
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
	if err != nil {
		return printSyncResult(res, err)
	}
	if res.Outcome == goal.OutcomeConfirmed {
		if err := humanauthority.RecordSetObligationProof(*root, operationID, proof); err != nil {
			fmt.Fprintln(os.Stderr, "goal set-obligation could not record its authority proof:", err)
			return 1
		}
	}
	if res.Outcome == goal.OutcomeConfirmed && temporaryAuthority {
		fmt.Printf("goal set-obligation: TEMPORARY authority under a recorded relayed word (human provenance not verified); re-approval due %s at an agent-free terminal\n", *reviewBy)
	}
	return printSyncResult(res, nil)
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
	runGoalRelease = runSyncOnly("release", func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		if f.arc != "" {
			return goal.ReleaseArc(req, f.id)
		}
		return goal.Release(req, f.id)
	}, "id")
	runGoalSteal = runSyncOnly("steal", func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		return goal.Steal(req, f.id)
	}, "id")
	runGoalLandReady = runSyncOnly("land-ready", func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		return goal.LandReady(req, f.id)
	}, "id")
	runGoalEdit = runSyncOnly("edit", func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		fields := goal.EditFields{Why: f.why, Evidence: f.evidence}
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
				proof, proofErr := proveGoalHumanAuthority("edit", f, humanauthority.ProveOrTemporaryGoalAuthority)
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
