package batch

import (
	"errors"
	"os"
	"testing"
	"time"
)

func returnBed(t *testing.T) Store {
	store := NewStore(t.TempDir(), scriptedProber{})
	must(t, store.Create(Record{Schema: 1, BatchID: testBatchID, State: StateOpen, Units: []Unit{{GoalID: "goal-a", Chain: "chain-a", Claim: Claim{Machine: "seat", Lineage: "lineage-a", Epoch: 4, Revision: 2, AccountingRevision: 1}, State: UnitJoined}}}))
	return store
}

func TestBatchReturnIsCrashSafeAndOnce(t *testing.T) {
	tests := []struct {
		name, target, crash, wantDisposition, holder string
		readError                                    bool
		wantHandBack, wantRelease                    int
	}{
		{"crash before return", ReturnTargetLive, "before-return", ReturnHandedBack, "", false, 1, 0},
		{"crash after hand-back", ReturnTargetLive, "after-return", ReturnAlreadyReturned, "", false, 1, 0},
		{"dead target", ReturnTargetDead, "", ReturnReleased, "", false, 0, 1},
		{"restarted target", ReturnTargetRestarted, "", ReturnReleased, "", false, 0, 1},
		{"unknown then live", ReturnTargetUnknown, "", ReturnHandedBack, "", false, 1, 0},
		{"ledger read error", ReturnTargetLive, "", ReturnHandedBack, "", true, 1, 0},
		{"claimed under another batch", ReturnTargetLive, "", ReturnAlreadyReturned, "other-batch", false, 0, 0},
		{"source pair holds under this batch", ReturnTargetLive, "", ReturnAlreadyReturned, "source", false, 0, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := returnBed(t)
			must(t, RequestReturn(store, testBatchID, "goal-a", UnitEjected, "broken proof", "landing+owner", time.Unix(1, 0)))
			ledger := ReturnLedgerGoal{Claimed: true, Machine: "landing", Lineage: "owner", Batch: testBatchID, Next: "Repair the unit."}
			switch test.holder {
			case "other-batch":
				ledger.Batch = "other-batch"
			case "source":
				ledger.Machine, ledger.Lineage = "seat", "lineage-a"
			}
			handBacks, releases, reads, targets, crashed := 0, 0, 0, 0, false
			store.seams.publish = func(point string) error {
				if point == test.crash && !crashed {
					crashed = true
					return os.ErrProcessDone
				}
				return nil
			}
			seams := ReturnSeams{
				Read: func(root, tree, goalID string) (ReturnLedgerGoal, error) {
					reads++
					if test.readError && reads == 1 {
						return ReturnLedgerGoal{}, errors.New("parse failed")
					}
					return ledger, nil
				},
				Target: func(source Claim) ReturnTarget {
					targets++
					if test.target == ReturnTargetUnknown && targets == 1 {
						return ReturnTarget{State: ReturnTargetUnknown}
					}
					state := test.target
					if state == ReturnTargetUnknown {
						state = ReturnTargetLive
					}
					return ReturnTarget{State: state, Epoch: 9}
				},
				HandBack: func(_ string, source Claim, epoch uint64) error {
					handBacks++
					witness(t, source.Machine == "seat" && epoch == 9, "hand-back source=%+v epoch=%d", source, epoch)
					ledger.Machine, ledger.Lineage, ledger.Batch = source.Machine, source.Lineage, ""
					return nil
				},
				Release: func(_ string, next string) error {
					releases++
					witness(t, next == "ejected: broken proof; Repair the unit.", "release next=%q", next)
					ledger.Claimed = false
					return nil
				},
			}
			firstErr := ReturnUnits(store, testBatchID, "tree-a", "landing+owner", time.Unix(2, 0), seams)
			witness(t, test.crash == "" || errors.Is(firstErr, os.ErrProcessDone), "crash error=%v", firstErr)
			if load(t, store).Units[0].State == UnitReturnPending {
				must(t, ReturnUnits(store, testBatchID, "tree-a", "landing+owner", time.Unix(3, 0), seams))
			}
			path, _ := store.recordPath(testBatchID)
			before := string(contents(t, path))
			must(t, ReturnUnits(store, testBatchID, "tree-a", "landing+owner", time.Unix(4, 0), seams))
			must(t, RequestReturn(store, testBatchID, "goal-a", UnitEjected, "broken proof", "landing+owner", time.Unix(5, 0)))
			witness(t, RequestReturn(store, testBatchID, "goal-a", UnitLanded, "other", "landing+owner", time.Unix(6, 0)) != nil, "terminal outcome was replaced")
			unit := load(t, store).Units[0]
			witness(t, unit.State == UnitEjected && unit.ReturnDisposition == test.wantDisposition && handBacks == test.wantHandBack && releases == test.wantRelease && string(contents(t, path)) == before, "unit=%+v hand-backs=%d releases=%d second tick changed=%v", unit, handBacks, releases, string(contents(t, path)) != before)
		})
	}
}

