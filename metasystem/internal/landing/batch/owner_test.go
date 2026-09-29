package batch

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type ownerBed struct {
	store             Store
	owner             *Owner
	now               time.Time
	tree, claimTree   string
	sample            proofrun.LoadSample
	samples           []proofrun.LoadSample
	claim             Claim
	fetchErr          error
	launches          int
	window            string
	launched          proofrun.LoadSample
	events, ticks     []string
	redOutcomes       []string
	lockDir, queueDir string
}

func ownerRecord(id, state string, joinedAt time.Time) Record {
	record := Record{Schema: 1, BatchID: id, State: state}
	if !joinedAt.IsZero() {
		record.Units = []Unit{{GoalID: "goal-a", Chain: "chain-a", Claim: Claim{Machine: "landing", Lineage: "owner", Epoch: 1, Revision: 2, AccountingRevision: 1}, State: UnitJoined}}
		record.History = []HistoryEntry{{At: joinedAt.Format(time.RFC3339Nano), Verb: "join", Detail: "goal-a joined"}}
	}
	if state == StateHeldTrunkRed {
		record.TrunkRed = &TrunkRedHold{Opid: "opid", Opids: []string{"opid"}}
	}
	return record
}

func newOwnerBed(t *testing.T, record Record, now time.Time) *ownerBed {
	t.Helper()
	root := t.TempDir()
	bed := &ownerBed{now: now, tree: "tree", sample: proofrun.LoadSample{OverlapKnown: true}, lockDir: filepath.Join(root, "owner-lock"), queueDir: filepath.Join(root, "owner-queue")}
	bed.store = NewStore(root, scriptedProber{7: identity.Alive})
	must(t, bed.store.Create(record))
	settings, err := config.NewBatchLanding(root, time.Minute, func() time.Time { return bed.now })
	must(t, err)
	readClaim := func(_, tree, _, _ string) (Claim, error) {
		bed.events = append(bed.events, "reconcile:"+tree)
		if tree != bed.claimTree {
			return Claim{}, os.ErrNotExist
		}
		return bed.claim, nil
	}
	returns := ReturnSeams{Read: func(_, tree, _ string) (ReturnLedgerGoal, error) {
		bed.events = append(bed.events, "return:"+tree)
		if tree == "A" {
			return ReturnLedgerGoal{}, os.ErrNotExist
		}
		return ReturnLedgerGoal{}, nil
	}}
	bed.owner, err = NewOwner(OwnerOptions{Store: bed.store, Settings: settings, Actor: "landing+owner", PID: 7,
		LockDir: bed.lockDir, QueueDir: bed.queueDir, Now: func() time.Time { return bed.now }, FetchTree: func() (string, error) { return bed.tree, bed.fetchErr }, ReadClaim: readClaim, Returns: returns, Rebind: func(id, tree string) error {
			bed.ticks = append(bed.ticks, id)
			bed.events = append(bed.events, "rebind:"+id+":"+tree)
			return nil
		}, Mint: func() (string, error) { return "minted-opid", nil }, LogRed: func(id string, outcome TrunkRedRecordOutcome) {
			bed.redOutcomes = append(bed.redOutcomes, id+"="+string(outcome))
		}, BaseCommit: func(tree string) (string, error) { return "commit-" + tree, nil },
		RunDiagnostic: func(string, DiagnosticRequest, Claim) (DiagnosticResult, error) {
			return DiagnosticResult{AttemptID: "diagnostic"}, nil
		},
		DescendsFrom: func(descendant, ancestor string) (bool, error) { return descendant != ancestor, nil },
		Sample: func() proofrun.LoadSample {
			if len(bed.samples) == 0 {
				return bed.sample
			}
			sample := bed.samples[0]
			bed.samples = bed.samples[1:]
			return sample
		}, Admission: func(proofrun.LoadSample) proofrun.AdmissionCap { return proofrun.AdmissionCap{Max: 8} }, Launch: func(request Dispatch) error {
			bed.launches++
			bed.launched, bed.window = request.Sample, request.Window
			return nil
		}, ProbeRun: func(string, Record) (RunProbe, error) { return RunProbe{State: RunLive}, nil }, After: func(time.Duration) <-chan time.Time { return make(chan time.Time) }, Report: func(string, error) {},
		// An empty readable board: nothing on any seat is underway.
		Pipeline: &scriptedBoard{picture: BoardPicture{Readable: true}},
		// The early acts do nothing unless a witness scripts them.
		Early: EarlySeams{Cheap: func(Record) (EarlyResult, error) { return EarlyResult{}, nil },
			Prove:  func(Record) (EarlyResult, error) { return EarlyResult{}, nil },
			Budget: func(Record) (bool, string) { return true, "" }}})
	must(t, err)
	return bed
}

