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
// spent all four on green proofs still publishes: the publication is
// admitted and its proof is the green fourth. Only the next execution is
// refused: no child starts and no attempt is recorded, and the lane stops
// for a person.
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
	if pause, paused := lane.ReadPause(bed.home); !paused || pause.By != lane.StopLossBy {
		t.Fatalf("after the refused execution: pause %+v %t; want the lane stopped by its stop-loss", pause, paused)
	}
}
