package plain

import (
	"strings"

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
	waiting, err := Waiting(install)
	if err != nil || len(waiting) == 0 {
		return err
	}
	main, err := checkoutGit(checkout, seams).main()
	if err != nil {
		return err
	}
	incidents, err := seams.incidents(install, checkout, main)
	if err != nil || len(openIncidents(incidents)) == 0 {
		return err
	}
	if err := seams.fetchMain(checkout); err != nil {
		return err
	}
	main, err = checkoutGit(checkout, seams).main()
	if err != nil {
		return err
	}
	incidents, err = seams.incidents(install, checkout, main)
	if err != nil {
		return err
	}
	needed, err := movedIncidentMain(install, checkout, main, incidents, seams)
	if err != nil || !needed {
		return err
	}
	if seams.TimerHeld != nil && seams.TimerHeld() {
		return nil
	}
	seams.Trunk, seams.Gate, seams.Impact = true, false, false
	_, _, err = Start(install, checkout, seams)
	return err
}
