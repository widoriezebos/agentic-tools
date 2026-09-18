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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
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

type treeSignalProbeResult struct {
	exact identity.Exact
	state identity.Liveness
	err   error
}

type treeSignalProber map[int64]treeSignalProbeResult

func (prober treeSignalProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	result := prober[pid]
	return result.exact, result.state, result.err
}

func TestTreeSignalSparesOnlyAProvedCustodian(t *testing.T) {
	commands := make([]*exec.Cmd, 7)
	refs := make([]identity.Ref, len(commands))
	for index := range commands {
		command := exec.Command("sleep", "60")
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		commands[index] = command
		exact, state, err := (identity.KernelProber{}).Probe(int64(command.Process.Pid))
		if err != nil || state != identity.Alive {
			_ = command.Process.Kill()
			_ = command.Wait()
			t.Fatalf("probe fixture process %d: state=%s err=%v", command.Process.Pid, state, err)
		}
		refs[index] = exact.Ref()
		ref := refs[index]
		t.Cleanup(func() {
			_ = identity.SignalExact(identity.KernelProber{}, ref, syscall.SIGKILL)
			_ = command.Wait()
		})
	}

	probe := treeSignalProber{}
	for _, ref := range refs {
		probe[ref.Pid] = treeSignalProbeResult{exact: identity.Exact{Pid: ref.Pid, StartedAt: time.UnixMicro(ref.StartedAtUnixMicro), StartTicks: ref.StartTicks, BootID: ref.BootID}, state: identity.Alive}
	}
	proved := probe[refs[1].Pid]
	proved.exact.Environ, proved.exact.EnvironKnown = []string{"A=b", identity.FixtureCustodianEnv + "=1"}, true
	probe[refs[1].Pid] = proved
	probe[refs[2].Pid] = treeSignalProbeResult{err: fmt.Errorf("probe denied"), state: identity.Unknown}
	probe[refs[3].Pid] = treeSignalProbeResult{state: identity.Dead}
	unreadable := probe[refs[4].Pid]
	unreadable.exact.EnvironKnown = false
	probe[refs[4].Pid] = unreadable
	missing := probe[refs[5].Pid]
	missing.exact.Environ, missing.exact.EnvironKnown = []string{"A=b"}, true
	probe[refs[5].Pid] = missing
	wrong := probe[refs[6].Pid]
	wrong.exact.Environ, wrong.exact.EnvironKnown = []string{identity.FixtureCustodianEnv + "=0"}, true
	probe[refs[6].Pid] = wrong

	received := map[syscall.Signal]map[int]int{}
	sender := func(pid int, signal syscall.Signal) error {
		if received[signal] == nil {
			received[signal] = map[int]int{}
		}
		received[signal][pid]++
		return nil
	}
	members := make([]int, 0, len(refs)-1)
	for _, ref := range refs[1:] {
		members = append(members, int(ref.Pid))
	}
	for _, signal := range []syscall.Signal{syscall.SIGQUIT, syscall.SIGKILL} {
		signalProcessTree(members, int(refs[0].Pid), signal, probe, sender)
		if received[signal][int(refs[1].Pid)] != 0 {
			t.Fatalf("proved custodian %d received %s", refs[1].Pid, signal)
		}
		for _, ref := range append([]identity.Ref{refs[0]}, refs[2:]...) {
			if received[signal][int(ref.Pid)] != 1 {
				t.Fatalf("pid %d received %s %d times, want once; all=%v", ref.Pid, signal, received[signal][int(ref.Pid)], received[signal])
			}
		}
	}
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

type custodianKillTreeReader struct {
	fixture *supervisorHelperFixture
}

func (reader custodianKillTreeReader) Sample(rootPID int) (processTreeSample, error) {
	if len(reader.fixture.custodians) < 2 {
		return processTreeSample{}, fmt.Errorf("helper did not announce its custodian and fixture child")
	}
	return processTreeSample{Members: []int{rootPID, int(reader.fixture.custodians[0].Pid)}}, nil
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
	custodians          []identity.Ref
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

func TestSupervisorKillSparesTheFixtureCustodian(t *testing.T) {
	helper := newSupervisorHelperFixture(t, "custodian-reaped-child")
	t.Cleanup(func() {
		if helper.command.Process != nil {
			if exact, state, err := (identity.KernelProber{}).Probe(int64(helper.command.Process.Pid)); err == nil && state == identity.Alive {
				_ = identity.SignalExact(identity.KernelProber{}, exact.Ref(), syscall.SIGKILL)
			}
		}
		for _, ref := range helper.custodians {
			_ = identity.SignalExact(identity.KernelProber{}, ref, syscall.SIGKILL)
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	options := supervisorOptionsForTest(t, 0, 5*time.Second)
	options.Context = ctx
	options.Activity = newOutputActivity(time.Now())
	options.Reader = &readinessGatedTreeReader{
		ready: helper.ready, keepAlive: helper.keepAlive, activity: options.Activity, readyActivity: &helper.readyActivity,
		reader: custodianKillTreeReader{fixture: helper}, onSample: func(int, processTreeSample) { cancel() },
	}
	outcome := superviseCommand(helper.command, options)
	if outcome.Verdict != "cancelled" || outcome.WaitErr == nil {
		t.Fatalf("cancelled supervisor outcome=%+v", outcome)
	}
	if len(helper.custodians) != 2 {
		t.Fatalf("helper announced refs=%v, want custodian and child", helper.custodians)
	}
	custodian, child := helper.custodians[0], helper.custodians[1]
	waitProofProcessDead(t, custodian)
	logPath := fmt.Sprintf("%s.custodian-%d.log", os.Getenv("METASYSTEM_SUPERVISION_REGISTRY_HOME"), helper.command.Process.Pid)
	log, err := os.ReadFile(logPath)
	if err != nil || !strings.Contains(string(log), "action=kill pid="+strconv.FormatInt(child.Pid, 10)) {
		t.Fatalf("custodian log %s omitted the kill for pid %d: log=%q err=%v", logPath, child.Pid, log, err)
	}
	exact, state, err := (identity.KernelProber{}).Probe(child.Pid)
	if err != nil || state != identity.Dead && (state != identity.Alive || identity.SameIdentity(exact, child) && !exact.Zombie) {
		t.Fatalf("fixture child %d remained at its exact identity: state=%s zombie=%t err=%v", child.Pid, state, exact.Zombie, err)
	}
}

func waitProofProcessDead(t *testing.T, ref identity.Ref) {
	t.Helper()
	observed := identity.Unknown
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		exact, state, err := (identity.KernelProber{}).Probe(ref.Pid)
		observed = state
		if err == nil && (state == identity.Dead || state == identity.Alive && (!identity.SameIdentity(exact, ref) || exact.Zombie)) {
			return
		}
	}
	t.Fatalf("process %d did not become dead; state=%s", ref.Pid, observed)
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
		if readErr == nil {
			fixture.custodians, readErr = parseSupervisorHelperReady(line)
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
		reader: &supervisorHelperTreeReader{reader: reader, custodians: &fixture.custodians, prober: identity.KernelProber{}}, onSample: onSample,
	}
}

type supervisorHelperTreeReader struct {
	reader     processTreeReader
	custodians *[]identity.Ref
	prober     identity.Prober
}

func (reader *supervisorHelperTreeReader) Sample(rootPID int) (processTreeSample, error) {
	sample, err := reader.reader.Sample(rootPID)
	if err != nil {
		return processTreeSample{}, err
	}
	return excludeSupervisorHelperCustodians(sample, *reader.custodians, reader.prober), nil
}

func excludeSupervisorHelperCustodians(sample processTreeSample, custodians []identity.Ref, prober identity.Prober) processTreeSample {
	excluded := make(map[int]bool, len(custodians))
	for _, ref := range custodians {
		exact, state, err := prober.Probe(ref.Pid)
		if err == nil && state == identity.Alive && identity.SameIdentity(exact, ref) {
			excluded[int(ref.Pid)] = true
		}
	}
	members := make([]int, 0, len(sample.Members))
	for _, pid := range sample.Members {
		if !excluded[pid] {
			members = append(members, pid)
		}
	}
	sample.Members = members
	if sample.MemberCPU != nil {
		memberCPU := make([]processMemberCPU, 0, len(sample.MemberCPU))
		for _, member := range sample.MemberCPU {
			if !excluded[member.PID] {
				memberCPU = append(memberCPU, member)
			}
		}
		sample.MemberCPU = memberCPU
	}
	return sample
}

func TestSupervisorHelperCustodianAccountingUsesExactIdentity(t *testing.T) {
	custodian, ok := testenv.FixtureCustodian()
	if !ok {
		t.Fatal("test binary has no fixture custodian")
	}
	ownerPID := os.Getpid()
	sample := processTreeSample{
		Members: []int{ownerPID, int(custodian.Pid)},
		MemberCPU: []processMemberCPU{
			{PID: ownerPID, CPUSeconds: 0.5},
			{PID: int(custodian.Pid), CPUSeconds: 0.25},
		},
	}
	fixture := &supervisorHelperFixture{ready: make(chan error, 1), custodians: []identity.Ref{custodian}, keepAlive: make(chan struct{})}
	fixture.ready <- nil
	options := supervisorOptions{}
	fixture.gate(&options, &scriptedTreeReader{samples: []processTreeSample{sample}}, nil)
	if _, err := options.Reader.Sample(ownerPID); err != nil {
		t.Fatal(err)
	}
	got, err := options.Reader.Sample(ownerPID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Members) != 1 || got.Members[0] != ownerPID || len(got.MemberCPU) != 1 || got.MemberCPU[0].PID != ownerPID || sampleMemberCPUTotal(got) != 0.5 {
		t.Fatalf("custodian-filtered sample = %+v; want only owner pid %d and its CPU", got, ownerPID)
	}
	mismatched := custodian
	if mismatched.StartTicks != 0 {
		mismatched.StartTicks++
	} else {
		mismatched.StartedAtUnixMicro++
	}
	got = excludeSupervisorHelperCustodians(sample, []identity.Ref{mismatched}, identity.KernelProber{})
	if len(got.Members) != 2 || len(got.MemberCPU) != 2 {
		t.Fatalf("identity-mismatched sample = %+v; want both processes retained", got)
	}
}

func supervisorHelperHasNestedProcess(mode string) bool {
	switch mode {
	case "setsid-child", "reaped-child", "retained-child", "stage-writer", "custodian-reaped-child":
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
		child, custodians, err := startNestedSupervisorHelper(true, "busy-forever")
		if err != nil {
			os.Exit(31)
		}
		announceSupervisorHelperReady(custodians...)
		if child.Wait() != nil {
			os.Exit(31)
		}
		select {}
	case "reaped-child":
		child, custodians, err := startNestedSupervisorHelper(false, "reaping-member")
		if err != nil {
			os.Exit(32)
		}
		announceSupervisorHelperReady(custodians...)
		if child.Wait() != nil {
			os.Exit(32)
		}
		select {}
	case "reaping-member":
		child, custodians, err := startNestedSupervisorHelper(false, "busy-forever")
		if err != nil {
			os.Exit(32)
		}
		announceSupervisorHelperReady(custodians...)
		if child.Wait() != nil {
			os.Exit(32)
		}
		select {}
	case "retained-child":
		child, custodians, err := startNestedSupervisorHelper(false, "busy-forever")
		if err != nil {
			os.Exit(32)
		}
		announceSupervisorHelperReady(custodians...)
		if child.Wait() != nil {
			os.Exit(32)
		}
		select {}
	case "stage-writer":
		child, custodians, err := startNestedSupervisorHelper(true, "stage-child", args[0])
		if err != nil {
			os.Exit(33)
		}
		announceSupervisorHelperReady(custodians...)
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
	case "custodian-reaped-child":
		child, err := startTaggedSupervisorFixtureChild(t)
		if err != nil {
			os.Exit(36)
		}
		announceSupervisorHelperReady(child)
		select {}
	default:
		os.Exit(34)
	}
}

func startTaggedSupervisorFixtureChild(t *testing.T) (identity.Ref, error) {
	fixture := testutil.Fixture(t)
	fd, err := strconv.Atoi(os.Getenv("SUPERVISOR_NESTED_STDIN_FD"))
	if err != nil || fd < 3 {
		return identity.Ref{}, fmt.Errorf("fixture child stdin descriptor is unavailable")
	}
	input := os.NewFile(uintptr(fd), "supervisor-fixture-child-stdin")
	command := fixture.Shell("trap '' TERM HUP; printf 'ready\\n'; cat >/dev/null")
	command.Stdin = input
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return identity.Ref{}, err
	}
	if err := command.Start(); err != nil {
		return identity.Ref{}, err
	}
	fixture.Record(command.Process.Pid)
	if t.Failed() {
		_ = command.Process.Kill()
		return identity.Ref{}, fmt.Errorf("record fixture child")
	}
	if line, readErr := bufio.NewReader(stdout).ReadString('\n'); readErr != nil || line != "ready\n" {
		_ = command.Process.Kill()
		return identity.Ref{}, fmt.Errorf("fixture child readiness=%q err=%v", line, readErr)
	}
	exact, state, err := (identity.KernelProber{}).Probe(int64(command.Process.Pid))
	if err != nil || state != identity.Alive {
		_ = command.Process.Kill()
		return identity.Ref{}, fmt.Errorf("probe fixture child: state=%s err=%v", state, err)
	}
	if err := command.Process.Release(); err != nil {
		return identity.Ref{}, err
	}
	return exact.Ref(), nil
}

func announceSupervisorHelperReady(descendants ...identity.Ref) {
	custodian, ok := testenv.FixtureCustodian()
	if !ok {
		os.Exit(35)
	}
	custodians := append([]identity.Ref{custodian}, descendants...)
	values := make([]string, len(custodians))
	for index, ref := range custodians {
		value, err := identity.EncodeRef(ref)
		if err != nil {
			os.Exit(35)
		}
		values[index] = value
	}
	if _, err := fmt.Fprintf(os.Stdout, "ready %s\n", strings.Join(values, "|")); err != nil {
		os.Exit(35)
	}
}

func parseSupervisorHelperReady(line string) ([]identity.Ref, error) {
	encoded, ok := strings.CutSuffix(strings.TrimPrefix(line, "ready "), "\n")
	if !ok || !strings.HasPrefix(line, "ready ") || encoded == "" {
		return nil, fmt.Errorf("helper announced %q, want ready with custodian identities", line)
	}
	values := strings.Split(encoded, "|")
	refs := make([]identity.Ref, len(values))
	for index, value := range values {
		ref, err := identity.ParseRef(value)
		if err != nil {
			return nil, fmt.Errorf("helper announced invalid custodian identity %q: %w", value, err)
		}
		refs[index] = ref
	}
	return refs, nil
}

func startNestedSupervisorHelper(setsid bool, mode string, args ...string) (*exec.Cmd, []identity.Ref, error) {
	command := rawSupervisorHelperCommand(mode, args...)
	command.Stderr = os.Stderr
	if setsid {
		command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}
	fd, err := strconv.Atoi(os.Getenv("SUPERVISOR_NESTED_STDIN_FD"))
	if err != nil || fd < 3 {
		return nil, nil, fmt.Errorf("nested helper stdin descriptor is unavailable")
	}
	nestedInput := os.NewFile(uintptr(fd), "supervisor-nested-stdin")
	if nestedInput == nil {
		return nil, nil, fmt.Errorf("open nested helper stdin descriptor %d", fd)
	}
	command.Stdin = nestedInput
	var forwardedInput *os.File
	if grandchildDescriptor := os.Getenv("SUPERVISOR_GRANDCHILD_STDIN_FD"); grandchildDescriptor != "" {
		grandchildFD, descriptorErr := strconv.Atoi(grandchildDescriptor)
		if descriptorErr != nil || grandchildFD < 3 {
			return nil, nil, fmt.Errorf("grandchild helper stdin descriptor is unavailable")
		}
		forwardedInput = os.NewFile(uintptr(grandchildFD), "supervisor-grandchild-stdin")
		if forwardedInput == nil {
			return nil, nil, fmt.Errorf("open grandchild helper stdin descriptor %d", grandchildFD)
		}
		command.ExtraFiles = append(command.ExtraFiles, forwardedInput)
		command.Env = supervisorHelperChildEnvironment(command.Env)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		if forwardedInput != nil {
			_ = forwardedInput.Close()
		}
		return nil, nil, err
	}
	if err := command.Start(); err != nil {
		if forwardedInput != nil {
			_ = forwardedInput.Close()
		}
		return nil, nil, err
	}
	if forwardedInput != nil {
		_ = forwardedInput.Close()
	}
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		return nil, nil, fmt.Errorf("nested %s helper readiness %q: %w", mode, line, err)
	}
	custodians, err := parseSupervisorHelperReady(line)
	if err != nil {
		return nil, nil, fmt.Errorf("nested %s helper readiness: %w", mode, err)
	}
	return command, custodians, nil
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