func TestBatchOwnerRecordsHeldTrunkRed(t *testing.T) {
	now := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	newRecord := func() Record {
		record := ownerRecord(testBatchID, StateHeldTrunkRed, now.Add(-time.Minute))
		record.TrunkRed.Red = TrunkRed{BatchID: testBatchID, AttemptID: "attempt", Groups: []RedGroup{{ID: "fast"}}}
		return record
	}
	t.Run("pending failed and recorded", func(t *testing.T) {
		bed := newOwnerBed(t, newRecord(), now)
		calls := 0
		ledger := recordOwner(func(opid string, _ TrunkRed) ([]EntryRef, error) {
			calls++
			switch calls {
			case 1:
				return nil, ErrTrunkRedRecordPending
			case 2:
				return nil, &TrunkRedRecordFailed{Outcome: "abandoned", Evidence: "owner ended"}
			default:
				if opid != "opid-2" {
					t.Fatalf("record opid=%q, want opid-2", opid)
				}
				return []EntryRef{{ID: "entry-1", Group: "fast"}}, nil
			}
		})
		bed.store = bed.store.WithLedgerOwner(ledger)
		bed.owner.store = bed.store
		mintCalls := 0
		bed.owner.mint = func() (string, error) { mintCalls++; return "opid-2", nil }
		if err := bed.owner.Tick(testBatchID); err != nil {
			t.Fatalf("pending ledger transaction was reported as a tick error: %v", err)
		}
		var failed *TrunkRedRecordFailed
		if err := bed.owner.Tick(testBatchID); !errors.As(err, &failed) {
			t.Fatalf("failed tick error=%v", err)
		}
		afterFailure := load(t, bed.store)
		if afterFailure.TrunkRed.Opid != "opid-2" || strings.Join(afterFailure.TrunkRed.Opids, ",") != "opid,opid-2" || mintCalls != 1 {
			t.Fatalf("failed tick record=%+v mint calls=%d", afterFailure.TrunkRed, mintCalls)
		}
		must(t, bed.owner.Tick(testBatchID))
		final := load(t, bed.store)
		if len(final.TrunkRed.Entries) != 1 || final.TrunkRed.Entries[0].ID != "entry-1" || !slices.Equal(bed.redOutcomes, []string{testBatchID + "=recorded"}) {
			t.Fatalf("recorded tick hold=%+v outcomes=%v", final.TrunkRed, bed.redOutcomes)
		}
	})
	t.Run("moved batch is logged", func(t *testing.T) {
		bed := newOwnerBed(t, newRecord(), now)
		ledger := recordOwner(func(string, TrunkRed) ([]EntryRef, error) {
			must(t, bed.store.Update(testBatchID, func(record *Record) error { record.State = StateOpen; return nil }))
			return []EntryRef{{ID: "entry-1", Group: "fast"}}, nil
		})
		bed.store = bed.store.WithLedgerOwner(ledger)
		bed.owner.store = bed.store
		must(t, bed.owner.Tick(testBatchID))
		if !slices.Equal(bed.redOutcomes, []string{testBatchID + "=moved-on"}) || load(t, bed.store).State != StateOpen {
			t.Fatalf("outcomes=%v record=%+v", bed.redOutcomes, load(t, bed.store))
		}
	})
}

