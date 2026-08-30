package main

// The mutation surface of the SYNCED backlog: every command builds
// one VerbRequest and calls the engine verb whose authority rules
// decide — the CLI adds no policy of its own. On a checkout still
// carrying the legacy ledger these commands do not apply; the
// legacy wrappers own that world untouched.

import (
	"flag"
	"fmt"
	"os"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalrevision"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// syncReq assembles the one request every synced verb consumes. A
// mutation without an identity refuses: the silent "session"
// default minted claims under a generic lineage, and steal and
// succession then judged the wrong owner.
func syncReq(root, by, lineageFlag string) (goal.VerbRequest, error) {
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
		return goal.VerbRequest{}, fmt.Errorf("mutations carry their coordinator's identity: export METASYSTEM_OWNER_LINEAGE or pass --lineage")
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
		Endpoint: e,
		Actor:    goal.Actor{Machine: machine, Lineage: lineage, Human: by},
		Ulid:     ulid,
		Now:      now,
	}
	identity, classifyErr := lease.ClassifyVerb(root, int64(os.Getppid()))
	if classifyErr == nil {
		if identity.ClaimEpoch != nil && (identity.Holder || by != "" || identity.Class == lease.ClassHuman) {
			req.ClaimEpoch = *identity.ClaimEpoch
		} else if identity.Class == lease.ClassHuman {
			// A direct human may prepare the first claimed revision before a
			// checkout lease exists. The first lease generation is one, so the
			// claim and its future dispatch records still share one coordinate.
			req.ClaimEpoch = 1
		}
	}
	return req, nil
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
	root, by, id, intent, next, origin, because, conclude, arc, pin string
	lineage, digest, elapsedLimit                                   string
	attemptLimit, reservedJobMinutesLimit, activeJobLimit           int64
	labels, unlabels                                                repeatedStrings
	claim, refreshOnly                                              bool
	keep                                                            int
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
	fs.StringVar(&f.id, "id", "", "goal id")
	fs.StringVar(&f.intent, "intent", "", "one-line intent")
	fs.StringVar(&f.next, "next", "", "the next step")
	fs.StringVar(&f.origin, "origin", "main", "creation provenance: human|main")
	fs.StringVar(&f.because, "because", "", "the park's reason")
	fs.StringVar(&f.conclude, "conclude", "", "the conclusion")
	fs.StringVar(&f.arc, "arc", "", "the destination arc")
	fs.StringVar(&f.pin, "pin", "", "the machine nickname a goal is pinned to (\"-\" clears)")
	fs.StringVar(&f.lineage, "lineage", "", "this coordinator's lineage (or export METASYSTEM_OWNER_LINEAGE)")
	fs.StringVar(&f.digest, "digest", "", "the declaration's freshness digest (declare-free)")
	fs.StringVar(&f.elapsedLimit, "elapsed-limit", "", "positive elapsed duration, for example 4h")
	fs.Int64Var(&f.attemptLimit, "attempt-limit", 0, "positive reservation-attempt limit")
	fs.Int64Var(&f.reservedJobMinutesLimit, "reserved-job-minutes-limit", 0, "positive reserved job-minute limit")
	fs.Int64Var(&f.activeJobLimit, "active-job-limit", 0, "positive concurrent-job limit")
	fs.Var(&f.labels, "label", "label token (repeatable)")
	fs.Var(&f.unlabels, "unlabel", "label token to remove (repeatable; edit only)")
	fs.BoolVar(&f.claim, "claim", false, "claim on open")
	fs.BoolVar(&f.refreshOnly, "refresh-only", false, "complete a died refresh")
	fs.IntVar(&f.keep, "keep", 10, "archive entries to keep")
	if fs.Parse(args) != nil {
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
	}
	if name != "open" && name != "claim" && name != "set-budget" && name != "resume" && f.hasAnyBudgetFlag() {
		fmt.Fprintf(os.Stderr, "goal %s does not take budget flags\n", name)
		return nil, false
	}
	return f, true
}

func (f *syncFlags) hasAnyBudgetFlag() bool {
	return f.elapsedLimit != "" || f.attemptLimit != 0 || f.reservedJobMinutesLimit != 0 || f.activeJobLimit != 0
}

