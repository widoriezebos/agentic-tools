package dispatch

import (
	"fmt"
	"os"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
)

// ReserveUnitLaunch binds a physical execution to its goal revision and unit
// round before it starts. Repeating the same admission joins that reservation.
func ReserveUnitLaunch(root, id, run string, file *goal.GoalFile, round int, cap int64, now time.Time) error {
	if file == nil || file.Claimed == nil {
		return fmt.Errorf("the unit launch has no claimed goal")
	}
	goal, revision := file.Id, file.Claimed.Revision
	if id == "" || run == "" || goal == "" || revision == 0 || round < 1 || cap < 1 {
		return fmt.Errorf("the unit launch has no complete reservation identity or cap")
	}
	return withRecordLock(root, id, func(path string) error {
		joined := false
		record, err := readObject(path)
		if err == nil {
			rev, _ := numInt(record["goalRevision"])
			n, _ := numInt(record["unitRound"])
			minutes, _ := numInt(record["capMin"])
			if asString(record["jobId"]) != id || asString(record["unitRun"]) != run || asString(record["goalId"]) != goal || uint64(rev) != revision || int(n) != round || minutes != cap {
				return fmt.Errorf("the physical launch %s already belongs to a different reservation", id)
			}
			if TerminalStatus(asString(record["status"])) {
				return fmt.Errorf("the physical launch %s already ended; a retry needs a new launch identity", id)
			}
			joined = true
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		projection := ProjectBudget(root, file, now)
		remedy := fmt.Sprintf("a person sets its budget: metasystem goal budget %s BOX", goal)
		if projection.Status != BudgetKnown {
			return refusal.New("BUDGET_UNKNOWN", "record="+projection.Unknown.Record+" reason="+projection.Unknown.Reason,
				fmt.Errorf("goal %s has unreadable spending evidence in %s, so this launch does not start; repair that record; %s", goal, projection.Unknown.Record, remedy))
		}
		admission := projection
		// Joining a pending reservation requests only its existing capacity.
		if joined {
			if projection.unitAttempts[fmt.Sprintf("%s/%d", run, round)] {
				admission.ReservedJobMinutes -= uint64(cap)
			}
			admission.OpenCapMinutes -= uint64(cap)
			admission.ActiveJobs--
		}
		// A round already counted by another execution uses no new attempt.
		if projection.unitAttempts[fmt.Sprintf("%s/%d", run, round)] && admission.Attempts <= admission.Limits.AttemptLimit {
			admission.Attempts--
		}
		breaches := budgetAdmissionBreaches(admission)
		breaches = append(breaches, projection.Breaches...)
		if admission.ReservedJobMinutes < admission.Limits.ReservedJobMinutesLimit && uint64(cap) > admission.Limits.ReservedJobMinutesLimit-admission.ReservedJobMinutes {
			breaches = append(breaches, BudgetBreach{Field: "reservedJobMinutesLimit", Used: fmt.Sprintf("%d+%d proposed", admission.ReservedJobMinutes, cap), Limit: fmt.Sprint(admission.Limits.ReservedJobMinutesLimit)})
		}
		if file.StopFence != nil || len(breaches) > 0 {
			return refusal.New("BUDGET_REFUSED", formatRefusalDetail(breaches, reservedMinutesEvidence(projection)),
				fmt.Errorf("goal %s has no budget for this launch, so it does not start; %s", goal, remedy))
		}
		if joined {
			return nil
		}
		return writeRecord(path, map[string]any{"jobId": id, "operationId": "unit-launch:" + id, "unitRun": run, "unitRound": round,
			"goalId": goal, "goalRevision": revision, "capMin": cap, "status": "pending", "createdAt": now.UTC().Format(time.RFC3339Nano)})
	})
}

// ReconcileUnitLaunch replaces the reservation's charge evidence, rather than
// subtracting a refund. An environment exclusion survives every later collection.
func ReconcileUnitLaunch(root, id, run, goal, status, cause, started, ended string, executed bool) error {
	if !TerminalStatus(status) {
		return fmt.Errorf("the unit execution %s has no terminal outcome", id)
	}
	return withRecordLock(root, id, func(path string) error {
		record, err := readObject(path)
		if os.IsNotExist(err) {
			// An execution admitted without a reservation supplies history,
			// but cannot supply a goal revision or an approved spending cap.
			record = map[string]any{"jobId": id, "operationId": "unit-launch:" + id, "unitRun": run, "goalId": goal, "unitUnaccounted": true}
		} else if err != nil {
			return err
		}
		if asString(record["jobId"]) != id || asString(record["unitRun"]) != run || asString(record["goalId"]) != goal {
			return fmt.Errorf("the unit execution %s does not match its reservation", id)
		}
		start, startErr := time.Parse(time.RFC3339Nano, started)
		end, endErr := time.Parse(time.RFC3339Nano, ended)
		if startErr != nil || endErr != nil || end.Before(start) {
			return fmt.Errorf("the unit execution %s has unreadable or regressed execution times", id)
		}
		if TerminalStatus(asString(record["status"])) {
			if asString(record["status"]) != status || asString(record["startedAt"]) != started || asString(record["endedAt"]) != ended || record["unitExecuted"] != executed {
				return fmt.Errorf("the unit execution %s contradicts its terminal reservation", id)
			}
			if cause != "environment" || record["unitEnvironment"] == true {
				return nil
			}
		}
		record["status"], record["startedAt"], record["endedAt"], record["unitExecuted"] = status, started, ended, executed
		if cause == "environment" {
			record["unitEnvironment"] = true
		}
		return writeRecord(path, record)
	})
}
