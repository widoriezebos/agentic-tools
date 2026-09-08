package run

import (
	"errors"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

func closedFence() stopfence.Record {
	return stopfence.Record{State: stopfence.StateClosed, Phase: stopfence.PhaseStopped, Generation: 4,
		ChangedAt: "2026-09-07T00:00:00Z", By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 74}}}
}

func TestCreationVerbsRefuseClosedFenceWithoutPublishing(t *testing.T) {
	for _, test := range []struct {
		name string
		call func(*Store) error
	}{
		{name: "launch", call: func(s *Store) error {
			_, err := s.Launch(mainCaller, LaunchParams{Id: "fenced-launch", Kind: "suite", Log: "run.log"})
			return err
		}},
		{name: "register", call: func(s *Store) error {
			return s.Register(mainCaller, LaunchParams{Id: "fenced-register", Kind: "custom", Log: "run.log"}, 41, "")
		}},
		{name: "adopt", call: func(s *Store) error { return s.Adopt(mainCaller, "fenced-adopt", 42) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := testStore(t)
			s.FenceRead = func(string) (stopfence.Record, error) { return closedFence(), nil }
			err := test.call(s)
			if err == nil || !strings.Contains(err.Error(), "since 2026-09-07T00:00:00Z, by stop pid 74") || !strings.Contains(err.Error(), "metasystem arm") {
				t.Fatalf("closed-fence refusal = %v", err)
			}
			if records, unreadable := s.List(); len(records) != 0 || len(unreadable) != 0 {
				t.Fatalf("closed fence published records: %+v unreadable=%v", records, unreadable)
			}
		})
	}
}

func TestCreatorRaceAfterWrapperSpawnEndsGroupAndFailsLaunch(t *testing.T) {
	s := testStore(t)
	pid := int64(50)
	probe := &argvProber{alive: true, pid: pid, start: 5000}
	s.Prober = probe
	s.GroupPresent = func(int64) (bool, bool) { return probe.alive, true }
	s.AllPids = func() ([]int64, error) {
		if probe.alive {
			return []int64{pid}, nil
		}
		return nil, nil
	}
	reads := 0
	s.FenceRead = func(string) (stopfence.Record, error) {
		reads++
		if reads <= 2 {
			return stopfence.Record{State: stopfence.StateOpen, Phase: stopfence.PhaseArmed, Generation: 7}, nil
		}
		return closedFence(), nil
	}
	creation, err := s.BeginCreation("run-launch")
	if err != nil {
		t.Fatal(err)
	}
	defer creation.Close()
	generation := creation.Generation
	nonce, err := s.Launch(mainCaller, LaunchParams{Id: "race-launch", Kind: "suite", Log: "run.log", FenceGeneration: &generation})
	if err != nil {
		t.Fatal(err)
	}
	probe.argv = []string{"metasystem", "run", "wrap", "--nonce", nonce}
	if err := s.Bind("race-launch", nonce, pid, pid); err != nil {
		t.Fatalf("wrapper bind: %v", err)
	}
	record, _ := s.Read("race-launch")
	if record.Status != StatusRunning || record.FenceGeneration != 7 {
		t.Fatalf("spawned wrapper record = %+v", record)
	}
	originalSignal := stopSignal
	var signals []syscall.Signal
	stopSignal = func(group int64, signal syscall.Signal) error {
		if group != pid {
			return errors.New("wrong wrapper group")
		}
		signals = append(signals, signal)
		probe.alive = false
		return nil
	}
	t.Cleanup(func() { stopSignal = originalSignal })
	if err := s.CompleteLaunch("race-launch", generation); err == nil || !strings.Contains(err.Error(), "while run race-launch started") {
		t.Fatalf("second-read result = %v", err)
	}
	record, _ = s.Read("race-launch")
	if record.Status != StatusEndedUnknown || record.Error == nil || *record.Error != "stopped by metasystem stop" {
		t.Fatalf("raced launch record = %+v", record)
	}
	if len(signals) != 1 || signals[0] != syscall.SIGTERM || !s.groupEmpty(pid) {
		t.Fatalf("raced wrapper group remains: signals=%v alive=%t", signals, probe.alive)
	}
	if err := creation.Close(); err != nil {
		t.Fatal(err)
	}
	claims, claimErr := stopfence.Claims(s.Root, generation)
	if claimErr != nil || len(claims) != 0 {
		t.Fatalf("completed wrapper left creation claim: %+v err=%v", claims, claimErr)
	}
}