func (f *syncFlags) budgetTuple(required bool) (*goal.Budget, error) {
	if !f.hasAnyBudgetFlag() {
		if required {
			return nil, fmt.Errorf("the complete budget tuple is required: --elapsed-limit, --attempt-limit, --reserved-job-minutes-limit, and --active-job-limit")
		}
		return nil, nil
	}
	if f.elapsedLimit == "" || f.attemptLimit == 0 || f.reservedJobMinutesLimit == 0 || f.activeJobLimit == 0 {
		return nil, fmt.Errorf("budget flags are all-or-nothing: supply --elapsed-limit, --attempt-limit, --reserved-job-minutes-limit, and --active-job-limit")
	}
	budget, err := goal.NewBudget(f.elapsedLimit, f.attemptLimit, f.reservedJobMinutesLimit, f.activeJobLimit)
	if err != nil {
		return nil, err
	}
	return &budget, nil
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
	req, err := syncReq(f.root, f.by, f.lineage)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1, true
	}
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
		if !need(f.id, "id") || !need(f.intent, "intent") || !need(f.next, "next") {
			return 2, true
		}
		if f.claim {
			budget, budgetErr := f.budgetTuple(true)
			if budgetErr != nil {
				fmt.Fprintln(os.Stderr, budgetErr)
				return 2, true
			}
			res, err := goal.OpenClaim(req, f.id, f.intent, f.origin, f.next, *budget, f.labels...)
			return printSyncResult(res, err), true
		}
		if f.hasAnyBudgetFlag() {
			fmt.Fprintln(os.Stderr, "goal open accepts a budget only with --claim; otherwise open the goal and use goal set-budget")
			return 2, true
		}
		res, err := goal.Open(req, f.id, f.intent, f.origin, f.next, f.labels...)
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
		req, err := syncReq(f.root, f.by, f.lineage)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		res, runErr := run(req, f)
		code := printSyncResult(res, runErr)
		return code
	}
}

func runGoalResume(args []string) int {
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
	proof, err := humanauthority.Prove(f.root, int64(os.Getppid()), nil, time.Now().UTC())
	if err != nil {
		fmt.Fprintln(os.Stderr, "goal resume could not prove enrolled human ancestry:", err)
		return 1
	}
	req, err := syncReq(f.root, f.by, f.lineage)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
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
	if err := humanauthority.RecordProof(f.root, goal.Opid(req.Ulid, req.Actor.Machine, req.Actor.Lineage), "goal resume", proof); err != nil {
		fmt.Fprintln(os.Stderr, "goal resume could not record its authority proof:", err)
		return 1
	}
	res, err := goal.Resume(goal.ResumeRequest{VerbRequest: req, GoalID: f.id, Budget: *budget, Authority: &proof})
	return printSyncResult(res, err)
}

func runGoalEnrollTerminal(args []string) int {
	flags := flag.NewFlagSet("goal enroll-terminal", flag.ContinueOnError)
	root := flags.String("root", ".", "checkout root")
	if flags.Parse(args) != nil {
		return 2
	}
	enrollment, err := humanauthority.Enroll(*root, int64(os.Getppid()), nil, time.Now().UTC())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	printJSON(enrollment)
	return 0
}

func runGoalSetObligation(args []string) int {
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
	proof, err := humanauthority.Prove(*root, int64(os.Getppid()), nil, time.Now().UTC())
	if err != nil {
		fmt.Fprintln(os.Stderr, "goal set-obligation could not prove enrolled human ancestry:", err)
		return 1
	}
	req, err := syncReq(*root, *by, *lineage)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	operationID := goal.Opid(req.Ulid, req.Actor.Machine, req.Actor.Lineage)
	if err := humanauthority.RecordProof(*root, operationID, "goal set-obligation", proof); err != nil {
		fmt.Fprintln(os.Stderr, "goal set-obligation could not record its authority proof:", err)
		return 1
	}
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
	return printSyncResult(res, err)
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
	runGoalSetBudget = runSyncOnly("set-budget", func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		budget, err := f.budgetTuple(true)
		if err != nil {
			return goal.PublishResult{}, err
		}
		return goal.SetBudget(req, f.id, *budget)
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
	runGoalEdit = runSyncOnly("edit", func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		fields := goal.EditFields{}
		if f.intent != "" {
			fields.Intent = &f.intent
		}
		if f.next != "" {
			fields.NextStep = &f.next
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
		return goal.Edit(req, f.id, fields)
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
