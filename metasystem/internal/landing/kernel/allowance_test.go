package kernel

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// TestGreenWorkPublishesAfterAllowance (K10, R8-08, rehearsal step 5): a
// batch has an allowance of four executions of any subject. A batch that
// spent all four on green proofs still publishes. The next execution is
// refused (no child starts, no attempt is recorded) and owes a person's
// alert, but the lane is not paused: publishing the green work and the
// agent's returns are still admitted after the refusal.
func TestGreenWorkPublishesAfterAllowance(t *testing.T) {
	t.Parallel()
	bed := newKernelBed(t)
	if _, err := bed.begin(); err != nil {
		t.Fatal(err)
	}
	var last batch.ProofAttempt
	for index := 0; index < lane.AllowanceExecutions; index++ {
		executable, _ := bed.fakeChild(fmt.Sprint("green-", index), passed("feature-standard"), verbresult.Result{Outcome: verbresult.Confirmed, Summary: "passed"}, 0)
		attempt, err := bed.prove(batch.SubjectBatch, executable)
		if err != nil || attempt.Status != batch.AttemptGreen {
			t.Fatalf("execution %d: %+v %v", index+1, attempt, err)
		}
		last = attempt
	}
	if err := lane.Gate(bed.home, lane.OpPublish, lane.AuthorityAgent, nil); err != nil {
		t.Fatalf("the spent allowance forbade publishing green work: %v", err)
	}
	proof, err := (PublishEvidence{Layout: bed.layout}).Proof(bed.batchID)
	if err != nil || proof.Attempt != last.ID || proof.Outcome != lane.OutcomeGreen {
		t.Fatalf("the proof publish reads = %+v %v; want the green fourth %s", proof, err, last.ID)
	}

	executable, argvFile := bed.fakeChild("fifth", passed("feature-standard"), verbresult.Result{Outcome: verbresult.Confirmed, Summary: "passed"}, 0)
	_, err = bed.prove(batch.SubjectMember+":"+bed.changeID, executable)
	var refusal *lane.Refusal
	if !errors.As(err, &refusal) || refusal.Code != lane.CodeAllowanceSpent {
		t.Fatalf("the fifth execution = %v; want the allowance's refusal", err)
	}
	if _, err := os.Stat(argvFile); !os.IsNotExist(err) {
		t.Fatal("the refused fifth execution started its child")
	}
	if record, _ := bed.store().Load(bed.batchID); len(record.Attempts) != lane.AllowanceExecutions {
		t.Fatalf("attempts recorded = %d; want the four", len(record.Attempts))
	}
	if _, paused := lane.ReadPause(bed.home); paused {
		t.Fatal("the spent allowance paused the lane")
	}
	if err := lane.Gate(bed.home, lane.OpPublish, lane.AuthorityAgent, nil); err != nil {
		t.Fatalf("after the refused execution, publishing the green work = %v; want it admitted", err)
	}
	if err := lane.Gate(bed.home, lane.OpReturn, lane.AuthorityAgent, nil); err != nil {
		t.Fatalf("after the refused execution, the agent's return = %v; want it admitted", err)
	}
	if proof, err := (PublishEvidence{Layout: bed.layout}).Proof(bed.batchID); err != nil || proof.Attempt != last.ID {
		t.Fatalf("after the refused execution the proof publish reads = %+v %v; want the green fourth", proof, err)
	}
	if hits, err := lane.PendingHits(bed.home); err != nil || len(hits) != 1 || hits[0].Kind != lane.HitAllowance {
		t.Fatalf("owed alerts = %+v %v; want the allowance's", hits, err)
	}
}
