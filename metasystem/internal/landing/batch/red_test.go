package batch

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

type redLedger struct {
	calls int
	last  TrunkRed
}

func (owner *redLedger) Record(_ string, red TrunkRed) ([]EntryRef, error) {
	owner.calls++
	owner.last = red
	refs := make([]EntryRef, 0, len(red.Groups))
	for _, group := range red.Groups {
		refs = append(refs, EntryRef{ID: TrunkRedID(group), Group: group.ID})
	}
	return refs, nil
}
func (*redLedger) Clear(string, EntryRef, Green) error { return nil }
func (*redLedger) Open() ([]OpenEntry, error)          { return nil, nil }

func diagnosingBed(t *testing.T) (assemblyBed, Store) {
	t.Helper()
	bed := assemblyFixture(t)
	bed.record.State = StateDiagnosing
	bed.record.Proof = &Proof{Status: "failed", AttemptID: "tip-attempt"}
	bed.record.Units[0].ChangedPaths = []string{"a.go"}
	bed.record.Units[1].ChangedPaths = []string{"b.go"}
	store := NewStore(bed.root, nil)
	must(t, store.Create(bed.record))
	return bed, store
}

func TestBatchOwnerSearch(t *testing.T) {
	failing := []RedGroup{{ID: "group", InputManifest: []string{"a.go"}}}
	t.Run("W12a one named unit ejects after a fresh green base", func(t *testing.T) {
		_, store := diagnosingBed(t)
		var requests []DiagnosticRequest
		err := DiagnoseRed(store, testBatchID, "owner", failing, "", time.Unix(2, 0), RedSeams{Run: func(request DiagnosticRequest) (DiagnosticResult, error) {
			requests = append(requests, request)
			return DiagnosticResult{AttemptID: "base"}, nil
		}})
		must(t, err)
		record := load(t, store)
		if len(requests) != 1 || !requests[0].NeverReuse || requests[0].Tree != record.BaseTree || record.Units[0].State != UnitReturnPending || record.Units[1].State != UnitJoined || record.State != StateOpen {
			t.Fatalf("requests=%+v record=%+v", requests, record)
		}
	})
	t.Run("W12b named units run alone serially in join order", func(t *testing.T) {
		_, store := diagnosingBed(t)
		must(t, store.Update(testBatchID, func(record *Record) error {
			record.Units = append(record.Units, Unit{GoalID: "goal-c", Chain: "chain-c", Claim: Claim{Machine: "seat", Lineage: "l", Epoch: 1, Revision: 9, AccountingRevision: 7}, State: UnitJoined, ChangedPaths: []string{"c.go"}})
			return nil
		}))
		groups := []RedGroup{{ID: "group", InputManifest: []string{"a.go", "b.go"}}}
		var requests []DiagnosticRequest
		calls := 0
		err := DiagnoseRed(store, testBatchID, "owner", groups, "", time.Unix(2, 0), RedSeams{Run: func(request DiagnosticRequest) (DiagnosticResult, error) {
			requests = append(requests, request)
			calls++
			if calls == 4 {
				return DiagnosticResult{AttemptID: "b", Groups: groups}, nil
			}
			return DiagnosticResult{AttemptID: "green"}, nil
		}})
		must(t, err)
		record := load(t, store)
		gotGoals := []string{}
		for _, request := range requests {
			gotGoals = append(gotGoals, request.GoalID)
		}
		if !slices.Equal(gotGoals, []string{"goal-c", "goal-c", "goal-a", "goal-b"}) ||
			requests[0].Claim.Revision != 9 || requests[2].Claim.Revision != 7 || requests[3].Claim.Revision != 8 ||
			record.Units[0].State != UnitJoined || record.Units[1].State != UnitReturnPending || record.Units[2].State != UnitJoined || record.State != StateOpen {
			t.Fatalf("requests=%+v record=%+v", requests, record)
		}
	})
	t.Run("W12c three named units eject C and name its landed partners", func(t *testing.T) {
		_, store := diagnosingBed(t)
		must(t, store.Update(testBatchID, func(record *Record) error {
			record.Units = append(record.Units, Unit{GoalID: "goal-c", Chain: "chain-c", Claim: Claim{Machine: "seat", Lineage: "l", Epoch: 1, Revision: 9, AccountingRevision: 7}, State: UnitJoined, ChangedPaths: []string{"c.go"}})
			return nil
		}))
		groups := []RedGroup{{ID: "group", InputManifest: []string{"*.go"}, LogPath: "logs/group.log", Failures: []Failure{{Name: "TestC"}}}}
		calls := 0
		err := DiagnoseRed(store, testBatchID, "owner", groups, "", time.Unix(2, 0), RedSeams{Run: func(DiagnosticRequest) (DiagnosticResult, error) {
			calls++
			if calls == 4 {
				return DiagnosticResult{AttemptID: "attempt-c", Groups: groups}, nil
			}
			return DiagnosticResult{AttemptID: "base"}, nil
		}})
		must(t, err)
		record := load(t, store)
		failure := record.Units[2].Failure
		for _, want := range []string{"attempt-c", "group", "logs/group.log", "TestC", "goal-a", "goal-b"} {
			if !strings.Contains(failure, want) {
				t.Fatalf("failure %q does not contain %q; record=%+v", failure, want, record)
			}
		}
		if calls != 4 || record.Units[0].State != UnitJoined || record.Units[1].State != UnitJoined || record.Units[2].State != UnitReturnPending {
			t.Fatalf("calls=%d record=%+v", calls, record)
		}
	})
	for _, test := range []struct {
		name    string
		groups  []RedGroup
		baseRed bool
	}{
		{"W12d red base holds trunk red", failing, true},
		{"W12e no named unit holds trunk red", []RedGroup{{ID: "group", InputManifest: []string{"other/**"}}}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, store := diagnosingBed(t)
			ledger := &redLedger{}
			err := DiagnoseRed(store, testBatchID, "owner", test.groups, "", time.Unix(2, 0), RedSeams{
				Run: func(request DiagnosticRequest) (DiagnosticResult, error) {
					if test.baseRed {
						return DiagnosticResult{AttemptID: "base", Groups: test.groups}, nil
					}
					return DiagnosticResult{AttemptID: "base"}, nil
				}, MintOpid: func() (string, error) { return "op-1", nil }, Ledger: ledger, BaseCommit: "base-commit",
			})
			must(t, err)
			record := load(t, store)
			if record.State != StateHeldTrunkRed || ledger.calls != 1 || ledger.last.BaseCommit != "base-commit" || slices.ContainsFunc(record.Units, func(unit Unit) bool { return unit.State != UnitJoined }) {
				t.Fatalf("record=%+v ledger calls=%d", record, ledger.calls)
			}
		})
	}
}

