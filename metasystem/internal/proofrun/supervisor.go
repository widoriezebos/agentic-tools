package proofrun

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

const (
	zeroConsumptionWindow    = 30 * time.Minute
	supervisorSampleInterval = 10 * time.Second
)

// supervisorLimitsForTest is the only package-level test override. Its zero
// value selects the group's production CPU budget and the production zero-CPU
// window.
var supervisorLimitsForTest supervisorLimits

type supervisorLimits struct {
	CPUBudgetSeconds      int64
	ZeroConsumptionWindow time.Duration
}

type processTreeSample struct {
	CPUSeconds            float64
	Members               []int
	MemberCPU             []processMemberCPU
	RetainVanishedMembers bool
	Stopped               bool
	Waiting               bool
	Partial               bool
}

type processMemberCPU struct {
	PID        int
	Started    string
	CPUSeconds float64
}

type processMemberIdentity struct {
	PID     int
	Started string
}

type processTreeReader interface {
	Sample(rootPID int) (processTreeSample, error)
}

type supervisorOptions struct {
	Context         context.Context
	Limits          supervisorLimits
	SampleInterval  time.Duration
	Activity        *outputActivity
	StageResultPath string
	Reader          processTreeReader
	Prober          identity.Prober
	Signal          identity.SignalFunc
	OnReading       func(reading string)
	OnVerdict       func(verdict string)
	Now             func() time.Time
	NewTicker       func(time.Duration) (<-chan time.Time, func())
	WaitCommand     func() error
	OnCancelSelect  func()
}

type supervisorOutcome struct {
	Verdict               string
	Reason                string
	Dump                  string
	CPUSeconds            float64
	LongestSilentSeconds  int64
	LongestZeroCPUSeconds int64
	Started               bool
	WaitErr               error
	RuleSuffix            string
}

type outputActivity struct {
	mu   sync.Mutex
	last time.Time
}

func newOutputActivity(now time.Time) *outputActivity {
	return &outputActivity{last: now}
}

func (activity *outputActivity) Mark(now time.Time) {
	activity.mu.Lock()
	if now.After(activity.last) {
		activity.last = now
	}
	activity.mu.Unlock()
}

func (activity *outputActivity) Last() time.Time {
	activity.mu.Lock()
	defer activity.mu.Unlock()
	return activity.last
}

type activityWriter struct {
	activity *outputActivity
	writer   io.Writer
}

func (writer *activityWriter) Write(data []byte) (int, error) {
	n, err := writer.writer.Write(data)
	if n > 0 {
		writer.activity.Mark(time.Now())
	}
	return n, err
}

type synchronizedBuffer struct {
	mu   sync.Mutex
	data []byte
}

func (buffer *synchronizedBuffer) Write(data []byte) (int, error) {
	buffer.mu.Lock()
	buffer.data = append(buffer.data, data...)
	buffer.mu.Unlock()
	return len(data), nil
}

func (buffer *synchronizedBuffer) Bytes() []byte {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return append([]byte(nil), buffer.data...)
}

func (buffer *synchronizedBuffer) String() string {
	return string(buffer.Bytes())
}

func productionSupervisorLimits(cpuBudget *int64) supervisorLimits {
	limits := supervisorLimits{ZeroConsumptionWindow: zeroConsumptionWindow}
	if cpuBudget != nil {
		limits.CPUBudgetSeconds = *cpuBudget
	}
	return limits
}

func groupSupervisorSettings(cpuBudget *int64) (supervisorLimits, time.Duration) {
	limits := productionSupervisorLimits(cpuBudget)
	interval := supervisorSampleInterval
	if supervisorLimitsForTest.CPUBudgetSeconds > 0 {
		limits.CPUBudgetSeconds = supervisorLimitsForTest.CPUBudgetSeconds
	}
	if supervisorLimitsForTest.ZeroConsumptionWindow > 0 {
		limits.ZeroConsumptionWindow = supervisorLimitsForTest.ZeroConsumptionWindow
		interval = limits.ZeroConsumptionWindow / 4
		if interval < 10*time.Millisecond {
			interval = 10 * time.Millisecond
		}
		if interval > supervisorSampleInterval {
			interval = supervisorSampleInterval
		}
	}
	return limits, interval
}

func progressRule(limits supervisorLimits) string {
	budget := "none"
	if limits.CPUBudgetSeconds > 0 {
		budget = fmt.Sprintf("%ds", limits.CPUBudgetSeconds)
	}
	window := "none"
	if limits.ZeroConsumptionWindow > 0 {
		if limits.ZeroConsumptionWindow%time.Minute == 0 {
			window = fmt.Sprintf("%dm", limits.ZeroConsumptionWindow/time.Minute)
		} else {
			window = limits.ZeroConsumptionWindow.String()
		}
	}
	return "cpu-budget/" + budget + "+zero-window/" + window
}

func consumptionRise(window time.Duration) time.Duration {
	rise := window / 1800
	if rise < 10*time.Millisecond {
		return 10 * time.Millisecond
	}
	return rise
}

