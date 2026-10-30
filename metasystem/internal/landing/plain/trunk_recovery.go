package plain

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// movedIncidentMain identifies code that has not been checked since an open
// incident's last sighting. Record changes alone do not change its premise.
func movedIncidentMain(install, checkout, main string, incidents []goal.TrunkRedEntry, seams ProveSeams) (bool, error) {
	moved := false
	for _, incident := range incidents {
		if incident.Closed != nil || incident.EntryClass() != goal.TrunkRedClassTrunkRed || len(incident.Sightings) == 0 {
			continue
		}
		base := incident.Sightings[len(incident.Sightings)-1].BaseCommit
		if base == "" || base == main {
			continue
		}
		inside, err := checkoutGit(checkout, seams).contains(main, base)
		if err != nil {
			return false, err
		}
		if !inside {
			continue
		}
		changed, err := codeMoved(install, checkout, base, main, seams)
		if err != nil {
			return false, err
		}
		moved = moved || changed
	}
	if !moved {
		return false, nil
	}
	results, err := Results(install)
	if err != nil {
		return false, err
	}
	for i := len(results) - 1; i >= 0; i-- {
		proof := results[i]
		if !proof.Trunk || proof.ClassificationPending || proof.Commit == "" || proof.Result != Green && proof.Result != Red {
			continue
		}
		if proof.Commit != main {
			inside, err := checkoutGit(checkout, seams).contains(main, proof.Commit)
			if err != nil {
				return false, err
			}
			if !inside {
				continue
			}
			changed, err := codeMoved(install, checkout, proof.Commit, main, seams)
			if err != nil {
				return false, err
			}
			if changed {
				continue
			}
		}
		return false, nil
	}
	return true, nil
}

func codeMoved(install, checkout, from, to string, seams ProveSeams) (bool, error) {
	paths, err := seams.git(checkout, "diff", "--name-only", "--no-renames", from, to)
	if err != nil {
		return false, err
	}
	paths = strings.TrimSpace(paths)
	return paths != "" && !recordsOnlyPaths(install, checkout, strings.Split(paths, "\n")), nil
}

// RecoverMovedMain uses the same detached trunk admission as landing prove.
// It runs before question synchronization, so a stale exception cannot gate it.
func RecoverMovedMain(install, checkout string, seams ProveSeams) error {
	if err := seams.fetchMain(checkout); err != nil {
		return err
	}
	seams.Trunk, seams.Gate, seams.Impact = true, false, false
	main, tree, err := seams.subject(checkout)
	if err != nil {
		return err
	}
	if _, ok := applicableMainProof(install, checkout, main, tree, seams, true); ok {
		return nil
	}
	if seams.TimerHeld != nil && seams.TimerHeld() {
		return nil
	}
	running, already, err := Start(install, checkout, seams)
	if seams.RecoveryStarted != nil {
		seams.RecoveryStarted(running, already)
	}
	var busy *Busy
	var repair *RepairPending
	if errors.As(err, &busy) || errors.As(err, &repair) {
		return nil
	}
	return err
}

// applicableMainProof uses only recorded execution facts; cadence detects environment changes.
func applicableMainProof(install, checkout, commit, tree string, seams ProveSeams, red bool) (Result, bool) {
	results, damaged, err := countedLines[Result](resultsPath(install))
	if err != nil || damaged != 0 || commit == "" || tree == "" {
		return Result{}, false
	}
	environment := ""
	for i := len(results) - 1; i >= 0; i-- {
		r := results[i]
		if r.didNotRun() || r.Result != Green && r.Result != Red {
			continue
		}
		if environment == "" {
			environment = r.Environment
			if environment == "" {
				break
			}
		}
		if r.Tree != tree {
			continue
		}
		if r.ClassificationPending || r.Environment == "" || r.Environment != environment {
			break
		}
		if r.Result == Red {
			if r.Cause != nil && r.Cause.Kind == "main" && len(r.Failed) > 0 {
				incidents, err := seams.incidents(install, checkout, commit)
				if err != nil || len(r.incidentIDs(incidents)) == 0 {
					return Result{}, false
				}
			}
			return r, red && r.Trunk && r.Commit == commit
		}
		if r.flakeInherited() {
			for _, later := range results[i+1:] {
				if later.Result == Red && !later.didNotRun() {
					ancestor, err := checkoutGit(checkout, seams).contains(commit, later.Commit)
					if err != nil || ancestor {
						return Result{}, false
					}
				}
			}
		}
		fullAt, err := time.Parse(time.RFC3339, r.FullAt)
		currentFull := err == nil && seams.now().Sub(fullAt) >= 0 && (r.kept() || seams.now().Sub(fullAt) < time.Hour)
		inherited := r.Scope == "scoped" || r.Scope == "inherited" || strings.HasPrefix(r.Reason, "inherits green from tree ")
		if inherited && !currentFull {
			continue
		}
		coverage := r.Scope == "full" && r.FullTree == r.Tree || inherited && currentFull
		if _, err := time.Parse(time.RFC3339, r.At); err != nil {
			continue
		}
		if r.Scope != "impact" && r.reusableGreen(seams.now()) && (r.Trunk && r.Commit == commit || coverage) {
			return r, true
		}
	}
	return Result{}, false
}

