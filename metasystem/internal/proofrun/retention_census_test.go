package proofrun

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// selfCensusReader reads only this test process, with the kernel's own
// cwd, executable and open-file reader.
func selfCensusReader() *diskstore.CensusReader {
	self := int64(os.Getpid())
	return &diskstore.CensusReader{UID: uint32(os.Getuid()), Pids: func() ([]int64, error) { return []int64{self}, nil },
		ProcessUID: identity.ProcessUID, Use: identity.ReadProcessUse, Command: func(int64) string { return "go test" }}
}

// The third read's probe, kept: a payload a live process is writing into,
// though no attempt record names that process, is kept by the use census
// taken over the payload just before its removal, and retried.
func TestAttemptPayloadInUseByAnUnrecordedWriterIsKept(t *testing.T) {
	t.Parallel()
	b := newRetentionBed(t)
	b.attempt("proof-old-0000000000000001", 60*retentionDay, true, nil)
	file, err := os.OpenFile(b.path("proof-old-0000000000000001", "testing", "plan", "live.log"), os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.WriteString("still writing"); err != nil {
		t.Fatal(err)
	}
	report := b.passWith(b.retention(1), selfCensusReader())
	if _, err := os.Stat(file.Name()); err != nil || len(report.Pending) == 0 || !strings.Contains(report.Pending[0].Reason, "in use by pid") {
		t.Fatalf("a payload with an open writer is kept, pending: %v %+v", err, report)
	}
}

// A census that cannot complete (a live process of ours it cannot read)
// keeps every payload, pending; a census not taken at all does too.
func TestAttemptPayloadWaitsForACompleteCensus(t *testing.T) {
	t.Parallel()
	b := newRetentionBed(t)
	b.attempt("proof-old-0000000000000001", 60*retentionDay, true, nil)
	gap := &diskstore.CensusReader{UID: 1, Pids: func() ([]int64, error) { return []int64{7}, nil },
		ProcessUID: func(int64) (uint32, bool) { return 1, true },
		Use:        func(int64) (identity.ProcessUse, error) { return identity.ProcessUse{}, errors.New("denied") },
		Command:    func(int64) string { return "agent" }, Ours: func(int64) bool { return true },
		Parent: func(int64) (int64, bool) { return 1, true }}
	if report := b.passWith(b.retention(1), gap); !b.payload("proof-old-0000000000000001") || len(report.Pending) == 0 {
		t.Fatalf("an incomplete census keeps the payload: %+v", report)
	}
	if report := b.passWith(b.retention(1), nil); !b.payload("proof-old-0000000000000001") || len(report.Pending) == 0 {
		t.Fatalf("no census keeps the payload: %+v", report)
	}
	b.pass(b.retention(1))
	if b.payload("proof-old-0000000000000001") {
		t.Fatal("with a complete census and no holder the payload goes")
	}
}
