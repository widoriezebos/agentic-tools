package batch

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
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
	root, seat := t.TempDir(), t.TempDir()
	initCommand := exec.Command("git", "init", "-q", root)
	initCommand.Env = gittree.ScrubbedEnviron()
	must(t, initCommand.Run())
	conf := filepath.Join(seat, "metasystem.conf")
	must(t, os.WriteFile(conf, []byte(config.BatchRootKey+"="+root+"\n"+config.BatchMaxWaitKey+"=1m\n"), 0o644))
	bed := &ownerBed{now: now, tree: "tree", sample: proofrun.LoadSample{OverlapKnown: true}, lockDir: filepath.Join(root, "owner-lock"), queueDir: filepath.Join(root, "owner-queue")}
	bed.store = NewStore(root, scriptedProber{7: identity.Alive})
	must(t, bed.store.Create(record))
	settings, err := config.ResolveBatchLanding(conf, seat, func() time.Time { return bed.now })
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
		}, Admission: func(proofrun.LoadSample) proofrun.AdmissionCap { return proofrun.AdmissionCap{Max: 8} }, Launch: func(id string, sample proofrun.LoadSample, window string) error {
			bed.launches++
			bed.launched, bed.window = sample, window
			return nil
		}, After: func(time.Duration) <-chan time.Time { return make(chan time.Time) }, Report: func(string, error) {}})
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
			if _, err := os.Stat(bed.lockDir); !errors.Is(err, os.ErrNotExist) {
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
	assembly, store, ledger := heldReopenBed(t)
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
	bed.owner.launch = func(string, proofrun.LoadSample, string) error {
		bed.launches++
		return bed.store.Update(testBatchID, func(record *Record) error { record.State = StateLanded; return nil })
	}
	var reports []error
	bed.owner.report = func(_ string, errorValue error) { reports = append(reports, errorValue) }
	must(t, bed.owner.Tick(testBatchID))
	if record := load(t, bed.store); record.State != StateLanded || len(ledger.cleared) != 0 || len(reports) != 1 {
		t.Fatalf("first pass record=%+v clears=%+v reports=%v", record, ledger.cleared, reports)
	}
	bed.owner.Resume()
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
			bed.owner.launch = func(string, proofrun.LoadSample, string) error {
				bed.launches++
				return bed.store.Update(testBatchID, func(record *Record) error { record.State = StateLanded; return nil })
			}
			must(t, bed.owner.Tick(testBatchID))
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
	must(t, bed.owner.Tick(testBatchID))
	_, lockErr := os.Stat(bed.lockDir)
	_, queueErr := os.Stat(bed.queueDir)
	witness(t, bed.launches == 0 && os.IsNotExist(lockErr) && os.IsNotExist(queueErr), "before deadline launches=%d lock=%v queue=%v", bed.launches, lockErr, queueErr)
	bed.now = joined.Add(time.Minute)
	bed.samples = []proofrun.LoadSample{{OverlapKnown: true}, {OverlapKnown: true, OverlappingLocal: 2}}
	must(t, bed.owner.Tick(testBatchID))
	witness(t, bed.launches == 1 && bed.window == "expired" && bed.launched.OverlappingLocal == 2, "deadline launch=%d/%s sample=%+v", bed.launches, bed.window, bed.launched)
}

func TestBatchOwnerWakesOnSignal(t *testing.T) {
	bed := newOwnerBed(t, ownerRecord(testBatchID, StateOpen, time.Time{}), time.Unix(1, 0))
	passes, waiting, release := make(chan struct{}, 2), make(chan struct{}, 1), make(chan struct{})
	bed.owner.fetchTree = func() (string, error) { passes <- struct{}{}; return "tree", nil }
	bed.owner.after = func(time.Duration) <-chan time.Time { waiting <- struct{}{}; <-release; return make(chan time.Time) }
	wake, stop, done := make(chan struct{}, 1), make(chan struct{}), make(chan struct{})
	go func() { bed.owner.Loop(time.Minute, wake, stop); close(done) }()
	<-passes
	<-waiting
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