func TestBatchOwnerSkipsRecordedHoldAndReleasesLockBeforeLedger(t *testing.T) {
	now := time.Unix(10, 0)
	t.Run("recorded hold", func(t *testing.T) {
		record := ownerRecord(testBatchID, StateHeldTrunkRed, now.Add(-time.Minute))
		record.TrunkRed.Red = TrunkRed{BaseTree: "tree", Groups: []RedGroup{{ID: "fast"}}}
		record.TrunkRed.Entries = []EntryRef{{ID: "entry", Group: "fast"}}
		bed := newOwnerBed(t, record, now)
		calls := 0
		bed.store = bed.store.WithLedgerOwner(recordOwner(func(string, TrunkRed) ([]EntryRef, error) {
			calls++
			return nil, errors.New("record should not be called")
		}))
		bed.owner.store = bed.store
		must(t, bed.owner.Tick(testBatchID))
		if calls != 0 || len(bed.redOutcomes) != 0 {
			t.Fatalf("recorded hold called ledger %d times and logged %v", calls, bed.redOutcomes)
		}
	})
	t.Run("proof lock release", func(t *testing.T) {
		record := ownerRecord(testBatchID, StateHeldTrunkRed, now.Add(-time.Minute))
		record.TrunkRed.Red = TrunkRed{BaseTree: "old", Groups: []RedGroup{{ID: "fast"}}}
		bed := newOwnerBed(t, record, now)
		lock := bed.owner.lock(testBatchID)
		if state, err := lock.poll(); err != nil || state != lockAcquired {
			t.Fatalf("acquire fixture proof lock: state=%v error=%v", state, err)
		}
		bed.owner.locks[testBatchID] = lock
		bed.store = bed.store.WithLedgerOwner(recordOwner(func(string, TrunkRed) ([]EntryRef, error) {
			if _, err := os.Stat(filepath.Join(bed.lockDir, "batch-"+testBatchID)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("ledger call retained the proof lock: %v", err)
			}
			return []EntryRef{{ID: "entry", Group: "fast"}}, nil
		}))
		bed.owner.store = bed.store
		must(t, bed.owner.Tick(testBatchID))
	})
}

func TestBatchOwnerSavesGreenTreeThatCannotClear(t *testing.T) {
	now := time.Unix(10, 0)
	assembly, store, ledger := heldReopenBed(t,
		expectedAssembly("moved-tree", []string{"goal-a", "goal-b"}, []string{"chain-a", "chain-b"}, []string{"owner-a", "owner-ab"}))
	record := load(t, store)
	bed := newOwnerBed(t, record, now)
	bed.tree = assembly.moved
	ledger.open[0].LastBaseCommit = "commit-" + assembly.moved
	bed.store = store.WithLedgerOwner(ledger)
	bed.owner.store = bed.store
	diagnostics := 0
	bed.owner.runDiagnostic = func(string, DiagnosticRequest, Claim) (DiagnosticResult, error) {
		diagnostics++
		return DiagnosticResult{AttemptID: "green"}, nil
	}
	must(t, bed.owner.Tick(testBatchID))
	must(t, bed.owner.Tick(testBatchID))
	if record := load(t, bed.store); record.TrunkRed.CheckedTree != assembly.moved || diagnostics != 1 {
		t.Fatalf("blocked green tree=%q diagnostics=%d, want saved tree and one run", record.TrunkRed.CheckedTree, diagnostics)
	}
}

func TestBatchOwnerRetriesGreenTipClearAfterLandingFinishes(t *testing.T) {
	now := time.Unix(10, 0)
	record := ownerRecord(testBatchID, StateLanding, now.Add(-time.Minute))
	record.Proof = &Proof{Status: "green", AttemptID: "tip-green", BaseCommit: "next-commit", Passed: []string{"fast"}}
	bed := newOwnerBed(t, record, now)
	ledger := &clearingLedger{open: []OpenEntry{{ID: "entry", Group: "fast", LastBaseCommit: "old-commit"}}, openErrors: []error{os.ErrPermission, nil}}
	bed.store = bed.store.WithLedgerOwner(ledger)
	bed.owner.store = bed.store
	bed.owner.launch = func(Dispatch) error {
		bed.launches++
		return bed.store.Update(testBatchID, func(record *Record) error { record.State = StateLanded; return nil })
	}
	var reports []error
	bed.owner.report = func(_ string, errorValue error) { reports = append(reports, errorValue) }
	must(t, bed.owner.Tick(testBatchID))
	bed.owner.settle()
	if record := load(t, bed.store); record.State != StateLanded || len(ledger.cleared) != 0 || len(reports) != 1 {
		t.Fatalf("first pass record=%+v clears=%+v reports=%v", record, ledger.cleared, reports)
	}
	bed.owner.Resume()
	bed.owner.settle()
	final := load(t, bed.store)
	_, complete := greenTipClearStatus(final)
	if len(ledger.cleared) != 1 || ledger.cleared[0].green.AttemptID != "tip-green" || !complete || bed.launches != 1 {
		t.Fatalf("retry record=%+v clears=%+v launches=%d", final, ledger.cleared, bed.launches)
	}
}

