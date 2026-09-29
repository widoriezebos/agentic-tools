package proofrun

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

var retentionNow = time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)

type retentionBed struct {
	t       *testing.T
	control string
	alive   map[int64]bool
}

func newRetentionBed(t *testing.T) *retentionBed {
	t.Helper()
	control, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &retentionBed{t: t, control: control, alive: map[int64]bool{}}
}

func (b *retentionBed) path(parts ...string) string {
	return filepath.Join(append([]string{b.control, "artifacts", "agents", "proof-runs"}, parts...)...)
}

func (b *retentionBed) write(path string, value any) {
	b.t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		b.t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		b.t.Fatal(err)
	}
}

// attempt writes one attempt that ended age ago (nil terminal when live),
// its process record with a suite pid, and its retained proof directory.
func (b *retentionBed) attempt(id string, age time.Duration, terminal bool, mutate func(*Attempt)) Attempt {
	b.t.Helper()
	attempt := Attempt{SchemaVersion: 2, AttemptID: id, StartedAt: retentionNow.Add(-age - time.Hour).Format(time.RFC3339Nano),
		ProcessKeys: []string{id + "-launch-1"}}
	if terminal {
		attempt.EndedAt = retentionNow.Add(-age).Format(time.RFC3339Nano)
		attempt.Terminal = &AttemptTerminal{Result: "passed", At: attempt.EndedAt}
	}
	if mutate != nil {
		mutate(&attempt)
	}
	b.write(b.path("attempts", id+".json"), attempt)
	pid := int64(len(id)*1000 + int(id[len(id)-1]))
	b.write(b.path("processes", id+"-launch-1.json"), Record{Suite: "testing", AttemptID: id, LaunchID: "launch-1",
		SuiteProcess: ProcessIdentity{Pid: pid, PidStartedAt: 1}})
	if err := os.MkdirAll(b.path(id, "testing", "plan"), 0o700); err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(b.path(id, "testing", "plan", "log"), []byte(strings.Repeat("x", 8192)), 0o600); err != nil {
		b.t.Fatal(err)
	}
	return attempt
}

func (b *retentionBed) pid(id string) int64 { return int64(len(id)*1000 + int(id[len(id)-1])) }

func (b *retentionBed) exists(id string) bool {
	_, err := os.Stat(b.path("attempts", id+".json"))
	_, dirErr := os.Stat(b.path(id))
	_, processErr := os.Stat(b.path("processes", id+"-launch-1.json"))
	if (err == nil) != (dirErr == nil) || (err == nil) != (processErr == nil) {
		b.t.Fatalf("attempt %s is half removed: record %v, dir %v, process %v", id, err, dirErr, processErr)
	}
	return err == nil
}

type fakeNamer struct {
	kind  string
	named []string
	err   error
}

func (n fakeNamer) Kind() string { return n.kind }
func (n fakeNamer) Named(context.Context, time.Time) ([]string, error) {
	return n.named, n.err
}

func (b *retentionBed) retention(target int64, namers ...AttemptNamer) *Retention {
	return &Retention{Control: b.control, Target: target, Keep: 14 * 24 * time.Hour, Namers: namers,
		Alive: func(ref identity.Ref) identity.Liveness {
			if b.alive[ref.Pid] {
				return identity.Alive
			}
			return identity.Dead
		}}
}

func (b *retentionBed) pass(class diskstore.Class) diskstore.Report {
	b.t.Helper()
	report, err := diskstore.RunPass(context.Background(), diskstore.PassOptions{Kind: "checkout", Name: b.control,
		Registry: diskstore.Registry{Dir: filepath.Join(b.control, "stores")}, Mode: diskstore.ModeApply, Now: retentionNow,
		Clock: func() time.Time { return retentionNow }, Classes: []diskstore.Class{class}})
	if err != nil {
		b.t.Fatal(err)
	}
	return report
}

const retentionDay = 24 * time.Hour

