package dispatch

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

var rulingApprovalRef = regexp.MustCompile(`^R-[0-9]+$`)

// SliceAdmission is the reservation-only judgment for one job cap. It does
// not inspect a claim's elapsedLimit: that field bounds a batch, while this
// norm governs only the individual slice being reserved.
type SliceAdmission struct {
	CapMinutes  uint64
	NormHours   uint64
	ApprovedRef string
	Refusal     string
}

func (v SliceAdmission) Refused() bool { return v.Refusal != "" }

// EvaluateSliceAdmission applies the configured norm and proves any supplied
// exception against the two durable places where a human can leave the word.
func EvaluateSliceAdmission(repoRoot string, capMinutes uint64, approvedRef string) (SliceAdmission, error) {
	norm, err := config.SliceNormHours(filepath.Join(repoRoot, "metasystem.conf"))
	if err != nil {
		return SliceAdmission{}, err
	}
	verdict := SliceAdmission{CapMinutes: capMinutes, NormHours: norm, ApprovedRef: approvedRef}
	if approvedRef != strings.TrimSpace(approvedRef) {
		verdict.Refusal = "SLICE_CAP_REFUSED: --approved-ref must exactly name a rulings-register row or human goal-history operation"
		return verdict, nil
	}
	if approvedRef != "" {
		approved, err := recordedHumanApproval(repoRoot, approvedRef)
		if err != nil {
			return verdict, err
		}
		if !approved {
			verdict.Refusal = fmt.Sprintf("SLICE_CAP_REFUSED: --approved-ref %s does not name a rulings-register row or human goal-history operation", approvedRef)
			return verdict, nil
		}
	}
	overNorm := norm <= math.MaxUint64/60 && capMinutes > norm*60
	if overNorm && approvedRef == "" {
		verdict.Refusal = fmt.Sprintf("SLICE_CAP_REFUSED: reservation cap %dm exceeds the %dh slice norm; bring the slice to Wido with --approved-ref naming the recorded human word, or carve it down", capMinutes, norm)
	}
	return verdict, nil
}

func recordedHumanApproval(repoRoot, ref string) (bool, error) {
	if rulingApprovalRef.MatchString(ref) {
		data, err := os.ReadFile(filepath.Join(repoRoot, "memory", "rulings.md"))
		if os.IsNotExist(err) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("read rulings register for --approved-ref: %w", err)
		}
		needle := "| " + ref + " |"
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), needle) {
				return true, nil
			}
		}
		return false, nil
	}
	if !goal.NewWorld(repoRoot) {
		return false, nil
	}
	endpoint, err := goal.ResolveEndpoint(repoRoot)
	if err != nil {
		return false, err
	}
	projection, err := goal.Project(endpoint, false, time.Now())
	if err != nil {
		return false, fmt.Errorf("read goal history for --approved-ref: %w", err)
	}
	contains := func(file *goal.GoalFile) bool {
		for _, line := range file.History {
			if line.Opid == ref && strings.HasPrefix(line.Actor, "human:") {
				return true
			}
		}
		return false
	}
	for _, file := range projection.Tree.Live {
		if contains(file) {
			return true, nil
		}
	}
	for _, file := range projection.Tree.Done {
		if contains(file) {
			return true, nil
		}
	}
	return false, nil
}