func superviseCommand(command *exec.Cmd, options supervisorOptions) supervisorOutcome {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	newTicker := options.NewTicker
	if newTicker == nil {
		newTicker = func(interval time.Duration) (<-chan time.Time, func()) {
			ticker := time.NewTicker(interval)
			return ticker.C, ticker.Stop
		}
	}
	startedAt := now()
	if options.Activity == nil {
		options.Activity = newOutputActivity(startedAt)
	}
	if options.SampleInterval <= 0 {
		options.SampleInterval = supervisorSampleInterval
	}
	if options.Reader == nil {
		options.Reader = newProcessTreeReader()
	}
	if options.Context == nil {
		options.Context = context.Background()
	}
	if options.Prober == nil {
		options.Prober = identity.KernelProber{}
	}
	if options.Signal == nil {
		options.Signal = syscall.Kill
	}
	riseSeconds := consumptionRise(options.Limits.ZeroConsumptionWindow).Seconds()
	finishCustody, signalCustody, rootRef, err := startResourceCommand(options.Context, command, HostResourceLeaseFromContext(options.Context))
	if err != nil {
		return supervisorOutcome{WaitErr: err}
	}
	outcome := supervisorOutcome{Started: true}
	if source, ok := options.Reader.(interface{ ProgressRuleSuffix() string }); ok {
		outcome.RuleSuffix = source.ProgressRuleSuffix()
	}
	waitCommand := options.WaitCommand
	if waitCommand == nil {
		waitCommand = command.Wait
	}
	waited := make(chan error, 1)
	go func() { waited <- waitCommand() }()

	ticks, stopTicker := newTicker(options.SampleInterval)
	defer stopTicker()
	lastStageSize := int64(0)
	zeroCPUStarted, zeroCPUBase := startedAt, float64(0)
	readerFailures := 0
	dumpRequested := false
	dumpCPU := float64(0)
	dumpStarted := time.Time{}
	currentReading := ""
	wasBlocked := false
	memberHighWater := map[processMemberIdentity]float64{}
	recordVerdict := func(verdict, reason string) {
		outcome.Verdict = verdict
		outcome.Reason = reason
		if options.OnVerdict != nil {
			options.OnVerdict(verdict)
		}
	}
	recordReading := func(reading string) {
		currentReading = reading
		if options.OnReading != nil {
			options.OnReading(reading)
		}
	}
	stopAndWait := func() error {
		// The still-live pre-birth custodian owns group discovery and exact
		// signaling. Asking it to finish before waiting lets it close output
		// held by a descendant without ever adopting a saved numeric group.
		custodyErr := finishCustody()
		return errors.Join(<-waited, custodyErr)
	}

	finishWait := func(waitErr error) supervisorOutcome {
		outcome.WaitErr = waitErr
		if command.ProcessState != nil {
			directCPU := command.ProcessState.UserTime().Seconds() + command.ProcessState.SystemTime().Seconds()
			if directCPU > outcome.CPUSeconds {
				outcome.CPUSeconds = directCPU
			}
		}
		finishedAt := now()
		updateSupervisorDurations(&outcome, finishedAt, options.Activity.Last(), zeroCPUStarted)
		if dumpRequested && outcome.Dump == "" {
			outcome.Dump = "dump: complete"
		}
		return outcome
	}

	for {
		select {
		case <-options.Context.Done():
			if options.OnCancelSelect != nil {
				options.OnCancelSelect()
			}
			// The pre-birth custodian drains the group even when cancellation arrives
			// before the first periodic sample discovers a descendant.
			outcome.Verdict = "cancelled"
			outcome.Reason = options.Context.Err().Error()
			return finishWait(stopAndWait())
		case waitErr := <-waited:
			if options.Context.Err() != nil {
				outcome.Verdict = "cancelled"
				outcome.Reason = options.Context.Err().Error()
			}
			return finishWait(errors.Join(waitErr, finishCustody()))
		case sampledAt := <-ticks:
			if stageResultGrew(options.StageResultPath, &lastStageSize) {
				options.Activity.Mark(sampledAt)
			}
			// The numeric root is only a lookup key while it still denotes the
			// exact worker bound to the live custodian. A reaped or uninspectable
			// root cannot authorize another tree discovery.
			if identity.AliveRef(options.Prober, rootRef) != identity.Alive {
				continue
			}
			sample, sampleErr := options.Reader.Sample(command.Process.Pid)
			if sampleErr != nil {
				readerFailures++
				if readerFailures < 3 {
					continue
				}
				recordVerdict("invalid", fmt.Sprintf("process reader failed three consecutive samples: %v", sampleErr))
				return finishWait(stopAndWait())
			}
			readerFailures = 0
			sampleCPU := sample.CPUSeconds
			if sample.MemberCPU != nil {
				sampleCPU = 0
				if sample.RetainVanishedMembers {
					for _, member := range sample.MemberCPU {
						identity := processMemberIdentity{PID: member.PID, Started: member.Started}
						if member.CPUSeconds > memberHighWater[identity] {
							memberHighWater[identity] = member.CPUSeconds
						}
					}
					for _, cpu := range memberHighWater {
						sampleCPU += cpu
					}
				} else {
					for _, member := range sample.MemberCPU {
						sampleCPU += member.CPUSeconds
					}
				}
			}
			if sampleCPU > outcome.CPUSeconds {
				outcome.CPUSeconds = sampleCPU
			}
			if outcome.CPUSeconds-zeroCPUBase >= riseSeconds {
				zeroCPUBase = outcome.CPUSeconds
				zeroCPUStarted = sampledAt
			}
			updateSupervisorDurations(&outcome, sampledAt, options.Activity.Last(), zeroCPUStarted)
			blockedNow := sample.Stopped || sample.Waiting
			if wasBlocked && !blockedNow {
				zeroCPUBase = outcome.CPUSeconds
				zeroCPUStarted = sampledAt
			}
			wasBlocked = blockedNow
			madeProgress := sampledAt.Sub(zeroCPUStarted) < options.SampleInterval || sampledAt.Sub(options.Activity.Last()) < options.Limits.ZeroConsumptionWindow
			if currentReading != "" && (madeProgress || currentReading == "stopped" && !sample.Stopped || currentReading == "waiting on the host" && !sample.Waiting) {
				currentReading = ""
				zeroCPUBase = outcome.CPUSeconds
				zeroCPUStarted = sampledAt
			}

			if dumpRequested {
				if outcome.CPUSeconds-dumpCPU >= riseSeconds {
					outcome.Dump = "dump: not produced, the process kept computing"
					custodyErr := finishCustody()
					return finishWait(errors.Join(<-waited, custodyErr))
				}
				if options.Limits.ZeroConsumptionWindow <= 0 {
					outcome.Dump = "dump: killed while writing"
					custodyErr := finishCustody()
					return finishWait(errors.Join(<-waited, custodyErr))
				}
				quietFor := sampledAt.Sub(dumpStarted)
				if options.Limits.ZeroConsumptionWindow > 0 && outcome.CPUSeconds-dumpCPU < riseSeconds && quietFor >= options.Limits.ZeroConsumptionWindow &&
					sampledAt.Sub(options.Activity.Last()) >= options.Limits.ZeroConsumptionWindow {
					outcome.Dump = "dump: killed while writing"
					custodyErr := finishCustody()
					return finishWait(errors.Join(<-waited, custodyErr))
				}
				continue
			}

			if options.Limits.CPUBudgetSeconds > 0 && outcome.CPUSeconds >= float64(options.Limits.CPUBudgetSeconds) {
				recordVerdict("runaway", fmt.Sprintf("cpu budget exhausted: %.2fs of %ds", outcome.CPUSeconds, options.Limits.CPUBudgetSeconds))
			} else if options.Limits.ZeroConsumptionWindow > 0 &&
				sampledAt.Sub(zeroCPUStarted) >= options.Limits.ZeroConsumptionWindow &&
				sampledAt.Sub(options.Activity.Last()) >= options.Limits.ZeroConsumptionWindow {
				switch {
				case sample.Stopped:
					if currentReading != "stopped" {
						recordReading("stopped")
					}
					continue
				case sample.Waiting:
					if currentReading != "waiting on the host" {
						recordReading("waiting on the host")
					}
					continue
				default:
					recordVerdict("dead", fmt.Sprintf("no CPU or output progress for %s", options.Limits.ZeroConsumptionWindow))
				}
			}
			if outcome.Verdict == "runaway" || outcome.Verdict == "dead" {
				signalCustody(syscall.SIGQUIT, options.Prober, options.Signal)
				dumpRequested = true
				dumpCPU = outcome.CPUSeconds
				dumpStarted = sampledAt
			}
		}
	}
}

func updateSupervisorDurations(outcome *supervisorOutcome, now, outputAt, zeroCPUAt time.Time) {
	if silent := int64(now.Sub(outputAt).Seconds()); silent > outcome.LongestSilentSeconds {
		outcome.LongestSilentSeconds = silent
	}
	if zero := int64(now.Sub(zeroCPUAt).Seconds()); zero > outcome.LongestZeroCPUSeconds {
		outcome.LongestZeroCPUSeconds = zero
	}
}

func stageResultGrew(path string, previous *int64) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	current := info.Size()
	grew := current > *previous
	*previous = current
	return grew
}

func containsExactEnvironmentEntry(environment []string, wanted string) bool {
	for _, entry := range environment {
		if entry == wanted {
			return true
		}
	}
	return false
}

func uniqueProcessIDs(members []int, rootPID int) []int {
	seen := map[int]bool{}
	for _, pid := range append(append([]int(nil), members...), rootPID) {
		if pid > 0 {
			seen[pid] = true
		}
	}
	result := make([]int, 0, len(seen))
	for pid := range seen {
		result = append(result, pid)
	}
	sort.Ints(result)
	return result
}
