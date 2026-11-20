package dispatch

import (
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
)

// UnitLaunchEnded excludes an ended physical launch from the running job projection.
func UnitLaunchEnded(record map[string]any, launchStatus func(string) (string, error)) bool {
	lens := JobRecordOf(record)
	if asString(record["unitRun"]) == "" || lens.OperationID() != "unit-launch:"+lens.JobID() || TerminalStatus(lens.Status()) {
		return false
	}
	if launchStatus == nil {
		launchStatus = func(id string) (string, error) {
			execution, err := (launch.Store{}).Read(id)
			return string(execution.State), err
		}
	}
	state, err := launchStatus(lens.JobID())
	return err == nil && launch.State(state).Terminal()
}

// ReserveUnitLaunch binds a physical execution to its goal revision and unit
// round before it starts. Repeating the same admission joins that reservation.
// A retry is its own attempt, not part of the round's counted one.
func ReserveUnitLaunch(root, id, run string, file *goal.GoalFile, round int, cap int64, now time.Time, retry bool, homes ...string) error {
	return reserveRoundLaunch(root, id, run, file, round, cap, now, retry, "", homes...)
}

// ReserveRoundLaunch charges each physical launch its minutes. Checks and
// reads of one frozen subject occupy one active unit job until the last ends.
func ReserveRoundLaunch(root, id, run string, file *goal.GoalFile, round int, cap int64, now time.Time, retry bool, kind string) error {
	return reserveRoundLaunch(root, id, run, file, round, cap, now, retry, kind)
}

func reserveRoundLaunch(root, id, run string, file *goal.GoalFile, round int, cap int64, now time.Time, retry bool, kind string, homes ...string) error {
	if file == nil || file.Claimed == nil {
		return fmt.Errorf("the unit launch has no claimed goal")
	}
	if file.StopCapability == nil || file.StopCapability.Machine != file.Claimed.Machine || file.StopCapability.Revision != file.Claimed.Revision || file.StopCapability.ClaimEpoch < 1 {
		return fmt.Errorf("the unit launch has no proven claim custody; resume or re-claim it before dispatch")
	}
	goal, revision := file.Id, file.Claimed.Revision
	if id == "" || run == "" || goal == "" || revision == 0 || round < 1 || cap < 1 {
		return fmt.Errorf("the unit launch has no complete reservation identity or cap")
	}
	return withOperationPublicationLock(root, func() error {
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
				joined = !TerminalStatus(asString(record["status"]))
				if !joined && (record["unitExecuted"] != false || record["unitEnvironment"] != true) {
					return fmt.Errorf("the physical launch %s already ended; a retry needs a new launch identity", id)
				}
			}
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			projection := ProjectBudget(root, file, now, homes...)
			remedy := fmt.Sprintf("a person sets its budget: metasystem goal budget %s BOX", goal)
			if projection.Status != BudgetKnown {
				return refusal.New("BUDGET_UNKNOWN", "record="+projection.Unknown.Record+" reason="+projection.Unknown.Reason,
					fmt.Errorf("goal %s has unreadable spending evidence in %s, so this launch does not start; repair that record; %s", goal, projection.Unknown.Record, remedy))
			}
			admission, attempt := projection, fmt.Sprintf("%s/%d", run, round)
			pair := kind == "proof" || kind == "read"
			sharesJob := pair && projection.activeRounds[attempt]
			if retry {
				attempt += "/" + id
			}
			// Joining a pending reservation requests only its existing capacity.
			if joined {
				if projection.unitAttempts[attempt] {
					admission.ReservedJobMinutes -= uint64(cap)
				}
				admission.OpenCapMinutes -= uint64(cap)
			}
			if joined || sharesJob {
				admission.ActiveJobs--
			}
			// A round already counted by another execution uses no new attempt.
			if projection.unitAttempts[attempt] && admission.Attempts <= admission.Limits.AttemptLimit {
				admission.Attempts--
			}
			breaches := budgetAdmissionBreaches(admission)
			breaches = append(breaches, projection.Breaches...)
			beforeElapsedWaiver := len(breaches)
			if projection.unitRunsExecuted[run] {
				breaches = slices.DeleteFunc(breaches, func(b BudgetBreach) bool { return b.Field == "elapsedLimit" })
			}
			elapsedWaived := len(breaches) < beforeElapsedWaiver
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
			reservation := map[string]any{"jobId": id, "operationId": "unit-launch:" + id, "unitRun": run, "unitRound": round,
				"goalId": goal, "goalRevision": revision, "machineId": file.Claimed.Machine, "claimEpoch": file.StopCapability.ClaimEpoch,
				"capMin": cap, "status": "pending", "createdAt": now.UTC().Format(time.RFC3339Nano)}
			if pair {
				reservation["unitRoundRole"] = kind
			}
			if elapsedWaived {
				reservation["elapsedOverrunMin"] = int64(max(0, projection.Elapsed-projection.Limits.ElapsedDuration()) / time.Minute)
			}
			if retry {
				reservation["unitRetryAttempt"] = true
			}
			return writeRecord(path, reservation)
		})
	})
}

