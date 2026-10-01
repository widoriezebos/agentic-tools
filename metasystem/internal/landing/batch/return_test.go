package batch

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
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
		{"occupied source", ReturnTargetOccupied, "", ReturnReleased, "", false, 0, 1},
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
				Target: func(unit Unit) ReturnTarget {
					targets++
					if test.target == ReturnTargetUnknown && targets == 1 {
						return ReturnTarget{State: ReturnTargetUnknown}
					}
					state := test.target
					if state == ReturnTargetUnknown {
						state = ReturnTargetLive
					}
					reason := ""
					if state == ReturnTargetOccupied {
						reason = "source machine holds goal-b"
					}
					return ReturnTarget{State: state, Epoch: 9, Reason: reason}
				},
				HandBack: func(_ string, source Claim, epoch uint64) error {
					handBacks++
					witness(t, source.Machine == "seat" && epoch == 9, "hand-back source=%+v epoch=%d", source, epoch)
					ledger.Machine, ledger.Lineage, ledger.Batch = source.Machine, source.Lineage, ""
					return nil
				},
				Release: func(_ string, next string) error {
					releases++
					want := "ejected: broken proof; Repair the unit."
					if test.target == ReturnTargetOccupied {
						want = "source machine holds goal-b; " + want
					}
					witness(t, next == want, "release next=%q", next)
					ledger.Claimed = false
					return nil
				},
			}
			firstErr := returnUnitsErr(store, testBatchID, "tree-a", "landing+owner", time.Unix(2, 0), seams)
			witness(t, test.crash == "" || errors.Is(firstErr, os.ErrProcessDone), "crash error=%v", firstErr)
			if load(t, store).Units[0].State == UnitReturnPending {
				must(t, returnUnitsErr(store, testBatchID, "tree-a", "landing+owner", time.Unix(3, 0), seams))
			}
			path, _ := store.recordPath(testBatchID)
			before := string(contents(t, path))
			must(t, returnUnitsErr(store, testBatchID, "tree-a", "landing+owner", time.Unix(4, 0), seams))
			must(t, RequestReturn(store, testBatchID, "goal-a", UnitEjected, "broken proof", "landing+owner", time.Unix(5, 0)))
			witness(t, RequestReturn(store, testBatchID, "goal-a", UnitLanded, "other", "landing+owner", time.Unix(6, 0)) != nil, "terminal outcome was replaced")
			unit := load(t, store).Units[0]
			witness(t, unit.State == UnitEjected && unit.ReturnDisposition == test.wantDisposition && handBacks == test.wantHandBack && releases == test.wantRelease && string(contents(t, path)) == before, "unit=%+v hand-backs=%d releases=%d second tick changed=%v", unit, handBacks, releases, string(contents(t, path)) != before)
		})
	}
}

func TestBatchFailedHandbackSeesOccupiedSourceOnNextTick(t *testing.T) {
	store := returnBed(t)
	must(t, RequestReturn(store, testBatchID, "goal-a", UnitEjected, "broken proof", "landing+owner", time.Unix(1, 0)))
	targets, releases := 0, 0
	seams := ReturnSeams{
		Read: func(string, string, string) (ReturnLedgerGoal, error) {
			return ReturnLedgerGoal{Claimed: true, Machine: "landing", Lineage: "owner", Batch: testBatchID}, nil
		},
		Target: func(Unit) ReturnTarget {
			targets++
			if targets == 1 {
				return ReturnTarget{State: ReturnTargetLive, Epoch: 8}
			}
			return ReturnTarget{State: ReturnTargetOccupied, Reason: "source machine holds goal-b"}
		},
		HandBack: func(string, Claim, uint64) error { return errors.New("source stopped before handback") },
		Release: func(_ string, next string) error {
			releases++
			witness(t, next == "source machine holds goal-b; ejected: broken proof", "occupied Next=%q", next)
			return nil
		},
	}
	witness(t, returnUnitsErr(store, testBatchID, "tree-a", "landing+owner", time.Unix(2, 0), seams) != nil, "failed handback returned success")
	witness(t, load(t, store).Units[0].State == UnitReturnPending, "failed handback did not remain pending")
	must(t, returnUnitsErr(store, testBatchID, "tree-b", "landing+owner", time.Unix(3, 0), seams))
	unit := load(t, store).Units[0]
	witness(t, targets == 2 && releases == 1 && unit.State == UnitEjected && unit.ReturnDisposition == ReturnReleased,
		"targets=%d releases=%d unit=%+v", targets, releases, unit)
}

