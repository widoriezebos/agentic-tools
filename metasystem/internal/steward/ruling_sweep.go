package steward

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/narratordigest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/rulings"
)

const rulingDigestCeiling = 5

// The register reader is internal/rulings now, with its acceptance rules and
// its defect wording lifted whole. The sweep reads the same scheduled subset
// it always read, under the same two names, so nothing below this line
// changed when the browser's Decisions page became the reader's second
// caller.
type rulingReview = rulings.Review

type dueRulingReview struct {
	rulingReview
	Evidence string
}

type rulingReviewDefect = rulings.Defect

type rulingSweepState struct {
	Schema       int    `json:"schema"`
	AfterID      string `json:"afterId,omitempty"`
	LastDigestAt string `json:"lastDigestAt,omitempty"`
}

func readRulingReviewRegister(repoRoot string) ([]rulingReview, []rulingReviewDefect, error) {
	return rulings.ReadReviews(repoRoot)
}

func rulingSweepPath(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "ruling-review-sweep.json")
}

func loadRulingSweep(repoRoot string) (rulingSweepState, error) {
	data, err := os.ReadFile(rulingSweepPath(repoRoot))
	if os.IsNotExist(err) {
		return rulingSweepState{Schema: 1}, nil
	}
	if err != nil {
		return rulingSweepState{}, err
	}
	var state rulingSweepState
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return rulingSweepState{}, fmt.Errorf("ruling review sweep state is malformed: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF || state.Schema != 1 {
		return rulingSweepState{}, fmt.Errorf("ruling review sweep state has an unknown contract")
	}
	if state.LastDigestAt != "" {
		if _, err := time.Parse(time.RFC3339, state.LastDigestAt); err != nil {
			return rulingSweepState{}, fmt.Errorf("ruling review sweep timestamp is invalid")
		}
	}
	return state, nil
}

func saveRulingSweep(repoRoot string, state rulingSweepState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	durable, err := atomicfile.WriteText(rulingSweepPath(repoRoot), string(data)+"\n", repoRoot)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("ruling review sweep state durability is unknown")
	}
	return nil
}

func rulingEventEvidence(repoRoot, event string, reviews []rulingReview) (observed bool, evidence string, mechanicallyObservable bool) {
	switch event {
	case "first-measured-report-exists", "first-measured-would-have-triggered-report-exists":
		path := filepath.Join(repoRoot, "artifacts", "agents", "governance", "correlation-policy-a-report.json")
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return true, "event-observed=" + event, true
		}
		return false, "", true
	case "superseded-by-r22-m1":
		for _, review := range reviews {
			if review.ID == "R-22-m1" {
				return true, "event-observed=" + event, true
			}
		}
		return false, "", true
	default:
		return true, "needs-attention=event-condition-unobservable:" + event, false
	}
}

func eligibleRulingReviews(repoRoot string, reviews []rulingReview, now time.Time) []dueRulingReview {
	day := now.UTC().Format("2006-01-02")
	var due []dueRulingReview
	for _, review := range reviews {
		if review.Due != "" && review.Due <= day {
			due = append(due, dueRulingReview{rulingReview: review, Evidence: "due=" + review.Due})
			continue
		}
		if review.Event != "" {
			observed, evidence, mechanicallyObservable := rulingEventEvidence(repoRoot, review.Event, reviews)
			if observed && (review.Due == "" || mechanicallyObservable) {
				due = append(due, dueRulingReview{rulingReview: review, Evidence: evidence})
			}
		}
	}
	sort.Slice(due, func(i, j int) bool { return due[i].ID < due[j].ID })
	return due
}

func rotatedRulingReviews(due []dueRulingReview, afterID string, ceiling int) []dueRulingReview {
	if len(due) == 0 || ceiling < 1 {
		return nil
	}
	start := 0
	if afterID != "" {
		for index := range due {
			if due[index].ID > afterID {
				start = index
				break
			}
			if index == len(due)-1 {
				start = 0
			}
		}
	}
	count := len(due)
	if count > ceiling {
		count = ceiling
	}
	shown := make([]dueRulingReview, 0, count)
	for offset := 0; offset < count; offset++ {
		shown = append(shown, due[(start+offset)%len(due)])
	}
	return shown
}

func dueRulingReviewText(due, shown []dueRulingReview, ceiling int) string {
	if ceiling < 1 {
		return ""
	}
	if len(shown) == 0 {
		return ""
	}
	items := make([]string, len(shown))
	for index := range shown {
		items[index] = fmt.Sprintf("%s owner=%s class=%s %s choice=adopt|revise|withdraw",
			shown[index].ID, shown[index].Owner, shown[index].Class, shown[index].Evidence)
	}
	text := fmt.Sprintf("Ruling review sweep: %s", strings.Join(items, "; "))
	if len(due) > len(shown) {
		text += fmt.Sprintf("; +%d more rotate behind the %d-item attention ceiling", len(due)-len(shown), ceiling)
	}
	return text
}

func rulingReviewDefectText(defects []rulingReviewDefect, ceiling int) string {
	if len(defects) == 0 || ceiling < 1 {
		return ""
	}
	count := len(defects)
	if count > ceiling {
		count = ceiling
	}
	items := make([]string, count)
	for index := 0; index < count; index++ {
		items[index] = fmt.Sprintf("%s defect=%s", defects[index].Label, defects[index].Reason)
		if defects[index].Ownerless {
			items[index] += " choice=adopt|withdraw"
		}
	}
	text := strings.Join(items, "; ")
	if len(defects) > count {
		text += fmt.Sprintf("; +%d more behind the %d-item attention ceiling", len(defects)-count, ceiling)
	}
	return text
}

func rulingReviewSweepText(defects []rulingReviewDefect, due, shown []dueRulingReview, ceiling int) string {
	defectText := rulingReviewDefectText(defects, ceiling)
	dueText := dueRulingReviewText(due, shown, ceiling)
	if defectText == "" {
		return dueText
	}
	if dueText == "" {
		return "Ruling review sweep: " + defectText
	}
	return "Ruling review sweep: " + defectText + "; " + strings.TrimPrefix(dueText, "Ruling review sweep: ")
}

// sweepRulingReviews emits at most one digest entry. Rows never acquire their
// own delivery or notification path.
func sweepRulingReviews(repoRoot string, now time.Time) error {
	reviews, defects, err := readRulingReviewRegister(repoRoot)
	if err != nil {
		return err
	}
	state, err := loadRulingSweep(repoRoot)
	if err != nil {
		return err
	}
	if state.LastDigestAt != "" {
		last, _ := time.Parse(time.RFC3339, state.LastDigestAt)
		if now.UTC().Before(last.Add(24 * time.Hour)) {
			return nil
		}
	}
	due := eligibleRulingReviews(repoRoot, reviews, now)
	shown := rotatedRulingReviews(due, state.AfterID, rulingDigestCeiling)
	text := rulingReviewSweepText(defects, due, shown, rulingDigestCeiling)
	if text == "" {
		return nil
	}
	sourceID := now.UTC().Format("2006-01-02") + "-after-" + state.AfterID
	if err := narratordigest.Append(repoRoot, []narratordigest.Entry{{Kind: "lowlight", Text: text,
		SourceType: "ruling-review-sweep", SourceID: sourceID}}, now); err != nil {
		return err
	}
	if len(shown) > 0 {
		state.AfterID = shown[len(shown)-1].ID
	}
	state.LastDigestAt = now.UTC().Format(time.RFC3339)
	return saveRulingSweep(repoRoot, state)
}
