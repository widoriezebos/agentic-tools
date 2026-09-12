package proofrun

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestSupervisorProductionLimitsAndGoTimeoutDisabled(t *testing.T) {
	supervisorLimitsForTest = supervisorLimits{}
	if (supervisorOptions{}).OnReading != nil {
		t.Fatal("supervisor options installed an unexpected reading callback")
	}
	budget := int64(37)
	limits, interval := groupSupervisorSettings(&budget)
	if limits.CPUBudgetSeconds != budget || limits.ZeroConsumptionWindow != 30*time.Minute || interval != 10*time.Second {
		t.Fatalf("production limits = %+v interval=%s", limits, interval)
	}
	if rise := consumptionRise(limits.ZeroConsumptionWindow); rise != time.Second {
		t.Fatalf("production consumption rise = %s, want 1s", rise)
	}
	if rise := consumptionRise(400 * time.Millisecond); rise != 10*time.Millisecond {
		t.Fatalf("shortened-window consumption rise = %s, want 10ms", rise)
	}
	activity := newOutputActivity(time.Now())
	latest := activity.Last()
	activity.Mark(latest.Add(-time.Second))
	if got := activity.Last(); !got.Equal(latest) {
		t.Fatalf("output activity moved backwards from %s to %s", latest, got)
	}
	root := t.TempDir()
	writeTestResultFile(t, filepath.Join(root, "go.mod"), []byte("module example.invalid/timeoutless\n\ngo 1.22\n"), 0o644)
	writeTestResultFile(t, filepath.Join(root, "sample_test.go"), []byte("package timeoutless\nimport \"testing\"\nfunc TestSample(t *testing.T) {}\n"), 0o644)
	group := testpolicy.Group{Adapter: "go", Packages: []string{"."}, Tests: []byte(`"all"`)}
	args, _, _, _, err := goArguments(context.Background(), group, root, os.Environ())
	if err != nil || len(args) < 6 || args[3] != "-count=1" || args[4] != "-"+"timeout" || args[5] != "0" {
		t.Fatalf("Go argv = %v, err=%v", args, err)
	}
}

