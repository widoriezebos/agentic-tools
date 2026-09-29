package proofrun

// Witnesses of the third B3 read (the reader's probes that still apply
// after the Round B3-3 scope cut; each but the last failed on 8ddaa3618).

import (
	"os"
	"path/filepath"
	"testing"
)

// R3-P1b (P1 / R3 variant): a record of a KNOWN schema written by a newer
// engine that added a reference field without a schema bump. The unknown
// field is dropped by json.Unmarshal, so its reference is never seen.
func TestB3Read3UnknownReferenceFieldDropped(t *testing.T) {
	t.Parallel()
	b := newRetentionBed(t)
	b.attempt("proof-old-0000000000000001", 60*retentionDay, true, nil)
	b.attempt("proof-new-0000000000000002", 1*retentionDay, true, nil)
	path := b.path("attempts", "proof-new-0000000000000002.json")
	data, _ := os.ReadFile(path)
	data = append([]byte(`{"replayOfAttempt":"proof-old-0000000000000001",`), data[1:]...)
	os.WriteFile(path, data, 0o600)
	report := b.pass(b.retention(1))
	t.Logf("pending=%+v actions=%+v", report.Pending, report.Actions)
	if !b.payload("proof-old-0000000000000001") {
		t.Fatalf("payload named by a newer engine's field (known schema, unknown field) was released")
	}
}

// R3-U5b-a (rule 2/3): an attempt whose id is a structure directory of the
// proof-run store. Its "payload" is that directory: every attempt record
// (accounting) under proof-runs/attempts is deleted.
func TestB3Read3AttemptIDNamingAStructureDirectory(t *testing.T) {
	t.Parallel()
	b := newRetentionBed(t)
	b.attempt("proof-keep-0000000000000003", 1*retentionDay, true, nil)
	b.attempt("attempts", 60*retentionDay, true, nil)
	// attempt() made proof-runs/attempts/testing/plan/log as its "payload".
	report := b.pass(b.retention(1))
	t.Logf("pending=%+v actions=%+v", report.Pending, report.Actions)
	if _, err := os.Stat(b.path("attempts", "proof-keep-0000000000000003.json")); err != nil {
		t.Fatalf("attempt records (accounting) deleted as the payload of attempt id \"attempts\": %v", err)
	}
}

// R3-U5b-b: same, the landing batch proof results directory.
func TestB3Read3AttemptIDBatchDeletesBatchResults(t *testing.T) {
	t.Parallel()
	b := newRetentionBed(t)
	b.write(b.path("batch", "batch-1.json"), map[string]string{"result": "passed"})
	b.attempt("batch", 60*retentionDay, true, nil)
	report := b.pass(b.retention(1))
	t.Logf("pending=%+v actions=%+v", report.Pending, report.Actions)
	if _, err := os.Stat(b.path("batch", "batch-1.json")); err != nil {
		t.Fatalf("landing batch proof result deleted as the payload of attempt id \"batch\": %v", err)
	}
}

// R3-N5 variant: the processes directory cannot be listed. Glob swallows
// the error, so the attempt has no process records and its live suite pid
// is never probed.
func TestB3Read3UnreadableProcessesDirectory(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root reads everything")
	}
	b := newRetentionBed(t)
	b.attempt("proof-old-0000000000000001", 60*retentionDay, true, nil)
	b.alive[b.pid("proof-old-0000000000000001")] = true
	dir := b.path("processes")
	os.Chmod(dir, 0o000)
	defer os.Chmod(dir, 0o700)
	report := b.pass(b.retention(1))
	t.Logf("pending=%+v actions=%+v", report.Pending, report.Actions)
	if !b.payload("proof-old-0000000000000001") {
		t.Fatalf("payload of an attempt whose (unreadable) process record names a live pid was released")
	}
}

// R3-rule2: a payload directory that is a symlink: ReadDir follows it and
// the entries of the link's target (outside the store) are removed.
func TestB3Read3SymlinkedPayloadDirectory(t *testing.T) {
	t.Parallel()
	b := newRetentionBed(t)
	b.attempt("proof-old-0000000000000001", 60*retentionDay, true, nil)
	outside := filepath.Join(t.TempDir(), "outside")
	os.MkdirAll(outside, 0o700)
	os.WriteFile(filepath.Join(outside, "precious.txt"), []byte("not the store's"), 0o600)
	os.RemoveAll(b.path("proof-old-0000000000000001"))
	os.Symlink(outside, b.path("proof-old-0000000000000001"))
	// give it measurable size through the link target
	os.WriteFile(filepath.Join(outside, "big"), make([]byte, 1<<16), 0o600)
	report := b.pass(b.retention(1))
	t.Logf("pending=%+v actions=%+v", report.Pending, report.Actions)
	if _, err := os.Stat(filepath.Join(outside, "precious.txt")); err != nil {
		t.Fatalf("content outside the store removed through a symlinked payload directory")
	}
}

// R3-P1a (P1 variant): an attempt record of a FUTURE schema that reuses an
// old attempt. The class must hold.
func TestB3Read3FutureSchemaHolds(t *testing.T) {
	t.Parallel()
	b := newRetentionBed(t)
	b.attempt("proof-old-0000000000000001", 60*retentionDay, true, nil)
	b.attempt("proof-new-0000000000000002", 1*retentionDay, true, func(a *Attempt) {
		a.SchemaVersion = 99
		a.TestResult = &TestResult{Groups: []GroupResult{{ID: "g", ReuseAttempt: "proof-old-0000000000000001"}}}
	})
	report := b.pass(b.retention(1))
	t.Logf("pending=%+v actions=%+v", report.Pending, report.Actions)
	if !b.payload("proof-old-0000000000000001") {
		t.Fatalf("payload reused by a future-schema attempt was released")
	}
}
