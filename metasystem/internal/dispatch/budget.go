package dispatch

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// BudgetProjectionStatus separates complete spending evidence from a typed
// refusal to guess. Unknown never becomes an all-zero projection.
type BudgetProjectionStatus string

const (
	BudgetKnown   BudgetProjectionStatus = "KNOWN"
	BudgetUnknown BudgetProjectionStatus = "BUDGET_UNKNOWN"
)

// ElapsedBudgetState names which elapsed threshold the projection has crossed.
// An empty state means elapsed time is still below the admission limit.
type ElapsedBudgetState string

const (
	AdmissionClosedElapsed ElapsedBudgetState = "ADMISSION_CLOSED_ELAPSED"
	ElapsedBreach          ElapsedBudgetState = "ELAPSED_BREACH"
)

// BudgetUnknownEvidence names the exact authoritative record that prevents a
// trustworthy projection.
type BudgetUnknownEvidence struct {
	Code   BudgetProjectionStatus
	Record string
	Reason string
}

// BudgetBreach is one limit comparison that health can surface without
// turning the projection into an enforcement mechanism.
type BudgetBreach struct {
	Field string
	Used  string
	Limit string
	State ElapsedBudgetState
}

// BudgetProjection is the complete four-dimensional view for one claimed
// goal revision. Limits come from the goal; all spending comes from job
// records, except elapsed time which comes from the revision's claim row.
type BudgetProjection struct {
	Status              BudgetProjectionStatus
	GoalID              string
	GoalRevision        uint64
	Limits              goal.Budget
	Attempts            uint64
	ReservedJobMinutes  uint64
	ActiveJobs          uint64
	Elapsed             time.Duration
	ElapsedGracePercent uint64
	ElapsedBreachLimit  time.Duration
	ElapsedState        ElapsedBudgetState
	Breaches            []BudgetBreach
	Unknown             *BudgetUnknownEvidence
}

func unknownBudget(id string, revision uint64, record, reason string) BudgetProjection {
	evidence := &BudgetUnknownEvidence{Code: BudgetUnknown, Record: record, Reason: reason}
	return BudgetProjection{Status: BudgetUnknown, GoalID: id, GoalRevision: revision, Unknown: evidence}
}

// GoalRecordBudgetUnknown extracts a claim-binding contradiction from a
// rejected accepted-tree read. The tree parser supplies the exact goal path;
// callers keep that path when they turn the contradiction into a refusal.
func GoalRecordBudgetUnknown(err error) (*BudgetUnknownEvidence, bool) {
	var readErr *goal.TreeReadError
	if !errors.As(err, &readErr) {
		return nil, false
	}
	const marker = ": BUDGET_UNKNOWN "
	for _, problem := range readErr.Problems {
		text := string(problem)
		at := strings.Index(text, marker)
		if at < 0 {
			continue
		}
		record := text[:at]
		if !strings.HasPrefix(record, "plans/goals/") {
			continue
		}
		return &BudgetUnknownEvidence{
			Code: BudgetUnknown, Record: record, Reason: text[at+len(marker):],
		}, true
	}
	return nil, false
}

func goalRecordPath(id string) string {
	return filepath.ToSlash(filepath.Join("plans", "goals", id+".md"))
}

func jobRecordPath(name string) string {
	return filepath.ToSlash(filepath.Join("artifacts", "agents", "jobs", name))
}

