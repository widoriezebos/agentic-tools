// Package kernel holds the landing lane's two deterministic rails over the
// lane checkout (simple lane): landing prove, which runs the tests of the
// tree the checkout holds and records the result for that exact tree, and
// landing push, which puts on main only a tree proven green, and only as a
// fast-forward. Every path comes from the registered lane.Layout, and both
// pass the pause under the host flock (lane.Gate) immediately before they
// act.
package kernel

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/strictjson"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// CodeProveRefused is landing prove's refusal of a tree it cannot run.
const CodeProveRefused = "LANE_PROVE_REFUSED"

const testRunVerb = "internal test run"

var objectID = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)

// Refusal is a kernel verb's refusal: nothing was recorded, started or
// pushed.
type Refusal struct {
	Code, Reason, Next string
}

func (refusal *Refusal) Error() string { return refusal.Reason }

// RefusalCode is the registered code, for --verbose, --json and records.
func (refusal *Refusal) RefusalCode() string { return refusal.Code }

// ProveRequest is one landing prove [--tree T].
type ProveRequest struct {
	Home   string
	Layout lane.Layout
	// Tree is the tree (or a commit of it) to prove; empty proves the lane
	// checkout's HEAD.
	Tree  string
	Actor string
	// Attempt is the attempt id a detached start chose for this proof
	// (StartProof); empty draws a new one.
	Attempt string
}

// ProveSeams are prove's effects; ProductionProveSeams is the zero case.
type ProveSeams struct {
	// Executable is the engine the test run child runs: this process's.
	Executable func() (string, error)
	Now        func() time.Time
	NewID      func() (string, error)
	// Prober reads whether a recorded prover still runs; nil is the
	// kernel's.
	Prober identity.Prober
}

// ProductionProveSeams are the production effects.
func ProductionProveSeams() ProveSeams {
	return ProveSeams{Executable: os.Executable, Now: func() time.Time { return time.Now().UTC() }, NewID: newAttemptID, Prober: identity.KernelProber{}}
}

func newAttemptID() (string, error) {
	id, err := goal.NewOperationULID()
	return strings.ToLower(id), err
}

// TreeProof is landing prove's result for one tree, kept by the tree: what
// landing push reads, and the last proof landing status shows.
type TreeProof struct {
	Tree string `json:"tree"`
	// Commit is the lane checkout's commit of the tree, when it named one.
	Commit    string   `json:"commit,omitempty"`
	Attempt   string   `json:"attempt"`
	Status    string   `json:"status"`
	Reason    string   `json:"reason,omitempty"`
	RedGroups []string `json:"redGroups,omitempty"`
	// Batches are the queued hand-offs the attempt is recorded on.
	Batches    []string `json:"batches,omitempty"`
	ResultPath string   `json:"resultPath"`
	// Process is the exact identity of the process that runs the proof,
	// while its status is running: a running proof whose process is gone
	// died without a result.
	Process   string `json:"process,omitempty"`
	StartedAt string `json:"startedAt"`
	EndedAt   string `json:"endedAt,omitempty"`
}

func proofsDir(layout lane.Layout) string {
	return filepath.Join(string(layout.Checkout), "artifacts", "agents", "landing-proofs")
}

// ReadTreeProof is the kept result of tree; false when it was never proven.
func ReadTreeProof(layout lane.Layout, tree string) (TreeProof, bool, error) {
	if !objectID.MatchString(tree) {
		return TreeProof{}, false, fmt.Errorf("%q is not a tree id", tree)
	}
	var proof TreeProof
	err := strictjson.Read(filepath.Join(proofsDir(layout), tree+".json"), &proof)
	if errors.Is(err, os.ErrNotExist) {
		return TreeProof{}, false, nil
	}
	return proof, err == nil, err
}

// LastTreeProof is the tree result recorded last; false when none is.
func LastTreeProof(layout lane.Layout) (TreeProof, bool, error) {
	paths, err := filepath.Glob(filepath.Join(proofsDir(layout), "*.json"))
	if err != nil {
		return TreeProof{}, false, err
	}
	var last TreeProof
	found := false
	for _, path := range paths {
		var proof TreeProof
		if err := strictjson.Read(path, &proof); err != nil {
			continue
		}
		if !found || proof.StartedAt > last.StartedAt {
			last, found = proof, true
		}
	}
	return last, found, nil
}

