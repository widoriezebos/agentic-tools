package proofrun

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
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
	OnReading       func(reading string)
	OnVerdict       func(verdict string)
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
	startedAt := time.Now()
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
	riseSeconds := consumptionRise(options.Limits.ZeroConsumptionWindow).Seconds()
	if err := command.Start(); err != nil {
		return supervisorOutcome{WaitErr: err}
	}
	outcome := supervisorOutcome{Started: true}
	if source, ok := options.Reader.(interface{ ProgressRuleSuffix() string }); ok {
		outcome.RuleSuffix = source.ProgressRuleSuffix()
	}
	waited := make(chan error, 1)
	go func() { waited <- command.Wait() }()

	ticker := time.NewTicker(options.SampleInterval)
	defer ticker.Stop()
	lastStageSize := int64(0)
	zeroCPUStarted, zeroCPUBase := startedAt, float64(0)
	readerFailures := 0
	lastMembers := []int{command.Process.Pid}
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

	finishWait := func(waitErr error) supervisorOutcome {
		outcome.WaitErr = waitErr
		if command.ProcessState != nil {
			directCPU := command.ProcessState.UserTime().Seconds() + command.ProcessState.SystemTime().Seconds()
			if directCPU > outcome.CPUSeconds {
				outcome.CPUSeconds = directCPU
			}
		}
		now := time.Now()
		updateSupervisorDurations(&outcome, now, options.Activity.Last(), zeroCPUStarted)
		if dumpRequested && outcome.Dump == "" {
			outcome.Dump = "dump: complete"
		}
		return outcome
	}

	for {
		select {
		case <-options.Context.Done():
			killProcessTree(lastMembers, command.Process.Pid)
			outcome.Verdict = "cancelled"
			outcome.Reason = options.Context.Err().Error()
			return finishWait(<-waited)
		case waitErr := <-waited:
			if options.Context.Err() != nil {
				killProcessTree(lastMembers, command.Process.Pid)
				outcome.Verdict = "cancelled"
				outcome.Reason = options.Context.Err().Error()
			}
			return finishWait(waitErr)
		case sampledAt := <-ticker.C:
			if stageResultGrew(options.StageResultPath, &lastStageSize) {
				options.Activity.Mark(sampledAt)
			}
			sample, sampleErr := options.Reader.Sample(command.Process.Pid)
			if sampleErr != nil {
				readerFailures++
				if readerFailures < 3 {
					continue
				}
				recordVerdict("invalid", fmt.Sprintf("process reader failed three consecutive samples: %v", sampleErr))
				killProcessTree(lastMembers, command.Process.Pid)
				return finishWait(<-waited)
			}
			readerFailures = 0
			if len(sample.Members) > 0 {
				lastMembers = append([]int(nil), sample.Members...)
			}
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
					killProcessTree(lastMembers, command.Process.Pid)
					outcome.Dump = "dump: not produced, the process kept computing"
					return finishWait(<-waited)
				}
				if options.Limits.ZeroConsumptionWindow <= 0 {
					killProcessTree(lastMembers, command.Process.Pid)
					outcome.Dump = "dump: killed while writing"
					return finishWait(<-waited)
				}
				quietFor := sampledAt.Sub(dumpStarted)
				if options.Limits.ZeroConsumptionWindow > 0 && outcome.CPUSeconds-dumpCPU < riseSeconds && quietFor >= options.Limits.ZeroConsumptionWindow &&
					sampledAt.Sub(options.Activity.Last()) >= options.Limits.ZeroConsumptionWindow {
					killProcessTree(lastMembers, command.Process.Pid)
					outcome.Dump = "dump: killed while writing"
					return finishWait(<-waited)
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
				signalProcessTree(lastMembers, command.Process.Pid, syscall.SIGQUIT)
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

func signalProcessTree(members []int, rootPID int, signal syscall.Signal) {
	pids := uniqueProcessIDs(members, rootPID)
	for index := len(pids) - 1; index >= 0; index-- {
		if err := syscall.Kill(pids[index], signal); err != nil && !errors.Is(err, syscall.ESRCH) {
			continue
		}
	}
}

func killProcessTree(members []int, rootPID int) {
	signalProcessTree(members, rootPID, syscall.SIGKILL)
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

func parseCPUTime(value string) (float64, error) {
	value = strings.TrimSpace(value)
	days := int64(0)
	if before, after, ok := strings.Cut(value, "-"); ok {
		parsed, err := parsePositiveInt(before)
		if err != nil {
			return 0, fmt.Errorf("parse cputime days: %w", err)
		}
		days, value = parsed, after
	}
	parts := strings.Split(value, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, fmt.Errorf("invalid cputime %q", value)
	}
	seconds, err := parseCPUTimeSeconds(parts[len(parts)-1])
	if err != nil || seconds >= 60 {
		return 0, fmt.Errorf("invalid cputime seconds %q", parts[len(parts)-1])
	}
	minutes, err := parsePositiveInt(parts[len(parts)-2])
	if err != nil || len(parts) == 3 && minutes >= 60 {
		return 0, fmt.Errorf("invalid cputime minutes %q", parts[len(parts)-2])
	}
	hours := int64(0)
	if len(parts) == 3 {
		hours, err = parsePositiveInt(parts[0])
		if err != nil || hours >= 24 && days > 0 {
			return 0, fmt.Errorf("invalid cputime hours %q", parts[0])
		}
	}
	return float64(((days*24)+hours)*60+minutes)*60 + seconds, nil
}

func parseCPUTimeSeconds(value string) (float64, error) {
	whole, fraction, hasFraction := strings.Cut(value, ".")
	seconds, err := parsePositiveInt(whole)
	if err != nil {
		return 0, err
	}
	if !hasFraction {
		return float64(seconds), nil
	}
	if fraction == "" {
		return 0, fmt.Errorf("empty fractional seconds")
	}
	place := 0.1
	result := float64(seconds)
	for _, digit := range fraction {
		if digit < '0' || digit > '9' {
			return 0, fmt.Errorf("non-decimal fractional seconds %q", fraction)
		}
		result += float64(digit-'0') * place
		place /= 10
	}
	return result, nil
}

func parsePositiveInt(value string) (int64, error) {
	var result int64
	if value == "" {
		return 0, fmt.Errorf("empty integer")
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return 0, fmt.Errorf("non-decimal integer %q", value)
		}
		result = result*10 + int64(digit-'0')
	}
	return result, nil
}
