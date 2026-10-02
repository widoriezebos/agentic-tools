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

	"fmt"
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

// SweepGoalWorktrees runs a goal-branch sweep inside the critical section of
// every registered worktree of the goal (Part B 3.2 "Goal"; Round D3 N1):
// goal done's, a hand landing's last-landing sweep and the batch lane's P6
// sweep alike. Each record lock is taken without waiting and reloaded, the
// worktree judged by the workspace rules and the use census, every tip
// archived and read back, then the sweep runs. A goal with no registered
// worktree sweeps as before. A worktree an engine verb is inside, or one
// something still keeps, is left for the disk pass, which retries. The
// registry is the one the goal's worktrees are recorded in: the state root
// of root's installation.
func SweepGoalWorktrees(root, goalID string, sweep func(context.Context) error) error {
	layout, err := stateroot.ResolveLayout(root)
	if err != nil {
		return err
	}
	registry := diskstore.CheckoutRegistry(StoreControl(layout.InstallationRoot))
	records, err := diskstore.FindLinkedWorktrees(registry, diskstore.GoalWorktreeClass, diskstore.Owner{Kind: diskstore.OwnerGoal, Ref: goalID})
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return sweep(context.Background())
	}
	ids := make([]string, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.ID)
	}
	outcome, err := diskstore.ReleaseLinkedWorktrees(context.Background(), registry, ids, diskstore.LinkedRelease{
		GitRoot: layout.GitRoot, Git: ExecWorkspaceGit, Now: time.Now().UTC(), By: "the goal-branch sweep",
		TakeCensus: func() *diskstore.UseCensus {
			home, _ := HomeStateRoot()
			census := diskstore.TakeUseCensus(context.Background(), *KernelCensusReader(identity.KernelProcessTable{}, home, append(ArmedCheckouts(), root)))
			return &census
		},
		Remove: sweep})
	switch {
	case err != nil:
		return err
	case outcome.Done:
		return nil
	}
	return fmt.Errorf("goal/%s's worktree %s is kept for now: %s; the steward's disk pass retries the sweep (%s)", goalID, outcome.Path, outcome.Reason, outcome.Command)
}

// StoreControl is the control root whose checkout registry records the
// stores of an installation's goals and seats: its state root, else the
// installation itself (Round D3 N9: every route reads the same registry).
func StoreControl(installation string) string {
	if root, err := stateroot.RootForInstallation(installation); err == nil && filepath.IsAbs(root) {
		return root
	}
	return installation
}