// TestBatchSettleWritesLandedOrReturned (R24, U10a-3, landed and returned):
// a unit that settles landed writes landed on the goal's card, one that
// settles ejected writes returned.
func TestBatchSettleWritesLandedOrReturned(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		outcome string
		want    board.Stage
	}{{UnitLanded, board.StageLanded}, {UnitEjected, board.StageReturned}} {
		goalID := "goal-card-" + row.outcome
		seat := board.Seat{Machine: "m1-batch-settle", Installation: "/checkouts/m1-batch-settle/metasystem"}
		seedBoardCard(t, seat, goalID, []board.Stage{board.StageClaimedIdle}, time.Date(2026, 9, 29, 9, 0, 0, 0, time.UTC))
		store := NewStore(t.TempDir(), scriptedProber{})
		must(t, store.Create(Record{Schema: 1, BatchID: testBatchID, State: StateOpen, Units: []Unit{{GoalID: goalID, Chain: "chain-a", Claim: Claim{Machine: seat.Machine, Lineage: "lineage-a", Epoch: 4, Revision: 2, AccountingRevision: 1}, State: UnitJoined}}}))
		must(t, RequestReturn(store, testBatchID, goalID, row.outcome, "settled", "landing+owner", time.Unix(1, 0)))
		ledger := ReturnLedgerGoal{Claimed: true, Machine: "landing", Lineage: "owner", Batch: testBatchID}
		must(t, returnUnitsErr(store, testBatchID, "tree-a", "landing+owner", time.Unix(2, 0), ReturnSeams{
			Read:     func(string, string, string) (ReturnLedgerGoal, error) { return ledger, nil },
			Target:   func(Unit) ReturnTarget { return ReturnTarget{State: ReturnTargetLive, Epoch: 9} },
			HandBack: func(string, Claim, uint64) error { return nil },
			Release:  func(string, string) error { return nil },
		}))
		home, _ := board.Home()
		picture, _ := board.Read(home, []board.Seat{seat}, nil, time.Unix(3, 0), time.Hour)
		found := false
		for _, card := range picture.Cards {
			if card.Goal == goalID {
				found = card.Stage == row.want && card.Batch == testBatchID
			}
		}
		if !found {
			t.Fatalf("%s: board %+v, want %s", row.outcome, picture, row.want)
		}
	}
}

// returnUnitsErr is ReturnUnits for a caller that reads only its error, as
// the owner's tick does.
func returnUnitsErr(store Store, batchID, tree, actor string, at time.Time, seams ReturnSeams) error {
	_, err := ReturnUnits(store, batchID, tree, actor, at, seams)
	return err
}

// Design r10 §1 step 4: ReturnUnits never skips a member in silence. An
// unreadable ledger entry and an unprovable seat are each listed with their
// reason, and a failed release is listed before its error ends the call;
// every listed member stays return-pending for the next try.
func TestReturnUnitsListsEveryMemberItCouldNotReturn(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, want string
		read       error
		target     string
		release    error
	}{
		{name: "unreadable ledger", want: "cannot be read: parse failed", read: errors.New("parse failed"), target: ReturnTargetLive},
		{name: "unprovable seat", want: "unknown: source identity is not provable", target: ReturnTargetUnknown},
		{name: "failed release", want: "push rejected", target: ReturnTargetDead, release: errors.New("push rejected")},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			store := returnBed(t)
			must(t, RequestReturn(store, testBatchID, "goal-a", UnitEjected, "broken proof", "landing+owner", time.Unix(1, 0)))
			failures, err := ReturnUnits(store, testBatchID, "tree", "landing+owner", time.Unix(2, 0), ReturnSeams{
				Read: func(string, string, string) (ReturnLedgerGoal, error) {
					return ReturnLedgerGoal{Claimed: true, Machine: "landing", Lineage: "owner", Batch: testBatchID}, test.read
				},
				Target: func(Unit) ReturnTarget {
					return ReturnTarget{State: test.target, Reason: map[bool]string{true: "source identity is not provable"}[test.target == ReturnTargetUnknown]}
				},
				HandBack: func(string, Claim, uint64) error { return nil },
				Release:  func(string, string) error { return test.release },
			})
			if (err != nil) != (test.release != nil) {
				t.Fatalf("err = %v", err)
			}
			if len(failures) != 1 || failures[0].GoalID != "goal-a" || !strings.Contains(failures[0].Reason, test.want) {
				t.Fatalf("failures = %+v; want goal-a listed with %q", failures, test.want)
			}
			if unit := load(t, store).Units[0]; unit.State != UnitReturnPending {
				t.Fatalf("a listed member settled as %s", unit.State)
			}
		})
	}
}