func TestBatchOwnerResumesLandingAfterTrunkRedClearError(t *testing.T) {
	record := ownerRecord(testBatchID, StateLanding, time.Unix(1, 0))
	record.Proof = &Proof{Status: "green", BaseCommit: "next-commit"}
	bed := newOwnerBed(t, record, time.Unix(2, 0))
	ledger := &clearingLedger{openErr: os.ErrPermission}
	bed.store = bed.store.WithLedgerOwner(ledger)
	bed.owner.store = bed.store
	var reported error
	bed.owner.report = func(id string, err error) {
		if id != testBatchID {
			t.Fatalf("reported batch=%q", id)
		}
		reported = err
	}
	must(t, bed.owner.Tick(testBatchID))
	bed.owner.settle()
	if bed.launches != 1 || reported == nil || !strings.Contains(reported.Error(), os.ErrPermission.Error()) {
		t.Fatalf("launches=%d reported=%v", bed.launches, reported)
	}
}

func TestBatchOwnerHoldsLandingOnRegisteredTrunkRedButNotStatusAlone(t *testing.T) {
	for _, test := range []struct {
		name     string
		open     []OpenEntry
		state    string
		launches int
	}{
		{name: "missing or overdue status has no register entry", state: StateLanded, launches: 1},
		{name: "cadence red register entry", open: []OpenEntry{{ID: "cadence-entry", Group: "section/deep", LastBaseCommit: "trunk-before"}}, state: StateHeldTrunkRed},
	} {
		t.Run(test.name, func(t *testing.T) {
			now := time.Unix(20, 0)
			record := ownerRecord(testBatchID, StateLanding, now.Add(-time.Minute))
			record.BaseTree = "tree"
			record.Proof = &Proof{Status: "green", AttemptID: "standard-green", BaseCommit: "trunk-after"}
			bed := newOwnerBed(t, record, now)
			ledger := &clearingLedger{open: slices.Clone(test.open)}
			bed.store = bed.store.WithLedgerOwner(ledger)
			bed.owner.store = bed.store
			bed.owner.launch = func(Dispatch) error {
				bed.launches++
				return bed.store.Update(testBatchID, func(record *Record) error { record.State = StateLanded; return nil })
			}
			must(t, bed.owner.Tick(testBatchID))
			bed.owner.settle()
			got := load(t, bed.store)
			if got.State != test.state || bed.launches != test.launches {
				t.Fatalf("record=%+v launches=%d", got, bed.launches)
			}
			if test.state == StateHeldTrunkRed && (got.TrunkRed == nil || len(got.TrunkRed.Entries) != 1 || got.TrunkRed.Entries[0].ID != "cadence-entry") {
				t.Fatalf("registered cadence hold=%+v", got.TrunkRed)
			}
		})
	}
}

func TestBatchOwnerLaunchesAtMaximumWait(t *testing.T) {
	witness(t, !strings.Contains(string(contents(t, "owner.go")), "time.Now("), "owner reads the wall clock")
	joined := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	bed := newOwnerBed(t, ownerRecord(testBatchID, StateOpen, joined), joined.Add(time.Minute-time.Second))
	// The max wait decides only when the host board cannot be read (D14).
	bed.owner.pipeline = &scriptedBoard{picture: BoardPicture{Reason: "registry: unreadable"}}
	must(t, bed.owner.Tick(testBatchID))
	_, lockErr := os.Stat(bed.lockDir)
	_, queueErr := os.Stat(bed.queueDir)
	witness(t, bed.launches == 0 && os.IsNotExist(lockErr) && os.IsNotExist(queueErr), "before deadline launches=%d lock=%v queue=%v", bed.launches, lockErr, queueErr)
	bed.now = joined.Add(time.Minute)
	bed.samples = []proofrun.LoadSample{{OverlapKnown: true}, {OverlapKnown: true, OverlappingLocal: 2}}
	must(t, bed.owner.Tick(testBatchID))
	bed.owner.settle()
	witness(t, bed.launches == 1 && bed.window == "expired" && bed.launched.OverlappingLocal == 2, "deadline launch=%d/%s sample=%+v", bed.launches, bed.window, bed.launched)
}