// TestAttemptRetentionKeepsEveryRootAndRemovesTheOldestRest is U5b's
// witness (3.5 "Proof records and retained proof"): over
// disk.proof-target-gib the oldest terminal attempts past
// disk.proof-keep-days go, each with its process records and its retained
// proof, until the store is under its target; a live attempt, a young
// one, one whose freshness has not expired, one a retained attempt reuses
// (transitively), one a record kind names, and one whose recorded process
// is alive are kept; a repeat under the target removes nothing.
func TestAttemptRetentionKeepsEveryRootAndRemovesTheOldestRest(t *testing.T) {
	t.Parallel()
	b := newRetentionBed(t)
	b.attempt("proof-a-0000000000000001", 60*retentionDay, true, nil)
	b.attempt("proof-a-0000000000000002", 50*retentionDay, true, nil)
	b.attempt("proof-a-0000000000000003", 40*retentionDay, true, nil)
	b.attempt("proof-live-0000000000000004", 90*retentionDay, false, nil)
	b.attempt("proof-young-0000000000000005", 2*retentionDay, true, func(a *Attempt) {
		a.TestResult = &TestResult{Groups: []GroupResult{{ID: "g", ReuseAttempt: "proof-reused-0000000000000006"}}}
	})
	b.attempt("proof-reused-0000000000000006", 80*retentionDay, true, func(a *Attempt) { a.PreviousAttempt = "proof-deep-0000000000000007" })
	b.attempt("proof-deep-0000000000000007", 85*retentionDay, true, nil)
	b.attempt("proof-fresh-0000000000000008", 70*retentionDay, true, func(a *Attempt) {
		a.FreshnessExpiresAt = retentionNow.Add(retentionDay).Format(time.RFC3339Nano)
	})
	b.attempt("proof-named-0000000000000009", 75*retentionDay, true, nil)
	b.attempt("proof-alive-0000000000000010", 65*retentionDay, true, nil)
	b.alive[b.pid("proof-alive-0000000000000010")] = true

	var total, one int64
	entries, _ := os.ReadDir(b.path())
	for _, entry := range entries {
		bytes, _, _ := diskstore.Measure(context.Background(), b.path(entry.Name()))
		total += bytes
	}
	one = attemptBytes(b.control, "proof-a-0000000000000003", []string{b.path("processes", "proof-a-0000000000000003-launch-1.json")})
	report := b.pass(b.retention(total-2*one, fakeNamer{kind: "fixture", named: []string{"proof-named-0000000000000009"}}))
	for id, want := range map[string]bool{
		"proof-a-0000000000000001": false, "proof-a-0000000000000002": false, "proof-a-0000000000000003": true,
		"proof-live-0000000000000004": true, "proof-young-0000000000000005": true, "proof-reused-0000000000000006": true,
		"proof-deep-0000000000000007": true, "proof-fresh-0000000000000008": true, "proof-named-0000000000000009": true,
		"proof-alive-0000000000000010": true,
	} {
		if got := b.exists(id); got != want {
			t.Errorf("attempt %s present = %v, want %v", id, got, want)
		}
	}
	var keptAlive bool
	for _, line := range report.Kept {
		keptAlive = keptAlive || strings.Contains(line.Path, "proof-alive") && strings.Contains(line.Reason, "alive")
	}
	if !keptAlive {
		t.Errorf("an attempt with a live recorded process is kept naming it: %+v", report.Kept)
	}
	if again := b.pass(b.retention(total - 2*one)); len(again.Actions) != 0 {
		t.Errorf("a repeat under the target removes nothing: %+v", again.Actions)
	}
}

