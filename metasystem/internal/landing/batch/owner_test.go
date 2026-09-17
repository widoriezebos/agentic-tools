package batch

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
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
	must(t, exec.Command("git", "init", "-q", root).Run())
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
	bed.owner = &Owner{store: bed.store, settings: settings, actor: "landing+owner", ownerSeams: ownerSeams{now: func() time.Time { return bed.now }, fetchTree: func() (string, error) { return bed.tree, bed.fetchErr }, readClaim: readClaim, returns: returns, rebind: func(id string) error {
		bed.ticks = append(bed.ticks, id)
		bed.events = append(bed.events, "rebind:"+id)
		return nil
	}, sample: func() proofrun.LoadSample {
		if len(bed.samples) == 0 {
			return bed.sample
		}
		sample := bed.samples[0]
		bed.samples = bed.samples[1:]
		return sample
	}, admission: func(proofrun.LoadSample) proofrun.AdmissionCap { return proofrun.AdmissionCap{Max: 8} }, launch: func(id string, sample proofrun.LoadSample, window string) error {
		bed.launches++
		bed.launched, bed.window = sample, window
		return nil
	}, lock: func(id string) *proofLock {
		return newProofLock(bed.store, bed.lockDir, bed.queueDir, "batch:"+id, 7, func() time.Time { return bed.now })
	}, after: func(time.Duration) <-chan time.Time { return make(chan time.Time) }, report: func(string, error) {}, locks: map[string]*proofLock{}}}
	return bed
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
	witness(t, record.Units[1].State == UnitJoined && slices.Equal(bed.events, []string{"return:A", "rebind:" + testBatchID, "reconcile:B", "return:B", "rebind:" + testBatchID}), "record=%+v events=%v", record, bed.events)
}

func TestBatchOwnerReturnsAfterReconcile(t *testing.T) {
	record := ownerRecord(testBatchID, StateOpen, time.Time{})
	record.Units = []Unit{{GoalID: "goal-a", Chain: "chain-a", Claim: Claim{Machine: "landing", Lineage: "owner", Epoch: 1, Revision: 2, AccountingRevision: 1}, State: UnitJoining}}
	bed := newOwnerBed(t, record, time.Unix(3, 0))
	must(t, bed.owner.Tick(testBatchID))
	unit := load(t, bed.store).Units[0]
	witness(t, unit.State == UnitEjected && unit.ReturnDisposition == ReturnAlreadyReturned && slices.Equal(bed.events, []string{"reconcile:tree", "return:tree", "rebind:" + testBatchID}), "unit=%+v events=%v", unit, bed.events)
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