func TestBatchLedgerReadersHonorNestedModuleRoot(t *testing.T) {
	t.Parallel()
	repository := t.TempDir()
	module := filepath.Join(repository, "metasystem")
	ledgerPath := filepath.Join(module, "plans", "goals", "goal-a.md")
	ledgerBytes := goalBed("goal-a")
	must(t, os.MkdirAll(filepath.Join(module, "plans", "goals"), 0o755))
	must(t, os.WriteFile(ledgerPath, ledgerBytes, 0o644))
	tree := strings.Repeat("a", 40)
	blobHeader := []byte(fmt.Sprintf("blob %d\x00", len(ledgerBytes)))
	blob := sha1.Sum(append(blobHeader, ledgerBytes...))
	blobID := fmt.Sprintf("%x", blob)
	const goalPath = "metasystem/plans/goals/goal-a.md"
	answers := []struct {
		args   []string
		stdout []byte
	}{
		{[]string{"rev-parse", "--show-prefix"}, []byte("metasystem/\n")},
		{[]string{"--literal-pathspecs", "ls-tree", "-r", "-z", "--full-tree", tree, "--", goalPath}, []byte(fmt.Sprintf("100644 blob %s\t%s\x00", blobID, goalPath))},
		{[]string{"cat-file", "blob", blobID}, ledgerBytes},
	}
	prefix := append([]string{"-C", module}, []string{
		"-c", "core.fileMode=true", "-c", "diff.noprefix=false", "-c", "diff.mnemonicPrefix=false",
		"-c", "apply.ignoreWhitespace=no", "-c", "core.logAllRefUpdates=false", "-c", "core.useReplaceRefs=false",
		"-c", "gc.auto=0", "-c", "maintenance.auto=false",
	}...)
	calls := 0
	workspace := gittree.Workspace{Dir: module, RawSource: func(request gittree.RawRequest) gittree.RawResult {
		t.Helper()
		live, err := os.ReadFile(ledgerPath)
		if err != nil || !bytes.Equal(live, ledgerBytes) {
			t.Fatalf("nested ledger bytes changed: %q, %v", live, err)
		}
		if calls >= len(answers) {
			t.Fatalf("unexpected extra raw request: %+v", request)
		}
		want := answers[calls%len(answers)]
		if request.Dir != module || request.Stdin != nil || !slices.Equal(request.Env, gittree.ScrubbedEnviron()) ||
			!slices.Equal(request.Args, append(slices.Clone(prefix), want.args...)) || request.Operation != "git "+strings.Join(want.args, " ") {
			t.Fatalf("raw request %d = %+v, want %q", calls, request, want.args)
		}
		calls++
		return gittree.RawResult{Stdout: bytes.Clone(want.stdout)}
	}}
	ledger, err := readReturnLedgerGoalWithWorkspace(workspace, tree, "goal-a")
	if err != nil || !ledger.Claimed || ledger.Machine != "landing" || ledger.Batch != testBatchID {
		t.Fatalf("nested return ledger=%+v error=%v", ledger, err)
	}
	if calls != len(answers) {
		t.Fatalf("raw calls = %d, want %d", calls, len(answers))
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
}