func TestBatchOwnerWakesOnSignal(t *testing.T) {
	bed := newOwnerBed(t, ownerRecord(testBatchID, StateOpen, time.Time{}), time.Unix(1, 0))
	passes, waiting, release := make(chan struct{}, 2), make(chan struct{}, 1), make(chan struct{})
	bed.owner.fetchTree = func() (string, error) { passes <- struct{}{}; return "tree", nil }
	bed.owner.after = func(time.Duration) <-chan time.Time { waiting <- struct{}{}; <-release; return make(chan time.Time) }
	wake, stop, done := make(chan struct{}, 1), make(chan struct{}), make(chan struct{})
	go func() { bed.owner.Loop(time.Minute, wake, stop); close(done) }()
	select {
	case <-passes:
	case <-done:
		t.Fatal("batch owner returned before its first pass")
	}
	select {
	case <-waiting:
	case <-done:
		t.Fatal("batch owner returned before waiting for a wake signal")
	}
	wake <- struct{}{}
	close(stop)
	close(release)
	select {
	case <-passes:
	case <-done:
		select {
		case <-passes:
		default:
			t.Fatal("owner stopped without running the wake pass")
		}
	}
	<-done
}

func TestBatchOwnerReconcilesAtFetchedTree(t *testing.T) {
	bed := newOwnerBed(t, ownerRecord(testBatchID, StateOpen, time.Time{}), time.Unix(2, 0))
	path, _ := bed.store.recordPath(testBatchID)
	before := string(contents(t, path))
	bed.fetchErr = os.ErrInvalid
	witness(t, bed.owner.Tick(testBatchID) == os.ErrInvalid && string(contents(t, path)) == before && len(bed.events) == 0, "fetch failure wrote state or ran a step")
	bed.fetchErr = nil
	pending := Unit{GoalID: "old", Chain: "old-chain", Claim: Claim{Machine: "seat", Lineage: "old", Epoch: 1, Revision: 1, AccountingRevision: 1}, State: UnitReturnPending, unitRecordFields: unitRecordFields{Outcome: UnitEjected, Failure: "old"}}
	must(t, bed.store.Update(testBatchID, func(record *Record) error { record.Units = append(record.Units, pending); return nil }))
	bed.tree = "A"
	must(t, bed.owner.Tick(testBatchID))
	joining := Unit{GoalID: "goal-a", Chain: "chain-a", Claim: Claim{Machine: "landing", Lineage: "owner", Epoch: 1, Revision: 2, AccountingRevision: 1}, State: UnitJoining}
	must(t, bed.store.Update(testBatchID, func(record *Record) error {
		record.Units = append(record.Units, joining)
		appendUnitHistory(record, bed.now, "join", "seat", joining.GoalID, "", UnitJoining)
		return nil
	}))
	bed.tree, bed.claimTree, bed.claim = "B", "B", joining.Claim
	must(t, bed.owner.Tick(testBatchID))
	record := load(t, bed.store)
	witness(t, record.Units[1].State == UnitJoined && slices.Equal(bed.events, []string{"return:A", "rebind:" + testBatchID + ":A", "reconcile:B", "return:B", "rebind:" + testBatchID + ":B"}), "record=%+v events=%v", record, bed.events)
}

func TestBatchOwnerReturnsAfterReconcile(t *testing.T) {
	record := ownerRecord(testBatchID, StateOpen, time.Time{})
	record.Units = []Unit{{GoalID: "goal-a", Chain: "chain-a", Claim: Claim{Machine: "landing", Lineage: "owner", Epoch: 1, Revision: 2, AccountingRevision: 1}, State: UnitJoining}}
	bed := newOwnerBed(t, record, time.Unix(3, 0))
	must(t, bed.owner.Tick(testBatchID))
	unit := load(t, bed.store).Units[0]
	witness(t, unit.State == UnitEjected && unit.ReturnDisposition == ReturnAlreadyReturned && slices.Equal(bed.events, []string{"reconcile:tree", "return:tree", "rebind:" + testBatchID + ":tree"}), "unit=%+v events=%v", unit, bed.events)
}

