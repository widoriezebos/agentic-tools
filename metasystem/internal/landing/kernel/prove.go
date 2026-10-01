// Package kernel holds the landing lane's kernel operations that act on a
// batch (lane runtime design r10 §2): landing begin, which records the
// canonical series before anything executes, and landing prove, the only
// way a batch's tests run. Every path comes from the registered lane.Layout,
// and every operation passes the pause under the host flock (lane.Gate)
// immediately before it acts.
package kernel

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/custody"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/custody/laneprobe"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/strictjson"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// CodeProveRefused is landing prove's refusal of a subject it cannot run.
const CodeProveRefused = "LANE_PROVE_REFUSED"

const testRunVerb = "internal test run"

// Refusal is a kernel verb's refusal: nothing was recorded or started.
type Refusal struct {
	Code, Reason, Next string
}

func (refusal *Refusal) Error() string { return refusal.Reason }

// RefusalCode is the registered code, for --verbose, --json and records.
func (refusal *Refusal) RefusalCode() string { return refusal.Code }

func proveRefused(reason, next string) error {
	return &Refusal{Code: CodeProveRefused, Reason: reason, Next: next}
}

// ProveRequest is one landing prove --batch ID --subject SUBJECT.
type ProveRequest struct {
	Home    string
	Layout  lane.Layout
	BatchID string
	// Subject is batch, base or member:M.
	Subject string
	Actor   string
}

// ProveSeams are prove's effects; ProductionProveSeams is the zero case.
type ProveSeams struct {
	// Executable is the engine the test run child runs: this process's,
	// which the kernel admitted as the lane's enrolled engine (K5).
	Executable func() (string, error)
	// Select plans the tests of a subject tree whose member recorded no
	// selection, on the lane's account.
	Select func(checkout lane.CheckoutRoot, account, tree string) ([]string, error)
	Prober identity.Prober
	Now    func() time.Time
	NewID  func() (string, error)
	// Custody is the custody barrier's reads for the lane at home whose
	// installation is install; nil reads the production probes.
	Custody func(home, install string) custody.Probes
}

// ProductionProveSeams are the production effects.
func ProductionProveSeams() ProveSeams {
	return ProveSeams{Executable: os.Executable, Select: selectSubjectGroups, Prober: identity.KernelProber{},
		Now: func() time.Time { return time.Now().UTC() }, NewID: newAttemptID}
}

func newAttemptID() (string, error) {
	id, err := goal.NewOperationULID()
	return strings.ToLower(id), err
}

// subject is a parsed prove subject: what tree runs, and how.
type subject struct {
	kind, member, tree, commit, purpose string
	groups                              []string
}

