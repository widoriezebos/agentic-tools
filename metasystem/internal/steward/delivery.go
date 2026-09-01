package steward

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

type claimedDeliveryGoal struct {
	id           string
	claimAt      time.Time
	elapsedLimit time.Duration
	elapsedLabel string
	receipt      time.Time
	jobs         []deliveryJob
	failures     []string
}

type deliveryJob struct {
	id          string
	status      string
	errorText   string
	reservedAt  time.Time
	createdAt   time.Time
	endedAt     time.Time
	capMin      int64
	capDeadline time.Time
}

func checkClaimedGoalDelivery(repoRoot string, now time.Time) RoleVerdict {
	if !goal.NewWorld(repoRoot) {
		return roleAlive(RoleClaimedGoalDelivery, "the bootstrap ledger has no claimed-goal delivery records")
	}
	machine, err := goal.ResolveMachine(repoRoot)
	if err != nil {
		return deliveryDead("the claimed-goal machine identity is unreadable: " + err.Error())
	}
	endpoint, err := goal.ResolveEndpoint(repoRoot)
	if err != nil {
		return deliveryDead("the claimed-goal ledger endpoint is unreadable: " + err.Error())
	}
	projection, err := goal.Project(endpoint, false, now)
	if err != nil {
		return deliveryDead("the claimed-goal ledger is unreadable: " + err.Error())
	}

	ids := make([]string, 0, len(projection.Tree.Live))
	for id := range projection.Tree.Live {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	claimed := make(map[string]*claimedDeliveryGoal)
	for _, id := range ids {
		file := projection.Tree.Live[id]
		if file.State != goal.StateClaimed || file.Claimed == nil || file.Claimed.Machine != machine {
			continue
		}
		claimAt, parseErr := time.Parse(time.RFC3339, file.Claimed.At)
		entry := &claimedDeliveryGoal{id: id, claimAt: claimAt}
		if parseErr != nil {
			entry.failures = append(entry.failures, fmt.Sprintf("goal %s has unreadable claim time %q", id, file.Claimed.At))
		}
		if file.Budget == nil {
			entry.failures = append(entry.failures, fmt.Sprintf("goal %s has no elapsed limit", id))
		} else if elapsed, ok := goal.ParseWorkingDuration(file.Budget.ElapsedLimit); !ok {
			entry.failures = append(entry.failures, fmt.Sprintf("goal %s has unreadable elapsed limit %q", id, file.Budget.ElapsedLimit))
		} else {
			entry.elapsedLimit = elapsed
			entry.elapsedLabel = file.Budget.ElapsedLimit
		}
		claimed[id] = entry
	}
	if len(claimed) == 0 {
		return roleAlive(RoleClaimedGoalDelivery, "there are no goals claimed by this machine")
	}

	if err := readLandingReceipts(filepath.Join(repoRoot, "memory", "receipts.log"), claimed); err != nil {
		return deliveryDead(err.Error())
	}
	readDeliveryJobs(repoRoot, claimed)

	normHours, err := config.SliceNormHours(filepath.Join(repoRoot, "metasystem.conf"))
	if err != nil {
		return deliveryDead("the slice norm configuration is unreadable: " + err.Error())
	}
	norm := time.Duration(normHours) * time.Hour
	var dead, alive []string
	for _, id := range ids {
		entry, ok := claimed[id]
		if !ok {
			continue
		}
		if len(entry.failures) > 0 {
			dead = append(dead, entry.failures...)
			continue
		}
		if failure, ok := newestUnrecoveredFailure(*entry); ok {
			dead = append(dead, fmt.Sprintf("goal %s failed without recovery: job %s ended %s ago with error %s",
				id, failure.id, deliveryAge(now, failure.endedAt), failure.errorText))
			continue
		}
		sliceBurned := false
		for _, job := range entry.jobs {
			if deliveryTerminalStatus(job.status) || entry.receipt.After(job.createdAt) {
				continue
			}
			burnBoundary := job.capDeadline.Add(time.Duration(job.capMin) * time.Minute / 2)
			if burnBoundary.Before(now) {
				overrunPercent := float64(now.Sub(job.capDeadline)) / float64(time.Duration(job.capMin)*time.Minute) * 100
				dead = append(dead, fmt.Sprintf("goal %s has burned without delivery: job %s exceeded its own %d-minute budget by %.1f%% without a newer landing receipt",
					id, job.id, job.capMin, overrunPercent))
				sliceBurned = true
				break
			}
		}
		if sliceBurned {
			continue
		}
		if entry.elapsedLimit < norm && !entry.receipt.After(entry.claimAt) {
			claimBoundary := entry.claimAt.Add(entry.elapsedLimit + entry.elapsedLimit/2)
			if claimBoundary.Before(now) {
				dead = append(dead, fmt.Sprintf("goal %s has burned without delivery: claim age %s exceeds 150%% of its own %s elapsed limit",
					id, deliveryAge(now, entry.claimAt), entry.elapsedLabel))
				continue
			}
		}
		if entry.receipt.IsZero() {
			alive = append(alive, fmt.Sprintf("goal %s has no landing receipt yet", id))
		} else {
			alive = append(alive, fmt.Sprintf("goal %s newest delivery evidence is %s old", id, deliveryAge(now, entry.receipt)))
		}
	}
	if len(dead) > 0 {
		return deliveryDead(strings.Join(dead, "; "))
	}
	return roleAlive(RoleClaimedGoalDelivery, strings.Join(alive, "; "))
}

func readLandingReceipts(path string, claimed map[string]*claimedDeliveryGoal) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("the landing receipts log %s is unreadable: %w", path, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		for id, entry := range claimed {
			if !strings.Contains(line, "goal="+id) {
				continue
			}
			epoch, parseErr := strconv.ParseInt(strings.SplitN(line, "|", 2)[0], 10, 64)
			if parseErr != nil {
				entry.failures = append(entry.failures, fmt.Sprintf("goal %s has an unreadable landing receipt in %s", id, path))
				continue
			}
			stamp := time.Unix(epoch, 0).UTC()
			if stamp.After(entry.receipt) {
				entry.receipt = stamp
			}
		}
	}
	return nil
}

