package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

const ownerHelmA, ownerHelmC = "01j5x00000000000000000aa01", "01j5x00000000000000000cc01"

type ownerHelmProber struct {
	mu   sync.Mutex
	dead map[int64]bool
}

func (p *ownerHelmProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.dead[pid] {
		return identity.Exact{Pid: pid}, identity.Dead, nil
	}
	return identity.Exact{Pid: pid, StartedAt: time.Unix(pid, 0)}, identity.Alive, nil
}

// ownerHelmBed is a landing checkout L (a directory with .git) holding batches
// A (seat A) and C (seat C), queued by one owner pid (7) on a fixed clock
// behind a busy foreign proof lock (pid 4242).
type ownerHelmBed struct {
	landing, seatA, seatC, lockDir, queueDir string
	prober                                   *ownerHelmProber
	owner                                    *batch.Owner
	launched                                 []string
	resumes, cadences                        int
	out                                      bytes.Buffer
	now                                      time.Time
}

func ownerHelmCheckout(t *testing.T) string {
	root := filepath.Join(t.TempDir(), "checkout")
	for _, dir := range []string{".git", "metasystem"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func newOwnerHelmBed(t *testing.T) *ownerHelmBed {
	t.Helper()
	bed := &ownerHelmBed{landing: ownerHelmCheckout(t), seatA: filepath.Join(ownerHelmCheckout(t), "metasystem"),
		seatC: filepath.Join(ownerHelmCheckout(t), "metasystem"), prober: &ownerHelmProber{dead: map[int64]bool{}},
		now: time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)}
	bed.lockDir, bed.queueDir = filepath.Join(bed.landing, "proof-lock"), filepath.Join(bed.landing, "proof-queue")
	store := batch.NewStore(bed.landing, bed.prober)
	joined := bed.now.Add(-time.Minute).Format(time.RFC3339Nano)
	for id, seat := range map[string]string{ownerHelmA: bed.seatA, ownerHelmC: bed.seatC} {
		record := batch.Record{Schema: 1, BatchID: id, State: batch.StateLanding,
			Units:   []batch.Unit{{GoalID: "goal-" + id[24:], Chain: "chain", SeatRoot: seat, State: batch.UnitJoined, Claim: batch.Claim{Machine: "landing", Lineage: "owner", Epoch: 1, Revision: 2, AccountingRevision: 1}}},
			History: []batch.HistoryEntry{{At: joined, Verb: "join", Detail: "goal-" + id[24:] + " joined"}}}
		if err := store.Create(record); err != nil {
			t.Fatal(err)
		}
	}
	settings, err := config.NewBatchLanding(bed.landing, time.Minute, func() time.Time { return bed.now })
	if err != nil {
		t.Fatal(err)
	}
	clock := func() time.Time { return bed.now }
	bed.owner, err = batch.NewOwner(batch.OwnerOptions{Store: store, Settings: settings, Actor: "landing+owner", PID: 7,
		LockDir: bed.lockDir, QueueDir: bed.queueDir, Now: clock, FetchTree: func() (string, error) { return "tree", nil },
		ReadClaim: func(string, string, string, string) (batch.Claim, error) { return batch.Claim{}, os.ErrNotExist },
		Returns:   batch.ReturnSeams{Read: func(string, string, string) (batch.ReturnLedgerGoal, error) { return batch.ReturnLedgerGoal{}, nil }},
		Rebind:    func(string, string) error { return nil }, Mint: func() (string, error) { return "opid", nil },
		LogRed: func(string, batch.TrunkRedRecordOutcome) {}, BaseCommit: func(tree string) (string, error) { return "commit-" + tree, nil },
		RunDiagnostic: func(string, batch.DiagnosticRequest, batch.Claim) (batch.DiagnosticResult, error) {
			return batch.DiagnosticResult{}, nil
		},
		DescendsFrom: func(string, string) (bool, error) { return true, nil },
		Sample:       func() proofrun.LoadSample { return proofrun.LoadSample{OverlapKnown: true} },
		Admission:    func(proofrun.LoadSample) proofrun.AdmissionCap { return proofrun.AdmissionCap{Max: 8} },
		Launch: func(id string, _ proofrun.LoadSample, _ string) error {
			bed.launched = append(bed.launched, id)
			return nil
		}, After: func(time.Duration) <-chan time.Time { return make(chan time.Time) }, Report: func(string, error) {},
		HelmActive: func(root string) bool { return helm.Active(root).Active }})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(bed.lockDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bed.lockDir, "owner"), []byte("m1e 4242 2030-01-01T00:00:00Z hand\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return bed
}

func (bed *ownerHelmBed) pass() {
	batchOwnerPassWith(bed.owner, bed.landing, batchOwnerPassSeams{helm: helm.Active,
		resume: func(owner *batch.Owner) { bed.resumes++; owner.Resume() }, cadence: func() { bed.cadences++ },
		out: &bed.out, now: func() time.Time { return bed.now }})
}

func (bed *ownerHelmBed) queue(t *testing.T) []string {
	entries, err := os.ReadDir(bed.queueDir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func (bed *ownerHelmBed) snapshot(t *testing.T) string {
	var parts []string
	for _, path := range []string{filepath.Join(bed.lockDir, "owner"),
		filepath.Join(bed.landing, "artifacts", "agents", "landing-batches", ownerHelmA+".json"),
		filepath.Join(bed.landing, "artifacts", "agents", "landing-batches", ownerHelmC+".json")} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		parts = append(parts, string(data))
	}
	return strings.Join(parts, "\x00")
}

func (bed *ownerHelmBed) yields(t *testing.T) string {
	data, err := os.ReadFile(filepath.Join(bed.landing, ".git", "metasystem", "helm-yields.log"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(data)
}

func ownerHelmTake(t *testing.T, root string) {
	if _, err := helm.Write(root, helm.Record{By: "Wido", At: "2030-01-02T03:04:05Z", Reason: "by hand"}); err != nil {
		t.Fatal(err)
	}
}

func ownerHelmReturn(t *testing.T, root string) {
	if _, err := helm.Remove(root); err != nil {
		t.Fatal(err)
	}
}

func TestLandingSeatHelmWithdrawsEveryRegistration(t *testing.T) {
	t.Parallel()
	t.Run("HM-4 HM-7", func(t *testing.T) {
		bed := newOwnerHelmBed(t)
		bed.pass()
		if queued := bed.queue(t); len(queued) != 2 || bed.resumes != 1 || bed.cadences != 1 {
			t.Fatalf("setup queue=%v resumes=%d cadences=%d, want two registrations", queued, bed.resumes, bed.cadences)
		}
		before := bed.snapshot(t)
		ownerHelmTake(t, bed.landing)
		bed.pass()
		bed.pass()
		if bed.resumes != 1 || bed.cadences != 1 {
			t.Fatalf("under the landing seat's helm resumes=%d cadences=%d, want neither called", bed.resumes, bed.cadences)
		}
		if queued := bed.queue(t); len(queued) != 0 {
			t.Fatalf("queue=%v under the landing seat's helm, want every registration withdrawn", queued)
		}
		if bed.snapshot(t) != before {
			t.Fatalf("the foreign lock or a batch record changed under the helm")
		}
		if lines := strings.Count(bed.out.String(), "helm:"); lines != 1 || strings.Count(bed.yields(t), "\n") != 1 {
			t.Fatalf("helm lines=%d yields=%q, want one of each per hold", lines, bed.yields(t))
		}
		ownerHelmReturn(t, bed.landing)
		bed.pass()
		if queued := bed.queue(t); len(queued) != 2 || queued[0] == queued[1] || bed.resumes != 2 || bed.cadences != 2 {
			t.Fatalf("after return queue=%v resumes=%d cadences=%d, want both registered again", queued, bed.resumes, bed.cadences)
		}
		bed.prober.mu.Lock()
		bed.prober.dead[4242] = true
		bed.prober.mu.Unlock()
		bed.pass()
		if strings.Join(bed.launched, ",") != ownerHelmA+","+ownerHelmC {
			t.Fatalf("launched=%v, want A then C once the lock is free", bed.launched)
		}
	})
}

func TestCadenceRunsUnderOtherSeatHelm(t *testing.T) {
	t.Parallel()
	t.Run("HM-11 HM-12", func(t *testing.T) {
		bed := newOwnerHelmBed(t)
		bed.pass()
		ownerHelmTake(t, bed.seatA)
		bed.pass()
		bed.pass()
		if bed.cadences != 3 || bed.resumes != 3 {
			t.Fatalf("cadences=%d resumes=%d, want the cadence and the pass every time", bed.cadences, bed.resumes)
		}
		if lines := strings.Count(bed.out.String(), "helm:"); lines != 1 || !strings.Contains(bed.out.String(), ownerHelmA) {
			t.Fatalf("helm lines=%q, want one naming batch A", bed.out.String())
		}
		yields := bed.yields(t)
		if strings.Count(yields, "\n") != 1 || !strings.Contains(yields, ownerHelmA) || !strings.Contains(yields, bed.seatA) ||
			!strings.Contains(yields, "landing-batch-owner-"+ownerHelmA+"-7") || !strings.Contains(yields, `"by":"Wido"`) {
			t.Fatalf("landing seat yields=%q, want one record naming the batch, the seat, the entry and the holder", yields)
		}
	})
}