func TestBatchEjectAndReassemble(t *testing.T) {
	bed, store := diagnosingBed(t)
	if err := DiagnoseRed(store, testBatchID, "owner", []RedGroup{{ID: "group", InputManifest: []string{"a.go"}}}, "", time.Unix(2, 0), RedSeams{Run: func(DiagnosticRequest) (DiagnosticResult, error) {
		return DiagnosticResult{AttemptID: "base"}, nil
	}}); err != nil {
		t.Fatal(err)
	}
	record := load(t, store)
	want, err := assembleUnits(bed.root, bed.base, []Unit{bed.record.Units[1]})
	must(t, err)
	if record.TipTree != want[0] || !slices.Equal(record.PrefixTrees, want) || record.Proof != nil || record.Seal != nil {
		t.Fatalf("reassembled=%+v want=%v", record, want)
	}
}

func TestBatchDiagnosticRefusalHoldsWithoutEjection(t *testing.T) {
	_, store := diagnosingBed(t)
	var next string
	err := DiagnoseRed(store, testBatchID, "owner", []RedGroup{{ID: "group"}}, "", time.Unix(2, 0), RedSeams{
		Run: func(DiagnosticRequest) (DiagnosticResult, error) {
			return DiagnosticResult{}, &DiagnosticRefusal{Status: "BATCH_MEMBER_BUDGET_REFUSED"}
		},
		UpdateNext: func(_ string, status string) error { next = status; return nil },
	})
	must(t, err)
	record := load(t, store)
	if record.State != StateHeldUnclassified || next == "" || slices.ContainsFunc(record.Units, func(unit Unit) bool { return unit.State != UnitJoined }) {
		t.Fatalf("record=%+v next=%q", record, next)
	}
}

func TestBatchReopenNeedsNewTree(t *testing.T) {
	bed, store := diagnosingBed(t)
	ledger := &redLedger{}
	groups := []RedGroup{{ID: "group", InputManifest: []string{"a.go"}}}
	must(t, DiagnoseRed(store, testBatchID, "owner", groups, "", time.Unix(2, 0), RedSeams{Run: func(DiagnosticRequest) (DiagnosticResult, error) {
		return DiagnosticResult{AttemptID: "base", Groups: groups}, nil
	}, MintOpid: func() (string, error) { return "op-1", nil }, Ledger: ledger}))
	if err := ReopenHeld(store, testBatchID, bed.base, "owner", time.Unix(3, 0)); err == nil || !strings.Contains(err.Error(), "BATCH_REOPEN_SAME_TREE") {
		t.Fatalf("same-tree reopen=%v", err)
	}
	if err := ReopenHeld(store, testBatchID, bed.moved, "owner", time.Unix(3, 0)); err != nil {
		t.Fatal(err)
	}
	if record := load(t, store); record.State != StateOpen || record.BaseTree != bed.moved || record.TrunkRed != nil {
		t.Fatalf("reopened=%+v", record)
	}
}

func TestTrunkRedHookAndNarrowHold(t *testing.T) {
	_, store := diagnosingBed(t)
	groups := []RedGroup{{ID: "group", InputManifest: []string{"a.go"}}}
	err := DiagnoseRed(store, testBatchID, "owner", groups, "", time.Unix(2, 0), RedSeams{Run: func(DiagnosticRequest) (DiagnosticResult, error) {
		return DiagnosticResult{Groups: groups}, nil
	}})
	if !errors.Is(err, errLedgerOwnerUnbound) || load(t, store).State != StateDiagnosing {
		t.Fatalf("unbound hook error=%v state=%s", err, load(t, store).State)
	}
}

func TestDiagnosticInputMatchingUsesManifestPathsNotSuffixes(t *testing.T) {
	if diagnosticPathMatches("pkg/a.go", "other/a.go") {
		t.Fatal("a shared filename suffix named an unrelated unit")
	}
	if !diagnosticPathMatches("pkg/**", "pkg/sub/a.go") {
		t.Fatal("manifest subtree did not name its changed unit")
	}
}
