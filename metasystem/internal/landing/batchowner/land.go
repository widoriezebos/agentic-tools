package batchowner

// The lane's landed-trailer recovery (lane design r10 §1 step 3): a batch
// whose series is on main is recognized member by member from origin's
// trailers and finalized; and the reads of origin's main the lane's verbs
// share.

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

var BatchGoalBranchSweep = goalbranch.Sweep
var BatchRecoveryGoalNext = func(root, goalID string, at time.Time) (string, error) {
	endpoint, err := goalbranch.MainEndpoint(root)
	if err != nil {
		return "", err
	}
	projection, err := goal.Project(endpoint, true, at)
	if err != nil {
		return "", err
	}
	for _, goals := range []map[string]*goal.GoalFile{projection.Tree.Live, projection.Tree.Done, projection.Tree.Abandoned} {
		if file := goals[goalID]; file != nil {
			return file.NextStep, nil
		}
	}
	return "", fmt.Errorf("goal %s is absent from the current ledger", goalID)
}

func batchSeriesOnEndpoint(root, origin, tip string) (bool, error) {
	return batchSeriesOnEndpointWithRunner(root, origin, tip, func(cmd *exec.Cmd) error { return cmd.Run() })
}

func batchSeriesOnEndpointWithRunner(root, origin, tip string, runGit func(*exec.Cmd) error) (bool, error) {
	if origin == "" || tip == "" {
		return false, nil
	}
	command := exec.Command("git", "-C", root, "merge-base", "--is-ancestor", tip, origin)
	command.Env = gittree.ScrubbedEnviron()
	err := runGit(command)
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}
func FetchBatchOrigin(root string) (string, string, error) {
	command := exec.Command("git", "-C", root, "fetch", "origin", "+refs/heads/main:refs/remotes/origin/main")
	command.Env = gittree.ScrubbedEnviron()
	if output, err := command.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("%s: fetch origin/main: %s: %w", codeLandPushRefused, strings.TrimSpace(string(output)), err)
	}
	commit, err := GitOutput(root, "rev-parse", "refs/remotes/origin/main")
	if err != nil {
		return "", "", err
	}
	tree, err := GitOutput(root, "rev-parse", commit+"^{tree}")
	return commit, tree, err
}

// RecoverySeams are the landed-trailer recovery seams of the lane whose
// checkout is root and whose installation is controlRoot, editing goals
// through calls under the authority invoke names, read when each edit is
// made. With baseTree set, a trailer
// counts only on a first-parent commit of origin/main after the newest one
// whose tree is baseTree (the batch's base): an earlier landing of the same
// source, chain or change, reverted since, is never taken for this batch's.
// A base that is not on main finds nothing.
func RecoverySeams(root, controlRoot, baseTree string, store batch.Store, id string, at time.Time, gitRead func(string, ...string) (string, error), calls *BatchOwnerCallSet, invoke func() (ownercall.Invocation, error)) batch.RecoverySeams {
	findTrailer := func(matches func(string) bool) (string, bool, error) {
		// Each commit is its hash, its tree when bounded, and its message.
		format, width := "%H%x00%B%x00", 2
		if baseTree != "" {
			format, width = "%H%x00%T%x00%B%x00", 3
		}
		output, err := gitRead(root, "log", "--first-parent", "origin/main", "--format="+format)
		if err != nil {
			return "", false, err
		}
		parts := strings.Split(output, "\x00")
		limit := len(parts)
		if baseTree != "" {
			limit = -1
			for index := 0; index+2 < len(parts); index += 3 {
				if strings.TrimSpace(parts[index+1]) == baseTree {
					limit = index
					break
				}
			}
			if limit < 0 {
				return "", false, nil
			}
		}
		for index := 0; index+width-1 < limit; index += width {
			for _, line := range strings.Split(parts[index+width-1], "\n") {
				if matches(strings.TrimSpace(line)) {
					return strings.TrimSpace(parts[index]), true, nil
				}
			}
		}
		return "", false, nil
	}
	return batch.RecoverySeams{
		OriginCommit: func(unit batch.Unit) (string, bool, error) {
			return findTrailer(func(line string) bool { return landingProvenanceNamesChain(line, unit.Chain) })
		},
		OriginChange: func(unit batch.Unit) (string, bool, error) {
			return findTrailer(func(line string) bool { return line == batch.LandingChangeTrailer+": "+unit.GoalID })
		},
		OriginSource: func(_ batch.Unit, source string) (string, bool, error) {
			return findTrailer(func(line string) bool { return line == "Goal-Source: "+source })
		},
		SweepGoalBranch: func(unit batch.Unit, _ string) error {
			endpoint, err := goalbranch.MainEndpoint(controlRoot)
			if err != nil {
				return err
			}
			record, err := store.Load(id)
			if err != nil {
				return err
			}
			transport := ""
			if _, remoteErr := goalbranch.ScrubbedGit(controlRoot, "remote", "get-url", "transport"); remoteErr == nil {
				transport = "transport"
			}
			// Inside the critical section of the goal's registered
			// worktrees, as goal done's sweep (Round D3 N1).
			return steward.SweepGoalWorktrees(controlRoot, unit.GoalID, func(ctx context.Context) error {
				_, err := BatchGoalBranchSweep(goalbranch.SweepRequest{Repo: controlRoot, Remote: endpoint.Remote, Transport: transport,
					EndpointTip: record.Landing.PushedTip, GoalID: unit.GoalID, CheckClaim: Engine.BranchClaimCheck(controlRoot, unit.GoalID, endpoint), Context: ctx})
				return err
			})
		},
		Finalize: func(unit batch.Unit, commit string) error {
			next := RecoveredBatchNext(unit, commit)
			current, err := BatchRecoveryGoalNext(controlRoot, unit.GoalID, at)
			if err != nil {
				return err
			}
			if current == next {
				return nil
			}
			invocation, err := invoke()
			if err != nil {
				return err
			}
			return calls.EditNext(invocation, controlRoot, unit.GoalID, next)
		},
		Cleanup: func() error {
			record, err := store.Load(id)
			if err != nil {
				return err
			}
			return batch.CleanupLandingBranch(root, id, record.Landing.PushedTip)
		},
		Release: ReleaseMemberSet(id, at),
	}
}

// landingProvenanceNamesChain mirrors the field parsing in internal/landing/held.go.
func landingProvenanceNamesChain(line, chain string) bool {
	value, found := strings.CutPrefix(line, "Landing-Provenance:")
	if !found {
		return false
	}
	for _, field := range strings.Fields(value) {
		if field == "chain="+chain {
			return true
		}
	}
	return false
}

func RecoveredBatchNext(unit batch.Unit, commit string) string {
	if len(unit.CommitIDs) != 0 {
		return "landed commit:" + commit + ":source=" + unit.CommitIDs[len(unit.CommitIDs)-1]
	}
	return "landed commit:" + commit + ":chain=" + unit.Chain
}

func GitOutput(root string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	output, err := command.Output()
	return strings.TrimSpace(string(output)), err
}