func TestSupervisorLegitimateWaitDeadDumpAndReaderFailures(t *testing.T) {
	t.Run("legitimate wait shorter than window passes", func(t *testing.T) {
		helper := newSupervisorHelperFixture(t, "idle")
		options := supervisorOptionsForTest(t, 0, 150*time.Millisecond)
		helper.gate(&options, &scriptedTreeReader{samples: []processTreeSample{{}, {}, {}, {}}}, func(call int, _ processTreeSample) {
			if call == 2 {
				helper.closeInput()
			}
		})
		outcome := superviseCommand(helper.command, options)
		if outcome.Verdict != "" || outcome.WaitErr != nil {
			t.Fatalf("legitimate wait outcome=%+v", outcome)
		}
	})
	t.Run("dead process dumps and exits", func(t *testing.T) {
		helper := newSupervisorHelperFixture(t, "idle")
		reader := &scriptedTreeReader{samples: []processTreeSample{{}, {}, {}, {}}}
		options := supervisorOptionsForTest(t, 0, 40*time.Millisecond)
		verdicts := []string{}
		options.OnVerdict = func(verdict string) {
			verdicts = append(verdicts, verdict)
			helper.keepAliveUntilExit()
		}
		helper.gate(&options, reader, nil)
		outcome := superviseCommand(helper.command, options)
		if outcome.Verdict != "dead" || outcome.Dump != "dump: complete" || !strings.Contains(outcome.Reason, "no CPU or output progress") || len(verdicts) != 1 || verdicts[0] != "dead" {
			t.Fatalf("dead outcome=%+v", outcome)
		}
		log := helper.log.String()
		if !strings.Contains(log, "goroutine ") || !strings.Contains(log, "SIGQUIT") {
			t.Fatalf("dead helper dump omitted Go SIGQUIT goroutines: %q", log)
		}
	})
	t.Run("three outright failures are invalid", func(t *testing.T) {
		helper := newSupervisorHelperFixture(t, "idle")
		reader := &scriptedTreeReader{fail: fmt.Errorf("synthetic reader failure")}
		options := supervisorOptionsForTest(t, 0, 40*time.Millisecond)
		verdicts := []string{}
		options.OnVerdict = func(verdict string) { verdicts = append(verdicts, verdict) }
		helper.gate(&options, reader, nil)
		outcome := superviseCommand(helper.command, options)
		if outcome.Verdict != "invalid" || !strings.Contains(outcome.Reason, "three consecutive") || len(verdicts) != 1 || verdicts[0] != "invalid" {
			t.Fatalf("reader-failure outcome=%+v", outcome)
		}
	})
	t.Run("nil verdict callback preserves quiet dump behavior", func(t *testing.T) {
		helper := newSupervisorHelperFixture(t, "ignore-quit-idle")
		options := supervisorOptionsForTest(t, 0, 40*time.Millisecond)
		if options.OnVerdict != nil {
			t.Fatal("supervisor options installed an unexpected verdict callback")
		}
		helper.gate(&options, &scriptedTreeReader{samples: []processTreeSample{{}, {}, {}, {}, {}, {}, {}, {}}}, nil)
		outcome := superviseCommand(helper.command, options)
		if outcome.Verdict != "dead" || outcome.Dump != "dump: killed while writing" {
			t.Fatalf("quiet dump outcome=%+v", outcome)
		}
	})
	t.Run("Darwin retention registers a surviving member rise", func(t *testing.T) {
		helper := newSupervisorHelperFixture(t, "idle")
		reader := &scriptedTreeReader{samples: []processTreeSample{
			{MemberCPU: []processMemberCPU{{PID: 71, Started: "vanishing", CPUSeconds: 1}, {PID: 72, Started: "root", CPUSeconds: 0}}, RetainVanishedMembers: true},
			{MemberCPU: []processMemberCPU{{PID: 72, Started: "root", CPUSeconds: 0}}, RetainVanishedMembers: true},
			{MemberCPU: []processMemberCPU{{PID: 72, Started: "root", CPUSeconds: 0}}, RetainVanishedMembers: true},
			{MemberCPU: []processMemberCPU{{PID: 72, Started: "root", CPUSeconds: 0}}, RetainVanishedMembers: true},
			{MemberCPU: []processMemberCPU{{PID: 72, Started: "root", CPUSeconds: 0.01}}, RetainVanishedMembers: true},
			{MemberCPU: []processMemberCPU{{PID: 72, Started: "root", CPUSeconds: 0.01}}, RetainVanishedMembers: true},
		}}
		options := supervisorOptionsForTest(t, 0, 40*time.Millisecond)
		rootRose, closeAfterRise := false, false
		helper.gate(&options, reader, func(_ int, sample processTreeSample) {
			if closeAfterRise {
				helper.closeInput()
				return
			}
			for _, member := range sample.MemberCPU {
				if member.Started == "root" && member.CPUSeconds >= 0.01 {
					rootRose = true
					closeAfterRise = true
				}
			}
		})
		outcome := superviseCommand(helper.command, options)
		if outcome.Verdict != "" || !rootRose || outcome.CPUSeconds < 1.01 {
			t.Fatalf("retained member did not preserve the later root rise: outcome=%+v rootRose=%v", outcome, rootRose)
		}
	})
	t.Run("non-retaining samples do not sum vanished members", func(t *testing.T) {
		helper := newSupervisorHelperFixture(t, "idle")
		reader := &scriptedTreeReader{samples: []processTreeSample{
			{MemberCPU: []processMemberCPU{{PID: 71, Started: "vanished", CPUSeconds: 4}}},
			{MemberCPU: []processMemberCPU{{PID: 72, Started: "live", CPUSeconds: 1}}},
		}}
		options := supervisorOptionsForTest(t, 0, 120*time.Millisecond)
		helper.gate(&options, reader, func(call int, _ processTreeSample) {
			if call == 2 {
				helper.closeInput()
			}
		})
		outcome := superviseCommand(helper.command, options)
		if outcome.Verdict != "" || outcome.CPUSeconds != 4 {
			t.Fatalf("non-retaining sample summed a vanished member: %+v", outcome)
		}
	})
}