// Prove runs one subject of batch request.BatchID's current opening as a
// lane-charged test run (K6) and records the attempt on the batch, before
// its child starts and again with its typed outcome: green, red, or
// unavailable, which is never red. The attempt is recorded and its child
// started under the pause, in lane.Gate.
func Prove(request ProveRequest, seams ProveSeams) (batch.ProofAttempt, error) {
	checkout := string(request.Layout.Checkout)
	store := batch.NewStore(checkout, nil)
	record, err := store.Load(request.BatchID)
	if err != nil {
		return batch.ProofAttempt{}, proveRefused(fmt.Sprintf("batch %s can't be read: %v", request.BatchID, err), "run landing status to see the lane's batches")
	}
	opening, ok := record.CurrentOpening()
	if !ok {
		return batch.ProofAttempt{}, proveRefused(fmt.Sprintf("batch %s has no series yet, so there is nothing to prove", request.BatchID),
			"compose the batch and run landing begin first")
	}
	account := lane.AccountID(checkout)
	executable, err := seams.Executable()
	if err != nil {
		return batch.ProofAttempt{}, err
	}
	target, err := resolveSubject(request, record, opening, seams)
	if err != nil {
		return batch.ProofAttempt{}, err
	}
	detached, err := (gittree.Workspace{Dir: checkout}).NewDetachedWorktree(target.tree)
	if err != nil {
		return batch.ProofAttempt{}, fmt.Errorf("project the %s tree %s: %w", target.kind, target.tree, err)
	}
	defer detached.Close()
	execution := request.Layout.Execution(lane.CheckoutRoot(detached.Workspace().Dir))
	id, err := seams.NewID()
	if err != nil {
		return batch.ProofAttempt{}, err
	}
	resultDir := filepath.Join(string(request.Layout.Install), "artifacts", "agents", "proof-runs", "batch")
	if err := os.MkdirAll(resultDir, 0o700); err != nil {
		return batch.ProofAttempt{}, err
	}
	attempt := batch.ProofAttempt{ID: id, OpID: opening.OpID, Subject: target.kind, Member: target.member, Commit: target.commit, Tree: target.tree,
		Purpose: target.purpose, Groups: target.groups, Actor: request.Actor, ResultPath: filepath.Join(resultDir, request.BatchID+"-"+id+".json"),
		StartedAt: seams.Now().Format(time.RFC3339Nano)}
	command := batchowner.BatchProofCommand(executable, proveArgs(request.Layout, execution, account, target, attempt.ResultPath))
	command.Dir, command.Env = string(execution), gittree.ScrubbedEnviron()
	read := verbresult.Capture(command, testRunVerb)
	started := false
	gateErr := lane.Gate(request.Home, lane.OpProve, lane.AuthorityAgent, func(registered lane.Record) error {
		layout, err := registered.Layout()
		if err != nil || layout.Checkout != request.Layout.Checkout || layout.Install != request.Layout.Install {
			return proveRefused("the landing lane moved while the test run was prepared, so nothing was started", "run the same command again")
		}
		// The one custody barrier (K9): nothing starts while landing work
		// still runs or can't be read.
		probes := laneprobe.Production(request.Home, string(request.Layout.Install), true)
		if seams.Custody != nil {
			probes = seams.Custody(request.Home, string(request.Layout.Install))
		}
		if err := custody.Clear(request.Home, probes, false); err != nil {
			return err
		}
		if err := batch.StartAttempt(store, request.BatchID, attempt); err != nil {
			return err
		}
		// The child is custodied from its start: a record opened before it,
		// bound to its exact identity and its own process group, and the
		// groups the proof launcher makes inside it bind to the same record.
		_, startErr := custody.Start(request.Home, custody.KindProve, "batch "+request.BatchID+" attempt "+id+" "+request.Subject, seams.Now(), command)
		if command.Process == nil {
			reason := "the test run could not start: " + startErr.Error()
			return errors.Join(startErr, batch.FinishAttempt(store, request.BatchID, id, batch.AttemptUnavailable, "", reason, nil, seams.Now()))
		}
		// A child whose custody could not be bound still runs to its end;
		// its record then reads unknown and holds the lane until a person
		// goes past it.
		started = true
		return nil
	})
	if !started {
		if recorded, loadErr := store.Load(request.BatchID); loadErr == nil {
			if ended, ok := recorded.Attempt(id); ok && ended.Terminal() {
				return ended, nil
			}
		}
		return batch.ProofAttempt{}, gateErr
	}
	if exact, liveness, probeErr := seams.Prober.Probe(int64(command.Process.Pid)); probeErr == nil && liveness == identity.Alive {
		// The custody barrier (K9) reads the child from here.
		_ = batch.SetAttemptChild(store, request.BatchID, id, exact.Ref())
	}
	child, childErr := read(command.Wait())
	var result proofrun.TestResult
	resultErr := strictjson.Read(attempt.ResultPath, &result)
	status, reason, red := classify(target.purpose, child, childErr, result, resultErr)
	if err := batch.FinishAttempt(store, request.BatchID, id, status, result.AttemptID, reason, red, seams.Now()); err != nil {
		return batch.ProofAttempt{}, err
	}
	recorded, err := store.Load(request.BatchID)
	if err != nil {
		return batch.ProofAttempt{}, err
	}
	ended, _ := recorded.Attempt(id)
	return ended, nil
}

// resolveSubject is the tree, purpose and tests of a prove subject: the
// candidate's tree as a delivery run; B's, or B plus exactly member M's
// admitted contribution, as a diagnostic of the members' selected tests.
func resolveSubject(request ProveRequest, record batch.Record, opening batch.Opening, seams ProveSeams) (subject, error) {
	switch {
	case request.Subject == batch.SubjectBatch:
		return subject{kind: batch.SubjectBatch, tree: opening.Tree, commit: opening.Candidate, purpose: "delivery"}, nil
	case request.Subject == batch.SubjectBase:
		var groups []string
		for _, name := range opening.Members {
			unit, _ := joinedMember(record, name)
			selected, err := memberGroups(request, unit, opening, seams)
			if err != nil {
				return subject{}, err
			}
			groups = append(groups, selected...)
		}
		slices.Sort(groups)
		groups = slices.Compact(groups)
		if len(groups) == 0 {
			return subject{}, proveRefused(fmt.Sprintf("batch %s's members select no tests, so the base has none to run", request.BatchID), "prove the batch subject instead")
		}
		return subject{kind: batch.SubjectBase, tree: opening.BaseTree, commit: opening.Base, purpose: "diagnostic", groups: groups}, nil
	}
	name, found := strings.CutPrefix(request.Subject, batch.SubjectMember+":")
	if !found || name == "" {
		return subject{}, proveRefused(fmt.Sprintf("%q is not a subject; a subject is batch, base or member:M", request.Subject),
			"run landing prove --batch "+request.BatchID+" --subject batch")
	}
	if !slices.Contains(opening.Members, name) {
		return subject{}, proveRefused(fmt.Sprintf("%s is not a member of batch %s's series", name, request.BatchID),
			"name a member landing status lists for the batch")
	}
	unit, ok := joinedMember(record, name)
	if !ok {
		return subject{}, proveRefused(fmt.Sprintf("%s is no longer a joined member of batch %s", name, request.BatchID), "run landing status to see the batch")
	}
	tree, err := batch.MemberSubjectTree(string(request.Layout.Checkout), opening.BaseTree, unit)
	if err != nil {
		return subject{}, fmt.Errorf("build member %s's tree on the base: %w", name, err)
	}
	groups, err := memberGroups(request, unit, opening, seams)
	if err != nil {
		return subject{}, err
	}
	if len(groups) == 0 {
		return subject{}, proveRefused(fmt.Sprintf("%s selects no tests, so there is nothing to prove for it", name), "prove the batch subject instead")
	}
	return subject{kind: batch.SubjectMember, member: name, tree: tree, purpose: "diagnostic", groups: groups}, nil
}

