package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	usagepkg "github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
)

// proveRoundTestRun is the proof itself: the installation's own test run on
// the round's tree. A test swaps it for a recorder.
var proveRoundTestRun = runTestRun

// proveRoundPurpose is the one purpose a round's proof can have. A delivery
// attempt proves the seat's own staged index and nothing else (prepareTesting
// refuses any other candidate), because its receipt is what lands. A round's
// worktree snapshot is a foreign tree by construction, so it is proved as a
// diagnostic attempt: every selected group runs or is reused, every failure
// is collected for one follow-up, and the landing's delivery receipt reuses
// the passed groups by execution identity.
const proveRoundPurpose = "diagnostic"

// ProofRoundRecord is what a proved round leaves in its round directory:
// which tree was proved, by which attempt, with what outcome. The attempt
// record under artifacts/agents/proof-runs/attempts holds the per-group
// evidence and the reuse the next round or the landing can draw on.
type ProofRoundRecord struct {
	SchemaVersion int    `json:"schemaVersion"`
	RootJob       string `json:"rootJob"`
	Round         int64  `json:"round"`
	GoalID        string `json:"goalId"`
	Purpose       string `json:"purpose"`
	CandidateTree string `json:"candidateTree"`
	AttemptID     string `json:"attemptId,omitempty"`
	Sufficient    bool   `json:"sufficient"`
	ExitStatus    int    `json:"exitStatus"`
	ProvedAt      string `json:"provedAt"`
}

// runDispatchProveRound proves a delegate round where proof can be made and
// kept: on the orchestrator's enrolled engine, against the job worktree as
// the round left it (delegates never commit: HEAD plus every change in the
// working tree, the same snapshot conformance reviews), recorded on this
// installation. A delegate cannot make this proof inside its sandbox (its
// worktree carries no enrolled engine, and an attempt recorded there would
// be invisible to the landing), so the orchestrator makes it on return;
// retained proof is reused by execution identity, so a round that changed
// no group's inputs proves in seconds and the landing reuses the last
// round's passed groups.
func runDispatchProveRound(args []string) int {
	flags := flag.NewFlagSet("job prove-round", flag.ContinueOnError)
	root := flags.String("root", ".", "MetaSystem installation root")
	job := flags.String("job", "", "any job id of the chain")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	if *job == "" {
		fmt.Fprintln(os.Stderr, "job prove-round: --job is required")
		return 2
	}
	installation, err := canonicalPath(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "job prove-round:", err)
		return 1
	}
	proof, refusal, err := prepareProofRound(installation, *job)
	if err != nil {
		fmt.Fprintln(os.Stderr, "job prove-round:", err)
		return 1
	}
	if refusal != "" {
		fmt.Fprintln(os.Stderr, "job prove-round refused:", refusal)
		return 2
	}
	// Only an attempt this run makes is the round's proof: an older attempt
	// for the same tree is not linked, however green.
	before, err := proofrun.ReadAttempts(installation)
	if err != nil {
		fmt.Fprintln(os.Stderr, "job prove-round: read the attempts:", err)
		return 1
	}
	known := make(map[string]bool, len(before))
	for _, attempt := range before {
		known[attempt.AttemptID] = true
	}
	fmt.Printf("prove-round: chain %s round %d tree %s goal %s purpose %s\n", proof.RootJob, proof.Round, proof.CandidateTree, proof.GoalID, proof.Purpose)
	proof.ExitStatus = proveRoundTestRun([]string{"--root", installation, "--tree", proof.CandidateTree,
		"--goal", proof.GoalID, "--mode", "auto", "--purpose", proof.Purpose})
	proof.ProvedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := writeProofRoundRecord(installation, proof); err != nil {
		fmt.Fprintln(os.Stderr, "job prove-round: record the proof:", err)
		return 1
	}
	after, err := proofrun.ReadAttempts(installation)
	if err != nil {
		fmt.Fprintln(os.Stderr, "job prove-round: read the attempts after the run (the round record names no attempt):", err)
		return 1
	}
	attempt, found := newestAttemptForTree(after, proof.CandidateTree, proof.GoalID, known)
	if found {
		proof.AttemptID = attempt.AttemptID
		proof.Sufficient = attempt.TestResult != nil && attempt.TestResult.Delivery.Sufficient
		if err := writeProofRoundRecord(installation, proof); err != nil {
			fmt.Fprintln(os.Stderr, "job prove-round: record the proof:", err)
			return 1
		}
	}
	switch {
	case !found:
		fmt.Printf("prove-round: the run recorded no attempt for tree %s (test run exit %d)\n", proof.CandidateTree, proof.ExitStatus)
	case proof.Sufficient:
		fmt.Printf("prove-round: attempt %s sufficient\n", proof.AttemptID)
	default:
		fmt.Printf("prove-round: attempt %s insufficient (test run exit %d)\n", proof.AttemptID, proof.ExitStatus)
	}
	return proof.ExitStatus
}

