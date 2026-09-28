package batch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathpattern"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter/fakeadapter"
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
	t.Run("W12d red base holds trunk red", func(t *testing.T) {
		_, store := diagnosingBed(t)
		strictReassembly(t, &store)
		ledger := &redLedger{}
		err := DiagnoseRed(store, testBatchID, "owner", failing, "", time.Unix(2, 0), RedSeams{
			Run: func(DiagnosticRequest) (DiagnosticResult, error) {
				return DiagnosticResult{AttemptID: "base", Groups: failing}, nil
			}, MintOpid: func() (string, error) { return "op-1", nil }, Ledger: ledger, BaseCommit: "base-commit",
		})
		must(t, err)
		record := load(t, store)
		if record.State != StateHeldTrunkRed || ledger.calls != 1 || ledger.last.BaseCommit != "base-commit" || slices.ContainsFunc(record.Units, func(unit Unit) bool { return unit.State != UnitJoined }) {
			t.Fatalf("record=%+v ledger calls=%d", record, ledger.calls)
		}
	})
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
	if record.State != StateDiagnosing || record.Proof == nil || record.Proof.Status != "failed" || next == "" ||
		slices.ContainsFunc(record.Units, func(unit Unit) bool { return unit.State != UnitJoined }) {
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
			} else if record.State != StateDiagnosing || record.Proof == nil ||
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
	t.Parallel()
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
	t.Parallel()
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

// fakeRedLanguage binds red groups to the fake, non-Go adapter: fake-affected/*
// groups are package-selection expansions whose manifest is the whole module,
// section/* groups are section beds, anything else is a targeted fake group.
func fakeRedLanguage(fake *fakeadapter.Adapter) func(RedGroup) (adapter.Adapter, bool) {
	return func(group RedGroup) (adapter.Adapter, bool) {
		if strings.HasPrefix(group.ID, "section/") {
			section, _ := adapter.Resolve(testpolicy.Group{Adapter: "section"})
			return section, false
		}
		return fake, strings.HasPrefix(group.ID, "fake-affected/")
	}
}

func fakeClosure(changed, dependents []string) *adapter.Closure {
	return &adapter.Closure{Module: "com.example", Changed: changed, Dependents: dependents}
}

// threeMemberRedBed is A (closure ledger), B (closure payments, dependent
// ledger) and C (closure reports), each with its own changed paths.
func threeMemberRedBed(t *testing.T) (policyBed, Store) {
	t.Helper()
	bed := policyFixture(t)
	bed.record.State = StateDiagnosing
	bed.record.Proof = &Proof{Status: "failed", AttemptID: "tip-attempt"}
	bed.record.Units = append(bed.record.Units, Unit{GoalID: "goal-c", Chain: "chain-c",
		Claim: Claim{Machine: "seat", Lineage: "l", Epoch: 1, Revision: 9, AccountingRevision: 7}, State: UnitJoined})
	bed.record.Units[0].ChangedPaths, bed.record.Units[0].Closure = []string{"ledger/Ledger.java"}, fakeClosure([]string{"ledger"}, nil)
	bed.record.Units[1].ChangedPaths, bed.record.Units[1].Closure = []string{"payments/Pay.java"}, fakeClosure([]string{"payments"}, []string{"ledger"})
	bed.record.Units[2].ChangedPaths, bed.record.Units[2].Closure = []string{"fixtures/land.sh"}, fakeClosure([]string{"reports"}, nil)
	store := NewStore(bed.root, nil)
	must(t, store.Create(bed.record))
	return bed, store
}

var wholeModuleManifest = []string{"*", "*/**"}

func paymentRed() RedGroup {
	return RedGroup{ID: "fake-affected/payments", Status: "failed", InputManifest: wholeModuleManifest, LogPath: "logs/payments.log",
		Failures: []Failure{{Report: "junit-xml", Classname: "com.example.PaymentTest", Name: "rejectsInvalidInput", Status: "failed"}}}
}

func neverHeldUnclassified(t *testing.T, record Record) {
	t.Helper()
	for _, entry := range record.History {
		if entry.To == StateHeldUnclassified {
			t.Fatalf("a red with a green base reached held-unclassified: %+v", entry)
		}
	}
}

