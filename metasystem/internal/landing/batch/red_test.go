package batch

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathpattern"
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

func diagnosingBed(t *testing.T) (policyBed, Store) {
	t.Helper()
	bed := policyFixture(t)
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
		bed, store := diagnosingBed(t)
		strictReassembly(t, &store, expectedAssembly(bed.base, []string{"goal-b"}, []string{"chain-b"}, []string{"prefix-b"}))
		var requests []DiagnosticRequest
		err := DiagnoseRed(store, testBatchID, "owner", failing, "", time.Unix(2, 0), RedSeams{Run: func(request DiagnosticRequest) (DiagnosticResult, error) {
			requests = append(requests, request)
			return DiagnosticResult{AttemptID: "base"}, nil
		}})
		must(t, err)
		record := load(t, store)
		if len(requests) != 1 || !requests[0].NeverReuse || requests[0].Tree != record.BaseTree || record.Units[0].State != UnitReturnPending || record.Units[1].State != UnitJoined || record.State != StateOpen || record.TipTree != "prefix-b" || !slices.Equal(record.PrefixTrees, []string{"prefix-b"}) {
			t.Fatalf("requests=%+v record=%+v", requests, record)
		}
	})
	t.Run("W12b named units run alone serially in join order", func(t *testing.T) {
		bed := policyFixture(t)
		unnamedTail := Unit{GoalID: "goal-c", Chain: "chain-c", Claim: Claim{Machine: "seat", Lineage: "l", Epoch: 1, Revision: 9, AccountingRevision: 7}, State: UnitJoined, ChangedPaths: []string{"c.go"}}
		bed.record.Units[0].ChangedPaths = []string{"a.go"}
		bed.record.Units[1].ChangedPaths = []string{"b.go"}
		bed.record.Units = []Unit{bed.record.Units[0], unnamedTail, bed.record.Units[1]}
		bed.record.State = StateDiagnosing
		bed.record.Proof = &Proof{Status: "failed", AttemptID: "tip-attempt"}
		store := NewStore(bed.root, nil)
		must(t, store.Create(bed.record))
		strictReassembly(t, &store,
			expectedAssembly(bed.base, []string{"goal-c"}, []string{"chain-c"}, []string{"prefix-c"}),
			expectedAssembly(bed.base, []string{"goal-a", "goal-c"}, []string{"chain-a", "chain-c"}, []string{"prefix-a", "prefix-ac"}),
			expectedAssembly(bed.base, []string{"goal-a", "goal-c", "goal-b"}, []string{"chain-a", "chain-c", "chain-b"}, []string{"prefix-a", "prefix-ac", "prefix-acb"}),
			expectedAssembly(bed.base, []string{"goal-a", "goal-c"}, []string{"chain-a", "chain-c"}, []string{"prefix-a", "prefix-ac"}))
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
		if !slices.Equal(gotGoals, []string{"goal-b", "goal-c", "goal-a", "goal-b"}) ||
			requests[0].Claim.Revision != 8 || requests[1].Claim.Revision != 9 || requests[1].Claim.AccountingRevision != 7 ||
			requests[2].Claim.Revision != 7 || requests[3].Claim.Revision != 8 ||
			record.Units[0].State != UnitJoined || record.Units[1].State != UnitJoined || record.Units[2].State != UnitReturnPending || record.State != StateOpen ||
			record.TipTree != "prefix-ac" || !slices.Equal(record.PrefixTrees, []string{"prefix-a", "prefix-ac"}) {
			t.Fatalf("requests=%+v record=%+v", requests, record)
		}
	})
	t.Run("W12c three named units eject C and name its landed partners", func(t *testing.T) {
		bed, store := diagnosingBed(t)
		must(t, store.Update(testBatchID, func(record *Record) error {
			record.Units = append(record.Units, Unit{GoalID: "goal-c", Chain: "chain-c", Claim: Claim{Machine: "seat", Lineage: "l", Epoch: 1, Revision: 9, AccountingRevision: 7}, State: UnitJoined, ChangedPaths: []string{"c.go"}})
			return nil
		}))
		strictReassembly(t, &store,
			expectedAssembly(bed.base, []string{"goal-a"}, []string{"chain-a"}, []string{"prefix-a"}),
			expectedAssembly(bed.base, []string{"goal-a", "goal-b"}, []string{"chain-a", "chain-b"}, []string{"prefix-a", "prefix-ab"}),
			expectedAssembly(bed.base, []string{"goal-a", "goal-b", "goal-c"}, []string{"chain-a", "chain-b", "chain-c"}, []string{"prefix-a", "prefix-ab", "prefix-abc"}),
			expectedAssembly(bed.base, []string{"goal-a", "goal-b"}, []string{"chain-a", "chain-b"}, []string{"prefix-a", "prefix-ab"}))
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
		if calls != 4 || record.Units[0].State != UnitJoined || record.Units[1].State != UnitJoined || record.Units[2].State != UnitReturnPending ||
			record.TipTree != "prefix-ab" || !slices.Equal(record.PrefixTrees, []string{"prefix-a", "prefix-ab"}) {
			t.Fatalf("calls=%d record=%+v", calls, record)
		}
	})
	for _, test := range []struct {
		name    string
		groups  []RedGroup
		baseRed bool
	}{
		{"W12d red base holds trunk red", failing, true},
		{"W12e green base without attribution holds unclassified", []RedGroup{{ID: "group", InputManifest: []string{"other/**"}}}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, store := diagnosingBed(t)
			strictReassembly(t, &store)
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
			if test.baseRed {
				if record.State != StateHeldTrunkRed || ledger.calls != 1 || ledger.last.BaseCommit != "base-commit" || slices.ContainsFunc(record.Units, func(unit Unit) bool { return unit.State != UnitJoined }) {
					t.Fatalf("record=%+v ledger calls=%d", record, ledger.calls)
				}
			} else if record.State != StateHeldUnclassified || ledger.calls != 0 || slices.ContainsFunc(record.Units, func(unit Unit) bool { return unit.State != UnitJoined }) {
				t.Fatalf("unattributed failure was assigned to trunk or member: record=%+v ledger calls=%d", record, ledger.calls)
			}
		})
	}
}