// prepareProofRound resolves the chain and the tree to prove. A refusal is
// a reason in plain words and no error: the record was readable, the round
// is just not provable this way.
func prepareProofRound(installation, job string) (ProofRoundRecord, string, error) {
	jobs := filepath.Join(installation, "artifacts", "agents", "jobs")
	rootJob, err := usagepkg.RootJobID(jobs, job)
	if err != nil {
		return ProofRoundRecord{}, "", err
	}
	record, err := dispatchcore.ReadRecordObject(filepath.Join(jobs, rootJob+".json"))
	if err != nil {
		return ProofRoundRecord{}, "", err
	}
	proof := ProofRoundRecord{SchemaVersion: 1, RootJob: rootJob, Purpose: proveRoundPurpose}
	proof.GoalID, _ = record["goalId"].(string)
	launchMode, _ := record["launchMode"].(string)
	workspace, _ := record["workspaceRoot"].(string)
	if launchMode == "" {
		// Older records carry no launch mode; the dispatcher infers it from
		// where the workspace lives, and so does this verb.
		launchMode = "shared-checkout"
		if strings.HasPrefix(workspace+"/", filepath.Join(installation, "artifacts", "agents", "worktrees")+"/") {
			launchMode = "worktree"
		}
	}
	if launchMode != "worktree" {
		return proof, fmt.Sprintf("job %s runs in the shared checkout, so its tree is this checkout's own index: prove it as your own work with metasystem test run", rootJob), nil
	}
	if proof.GoalID == "" {
		return proof, fmt.Sprintf("job %s carries no goal, and a proof binds one", rootJob), nil
	}
	if workspace == "" {
		return proof, fmt.Sprintf("job %s records no worktree", rootJob), nil
	}
	if info, statErr := os.Stat(workspace); statErr != nil || !info.IsDir() {
		return proof, fmt.Sprintf("the worktree of job %s is gone: %s", rootJob, workspace), nil
	}
	latest, err := dispatchcore.LatestChainRecord(jobs, rootJob)
	if err != nil {
		return ProofRoundRecord{}, "", err
	}
	latestRecord, err := dispatchcore.ReadRecordObject(latest)
	if err != nil {
		return ProofRoundRecord{}, "", err
	}
	switch round := latestRecord["round"].(type) {
	case float64:
		proof.Round = int64(round)
	case json.Number:
		if value, err := round.Int64(); err == nil {
			proof.Round = value
		}
	}
	latestID, _ := latestRecord["jobId"].(string)
	latestStatus, _ := latestRecord["status"].(string)
	if !dispatchcore.TerminalStatus(latestStatus) {
		return proof, fmt.Sprintf("round %d of job %s (%s) is %s; a round is proved after it has returned", proof.Round, rootJob, latestID, latestStatus), nil
	}
	// The tree proved is the worktree as it stands: delegates never commit,
	// so HEAD plus every working-tree change, tracked and untracked alike,
	// is the round's work, exactly the snapshot conformance reviews.
	proof.CandidateTree, err = (gittree.Workspace{Dir: workspace}).Snapshot("HEAD")
	if err != nil {
		return ProofRoundRecord{}, "", err
	}
	return proof, "", nil
}

// newestAttemptForTree picks the attempt the run just made: the newest one
// that proved this tree for this goal and did not exist before the run.
func newestAttemptForTree(attempts []proofrun.Attempt, tree, goalID string, known map[string]bool) (proofrun.Attempt, bool) {
	var newest proofrun.Attempt
	var newestStart time.Time
	found := false
	for _, attempt := range attempts {
		if known[attempt.AttemptID] || attempt.TestResult == nil || attempt.TestResult.CandidateTree != tree || attempt.GoalID != goalID {
			continue
		}
		started, err := time.Parse(time.RFC3339Nano, attempt.StartedAt)
		if err != nil {
			continue
		}
		if !found || started.After(newestStart) {
			newest, newestStart, found = attempt, started, true
		}
	}
	return newest, found
}

func proofRoundRecordPath(installation string, proof ProofRoundRecord) string {
	return filepath.Join(installation, "artifacts", "agents", proof.RootJob, "rounds", fmt.Sprint(proof.Round), "proof.json")
}

func writeProofRoundRecord(installation string, proof ProofRoundRecord) error {
	path := proofRoundRecordPath(installation, proof)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(proof, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), "proof.json.*")
	if err != nil {
		return err
	}
	if _, err := temporary.Write(append(data, '\n')); err != nil {
		_ = temporary.Close()
		_ = os.Remove(temporary.Name())
		return err
	}
	if err := temporary.Close(); err != nil {
		_ = os.Remove(temporary.Name())
		return err
	}
	if err := os.Chmod(temporary.Name(), 0o644); err != nil {
		_ = os.Remove(temporary.Name())
		return err
	}
	if err := os.Rename(temporary.Name(), path); err != nil {
		_ = os.Remove(temporary.Name())
		return err
	}
	return nil
}