// flakeRecordsReason marks a main check inherited across goal records and the
// incident register only; proof.trunk-every, not the scoped hour, bounds it.
// The value is stored in results and stays as first written.
const flakeRecordsReason = "main moved by flake records only; the last full green stands"

func (r Result) flakeInherited() bool {
	return r.Trunk && r.Result == Green && r.Scope == "inherited" && r.ScopeReason == flakeRecordsReason
}

// inheritFlakeRecords records that tree inherits main's last executed full
// green when every change since it is a goal record or the incident register.
// A later red, any other change or any unreadable fact records nothing, and the check runs.
func inheritFlakeRecords(install, checkout, commit, tree string, seams ProveSeams) error {
	results, damaged, err := countedLines[Result](resultsPath(install))
	if err != nil || damaged != 0 {
		return nil
	}
	var base Result
	newest, seen := "", false
	for i := len(results) - 1; i >= 0 && base.Commit == ""; i-- {
		r := results[i]
		if r.Tree == tree {
			return nil
		}
		inherited := r.Scope == "inherited" || strings.HasPrefix(r.Reason, "inherits green from tree ")
		if !seen && !r.didNotRun() && (r.Result == Green || r.Result == Red) && !inherited {
			newest, seen = r.Environment, true
			if newest == "" {
				return nil
			}
		}
		if r.Commit == "" || r.didNotRun() || r.Result != Red && (r.Result != Green || r.Scope != "full" || r.FullTree != r.Tree) {
			continue
		}
		if _, err := seams.git(checkout, "merge-base", "--is-ancestor", r.Commit, commit); err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) && exit.ExitCode() == 1 {
				continue
			}
			return nil
		}
		if r.Result == Red || r.Environment == "" {
			return nil
		}
		base = r
	}
	fullAt, err := time.Parse(time.RFC3339, base.FullAt)
	if base.Commit == "" || base.Environment != newest || err != nil || seams.now().Before(fullAt) || !goalRecordsBetween(install, checkout, base.FullTree, tree, seams) {
		return nil
	}
	return appendLine(resultsPath(install), Result{Trunk: true, Commit: commit, Tree: tree, Result: Green, Attempt: seams.newID(),
		At: seams.now().Format(time.RFC3339), Log: base.Log, Scope: "inherited", ScopeReason: flakeRecordsReason,
		Reason: "inherits green from tree " + Short(base.FullTree) + "; main moved by goal records only", Base: base.FullTree, BaseCommit: base.Commit,
		FullTree: base.FullTree, FullAt: base.FullAt, Environment: base.Environment}, seams)
}

// goalRecordsBetween says every path changed from one tree to the next is a
// goal page or the incident register, present in the new tree and read
// without problems by the ledger. Only the new tree is read.
func goalRecordsBetween(install, checkout, from, to string, seams ProveSeams) bool {
	changed, err := seams.git(checkout, "diff", "--name-only", "--no-renames", from, to)
	paths := strings.Split(strings.TrimSpace(changed), "\n")
	prefix, relErr := filepath.Rel(checkout, install)
	if err != nil || relErr != nil || strings.TrimSpace(changed) == "" || !recordsOnlyPaths(install, checkout, paths) {
		return false
	}
	after := map[string][]byte{}
	for _, path := range paths {
		name, inside := strings.CutPrefix(path, filepath.ToSlash(prefix)+"/")
		if prefix == "." {
			name, inside = path, true
		}
		if !inside || !goal.GoalRecordPath(name) {
			return false
		}
		now, err := seams.git(checkout, "show", to+":"+path)
		if err != nil {
			return false
		}
		after[name] = []byte(now)
	}
	return goal.GoalRecordsOnly(after)
}