func TestBatchOwnerHoldsPersistentUnknownCensus(t *testing.T) {
	joined := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	bed := newOwnerBed(t, ownerRecord(testBatchID, StateOpen, joined), joined.Add(time.Minute-time.Second))
	bed.sample = proofrun.LoadSample{}
	must(t, bed.owner.Tick(testBatchID))
	witness(t, !load(t, bed.store).CensusHold, "hold set before maximum wait")
	bed.now = joined.Add(time.Minute)
	must(t, bed.owner.Tick(testBatchID))
	path, _ := bed.store.recordPath(testBatchID)
	held := string(contents(t, path))
	must(t, bed.owner.Tick(testBatchID))
	witness(t, load(t, bed.store).CensusHold && string(contents(t, path)) == held && bed.launches == 0, "unknown census record=%+v launches=%d", load(t, bed.store), bed.launches)
	bed.sample = proofrun.LoadSample{OverlapKnown: true}
	must(t, bed.owner.Tick(testBatchID))
	record := load(t, bed.store)
	history := string(contents(t, path))
	witness(t, !record.CensusHold && strings.Count(history, `"verb": "hold"`) == 1 && strings.Count(history, `"verb": "unhold"`) == 1, "hold history=%+v", record.History)
}

func TestBatchOwnerResumesEveryLiveBatch(t *testing.T) {
	joined := time.Unix(1, 0)
	bed := newOwnerBed(t, ownerRecord(testBatchID, StateOpen, time.Time{}), joined.Add(time.Hour))
	must(t, bed.store.Create(ownerRecord("01j5x00000000000000000ba02", StateOpen, time.Time{})))
	must(t, bed.store.Create(ownerRecord("01j5x00000000000000000ba03", StateLanded, time.Time{})))
	must(t, bed.store.Create(ownerRecord("01j5x00000000000000000ba04", StateHeldTrunkRed, joined)))
	bed.owner.Resume()
	witness(t, slices.Equal(bed.ticks, []string{testBatchID, "01j5x00000000000000000ba02", "01j5x00000000000000000ba04"}) && bed.launches == 0, "ticks=%v launches=%d", bed.ticks, bed.launches)
}

func TestBatchOwnerReportsResumeEnumerationFailure(t *testing.T) {
	bed := newOwnerBed(t, ownerRecord(testBatchID, StateOpen, time.Time{}), time.Unix(5, 0))
	reported := make(chan error, 1)
	bed.owner.glob = func(string) ([]string, error) { return nil, os.ErrPermission }
	bed.owner.report = func(id string, err error) {
		if id != "" {
			t.Fatalf("enumeration failure reported as batch %q", id)
		}
		reported <- err
	}
	bed.owner.Resume()
	select {
	case err := <-reported:
		witness(t, strings.Contains(err.Error(), "enumerate landing batches") && strings.Contains(err.Error(), os.ErrPermission.Error()), "reported error=%v", err)
	default:
		t.Fatal("Resume dropped the Glob failure")
	}
}