func TestSupervisorHostWaitReadingIsNonterminalAndResetsWindow(t *testing.T) {
	helper := newSupervisorHelperFixture(t, "idle")
	reported := false
	reader := &readingUntilReportedTreeReader{reported: &reported, waiting: true}
	readings := make(chan string, 1)
	options := supervisorOptionsForTest(t, 0, 40*time.Millisecond)
	helper.gate(&options, reader, func(_ int, sample processTreeSample) {
		if reported && !sample.Waiting {
			helper.closeInput()
		}
	})
	options.OnReading = func(reading string) {
		reported = true
		readings <- reading
	}
	outcome := superviseCommand(helper.command, options)
	select {
	case state := <-readings:
		if state != "waiting on the host" {
			t.Fatalf("host wait reading=%q", state)
		}
	default:
		t.Fatal("host wait was not reported")
	}
	if outcome.Verdict != "" || outcome.WaitErr != nil {
		t.Fatalf("resumed host wait outcome=%+v", outcome)
	}
}

func TestSupervisorStoppedProcessIsReportedLeftAliveAndResumes(t *testing.T) {
	if _, err := processStoppedForTest(os.Getpid()); err != nil {
		t.Skipf("platform task-state reader is unavailable on this test host: %v", err)
	}
	helper := newSupervisorHelperFixture(t, "stop-then-exit")
	reader := &scriptedTreeReader{samples: []processTreeSample{{Stopped: true}}}
	reportedStopped, observedStopped := false, false
	var stateReadErr error
	options := supervisorOptionsForTest(t, 0, 80*time.Millisecond)
	helper.gate(&options, reader, func(_ int, _ processTreeSample) {
		if !reportedStopped || helper.command.Process == nil {
			return
		}
		stopped, err := processStoppedForTest(helper.command.Process.Pid)
		if err != nil {
			stateReadErr = err
			_ = syscall.Kill(helper.command.Process.Pid, syscall.SIGCONT)
			helper.keepAliveUntilExit()
			return
		}
		if stopped {
			observedStopped = true
			if err := syscall.Kill(helper.command.Process.Pid, syscall.SIGCONT); err != nil {
				stateReadErr = err
			}
			helper.keepAliveUntilExit()
		}
	})
	options.OnReading = func(state string) {
		if state != "stopped" {
			stateReadErr = fmt.Errorf("reading = %q, want stopped", state)
		}
		reportedStopped = true
	}
	outcome := superviseCommand(helper.command, options)
	if outcome.Verdict != "" || outcome.WaitErr != nil || !reportedStopped || !observedStopped || stateReadErr != nil {
		t.Fatalf("resumed process outcome=%+v reported=%v observed=%v stateReadErr=%v", outcome, reportedStopped, observedStopped, stateReadErr)
	}
}

func TestSupervisorDescendantAndReapedChildCPUAreCounted(t *testing.T) {
	t.Run("setsid descendant", func(t *testing.T) {
		reader := availableProcessTreeReader(t)
		helper := newSupervisorHelperFixture(t, "setsid-child")
		observedCPU, observedMembers := float64(0), 0
		options := supervisorOptionsForTest(t, 10, 400*time.Millisecond)
		options.Limits.ZeroConsumptionWindow = 0
		releasedNested := false
		helper.gate(&options, reader, func(_ int, sample processTreeSample) {
			cpu := sampleMemberCPUTotal(sample)
			if cpu > observedCPU {
				observedCPU = cpu
			}
			if len(sample.Members) > observedMembers {
				observedMembers = len(sample.Members)
			}
			if !releasedNested && observedCPU >= 0.25 && observedMembers >= 2 {
				releasedNested = true
				helper.releaseNested()
			} else if releasedNested && len(sample.Members) < 2 {
				helper.closeInput()
			}
		})
		outcome := superviseCommand(helper.command, options)
		if outcome.Verdict != "" || outcome.WaitErr != nil || observedCPU < 0.25 || observedMembers < 2 {
			t.Fatalf("real setsid descendant outcome=%+v observedCPU=%.2f members=%d", outcome, observedCPU, observedMembers)
		}
	})
	t.Run("reaped child", testPlatformReapedChildCPU)
}