func TestRedWithGreenBaseNeverHolds(t *testing.T) {
	t.Parallel()
	green := func(DiagnosticRequest) (DiagnosticResult, error) {
		return DiagnosticResult{AttemptID: "base-green"}, nil
	}
	t.Run("one named member is ejected and the survivor goes on", func(t *testing.T) {
		t.Parallel()
		bed, store := diagnosingBed(t)
		must(t, store.Update(testBatchID, func(record *Record) error {
			record.Units[0].Closure, record.Units[1].Closure = fakeClosure([]string{"payments"}, nil), fakeClosure([]string{"reports"}, nil)
			return nil
		}))
		strictReassembly(t, &store, expectedAssembly(bed.base, []string{"goal-b"}, []string{"chain-b"}, []string{"prefix-b"}))
		must(t, DiagnoseRed(store, testBatchID, "owner", []RedGroup{paymentRed()}, "", time.Unix(2, 0),
			RedSeams{Run: green, Adapter: fakeRedLanguage(fakeadapter.New())}))
		record := load(t, store)
		if record.State != StateOpen || record.Units[0].State != UnitReturnPending || record.Units[0].Outcome != UnitEjected ||
			record.Units[1].State != UnitJoined || record.TipTree != "prefix-b" {
			t.Fatalf("named member was not ejected with the survivor kept: %+v", record)
		}
		neverHeldUnclassified(t, record)
	})
	t.Run("nobody named returns every member and the batch dissolves", func(t *testing.T) {
		t.Parallel()
		_, store := diagnosingBed(t)
		strictReassembly(t, &store)
		unowned := paymentRed()
		unowned.Failures[0].Classname = "com.example.UnknownTest"
		must(t, DiagnoseRed(store, testBatchID, "owner", []RedGroup{unowned}, "", time.Unix(2, 0),
			RedSeams{Run: green, Adapter: fakeRedLanguage(fakeadapter.New()), Ledger: &flakeLedger{}, MintOpid: func() (string, error) { return "op-1", nil }}))
		record := load(t, store)
		if record.State != StateDissolved || record.Proof != nil {
			t.Fatalf("an unnamed red did not dissolve the batch: %+v", record)
		}
		for _, unit := range record.Units {
			if unit.State != UnitReturnPending || unit.Outcome != UnitEjected || !strings.Contains(unit.Failure, "logs/payments.log") ||
				!strings.Contains(unit.Failure, "rejectsInvalidInput") {
				t.Fatalf("member %s was not returned with the log: %+v", unit.GoalID, unit)
			}
		}
		neverHeldUnclassified(t, record)
	})
}

func TestNamingByOwnerUnitInClosure(t *testing.T) {
	t.Parallel()
	fake := fakeadapter.New()
	for classname, unit := range fake.Owners {
		if classname == unit {
			t.Fatalf("the fake's classname %s equals its unit name", classname)
		}
	}
	for _, test := range []struct {
		name      string
		red       RedGroup
		survivors []string
		named     string
	}{
		{"owner unit in the closure, not the whole-module manifest", paymentRed(), []string{"goal-a", "goal-c"}, "goal-b"},
		{"a section red names by its manifest", RedGroup{ID: "section/land-fixtures", Status: "failed",
			InputManifest: []string{"fixtures/**"}, LogPath: "logs/land.log"}, []string{"goal-a", "goal-b"}, "goal-c"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			bed, store := threeMemberRedBed(t)
			chains := []string{}
			for _, goalID := range test.survivors {
				chains = append(chains, "chain-"+strings.TrimPrefix(goalID, "goal-"))
			}
			strictReassembly(t, &store, expectedAssembly(bed.base, test.survivors, chains, []string{"prefix-1", "prefix-2"}))
			must(t, DiagnoseRed(store, testBatchID, "owner", []RedGroup{test.red}, "", time.Unix(2, 0), RedSeams{
				Run: func(DiagnosticRequest) (DiagnosticResult, error) {
					return DiagnosticResult{AttemptID: "base-green"}, nil
				},
				Adapter: fakeRedLanguage(fakeadapter.New()),
			}))
			record := load(t, store)
			for _, unit := range record.Units {
				want := UnitJoined
				if unit.GoalID == test.named {
					want = UnitReturnPending
				}
				if unit.State != want {
					t.Fatalf("member %s state=%s, want %s; record=%+v", unit.GoalID, unit.State, want, record)
				}
			}
		})
	}
}

func TestEjectAllNamedAtOnce(t *testing.T) {
	t.Parallel()
	bed, store := threeMemberRedBed(t)
	strictReassembly(t, &store, expectedAssembly(bed.base, []string{"goal-c"}, []string{"chain-c"}, []string{"prefix-c"}))
	red := paymentRed()
	red.Failures = append(red.Failures, Failure{Report: "junit-xml", Classname: "com.example.LedgerTest", Name: "balances", Status: "failed"})
	runs := 0
	must(t, DiagnoseRed(store, testBatchID, "owner", []RedGroup{red}, "", time.Unix(2, 0), RedSeams{
		Run: func(request DiagnosticRequest) (DiagnosticResult, error) {
			runs++
			if request.Tree != bed.base {
				t.Fatalf("a diagnostic ran on %s, not the base", request.Tree)
			}
			return DiagnosticResult{AttemptID: "base-green"}, nil
		},
		Adapter: fakeRedLanguage(fakeadapter.New()),
	}))
	record := load(t, store)
	if runs != 1 || record.State != StateOpen || record.Units[0].State != UnitReturnPending || record.Units[1].State != UnitReturnPending ||
		record.Units[2].State != UnitJoined || record.TipTree != "prefix-c" {
		t.Fatalf("runs=%d record=%+v", runs, record)
	}
	reassemblies := 0
	for _, entry := range record.History {
		if entry.Verb == "reassemble" {
			reassemblies++
		}
	}
	if reassemblies != 1 {
		t.Fatalf("named members were not ejected in one reassembly: %+v", record.History)
	}
}

