package batch

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

const ReturnTargetLive, ReturnTargetDead, ReturnTargetRestarted, ReturnTargetOccupied, ReturnTargetUnknown = "live-same-instance", "dead", "restarted", "occupied", "unknown"

type ReturnLedgerGoal struct {
	Claimed                       bool
	Machine, Lineage, Batch, Next string
	ClaimEpoch                    uint64
}
type ReturnTarget struct {
	State  string
	Epoch  uint64
	Reason string
}
type ReturnSeams struct {
	Read     func(root, tree, goalID string) (ReturnLedgerGoal, error)
	Target   func(Unit) ReturnTarget
	HandBack func(goalID string, source Claim, reboundEpoch uint64) error
	Release  func(goalID, next string) error
}

// ReadReturnLedgerGoal reads return custody from the exact fetched tree used
// for the rest of an owner tick.
func ReadReturnLedgerGoal(root, tree, goalID string) (ReturnLedgerGoal, error) {
	return readReturnLedgerGoalWithWorkspace(gittree.Workspace{Dir: root}, tree, goalID)
}

func readReturnLedgerGoalWithWorkspace(workspace gittree.Workspace, tree, goalID string) (ReturnLedgerGoal, error) {
	prefix, err := workspace.Prefix()
	if err != nil {
		return ReturnLedgerGoal{}, err
	}
	path := prefix + filepath.ToSlash(filepath.Join("plans", "goals", goalID+".md"))
	data, present, err := workspace.FileAt(tree, path)
	if err != nil || !present {
		return ReturnLedgerGoal{}, fmt.Errorf("goal ledger entry %s is absent from tree %s: %w", goalID, tree, err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		return ReturnLedgerGoal{}, fmt.Errorf("goal ledger entry %s is invalid: %v", goalID, problems)
	}
	out := ReturnLedgerGoal{Next: file.NextStep}
	if file.Claimed != nil {
		out.Claimed, out.Machine, out.Lineage = true, file.Claimed.Machine, file.Claimed.Lineage
		out.Batch = file.Claimed.HandedOver.Batch
		if file.StopCapability != nil {
			if file.StopCapability.ClaimEpoch > 0 {
				out.ClaimEpoch = uint64(file.StopCapability.ClaimEpoch)
			}
		}
	}
	return out, nil
}

func RequestReturn(store Store, batchID, goalID, outcome, reason, actor string, at time.Time) error {
	return store.Update(batchID, func(record *Record) error { return requestUnitReturn(record, goalID, outcome, reason, actor, at) })
}
func requestUnitReturn(record *Record, goalID, outcome, reason, actor string, at time.Time) error {
	for index := range record.Units {
		unit := &record.Units[index]
		if unit.GoalID == goalID {
			if (unit.State == UnitReturnPending || terminalUnitState(unit.State)) && unit.Outcome == outcome && unit.Failure == reason {
				return nil
			}
			if unit.State != UnitJoining && unit.State != UnitJoined {
				return fmt.Errorf("unit %s cannot request return from %s", goalID, unit.State)
			}
			from := unit.State
			unit.State, unit.Outcome, unit.Failure, unit.ReturnDisposition = UnitReturnPending, outcome, reason, ""
			appendUnitHistory(record, at, "return-request", actor, goalID, from, UnitReturnPending)
			return nil
		}
	}
	return fmt.Errorf("batch unit %s is absent", goalID)
}
func ReturnUnits(store Store, batchID, tree, actor string, at time.Time, seams ReturnSeams) error {
	return store.locked(func() error {
		record, err := store.Load(batchID)
		if err != nil {
			return err
		}
		for _, unit := range record.Units {
			if unit.State != UnitReturnPending {
				continue
			}
			if err := store.seams.publish("before-return"); err != nil {
				return err
			}
			ledger, readErr := seams.Read(store.root, tree, unit.GoalID)
			if readErr != nil {
				continue
			}
			if !ledger.Claimed || ledger.Batch != batchID || ledger.Machine == unit.Claim.Machine && ledger.Lineage == unit.Claim.Lineage {
				if err := settleReturn(store, batchID, unit.GoalID, ReturnAlreadyReturned, actor, at); err != nil {
					return err
				}
				continue
			}
			target := seams.Target(unit)
			switch target.State {
			case ReturnTargetLive:
				err = seams.HandBack(unit.GoalID, unit.Claim, target.Epoch)
			case ReturnTargetDead, ReturnTargetRestarted, ReturnTargetOccupied:
				next := unit.Outcome + ": " + unit.Failure
				if unit.LastUnit != "" {
					next = unit.Outcome + " " + unit.GoalID + " through " + unit.LastUnit + ": " + unit.Failure
				}
				if target.Reason != "" {
					next = target.Reason + "; " + next
				}
				if ledger.Next != "" {
					next += "; " + ledger.Next
				}
				err = seams.Release(unit.GoalID, next)
			default:
				continue
			}
			if err != nil {
				return err
			}
			if err := store.seams.publish("after-return"); err != nil {
				return err
			}
			disposition := ReturnHandedBack
			if target.State != ReturnTargetLive {
				disposition = ReturnReleased
			}
			if err := settleReturn(store, batchID, unit.GoalID, disposition, actor, at); err != nil {
				return err
			}
		}
		return nil
	})
}

func settleReturn(store Store, batchID, goalID, disposition, actor string, at time.Time) error {
	var settled Unit
	err := store.updateLocked(batchID, func(record *Record) error {
		for index := range record.Units {
			unit := &record.Units[index]
			if unit.GoalID == goalID && unit.State == UnitReturnPending {
				unit.State, unit.ReturnDisposition = unit.Outcome, disposition
				appendUnitHistory(record, at, "return", actor, goalID, UnitReturnPending, unit.State)
				settled = *unit
				return nil
			}
		}
		return fmt.Errorf("return-pending unit %s is absent", goalID)
	})
	if err == nil {
		stage := board.StageReturned
		if settled.State == UnitLanded {
			stage = board.StageLanded
		}
		writeUnitCard(goalID, settled.Claim.Machine, stage, batchID, at)
	}
	return err
}

// boardHistory is the goal's closed stage spans on the host board, copied
// into the unit at its join.
func boardHistory(goalID string) []board.StageSpan {
	home, err := board.Home()
	if err != nil {
		return nil
	}
	return board.History(home, goalID)
}

// writeUnitCard projects a lane transition onto the goal's card (D14, R24):
// joined at the join, landed or returned when the unit settles. The card is
// the goal's live card on whichever seat holds it, or, when the claim was
// released on the way back, the seat the unit joined from. A card that cannot
// be written is reported; the batch record stands.
func writeUnitCard(goalID, machine string, stage board.Stage, batchID string, at time.Time) {
	home, err := board.Home()
	if err != nil {
		return
	}
	seat, ok := board.Seat{}, false
	if live, found := board.LiveCard(home, goalID); found {
		seat, ok = live.Seat, true
	} else if installation, found := board.SeatInstallation(home, machine); found {
		seat, ok = board.Seat{Machine: machine, Installation: installation}, true
	}
	if !ok {
		return
	}
	card := board.Card{Seat: seat, Goal: goalID, Stage: stage, Batch: batchID, Writer: board.Writer{Component: "landing-batch", At: at}}
	if err := board.Write(card); err != nil {
		fmt.Fprintf(os.Stderr, "batch %s: goal %s: the board card was not written: %v\n", batchID, goalID, err)
	}
}