func TestBatchEjectAndReassemble(t *testing.T) {
	bed, store := diagnosingBed(t)
	want := []string{"prefix-b"}
	strictReassembly(t, &store, expectedAssembly(bed.base, []string{"goal-b"}, []string{"chain-b"}, want))
	if err := DiagnoseRed(store, testBatchID, "owner", []RedGroup{{ID: "group", InputManifest: []string{"a.go"}}}, "", time.Unix(2, 0), RedSeams{Run: func(DiagnosticRequest) (DiagnosticResult, error) {
		return DiagnosticResult{AttemptID: "base"}, nil
	}}); err != nil {
		t.Fatal(err)
	}
	record := load(t, store)
	if record.TipTree != want[0] || !slices.Equal(record.PrefixTrees, want) || record.Proof != nil || record.Seal != nil {
		t.Fatalf("reassembled=%+v want=%v", record, want)
	}
}

func TestBatchSingleOwnerRedEjectsAndSurvivorsLand(t *testing.T) {
	bed, store := diagnosingBed(t)
	strictReassembly(t, &store, expectedAssembly(bed.base, []string{"goal-b"}, []string{"chain-b"}, []string{"prefix-b"}))
	handBacks := 0
	returns := ReturnSeams{
		Read: func(string, string, string) (ReturnLedgerGoal, error) {
			return ReturnLedgerGoal{Claimed: true, Machine: "landing", Lineage: "owner", Batch: testBatchID}, nil
		},
		Target: func(Unit) ReturnTarget { return ReturnTarget{State: ReturnTargetLive, Epoch: 9} },
		HandBack: func(string, Claim, uint64) error {
			handBacks++
			return nil
		},
		Release: func(string, string) error { t.Fatal("live joiner claim was released"); return nil },
	}
	failing := []RedGroup{{ID: "group", InputManifest: []string{"a.go"}, LogPath: "logs/group.log", Failures: []Failure{{Name: "TestOwnedFailure"}}}}
	must(t, DiagnoseRed(store, testBatchID, "owner", failing, "", time.Unix(2, 0), RedSeams{Run: func(request DiagnosticRequest) (DiagnosticResult, error) {
		if request.Tree != load(t, store).BaseTree || !request.NeverReuse {
			t.Fatalf("base diagnostic request=%+v", request)
		}
		return DiagnosticResult{AttemptID: "base-green"}, nil
	}}))
	reassembled := load(t, store)
	if reassembled.Units[0].State != UnitReturnPending || reassembled.Units[1].State != UnitJoined || reassembled.State != StateOpen {
		t.Fatalf("red classification=%+v", reassembled)
	}
	for _, want := range []string{"base-green", "group", "logs/group.log", "TestOwnedFailure"} {
		if !strings.Contains(reassembled.Units[0].Failure, want) {
			t.Fatalf("ejection failure %q lacks %q", reassembled.Units[0].Failure, want)
		}
	}
	must(t, ReturnUnits(store, testBatchID, "tree", "owner", time.Unix(3, 0), returns))
	if ejected := load(t, store).Units[0]; ejected.State != UnitEjected || ejected.ReturnDisposition != ReturnHandedBack {
		t.Fatalf("ejected custody=%+v", ejected)
	}
	must(t, store.Update(testBatchID, func(record *Record) error {
		record.State = StateLanding
		record.Proof = &Proof{Status: "green", AttemptID: "survivor-green"}
		record.Receipts = map[string]PrefixReceipt{"goal-b": {GoalID: "goal-b", Tree: record.TipTree, AttemptID: "survivor-green"}}
		record.History = append(record.History, HistoryEntry{At: time.Unix(3, 0).UTC().Format(time.RFC3339Nano), Verb: "prove", From: StateProving, To: StateLanding, Actor: "owner"})
		return nil
	}))
	var events []string
	must(t, LandSeries(store, testBatchID, "owner", time.Unix(4, 0), greenLandSeams(&events)))
	wantEvents := []string{"apply:goal-b", "receipt:goal-b", "commit:goal-b", "held", "push", "cleanup"}
	if !slices.Equal(events, wantEvents) || !load(t, store).Landing.PushComplete {
		t.Fatalf("survivor landing events=%v record=%+v", events, load(t, store))
	}
	finalized := 0
	must(t, RecoverPushedSeries(store, testBatchID, "owner", time.Unix(5, 0), RecoverySeams{
		OriginCommit: func(unit Unit) (string, bool, error) {
			if unit.GoalID != "goal-b" {
				t.Fatalf("recovery inspected ejected unit %+v", unit)
			}
			return "commit-b", true, nil
		},
		Finalize: func(unit Unit, commit string) error {
			if unit.GoalID != "goal-b" || commit != "commit-b" {
				t.Fatalf("finalize unit=%+v commit=%s", unit, commit)
			}
			finalized++
			return nil
		},
		Rearm:   func(string) error { return nil },
		Cleanup: func() error { t.Fatal("landing cleanup ran twice"); return nil },
	}))
	must(t, ReturnUnits(store, testBatchID, "tree", "owner", time.Unix(6, 0), returns))
	landed := load(t, store)
	if landed.State != StateLanded || landed.Units[0].State != UnitEjected || landed.Units[1].State != UnitLanded || !landed.Units[1].P6Done || finalized != 1 || handBacks != 2 {
		t.Fatalf("final landed batch=%+v finalized=%d handbacks=%d", landed, finalized, handBacks)
	}
}

