package plain

import (
	"errors"
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
		fullAt, err := time.Parse(time.RFC3339, r.FullAt)
		currentFull := err == nil && seams.now().Sub(fullAt) >= 0 && seams.now().Sub(fullAt) < time.Hour
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
