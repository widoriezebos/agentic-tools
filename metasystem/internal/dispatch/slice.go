package dispatch

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

var (
	rulingApprovalRef = regexp.MustCompile(`^R-[0-9]+(?:[a-z]|-[a-z0-9][a-z0-9-]*)?$`)
)

// SliceAdmission is the reservation-only judgment for one job cap. It does
// not inspect a claim's elapsedLimit: that field bounds a batch, while this
// norm governs only the individual slice being reserved.
type SliceAdmission struct {
	CapMinutes    uint64
	NormHours     uint64
	ApprovedRef   string
	ApprovalClaim *SliceApprovalClaim
	Reason        string
	Refusal       string
}

// SliceApprovalClaim is the complete approval assertion fixed when a
// reservation is published. A later setup cannot substitute any coordinate.
type SliceApprovalClaim struct {
	ApprovedRef  string `json:"approvedRef"`
	GoalID       string `json:"goalId"`
	GoalRevision uint64 `json:"goalRevision"`
	CapMinutes   uint64 `json:"capMin"`
}

type recordedSliceApproval struct {
	Exists         bool
	GoalCovered    bool
	CapMinutes     uint64
	CapProven      bool
	GoalRevision   uint64
	RevisionProven bool
}

func (v SliceAdmission) Refused() bool { return v.Refusal != "" }

// EvaluateSliceAdmission applies the configured norm and proves any supplied
// exception against the two durable places where a human can leave the word.
func EvaluateSliceAdmission(repoRoot string, capMinutes uint64, approvedRef, goalID string, goalRevision uint64) (SliceAdmission, error) {
	norm, err := config.SliceNormHours(filepath.Join(repoRoot, "metasystem.conf"))
	if err != nil {
		return SliceAdmission{}, err
	}
	verdict := SliceAdmission{CapMinutes: capMinutes, NormHours: norm, ApprovedRef: approvedRef}
	if approvedRef != strings.TrimSpace(approvedRef) {
		verdict.Reason = "REFUSED-APPROVAL-REFERENCE"
		verdict.Refusal = "SLICE_CAP_REFUSED: --approved-ref must exactly name a rulings-register row or human goal-history operation"
		return verdict, nil
	}
	if approvedRef != "" {
		if goalID == "" || goalRevision == 0 {
			verdict.Reason = "REFUSED-APPROVAL-GOAL-MISMATCH"
			verdict.Refusal = "SLICE_CAP_REFUSED: --approved-ref requires this slice's goal id and positive accepted revision"
			return verdict, nil
		}
		approval, err := recordedHumanApproval(repoRoot, approvedRef, goalID)
		if err != nil {
			return verdict, err
		}
		if !approval.Exists {
			verdict.Reason = "REFUSED-APPROVAL-REFERENCE"
			verdict.Refusal = fmt.Sprintf("SLICE_CAP_REFUSED: --approved-ref %s does not name a rulings-register row or human goal-history operation", approvedRef)
			return verdict, nil
		}
		if !approval.GoalCovered || !approval.CapProven || !approval.RevisionProven {
			verdict.Reason = "REFUSED-APPROVAL-CAP-UNPROVEN"
			verdict.Refusal = fmt.Sprintf("SLICE_CAP_REFUSED: --approved-ref %s must approve this goal with the exact token form goal=<id> capMin=<n> goalRevision=<r>", approvedRef)
			return verdict, nil
		}
		if approval.CapMinutes < capMinutes {
			verdict.Reason = "REFUSED-APPROVAL-CAP-EXCEEDED"
			verdict.Refusal = fmt.Sprintf("SLICE_CAP_REFUSED: reservation cap %dm exceeds --approved-ref %s's proven %dm cap", capMinutes, approvedRef, approval.CapMinutes)
			return verdict, nil
		}
		if approval.GoalRevision != goalRevision {
			verdict.Reason = "REFUSED-APPROVAL-REVISION-MISMATCH"
			verdict.Refusal = fmt.Sprintf("SLICE_CAP_REFUSED: --approved-ref %s covers goal %s revision %d, not requested revision %d; re-approval is required", approvedRef, goalID, approval.GoalRevision, goalRevision)
			return verdict, nil
		}
		verdict.ApprovalClaim = &SliceApprovalClaim{
			ApprovedRef: approvedRef, GoalID: goalID,
			GoalRevision: approval.GoalRevision, CapMinutes: approval.CapMinutes,
		}
	}
	overNorm := norm <= math.MaxUint64/60 && capMinutes > norm*60
	if overNorm && approvedRef == "" {
		verdict.Reason = "REFUSED-SLICE-CAP"
		verdict.Refusal = fmt.Sprintf("SLICE_CAP_REFUSED: reservation cap %dm exceeds the %dh slice norm; bring the slice to Wido with --approved-ref naming the recorded human word, or carve it down", capMinutes, norm)
	}
	return verdict, nil
}

