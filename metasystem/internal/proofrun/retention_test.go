package proofrun

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
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

// exists reports an attempt's records, which retention never removes.
func (b *retentionBed) exists(id string) bool {
	_, err := os.Stat(b.path("attempts", id+".json"))
	_, processErr := os.Stat(b.path("processes", id+"-launch-1.json"))
	if (err == nil) != (processErr == nil) {
		b.t.Fatalf("attempt %s is half removed: record %v, process %v", id, err, processErr)
	}
	return err == nil
}

// payload reports whether an attempt's retained proof is still there.
func (b *retentionBed) payload(id string) bool {
	_, err := os.Stat(b.path(id, "testing"))
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

// emptyCensusReader is a host with no other process: its census is
// complete and names no holder.
func emptyCensusReader() *diskstore.CensusReader {
	return &diskstore.CensusReader{Pids: func() ([]int64, error) { return nil, nil }}
}

func (b *retentionBed) pass(class diskstore.Class) diskstore.Report {
	b.t.Helper()
	return b.passWith(class, emptyCensusReader())
}

func (b *retentionBed) passWith(class diskstore.Class, reader *diskstore.CensusReader) diskstore.Report {
	b.t.Helper()
	report, err := diskstore.RunPass(context.Background(), diskstore.PassOptions{Kind: "checkout", Name: b.control,
		Registry: diskstore.Registry{Dir: filepath.Join(b.control, "stores")}, Mode: diskstore.ModeApply, Now: retentionNow,
		Clock: func() time.Time { return retentionNow }, Classes: []diskstore.Class{class}, CensusReader: reader})
	if err != nil {
		b.t.Fatal(err)
	}
	return report
}

const retentionDay = 24 * time.Hour

// TestAttemptRetentionKeepsEveryRootAndRemovesTheOldestRest is U5b's
// witness (3.5; Round B3-2 R1): over disk.proof-target-gib the payloads of
// the oldest terminal attempts past disk.proof-keep-days go until the store
// is under its target, each leaving its note, while every attempt record
// and process record stays; the payload of a live attempt, a young one,
// one whose freshness has not expired, one a retained attempt reuses
// (transitively), one a record kind names, and one whose recorded process
// is alive stays; a repeat under the target releases nothing.
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
	one = payloadBytes(context.Background(), b.path("proof-a-0000000000000003"))
	report := b.pass(b.retention(total-2*one, fakeNamer{kind: "fixture", named: []string{"proof-named-0000000000000009"}}))
	for id, want := range map[string]bool{
		"proof-a-0000000000000001": false, "proof-a-0000000000000002": false, "proof-a-0000000000000003": true,
		"proof-live-0000000000000004": true, "proof-young-0000000000000005": true, "proof-reused-0000000000000006": true,
		"proof-deep-0000000000000007": true, "proof-fresh-0000000000000008": true, "proof-named-0000000000000009": true,
		"proof-alive-0000000000000010": true,
	} {
		if !b.exists(id) {
			t.Errorf("attempt %s lost its records", id)
		}
		if got := b.payload(id); got != want {
			t.Errorf("attempt %s payload present = %v, want %v", id, got, want)
		}
	}
	if _, err := os.Stat(b.path("proof-a-0000000000000001", PayloadNote)); err != nil {
		t.Errorf("a released payload leaves its note: %v", err)
	}
	var keptAlive bool
	for _, line := range report.Kept {
		keptAlive = keptAlive || strings.Contains(line.Path, "proof-alive") && strings.Contains(line.Reason, "alive")
	}
	if !keptAlive {
		t.Errorf("an attempt with a live recorded process is kept naming it: %+v", report.Kept)
	}
	if again := b.pass(b.retention(total)); len(again.Actions) != 0 {
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
	if !b.payload("proof-a-0000000000000001") || len(report.Pending) != 1 || !strings.Contains(report.Pending[0].Reason, "fixture") {
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
	if !b.payload("proof-a-0000000000000001") || len(report.Pending) != 1 {
		t.Fatalf("a held mutation lock is pending: %+v", report)
	}
	held.Release()

	class := b.retention(1)
	pass := &diskstore.Pass{Now: retentionNow, Mode: diskstore.ModeApply}
	items, err := class.Plan(context.Background(), pass)
	if err != nil || len(items) == 0 || items[0].Verdict.Decision != diskstore.Release {
		t.Fatalf("plan: %+v %v", items, err)
	}
	b.attempt("proof-new-0000000000000002", 0, false, func(a *Attempt) {
		a.TestResult = &TestResult{Groups: []GroupResult{{ID: "g", ReuseAttempt: "proof-a-0000000000000001"}}}
	})
	future := retentionNow.Add(time.Minute)
	if err := os.Chtimes(b.path("attempts", "proof-new-0000000000000002.json"), future, future); err != nil {
		t.Fatal(err)
	}
	if verdict := class.Apply(context.Background(), pass, items[0]); verdict.Decision == diskstore.Release || !b.payload("proof-a-0000000000000001") {
		t.Fatalf("an attempt reused since the plan is kept: %+v", verdict)
	}
}

// A release cut short is finished by the next pass: the note is written
// first, and the attempt's records are never touched.
func TestAttemptRetentionFinishesAnInterruptedRelease(t *testing.T) {
	t.Parallel()
	b := newRetentionBed(t)
	b.attempt("proof-a-0000000000000001", 60*retentionDay, true, nil)
	if err := os.WriteFile(b.path("proof-a-0000000000000001", PayloadNote), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	b.pass(b.retention(1))
	if b.payload("proof-a-0000000000000001") || !b.exists("proof-a-0000000000000001") {
		t.Fatal("the interrupted release is finished and the records stay")
	}
}

// An attempt record of a schema this engine does not know holds the class.
func TestAttemptRetentionHoldsOnAnUnknownSchema(t *testing.T) {
	t.Parallel()
	b := newRetentionBed(t)
	b.attempt("proof-a-0000000000000001", 60*retentionDay, true, nil)
	b.attempt("proof-b-0000000000000002", 60*retentionDay, true, func(a *Attempt) { a.SchemaVersion = 9 })
	report := b.pass(b.retention(1))
	if !b.payload("proof-a-0000000000000001") || len(report.Pending) != 1 || !strings.Contains(report.Pending[0].Reason, "schema 9") {
		t.Fatalf("an unknown schema holds the class: %+v", report)
	}
}

// An unreadable process record counts as a live process (N5).
func TestAttemptRetentionReadsAnUnreadableProcessRecordAsAlive(t *testing.T) {
	t.Parallel()
	b := newRetentionBed(t)
	b.attempt("proof-a-0000000000000001", 60*retentionDay, true, nil)
	class := b.retention(1)
	unit := &attemptUnit{records: []string{b.path("processes", "missing.json")}}
	if reason := class.processesAlive(unit); !strings.Contains(reason, "counts as alive") {
		t.Fatalf("reason = %q", reason)
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

// An attempt filled by reflection (every string field named or tagged
// attempt or reuse given a fresh id) names all of them but itself through
// AttemptReferences, and a scratch record likewise through its kind
// (Round B3-2 R3).
func TestAttemptReferencesCoverEveryAttemptField(t *testing.T) {
	t.Parallel()
	fill := &attemptFill{}
	var attempt Attempt
	fill.fill(reflect.ValueOf(&attempt).Elem(), "Attempt", false, 0)
	for index, where := range fill.where {
		if where == "Attempt.AttemptID" {
			// The attempt's own id names itself, not another attempt.
			fill.ids = append(fill.ids[:index], fill.ids[index+1:]...)
			fill.where = append(fill.where[:index], fill.where[index+1:]...)
			break
		}
	}
	if len(fill.ids) < 4 {
		t.Fatalf("the fill reached only %d attempt fields", len(fill.ids))
	}
	if missing := fill.uncovered(AttemptReferences(attempt), attemptNotAttemptIDs); len(missing) > 0 {
		t.Fatalf("attempt fields AttemptReferences does not name:\n%s", strings.Join(missing, "\n"))
	}
	b := newRetentionBed(t)
	scratchFill := &attemptFill{}
	var record ScratchRecord
	scratchFill.fill(reflect.ValueOf(&record).Elem(), "ScratchRecord", false, 0)
	record.Schema = scratchSchema
	b.write(filepath.Join(ScratchStore(b.control), "scratch-x.json"), record)
	named, err := ScratchNamer{Control: b.control}.Named(context.Background(), retentionNow)
	if err != nil {
		t.Fatal(err)
	}
	if missing := scratchFill.uncovered(named, nil); len(missing) > 0 || len(scratchFill.ids) == 0 {
		t.Fatalf("scratch fields no reader names: %v", missing)
	}
}

// attemptNotAttemptIDs are the attempt record fields the detector's name
// rule matches that carry no attempt id, each with why.
var attemptNotAttemptIDs = map[string]string{
	"Attempt.Retry.PriorAttribution":              "the prior attempt's load attribution, a sentence",
	"Attempt.TestResult.EngineRearm.SourceCommit": "a git commit the engine was rebuilt from",
}

// The detector catches the two shapes the third read found it missed: an
// attempt id in a struct nested under an attempt-named slice, and a field
// named for a proof with no "attempt" in its name.
func TestAttemptFieldDetectorCatchesNestedAndProofNamedFields(t *testing.T) {
	t.Parallel()
	var record struct {
		ReplayAttempts []struct{ Source string } `json:"replayAttempts"`
		PriorProof     string                    `json:"priorProof"`
		Nested         struct {
			Deeper []struct{ ReuseFrom string }
		}
		Unrelated string
	}
	fill := &attemptFill{}
	fill.fill(reflect.ValueOf(&record).Elem(), "Variant", false, 0)
	missing := strings.Join(fill.uncovered(nil, nil), "\n")
	for _, want := range []string{"Variant.ReplayAttempts[].Source", "Variant.PriorProof", "Variant.Nested.Deeper[].ReuseFrom"} {
		if !strings.Contains(missing, want) {
			t.Errorf("the detector misses %s:\n%s", want, missing)
		}
	}
	if strings.Contains(missing, "Unrelated") {
		t.Errorf("the detector marks a field its rule does not name:\n%s", missing)
	}
}