func TestBaseRedTwiceStillHoldsTrunkRed(t *testing.T) {
	t.Parallel()
	for _, secondRed := range []bool{true, false} {
		t.Run(fmt.Sprintf("second base run red=%t", secondRed), func(t *testing.T) {
			t.Parallel()
			_, store := diagnosingBed(t)
			strictReassembly(t, &store)
			fake, red, ledger := fakeadapter.New(), paymentRed(), &flakeLedger{}
			var requests []DiagnosticRequest
			must(t, DiagnoseRed(store, testBatchID, "owner", []RedGroup{red, {ID: "section/land-fixtures", Status: "failed"}}, "", time.Unix(2, 0), RedSeams{
				Run: func(request DiagnosticRequest) (DiagnosticResult, error) {
					requests = append(requests, request)
					if len(requests) == 2 && !secondRed {
						return DiagnosticResult{AttemptID: "base-2"}, nil
					}
					return DiagnosticResult{AttemptID: fmt.Sprintf("base-%d", len(requests)), Groups: []RedGroup{red}}, nil
				},
				Adapter: fakeRedLanguage(fake), MintOpid: func() (string, error) { return "op-1", nil }, Ledger: ledger, BaseCommit: "base-commit",
			}))
			record := load(t, store)
			if len(requests) != 2 || requests[1].Tree != record.BaseTree || !requests[1].NeverReuse ||
				!slices.Equal(requests[1].Fresh, fake.Fresh) || !slices.Equal(requests[1].Groups, []string{red.ID}) || len(requests[0].Fresh) != 0 {
				t.Fatalf("the second base run was not one executed run of the red groups: %+v", requests)
			}
			if secondRed {
				if record.State != StateHeldTrunkRed || ledger.calls != 1 || ledger.last.AttemptID != "base-2" ||
					slices.ContainsFunc(record.Units, func(unit Unit) bool { return unit.State != UnitJoined }) {
					t.Fatalf("a base red twice did not hold as a trunk red: record=%+v ledger=%d", record, ledger.calls)
				}
				return
			}
			if record.State != StateDissolved || ledger.calls != 0 ||
				slices.ContainsFunc(record.Units, func(unit Unit) bool { return unit.State != UnitReturnPending }) {
				t.Fatalf("a base red then green held or landed: record=%+v ledger=%d", record, ledger.calls)
			}
			neverHeldUnclassified(t, record)
		})
	}
}

func TestUnavailableDiagnosticStaysDiagnosing(t *testing.T) {
	t.Parallel()
	for _, unavailable := range []error{&DiagnosticRefusal{Status: "PROOF_ADMISSION_REFUSED host cap"}, errors.New("runner exited without a result")} {
		t.Run(unavailable.Error(), func(t *testing.T) {
			t.Parallel()
			bed, store := diagnosingBed(t)
			strictReassembly(t, &store, expectedAssembly(bed.base, []string{"goal-b"}, []string{"chain-b"}, []string{"prefix-b"}))
			failing := []RedGroup{{ID: "group", InputManifest: []string{"a.go"}}}
			var next string
			seams := RedSeams{
				Run:        func(DiagnosticRequest) (DiagnosticResult, error) { return DiagnosticResult{}, unavailable },
				UpdateNext: func(_ string, status string) error { next = status; return nil },
			}
			err := DiagnoseRed(store, testBatchID, "owner", failing, "", time.Unix(2, 0), seams)
			var refusal *DiagnosticRefusal
			if errors.As(unavailable, &refusal) != (err == nil) {
				t.Fatalf("diagnosis error=%v", err)
			}
			record := load(t, store)
			if record.State != StateDiagnosing || record.Proof == nil || record.Proof.Status != "failed" || !strings.Contains(next, "next tick") ||
				slices.ContainsFunc(record.Units, func(unit Unit) bool { return unit.State != UnitJoined }) {
				t.Fatalf("an unavailable diagnostic did not leave the batch diagnosing: %+v next=%q", record, next)
			}
			seams.Run = func(DiagnosticRequest) (DiagnosticResult, error) {
				return DiagnosticResult{AttemptID: "base-green"}, nil
			}
			must(t, DiagnoseRed(store, testBatchID, "owner", failing, "", time.Unix(3, 0), seams))
			if record := load(t, store); record.State != StateOpen || record.Units[0].State != UnitReturnPending {
				t.Fatalf("the next tick did not decide the red: %+v", record)
			}
		})
	}
}

// detectingFake is the fake adapter as the one language adapter that
// recognises a single test's checkout.
type detectingFake struct {
	*fakeadapter.Adapter
	root string
}

func (fake detectingFake) Detects(root string) bool { return root == fake.root }

func TestLaneDecisionsWithAFakeAdapter(t *testing.T) {
	t.Parallel()
	bed := newOrdinaryJoinBed(t)
	fake := fakeadapter.New()
	adapter.Register("fake-"+filepath.Base(bed.root), detectingFake{Adapter: fake, root: bed.root})
	unitTree := bed.expectJoin(joiningUnit("goal-a", "chain-a"), testpolicy.Plan{RequiredMode: testpolicy.ModeStandard})
	must(t, PublishJoin(bed.store, testBatchID, joiningUnit("goal-a", "chain-a"), "seat+goal-a", time.Unix(1, 0),
		joinPlanMode(testpolicy.ModeStandard), func() error { return nil }))
	unit := load(t, bed.store).Units[0]
	if unit.State != UnitJoined || unit.Closure == nil || unit.Closure.Tree != unitTree ||
		!slices.Equal(unit.Closure.Changed, []string{"payments"}) || !slices.Equal(unit.Closure.Dependents, []string{"ledger"}) {
		t.Fatalf("join did not record the adapter's closure: %+v", unit)
	}
	if !slices.Equal(*fake.Calls, []string{"Closure"}) {
		t.Fatalf("fake adapter calls at join=%v", *fake.Calls)
	}
}

