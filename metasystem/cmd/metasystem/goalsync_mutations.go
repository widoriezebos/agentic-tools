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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
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
	now, err := goalCommandNow()
	if err != nil {
		return goal.VerbRequest{}, err
	}
	return goal.VerbRequest{
		Endpoint: e,
		Actor:    goal.Actor{Machine: machine, Lineage: lineage, Human: by},
		Ulid:     ulid,
		Now:      now,
	}, nil
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
	lineage, digest, remaining                                      string
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
	fs.StringVar(&f.remaining, "remaining", "", "the claimant's current remaining-work estimate")
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
	if name != "estimate" && f.remaining != "" {
		fmt.Fprintf(os.Stderr, "goal %s does not take --remaining\n", name)
		return nil, false
	}
	return f, true
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
			res, err := goal.OpenClaim(req, f.id, f.intent, f.origin, f.next, f.labels...)
			return printSyncResult(res, err), true
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
		return printSyncResult(res, err), true
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
			if r == "remaining" && f.remaining == "" {
				fmt.Fprintf(os.Stderr, "goal %s needs --remaining\n", name)
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
		if code == 0 && (name == "claim" || name == "estimate") {
			printGoalBanners(f.root, "")
		}
		return code
	}
}

var (
	runGoalClaim = runSyncOnly("claim", func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		if f.arc != "" {
			return goal.ClaimArc(req, f.id)
		}
		return goal.Claim(req, f.id)
	}, "id")
	runGoalEstimate = runSyncOnly("estimate", func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		return goal.Estimate(req, f.id, f.remaining)
	}, "id", "remaining")
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
