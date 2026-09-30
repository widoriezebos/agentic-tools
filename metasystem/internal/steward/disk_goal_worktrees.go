package steward

// The goal and session worktrees' proofs in a checkout pass (design
// engine-owns-disk-lifetimes Part B, 3.2 "Goal" and "Seat", U5e): a
// concluded goal's registered worktree is swept by the same sweep goal done
// runs, after the sweep plan refuses nothing; a second session's worktree
// is released once every main announced in it is dead.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// GoalBranchSweep is goal done's sweep of one goal's branch and worktrees,
// which the engine registers (the goal-branch owner imports this package, so
// it cannot be imported here): Plan answers the refusal the sweep would
// return, read only and without a fetch; Sweep sweeps. Both are bound to the
// pass's context. Without a registration the goal-worktree proof holds.
type GoalBranchSweep struct {
	Plan  func(ctx context.Context, repo, goalID, dropped string) (refusal string, err error)
	Sweep func(ctx context.Context, repo, goalID, dropped string) error
}

var registeredGoalBranchSweep GoalBranchSweep

// RegisterGoalBranchSweep installs the engine's goal-branch sweep.
func RegisterGoalBranchSweep(sweep GoalBranchSweep) { registeredGoalBranchSweep = sweep }

// goalSweeper reads the goal's dropped units from the accepted ledger and
// calls the registered sweep.
type goalSweeper struct {
	top    string
	ledger *ledgerView
	sweep  GoalBranchSweep
}

func (s goalSweeper) dropped(goalID string) (string, error) {
	projection, err := s.ledger.get()
	if err != nil {
		return "", err
	}
	if file := projection.Tree.Done[goalID]; file != nil {
		return file.NextStep, nil
	}
	return "", nil
}

func (s goalSweeper) plan(ctx context.Context, goalID string) (string, error) {
	if s.sweep.Plan == nil {
		return "", errors.New("this engine has no goal-branch sweep registered")
	}
	dropped, err := s.dropped(goalID)
	if err != nil {
		return "", err
	}
	return s.sweep.Plan(ctx, s.top, goalID, dropped)
}

func (s goalSweeper) run(ctx context.Context, goalID string) error {
	if s.sweep.Sweep == nil {
		return errors.New("this engine has no goal-branch sweep registered")
	}
	dropped, err := s.dropped(goalID)
	if err != nil {
		return err
	}
	return s.sweep.Sweep(ctx, s.top, goalID, dropped)
}

// linkedWorktreeProofs adds the goal- and session-worktree proofs beside the
// workspace proof under each owner kind.
func linkedWorktreeProofs(proofs map[diskstore.OwnerKind]diskstore.OwnerProof, top string, layout stateroot.Layout, ledger *ledgerView, workspace diskstore.WorkspaceProof,
	now time.Time, grace time.Duration) {
	sweeper := goalSweeper{top: top, ledger: ledger, sweep: registeredGoalBranchSweep}
	proofs[diskstore.OwnerGoal] = diskstore.ClassProofs{Owner: diskstore.OwnerGoal, ByClass: map[string]diskstore.OwnerProof{
		diskstore.WorkspaceClass: workspace,
		diskstore.GoalWorktreeClass: diskstore.GoalWorktreeProof{GitRoot: layout.GitRoot, Git: ExecWorkspaceGit, Ended: workspace.Ended,
			Plan: sweeper.plan, Sweep: sweeper.run, Now: now},
	}}
	proofs[diskstore.OwnerSession] = diskstore.ClassProofs{Owner: diskstore.OwnerSession, ByClass: map[string]diskstore.OwnerProof{
		diskstore.WorkspaceClass: workspace,
		diskstore.SessionWorktreeClass: diskstore.SessionWorktreeProof{GitRoot: layout.GitRoot, Git: ExecWorkspaceGit, Grace: grace, Now: now,
			Main: sessionMainLiveness(identity.KernelProber{}), BootstrapDead: bootstrapDead(identity.KernelProber{})},
	}}
}

// sessionMainLiveness reads the mains announced in a second session's
// checkout (the worktree, or the installation inside it), as health does.
func sessionMainLiveness(prober identity.Prober) func(path string) (diskstore.MainLiveness, string) {
	return func(path string) (diskstore.MainLiveness, string) {
		root := path
		if layout, err := stateroot.ResolveLayout(path); err == nil {
			root = layout.InstallationRoot
		}
		// No announcement with a session is no main, whatever else the
		// directory holds (Round D3 N10): a starting session is kept by its
		// reserved state, never judged dead from an absence.
		announced, err := announcedMains(filepath.Join(root, "artifacts", "agents", "mains"))
		if err != nil {
			return diskstore.MainUnknown, "the session's announcements cannot be read: " + err.Error()
		}
		if !announced {
			return diskstore.MainNone, "no main has announced itself in " + root
		}
		verdict := checkSessionMain(root, prober)
		switch verdict.Status {
		case HealthAlive:
			return diskstore.MainAlive, verdict.Reason
		case HealthDead:
			return diskstore.MainDead, verdict.Reason
		}
		return diskstore.MainUnknown, verdict.Reason
	}
}

// bootstrapDead probes a second session's recorded bootstrap process.
func bootstrapDead(prober identity.Prober) func(string) (bool, bool) {
	return func(encoded string) (bool, bool) {
		pid, started, err := diskstore.ParseBootstrapRef(encoded)
		if err != nil {
			return false, false
		}
		switch identity.AliveRef(prober, identity.Ref{Pid: pid, StartedAtSec: started}) {
		case identity.Dead:
			return true, true
		case identity.Alive:
			return false, true
		}
		return false, false
	}
}

// announcedMains reports whether the announcements directory holds a main
// announcement (a JSON file naming a session); an unreadable announcement
// is an error.
func announcedMains(directory string) (bool, error) {
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		value, err := readHealthObject(filepath.Join(directory, entry.Name()))
		if err != nil {
			return false, err
		}
		if session, _ := value["sessionId"].(string); session != "" {
			return true, nil
		}
	}
	return false, nil
}
