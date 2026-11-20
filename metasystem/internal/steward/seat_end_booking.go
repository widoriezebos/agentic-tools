package steward

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
)

func seatEndRunner(root, home string, now time.Time, prober identity.Prober) *launch.UnitRunner {
	if prober == nil {
		prober = identity.KernelProber{}
	}
	return &launch.UnitRunner{Root: filepath.Join(home, "unit"), Installation: root, Git: launch.OSGitRunner{}, Manager: &launch.Manager{
		Store: launch.Store{Root: filepath.Join(home, "launch")}, Prober: prober, Processes: launch.OSProcesses{Prober: prober}, Now: func() time.Time { return now },
	}}
}

// BookEndedWork scans the checkout's reservations independently of its current goal
// claim. A paused goal's completed process is still a completed process.
func BookEndedWork(root string, tick time.Duration, runner *launch.UnitRunner) (skipped int) {
	if tick <= 0 {
		return 0
	}
	now := runner.Manager.Now()
	before := now.Add(-tick)
	log := func(message string) {
		if err := logOrphanSeatRun(root, message, now); err != nil {
			skipped++
		}
	}
	note := func(subject string, err error) {
		if launch.IsCode(err, "UNIT_RUN_BUSY") {
			return
		}
		skipped++
		log("end booking left " + subject + ": " + err.Error())
	}
	jobs, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "jobs", "*.json"))
	if err != nil {
		note("job inventory", err)
		return
	}
	// A checkout-local reservation is also the ownership witness for unit
	// launches retained before launch stamps existed.
	reservations := map[string]map[string]any{}
	for _, path := range jobs {
		job, err := dispatch.ReadRecordObject(path)
		if err != nil {
			note(filepath.Base(path), err)
			continue
		}
		id, _ := job["jobId"].(string)
		if id != strings.TrimSuffix(filepath.Base(path), ".json") || job["operationId"] != "unit-launch:"+id {
			continue
		}
		reservations[id] = job
	}
	owns := func(run launch.UnitRunRecord, execution launch.Record) bool {
		return seatOwnsLaunch(root, run, execution, reservations[execution.ID])
	}
	entries, err := os.ReadDir(runner.Root)
	if err != nil && !os.IsNotExist(err) {
		note("run inventory", err)
	}
	localSteps := map[string]bool{}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		run, err := runner.Status(entry.Name())
		if err != nil {
			note("run "+entry.Name(), err)
			continue
		}
		if run.ID != entry.Name() {
			note("run "+entry.Name(), fmt.Errorf("record identity differs from its directory"))
			continue
		}
		owned := false
		for _, round := range run.Rounds {
			for _, step := range round.Steps {
				ids := append(append([]string{}, step.LaunchIDs...), step.LaunchID)
				for _, id := range ids {
					execution, err := runner.Manager.Store.Read(id)
					if err == nil && owns(run, execution) {
						owned = true
						localSteps[id] = true
					}
				}
			}
		}
		if !owned || run.State != "running" && run.State != launch.UnitStepEnded {
			continue
		}
		bookings, err := runner.BookEndedSteps(run.ID, before, owns)
		if err != nil {
			note("run "+run.ID, err)
			continue
		}
		for _, booking := range bookings {
			log(fmt.Sprintf("booked end of run %s (launch %s %s)", run.ID, booking.Launch, booking.State))
		}
	}
	for id, job := range reservations {
		status, _ := job["status"].(string)
		if dispatch.TerminalStatus(status) {
			continue
		}
		run, _ := job["unitRun"].(string)
		goal, _ := job["goalId"].(string)
		execution, err := runner.Manager.Store.Read(id)
		if os.IsNotExist(err) {
			created, _ := job["createdAt"].(string)
			if seatOlderThanTick(created, SeatBusyOptions{Now: now, Tick: tick}) {
				log(fmt.Sprintf("ended reservation %s.json of goal %s (launch %s: no launch record)", id, goal, id))
			}
			continue
		}
		if err != nil {
			note("reservation "+id, err)
			continue
		}
		if execution.State == launch.Starting || execution.State == launch.Running {
			continue
		}
		owned, present := seatLaunchStamp(root, execution, run)
		if !owned && (present || !localSteps[id]) {
			log("end booking left reservation " + id + ": no matching installation stamp")
			continue
		}
		if run == "" || goal == "" || execution.Goal != "" && execution.Goal != goal {
			note("reservation "+id, fmt.Errorf("launch and reservation identities differ"))
			continue
		}
		if err := runner.Manager.FinishedBefore(execution, before); err != nil {
			note("reservation "+id, err)
			continue
		}
		changed, err := dispatch.BookEndedUnitLaunch(root, execution, run, goal, execution.Cause, now)
		if err != nil {
			note("reservation "+id, err)
			continue
		}
		if changed {
			log(fmt.Sprintf("booked end of reservation %s (launch %s %s)", id, id, execution.State))
		}
	}
	return skipped
}

func seatLaunchStamp(root string, execution launch.Record, run string) (owned, present bool) {
	var installation, unit string
	_, hasInstallation := execution.AdapterData["unitInstallation"]
	_, hasRun := execution.AdapterData["unitRun"]
	present = hasInstallation || hasRun
	_ = json.Unmarshal(execution.AdapterData["unitInstallation"], &installation)
	_ = json.Unmarshal(execution.AdapterData["unitRun"], &unit)
	return filepath.IsAbs(installation) && realpath.Resolve(installation) == realpath.Resolve(root) && unit == run, present
}

func seatOwnsLaunch(root string, run launch.UnitRunRecord, execution launch.Record, job map[string]any) bool {
	if execution.Goal != "" && execution.Goal != run.Goal {
		return false
	}
	if owned, present := seatLaunchStamp(root, execution, run.ID); present {
		return owned
	}
	return job != nil && job["jobId"] == execution.ID && job["operationId"] == "unit-launch:"+execution.ID && job["unitRun"] == run.ID && job["goalId"] == run.Goal
}

// UncollectedCheckoutRuns observes the same checkout ownership as end booking,
// including unnamed and paused work. It never books or continues a run.
func UncollectedCheckoutRuns(root string, runner *launch.UnitRunner) ([]UncollectedRun, error) {
	work, unknown, err := runner.GoalRuns("")
	if err != nil {
		return nil, err
	}
	if len(unknown) != 0 {
		return nil, fmt.Errorf("uncollected work: %s", strings.Join(unknown, "; "))
	}
	var runs []UncollectedRun
	for _, item := range work {
		run := *item.Record
		step, execution, orphan := orphanStep(runner.Manager.Store, run, func(ref identity.Ref) identity.Liveness { return identity.AliveRef(identity.KernelProber{}, ref) })
		if !orphan {
			continue
		}
		job, _ := dispatch.ReadRecordObject(filepath.Join(root, "artifacts", "agents", "jobs", execution.ID+".json"))
		if seatOwnsLaunch(root, run, execution, job) {
			runs = append(runs, UncollectedRun{Goal: run.Goal, Run: run.ID, LaunchID: step.LaunchID, Step: step.Name, EndedAt: execution.FinishedAt})
		}
	}
	return runs, nil
}