func TestSupervisorSectionResultGrowthCountsAsOutput(t *testing.T) {
	resultPath := filepath.Join(t.TempDir(), "stage-results.tsv")
	helper := newSupervisorHelperFixture(t, "stage-writer", resultPath)
	reader := availableProcessTreeReader(t)
	options := supervisorOptionsForTest(t, 10, 80*time.Millisecond)
	options.Limits.ZeroConsumptionWindow = 0
	activity := newOutputActivity(time.Now())
	activityBefore := activity.Last()
	observedMembers := 0
	observedStageGrowth := false
	releasedNested := false
	options.Activity, options.StageResultPath = activity, resultPath
	helper.gate(&options, reader, func(_ int, sample processTreeSample) {
		if len(sample.Members) > observedMembers {
			observedMembers = len(sample.Members)
		}
		if !helper.readyActivity.IsZero() && activity.Last().After(helper.readyActivity) {
			observedStageGrowth = true
		}
		if !releasedNested && len(sample.Members) >= 2 && observedStageGrowth {
			releasedNested = true
			helper.releaseNested()
		} else if releasedNested && len(sample.Members) < 2 {
			helper.closeInput()
		}
	})
	outcome := superviseCommand(helper.command, options)
	if outcome.Verdict != "" || outcome.WaitErr != nil || observedMembers < 2 || !observedStageGrowth || !activity.Last().After(activityBefore) {
		t.Fatalf("real stage-result growth outcome=%+v members=%d growth=%v activityBefore=%s activityAfter=%s", outcome, observedMembers, observedStageGrowth, activityBefore, activity.Last())
	}
}

func TestSupervisorKillsSIGQUITIgnoringBusyChildAfterCounterRises(t *testing.T) {
	t.Run("counter rises", func(t *testing.T) {
		helper := newSupervisorHelperFixture(t, "ignore-quit-busy")
		reader := &scriptedTreeReader{samples: []processTreeSample{{CPUSeconds: 1}, {CPUSeconds: 2}}}
		options := supervisorOptionsForTest(t, 1, 40*time.Millisecond)
		verdicts := []string{}
		options.OnVerdict = func(verdict string) { verdicts = append(verdicts, verdict) }
		helper.gate(&options, reader, nil)
		outcome := superviseCommand(helper.command, options)
		if outcome.Verdict != "runaway" || outcome.Dump != "dump: not produced, the process kept computing" || len(verdicts) != 1 || verdicts[0] != "runaway" {
			t.Fatalf("SIGQUIT-ignoring outcome=%+v", outcome)
		}
	})
	t.Run("no window kills after quiet sample", func(t *testing.T) {
		helper := newSupervisorHelperFixture(t, "ignore-quit-idle")
		options := supervisorOptions{Limits: supervisorLimits{CPUBudgetSeconds: 1}, SampleInterval: 10 * time.Millisecond}
		helper.gate(&options, &scriptedTreeReader{samples: []processTreeSample{{CPUSeconds: 1}, {CPUSeconds: 1}}}, nil)
		outcome := superviseCommand(helper.command, options)
		if outcome.Verdict != "runaway" || outcome.Dump != "dump: killed while writing" {
			t.Fatalf("no-window quiet dump outcome=%+v", outcome)
		}
	})
}

func TestRunawayAndDeadAreFailedDeliveryStatuses(t *testing.T) {
	for _, status := range []string{"runaway", "dead"} {
		reason := status + " reason carried on the group line"
		result := validSupervisorStatusResult(status, reason)
		if result.Delivery.Sufficient || len(result.Delivery.FailingGroups) != 1 {
			t.Fatalf("status %s delivery=%+v", status, result.Delivery)
		}
		if err := ValidateTestResult(result); err != nil {
			t.Fatalf("status %s was refused by result validation: %v", status, err)
		}
		line := captureTestGroupProgress(t, status, reason)
		if !strings.Contains(line, "TEST-GROUP end group status="+status+" reason="+reason) {
			t.Fatalf("status %s progress line omitted its reason: %q", status, line)
		}
	}
	unknown := validSupervisorStatusResult("unknown-supervisor-status", "unknown")
	if err := ValidateTestResult(unknown); err == nil || !strings.Contains(err.Error(), "invalid status") {
		t.Fatalf("unknown group status was accepted: %v", err)
	}
}

type scriptedTreeReader struct {
	samples []processTreeSample
	fail    error
	index   int
}

type readingUntilReportedTreeReader struct {
	reported *bool
	waiting  bool
}

