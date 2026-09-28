package main

// The owner functions public commands call in their own process
// (plans/designs/verbs-object-action.md 6.2). Each replaced a child of the
// engine; the edge supplies the current process as the caller identity,
// because that process is the parent the child classified, and the owner's
// output comes back on the caller's buffers exactly as the child's pipes
// carried it. Production uses defaultIntentOwnerCalls; tests give an
// invocation its own.

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/missionrunner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

type intentOwnerCalls struct {
	// brain declares or withdraws the coordinator; its human gate
	// classifies caller.
	brain func(choice string, caller processIdentity, stdout, stderr io.Writer, root, by string) int
	// goalFetch is the read-side advance of the goal ledger.
	goalFetch func(stdout, stderr io.Writer, root string) int
	// goalRepair accepts fetched remote history; the brain's human-word
	// gate classifies caller.
	goalRepair func(caller processIdentity, stdout, stderr io.Writer, root, by string) int
	// configKeys enumerates configured keys.
	configKeys func(stdout io.Writer, conf, matching string) int
	// configValidate validates the configuration domain.
	configValidate func(stdout, stderr io.Writer, conf, repo string) int
	// delegate is the delegate boundary: dispatch, review, follow-up.
	delegate func(request delegateRequest, stdout, stderr io.Writer) int
	// goalReconcile and goalMigrate are the goal owners with the argv their
	// former children carried after the verb; the supplied caller in
	// dependencies.authorityFacts is what their classification and human
	// proof start from.
	goalReconcile func(dependencies syncRequestDependencies, stdout, stderr io.Writer, dir string, args []string) int
	goalMigrate   func(dependencies syncRequestDependencies, stdout, stderr io.Writer, dir string, args []string) int
	// goalCarry records a person's carry word for one landing candidate.
	goalCarry func(dependencies syncRequestDependencies, stdout, stderr io.Writer, dir string, args []string) int
	// landingTestReceipt runs the landing proof on one candidate tree and
	// prints its receipt; caller is what its proof admission classifies.
	landingTestReceipt func(caller processIdentity, stdout, stderr io.Writer, dir string, args []string) int
	// channelWait waits for one channel question's answer; caller is the
	// waiting process the durable wait registers, lineage the channel ledger
	// identity's.
	channelWait func(caller processIdentity, lineage string, stdout, stderr io.Writer, args []string) int
	// missionStatus prints a mission's runner status line.
	missionStatus func(stdout, stderr io.Writer, root, mission string) int
	// missionLaunch starts or resumes a mission's detached run loop; a
	// closed fence's human reopening classifies caller.
	missionLaunch func(caller processIdentity, stdout, stderr io.Writer, root, mission, mode string) int
	// missionResolveTaint applies a person's typed resolution; the
	// human-reserved gate classifies caller.
	missionResolveTaint func(caller processIdentity, stdout, stderr io.Writer, request missionResolveRequest) int
}

// missionResolveRequest is one typed taint resolution: restore to a recorded
// safe tree, or adopt the disputed workspace waiving the named claims.
type missionResolveRequest struct {
	root, mission string
	taint         int64
	variant, tree string
	by, reason    string
	waived        []string
}

// words is the resolution as the former child's argv carried it.
func (r missionResolveRequest) words() []string {
	words := []string{"mission", "resolve-taint", "--root", r.root, "--mission", r.mission, "--taint", strconv.FormatInt(r.taint, 10)}
	if r.variant == "restore" {
		words = append(words, "--restore", r.tree)
	} else {
		words = append(words, "--adopt")
		for _, claim := range r.waived {
			words = append(words, "--waives", claim)
		}
	}
	return append(words, "--by", r.by, "--reason", r.reason)
}

