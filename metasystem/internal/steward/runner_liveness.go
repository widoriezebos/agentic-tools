package steward

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// RunnerReading is what the steward runner's process and its own published
// progress say about it. Taking the reading holds no lock and writes nothing,
// so a health pass can read the very runner that is publishing at that moment.
type RunnerReading string

const (
	RunnerDead        RunnerReading = "dead"
	RunnerUnknown     RunnerReading = "unknown"
	RunnerFailing     RunnerReading = "failing"
	RunnerStarting    RunnerReading = "starting"
	RunnerWaiting     RunnerReading = "waiting"
	RunnerProgressing RunnerReading = "progressing"
)

// maxRunnerQuietMinutes is the largest threshold a time.Duration can hold.
const maxRunnerQuietMinutes = int64(^uint64(0)>>1) / int64(time.Minute)

// RunnerLivenessReport is one reading of the steward runner.
type RunnerLivenessReport struct {
	Reading  RunnerReading
	Pid      int64
	Detail   string          // one plain sentence for the health row
	Cause    RemedyCause     // set for dead, failing and unknown
	Quiet    bool            // no progress for longer than the quiet threshold
	Progress *RunnerProgress // this runner's own record, when it has one
}

// RunnerLiveness reads the steward runner of repoRoot. now measures durations
// only and may be older than the runner's latest publication.
func RunnerLiveness(repoRoot string, now time.Time) RunnerLivenessReport {
	return runnerLiveness(repoRoot, now, runnerObservedAt(now), identity.KernelProber{})
}

// runnerObservedAt gives the time a progress record is checked against. A
// caller's now can predate a record published while the caller waited for a
// lock, so the clock is read again once the record is in hand.
func runnerObservedAt(now time.Time) func() time.Time {
	return func() time.Time {
		if current := runnerNow(); current.After(now) {
			return current
		}
		return now
	}
}

// runnerLiveness decides in a fixed order and the first match ends it. observed
// runs after the progress record is read, so only a clock that really moved
// back can make the record look newer than the observation.
func runnerLiveness(repoRoot string, now time.Time, observed func() time.Time, prober identity.Prober) RunnerLivenessReport {
	var runner RunnerRecord
	if err := readJSON(runnerRecordPath(repoRoot), &runner); err != nil {
		if os.IsNotExist(err) {
			return RunnerLivenessReport{Reading: RunnerDead, Cause: CauseUnavailable, Detail: "no steward runner is recorded"}
		}
		return RunnerLivenessReport{Reading: RunnerUnknown, Cause: CauseUnreadable, Detail: "the steward runner record is unreadable"}
	}
	report := RunnerLivenessReport{Pid: runner.Pid}
	conclude := func(reading RunnerReading, cause RemedyCause, detail string) RunnerLivenessReport {
		report.Reading, report.Cause, report.Detail = reading, cause, detail
		return report
	}
	// LiveRef reads an exited, unreaped runner as dead: it holds a process
	// table entry and does no work.
	process := identity.Ref{Pid: runner.Pid, StartedAtSec: runner.PidStartedAt, StartTicks: runner.StartTicks, BootID: runner.BootID}
	switch identity.LiveRef(prober, process) {
	case identity.Dead:
		return conclude(RunnerDead, CauseUnavailable, fmt.Sprintf("recorded runner pid %d is dead", runner.Pid))
	case identity.Unknown:
		return conclude(RunnerUnknown, CauseUnreadable, fmt.Sprintf("recorded runner pid %d cannot be inspected", runner.Pid))
	}
	installed, _, err := installedEnrollment(repoRoot)
	if err != nil {
		return conclude(RunnerUnknown, CauseUnreadable, "the steward installation generation is unreadable")
	}
	if runner.Capable != 1 {
		return conclude(RunnerUnknown, CauseObservationPending, fmt.Sprintf("runner pid %d does not publish its progress", runner.Pid))
	}
	quietAfter, failedLimit, err := runnerLivenessSettings(repoRoot)
	if err != nil {
		return conclude(RunnerUnknown, CauseSettingsInvalid, "the steward runner settings are invalid: "+err.Error())
	}
	record, own, err := ownRunnerProgress(repoRoot, runner)
	if err != nil {
		return conclude(RunnerUnknown, CauseUnreadable, "the runner progress record is unreadable")
	}
	if !own || record.Generation != installed.Generation {
		return conclude(RunnerStarting, "", fmt.Sprintf("runner pid %d has started and has not published yet", runner.Pid))
	}
	report.Progress = &record
	if record.At.After(observed()) {
		return conclude(RunnerUnknown, CauseClockRegressed, "CLOCK_REGRESSED: runner progress is later than current UTC")
	}
	if record.State != "working" && record.State != "waiting" {
		return conclude(RunnerUnknown, CauseUnreadable, "the runner progress record is unreadable")
	}
	report.Quiet = runnerQuiet(record, now, quietAfter)
	if record.FailedTicks >= failedLimit {
		return conclude(RunnerFailing, CauseUnavailable, fmt.Sprintf("runner pid %d failed its last %d ticks", runner.Pid, record.FailedTicks))
	}
	detail := runnerActivityText(record) + " for " + runnerAge(now, record.OperationSince)
	if report.Quiet {
		detail += "; quiet for " + runnerAge(now, record.ProgressAt)
	}
	if record.State == "waiting" {
		return conclude(RunnerWaiting, "", detail)
	}
	return conclude(RunnerProgressing, "", detail)
}

// runnerQuiet measures a wait with a deadline from that deadline, because its
// owner expects no progress before then.
func runnerQuiet(record RunnerProgress, now time.Time, after time.Duration) bool {
	since := record.ProgressAt
	if record.State == "waiting" && record.Deadline != nil {
		since = *record.Deadline
	}
	return now.Sub(since) > after
}

// runnerAge is never negative: the record can be newer than the caller's now.
func runnerAge(now, since time.Time) string {
	return max(now.Sub(since), 0).Round(time.Second).String()
}

// runnerLivenessSettings returns the quiet threshold and the failed-tick limit.
// A checkout proven to have no metasystem.conf runs on the compiled defaults,
// because the configuration reader refuses a file it cannot open.
func runnerLivenessSettings(repoRoot string) (time.Duration, int, error) {
	quietMinutes := config.MustIntDefault("steward.runner-quiet-min")
	failedTicks := config.MustIntDefault("steward.runner-failed-ticks")
	if _, statErr := os.Stat(filepath.Join(repoRoot, "metasystem.conf")); statErr == nil || !os.IsNotExist(statErr) {
		var err error
		if quietMinutes, err = boundedConfig(repoRoot, "steward.runner-quiet-min", quietMinutes, 1); err != nil {
			return 0, 0, err
		}
		if failedTicks, err = boundedConfig(repoRoot, "steward.runner-failed-ticks", failedTicks, 1); err != nil {
			return 0, 0, err
		}
	}
	if int64(quietMinutes) > maxRunnerQuietMinutes {
		return 0, 0, fmt.Errorf("steward.runner-quiet-min must be no greater than %d", maxRunnerQuietMinutes)
	}
	return time.Duration(quietMinutes) * time.Minute, failedTicks, nil
}
