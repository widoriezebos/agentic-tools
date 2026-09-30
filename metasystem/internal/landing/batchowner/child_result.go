package batchowner

import (
	"errors"
	"os/exec"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// The landing owner's children answer with the --json envelope
// (internal/verbresult, design structured-output.md U1): the owner reads
// their outcome and code, never their words.
const (
	testRunVerb      = "internal test run"
	testPlanVerb     = "test plan"
	laneTestPlanVerb = "internal test plan"
)

// runTestRunChild runs an internal test run child, whose argv carries
// --json, and reads its envelope. The child's stream (its banner, the
// suite's output) arrives on stderr and is only quoted.
func runTestRunChild(command *exec.Cmd) (verbresult.Result, error) {
	return verbresult.Run(command, testRunVerb)
}

// admissionKind is how the owner handles a refused test run: the one
// classifier every launch shares (it was four disagreeing copies).
type admissionKind string

const (
	admissionRevision admissionKind = "revision" // the goal moved since the seal: eject the member
	admissionBudget   admissionKind = "budget"   // the member's budget has no room: withdraw it
	admissionFenced   admissionKind = "fenced"   // a stop holds the member: eject it
	admissionOther    admissionKind = "capacity" // anything else: the run is retried later
)

// classifyAdmission reads a refused child's code and, for a refused
// candidate, the state its data carries.
func classifyAdmission(result verbresult.Result) admissionKind {
	switch result.Code {
	case "GOAL_REVISION_MOVED":
		return admissionRevision
	case "BATCH_MEMBER_BUDGET_REFUSED", "BUDGET_REFUSED":
		return admissionBudget
	case "CANDIDATE_GOAL_REFUSED":
		var data struct {
			State string `json:"state"`
		}
		if result.DecodeData(&data) == nil && data.State == "fenced" {
			return admissionFenced
		}
	}
	return admissionOther
}

// admissionRefusal is the batch's refusal for a refused child: its reason
// is the child's summary and command, its code the child's code.
func admissionRefusal(result verbresult.Result) error {
	reason := result.Err().Error()
	switch classifyAdmission(result) {
	case admissionRevision:
		return &batch.PrefixRevisionRefusal{Reason: reason}
	case admissionBudget:
		return &batch.PrefixBudgetRefusal{Reason: reason}
	case admissionFenced:
		return &batch.PrefixFencedRefusal{Reason: reason}
	}
	return &batch.PrefixAdmissionRefusal{Code: result.Code, Reason: reason}
}

// laneHold reports whether an error is one a batch charged to the lane holds
// on with its plain reason, rather than a failure (U11b): the lane's account
// cannot be named.
func laneHold(err error) bool {
	var coded *refusal.Coded
	return errors.As(err, &coded) && coded.Code == lane.CodeAccountUnresolved
}