func defaultIntentOwnerCalls() *intentOwnerCalls {
	return &intentOwnerCalls{
		brain: func(choice string, caller processIdentity, stdout, stderr io.Writer, root, by string) int {
			if choice == "withdraw" {
				return brainWithdraw(caller, stdout, stderr, cleanOwnerRoot(root), by, false)
			}
			return brainDeclare(caller, stdout, stderr, cleanOwnerRoot(root), by, false)
		},
		goalFetch: func(stdout, stderr io.Writer, root string) int {
			return goalFetchTo(stdout, stderr, cleanOwnerRoot(root), goal.ResolveEndpoint)
		},
		goalRepair: func(caller processIdentity, stdout, stderr io.Writer, root, by string) int {
			facts := defaultGoalAuthorityReadFacts()
			facts.caller = caller
			return goalRepairAcceptRemoteTo(stdout, stderr, cleanOwnerRoot(root), by, facts, goal.ResolveEndpoint)
		},
		configKeys: func(stdout io.Writer, conf, matching string) int {
			return configKeysTo(stdout, conf, matching, os.Environ())
		},
		configValidate: func(stdout, stderr io.Writer, conf, repo string) int {
			return configValidateTo(stdout, stderr, conf, cleanOwnerRoot(repo))
		},
		delegate: runDelegateWith,
		missionStatus: func(stdout, stderr io.Writer, root, mission string) int {
			engine := missionrunner.NewEngine(cleanOwnerRoot(root), mission)
			engine.Output, engine.Errors = stdout, stderr
			return engine.Status()
		},
		missionLaunch:      missionLaunchTo,
		landingTestReceipt: landingTestReceiptFor,
		channelWait: func(caller processIdentity, lineage string, stdout, stderr io.Writer, args []string) int {
			return channelWaitWith(caller.pid, lineage, stdout, stderr, args, nil)
		},
		goalReconcile: func(dependencies syncRequestDependencies, stdout, stderr io.Writer, _ string, args []string) int {
			return goalReconcileWith(dependencies, stdout, stderr, args)
		},
		goalMigrate: func(dependencies syncRequestDependencies, stdout, stderr io.Writer, _ string, args []string) int {
			return goalMigrateWith(dependencies, stdout, stderr, args)
		},
		goalCarry: func(dependencies syncRequestDependencies, stdout, stderr io.Writer, _ string, args []string) int {
			return goalCarryLandingWith(dependencies, stdout, stderr, args)
		},
		missionResolveTaint: func(caller processIdentity, stdout, stderr io.Writer, request missionResolveRequest) int {
			engine := missionrunner.NewEngine(cleanOwnerRoot(request.root), request.mission)
			engine.Output, engine.Errors, engine.Caller = stdout, stderr, caller.pid
			return engine.ResolveTaint(request.taint, request.variant, request.tree, request.by, request.reason, request.waived)
		},
	}
}

// missionLaunchTo is mission start and resume under an explicit caller: the
// fence check (a closed fence reopens only for a person, classified from
// caller), then the runner's launch at the fence's generation.
func missionLaunchTo(caller processIdentity, stdout, stderr io.Writer, root, mission, mode string) int {
	root = cleanOwnerRoot(root)
	generation, code := missionFenceBeforeArmFor(caller, stderr, root, mode, stateroot.RepositoryTop, lease.ClassifyAt)
	if code != 0 {
		return code
	}
	engine, err := missionRunnerCommandEngine(root, mission)
	if err != nil {
		fmt.Fprintln(stderr, "mission "+mode+":", err)
		return 1
	}
	engine.Output, engine.Errors, engine.Caller = stdout, stderr, caller.pid
	return engine.LaunchAtGeneration(mode, false, generation)
}

// ownerCalls returns the invocation's owner functions.
func (inv *intentInvocation) ownerCalls() *intentOwnerCalls {
	delivery := inv.delivery()
	if delivery.calls == nil {
		delivery.calls = defaultIntentOwnerCalls()
	}
	return delivery.calls
}

// goalOwnerCall runs one argv-shaped goal owner in this process where a
// child used to run it: the invocation's request dependencies, with this
// process as the supplied caller (the parent the child classified) and the
// owner's report on fresh buffers rather than the public result.
func (inv *intentInvocation) goalOwnerCall(owner func(syncRequestDependencies, io.Writer, io.Writer, string, []string) int, args ...string) intentProcessResult {
	dependencies, dir := inv.owners.dependencies, inv.layout.InstallationRoot
	dependencies.report = nil
	dependencies.authorityFacts.caller = currentProcessIdentity()
	return ownerCall(func(stdout, stderr io.Writer) int { return owner(dependencies, stdout, stderr, dir, args) })
}

// ownerCall runs one owner function in this process on fresh buffers and
// returns what it wrote and its status, as a child's pipes and exit code
// were returned.
func ownerCall(run func(stdout, stderr io.Writer) int) intentProcessResult {
	var stdout, stderr bytes.Buffer
	code := run(&stdout, &stderr)
	return intentProcessResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), code: code}
}

// landingTestReceiptFor is landing test-receipt under an explicit caller: its
// testing run and its proof admission classify and authenticate from the
// caller, and the receipt and refusals go to the caller's streams.
func landingTestReceiptFor(caller processIdentity, stdout, stderr io.Writer, _ string, args []string) int {
	invocation := testRunInvocation{callerPID: caller.pid, stdout: stdout, stderr: stderr}
	admit := func(request proofLaunchAdmission) (proofrun.Attempt, proofrun.LaunchResult, bool, error) {
		request.CallerPID = caller.pid
		return admitProofLaunch(request)
	}
	return landingTestReceiptTo(stdout, stderr, context.Background(), goalCommandClock, nil,
		func(testArgs []string) int { return runTestRunWith(invocation, testArgs) }, args,
		landing.PrepareTestReceipt, admit, landing.PublishCommittedReceiptAt, commitProofTerminal)
}