// flakeLedger is the register's intake in memory: open entries by class and
// every pending, hang and promotion the lane records. A promotion opens a
// known-flake entry per identity, allowed for three days.
type flakeLedger struct {
	redLedger
	entries    []OpenEntry
	pendings   []FlakeSighting
	hangs      []HangSighting
	promotions []Promotion
}

func (l *flakeLedger) Open() ([]OpenEntry, error) { return slices.Clone(l.entries), nil }
func (l *flakeLedger) OpenByClass(classes ...string) ([]OpenEntry, error) {
	return slices.DeleteFunc(slices.Clone(l.entries), func(entry OpenEntry) bool { return !slices.Contains(classes, entry.Class) }), nil
}
func (l *flakeLedger) RecordPending(_ string, sighting FlakeSighting) ([]EntryRef, error) {
	l.pendings = append(l.pendings, sighting)
	return []EntryRef{{ID: fmt.Sprintf("pending-%d", len(l.pendings))}}, nil
}
func (l *flakeLedger) RecordHang(_ string, hang HangSighting) ([]EntryRef, error) {
	l.hangs = append(l.hangs, hang)
	return []EntryRef{{ID: "hang-1"}}, nil
}
func (l *flakeLedger) Promote(_ string, promotion Promotion) ([]EntryRef, error) {
	l.promotions = append(l.promotions, promotion)
	for _, group := range promotion.Groups {
		l.known(FlakeID(group.ID, group.Failures[0]), promotion.SeenAt.Add(72*time.Hour))
	}
	return nil, nil
}
func (l *flakeLedger) known(identity string, until time.Time) {
	l.entries = append(l.entries, OpenEntry{ID: "known-" + identity, Identity: identity, Class: ClassKnownFlake, AllowanceUntil: until, Owner: "Wido"})
}

var flakeNow = time.Date(2026, 10, 1, 13, 0, 0, 0, time.UTC)

// knownRed is a complete, identified red of the fake adapter's PaymentTest.
func knownRed() RedGroup {
	red := paymentRed()
	red.CollectionComplete, red.Adapter = true, "fake"
	return red
}

// flakeScript answers base runs in order and the tip run with its classification.
type flakeScript struct {
	base     []DiagnosticResult
	tip      DiagnosticResult
	requests []DiagnosticRequest
}

func (script *flakeScript) run(request DiagnosticRequest) (DiagnosticResult, error) {
	script.requests = append(script.requests, request)
	if request.Tree == "tip-tree" {
		return script.tip, nil
	}
	if len(script.base) == 0 {
		return DiagnosticResult{AttemptID: "base-green"}, nil
	}
	next := script.base[0]
	script.base = script.base[1:]
	return next, nil
}

// passedAt is a classification run whose groups executed and passed at identity.
func passedAt(identity string, groups ...RedGroup) DiagnosticResult {
	run := DiagnosticResult{AttemptID: "class-attempt"}
	for _, group := range groups {
		run.Evidence = append(run.Evidence, GroupEvidence{ID: group.ID, Status: "passed", ExecutionIdentity: identity,
			LogPath: "logs/class.log", NativeLaunched: true, CollectionComplete: true})
	}
	return run
}

// flakeBed is a diagnosing two-member batch whose tip proof is red on groups
// and green on fake-other; nobody is named (no member closure).
func flakeBed(t *testing.T, groups ...RedGroup) (Store, *flakeLedger) {
	t.Helper()
	return flakeBedWith(t, func(*Record) {}, groups...)
}

func flakeBedWith(t *testing.T, prepare func(*Record), groups ...RedGroup) (Store, *flakeLedger) {
	t.Helper()
	bed := policyFixture(t)
	bed.record.State, bed.record.TipTree = StateDiagnosing, "tip-tree"
	bed.record.Units[0].Approver, bed.record.Units[1].Approver = "Ann", "Bob"
	proof := &Proof{Status: "failed", Tree: "tip-tree", AttemptID: "tip-attempt", SelectedGroups: []string{"fake-other"},
		Executions: []string{"fake-other"}, GroupIdentities: map[string]string{}, RedGroups: groups}
	for _, group := range groups {
		proof.SelectedGroups = append(proof.SelectedGroups, group.ID)
		proof.Executions = append(proof.Executions, group.ID)
		proof.GroupIdentities[group.ID] = "identity-" + group.ID
	}
	bed.record.Proof = proof
	prepare(&bed.record)
	store := NewStore(bed.root, nil)
	must(t, store.Create(bed.record))
	strictReassembly(t, &store)
	return store, &flakeLedger{}
}

func flakeSeams(script *flakeScript, ledger *flakeLedger, sources map[string]string) RedSeams {
	return RedSeams{Run: script.run, Adapter: fakeRedLanguage(fakeadapter.New()), Ledger: ledger, BaseCommit: "base-commit",
		MintOpid: func() (string, error) { return "op-1", nil }, Location: time.UTC,
		Sources: func(Record) (map[string]string, error) { return sources, nil }}
}