// ReconcileUnitLaunch replaces the reservation's charge evidence, rather than
// subtracting a refund. An environment exclusion survives every later collection.
func ReconcileUnitLaunch(root, id, run, goal string, revision uint64, status, cause, started, ended string, executed bool) error {
	_, err := reconcileUnitLaunch(root, id, run, goal, revision, status, cause, started, ended, executed, nil)
	return err
}

// BookEndedUnitLaunch settles only an existing open reservation. A person's
// terminal decision and an earlier collection both win over this observation.
func BookEndedUnitLaunch(root string, execution launch.Record, run, goal, cause string, now time.Time) (bool, error) {
	return reconcileUnitLaunch(root, execution.ID, run, goal, 0, string(execution.State), cause, execution.StartedAt, execution.FinishedAt, execution.Executed(), &now)
}

func reconcileUnitLaunch(root, id, run, goal string, revision uint64, status, cause, started, ended string, executed bool, bookedAt *time.Time) (bool, error) {
	if !TerminalStatus(status) {
		return false, fmt.Errorf("the unit execution %s has no terminal outcome", id)
	}
	changed := false
	err := withRecordLock(root, id, func(path string) error {
		record, err := readObject(path)
		if bookedAt != nil {
			if err != nil {
				return err
			}
			if TerminalStatus(asString(record["status"])) {
				return nil
			}
			if asString(record["operationId"]) != "unit-launch:"+id || asString(record["status"]) != "pending" && asString(record["status"]) != "running" {
				return fmt.Errorf("the unit execution %s has no open reservation", id)
			}
		}
		if os.IsNotExist(err) {
			// Unreserved history is bound to the observed goal revision, but
			// supplies no approved spending cap or live process custody.
			if revision == 0 {
				return fmt.Errorf("the unreserved unit execution %s has no observed goal revision", id)
			}
			record = map[string]any{"jobId": id, "operationId": "unit-launch:" + id, "unitRun": run, "goalId": goal, "goalRevision": revision, "unitUnaccounted": true}
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
		// A takeover's sweep failed this reservation before sweeps left unit
		// reservations alone; it never started, so its real outcome replaces it.
		swept := TerminalStatus(asString(record["status"])) && asString(record["phase"]) == "claim-sweep" && asString(record["startedAt"]) == ""
		if swept {
			delete(record, "phase")
			delete(record, "error")
		}
		if _, owned := record["unitExecuted"]; owned && TerminalStatus(asString(record["status"])) && !swept {
			if asString(record["status"]) != status || record["unitExecuted"] != executed || executed && (asString(record["startedAt"]) != started || asString(record["endedAt"]) != ended) {
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
		if bookedAt != nil {
			record["endBookedBy"], record["endBookedAt"], record["endBookedLaunch"] = "steward", bookedAt.UTC().Format(time.RFC3339Nano), id
		}
		if err := writeRecord(path, record); err != nil {
			return err
		}
		changed = true
		return nil
	})
	return changed, err
}