type readinessGatedTreeReader struct {
	ready         <-chan error
	keepAlive     <-chan struct{}
	activity      *outputActivity
	readyActivity *time.Time
	reader        processTreeReader
	readyObserved bool
	readyErr      error
	call          int
	onSample      func(int, processTreeSample)
}

type supervisorHelperFixture struct {
	command             *exec.Cmd
	stdin               io.WriteCloser
	nestedInput         io.WriteCloser
	nestedInputRead     io.Closer
	grandchildInput     io.WriteCloser
	grandchildInputRead io.Closer
	stdout              *bufio.Reader
	ready               chan error
	keepAlive           chan struct{}
	log                 *synchronizedBuffer
	readyActivity       time.Time
	closeOnce           sync.Once
	nestedCloseOnce     sync.Once
	grandchildCloseOnce sync.Once
	keepOnce            sync.Once
}

func supervisorOptionsForTest(t *testing.T, cpuBudget int64, window time.Duration) supervisorOptions {
	t.Helper()
	if supervisorLimitsForTest != (supervisorLimits{}) {
		t.Fatal("supervisor test limits leaked between tests")
	}
	supervisorLimitsForTest = supervisorLimits{CPUBudgetSeconds: cpuBudget, ZeroConsumptionWindow: window}
	t.Cleanup(func() { supervisorLimitsForTest = supervisorLimits{} })
	limits, interval := groupSupervisorSettings(nil)
	return supervisorOptions{Limits: limits, SampleInterval: interval}
}

func (reader *scriptedTreeReader) Sample(rootPID int) (processTreeSample, error) {
	if reader.fail != nil {
		return processTreeSample{}, reader.fail
	}
	index := reader.index
	if index >= len(reader.samples) {
		index = len(reader.samples) - 1
	}
	reader.index++
	if index < 0 {
		return processTreeSample{Members: []int{rootPID}}, nil
	}
	sample := reader.samples[index]
	if len(sample.Members) == 0 {
		sample.Members = []int{rootPID}
	}
	return sample, nil
}

func (reader *readingUntilReportedTreeReader) Sample(rootPID int) (processTreeSample, error) {
	return processTreeSample{Members: []int{rootPID}, Waiting: reader.waiting && !*reader.reported}, nil
}

func (reader *readinessGatedTreeReader) Sample(rootPID int) (processTreeSample, error) {
	if reader.readyErr != nil {
		return processTreeSample{}, reader.readyErr
	}
	if !reader.readyObserved {
		select {
		case reader.readyErr = <-reader.ready:
			if reader.readyErr != nil {
				return processTreeSample{}, reader.readyErr
			}
			reader.readyObserved = true
		default:
		}
		markedAt := time.Now()
		reader.activity.Mark(markedAt)
		if reader.readyObserved {
			*reader.readyActivity = markedAt
		}
		return processTreeSample{Members: []int{rootPID}}, nil
	}
	sample, err := reader.reader.Sample(rootPID)
	if err != nil {
		return processTreeSample{}, err
	}
	reader.call++
	if reader.onSample != nil {
		reader.onSample(reader.call, sample)
	}
	select {
	case <-reader.keepAlive:
		reader.activity.Mark(time.Now())
	default:
	}
	return sample, nil
}