// everyMemberReturned asserts the batch dissolved, composed nothing, and
// each member's return carries text.
func everyMemberReturned(t *testing.T, store Store, text string) Record {
	t.Helper()
	record := load(t, store)
	if record.State != StateDissolved || record.Proof != nil {
		t.Fatalf("the batch was not returned whole: %+v", record)
	}
	for _, unit := range record.Units {
		if unit.State != UnitReturnPending || !strings.Contains(unit.Failure, text) {
			t.Fatalf("member %s was not returned naming %q: %+v", unit.GoalID, text, unit)
		}
	}
	neverHeldUnclassified(t, record)
	return record
}

func TestKnownFlakeLandsOnComposedProofAndRecordsSighting(t *testing.T) {
	t.Parallel()
	red := knownRed()
	identity := FlakeID(red.ID, red.Failures[0])
	lands := map[string]string{red.ID: "class-attempt", "fake-other": "tip-attempt"}
	for _, test := range []struct {
		name, class string
		run         DiagnosticResult
		sources     map[string]string
		text        string
	}{
		{"lands on a known flake", ClassKnownFlake, passedAt("identity-"+red.ID, red), lands, ""},
		{"no entry", "", passedAt("identity-"+red.ID, red), lands, "is not a known flake"},
		{"a pending entry carries nothing", ClassPendingFlake, passedAt("identity-"+red.ID, red), lands, "is not a known flake"},
		{"the recovered group cited from the failed attempt", ClassKnownFlake, passedAt("identity-"+red.ID, red),
			map[string]string{red.ID: "tip-attempt", "fake-other": "tip-attempt"}, "does not resolve"},
		{"a reused classification group", ClassKnownFlake, DiagnosticResult{AttemptID: "class-attempt",
			Evidence: []GroupEvidence{{ID: red.ID, Status: "reused", ExecutionIdentity: "identity-" + red.ID, CollectionComplete: true}}}, lands, "not an executed"},
		{"a classification at another identity", ClassKnownFlake, passedAt("identity-moved", red), lands, "execution identity"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			store, ledger := flakeBed(t, red)
			if test.class != "" {
				ledger.entries = []OpenEntry{{ID: "F", Identity: identity, Class: test.class, AllowanceUntil: flakeNow.Add(time.Hour), Owner: "Wido"}}
			}
			script := &flakeScript{tip: test.run}
			must(t, DiagnoseRed(store, testBatchID, "owner", []RedGroup{red}, "", flakeNow, flakeSeams(script, ledger, test.sources)))
			if test.text != "" {
				everyMemberReturned(t, store, test.text)
				return
			}
			record := load(t, store)
			classification := script.requests[len(script.requests)-1]
			if record.State != StateLanding || record.Proof.Status != "green" || classification.Tree != "tip-tree" || !classification.NeverReuse ||
				!slices.Equal(classification.Fresh, fakeadapter.New().Fresh) || record.History[len(record.History)-1].Actor != "owner" {
				t.Fatalf("the known flake did not land on a composed proof: %+v requests=%+v", record, script.requests)
			}
			if record.Proof.Sources[red.ID] != (Source{Kind: SourceReused, Attempt: "class-attempt"}) ||
				record.Proof.Sources["fake-other"] != (Source{Kind: SourceExecuted, Attempt: "tip-attempt"}) ||
				!slices.Equal(record.Proof.Flakes, []FlakeUse{{Identity: identity, EntryID: "F", AllowanceUntil: flakeNow.Add(time.Hour)}}) {
				t.Fatalf("composed sources=%+v flakes=%+v", record.Proof.Sources, record.Proof.Flakes)
			}
			if len(ledger.pendings) != 1 || ledger.pendings[0].TipTree != "tip-tree" || ledger.pendings[0].RedAttempt != "tip-attempt" ||
				ledger.pendings[0].GreenAttempt != "class-attempt" || len(ledger.promotions)+len(ledger.hangs) != 0 || len(ledger.entries) != 1 {
				t.Fatalf("the landing did not add exactly one sighting: %+v", ledger)
			}
		})
	}
}