func keepTreeProof(layout lane.Layout, proof TreeProof) error {
	data, err := json.MarshalIndent(proof, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(proofsDir(layout), 0o700); err != nil {
		return err
	}
	_, err = atomicfile.WriteText(filepath.Join(proofsDir(layout), proof.Tree+".json"), string(data)+"\n", string(layout.Checkout))
	return err
}

// queued are the batch records with members waiting to land, and those
// members, in record order.
func queued(store batch.Store) ([]batch.Record, map[string][]string, error) {
	records, err := store.Records()
	if err != nil {
		return nil, nil, err
	}
	var waiting []batch.Record
	members := map[string][]string{}
	for _, record := range records {
		for _, unit := range record.Units {
			if unit.State == batch.UnitJoined {
				members[record.BatchID] = append(members[record.BatchID], unit.GoalID)
			}
		}
		if len(members[record.BatchID]) != 0 {
			waiting = append(waiting, record)
		}
	}
	return waiting, members, nil
}

// Prove runs the tests of the lane checkout's HEAD tree, or of request.Tree,
// as one lane-charged delivery run (the selected tests a seat's own landing
// runs) and records the attempt on every queued hand-off's batch, before its
// child starts and again with its typed outcome: green, red, or
// unavailable, which is never red. The outcome is kept for the tree, for
// landing push. The attempt is recorded and its child started under the
// pause, in lane.Gate.
func Prove(request ProveRequest, seams ProveSeams) (TreeProof, error) {
	checkout := string(request.Layout.Checkout)
	workspace := gittree.Workspace{Dir: checkout}
	tree, commit, err := subjectOf(workspace, request.Tree)
	if err != nil {
		return TreeProof{}, err
	}
	store := batch.NewStore(checkout, nil)
	records, members, err := queued(store)
	if err != nil {
		return TreeProof{}, err
	}
	executable, err := seams.Executable()
	if err != nil {
		return TreeProof{}, err
	}
	detached, err := workspace.NewDetachedWorktree(tree)
	if err != nil {
		return TreeProof{}, fmt.Errorf("project the tree %s: %w", tree, err)
	}
	defer detached.Close()
	execution := request.Layout.Execution(lane.CheckoutRoot(detached.Workspace().Dir))
	id := request.Attempt
	if id == "" {
		if id, err = seams.NewID(); err != nil {
			return TreeProof{}, err
		}
	}
	self, err := selfProcess(seams.prober())
	if err != nil {
		return TreeProof{}, err
	}
	resultDir := filepath.Join(string(request.Layout.Install), "artifacts", "agents", "proof-runs", "batch")
	if err := os.MkdirAll(resultDir, 0o700); err != nil {
		return TreeProof{}, err
	}
	proof := TreeProof{Tree: tree, Commit: commit, Attempt: id, Status: batch.AttemptRunning,
		ResultPath: filepath.Join(resultDir, "lane-"+id+".json"), StartedAt: seams.Now().Format(time.RFC3339Nano)}
	for _, record := range records {
		proof.Batches = append(proof.Batches, record.BatchID)
	}
	command := batchowner.BatchProofCommand(executable, proveArgs(request.Layout, execution, lane.AccountID(checkout), tree, proof.ResultPath))
	command.Dir, command.Env = string(execution), gittree.ScrubbedEnviron()
	read := verbresult.Capture(command, testRunVerb)
	finish := func(status, runAttempt, reason string, red []string) error {
		proof.Status, proof.Reason, proof.RedGroups, proof.Process = status, reason, red, ""
		proof.EndedAt = seams.Now().Format(time.RFC3339Nano)
		var errs []error
		for _, record := range records {
			errs = append(errs, batch.FinishAttempt(store, record.BatchID, id, status, runAttempt, reason, red, seams.Now()))
		}
		return errors.Join(append(errs, keepTreeProof(request.Layout, proof))...)
	}
	started := false
	gateErr := lane.Gate(request.Home, lane.OpProve, lane.AuthorityAgent, func(registered lane.Record) error {
		layout, err := registered.Layout()
		if err != nil || layout.Checkout != request.Layout.Checkout || layout.Install != request.Layout.Install {
			return &Refusal{Code: CodeProveRefused, Reason: "the landing lane moved while the test run was prepared, so nothing was started", Next: "run the same command again"}
		}
		running, err := settleRunning(request.Layout, store, seams.prober(), seams.Now(), id)
		if err != nil {
			return err
		}
		if running != nil {
			return runningRefusal(*running)
		}
		proof.Process = self
		if err := keepTreeProof(request.Layout, proof); err != nil {
			return err
		}
		for _, record := range records {
			attempt := batch.ProofAttempt{ID: id, Subject: batch.SubjectBatch, Covers: members[record.BatchID], Commit: commit, Tree: tree,
				Purpose: "delivery", Actor: request.Actor, ResultPath: proof.ResultPath, StartedAt: proof.StartedAt}
			if err := batch.StartAttempt(store, record.BatchID, attempt); err != nil {
				return err
			}
		}
		if err := command.Start(); err != nil {
			return errors.Join(err, finish(batch.AttemptUnavailable, "", "the test run could not start: "+err.Error(), nil))
		}
		started = true
		return nil
	})
	if !started {
		if proof.Status != batch.AttemptRunning {
			return proof, nil
		}
		return TreeProof{}, gateErr
	}
	child, childErr := read(command.Wait())
	var result proofrun.TestResult
	resultErr := strictjson.Read(proof.ResultPath, &result)
	status, reason, red := classify(child, childErr, result, resultErr)
	if err := finish(status, result.AttemptID, reason, red); err != nil {
		return proof, err
	}
	return proof, nil
}

// subjectOf is the tree a prove of subject (a commit or tree of the lane
// checkout; empty is its HEAD) proves, and its commit when it names one.
func subjectOf(workspace gittree.Workspace, subject string) (tree, commit string, err error) {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		subject = "HEAD"
	}
	tree, err = workspace.TreeOf(subject)
	if err != nil {
		return "", "", &Refusal{Code: CodeProveRefused, Reason: fmt.Sprintf("%s is not a tree of the lane checkout", subject),
			Next: "name a commit or tree of the lane checkout, or prove its HEAD"}
	}
	commit, _ = batchowner.GitOutput(workspace.Dir, "rev-parse", "--verify", "--quiet", subject+"^{commit}")
	return tree, commit, nil
}

