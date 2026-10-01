package batchowner

import (
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

var batchJoinAdmissionExecutable = os.Executable

// deferredJoinAdmission is a goal's join admission (design r10 K6): the
// join is custody handover plus enqueue and starts no test run. The member
// joins with its exact admission tree recorded; landing prove's member
// subject is where its tests run.
func deferredJoinAdmission(_ string, _ string, unit batch.Unit) (batch.JoinAdmission, error) {
	if unit.Admission == nil || unit.Admission.Tree == "" {
		return batch.JoinAdmission{}, fmt.Errorf("%s: %s has no exact admission tree", codeJoinTestDropped, unit.GoalID)
	}
	return batch.JoinAdmission{Tree: unit.Admission.Tree, Status: batch.AdmissionDeferred}, nil
}