func newSupervisorHelperFixture(t *testing.T, mode string, args ...string) *supervisorHelperFixture {
	t.Helper()
	command := rawSupervisorHelperCommand(mode, args...)
	var nestedInput io.WriteCloser
	var nestedInputRead io.Closer
	var grandchildInput io.WriteCloser
	var grandchildInputRead io.Closer
	if supervisorHelperHasNestedProcess(mode) {
		readEnd, writeEnd, pipeErr := os.Pipe()
		if pipeErr != nil {
			t.Fatalf("open %s nested helper stdin: %v", mode, pipeErr)
		}
		command.ExtraFiles = append(command.ExtraFiles, readEnd)
		command.Env = append(command.Env, "SUPERVISOR_NESTED_STDIN_FD=3")
		nestedInput, nestedInputRead = writeEnd, readEnd
	}
	if mode == "reaped-child" {
		readEnd, writeEnd, pipeErr := os.Pipe()
		if pipeErr != nil {
			_ = nestedInput.Close()
			_ = nestedInputRead.Close()
			t.Fatalf("open %s grandchild stdin: %v", mode, pipeErr)
		}
		command.ExtraFiles = append(command.ExtraFiles, readEnd)
		command.Env = append(command.Env, "SUPERVISOR_GRANDCHILD_STDIN_FD=4")
		grandchildInput, grandchildInputRead = writeEnd, readEnd
	}
	stdin, err := command.StdinPipe()
	if err != nil {
		if nestedInput != nil {
			_ = nestedInput.Close()
			_ = nestedInputRead.Close()
		}
		if grandchildInput != nil {
			_ = grandchildInput.Close()
			_ = grandchildInputRead.Close()
		}
		t.Fatalf("open %s helper stdin: %v", mode, err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		if nestedInput != nil {
			_ = nestedInput.Close()
			_ = nestedInputRead.Close()
		}
		if grandchildInput != nil {
			_ = grandchildInput.Close()
			_ = grandchildInputRead.Close()
		}
		t.Fatalf("open %s helper stdout: %v", mode, err)
	}
	fixture := &supervisorHelperFixture{
		command:             command,
		stdin:               stdin,
		nestedInput:         nestedInput,
		nestedInputRead:     nestedInputRead,
		grandchildInput:     grandchildInput,
		grandchildInputRead: grandchildInputRead,
		stdout:              bufio.NewReader(stdout),
		ready:               make(chan error, 1),
		keepAlive:           make(chan struct{}),
		log:                 &synchronizedBuffer{},
	}
	command.Stderr = fixture.log
	go func() {
		line, readErr := fixture.stdout.ReadString('\n')
		if readErr == nil && line != "ready\n" {
			readErr = fmt.Errorf("helper announced %q, want %q", line, "ready\n")
		}
		fixture.ready <- readErr
	}()
	t.Cleanup(fixture.closeInput)
	return fixture
}

func (fixture *supervisorHelperFixture) closeInput() {
	fixture.releaseGrandchild()
	fixture.releaseNested()
	fixture.keepAliveUntilExit()
	fixture.closeOnce.Do(func() {
		_ = fixture.stdin.Close()
		if fixture.nestedInputRead != nil {
			_ = fixture.nestedInputRead.Close()
		}
		if fixture.grandchildInputRead != nil {
			_ = fixture.grandchildInputRead.Close()
		}
	})
}

func (fixture *supervisorHelperFixture) releaseNested() {
	fixture.nestedCloseOnce.Do(func() {
		if fixture.nestedInput != nil {
			_ = fixture.nestedInput.Close()
		}
	})
}

func (fixture *supervisorHelperFixture) releaseGrandchild() {
	fixture.grandchildCloseOnce.Do(func() {
		if fixture.grandchildInput != nil {
			_ = fixture.grandchildInput.Close()
		}
	})
}

func (fixture *supervisorHelperFixture) keepAliveUntilExit() {
	fixture.keepOnce.Do(func() { close(fixture.keepAlive) })
}

func (fixture *supervisorHelperFixture) gate(options *supervisorOptions, reader processTreeReader, onSample func(int, processTreeSample)) {
	if options.Activity == nil {
		options.Activity = newOutputActivity(time.Now())
	}
	options.Reader = &readinessGatedTreeReader{
		ready: fixture.ready, keepAlive: fixture.keepAlive, activity: options.Activity, readyActivity: &fixture.readyActivity,
		reader: reader, onSample: onSample,
	}
}

func supervisorHelperHasNestedProcess(mode string) bool {
	switch mode {
	case "setsid-child", "reaped-child", "retained-child", "stage-writer":
		return true
	default:
		return false
	}
}

func rawSupervisorHelperCommand(mode string, args ...string) *exec.Cmd {
	arguments := append([]string{"-test.run=^TestSupervisorProcessHelper$", "--", mode}, args...)
	command := exec.Command(os.Args[0], arguments...)
	command.Env = append(os.Environ(), "GO_WANT_SUPERVISOR_HELPER=1", "GOMAXPROCS=1")
	return command
}

func TestSupervisorProcessHelper(t *testing.T) {
	if os.Getenv("GO_WANT_SUPERVISOR_HELPER") != "1" {
		return
	}
	separator := 0
	for index, value := range os.Args {
		if value == "--" {
			separator = index + 1
			break
		}
	}
	mode := os.Args[separator]
	args := os.Args[separator+1:]
	go func() {
		_, _ = io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	}()
	switch mode {
	case "idle":
		announceSupervisorHelperReady()
		select {}
	case "ignore-quit-idle":
		signal.Ignore(syscall.SIGQUIT)
		announceSupervisorHelperReady()
		select {}
	case "stop-then-exit":
		announceSupervisorHelperReady()
		_ = syscall.Kill(os.Getpid(), syscall.SIGSTOP)
	case "busy-for":
		announceSupervisorHelperReady()
		duration, _ := time.ParseDuration(args[0])
		busyForCPU(duration)
	case "busy-forever":
		announceSupervisorHelperReady()
		//lint:ignore SA5002 This helper is the busy process tree that the supervisor rows judge.
		for {
		}
	case "ignore-quit-busy":
		signal.Ignore(syscall.SIGQUIT)
		announceSupervisorHelperReady()
		//lint:ignore SA5002 This helper is the busy process tree that the supervisor rows judge.
		for {
		}
	case "setsid-child":
		child, err := startNestedSupervisorHelper(true, "busy-forever")
		if err != nil {
			os.Exit(31)
		}
		announceSupervisorHelperReady()
		if child.Wait() != nil {
			os.Exit(31)
		}
		select {}
	case "reaped-child":
		child, err := startNestedSupervisorHelper(false, "reaping-member")
		if err != nil {
			os.Exit(32)
		}
		announceSupervisorHelperReady()
		if child.Wait() != nil {
			os.Exit(32)
		}
		select {}
	case "reaping-member":
		child, err := startNestedSupervisorHelper(false, "busy-forever")
		if err != nil {
			os.Exit(32)
		}
		announceSupervisorHelperReady()
		if child.Wait() != nil {
			os.Exit(32)
		}
		select {}
	case "retained-child":
		child, err := startNestedSupervisorHelper(false, "busy-forever")
		if err != nil {
			os.Exit(32)
		}
		announceSupervisorHelperReady()
		if child.Wait() != nil {
			os.Exit(32)
		}
		select {}
	case "stage-writer":
		child, err := startNestedSupervisorHelper(true, "stage-child", args[0])
		if err != nil {
			os.Exit(33)
		}
		announceSupervisorHelperReady()
		if child.Wait() != nil {
			os.Exit(33)
		}
		select {}
	case "stage-child":
		file, err := os.OpenFile(args[0], os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			os.Exit(33)
		}
		_, writeErr := fmt.Fprintln(file, strconv.Itoa(0))
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			os.Exit(33)
		}
		announceSupervisorHelperReady()
		sequence := 1
		for {
			busyForCPU(50 * time.Millisecond)
			file, err := os.OpenFile(args[0], os.O_APPEND|os.O_WRONLY, 0o600)
			if err != nil {
				os.Exit(33)
			}
			_, writeErr := fmt.Fprintln(file, strconv.Itoa(sequence))
			closeErr := file.Close()
			if writeErr != nil || closeErr != nil {
				os.Exit(33)
			}
			sequence++
		}
	default:
		os.Exit(34)
	}
}