func TestBatchTerminalStateRequiresReturnPending(t *testing.T) {
	store := returnBed(t)
	err := store.Update(testBatchID, func(record *Record) error {
		record.Units[0].State, record.Units[0].Outcome, record.Units[0].ReturnDisposition = UnitEjected, UnitEjected, ReturnReleased
		return nil
	})
	witness(t, err != nil, "direct terminal write was accepted")
	err = store.Update(testBatchID, func(record *Record) error {
		unit := record.Units[0]
		unit.GoalID, unit.Chain, unit.State = "goal-b", "chain-b", UnitLanded
		record.Units = append(record.Units, unit)
		return nil
	})
	witness(t, err != nil, "new terminal unit was accepted")
	unit := load(t, store).Units[0]
	unit.State, unit.Outcome, unit.ReturnDisposition = UnitLanded, UnitLanded, ReturnReleased
	witness(t, NewStore(t.TempDir(), scriptedProber{}).Create(Record{Schema: 1, BatchID: testBatchID, State: StateOpen, Units: []Unit{unit}}) != nil, "new record started with a terminal unit")
	for _, fields := range []unitRecordFields{{Failure: "reason"}, {Outcome: UnitEjected}, {Outcome: UnitEjected, Failure: "reason", ReturnDisposition: ReturnReleased}} {
		err = store.Update(testBatchID, func(record *Record) error {
			record.Units[0].State, record.Units[0].unitRecordFields = UnitReturnPending, fields
			return nil
		})
		witness(t, err != nil, "malformed return-pending unit was accepted: %+v", fields)
	}
	must(t, RequestReturn(store, testBatchID, "goal-a", UnitEjected, "reason", "landing+owner", time.Unix(1, 0)))
	for name, change := range map[string]func(*Unit){"missing disposition": func(unit *Unit) { unit.State = unit.Outcome }, "unknown disposition": func(unit *Unit) { unit.State, unit.ReturnDisposition = unit.Outcome, "other" }, "wrong outcome": func(unit *Unit) { unit.State, unit.ReturnDisposition = UnitLanded, ReturnReleased }} {
		err = store.Update(testBatchID, func(record *Record) error { change(&record.Units[0]); return nil })
		witness(t, err != nil, "%s was accepted", name)
	}
	must(t, store.Update(testBatchID, func(record *Record) error { record.Units[0].State = UnitJoining; return nil }))
	handBacks := 0
	store.seams.publish = func(string) error { handBacks++; return nil }
	must(t, ReconcileJoins(store, testBatchID, "tree-a", "landing+owner", time.Unix(2, 0), func(string, string, string, string) (Claim, error) { return Claim{}, os.ErrNotExist }))
	must(t, ReconcileJoins(store, testBatchID, "tree-a", "landing+owner", time.Unix(3, 0), nil))
	unit = load(t, store).Units[0]
	witness(t, unit.State == UnitReturnPending && unit.Outcome == UnitEjected && unit.Failure == "join-incomplete" && unit.ReturnDisposition == "" && handBacks == 0, "join failure returned instead of pending: %+v", unit)
}
