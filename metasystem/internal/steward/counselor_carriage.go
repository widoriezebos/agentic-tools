package steward

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/counselor"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/narratordigest"
)

const (
	counselorBriefCadenceKey          = "metasystem.counselor.brief-cadence-hours"
	defaultCounselorBriefCadenceHours = 24
	counselorBriefDelivered           = "DELIVERED"
	counselorBriefRenderFailed        = "RENDER_FAILED"
)

type counselorBriefCursor struct {
	Schema       int    `json:"schema"`
	PeriodStart  string `json:"periodStart"`
	CadenceHours int    `json:"cadenceHours"`
	Status       string `json:"status"`
	RecordedAt   string `json:"recordedAt"`
	Failure      string `json:"failure,omitempty"`
}

func counselorBriefCursorPath(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "counselor-brief-cursor.json")
}

func counselorBriefCadenceHours(repoRoot string) (int, error) {
	value, _, err := config.Get(config.GetParams{
		Key: counselorBriefCadenceKey, ConfPath: filepath.Join(repoRoot, "metasystem.conf"),
		Default: strconv.Itoa(defaultCounselorBriefCadenceHours), DefaultSet: true,
	})
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return 0, err
		}
		value = strconv.Itoa(defaultCounselorBriefCadenceHours)
	}
	hours, err := strconv.Atoi(value)
	if err != nil || hours < 1 || hours > int(math.MaxInt64/int64(time.Hour)) {
		return 0, fmt.Errorf("%s must be a positive integer that fits a time duration", counselorBriefCadenceKey)
	}
	return hours, nil
}

func counselorBriefPeriodStart(now time.Time, cadenceHours int) time.Time {
	periodSeconds := int64(cadenceHours) * int64(time.Hour/time.Second)
	epoch := now.UTC().Unix()
	remainder := epoch % periodSeconds
	if remainder < 0 {
		remainder += periodSeconds
	}
	return time.Unix(epoch-remainder, 0).UTC()
}

func counselorBriefSourceID(periodStart time.Time, cadenceHours int) string {
	return periodStart.UTC().Format("20060102T150405Z") + "-" + strconv.Itoa(cadenceHours) + "h"
}

func loadCounselorBriefCursor(repoRoot string) (counselorBriefCursor, error) {
	data, err := os.ReadFile(counselorBriefCursorPath(repoRoot))
	if os.IsNotExist(err) {
		return counselorBriefCursor{Schema: 1}, nil
	}
	if err != nil {
		return counselorBriefCursor{}, err
	}
	var cursor counselorBriefCursor
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cursor); err != nil {
		return counselorBriefCursor{}, fmt.Errorf("counselor brief cursor is malformed: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return counselorBriefCursor{}, fmt.Errorf("counselor brief cursor has trailing JSON or an unknown contract")
	}
	if cursor.Schema != 1 || cursor.CadenceHours < 1 ||
		(cursor.Status != counselorBriefDelivered && cursor.Status != counselorBriefRenderFailed) {
		return counselorBriefCursor{}, fmt.Errorf("counselor brief cursor has an unknown contract")
	}
	if _, err := time.Parse(time.RFC3339, cursor.PeriodStart); err != nil {
		return counselorBriefCursor{}, fmt.Errorf("counselor brief cursor period is invalid")
	}
	if _, err := time.Parse(time.RFC3339, cursor.RecordedAt); err != nil {
		return counselorBriefCursor{}, fmt.Errorf("counselor brief cursor timestamp is invalid")
	}
	if (cursor.Status == counselorBriefDelivered && cursor.Failure != "") ||
		(cursor.Status == counselorBriefRenderFailed && cursor.Failure == "") {
		return counselorBriefCursor{}, fmt.Errorf("counselor brief cursor status contradicts its failure evidence")
	}
	return cursor, nil
}

func saveCounselorBriefCursor(repoRoot string, cursor counselorBriefCursor) error {
	data, err := json.MarshalIndent(cursor, "", "  ")
	if err != nil {
		return err
	}
	durable, err := atomicfile.WriteText(counselorBriefCursorPath(repoRoot), string(data)+"\n", repoRoot)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("counselor brief cursor durability is unknown")
	}
	return nil
}