func readDeliveryJobs(repoRoot string, claimed map[string]*claimedDeliveryGoal) {
	paths, _ := filepath.Glob(filepath.Join(repoRoot, "artifacts", "agents", "jobs", "*.json"))
	sort.Strings(paths)
	for _, path := range paths {
		value, err := readHealthObject(path)
		if err != nil {
			for _, entry := range claimed {
				entry.failures = append(entry.failures, fmt.Sprintf("goal %s cannot read job record %s: %v", entry.id, path, err))
			}
			continue
		}
		goalID, _ := value["goalId"].(string)
		entry, relevant := claimed[goalID]
		if !relevant {
			continue
		}
		jobID, _ := value["jobId"].(string)
		if jobID == "" {
			jobID = strings.TrimSuffix(filepath.Base(path), ".json")
		}
		status, _ := value["status"].(string)
		job := deliveryJob{id: jobID, status: status}
		job.errorText, _ = value["error"].(string)
		if job.errorText == "" {
			job.errorText = "no error was recorded"
		}
		job.reservedAt, err = deliveryJobReservationTime(value)
		if err != nil {
			entry.failures = append(entry.failures, fmt.Sprintf("goal %s cannot read reservation time in job record %s: %v", goalID, path, err))
			continue
		}
		if status == "failed" {
			job.endedAt, err = deliveryRecordTime(value, "endedAt")
			if err != nil {
				entry.failures = append(entry.failures, fmt.Sprintf("goal %s cannot read failure time in job record %s: %v", goalID, path, err))
				continue
			}
		}
		if !deliveryTerminalStatus(status) {
			job.createdAt, err = deliveryRecordTime(value, "createdAt")
			if err != nil {
				entry.failures = append(entry.failures, fmt.Sprintf("goal %s cannot read creation time in job record %s: %v", goalID, path, err))
				continue
			}
			job.capMin, err = deliveryJobCapMinutes(value)
			if err != nil {
				entry.failures = append(entry.failures, fmt.Sprintf("goal %s has incomplete non-terminal job record %s: %v", goalID, path, err))
				continue
			}
			job.capDeadline, err = deliveryRecordTime(value, "capDeadline")
			if err != nil {
				entry.failures = append(entry.failures, fmt.Sprintf("goal %s has incomplete non-terminal job record %s: %v", goalID, path, err))
				continue
			}
		}
		entry.jobs = append(entry.jobs, job)
	}
}

func deliveryTerminalStatus(status string) bool {
	return status == "completed" || status == "failed" || status == "cancelled"
}

func deliveryJobCapMinutes(value map[string]any) (int64, error) {
	raw, ok := value["capMin"].(json.Number)
	if !ok {
		return 0, fmt.Errorf("capMin is missing")
	}
	minutes, err := strconv.ParseInt(raw.String(), 10, 64)
	if err != nil || minutes <= 0 || minutes > int64(time.Duration(1<<63-1)/time.Minute) {
		return 0, fmt.Errorf("capMin %q is not a positive integer number of minutes", raw)
	}
	return minutes, nil
}

func deliveryJobReservationTime(value map[string]any) (time.Time, error) {
	var newest time.Time
	for _, field := range []string{"createdAt", "startedAt"} {
		raw, present := value[field]
		if !present || raw == nil || raw == "" {
			continue
		}
		stamp, err := deliveryRecordTime(value, field)
		if err != nil {
			return time.Time{}, err
		}
		if stamp.After(newest) {
			newest = stamp
		}
	}
	if newest.IsZero() {
		return time.Time{}, fmt.Errorf("createdAt and startedAt are missing")
	}
	return newest, nil
}

func deliveryRecordTime(value map[string]any, field string) (time.Time, error) {
	raw, ok := value[field].(string)
	if !ok || raw == "" {
		return time.Time{}, fmt.Errorf("%s is missing", field)
	}
	stamp, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s %q is not an RFC 3339 time", field, raw)
	}
	return stamp, nil
}

func newestUnrecoveredFailure(entry claimedDeliveryGoal) (deliveryJob, bool) {
	var newest deliveryJob
	for _, job := range entry.jobs {
		if job.status == "failed" && job.endedAt.After(entry.claimAt) && job.endedAt.After(entry.receipt) && job.endedAt.After(newest.endedAt) {
			newest = job
		}
	}
	return newest, !newest.endedAt.IsZero()
}

func deliveryAge(now, evidence time.Time) time.Duration {
	age := now.Sub(evidence)
	if age < 0 {
		return 0
	}
	return age.Round(time.Minute)
}

func deliveryDead(reason string) RoleVerdict {
	role := roleDead(RoleClaimedGoalDelivery, reason, "inspect the named claim, receipt log, and job record; this health role takes no automatic action")
	role.NoAutomaticRemedy = true
	return role
}
