package steward

import (
	"bufio"
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
)

const rulingDigestCeiling = 5

type rulingReview struct {
	ID    string
	Owner string
	Class string
	Due   string
	Event string
}

type dueRulingReview struct {
	rulingReview
	Evidence string
}

type rulingSweepState struct {
	Schema       int    `json:"schema"`
	AfterID      string `json:"afterId,omitempty"`
	LastDigestAt string `json:"lastDigestAt,omitempty"`
}

func parseReviewCondition(value string) (class, due, event string, err error) {
	if value == "" {
		return "", "", "", nil
	}
	for _, token := range strings.Fields(value) {
		key, val, found := strings.Cut(token, "=")
		if !found || val == "" {
			return "", "", "", fmt.Errorf("review condition token %q is not key=value", token)
		}
		switch key {
		case "class":
			class = val
		case "due":
			due = val
		case "event":
			event = val
		default:
			return "", "", "", fmt.Errorf("unknown review condition key %q", key)
		}
	}
	if class != "temporary" && class != "experimental" && class != "delegated-authority" && class != "assumption-dependent" {
		return "", "", "", fmt.Errorf("review class %q is not schedulable", class)
	}
	if due == "" && event == "" {
		return "", "", "", fmt.Errorf("review condition needs due= or event=")
	}
	if due != "" {
		if _, parseErr := time.Parse("2006-01-02", due); parseErr != nil {
			return "", "", "", fmt.Errorf("review due date %q is invalid", due)
		}
	}
	return class, due, event, nil
}

func readRulingReviews(repoRoot string) ([]rulingReview, error) {
	file, err := os.Open(filepath.Join(repoRoot, "memory", "rulings.md"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var reviews []rulingReview
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "| R-") {
			continue
		}
		fields := strings.Split(line, "|")
		if len(fields) != 8 {
			return nil, fmt.Errorf("ruling row has %d columns, want 6", len(fields)-2)
		}
		id, owner, condition := strings.TrimSpace(fields[1]), strings.TrimSpace(fields[5]), strings.TrimSpace(fields[6])
		if owner == "" {
			return nil, fmt.Errorf("ruling %s has no accountable owner", id)
		}
		class, due, event, err := parseReviewCondition(condition)
		if err != nil {
			return nil, fmt.Errorf("ruling %s: %w", id, err)
		}
		if class != "" {
			reviews = append(reviews, rulingReview{ID: id, Owner: owner, Class: class, Due: due, Event: event})
		}
	}
	return reviews, scanner.Err()
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

func rulingEventEvidence(repoRoot, event string, reviews []rulingReview) (observed bool, evidence string) {
	switch event {
	case "first-measured-report-exists", "first-measured-would-have-triggered-report-exists":
		path := filepath.Join(repoRoot, "artifacts", "agents", "governance", "correlation-policy-a-report.json")
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return true, "event-observed=" + event
		}
		return false, ""
	case "superseded-by-r22-m1":
		for _, review := range reviews {
			if review.ID == "R-22-m1" {
				return true, "event-observed=" + event
			}
		}
		return false, ""
	default:
		return true, "needs-attention=event-condition-unobservable:" + event
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
			if observed, evidence := rulingEventEvidence(repoRoot, review.Event, reviews); observed {
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

// sweepRulingReviews emits at most one digest entry. Rows never acquire their
// own delivery or notification path.
func sweepRulingReviews(repoRoot string, now time.Time) error {
	reviews, err := readRulingReviews(repoRoot)
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
	text := dueRulingReviewText(due, shown, rulingDigestCeiling)
	if text == "" {
		return nil
	}
	sourceID := now.UTC().Format("2006-01-02") + "-after-" + state.AfterID
	if err := narratordigest.Append(repoRoot, []narratordigest.Entry{{Kind: "lowlight", Text: text,
		SourceType: "ruling-review-sweep", SourceID: sourceID}}, now); err != nil {
		return err
	}
	state.AfterID = shown[len(shown)-1].ID
	state.LastDigestAt = now.UTC().Format(time.RFC3339)
	return saveRulingSweep(repoRoot, state)
}
