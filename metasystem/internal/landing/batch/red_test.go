package batch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
			RedSeams{Run: green, Adapter: fakeRedLanguage(fakeadapter.New())}))
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
			fake, red, ledger := fakeadapter.New(), paymentRed(), &redLedger{}
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