func TestCreatorRaceAfterStopAndArmPrintsRetryInsteadOfIncompleteStop(t *testing.T) {
	s := testStore(t)
	pid := int64(50)
	probe := &argvProber{alive: true, pid: pid, start: 5000}
	s.Prober = probe
	s.GroupPresent = func(int64) (bool, bool) { return probe.alive, true }
	s.AllPids = func() ([]int64, error) {
		if probe.alive {
			return []int64{pid}, nil
		}
		return nil, nil
	}
	reads := 0
	s.FenceRead = func(string) (stopfence.Record, error) {
		reads++
		if reads <= 2 {
			return stopfence.Record{State: stopfence.StateOpen, Phase: stopfence.PhaseArmed, Generation: 7}, nil
		}
		return stopfence.Record{State: stopfence.StateOpen, Phase: stopfence.PhaseArmed, Generation: 9}, nil
	}
	creation, err := s.BeginCreation("run-launch")
	if err != nil {
		t.Fatal(err)
	}
	defer creation.Close()
	nonce, err := s.Launch(mainCaller, LaunchParams{Id: "rearmed-launch", Kind: "suite", Log: "run.log", FenceGeneration: &creation.Generation})
	if err != nil {
		t.Fatal(err)
	}
	probe.argv = []string{"metasystem", "run", "wrap", "--nonce", nonce}
	if err := s.Bind("rearmed-launch", nonce, pid, pid); err != nil {
		t.Fatal(err)
	}
	originalSignal := stopSignal
	stopSignal = func(int64, syscall.Signal) error { probe.alive = false; return nil }
	t.Cleanup(func() { stopSignal = originalSignal })
	err = s.CompleteLaunch("rearmed-launch", creation.Generation)
	if err == nil || !strings.Contains(err.Error(), "was stopped and armed again") || !strings.Contains(err.Error(), "caller may retry") || strings.Contains(err.Error(), "stop incomplete") {
		t.Fatalf("rearmed creator refusal = %v", err)
	}
	if probe.alive {
		t.Fatal("rearmed creator left its wrapper alive")
	}
}

func TestRegisterRaceFailsRecordAndNeverSignalsForeignProcess(t *testing.T) {
	s := testStore(t)
	s.Prober = fakeProber{verdicts: map[int64]identity.Liveness{51: identity.Alive}, starts: map[int64]int64{51: 5000}}
	reads := 0
	s.FenceRead = func(string) (stopfence.Record, error) {
		reads++
		if reads == 1 {
			return stopfence.Record{State: stopfence.StateOpen, Phase: stopfence.PhaseArmed, Generation: 9}, nil
		}
		return closedFence(), nil
	}
	originalSignal := stopSignal
	signals := 0
	stopSignal = func(int64, syscall.Signal) error { signals++; return nil }
	t.Cleanup(func() { stopSignal = originalSignal })
	err := s.Register(mainCaller, LaunchParams{Id: "race-register", Kind: "custom", Log: "run.log"}, 51, "")
	if err == nil || !strings.Contains(err.Error(), "while run race-register started") {
		t.Fatalf("register race = %v", err)
	}
	record, _ := s.Read("race-register")
	if record.Status != StatusEndedUnknown || record.FenceGeneration != 9 || record.Error == nil || *record.Error != "stopped by metasystem stop" {
		t.Fatalf("raced register record = %+v", record)
	}
	if signals != 0 {
		t.Fatalf("foreign process received %d signals", signals)
	}
	claims, claimErr := stopfence.Claims(s.Root, 100)
	if claimErr != nil || len(claims) != 0 {
		t.Fatalf("creation claim remained: %+v err=%v", claims, claimErr)
	}
}