func recordedHumanApproval(repoRoot, ref, goalID string) (recordedSliceApproval, error) {
	if rulingApprovalRef.MatchString(ref) {
		data, err := os.ReadFile(filepath.Join(repoRoot, "memory", "rulings.md"))
		if os.IsNotExist(err) {
			return recordedSliceApproval{}, nil
		}
		if err != nil {
			return recordedSliceApproval{}, fmt.Errorf("read rulings register for --approved-ref: %w", err)
		}
		needle := "| " + ref + " |"
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), needle) {
				capMinutes, goalRevision, tripleProven := strictSliceApprovalTriple(line, goalID)
				return recordedSliceApproval{
					Exists: true, GoalCovered: tripleProven,
					CapMinutes: capMinutes, CapProven: tripleProven,
					GoalRevision: goalRevision, RevisionProven: tripleProven,
				}, nil
			}
		}
		return recordedSliceApproval{}, nil
	}
	if !goal.NewWorld(repoRoot) {
		return recordedSliceApproval{}, nil
	}
	endpoint, err := goal.ResolveEndpoint(repoRoot)
	if err != nil {
		return recordedSliceApproval{}, err
	}
	projection, err := goal.Project(endpoint, false, time.Now())
	if err != nil {
		return recordedSliceApproval{}, fmt.Errorf("read goal history for --approved-ref: %w", err)
	}
	contains := func(file *goal.GoalFile) (recordedSliceApproval, bool) {
		for _, line := range file.History {
			if line.Opid == ref && strings.HasPrefix(line.Actor, "human:") {
				capMinutes, goalRevision, proven := strictSliceApprovalTriple(line.Reason, goalID)
				return recordedSliceApproval{
					Exists: true, GoalCovered: proven,
					CapMinutes: capMinutes, CapProven: proven,
					GoalRevision: goalRevision, RevisionProven: proven,
				}, true
			}
		}
		return recordedSliceApproval{}, false
	}
	for _, file := range projection.Tree.Live {
		if approval, found := contains(file); found {
			return approval, nil
		}
	}
	for _, file := range projection.Tree.Done {
		if approval, found := contains(file); found {
			return approval, nil
		}
	}
	return recordedSliceApproval{}, nil
}

func strictSliceApprovalTriple(text, goalID string) (uint64, uint64, bool) {
	type coordinates struct {
		capMinutes   uint64
		goalRevision uint64
	}
	matches := map[coordinates]bool{}
	fields := strings.Fields(text)
	for index := 0; index+2 < len(fields); index++ {
		goal, found := strings.CutPrefix(fields[index], "goal=")
		if !found || goal != goalID {
			continue
		}
		capText, capFound := strings.CutPrefix(fields[index+1], "capMin=")
		revisionText, revisionFound := strings.CutPrefix(fields[index+2], "goalRevision=")
		if !capFound || !revisionFound {
			continue
		}
		capMinutes, capErr := strconv.ParseUint(capText, 10, 64)
		goalRevision, revisionErr := strconv.ParseUint(revisionText, 10, 64)
		if capErr != nil || revisionErr != nil || capMinutes == 0 || goalRevision == 0 {
			continue
		}
		matches[coordinates{capMinutes: capMinutes, goalRevision: goalRevision}] = true
	}
	if len(matches) != 1 {
		return 0, 0, false
	}
	for match := range matches {
		return match.capMinutes, match.goalRevision, true
	}
	return 0, 0, false
}

func sliceApprovalClaim(claim *SliceApprovalClaim) any {
	if claim == nil {
		return nil
	}
	return *claim
}

func proveSliceApprovalClaim(repoRoot string, capMinutes uint64, approvedRef, goalID string, goalRevision uint64) (*SliceApprovalClaim, error) {
	verdict, err := EvaluateSliceAdmission(repoRoot, capMinutes, approvedRef, goalID, goalRevision)
	if err != nil {
		return nil, err
	}
	if verdict.Refused() {
		return nil, &OpError{Code: 9, Reason: verdict.Reason, Message: verdict.Refusal}
	}
	return verdict.ApprovalClaim, nil
}
