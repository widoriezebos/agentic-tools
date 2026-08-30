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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/obligationstate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
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
// goal revision. Limits come from the goal; spending comes from job records,
// live governed runs, and durable terminal obligation state. Elapsed time
// begins at the claim or the latest exact consumed discharge proof.
type BudgetProjection struct {
	Status              BudgetProjectionStatus
	GoalID              string
	GoalRevision        uint64
	Limits              goal.Budget
	StartedAt           time.Time
	WeightEpoch         *uint64
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

func obligationBudgetStart(repoRoot string, file *goal.GoalFile, claimedAt time.Time) (time.Time, *uint64, *BudgetUnknownEvidence) {
	if file.Obligation == nil {
		return claimedAt, nil, nil
	}
	logicalPath := "artifacts/agents/validation-weight.json"
	path := filepath.Join(repoRoot, filepath.FromSlash(logicalPath))
	state, err := readObject(path)
	if os.IsNotExist(err) {
		return claimedAt, nil, nil
	}
	if err != nil {
		return time.Time{}, nil, &BudgetUnknownEvidence{Code: BudgetUnknown, Record: logicalPath, Reason: "the validation-weight record is unreadable: " + err.Error()}
	}
	schema, schemaOK := numInt(state["schema"])
	generation, ok := numInt(state["generation"])
	if !schemaOK || schema != 1 || !ok || generation < 0 {
		return time.Time{}, nil, &BudgetUnknownEvidence{Code: BudgetUnknown, Record: logicalPath, Reason: "validation-weight schema or generation is missing or invalid"}
	}
	rawConsumed, present := state["consumedProofs"]
	if !present {
		if last, typed := state["lastDecision"].(map[string]any); typed {
			if applied, appliedTyped := last["applied"].(bool); appliedTyped && applied {
				return time.Time{}, nil, &BudgetUnknownEvidence{Code: BudgetUnknown, Record: logicalPath, Reason: "applied discharge has no durable consumed-proof ledger"}
			}
		}
		return claimedAt, nil, nil
	}
	consumed, ok := rawConsumed.([]any)
	if !ok {
		return time.Time{}, nil, &BudgetUnknownEvidence{Code: BudgetUnknown, Record: logicalPath, Reason: "consumed-proof ledger is not an array"}
	}
	latest := claimedAt
	latestRunID := ""
	latestProofEpoch := uint64(0)
	seenProofs := map[string]bool{}
	for _, item := range consumed {
		proof, typed := item.(map[string]any)
		if !typed {
			return time.Time{}, nil, &BudgetUnknownEvidence{Code: BudgetUnknown, Record: logicalPath, Reason: "consumed-proof ledger has an untyped entry"}
		}
		goalRevision, goalRevisionOK := numInt(proof["goalRevision"])
		obligationRevision, obligationRevisionOK := numInt(proof["obligationRevision"])
		proofEpoch, epochOK := numInt(proof["weightGeneration"])
		consumedAt, timeErr := time.Parse(time.RFC3339, asString(proof["consumedAt"]))
		reset, resetOK := proof["resetDecision"].(map[string]any)
		discharge, dischargeOK := proof["dischargeDecision"].(map[string]any)
		resetAuthorized := consequenceAuthorized(reset)
		dischargeAuthorized := consequenceAuthorized(discharge)
		proofKey := fmt.Sprintf("%s\x00%s\x00%d\x00%d\x00%d", asString(proof["runId"]), asString(proof["goalId"]),
			goalRevision, obligationRevision, proofEpoch)
		if asString(proof["runId"]) == "" || asString(proof["goalId"]) == "" || !goalRevisionOK || goalRevision < 1 ||
			!obligationRevisionOK || obligationRevision < 1 || !epochOK || proofEpoch < 0 || proofEpoch >= generation ||
			timeErr != nil || !resetOK || !dischargeOK || !resetAuthorized || !dischargeAuthorized || seenProofs[proofKey] {
			return time.Time{}, nil, &BudgetUnknownEvidence{Code: BudgetUnknown, Record: logicalPath, Reason: "consumed-proof ledger has an incomplete or unauthorized entry"}
		}
		seenProofs[proofKey] = true
		if asString(proof["goalId"]) == file.Id && uint64(goalRevision) == file.Claimed.Revision &&
			uint64(obligationRevision) == file.Obligation.Revision && !consumedAt.Before(claimedAt) &&
			(consumedAt.After(latest) || consumedAt.Equal(latest) && uint64(proofEpoch) > latestProofEpoch) {
			latest = consumedAt.UTC()
			latestRunID = asString(proof["runId"])
			latestProofEpoch = uint64(proofEpoch)
		}
	}
	if latestRunID != "" {
		states, err := obligationstate.LoadGoal(repoRoot, file.Id)
		if err != nil {
			return time.Time{}, nil, &BudgetUnknownEvidence{Code: BudgetUnknown, Record: "artifacts/agents/governed-obligations", Reason: err.Error()}
		}
		matched := false
		for _, obligation := range states {
			if obligation.GoalRevision != file.Claimed.Revision || obligation.ObligationRevision != file.Obligation.Revision {
				continue
			}
			for _, attempt := range obligation.Attempts {
				if attempt.RunID == latestRunID && attempt.Status == run.StatusGreen && attempt.Breaker == run.BreakerClosed && !attempt.Exhausted &&
					attempt.WeightGeneration == latestProofEpoch {
					matched = true
				}
			}
		}
		if !matched {
			return time.Time{}, nil, &BudgetUnknownEvidence{Code: BudgetUnknown, Record: logicalPath, Reason: "consumed budget epoch has no exact durable green proof"}
		}
		return latest, &latestProofEpoch, nil
	}
	return latest, nil, nil
}

func consequenceAuthorized(value map[string]any) bool {
	applied, appliedOK := value["apply"].(bool)
	if !appliedOK {
		applied, appliedOK = value["Apply"].(bool)
	}
	wouldRefuse, refuseOK := value["wouldRefuse"].(bool)
	if !refuseOK {
		wouldRefuse, refuseOK = value["WouldRefuse"].(bool)
	}
	return appliedOK && refuseOK && applied && !wouldRefuse
}

func currentWeightGeneration(repoRoot string) (uint64, *BudgetUnknownEvidence) {
	logicalPath := "artifacts/agents/validation-weight.json"
	state, err := readObject(filepath.Join(repoRoot, filepath.FromSlash(logicalPath)))
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, &BudgetUnknownEvidence{Code: BudgetUnknown, Record: logicalPath, Reason: "the validation-weight record is unreadable: " + err.Error()}
	}
	schema, schemaOK := numInt(state["schema"])
	generation, ok := numInt(state["generation"])
	if !schemaOK || schema != 1 || !ok || generation < 0 {
		return 0, &BudgetUnknownEvidence{Code: BudgetUnknown, Record: logicalPath, Reason: "validation-weight schema or generation is missing or invalid"}
	}
	return uint64(generation), nil
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
	budgetStartedAt, weightEpoch, startUnknown := obligationBudgetStart(repoRoot, file, claimedAt)
	if startUnknown != nil {
		return unknownBudget(file.Id, revision, startUnknown.Record, startUnknown.Reason)
	}
	if now.Before(budgetStartedAt) {
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
		Limits: *file.Budget, StartedAt: budgetStartedAt, WeightEpoch: weightEpoch, Elapsed: now.Sub(budgetStartedAt),
		ElapsedGracePercent: gracePercent, ElapsedBreachLimit: breachLimit,
	}
	jobsDir := filepath.Join(repoRoot, "artifacts", "agents", "jobs")
	entries, err := os.ReadDir(jobsDir)
	if err != nil {
		if os.IsNotExist(err) {
			entries = nil
		} else {
			return unknownBudget(file.Id, revision, jobRecordPath(""), "the job-record directory is unreadable: "+err.Error())
		}
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
		if budgetStartedAt.After(claimedAt) {
			startedAt, startErr := time.Parse(time.RFC3339, asString(record["startedAt"]))
			if startErr != nil {
				return unknownBudget(file.Id, revision, logicalPath, "the post-discharge reservation has no readable startedAt")
			}
			if !startedAt.After(budgetStartedAt) {
				continue
			}
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
	type durableAttempt struct {
		attempt obligationstate.TerminalAttempt
		record  string
		seen    bool
	}
	durable := map[string]*durableAttempt{}
	states, stateErr := obligationstate.LoadGoal(repoRoot, file.Id)
	if stateErr != nil {
		return unknownBudget(file.Id, revision, "artifacts/agents/governed-obligations", stateErr.Error())
	}
	for _, state := range states {
		statePath := obligationstate.RelativePath(repoRoot, state)
		if state.GoalRevision > revision {
			return unknownBudget(file.Id, revision, statePath, fmt.Sprintf("goalRevision %d is later than accepted claim revision %d", state.GoalRevision, revision))
		}
		if state.GoalRevision != revision {
			continue
		}
		for _, attempt := range state.Attempts {
			if first := durable[attempt.RunID]; first != nil {
				return unknownBudget(file.Id, revision, statePath, fmt.Sprintf("runId %q duplicates terminal state in %s", attempt.RunID, first.record))
			}
			durable[attempt.RunID] = &durableAttempt{attempt: attempt, record: statePath}
		}
	}
	for _, path := range run.RecordFiles(repoRoot) {
		logicalPath := filepath.ToSlash(strings.TrimPrefix(path, repoRoot+string(filepath.Separator)))
		record, readErr := (&run.Store{Root: repoRoot}).Read(strings.TrimSuffix(filepath.Base(path), ".json"))
		if readErr != nil || record == nil {
			reason := "the governed run record is unreadable"
			if readErr != nil {
				reason += ": " + readErr.Error()
			}
			return unknownBudget(file.Id, revision, logicalPath, reason)
		}
		if record.GoalId != file.Id {
			continue
		}
		if record.Governed == nil {
			return unknownBudget(file.Id, revision, logicalPath, "a goal-bound run has no governed-attempt binding")
		}
		governed := record.Governed
		if governed.GoalRevision > revision {
			return unknownBudget(file.Id, revision, logicalPath, fmt.Sprintf("goalRevision %d is later than accepted claim revision %d", governed.GoalRevision, revision))
		}
		if governed.GoalRevision != revision {
			continue
		}
		if run.Terminal(record.Status) {
			owned := durable[record.RunId]
			if owned == nil {
				return unknownBudget(file.Id, revision, logicalPath, "terminal governed spend has no durable obligation-state owner")
			}
			if reason := terminalStateContradiction(record, owned.attempt); reason != "" {
				return unknownBudget(file.Id, revision, logicalPath, reason)
			}
			owned.seen = true
			continue
		}
		if weightEpoch != nil {
			if !sameUint64(governed.BudgetEpoch, weightEpoch) {
				continue
			}
		} else if governed.BudgetEpoch != nil {
			return unknownBudget(file.Id, revision, logicalPath, "the governed run claims a missing budget epoch")
		} else if budgetStartedAt.After(claimedAt) {
			startedAt, startErr := time.Parse(time.RFC3339, record.StartedAt)
			if startErr != nil {
				return unknownBudget(file.Id, revision, logicalPath, "the governed run has no readable startedAt")
			}
			if !startedAt.After(budgetStartedAt) {
				continue
			}
		}
		cost := governed.ExecutionCostMinutes
		if projection.Attempts == math.MaxUint64 || cost > math.MaxUint64-projection.ReservedJobMinutes {
			return unknownBudget(file.Id, revision, logicalPath, "governed attempt accounting overflowed")
		}
		projection.Attempts++
		projection.ReservedJobMinutes += cost
		if !run.Terminal(record.Status) {
			if projection.ActiveJobs == math.MaxUint64 {
				return unknownBudget(file.Id, revision, logicalPath, "active execution accounting overflowed")
			}
			projection.ActiveJobs++
		}
	}
	for runID, owned := range durable {
		attempt := owned.attempt
		if !owned.seen && attempt.PrunedAt == "" {
			return unknownBudget(file.Id, revision, owned.record,
				fmt.Sprintf("durable obligation state claims unpruned spend for missing run %s", runID))
		}
		startedAt, startErr := time.Parse(time.RFC3339, attempt.StartedAt)
		if startErr != nil {
			return unknownBudget(file.Id, revision, owned.record, fmt.Sprintf("terminal attempt %s has unreadable startedAt", runID))
		}
		if weightEpoch != nil && !sameUint64(attempt.BudgetEpoch, weightEpoch) {
			continue
		}
		if weightEpoch == nil && attempt.BudgetEpoch != nil {
			return unknownBudget(file.Id, revision, owned.record, "terminal attempt claims a missing budget epoch")
		}
		if weightEpoch == nil && budgetStartedAt.After(claimedAt) && !startedAt.After(budgetStartedAt) {
			continue
		}
		if projection.Attempts == math.MaxUint64 || attempt.ObservedCostMinutes > math.MaxUint64-projection.ReservedJobMinutes {
			return unknownBudget(file.Id, revision, owned.record, "governed terminal attempt accounting overflowed")
		}
		projection.Attempts++
		projection.ReservedJobMinutes += attempt.ObservedCostMinutes
	}
	return finishBudgetProjection(projection)
}

func terminalStateContradiction(record *run.Record, attempt obligationstate.TerminalAttempt) string {
	if record == nil || record.Governed == nil || record.EndedAt == nil || record.Governed.ObservedCostMinutes == nil ||
		record.Governed.WeightGeneration == nil {
		return "terminal governed run is missing state needed to reconcile durable obligation spend"
	}
	g := record.Governed
	if attempt.RunID != record.RunId || attempt.Status != record.Status || attempt.StartedAt != record.StartedAt ||
		attempt.EndedAt != *record.EndedAt || attempt.AttemptOrdinal != g.AttemptOrdinal ||
		attempt.ExecutionCostMinutes != g.ExecutionCostMinutes || attempt.ObservedCostMinutes != *g.ObservedCostMinutes ||
		attempt.WeightGeneration != *g.WeightGeneration || !sameUint64(attempt.BudgetEpoch, g.BudgetEpoch) || attempt.Breaker != g.Breaker ||
		attempt.Exhausted != g.Exhausted || attempt.ExhaustionReason != g.ExhaustionReason ||
		attempt.RetroDebtRaised != g.RetroDebtRaised {
		return "terminal run evidence contradicts its durable obligation state"
	}
	return ""
}

func sameUint64(left, right *uint64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
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
