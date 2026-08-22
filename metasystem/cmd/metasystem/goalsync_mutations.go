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

// syncReq assembles the one request every synced verb consumes.
func syncReq(root, by string) (goal.VerbRequest, error) {
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
	lineage := os.Getenv("METASYSTEM_OWNER_LINEAGE")
	if lineage == "" {
		lineage = "session"
	}
	ulid, err := goalUlid()
	if err != nil {
		return goal.VerbRequest{}, err
	}
	return goal.VerbRequest{
		Endpoint: e,
		Actor:    goal.Actor{Machine: machine, Lineage: lineage, Human: by},
		Ulid:     ulid,
		Now:      time.Now().UTC(),
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
	root, by, id, intent, next, origin, because, conclude, arc string
	claim, refreshOnly                                         bool
	keep                                                       int
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
	fs.BoolVar(&f.claim, "claim", false, "claim on open")
	fs.BoolVar(&f.refreshOnly, "refresh-only", false, "complete a died refresh")
	fs.IntVar(&f.keep, "keep", 10, "archive entries to keep")
	if fs.Parse(args) != nil {
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
	req, err := syncReq(f.root, f.by)
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
		if !need(f.id, "id") || !need(f.intent, "intent") || !need(f.next, "next") {
			return 2, true
		}
		if f.claim {
			res, err := goal.OpenClaim(req, f.id, f.intent, f.origin, f.next)
			return printSyncResult(res, err), true
		}
		res, err := goal.Open(req, f.id, f.intent, f.origin, f.next)
		return printSyncResult(res, err), true
	case "park":
		if !need(f.id, "id") || !need(f.because, "because") {
			return 2, true
		}
		res, err := goal.Park(req, f.id, f.because)
		return printSyncResult(res, err), true
	case "unpark":
		if !need(f.id, "id") {
			return 2, true
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
		fmt.Fprintln(os.Stderr, "declare-free on the synced backlog is not wired yet; raise it if you need it")
		return 1, true
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
		}
		req, err := syncReq(f.root, f.by)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		res, runErr := run(req, f)
		return printSyncResult(res, runErr)
	}
}

var (
	runGoalClaim = runSyncOnly("claim", func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		if f.arc != "" {
			return goal.ClaimArc(req, f.id)
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
	runGoalEdit = runSyncOnly("edit", func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		fields := goal.EditFields{}
		if f.intent != "" {
			fields.Intent = &f.intent
		}
		if f.next != "" {
			fields.NextStep = &f.next
		}
		return goal.Edit(req, f.id, fields)
	}, "id")
	runGoalSetArc = runSyncOnly("set-arc", func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		return goal.SetArc(req, f.id, f.arc)
	}, "id", "arc")
	runGoalDetach = runSyncOnly("detach", func(req goal.VerbRequest, f *syncFlags) (goal.PublishResult, error) {
		return goal.Detach(req, f.id)
	}, "id")
)
