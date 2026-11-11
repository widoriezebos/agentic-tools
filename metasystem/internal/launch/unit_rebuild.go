package launch

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

// buildNamed releases only a cancelled, quiescent run. The released marker
// keeps its provenance across an interrupted preparation or reservation.
// The caller holds the tree and name locks.
func (runner *UnitRunner) buildNamed(key string, plan UnitPlan) (namedUnitEntry, bool, error) {
	entry, found, err := runner.readNamed(key)
	if err != nil || !found {
		return entry, found, err
	}
	real, _, err := namedUnitIdentity(plan)
	if err != nil {
		return entry, true, err
	}
	if entry.Worktree != real || entry.Goal != plan.Goal || entry.Unit != plan.Unit || !idPattern.MatchString(entry.Run) ||
		entry.State != namedReserved && entry.State != namedRecorded && entry.State != namedReleased {
		return entry, true, coded("UNIT_NAMED_ENTRY_CORRUPT", "entry="+runner.namedPath(key, ".json"), errors.New("the saved unit identity is damaged"))
	}
	record, err := runner.read(entry.Run)
	if errors.Is(err, os.ErrNotExist) && entry.State == namedReserved {
		return entry, true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return entry, true, coded("UNIT_NAMED_RUN_MISSING", "run="+entry.Run, errors.New("the run recorded for this unit no longer exists"))
	}
	if err != nil {
		return entry, true, coded("UNIT_NAMED_RUN_UNREADABLE", "run="+entry.Run, err)
	}
	recorded, pathErr := filepath.EvalSymlinks(record.Worktree)
	if pathErr != nil || recorded != real || record.Goal != plan.Goal || record.Unit != plan.Unit || record.ID != entry.Run {
		return entry, true, coded("UNIT_NAMED_ENTRY_CORRUPT", "run="+entry.Run, errors.New("the saved unit points at a different run"))
	}
	if record.State != "cancelled" {
		return entry, true, nil
	}
	if released, ended := runner.treeQuiescent(treeReservation{Run: entry.Run}); !released {
		return entry, true, &TreeWaitingError{Run: entry.Run, CanCancel: ended}
	}
	archive := filepath.Join(runner.runDir(entry.Run), "released-inputs")
	if entry.State != namedReleased {
		if _, err := atomicfile.CopyFile(runner.namedPath(key, ".json"), filepath.Join(runner.runDir(entry.Run), "released-named.json"), runner.root()); err != nil {
			return entry, true, err
		}
		entry.State = namedReleased
		if err := runner.writeNamed(key, entry); err != nil {
			return entry, true, err
		}
	}
	if _, err := os.Stat(archive); errors.Is(err, os.ErrNotExist) {
		err := os.Rename(filepath.Join(runner.root(), ".inputs", key), archive)
		if errors.Is(err, os.ErrNotExist) {
			err = os.MkdirAll(archive, 0700)
		}
		if err != nil {
			return entry, true, err
		}
	} else if err != nil {
		return entry, true, err
	}
	return entry, false, nil
}

// buildBefore counts a stopped run's retained result only when it is the
// whole starting change against this run's goal tip.
func (runner *UnitRunner) buildBefore(record *UnitRunRecord, round *UnitRound, plan UnitPlan) error {
	current, err := runner.WorktreeDiff(plan.Worktree, plan.Base)
	if err != nil {
		return err
	}
	if record.StoppedRun != "" && round.Number == 1 {
		stopped, err := runner.read(record.StoppedRun)
		var kept []byte
		for i := len(stopped.Rounds) - 1; err == nil && i >= 0; i-- {
			kept, err = os.ReadFile(filepath.Join(stopped.Rounds[i].Directory, "worktree.diff"))
			if !errors.Is(err, os.ErrNotExist) {
				break
			}
			err = nil
		}
		note := fmt.Sprintf("run %s's retained change is unavailable; the current change is not counted", record.StoppedRun)
		if err == nil && kept != nil {
			if bytes.Equal(current, kept) {
				var lines int64
				for _, block := range parseDiff(current) {
					lines += block.lines
				}
				note = fmt.Sprintf("run %s's change (%d lines) is this round's starting result", record.StoppedRun, lines)
				current = nil
			} else {
				changed := map[string]string{}
				for _, diff := range [][]byte{kept, current} {
					for _, block := range parseDiff(diff) {
						header, text := block.text[0], strings.Join(block.text, "\n")
						if previous, ok := changed[header]; ok && previous == text {
							delete(changed, header)
						} else {
							changed[header] = text
						}
					}
				}
				var headers []string
				for header := range changed {
					files := diffFiles([]byte(header))
					if len(files) == 0 {
						files = []string{strings.TrimPrefix(header, "diff --git ")}
					}
					headers = append(headers, files...)
				}
				sort.Strings(headers)
				note = fmt.Sprintf("the current change differs from run %s's retained change in %s; it is not counted", record.StoppedRun, strings.Join(headers, ", "))
			}
		}
		round.StartingResult = note
		if err := runner.save(*record); err != nil {
			return err
		}
	}
	_, err = atomicfile.WriteText(filepath.Join(round.Directory, "build-before", "worktree.diff"), string(current), runner.root())
	return err
}