func TestOnlyTrunkRedHoldsALanding(t *testing.T) {
	t.Parallel()
	flakes := []OpenEntry{
		{ID: "pending", Group: "fast", Class: ClassPendingFlake, LastBaseCommit: "trunk-before"},
		{ID: "known", Group: "fast", Class: ClassKnownFlake, LastBaseCommit: "trunk-before", AllowanceUntil: time.Unix(10, 0)},
		{ID: "hang", Group: "fast", Class: ClassHang, LastBaseCommit: "trunk-before"},
		{ID: "quality", Group: "fast", Class: ClassQuality, LastBaseCommit: "trunk-before"},
	}
	for _, test := range []struct {
		name  string
		open  []OpenEntry
		state string
	}{
		{name: "flake, hang and quality entries", open: flakes, state: StateLanded},
		{name: "trunk red", open: append(slices.Clone(flakes), OpenEntry{ID: "trunk", Group: "slow", Class: ClassTrunkRed, LastBaseCommit: "trunk-before"}), state: StateHeldTrunkRed},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			now := time.Unix(20, 0)
			record := ownerRecord(testBatchID, StateLanding, now.Add(-time.Minute))
			record.BaseTree = "tree"
			record.Proof = &Proof{Status: "green", AttemptID: "tip-green", BaseCommit: "trunk-after", Passed: []string{"fast"}}
			bed := newOwnerBed(t, record, now)
			ledger := &clearingLedger{open: slices.Clone(test.open)}
			bed.store = bed.store.WithLedgerOwner(ledger)
			bed.owner.store = bed.store
			bed.owner.launch = func(Dispatch) error {
				bed.launches++
				return bed.store.Update(testBatchID, func(record *Record) error { record.State = StateLanded; return nil })
			}
			must(t, bed.owner.Tick(testBatchID))
			bed.owner.settle()
			got := load(t, bed.store)
			if got.State != test.state || len(ledger.cleared) != 0 {
				t.Fatalf("state=%s launches=%d cleared=%+v, want %s and no flake closed by a green tip", got.State, bed.launches, ledger.cleared, test.state)
			}
			if test.state == StateHeldTrunkRed && (len(got.TrunkRed.Entries) != 1 || got.TrunkRed.Entries[0].ID != "trunk") {
				t.Fatalf("held on %+v, want only the trunk red", got.TrunkRed.Entries)
			}
		})
	}
}

// movedBaseBed seals batch B on base-1 behind the production Launch shape: a
// run seals an open batch, plans with its token, then waits for the test and
// finishes green with the group reused by identity, as a delivery proof on an
// unchanged group identity does. Main then moves to base-2, where the store's
// reassembly composes the members with the given outcome.
func movedBaseBed(t *testing.T, units []Unit, move BaseMove, assemble func(string, []Unit) ([]string, error)) (*ownerBed, chan Dispatch, chan struct{}, *[]string) {
	t.Helper()
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	record := ownerRecord(testBatchID, StateSealed, now.Add(-time.Minute))
	if units != nil {
		record.Units = units
	}
	record.BaseTree, record.SelectedGroups, record.TipTree = "base-1", []string{"unit-standard"}, "tip-1"
	for range joinedUnits(record.Units) {
		record.PrefixTrees = append(record.PrefixTrees, "tip-1")
	}
	bed := newOwnerBed(t, record, now)
	bed.tree, bed.sample = "base-1", hostSample(1)
	minted := 0
	bed.owner.mint = func() (string, error) { minted++; return "token-" + string(rune('0'+minted)), nil }
	entered, gate, reported := make(chan Dispatch, 4), make(chan struct{}), &[]string{}
	bed.owner.store = bed.owner.store.WithReassembly(assemble, nil, nil)
	bed.owner.report = func(id string, err error) { *reported = append(*reported, id+": "+err.Error()) }
	bed.owner.baseMove = func(from, to string) (BaseMove, error) {
		if from != "base-1" || to != "base-2" {
			return BaseMove{}, errors.New("unexpected move " + from + ".." + to)
		}
		return move, nil
	}
	plan := testpolicy.Plan{SelectedGroups: []string{"unit-standard"}}
	bed.owner.launch = func(request Dispatch) error {
		if err := bed.owner.store.Update(request.ID, func(record *Record) error {
			if record.State == StateOpen {
				record.Transition(StateSealed, bed.now, "seal", "owner", "")
			}
			return nil
		}); err != nil {
			return err
		}
		if _, err := RequireProofPlan(bed.owner.store, request.ID, "owner", request.Window, request.Token, request.Sample, plan, bed.now); err != nil {
			return err
		}
		entered <- request
		<-gate
		return FinishProof(bed.owner.store, request.ID, "owner", request.Token, proofrun.TestResult{AttemptID: "attempt-" + request.Token,
			Groups:   []proofrun.GroupResult{{ID: "unit-standard", Status: "reused", ReuseAttempt: "attempt-before", InputManifest: []string{"internal/**"}}},
			Delivery: proofrun.DeliveryJudgment{Sufficient: true}}, nil, bed.now)
	}
	return bed, entered, gate, reported
}