func announceSupervisorHelperReady() {
	if _, err := fmt.Fprintln(os.Stdout, "ready"); err != nil {
		os.Exit(35)
	}
}

func startNestedSupervisorHelper(setsid bool, mode string, args ...string) (*exec.Cmd, error) {
	command := rawSupervisorHelperCommand(mode, args...)
	command.Stderr = os.Stderr
	if setsid {
		command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}
	fd, err := strconv.Atoi(os.Getenv("SUPERVISOR_NESTED_STDIN_FD"))
	if err != nil || fd < 3 {
		return nil, fmt.Errorf("nested helper stdin descriptor is unavailable")
	}
	nestedInput := os.NewFile(uintptr(fd), "supervisor-nested-stdin")
	if nestedInput == nil {
		return nil, fmt.Errorf("open nested helper stdin descriptor %d", fd)
	}
	command.Stdin = nestedInput
	var forwardedInput *os.File
	if grandchildDescriptor := os.Getenv("SUPERVISOR_GRANDCHILD_STDIN_FD"); grandchildDescriptor != "" {
		grandchildFD, descriptorErr := strconv.Atoi(grandchildDescriptor)
		if descriptorErr != nil || grandchildFD < 3 {
			return nil, fmt.Errorf("grandchild helper stdin descriptor is unavailable")
		}
		forwardedInput = os.NewFile(uintptr(grandchildFD), "supervisor-grandchild-stdin")
		if forwardedInput == nil {
			return nil, fmt.Errorf("open grandchild helper stdin descriptor %d", grandchildFD)
		}
		command.ExtraFiles = append(command.ExtraFiles, forwardedInput)
		command.Env = supervisorHelperChildEnvironment(command.Env)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		if forwardedInput != nil {
			_ = forwardedInput.Close()
		}
		return nil, err
	}
	if err := command.Start(); err != nil {
		if forwardedInput != nil {
			_ = forwardedInput.Close()
		}
		return nil, err
	}
	if forwardedInput != nil {
		_ = forwardedInput.Close()
	}
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("nested %s helper readiness %q: %w", mode, line, err)
	}
	if line != "ready\n" {
		return nil, fmt.Errorf("nested %s helper readiness %q, want %q", mode, line, "ready\n")
	}
	return command, nil
}