func counselorBriefPeriodRecorded(cursor counselorBriefCursor, periodStart time.Time, cadenceHours int, status string) bool {
	return cursor.PeriodStart == periodStart.UTC().Format(time.RFC3339) &&
		cursor.CadenceHours == cadenceHours && cursor.Status == status
}

func recordCounselorRenderFailure(repoRoot string, cursor counselorBriefCursor, periodStart time.Time,
	cadenceHours int, now time.Time, renderErr error) error {
	failure := renderErr.Error()
	if counselorBriefPeriodRecorded(cursor, periodStart, cadenceHours, counselorBriefRenderFailed) && cursor.Failure == failure {
		return nil
	}
	sourceID := counselorBriefSourceID(periodStart, cadenceHours)
	line := fmt.Sprintf("HEALTH unhealthy — counselor=dead (brief render failed for the period beginning %s: %v; remedy: repair the counselor renderer and run metasystem steward tick --repo %q)",
		periodStart.Format(time.RFC3339), renderErr, repoRoot)
	if err := narratordigest.Append(repoRoot, []narratordigest.Entry{{
		Kind: "lowlight", Text: line, SourceType: "counselor-brief-health", SourceID: sourceID,
	}}, now); err != nil {
		return fmt.Errorf("publish counselor render health: %w", err)
	}
	if err := NarrateHealthLine(repoRoot, line); err != nil {
		return fmt.Errorf("narrate counselor render health: %w", err)
	}
	return saveCounselorBriefCursor(repoRoot, counselorBriefCursor{
		Schema: 1, PeriodStart: periodStart.Format(time.RFC3339), CadenceHours: cadenceHours,
		Status: counselorBriefRenderFailed, RecordedAt: now.UTC().Format(time.RFC3339), Failure: failure,
	})
}

// sweepCounselorBrief carries the complete renderer output directly into the
// narrator digest. No caller supplies a transformation, limit, or policy.
func sweepCounselorBrief(repoRoot string, now time.Time) error {
	now = now.UTC().Truncate(time.Second)
	cadenceHours, err := counselorBriefCadenceHours(repoRoot)
	if err != nil {
		return err
	}
	periodStart := counselorBriefPeriodStart(now, cadenceHours)
	cursor, err := loadCounselorBriefCursor(repoRoot)
	if err != nil {
		return err
	}
	if cursor.RecordedAt != "" {
		recordedAt, _ := time.Parse(time.RFC3339, cursor.RecordedAt)
		if now.Before(recordedAt) {
			return fmt.Errorf("counselor brief clock regressed behind its durable cursor")
		}
	}
	if counselorBriefPeriodRecorded(cursor, periodStart, cadenceHours, counselorBriefDelivered) {
		return nil
	}

	brief := counselor.Build(counselor.Options{Root: repoRoot, Now: func() time.Time { return now }})
	var rendered bytes.Buffer
	if err := counselor.Render(&rendered, brief); err != nil {
		return recordCounselorRenderFailure(repoRoot, cursor, periodStart, cadenceHours, now, err)
	}
	if rendered.Len() == 0 {
		return recordCounselorRenderFailure(repoRoot, cursor, periodStart, cadenceHours, now,
			fmt.Errorf("the counselor renderer produced no bytes"))
	}
	if err := narratordigest.AppendPayload(repoRoot, narratordigest.Payload{
		Kind: "lowlight", Body: rendered.Bytes(), SourceType: "counselor-brief",
		SourceID: counselorBriefSourceID(periodStart, cadenceHours),
	}, now); err != nil {
		return fmt.Errorf("publish counselor brief: %w", err)
	}
	return saveCounselorBriefCursor(repoRoot, counselorBriefCursor{
		Schema: 1, PeriodStart: periodStart.Format(time.RFC3339), CadenceHours: cadenceHours,
		Status: counselorBriefDelivered, RecordedAt: now.Format(time.RFC3339),
	})
}