// A record kind that cannot be read stops the class for the pass: which
// attempts it names is unknown, so nothing is removed.
func TestAttemptRetentionStopsOnAnUnreadableRecordKind(t *testing.T) {
	t.Parallel()
	b := newRetentionBed(t)
	b.attempt("proof-a-0000000000000001", 60*retentionDay, true, nil)
	report := b.pass(b.retention(1, fakeNamer{kind: "fixture", err: errors.New("ledger unreadable")}))
	if !b.exists("proof-a-0000000000000001") || len(report.Pending) != 1 || !strings.Contains(report.Pending[0].Reason, "fixture") {
		t.Fatalf("an unreadable kind stops the class: %+v", report)
	}
}

// The store's mutation lock held by a proof run makes the removal pending;
// an attempt written after the plan that reuses a candidate keeps it.
func TestAttemptRetentionRechecksUnderTheMutationLock(t *testing.T) {
	t.Parallel()
	b := newRetentionBed(t)
	b.attempt("proof-a-0000000000000001", 60*retentionDay, true, nil)
	held, err := AcquireMutation(b.control)
	if err != nil {
		t.Fatal(err)
	}
	report := b.pass(b.retention(1))
	if !b.exists("proof-a-0000000000000001") || len(report.Pending) != 1 {
		t.Fatalf("a held mutation lock is pending: %+v", report)
	}
	held.Release()

	class := b.retention(1)
	pass := &diskstore.Pass{Now: retentionNow, Mode: diskstore.ModeApply}
	items, err := class.Plan(context.Background(), pass)
	if err != nil || len(items) != 1 || items[0].Verdict.Decision != diskstore.Release {
		t.Fatalf("plan: %+v %v", items, err)
	}
	b.attempt("proof-new-0000000000000002", 0, false, func(a *Attempt) {
		a.TestResult = &TestResult{Groups: []GroupResult{{ID: "g", ReuseAttempt: "proof-a-0000000000000001"}}}
	})
	future := retentionNow.Add(time.Minute)
	if err := os.Chtimes(b.path("attempts", "proof-new-0000000000000002.json"), future, future); err != nil {
		t.Fatal(err)
	}
	if verdict := class.Apply(context.Background(), pass, items[0]); verdict.Decision == diskstore.Release || !b.exists("proof-a-0000000000000001") {
		t.Fatalf("an attempt reused since the plan is kept: %+v", verdict)
	}
}

// A removal cut short (its retained proof partly gone) is finished by the
// next pass: the attempt record goes last, so the attempt is found again.
func TestAttemptRetentionFinishesAnInterruptedRemoval(t *testing.T) {
	t.Parallel()
	b := newRetentionBed(t)
	b.attempt("proof-a-0000000000000001", 60*retentionDay, true, nil)
	if err := os.RemoveAll(b.path("proof-a-0000000000000001", "testing")); err != nil {
		t.Fatal(err)
	}
	b.pass(b.retention(1))
	if _, err := os.Stat(b.path("attempts", "proof-a-0000000000000001.json")); !os.IsNotExist(err) {
		t.Fatalf("the interrupted removal is finished: %v", err)
	}
}

// The scratch records' kind names the attempt every present scratch record
// names, and an unreadable scratch record is an error.
func TestScratchNamerReadsEveryScratchRecord(t *testing.T) {
	t.Parallel()
	b := newRetentionBed(t)
	b.write(filepath.Join(ScratchStore(b.control), "scratch-a.json"), ScratchRecord{Schema: scratchSchema, Run: "scratch-a", Attempt: "proof-a-0000000000000001"})
	b.write(filepath.Join(ScratchStore(b.control), "scratch-b.json"), ScratchRecord{Schema: scratchSchema, Run: "scratch-b"})
	named, err := ScratchNamer{Control: b.control}.Named(context.Background(), retentionNow)
	if err != nil || len(named) != 1 || named[0] != "proof-a-0000000000000001" {
		t.Fatalf("named = %v %v", named, err)
	}
	if err := os.WriteFile(filepath.Join(ScratchStore(b.control), "scratch-c.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (ScratchNamer{Control: b.control}).Named(context.Background(), retentionNow); err == nil {
		t.Fatal("an unreadable scratch record is an error")
	}
}