func supervisorHelperChildEnvironment(environment []string) []string {
	result := make([]string, 0, len(environment)+1)
	for _, value := range environment {
		if strings.HasPrefix(value, "SUPERVISOR_NESTED_STDIN_FD=") || strings.HasPrefix(value, "SUPERVISOR_GRANDCHILD_STDIN_FD=") {
			continue
		}
		result = append(result, value)
	}
	return append(result, "SUPERVISOR_NESTED_STDIN_FD=3")
}

func busyForCPU(duration time.Duration) {
	started := processCPU()
	for processCPU()-started < duration {
	}
}

func processCPU() time.Duration {
	var usage syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &usage)
	return timevalDuration(usage.Utime) + timevalDuration(usage.Stime)
}

func timevalDuration(value syscall.Timeval) time.Duration {
	return time.Duration(value.Sec)*time.Second + time.Duration(value.Usec)*time.Microsecond
}

func availableProcessTreeReader(t *testing.T) processTreeReader {
	t.Helper()
	reader := newProcessTreeReader()
	if _, err := reader.Sample(os.Getpid()); err != nil {
		t.Skipf("platform process reader is unavailable on this test host: %v", err)
	}
	return reader
}

func sampleMemberCPUTotal(sample processTreeSample) float64 {
	if sample.MemberCPU == nil {
		return sample.CPUSeconds
	}
	total := float64(0)
	for _, member := range sample.MemberCPU {
		total += member.CPUSeconds
	}
	return total
}

func validSupervisorStatusResult(status, reason string) TestResult {
	digest := strings.Repeat("a", 64)
	result := TestResult{SchemaVersion: TestResultSchemaVersion, Purpose: testpolicy.PurposeDelivery,
		RequestedMode: testpolicy.ModeStandard, RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard,
		ProjectRoot: "/fixture", BaseCommit: "base", CandidateTree: strings.Repeat("b", 40), ContractDigest: digest,
		BaseContractDigest: digest, PolicyEngineDigest: digest, BehaviorPolicyDigest: digest, PlanDigest: digest,
		RequiredGroups: []string{"group"}, SelectedGroups: []string{"group"}, LaunchCounts: LaunchCounts{CountsComplete: true},
		Cost: TestCost{DeclaredTargetMS: 1}, Groups: []GroupResult{{ID: "group", Kind: "unit", InputManifest: []string{"source"},
			Status: status, NotRunReason: reason, Obligations: []string{"delivery"}}}}
	result.RecomputeDelivery()
	return result
}

func captureTestGroupProgress(t *testing.T, status, reason string) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = writer
	progressErr := testGroupProgress(filepath.Join(t.TempDir(), "progress.jsonl"), "group", "end", status, reason)
	os.Stdout = original
	closeErr := writer.Close()
	data, readErr := io.ReadAll(reader)
	_ = reader.Close()
	if progressErr != nil || closeErr != nil || readErr != nil {
		t.Fatalf("capture TEST-GROUP line: progress=%v close=%v read=%v", progressErr, closeErr, readErr)
	}
	return string(data)
}
