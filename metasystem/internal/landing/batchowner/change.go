package batchowner

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

// ChangeJoinRequest is one change a seat committed, joining the landing lane
// (U11b): the seat's installation, the lane checkout, the commit, and the tip
// the goal's landing gate was read at when the change was made in a goal's
// name.
type ChangeJoinRequest struct {
	SeatRoot, LandingRoot, Commit, GateTip string
	At                                     time.Time
}

// ChangePinRef is where a seat pins a change bound for the lane: the lane
// checkout fetches it from there, and the seat's repeat finds it there.
func ChangePinRef(commit string) string {
	return "refs/metasystem/changes/" + strings.TrimPrefix(batch.ChangeID(commit), "change:")
}

// changeJoinDependencies are the effects of a change's join (U11b).
type changeJoinDependencies struct {
	git            func(dir string, args ...string) (string, error)
	Base           func(string) (string, error)
	Mint           func() (string, error)
	assemble       func(string, string, []batch.Unit) ([]string, error)
	ProtectedTests func(string, string, string) error
	Closure        func(root, baseTree, tree string) *adapter.Closure
	onMain         func(lane, commit string) (bool, error)
	Ensure         func(string) error
	prober         identity.Prober
}

func ProductionChangeJoinDependencies() changeJoinDependencies {
	return changeJoinDependencies{
		git: func(dir string, args ...string) (string, error) {
			command := exec.Command("git", append([]string{"-C", dir}, args...)...)
			command.Env = gittree.ScrubbedEnviron()
			output, err := command.CombinedOutput()
			if err != nil {
				return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(output)), err)
			}
			return strings.TrimSpace(string(output)), nil
		},
		Base: fetchLandingBaseTree,
		Mint: func() (string, error) {
			id, err := goal.NewOperationULID()
			return strings.ToLower(id), err
		},
		assemble: batch.AssembleUnits, ProtectedTests: ProductionBatchProtectedTests, Closure: batch.UnitClosure,
		Ensure: EnsureBatchOwner, prober: identity.KernelProber{},
		onMain: func(lane, commit string) (bool, error) {
			return batchSeriesOnEndpoint(lane, "refs/remotes/origin/main", commit)
		},
	}
}

// changeParentStack is what a change's parent needs under it on the lane's
// base: nothing when the parent is on origin/main or a change that landed,
// the live changes it stacks on (oldest first) when it is one of the lane's.
// A parent that is neither is refused, named.
func changeParentStack(store batch.Store, lane, parent string, onMain func(string, string) (bool, error)) ([]batch.Unit, error) {
	var stack []batch.Unit
	for {
		if on, err := onMain(lane, parent); err != nil {
			return nil, fmt.Errorf("whether parent %s is on origin/main cannot be read: %w", parent, err)
		} else if on {
			return stack, nil
		}
		records, err := store.Records()
		if err != nil {
			return nil, err
		}
		var holder *batch.Unit
		for _, record := range records {
			for index := range record.Units {
				unit := record.Units[index]
				if unit.IsChange() && unit.Change.Commit == parent && slices.Contains([]string{batch.UnitJoining, batch.UnitJoined, batch.UnitLanded}, unit.State) {
					holder = &unit
				}
			}
		}
		switch {
		case holder == nil:
			return nil, fmt.Errorf("its parent %s is not on origin/main nor in the landing lane; rebase it onto origin/main", parent)
		case holder.State == batch.UnitLanded:
			return stack, nil
		}
		stack = append([]batch.Unit{*holder}, stack...)
		parent = holder.Change.Parent
	}
}

