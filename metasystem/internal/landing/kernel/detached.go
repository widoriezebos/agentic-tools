package kernel

// The detached landing prove: the proof belongs to the lane, not to the
// process that asked for it. landing prove starts the very same blocking
// prove (landing prove --wait) in a session of its own, records it running
// with its process identity, and returns at once; the agent that asked may
// end its turn, and its session with it, without ending the proof. A proof
// whose process is gone without a result died: it holds nothing, and the
// next prove records it unavailable.

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/strictjson"
)

// CodeProveRunning is a prove refused because another tree's proof runs in
// the lane: one proof at a time.
const CodeProveRunning = "LANE_PROVE_RUNNING"

// diedReason is the recorded reason of a proof whose process ended without
// a result.
const diedReason = "the proof's process ended without a result; the next prove runs it again"

// StartSeams are a detached start's effects; ProductionStartSeams is the
// zero case.
type StartSeams struct {
	// Executable is the engine the detached proof runs: this process's.
	Executable func() (string, error)
	Now        func() time.Time
	NewID      func() (string, error)
	Prober     identity.Prober
	// Launch starts argv in a session of its own in dir, its output
	// appended to log, and returns its pid, which is its process group.
	Launch func(argv []string, dir, log string) (int64, error)
}

// ProductionStartSeams are the production effects.
func ProductionStartSeams() StartSeams {
	return StartSeams{Executable: os.Executable, Now: func() time.Time { return time.Now().UTC() }, NewID: newAttemptID,
		Prober: identity.KernelProber{}, Launch: func(argv []string, dir, log string) (int64, error) {
			return gaterun.LaunchDetached(gaterun.DetachedLaunch{Argv: argv, Dir: dir, Log: log})
		}}
}

// Started is a detached start: the proof that runs, and whether it already
// ran before this start (a repeat, which starts nothing).
type Started struct {
	Proof   TreeProof
	Already bool
}

// StartProof starts the proof of request's tree as a detached job and
// returns at once: landing prove --wait --tree T --attempt A in a session
// of its own, recorded running for its tree with its process identity under
// the pause (lane.Gate). Its result is recorded by that job exactly as a
// blocking prove records it. A repeat while the same tree's proof runs
// starts nothing; another tree's running proof refuses it.
func StartProof(request ProveRequest, seams StartSeams) (Started, error) {
	workspace := gittree.Workspace{Dir: string(request.Layout.Checkout)}
	tree, commit, err := subjectOf(workspace, request.Tree)
	if err != nil {
		return Started{}, err
	}
	executable, err := seams.Executable()
	if err != nil {
		return Started{}, err
	}
	prober := seams.Prober
	if prober == nil {
		prober = identity.KernelProber{}
	}
	store := batch.NewStore(string(request.Layout.Checkout), nil)
	var started Started
	err = lane.Gate(request.Home, lane.OpProve, lane.AuthorityAgent, func(registered lane.Record) error {
		layout, err := registered.Layout()
		if err != nil || layout.Checkout != request.Layout.Checkout || layout.Install != request.Layout.Install {
			return &Refusal{Code: CodeProveRefused, Reason: "the landing lane moved while the proof was prepared, so nothing was started", Next: "run the same command again"}
		}
		running, err := settleRunning(request.Layout, store, prober, seams.Now(), "")
		if err != nil {
			return err
		}
		if running != nil {
			if running.Tree == tree {
				started = Started{Proof: *running, Already: true}
				return nil
			}
			return runningRefusal(*running)
		}
		id, err := seams.NewID()
		if err != nil {
			return err
		}
		subject := commit
		if subject == "" {
			subject = tree
		}
		logs := filepath.Join(proofsDir(request.Layout), "runs")
		if err := os.MkdirAll(logs, 0o700); err != nil {
			return err
		}
		argv := []string{executable, "landing", "prove", "--wait", "--tree", subject, "--attempt", id}
		pid, err := seams.Launch(argv, string(request.Layout.Checkout), filepath.Join(logs, id+".log"))
		if err != nil {
			return fmt.Errorf("start the proof of tree %s: %w", tree, err)
		}
		proof := TreeProof{Tree: tree, Commit: commit, Attempt: id, Status: batch.AttemptRunning, StartedAt: seams.Now().Format(time.RFC3339Nano)}
		proof.Process, err = processOf(prober, pid)
		if err == nil {
			err = keepTreeProof(request.Layout, proof)
		}
		if err != nil {
			// Unrecorded, the job would run unseen: it is stopped. Its
			// group is the one just started, by its literal pid.
			_ = syscall.Kill(-int(pid), syscall.SIGKILL)
			return fmt.Errorf("record the proof of tree %s: %w", tree, err)
		}
		started = Started{Proof: proof}
		return nil
	})
	if err != nil {
		return Started{}, err
	}
	return started, nil
}