func TestRedEvidenceWithoutIdentitiesNeverQualifies(t *testing.T) {
	t.Parallel()
	with := func(change func(*RedGroup)) RedGroup {
		red := knownRed()
		change(&red)
		return red
	}
	unidentified := knownRed()
	unidentified.ID = "fake-affected/ledger"
	unidentified.Failures = []Failure{{Report: "junit-xml", Classname: "com.example.PaymentTest", Name: "<compile>", Status: "failed"}}
	for _, test := range []struct {
		name   string
		groups []RedGroup
		clause string
	}{
		{"(a) failed with an empty failure list", []RedGroup{with(func(red *RedGroup) { red.Failures = nil })}, "0 of 0 identified"},
		{"(b) a known failure beside a missing test", []RedGroup{with(func(red *RedGroup) {
			red.Missing = []Failure{{Classname: "com.example.PaymentTest", Name: "settles", Status: "missing-terminal"}}
		})}, "1 missing"},
		{"(c) collection incomplete", []RedGroup{with(func(red *RedGroup) { red.CollectionComplete = false })}, "collection complete false"},
		{"(d) invalid", []RedGroup{with(func(red *RedGroup) { red.Status, red.NotRunReason = "invalid", "unparseable collection" })}, "status invalid"},
		{"(e) a stall", []RedGroup{with(func(red *RedGroup) {
			red.Status, red.Failures, red.NotRunReason = "unavailable", nil, "suite stalled in section s (silent); evidence preserved before kill at /ev (note)"
		})}, "status unavailable"},
		{"(f) the build terminal", []RedGroup{with(func(red *RedGroup) { red.Failures = unidentified.Failures })}, "0 of 1 identified"},
		{"(g) a section red", []RedGroup{with(func(red *RedGroup) { red.ID, red.Adapter = "section/land-fixtures", "section" })}, "reports no test identities"},
		{"(h) one group known, one unidentified", []RedGroup{knownRed(), unidentified}, "group fake-affected/ledger is red without a failing test identity"},
		{"(i) past the allowance", []RedGroup{knownRed()}, "allowance expired on Thu 1 Oct 12:00 UTC (owner Wido)"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			store, ledger := flakeBed(t, test.groups...)
			for _, group := range test.groups {
				for _, failure := range group.Failures {
					until := flakeNow.Add(time.Hour)
					if strings.HasPrefix(test.name, "(i)") {
						until = flakeNow.Add(-time.Hour)
					}
					ledger.known(FlakeID(group.ID, failure), until)
				}
			}
			script := &flakeScript{tip: passedAt("identity-"+test.groups[0].ID, test.groups...)}
			sources := map[string]string{"fake-other": "tip-attempt"}
			for _, group := range test.groups {
				sources[group.ID] = "class-attempt"
			}
			must(t, DiagnoseRed(store, testBatchID, "owner", test.groups, "", flakeNow, flakeSeams(script, ledger, sources)))
			everyMemberReturned(t, store, test.clause)
		})
	}
}

func TestNewIntermittentRedReturnsEveryMemberAndOpensAPendingFlake(t *testing.T) {
	t.Parallel()
	red, ledger := knownRed(), &flakeLedger{}
	for batch := 1; batch <= 2; batch++ {
		store, _ := flakeBed(t, red)
		script := &flakeScript{tip: passedAt("identity-"+red.ID, red)}
		must(t, DiagnoseRed(store, testBatchID, "owner", []RedGroup{red}, "", flakeNow, flakeSeams(script, ledger, nil)))
		everyMemberReturned(t, store, "pending flake (entry pending-"+fmt.Sprint(batch)+"), not a known one")
		everyMemberReturned(t, store, "land again with metasystem work land")
		sighting := ledger.pendings[batch-1]
		if len(ledger.pendings) != batch || sighting.TipTree != "tip-tree" || sighting.RedAttempt != "tip-attempt" || sighting.GreenAttempt != "class-attempt" ||
			sighting.Groups[0].LogPath != "logs/payments.log" || sighting.GreenLogPath != "logs/class.log" || len(ledger.promotions) != 0 || len(ledger.entries) != 0 {
			t.Fatalf("batch %d did not open or sight a pending flake only: %+v", batch, ledger)
		}
	}
}

func TestSecondRedReturnsEveryMemberWithTheUnionText(t *testing.T) {
	t.Parallel()
	for _, known := range []bool{true, false} {
		t.Run(fmt.Sprintf("known=%t", known), func(t *testing.T) {
			t.Parallel()
			red := knownRed()
			store, ledger := flakeBed(t, red)
			if known {
				ledger.known(FlakeID(red.ID, red.Failures[0]), flakeNow.Add(time.Hour))
			}
			script := &flakeScript{tip: DiagnosticResult{AttemptID: "class-attempt", Groups: []RedGroup{red}}}
			must(t, DiagnoseRed(store, testBatchID, "owner", []RedGroup{red}, "", flakeNow, flakeSeams(script, ledger, nil)))
			everyMemberReturned(t, store, "failed twice on the batch tip (fake-affected/payments): the members' changes break it together")
			if len(ledger.pendings)+len(ledger.promotions)+len(ledger.hangs) != 0 {
				t.Fatalf("a red classification changed the register: %+v", ledger)
			}
		})
	}
}