func TestStopNeverSignalsAdoptedCustody(t *testing.T) {
	s := testStore(t)
	s.Prober = fakeProber{verdicts: map[int64]identity.Liveness{54: identity.Alive}, starts: map[int64]int64{54: 5400}}
	if err := s.Register(mainCaller, LaunchParams{Id: "foreign-run", Kind: "custom", Log: "run.log"}, 54, ""); err != nil {
		t.Fatal(err)
	}
	originalSignal := stopSignal
	signals := 0
	stopSignal = func(int64, syscall.Signal) error { signals++; return nil }
	t.Cleanup(func() { stopSignal = originalSignal })
	outcome, err := s.Stop("foreign-run")
	if err != nil || outcome.Result != StopResultNotStopped || outcome.Reason != "not the metasystem's process, not signalled" {
		t.Fatalf("adopted outcome = %+v err=%v", outcome, err)
	}
	if signals != 0 {
		t.Fatalf("adopted custody received %d signals", signals)
	}
	record, _ := s.Read("foreign-run")
	if record.Status != StatusRunning {
		t.Fatalf("adopted record was altered: %+v", record)
	}
}

func TestAdoptRaceFailsRecordAndNeverSignalsForeignProcess(t *testing.T) {
	s := testStore(t)
	probe := fakeProber{verdicts: map[int64]identity.Liveness{52: identity.Alive}, starts: map[int64]int64{52: 5200}}
	s.Prober = probe
	nonce := launchOne(t, s, "race-adopt")
	if err := s.Bind("race-adopt", nonce, 52, 52); err != nil {
		t.Fatal(err)
	}
	probe.verdicts[52] = identity.Dead
	probe.verdicts[53] = identity.Alive
	probe.starts[53] = 5300
	s.Prober = probe
	reads := 0
	s.FenceRead = func(string) (stopfence.Record, error) {
		reads++
		if reads == 1 {
			return stopfence.Record{State: stopfence.StateOpen, Phase: stopfence.PhaseArmed, Generation: 11}, nil
		}
		return closedFence(), nil
	}
	originalSignal := stopSignal
	signals := 0
	stopSignal = func(int64, syscall.Signal) error { signals++; return nil }
	t.Cleanup(func() { stopSignal = originalSignal })
	err := s.Adopt(mainCaller, "race-adopt", 53)
	if err == nil || !strings.Contains(err.Error(), "while run race-adopt started") {
		t.Fatalf("adopt race = %v", err)
	}
	record, _ := s.Read("race-adopt")
	if record.Status != StatusEndedUnknown || record.FenceGeneration != 11 || record.Error == nil || *record.Error != "stopped by metasystem stop" {
		t.Fatalf("raced adopt record = %+v", record)
	}
	if signals != 0 {
		t.Fatalf("foreign process received %d signals", signals)
	}
}

type argvProber struct {
	alive bool
	pid   int64
	start int64
	argv  []string
}

func (p *argvProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if pid != p.pid || !p.alive {
		return identity.Exact{}, identity.Dead, nil
	}
	return identity.Exact{Pid: pid, StartedAt: time.Unix(p.start, 0), Argv: p.argv, ArgvKnown: true}, identity.Alive, nil
}

func boundWrappedForStop(t *testing.T) (*Store, *argvProber, int64) {
	t.Helper()
	s := testStore(t)
	pid := int64(61)
	probe := &argvProber{alive: true, pid: pid, start: 6000}
	s.Prober = probe
	s.GroupPresent = func(int64) (bool, bool) { return probe.alive, true }
	s.AllPids = func() ([]int64, error) {
		if probe.alive {
			return []int64{pid}, nil
		}
		return nil, nil
	}
	nonce := launchOne(t, s, "stop-wrapped")
	probe.argv = []string{"metasystem", "run", "wrap", "--nonce", nonce}
	if err := s.Bind("stop-wrapped", nonce, pid, pid); err != nil {
		t.Fatal(err)
	}
	return s, probe, pid
}

