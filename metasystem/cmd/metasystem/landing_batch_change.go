package main

import (
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

// changeJoinDependencies are the effects of a change's join (U11b).
type changeJoinDependencies struct {
	git            func(dir string, args ...string) (string, error)
	base           func(string) (string, error)
	mint           func() (string, error)
	assemble       func(string, string, []batch.Unit) ([]string, error)
	protectedTests func(string, string, string) error
	closure        func(root, baseTree, tree string) *adapter.Closure
	ensure         func(string) error
	prober         identity.Prober
}

func productionChangeJoinDependencies() changeJoinDependencies {
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
		base: fetchLandingBaseTree,
		mint: func() (string, error) {
			id, err := goal.NewOperationULID()
			return strings.ToLower(id), err
		},
		assemble: batch.AssembleUnits, protectedTests: productionBatchProtectedTests, closure: batch.UnitClosure,
		ensure: ensureBatchOwner, prober: identity.KernelProber{},
	}
}

// executeChangeJoin joins one change the seat committed and pinned: the lane
// checkout fetches the pin from the seat (the same host), reads the commit's
// facts and trailers, checks the change alone on the current base (it
// applies, and it keeps every protected test), and joins it under the store
// lock. No plan, admission run, forecast or handover: a change holds no goal.
func executeChangeJoin(request changeJoinRequest, dependencies changeJoinDependencies) (batch.Record, error) {
	id, ref, lane := batch.ChangeID(request.Commit), changePinRef(request.Commit), request.LandingRoot
	unreadable := func(what string, err error) error {
		return fmt.Errorf("BATCH_CHANGE_UNREADABLE: change %s: %s: %w", id, what, err)
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
		return batch.Record{}, unreadable("its Machine trailer", fmt.Errorf("%q names no machine and lineage; a change is committed through the commit boundary", change.AskedBy))
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
	baseTree, err := dependencies.base(lane)
	if err != nil {
		return batch.Record{}, err
	}
	prefixes, err := dependencies.assemble(lane, baseTree, []batch.Unit{unit})
	if err != nil || len(prefixes) != 1 {
		return batch.Record{}, fmt.Errorf("prepare change %s on the lane's base: prefixes=%d: %w", id, len(prefixes), err)
	}
	if err := dependencies.protectedTests(lane, baseTree, prefixes[0]); err != nil {
		return batch.Record{}, err
	}
	unit.Closure = dependencies.closure(lane, baseTree, prefixes[0])
	newID, err := dependencies.mint()
	if err != nil {
		return batch.Record{}, err
	}
	record, err := batch.JoinChange(batch.NewStore(lane, dependencies.prober),
		batch.ChangeJoin{Unit: unit, BaseTree: baseTree, NewID: newID, Actor: change.AskedBy, At: request.At})
	if err != nil {
		return batch.Record{}, err
	}
	return record, dependencies.ensure(lane)
}
