package proofrun

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

// TestAdmission keeps the person's authority separate from attribution,
// goal accounting and the failed attempt whose retained evidence they rerun.
// Parsed records describe authority; only a fresh proof authorizes a rerun.
type TestAdmission struct {
	By                 string               `json:"by"`
	Authority          string               `json:"authority"`
	CheckedAt          string               `json:"checkedAt"`
	GoalID             string               `json:"goalId"`
	AccountingRevision uint64               `json:"accountingRevision"`
	Rerun              string               `json:"rerun,omitempty"`
	Proof              humanauthority.Proof `json:"proof"`
}

// AuthenticateTestAdmission leaves ordinary agents on the existing path.
func AuthenticateTestAdmission(root string, pid int64, reader humanauthority.Reader, now time.Time) *TestAdmission {
	proof, err := humanauthority.Prove(root, pid, reader, now)
	if err != nil || !proof.EnrolledTerminalFor(root) {
		return nil
	}
	admission := &TestAdmission{Authority: "enrolled-terminal", CheckedAt: proof.CheckedAt.Format(time.RFC3339Nano), Proof: proof}
	if proof.Helm != nil {
		admission.Authority, admission.By = "helm", proof.Helm.By
	} else {
		enrollment, err := humanauthority.ReadEnrollment(root)
		if err != nil || enrollment.Generation != proof.TerminalGeneration || enrollment.TerminalRef != proof.TerminalRef {
			return nil
		}
		admission.By = enrollment.Human
	}
	if strings.TrimSpace(admission.By) == "" {
		return nil
	}
	return admission
}

func (admission *TestAdmission) authorizes(root string) bool {
	return admission != nil && admission.Proof.EnrolledTerminalFor(root) && admission.validate() == nil
}

func (admission *TestAdmission) validate() error {
	if admission == nil {
		return nil
	}
	if !admission.Proof.RecordedEnrolled() || admission.CheckedAt != admission.Proof.CheckedAt.Format(time.RFC3339Nano) ||
		(admission.Authority == "helm") != (admission.Proof.Helm != nil) || admission.Proof.Helm != nil && admission.By != admission.Proof.Helm.By {
		return fmt.Errorf("test run human admission has contradictory authority")
	}
	_, err := time.Parse(time.RFC3339Nano, admission.CheckedAt)
	if strings.TrimSpace(admission.By) == "" || (admission.Authority != "enrolled-terminal" && admission.Authority != "helm") ||
		err != nil || admission.GoalID == "" || admission.AccountingRevision == 0 || admission.Rerun != "" && !safeAttemptID(admission.Rerun) {
		return fmt.Errorf("test run human admission is incomplete")
	}
	return nil
}

func sameTestAdmission(a, b *TestAdmission) bool {
	if a == nil || b == nil {
		return a == b
	}
	left, leftErr := json.Marshal(a)
	right, rightErr := json.Marshal(b)
	return leftErr == nil && rightErr == nil && bytes.Equal(left, right)
}