func TestStopWrappedRunReprovesAndConcludes(t *testing.T) {
	s, probe, pid := boundWrappedForStop(t)
	originalSignal := stopSignal
	var signals []syscall.Signal
	stopSignal = func(pgid int64, signal syscall.Signal) error {
		if pgid != pid {
			return errors.New("wrong group")
		}
		signals = append(signals, signal)
		probe.alive = false
		return nil
	}
	t.Cleanup(func() { stopSignal = originalSignal })
	outcome, err := s.Stop("stop-wrapped")
	if err != nil || outcome.Result != StopResultStopped || outcome.Signal != StopSignalTerm || outcome.Status != StatusEndedUnknown {
		t.Fatalf("stop outcome = %+v err=%v", outcome, err)
	}
	if len(signals) != 1 || signals[0] != syscall.SIGTERM {
		t.Fatalf("signals = %v", signals)
	}
	record, _ := s.Read("stop-wrapped")
	if record.Status != StatusEndedUnknown || record.Error == nil || *record.Error != "stopped by metasystem stop" {
		t.Fatalf("stopped record = %+v", record)
	}
}

func TestStopPreservesSidecarVerdictWrittenDuringOrderlySignal(t *testing.T) {
	s, probe, pid := boundWrappedForStop(t)
	record, err := s.Read("stop-wrapped")
	if err != nil || record == nil {
		t.Fatalf("read wrapped run: record=%+v err=%v", record, err)
	}
	originalSignal := stopSignal
	var signals []syscall.Signal
	stopSignal = func(pgid int64, signal syscall.Signal) error {
		if pgid != pid {
			return errors.New("wrong group")
		}
		signals = append(signals, signal)
		if err := s.WriteSidecar(record.RunId, record.Generation, record.LaunchNonce, 9); err != nil {
			return err
		}
		probe.alive = false
		return nil
	}
	t.Cleanup(func() { stopSignal = originalSignal })
	outcome, err := s.Stop("stop-wrapped")
	if err != nil || outcome.Result != StopResultStopped || outcome.Signal != StopSignalTerm || outcome.Status != StatusRed {
		t.Fatalf("stop outcome = %+v err=%v", outcome, err)
	}
	if len(signals) != 1 || signals[0] != syscall.SIGTERM {
		t.Fatalf("signals = %v", signals)
	}
	concluded, err := s.Read("stop-wrapped")
	if err != nil || concluded == nil || concluded.Status != StatusRed || concluded.ExitCode == nil || *concluded.ExitCode != 9 || concluded.Error != nil {
		t.Fatalf("sidecar verdict was not preserved: record=%+v err=%v", concluded, err)
	}
}

func TestStopReportsWrappedRunThatSurvivesKill(t *testing.T) {
	t.Setenv("METASYSTEM_FIXTURE_CAP_SCALE_MILLI", "1")
	s, _, _ := boundWrappedForStop(t)
	// The liveness seam stays present even after both injected signals: a
	// successful signal syscall is not proof that the group ended.
	s.GroupPresent = func(int64) (bool, bool) { return true, true }
	originalSignal := stopSignal
	var signals []syscall.Signal
	stopSignal = func(_ int64, signal syscall.Signal) error {
		signals = append(signals, signal)
		return nil
	}
	t.Cleanup(func() { stopSignal = originalSignal })
	outcome, err := s.Stop("stop-wrapped")
	if err != nil || outcome.Result != StopResultNotStopped || outcome.Signal != StopSignalKill || outcome.Reason != "group survived KILL" {
		t.Fatalf("survivor outcome = %+v err=%v", outcome, err)
	}
	if len(signals) != 2 || signals[0] != syscall.SIGTERM || signals[1] != syscall.SIGKILL {
		t.Fatalf("signals = %v", signals)
	}
	record, _ := s.Read("stop-wrapped")
	if record.Status != StatusRunning {
		t.Fatalf("surviving run was falsely terminalized: %+v", record)
	}
}