// ExecuteChangeJoin joins one change the seat committed and pinned: the lane
// checkout fetches the pin from the seat (the same host), reads the commit's
// facts and trailers, checks the change alone on the current base (it
// applies, and it keeps every protected test), and joins it under the store
// lock. No plan, admission run, forecast or handover: a change holds no goal.
func ExecuteChangeJoin(request ChangeJoinRequest, dependencies changeJoinDependencies) (batch.Record, error) {
	id, ref, lane := batch.ChangeID(request.Commit), ChangePinRef(request.Commit), request.LandingRoot
	unreadable := func(what string, err error) error {
		return fmt.Errorf("%s: change %s: %s: %w", codeChangeUnreadable, id, what, err)
	}
	seatTop, err := dependencies.git(request.SeatRoot, "rev-parse", "--show-toplevel")
	if err != nil {
		return batch.Record{}, unreadable("the seat's repository", err)
	}
	if _, err := dependencies.git(lane, "fetch", "--quiet", seatTop, "+"+ref+":"+ref); err != nil {
		return batch.Record{}, unreadable("fetch its pin into the lane", err)
	}
	if fetched, err := dependencies.git(lane, "rev-parse", "--verify", "--quiet", ref+"^{commit}"); err != nil || fetched != request.Commit {
		return batch.Record{}, unreadable("the pin names "+fetched+", not "+request.Commit, err)
	}
	parent, err := dependencies.git(lane, "rev-parse", request.Commit+"^")
	if err != nil {
		return batch.Record{}, unreadable("its parent", err)
	}
	message, err := dependencies.git(lane, "show", "-s", "--format=%B", request.Commit)
	if err != nil {
		return batch.Record{}, unreadable("its message", err)
	}
	change := batch.ChangeMember{Commit: request.Commit, Parent: parent, GateTip: request.GateTip}
	change.Subject, _, _ = strings.Cut(message, "\n")
	for _, line := range strings.Split(message, "\n") {
		key, value, found := strings.Cut(strings.TrimSpace(line), ": ")
		switch {
		case !found:
		case key == "Machine":
			change.AskedBy = value
		case key == "Goal-Item":
			change.Goal = value
		case key == "Goal-Revision":
			change.GoalRevision, _ = strconv.ParseUint(value, 10, 64)
		}
	}
	machine, lineage, found := strings.Cut(change.AskedBy, "+")
	if !found || machine == "" || lineage == "" {
		return batch.Record{}, unreadable("its Machine trailer", fmt.Errorf("%q names no machine and session; land the change with metasystem work land", change.AskedBy))
	}
	patch, err := dependencies.git(lane, "diff", "--binary", "--full-index", request.Commit+"^", request.Commit)
	if err != nil {
		return batch.Record{}, unreadable("its patch", err)
	}
	var paths []string
	for path := range batch.ChangedPaths([]byte(patch + "\n")) {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	unit := batch.NewChangeUnit(change, request.SeatRoot, machine, lineage, paths, nil)
	baseTree, err := dependencies.Base(lane)
	if err != nil {
		return batch.Record{}, err
	}
	store := batch.NewStore(lane, dependencies.prober)
	// The change's parent is on main, or a change the lane holds (the seat
	// committed on top of one not landed yet); anything else would land a
	// commit whose parent main never saw (N-3).
	stack, err := changeParentStack(store, lane, parent, dependencies.onMain)
	if err != nil {
		return batch.Record{}, fmt.Errorf("%s: change %s: %w", codeChangeParentUnknown, id, err)
	}
	prefixes, err := dependencies.assemble(lane, baseTree, append(stack, unit))
	if err != nil || len(prefixes) != len(stack)+1 {
		return batch.Record{}, fmt.Errorf("prepare change %s on the lane's base: prefixes=%d: %w", id, len(prefixes), err)
	}
	tree := prefixes[len(prefixes)-1]
	if err := dependencies.ProtectedTests(lane, baseTree, tree); err != nil {
		return batch.Record{}, err
	}
	unit.Closure = dependencies.Closure(lane, baseTree, tree)
	newID, err := dependencies.Mint()
	if err != nil {
		return batch.Record{}, err
	}
	record, err := batch.JoinChange(store,
		batch.ChangeJoin{Unit: unit, BaseTree: baseTree, NewID: newID, Actor: change.AskedBy, At: request.At})
	if err != nil {
		return batch.Record{}, err
	}
	if err := dependencies.Ensure(lane); err != nil {
		// The change is the lane's now; only its owner did not start (N-a).
		return record, &ChangeOwnerStartError{Record: record, Cause: err}
	}
	return record, nil
}

// ChangeOwnerStartError is a join that wrote the member but could not start
// the lane's owner: the change is joined and nothing is given back.
type ChangeOwnerStartError struct {
	Record batch.Record
	Cause  error
}

func (err *ChangeOwnerStartError) Error() string { return err.Cause.Error() }
func (err *ChangeOwnerStartError) Unwrap() error { return err.Cause }

// batchLaneAccount resolves the accounting identity of the host lane whose
// checkout is root: what a batch of changes is charged to (U11b). An
// unresolvable lane is LANE_ACCOUNT_UNRESOLVED.
func batchLaneAccount(root string) (string, error) {
	home, err := board.Home()
	if err != nil {
		return "", &refusal.Coded{Code: lane.CodeAccountUnresolved, Reason: fmt.Errorf("this computer's landing lane cannot be found: %w", err)}
	}
	return lane.ResolveAccount(home, batch.ModuleRoot(root))
}

// batchChargeID is the identity a proof keyed to unit is accounted to: a
// goal member's goal, or, for a change (every member a change), the lane.
func batchChargeID(root string, unit batch.Unit, account func(string) (string, error)) (string, error) {
	if !unit.IsChange() && !strings.HasPrefix(unit.GoalID, "change:") {
		return unit.GoalID, nil
	}
	if account == nil {
		account = batchLaneAccount
	}
	return account(root)
}

// accountFlag names who a batch proof is charged to on its argv: a goal, or
// the lane (U11b).
func accountFlag(id string) []string {
	if lane.IsAccount(id) {
		return []string{"--lane", id}
	}
	return []string{"--goal", id}
}

// accountRevisions are a goal's sealed revisions on the argv; the lane has
// none.
func accountRevisions(id string, claim batch.Claim) []string {
	if lane.IsAccount(id) {
		return nil
	}
	return []string{"--expected-goal-revision", fmt.Sprint(claim.Revision), "--expected-accounting-revision", fmt.Sprint(claim.AccountingRevision)}
}

// LaneSpend is what the lane at root charged to its own account: every
// retained attempt accounted to it (U11b).
func LaneSpend(root, account string) (lane.Spend, error) {
	attempts, err := proofrun.ReadAttempts(batch.ModuleRoot(root))
	if err != nil {
		return lane.Spend{}, err
	}
	spend := lane.Spend{Account: account}
	for _, attempt := range attempts {
		if attempt.AccountedGoal() == account {
			spend.Attempts++
			spend.ReservedMinutes += attempt.ReservedMinutes
		}
	}
	return spend, nil
}

// laneRegistrar names who registered the host lane and whether that is a
// person: a name with a complete goal.human.<name> identity in the lane
// checkout's metasystem.conf, as a goal's approver is bound (U11b).
func laneRegistrar(controlRoot string) (string, bool) {
	home, err := board.Home()
	if err != nil {
		return "", false
	}
	record, ok, err := lane.Read(home)
	if err != nil || !ok {
		return "", false
	}
	name := strings.TrimSpace(record.RegisteredBy)
	if name == "" || strings.ContainsAny(name, " /+") {
		return name, false
	}
	value, _, err := config.Get(config.GetParams{Key: "goal.human." + strings.ToLower(name), ConfPath: filepath.Join(controlRoot, "metasystem.conf")})
	return name, err == nil && strings.Contains(value, "@")
}