func TestStalledGroupOpensAHangEntryAndNeverCarriesALanding(t *testing.T) {
	t.Parallel()
	stall := RedGroup{ID: "fake-affected/payments", Status: "unavailable", Adapter: "fake", LogPath: "logs/stall.log", LongestSilentSeconds: 412,
		LongestZeroCPUSeconds: 400, NotRunReason: "suite stalled in section land-fixtures (silence); evidence preserved before kill at /ev/stall (dump: captured)",
		Missing: []Failure{{Classname: "com.example.PaymentTest", Name: "settles", Status: "missing-terminal"}}}
	for _, test := range []struct {
		name      string
		base      []DiagnosticResult
		hangs     []string
		trunkHeld bool
	}{
		{"on the tip", nil, []string{"tip-tree"}, false},
		{"on the base once", []DiagnosticResult{{AttemptID: "base-1", Groups: []RedGroup{stall}}}, []string{"", "tip-tree"}, false},
		{"on the base twice", []DiagnosticResult{{AttemptID: "base-1", Groups: []RedGroup{stall}}, {AttemptID: "base-2", Groups: []RedGroup{stall}}}, nil, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			store, ledger := flakeBed(t, stall)
			ledger.known(FlakeID(stall.ID, knownRed().Failures[0]), flakeNow.Add(time.Hour))
			script := &flakeScript{base: test.base, tip: passedAt("identity-"+stall.ID, stall)}
			must(t, DiagnoseRed(store, testBatchID, "owner", []RedGroup{stall}, "", flakeNow, flakeSeams(script, ledger, nil)))
			if test.trunkHeld {
				if record := load(t, store); record.State != StateHeldTrunkRed || ledger.calls != 1 {
					t.Fatalf("a stall twice on the base did not hold as a trunk red: %+v", record)
				}
				return
			}
			everyMemberReturned(t, store, "status unavailable")
			if len(ledger.hangs) != len(test.hangs) || len(script.requests) != len(test.base)+1 {
				t.Fatalf("hangs=%+v requests=%+v", ledger.hangs, script.requests)
			}
			for index, hang := range ledger.hangs {
				if hang.TipTree != test.hangs[index] || hang.Group.LogPath != "logs/stall.log" || hang.Evidence != (HangEvidence{EvidenceDir: "/ev/stall",
					Dump: "dump: captured", Section: "land-fixtures", LastStartedTest: "settles", LongestSilentSeconds: 412, LongestZeroCPUSeconds: 400}) {
					t.Fatalf("hang %d = %+v", index, hang)
				}
			}
		})
	}
}

func TestSourcesComeFromTheRetainedVerifier(t *testing.T) {
	t.Parallel()
	proof := Proof{Tree: "tip-tree", AttemptID: "tip", SelectedGroups: []string{"covered", "executed", "reused"}, Executions: []string{"executed"}}
	sources, err := ResolveSources(proof, map[string]string{"covered": "tip", "executed": "tip", "reused": "older"})
	if err != nil || !reflect.DeepEqual(sources, map[string]Source{"covered": {SourceCovered, "tip"}, "executed": {SourceExecuted, "tip"},
		"reused": {SourceReused, "older"}}) {
		t.Fatalf("sources=%+v err=%v", sources, err)
	}
	_, store := landingBed(t)
	must(t, store.Update(testBatchID, func(record *Record) error {
		record.Proof.SelectedGroups, record.Proof.Executions = proof.SelectedGroups, proof.Executions
		record.Proof.AttemptID = "tip"
		return nil
	}))
	must(t, RecordSources(store, testBatchID, "owner", flakeNow, func(Record) (map[string]string, error) {
		return map[string]string{"covered": "tip", "executed": "tip", "reused": "older"}, nil
	}))
	if got := load(t, store).Proof.Sources; !reflect.DeepEqual(got, sources) {
		t.Fatalf("recorded sources=%+v", got)
	}
	_, unresolved := landingBed(t)
	must(t, unresolved.Update(testBatchID, func(record *Record) error { record.Proof.SelectedGroups = proof.SelectedGroups; return nil }))
	must(t, RecordSources(unresolved, testBatchID, "owner", flakeNow, func(Record) (map[string]string, error) {
		return map[string]string{"covered": "tip-attempt"}, nil
	}))
	everyMemberReturned(t, unresolved, "group executed does not resolve")
	// A composed proof's recovered group resolves to the classification
	// attempt only at the tip plan's execution identity.
	red := knownRed()
	moved, ledger := flakeBed(t, red)
	ledger.known(FlakeID(red.ID, red.Failures[0]), flakeNow.Add(time.Hour))
	must(t, DiagnoseRed(moved, testBatchID, "owner", []RedGroup{red}, "", flakeNow, flakeSeams(&flakeScript{tip: passedAt("identity-moved", red)},
		ledger, map[string]string{red.ID: "class-attempt", "fake-other": "tip-attempt"})))
	everyMemberReturned(t, moved, "at the tip plan's execution identity")
}