// R6 (b): a proof in flight on base-1 when a landing moved an engine path in
// its manifest is reopened on base-2 at the tick, its completion is discarded
// by its token, and one ordinary re-proof lands on what reuse by identity kept.
func TestOthersReopenAndReproveWhenAnInputMoved(t *testing.T) {
	t.Parallel()
	move := BaseMove{Changed: []string{"records/2026-09-28.md"}, LandedBy: dispatchB}
	bed, entered, gate, reported := movedBaseBed(t, nil, move, func(base string, units []Unit) ([]string, error) {
		return []string{"tip-" + strings.TrimPrefix(base, "base-")}, nil
	})
	must(t, bed.owner.Tick(testBatchID))
	first := <-entered
	bed.tree = "base-2"
	must(t, bed.owner.Tick(testBatchID))
	kept := load(t, bed.owner.store)
	witness(t, kept.State == StateProving && kept.BaseTree == "base-1" && kept.Proof.Token == first.Token,
		"a records-only move reopened a proof whose inputs arrive with its result: %+v", kept)
	bed.owner.baseMove = func(string, string) (BaseMove, error) {
		return BaseMove{Changed: []string{"internal/landing/batch/owner.go"}, LandedBy: dispatchB}, nil
	}
	must(t, bed.owner.Tick(testBatchID))
	reopened := load(t, bed.owner.store)
	witness(t, reopened.State == StateOpen && reopened.BaseTree == "base-2" && reopened.TipTree == "tip-2" && reopened.Proof == nil &&
		reopened.History[len(reopened.History)-1].Verb == "trunk-moved", "an engine move did not reopen the proving batch on base-2: %+v", reopened)
	gate <- struct{}{}
	bed.owner.Complete(<-bed.owner.completions)
	witness(t, len(*reported) == 1 && strings.Contains((*reported)[0], "BATCH_PROOF_STALE_COMPLETION"), "old completion: reports=%q", *reported)
	must(t, bed.owner.Tick(testBatchID))
	second := <-entered
	gate <- struct{}{}
	bed.owner.settle()
	landed := load(t, bed.owner.store)
	witness(t, second.Token != first.Token && landed.State == StateLanding && landed.Proof.Token == second.Token && landed.Proof.Tree == "tip-2" &&
		landed.Proof.Reuse["unit-standard"] == "attempt-before" && len(landed.Proof.Executions) == 0,
		"re-proof: first=%+v second=%+v proof=%+v", first, second, landed.Proof)
}

// R6 (c): a member whose changes conflict with what the other batch landed is
// ejected with the files named and the next command; the survivors reopen.
func TestRebaseConflictEjectsTheMemberAndNamesFiles(t *testing.T) {
	t.Parallel()
	claim := Claim{Machine: "landing", Lineage: "owner", Epoch: 1, Revision: 2, AccountingRevision: 1}
	units := []Unit{{GoalID: "goal-c", Chain: "chain-c", Claim: claim, State: UnitJoined}, {GoalID: "goal-d", Chain: "chain-d", Claim: claim, State: UnitJoined}}
	move := BaseMove{Changed: []string{"internal/landing/batch/shared.go"}, LandedBy: dispatchB}
	bed, _, _, _ := movedBaseBed(t, units, move, func(base string, members []Unit) ([]string, error) {
		if members[0].GoalID == "goal-c" {
			return nil, &assemblyConflict{GoalID: "goal-c", Paths: []string{"internal/landing/batch/shared.go"},
				Cause: refuseBatch("BATCH_JOIN_CONFLICT", "unit goal-c paths internal/landing/batch/shared.go")}
		}
		return []string{"tip-d"}, nil
	})
	launches := 0
	bed.owner.launch = func(Dispatch) error { launches++; return nil }
	bed.tree = "base-2"
	must(t, bed.owner.Tick(testBatchID))
	record := load(t, bed.owner.store)
	want := "CONFLICT with what landed in batch " + dispatchB + " (files internal/landing/batch/shared.go). Rebase goal/goal-c on main, then metasystem work land goal-c."
	witness(t, record.State == StateOpen && record.BaseTree == "base-2" && record.TipTree == "tip-d" && launches == 0,
		"survivors did not reopen on base-2: %+v", record)
	witness(t, record.Units[0].State == UnitReturnPending && record.Units[0].Outcome == UnitEjected && record.Units[0].Failure == want &&
		record.Units[1].State == UnitJoined, "conflicting member: %+v", record.Units)
}