// ProjectBudget scans the reservation records once and derives the sole
// spending view for the claim revision. It has no side effects; health and
// pre-publication admission consume the same complete projection.
func ProjectBudget(repoRoot string, file *goal.GoalFile, now time.Time) BudgetProjection {
	if file == nil {
		return unknownBudget("", 0, "plans/goals", "the goal record is missing")
	}
	recordPath := goalRecordPath(file.Id)
	if file.State != goal.StateClaimed || file.Claimed == nil {
		return unknownBudget(file.Id, 0, recordPath, "the goal has no claim record")
	}
	revision := file.Claimed.Revision
	if file.Budget == nil {
		return unknownBudget(file.Id, revision, recordPath, "the claimed goal has no structured budget")
	}
	if revision == 0 {
		return unknownBudget(file.Id, 0, recordPath, "the claim record is revisionless")
	}
	if err := file.ValidateClaimRevision(); err != nil {
		return unknownBudget(file.Id, revision, recordPath, err.Error())
	}
	if err := file.Budget.Validate(); err != nil {
		return unknownBudget(file.Id, revision, recordPath, "the budget tuple is malformed: "+err.Error())
	}
	claimedAt, err := time.Parse(time.RFC3339, file.Claimed.At)
	if err != nil {
		return unknownBudget(file.Id, revision, recordPath, "the revision claim timestamp is malformed")
	}
	if now.Before(claimedAt) {
		return unknownBudget(file.Id, revision, recordPath, "CLOCK_REGRESSED: the revision claim is later than the observation")
	}
	gracePercent, err := config.ElapsedGracePercent(filepath.Join(repoRoot, "metasystem.conf"))
	if err != nil {
		return unknownBudget(file.Id, revision, "metasystem.conf", err.Error())
	}
	breachLimit, err := file.Budget.ElapsedBreachDuration(gracePercent)
	if err != nil {
		return unknownBudget(file.Id, revision, recordPath, err.Error())
	}

	projection := BudgetProjection{
		Status: BudgetKnown, GoalID: file.Id, GoalRevision: revision,
		Limits: *file.Budget, Elapsed: now.Sub(claimedAt),
		ElapsedGracePercent: gracePercent, ElapsedBreachLimit: breachLimit,
	}
	jobsDir := filepath.Join(repoRoot, "artifacts", "agents", "jobs")
	entries, err := os.ReadDir(jobsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return finishBudgetProjection(projection)
		}
		return unknownBudget(file.Id, revision, jobRecordPath(""), "the job-record directory is unreadable: "+err.Error())
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	operations := make(map[string]string)
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		logicalPath := jobRecordPath(entry.Name())
		path := filepath.Join(jobsDir, entry.Name())
		record, readErr := readObject(path)
		if readErr != nil {
			return unknownBudget(file.Id, revision, logicalPath, "the authoritative job record is unreadable: "+readErr.Error())
		}
		goalValue, present := record["goalId"]
		if !present {
			return unknownBudget(file.Id, revision, logicalPath, "the authoritative job record has no goalId field")
		}
		if goalValue == nil {
			if goalRevision, hasRevision := record["goalRevision"]; hasRevision && goalRevision != nil {
				return unknownBudget(file.Id, revision, logicalPath, "the authoritative job record has a goalRevision but explicitly no goalId")
			}
			continue
		}
		recordGoal, ok := goalValue.(string)
		if !ok || recordGoal == "" {
			return unknownBudget(file.Id, revision, logicalPath, "the authoritative job record has a contradictory goalId")
		}
		if recordGoal != file.Id {
			continue
		}
		lens := JobRecordOf(record)
		jobID := lens.JobID()
		wantJobID := strings.TrimSuffix(entry.Name(), ".json")
		if jobID == "" || jobID != wantJobID {
			return unknownBudget(file.Id, revision, logicalPath, fmt.Sprintf("jobId %q contradicts record name %q", jobID, wantJobID))
		}
		operationID := lens.OperationID()
		if operationID == "" {
			return unknownBudget(file.Id, revision, logicalPath, "the authoritative reservation has no operationId")
		}
		if first, duplicate := operations[operationID]; duplicate {
			return unknownBudget(file.Id, revision, logicalPath, fmt.Sprintf("operationId %q duplicates %s", operationID, first))
		}
		operations[operationID] = logicalPath
		recordRevision, ok := lens.GoalRevision()
		if !ok || recordRevision == 0 {
			return unknownBudget(file.Id, revision, logicalPath, "the authoritative reservation is revisionless")
		}
		if recordRevision > revision {
			return unknownBudget(file.Id, revision, logicalPath, fmt.Sprintf("goalRevision %d is later than accepted claim revision %d", recordRevision, revision))
		}
		capMinutes, ok := lens.CapMinutes()
		if !ok || capMinutes == 0 {
			return unknownBudget(file.Id, revision, logicalPath, "the authoritative reservation has no positive capMin")
		}
		status := lens.Status()
		if status != "pending-setup" && status != "pending" && status != "running" && !TerminalStatus(status) {
			return unknownBudget(file.Id, revision, logicalPath, fmt.Sprintf("status %q is outside the reservation lifecycle", status))
		}
		if recordRevision != revision {
			continue
		}
		if projection.Attempts == math.MaxUint64 {
			return unknownBudget(file.Id, revision, logicalPath, "attempt accounting overflowed")
		}
		projection.Attempts++
		if capMinutes > math.MaxUint64-projection.ReservedJobMinutes {
			return unknownBudget(file.Id, revision, logicalPath, "reserved job-minute accounting overflowed")
		}
		projection.ReservedJobMinutes += capMinutes
		if !TerminalStatus(status) {
			if projection.ActiveJobs == math.MaxUint64 {
				return unknownBudget(file.Id, revision, logicalPath, "active-job accounting overflowed")
			}
			projection.ActiveJobs++
		}
	}
	return finishBudgetProjection(projection)
}

func finishBudgetProjection(projection BudgetProjection) BudgetProjection {
	elapsedLimit := projection.Limits.ElapsedDuration()
	switch {
	case projection.Elapsed >= projection.ElapsedBreachLimit:
		projection.ElapsedState = ElapsedBreach
		projection.Breaches = append(projection.Breaches, BudgetBreach{
			Field: "elapsedLimit", Used: projection.Elapsed.Round(time.Second).String(),
			Limit: projection.ElapsedBreachLimit.String(), State: ElapsedBreach,
		})
	case projection.Elapsed >= elapsedLimit:
		projection.ElapsedState = AdmissionClosedElapsed
	}
	if projection.Attempts > projection.Limits.AttemptLimit {
		projection.Breaches = append(projection.Breaches, budgetIntegerBreach("attemptLimit", projection.Attempts, projection.Limits.AttemptLimit))
	}
	if projection.ReservedJobMinutes > projection.Limits.ReservedJobMinutesLimit {
		projection.Breaches = append(projection.Breaches, budgetIntegerBreach("reservedJobMinutesLimit", projection.ReservedJobMinutes, projection.Limits.ReservedJobMinutesLimit))
	}
	if projection.ActiveJobs > projection.Limits.ActiveJobLimit {
		projection.Breaches = append(projection.Breaches, budgetIntegerBreach("activeJobLimit", projection.ActiveJobs, projection.Limits.ActiveJobLimit))
	}
	return projection
}

func budgetIntegerBreach(field string, used, limit uint64) BudgetBreach {
	return BudgetBreach{Field: field, Used: strconv.FormatUint(used, 10), Limit: strconv.FormatUint(limit, 10)}
}