// TestBaseRedThenGreenOpensOrPromotesAKnownFlake is the lane's half: main's
// red then executed green promotes each identity with the owner the lane
// chose, and step 3 then decides with the identity known.
func TestBaseRedThenGreenOpensOrPromotesAKnownFlake(t *testing.T) {
	t.Parallel()
	ledgerRed := knownRed()
	ledgerRed.ID, ledgerRed.Failures[0].Classname = "fake-affected/ledger", "com.example.LedgerTest"
	for _, test := range []struct {
		name, owner, machine string
		red                  RedGroup
	}{
		{"the member whose closure holds the owner unit", "Ann", "seat-a", knownRed()},
		{"else the batch's last member", "Bob", "seat", ledgerRed},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			store, ledger := flakeBedWith(t, func(record *Record) {
				record.Units[0].Closure, record.Units[0].Claim.Machine = fakeClosure([]string{"payments"}, nil), "seat-a"
			}, test.red)
			script := &flakeScript{base: []DiagnosticResult{{AttemptID: "base-red", Groups: []RedGroup{test.red}}, {AttemptID: "base-green",
				Evidence: []GroupEvidence{{ID: test.red.ID, Status: "passed", LogPath: "logs/base-green.log"}}}}, tip: passedAt("identity-"+test.red.ID, test.red)}
			sources := map[string]string{test.red.ID: "class-attempt", "fake-other": "tip-attempt"}
			// The owner unit payments is in goal-a's closure, so naming would
			// eject goal-a; the base red makes it main's flake instead.
			must(t, DiagnoseRed(store, testBatchID, "owner", []RedGroup{test.red}, "", flakeNow, flakeSeams(script, ledger, sources)))
			if len(ledger.promotions) != 1 {
				t.Fatalf("promotions=%+v", ledger.promotions)
			}
			promotion := ledger.promotions[0]
			if promotion.Owner != test.owner || promotion.OwnerMachine != test.machine || promotion.RedAttempt != "base-red" || promotion.GreenAttempt != "base-green" ||
				promotion.TipTree != "" || promotion.BaseTree != "base-tree" || promotion.BaseCommit != "base-commit" || promotion.Location != time.UTC ||
				promotion.GreenLogPath != "logs/base-green.log" || promotion.Groups[0].Failures[0] != test.red.Failures[0] {
				t.Fatalf("promotion=%+v", promotion)
			}
			if record := load(t, store); record.State != StateLanding || record.Proof.Sources[test.red.ID].Attempt != "class-attempt" {
				t.Fatalf("the red did not continue at step 3 with the identity known: %+v", record)
			}
		})
	}
}

// TestPendingFlakeBecomesKnownOnlyWithMainEvidence is the lane's half: tip
// sightings stay pending, only main's red then green promotes.
func TestPendingFlakeBecomesKnownOnlyWithMainEvidence(t *testing.T) {
	t.Parallel()
	red, ledger := knownRed(), &flakeLedger{}
	for sighting := 1; sighting <= 2; sighting++ {
		store, _ := flakeBed(t, red)
		must(t, DiagnoseRed(store, testBatchID, "owner", []RedGroup{red}, "", flakeNow,
			flakeSeams(&flakeScript{tip: passedAt("identity-"+red.ID, red)}, ledger, nil)))
		everyMemberReturned(t, store, "pending flake")
	}
	if len(ledger.pendings) != 2 || len(ledger.promotions) != 0 {
		t.Fatalf("a tip sighting promoted: %+v", ledger)
	}
	store, _ := flakeBed(t, red)
	script := &flakeScript{base: []DiagnosticResult{{AttemptID: "base-red", Groups: []RedGroup{red}}}, tip: passedAt("identity-"+red.ID, red)}
	must(t, DiagnoseRed(store, testBatchID, "owner", []RedGroup{red}, "", flakeNow,
		flakeSeams(script, ledger, map[string]string{red.ID: "class-attempt", "fake-other": "tip-attempt"})))
	if len(ledger.promotions) != 1 || ledger.promotions[0].RedAttempt != "base-red" || load(t, store).State != StateLanding {
		t.Fatalf("main's red then green did not promote: %+v", ledger.promotions)
	}
}

func TestExpiredAllowanceBetweenPredicateAndPublicationRefusesPublication(t *testing.T) {
	t.Parallel()
	use := FlakeUse{Identity: "tr-payments", EntryID: "F", AllowanceUntil: flakeNow.Add(time.Hour)}
	for _, test := range []struct {
		name    string
		now     time.Time
		entries []OpenEntry
		resumed bool
		refused bool
	}{
		{"inside the allowance it publishes", flakeNow, []OpenEntry{{ID: "F", Identity: "tr-payments", Class: ClassKnownFlake, AllowanceUntil: use.AllowanceUntil}}, false, false},
		{"expired after the predicate", flakeNow.Add(2 * time.Hour), []OpenEntry{{ID: "F", Identity: "tr-payments", Class: ClassKnownFlake, AllowanceUntil: use.AllowanceUntil}}, false, true},
		{"expired when a restarted owner resumes", flakeNow.Add(2 * time.Hour), []OpenEntry{{ID: "F", Identity: "tr-payments", Class: ClassKnownFlake, AllowanceUntil: use.AllowanceUntil}}, true, true},
		{"closed by a person", flakeNow, nil, false, true},
		{"reclassified as a trunk red", flakeNow, []OpenEntry{{ID: "F", Identity: "tr-payments", Class: ClassTrunkRed}}, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, store := landingBed(t)
			must(t, store.Update(testBatchID, func(record *Record) error {
				record.Proof.Flakes = []FlakeUse{use}
				if test.resumed {
					record.Landing = &LandingProgress{Base: record.BaseTree, Commits: map[string]string{"goal-a": "commit-goal-a"}}
				}
				return nil
			}))
			var events []string
			seams := greenLandSeams(&events)
			seams.FlakeRegister = func() ([]OpenEntry, error) { return test.entries, nil }
			seams.Now = func() (time.Time, error) { return test.now, nil }
			err := LandSeries(store, testBatchID, "owner", flakeNow, seams)
			if !test.refused {
				must(t, err)
				if !slices.Contains(events, "push") {
					t.Fatalf("an allowed composed proof did not publish: %v", events)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "BATCH_FLAKE_ALLOWANCE_REFUSED") || slices.Contains(events, "push") {
				t.Fatalf("publication was not refused: err=%v events=%v", err, events)
			}
			everyMemberReturned(t, store, "known flake F")
		})
	}
}