// proveArgs is the child's argv: a delivery run of tree as the batch tip,
// charged to the lane, whose lane checkout and control root are named,
// never guessed.
func proveArgs(layout lane.Layout, execution lane.InstallRoot, account, tree, resultPath string) []string {
	return []string{"internal", "test", "run", "--root", string(execution), "--control-root", string(layout.Install),
		"--lane-checkout", string(layout.Checkout), "--lane", account, "--tree", tree,
		"--batch-tip", "--mode", "auto", "--purpose", "delivery", "--result", resultPath, "--json"}
}

// classify types a finished child: red only when a test failed; green when
// the run confirmed (a delivery run may reuse sufficient evidence); and
// unavailable for everything else — a refused admission, a run that could
// not finish, a result that cannot be read.
func classify(child verbresult.Result, childErr error, result proofrun.TestResult, resultErr error) (status, reason string, red []string) {
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
	if batchowner.BatchProofOutcomeAccepted(child, result) {
		return batch.AttemptGreen, "", nil
	}
	if child.Outcome != verbresult.Confirmed {
		return batch.AttemptUnavailable, "the test run did not finish: " + child.Err().Error(), nil
	}
	return batch.AttemptUnavailable, "the test run finished without a passing result for every test", nil
}

// gitIn runs git in dir with the scrubbed environment and returns its
// trimmed output, standard error with it when it fails.
func gitIn(dir string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	output, err := command.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(output)), fmt.Errorf("git %s: %s: %w", args[0], strings.TrimSpace(string(output)), err)
	}
	return strings.TrimSpace(string(output)), nil
}