func TestBatchDiagnosticRefusalHoldsWithoutEjection(t *testing.T) {
	_, store := diagnosingBed(t)
	strictReassembly(t, &store)
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

func TestBatchDiagnosticRefusalRequiresLiveExactFenceBeforeEjection(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		confirmed bool
		readErr   error
	}{
		{name: "confirmed exact fence", confirmed: true},
		{name: "refusal text without live fence"},
		{name: "unreadable live ledger", readErr: errors.New("accepted ledger unreadable")},
	} {
		t.Run(test.name, func(t *testing.T) {
			bed, store := diagnosingBed(t)
			if test.confirmed {
				strictReassembly(t, &store, expectedAssembly(bed.base, []string{"goal-a"}, []string{"chain-a"}, []string{"prefix-a"}))
			} else {
				strictReassembly(t, &store)
			}
			called := 0
			err := DiagnoseRed(store, testBatchID, "owner", []RedGroup{{ID: "group"}}, "", time.Unix(2, 0), RedSeams{
				Run: func(DiagnosticRequest) (DiagnosticResult, error) {
					return DiagnosticResult{}, &DiagnosticRefusal{Status: "CANDIDATE_GOAL_REFUSED state=fenced"}
				},
				ConfirmFenced: func(unit Unit) (string, bool, error) {
					called++
					if unit.GoalID != "goal-b" {
						t.Fatalf("diagnostic authority = %s, want goal-b", unit.GoalID)
					}
					return "live exact stop fence", test.confirmed, test.readErr
				},
			})
			must(t, err)
			record := load(t, store)
			if called != 1 {
				t.Fatalf("live fence checks = %d, want one", called)
			}
			if test.confirmed {
				if record.State != StateOpen || record.Units[1].State != UnitReturnPending || record.Units[1].Outcome != UnitEjected ||
					record.Units[1].Failure != "live exact stop fence" || record.Units[0].State != UnitJoined || record.Proof != nil || record.Seal != nil ||
					record.TipTree != "prefix-a" || !slices.Equal(record.PrefixTrees, []string{"prefix-a"}) {
					t.Fatalf("confirmed fence did not reassemble only the survivor: %+v", record)
				}
			} else if record.State != StateHeldUnclassified || record.Proof == nil ||
				slices.ContainsFunc(record.Units, func(unit Unit) bool { return unit.State != UnitJoined }) {
				t.Fatalf("unconfirmed fence was treated as an ejection: %+v", record)
			}
		})
	}
}