func joinedMember(record batch.Record, name string) (batch.Unit, bool) {
	for _, unit := range record.Units {
		if unit.GoalID == name && unit.State == batch.UnitJoined {
			return unit, true
		}
	}
	return batch.Unit{}, false
}

// memberGroups are a member's tests: the selection recorded when it joined,
// else the plan of its own tree on the base, on the lane's account.
func memberGroups(request ProveRequest, unit batch.Unit, opening batch.Opening, seams ProveSeams) ([]string, error) {
	if len(unit.SelectedGroups) != 0 {
		return slices.Clone(unit.SelectedGroups), nil
	}
	if unit.GoalID == "" || seams.Select == nil {
		return nil, nil
	}
	tree, err := batch.MemberSubjectTree(string(request.Layout.Checkout), opening.BaseTree, unit)
	if err != nil {
		return nil, err
	}
	return seams.Select(request.Layout.Checkout, lane.AccountID(string(request.Layout.Checkout)), tree)
}

// selectSubjectGroups plans tree on the lane's account through the lane's
// one planner (batchowner's, the same child a batch plan runs).
func selectSubjectGroups(checkout lane.CheckoutRoot, account, tree string) ([]string, error) {
	plan, err := batchowner.ProductionBatchTreePlan(string(checkout), account, tree, testpolicy.ModeAuto)
	return plan.SelectedGroups, err
}

// proveArgs is the child's argv: a run charged to the lane, whose lane
// checkout and control root are named, never guessed (K6).
func proveArgs(layout lane.Layout, execution lane.InstallRoot, account string, target subject, resultPath string) []string {
	args := []string{"internal", "test", "run", "--root", string(execution), "--control-root", string(layout.Install),
		"--lane-checkout", string(layout.Checkout), "--lane", account, "--tree", target.tree}
	if target.purpose == "delivery" {
		args = append(args, "--batch-tip", "--mode", "auto", "--purpose", "delivery")
	} else {
		args = append(args, "--mode", "canary", "--purpose", "diagnostic", "--groups", strings.Join(target.groups, ","), "--no-reuse")
	}
	return append(args, "--result", resultPath, "--json")
}

// classify types a finished child: red only when a test failed; green when
// the run confirmed (a delivery run may reuse sufficient evidence); and
// unavailable for everything else — a refused admission, a run that could
// not start or finish, a result that cannot be read.
func classify(purpose string, child verbresult.Result, childErr error, result proofrun.TestResult, resultErr error) (status, reason string, red []string) {
	if childErr != nil {
		return batch.AttemptUnavailable, "the test run's answer can't be read: " + childErr.Error(), nil
	}
	if child.Outcome == verbresult.Refused {
		return batch.AttemptUnavailable, "the test run was refused: " + child.Err().Error(), nil
	}
	if resultErr != nil {
		return batch.AttemptUnavailable, "the test run left no readable result: " + resultErr.Error(), nil
	}
	for _, group := range result.Groups {
		if group.Status == "failed" {
			red = append(red, group.ID)
		}
	}
	if len(red) != 0 {
		return batch.AttemptRed, "failing tests: " + strings.Join(red, ", "), red
	}
	passed := !slices.ContainsFunc(result.Groups, func(group proofrun.GroupResult) bool { return group.Status != "passed" && group.Status != "reused" })
	switch {
	case purpose == "delivery" && batchowner.BatchProofOutcomeAccepted(child, result):
		return batch.AttemptGreen, "", nil
	case purpose != "delivery" && child.Outcome == verbresult.Confirmed && passed && len(result.Groups) != 0:
		return batch.AttemptGreen, "", nil
	}
	if child.Outcome != verbresult.Confirmed {
		return batch.AttemptUnavailable, "the test run did not finish: " + child.Err().Error(), nil
	}
	return batch.AttemptUnavailable, "the test run finished without a passing result for every test", nil
}
