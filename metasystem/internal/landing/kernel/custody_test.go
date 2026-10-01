package kernel

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/custody"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// K9 (integration of K-b and K-f): every landing prove passes the lane's
// one custody barrier before anything is recorded or started, and its
// child is custodied from the moment it starts: a custody record of kind
// prove, opened before the start, bound to the child's exact identity and
// its own process group, settled once the child is gone. Live landing work
// holds the next prove; nothing is recorded and no child runs.
func TestProveIsCustodiedAndWaitsForLiveCustody(t *testing.T) {
	t.Parallel()
	bed := newKernelBed(t)
	if _, err := bed.begin(); err != nil {
		t.Fatal(err)
	}
	executable, _ := bed.fakeChild("batch", passed("feature-standard"), verbresult.Result{Outcome: verbresult.Confirmed, Summary: "passed"}, 0)
	attempt, err := bed.prove(batch.SubjectBatch, executable)
	if err != nil || attempt.Status != batch.AttemptGreen {
		t.Fatalf("prove: %+v %v", attempt, err)
	}
	records, unreadable, err := custody.Records(bed.home)
	if err != nil || len(unreadable) != 0 || len(records) != 1 {
		t.Fatalf("custody records = %+v %v %v; want the prove's one", records, unreadable, err)
	}
	record := records[0]
	if record.Kind != custody.KindProve || record.Child == "" || len(record.Groups) == 0 || !strings.Contains(record.Subject, attempt.ID) {
		t.Fatalf("the prove's custody = %+v; want kind prove naming attempt %s, bound to its child and group", record, attempt.ID)
	}
	if err := custody.Clear(bed.home, custody.Probes{}, false); err != nil {
		t.Fatalf("custody after the child ended: %v", err)
	}

	// Live landing work: an in-process execution of this test's own process.
	live, err := custody.Open(bed.home, custody.KindVerify, "a retained verification", kernelAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := custody.BindSelf(bed.home, live.ID); err != nil {
		t.Fatal(err)
	}
	before, _ := bed.store().Load(bed.batchID)
	executable, argvFile := bed.fakeChild("held", passed("feature-standard"), verbresult.Result{Outcome: verbresult.Confirmed, Summary: "passed"}, 0)
	_, err = bed.prove(batch.SubjectBatch, executable)
	var held *custody.Held
	if !errors.As(err, &held) || len(held.Live) == 0 {
		t.Fatalf("prove while landing work runs = %v; want it held by the live custody", err)
	}
	if after, _ := bed.store().Load(bed.batchID); len(after.Attempts) != len(before.Attempts) {
		t.Fatalf("a held prove recorded an attempt: %+v", after.Attempts)
	}
	if _, err := os.Stat(argvFile); !os.IsNotExist(err) {
		t.Fatalf("a held prove started its child: %v", err)
	}
	if err := custody.End(bed.home, live.ID); err != nil {
		t.Fatal(err)
	}
	if attempt, err := bed.prove(batch.SubjectBatch, executable); err != nil || attempt.Status != batch.AttemptGreen {
		t.Fatalf("prove once the work ended: %+v %v", attempt, err)
	}
}