func TestBatchReopenNeedsNewTree(t *testing.T) {
	bed, store := diagnosingBed(t)
	strictReassembly(t, &store, expectedAssembly(bed.moved, []string{"goal-a", "goal-b"}, []string{"chain-a", "chain-b"}, []string{"moved-a", "moved-ab"}))
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
	if record := load(t, store); record.State != StateOpen || record.BaseTree != bed.moved || record.TrunkRed != nil ||
		record.TipTree != "moved-ab" || !slices.Equal(record.PrefixTrees, []string{"moved-a", "moved-ab"}) {
		t.Fatalf("reopened=%+v", record)
	}
}

func TestTrunkRedHookAndNarrowHold(t *testing.T) {
	_, store := diagnosingBed(t)
	strictReassembly(t, &store)
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

func TestGLEPathDiagnosticManifestLiteralAttribution(t *testing.T) {
	t.Parallel()
	entry := pathpattern.EncodeLiteral("metasystem/pkg/[literal].go")
	if !diagnosticPathMatches(entry, "metasystem/pkg/[literal].go") {
		t.Fatal("discovered literal did not name its changed unit")
	}
	if diagnosticPathMatches(entry, "metasystem/pkg/aliteral.go") {
		t.Fatal("discovered literal matched another filename")
	}
}

func TestBatchDiagnosticAssemblyFailureHolds(t *testing.T) {
	bed, store := diagnosingBed(t)
	must(t, store.Update(testBatchID, func(record *Record) error {
		record.Units = append(record.Units, Unit{GoalID: "goal-c", Chain: "chain-c",
			Claim: Claim{Machine: "seat", Lineage: "l", Epoch: 1, Revision: 9, AccountingRevision: 7},
			State: UnitJoined, ChangedPaths: []string{"c.go"}})
		return nil
	}))
	wantErr := errors.New("unnamed patch does not compose")
	strictReassembly(t, &store, expectedReassembly{kind: "assemble", base: bed.base,
		goals: []string{"goal-c"}, chains: []string{"chain-c"}, err: wantErr})
	groups := []RedGroup{{ID: "group", InputManifest: []string{"a.go", "b.go"}}}
	requests := 0
	must(t, DiagnoseRed(store, testBatchID, "owner", groups, "", time.Unix(2, 0), RedSeams{
		Run: func(request DiagnosticRequest) (DiagnosticResult, error) {
			requests++
			if request.Tree != bed.base || !request.NeverReuse {
				t.Fatalf("base diagnostic request=%+v", request)
			}
			return DiagnosticResult{AttemptID: "base-green"}, nil
		},
	}))
	record := load(t, store)
	if requests != 1 || record.State != StateHeldUnclassified || record.Proof == nil ||
		!strings.Contains(record.Proof.Failure, wantErr.Error()) ||
		slices.ContainsFunc(record.Units, func(unit Unit) bool { return unit.State != UnitJoined }) {
		t.Fatalf("requests=%d record=%+v", requests, record)
	}
}

func TestBatchReopenAssemblyFailurePreservesHeldRecord(t *testing.T) {
	bed, store := diagnosingBed(t)
	groups := []RedGroup{{ID: "group", InputManifest: []string{"a.go"}}}
	ledger := &redLedger{}
	must(t, DiagnoseRed(store, testBatchID, "owner", groups, "", time.Unix(2, 0), RedSeams{
		Run: func(DiagnosticRequest) (DiagnosticResult, error) {
			return DiagnosticResult{AttemptID: "base-red", Groups: groups}, nil
		},
		MintOpid: func() (string, error) { return "op-1", nil }, Ledger: ledger,
	}))
	wantErr := errors.New("moved base unavailable")
	strictReassembly(t, &store, expectedReassembly{kind: "assemble", base: bed.moved,
		goals: []string{"goal-a", "goal-b"}, chains: []string{"chain-a", "chain-b"}, err: wantErr})
	path := filepath.Join(store.root, "artifacts", "agents", "landing-batches", testBatchID+".json")
	before, err := os.ReadFile(path)
	must(t, err)
	err = ReopenHeld(store, testBatchID, bed.moved, "owner", time.Unix(3, 0))
	after, readErr := os.ReadFile(path)
	must(t, readErr)
	if !errors.Is(err, wantErr) || !slices.Equal(before, after) || load(t, store).State != StateHeldTrunkRed {
		t.Fatalf("reopen error=%v record changed=%t", err, !slices.Equal(before, after))
	}
}

func TestBatchDiagnosticNamedConflictEjectsInOrder(t *testing.T) {
	bed, store := diagnosingBed(t)
	must(t, store.Update(testBatchID, func(record *Record) error {
		record.Units = append(record.Units, Unit{GoalID: "goal-c", Chain: "chain-c",
			Claim: Claim{Machine: "seat", Lineage: "l", Epoch: 1, Revision: 9, AccountingRevision: 7},
			State: UnitJoined, ChangedPaths: []string{"c.go"}})
		return nil
	}))
	conflict := &assemblyConflict{GoalID: "goal-b", Cause: errors.New("goal-b patch conflict")}
	strictReassembly(t, &store,
		expectedAssembly(bed.base, []string{"goal-a"}, []string{"chain-a"}, []string{"prefix-a"}),
		expectedReassembly{kind: "assemble", base: bed.base, goals: []string{"goal-b"}, chains: []string{"chain-b"}, err: conflict},
		expectedAssembly(bed.base, []string{"goal-c"}, []string{"chain-c"}, []string{"prefix-c"}),
		expectedAssembly(bed.base, []string{"goal-c"}, []string{"chain-c"}, []string{"prefix-c"}))
	groups := []RedGroup{{ID: "group", InputManifest: []string{"*.go"}, LogPath: "logs/group.log", Failures: []Failure{{Name: "TestA"}}}}
	var requested []string
	must(t, DiagnoseRed(store, testBatchID, "owner", groups, "", time.Unix(2, 0), RedSeams{
		Run: func(request DiagnosticRequest) (DiagnosticResult, error) {
			requested = append(requested, request.GoalID)
			if len(requested) == 2 {
				return DiagnosticResult{AttemptID: "a-red", Groups: groups}, nil
			}
			return DiagnosticResult{AttemptID: "green"}, nil
		},
	}))
	record := load(t, store)
	if !slices.Equal(requested, []string{"goal-c", "goal-a", "goal-c"}) || record.State != StateOpen ||
		record.Units[0].State != UnitReturnPending || record.Units[1].State != UnitReturnPending || record.Units[2].State != UnitJoined ||
		!strings.Contains(record.Units[0].Failure, "a-red") || !strings.Contains(record.Units[0].Failure, "TestA") ||
		!strings.Contains(record.Units[1].Failure, "cannot apply after returning goal-a") ||
		!strings.Contains(record.Units[1].Failure, conflict.Error()) ||
		record.TipTree != "prefix-c" || !slices.Equal(record.PrefixTrees, []string{"prefix-c"}) {
		t.Fatalf("requests=%v record=%+v", requested, record)
	}
	var returns []string
	for _, entry := range record.History {
		if entry.Verb == "return-request" {
			returns = append(returns, strings.Fields(entry.Detail)[0])
		}
	}
	if !slices.Equal(returns, []string{"goal-a", "goal-b"}) {
		t.Fatalf("return order=%v", returns)
	}
}