// ReadRunningProof is the newest proof recorded running in the lane and
// whether its process still runs: Dead is a proof that died without a
// result. false when no proof is recorded running.
func ReadRunningProof(layout lane.Layout, prober identity.Prober) (TreeProof, identity.Liveness, bool, error) {
	running, err := runningProofs(layout)
	if err != nil || len(running) == 0 {
		return TreeProof{}, identity.Dead, false, err
	}
	newest := running[0]
	for _, proof := range running[1:] {
		if proof.StartedAt > newest.StartedAt {
			newest = proof
		}
	}
	return newest, liveness(prober, newest), true, nil
}

func runningProofs(layout lane.Layout) ([]TreeProof, error) {
	paths, err := filepath.Glob(filepath.Join(proofsDir(layout), "*.json"))
	if err != nil {
		return nil, err
	}
	var running []TreeProof
	for _, path := range paths {
		var proof TreeProof
		if err := strictjson.Read(path, &proof); err != nil || proof.Status != batch.AttemptRunning {
			continue
		}
		running = append(running, proof)
	}
	return running, nil
}

// liveness says whether a running proof's process still runs; a proof
// without a readable process identity died.
func liveness(prober identity.Prober, proof TreeProof) identity.Liveness {
	if prober == nil {
		prober = identity.KernelProber{}
	}
	ref, err := identity.ParseRef(proof.Process)
	if err != nil {
		return identity.Dead
	}
	return identity.LiveRef(prober, ref)
}

// settleRunning reads the lane's running proofs other than own: one whose
// process still runs (or cannot be read) is returned; one that died is
// recorded unavailable, on its tree and its batches, and holds nothing.
func settleRunning(layout lane.Layout, store batch.Store, prober identity.Prober, now time.Time, own string) (*TreeProof, error) {
	running, err := runningProofs(layout)
	if err != nil {
		return nil, err
	}
	for _, proof := range running {
		if proof.Attempt == own {
			continue
		}
		if liveness(prober, proof) != identity.Dead {
			return &proof, nil
		}
		proof.Status, proof.Reason, proof.Process = batch.AttemptUnavailable, diedReason, ""
		proof.EndedAt = now.Format(time.RFC3339Nano)
		for _, id := range proof.Batches {
			// A batch that moved on (landed, dissolved, the attempt ended)
			// is no reason to keep the dead proof running.
			_ = batch.FinishAttempt(store, id, proof.Attempt, batch.AttemptUnavailable, "", diedReason, nil, now)
		}
		if err := keepTreeProof(layout, proof); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func runningRefusal(running TreeProof) *Refusal {
	return &Refusal{Code: CodeProveRunning,
		Reason: fmt.Sprintf("tree %s is being proven (attempt %s, since %s), so nothing else was started", short(running.Tree), running.Attempt, lane.LocalText(running.StartedAt)),
		Next:   "run: metasystem landing status shows when it ends"}
}

// processOf is the exact identity of the process at pid, encoded.
func processOf(prober identity.Prober, pid int64) (string, error) {
	exact, live, err := prober.Probe(pid)
	if err != nil || live != identity.Alive {
		return "", fmt.Errorf("the process %d can't be read (%v, %v)", pid, live, err)
	}
	return identity.EncodeRef(exact.Ref())
}

func (seams ProveSeams) prober() identity.Prober {
	if seams.Prober == nil {
		return identity.KernelProber{}
	}
	return seams.Prober
}

// selfProcess is this process's exact identity, encoded.
func selfProcess(prober identity.Prober) (string, error) {
	return processOf(prober, int64(os.Getpid()))
}
