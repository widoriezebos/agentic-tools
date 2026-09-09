package missionrunner

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

func fixtureOrphanTurn(t *testing.T) (string, Item, *exec.Cmd) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tag := "metasystem-host-orphan-fixture"
	command := exec.Command("sleep", "30")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	started, err := processStartedAt(command.Process.Pid)
	if err != nil {
		command.Process.Kill()
		t.Fatal(err)
	}
	pgid, err := unix.Getpgid(command.Process.Pid)
	if err != nil {
		command.Process.Kill()
		t.Fatal(err)
	}
	table := filepath.Join(root, "fixture-identities.json")
	if err := atomicWriteJSON(table, map[string]any{
		valueString(command.Process.Pid): map[string]any{
			"pidStartedAt": started, "pgid": pgid, "command": "fixture " + tag,
		},
	}); err != nil {
		command.Process.Kill()
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", table)
	recordPath := filepath.Join(root, "artifacts", "agents", "missions", "orphan", "turns", "orphan-t1", "turn.json")
	record := map[string]any{
		"missionId": "orphan", "turnId": "orphan-t1", "runtime": "fake",
		"status": "running", "outcome": "running", "pid": command.Process.Pid,
		"pidStartedAt": started, "pgid": pgid, "instanceTag": tag,
	}
	if err := atomicWriteJSON(recordPath, record); err != nil {
		command.Process.Kill()
		t.Fatal(err)
	}
	item, ok := itemFromRecord(ItemTurn, root, "orphan", "orphan-t1", recordPath, record)
	if !ok {
		command.Process.Kill()
		t.Fatal("fixture item identity is invalid")
	}
	item.Liveness = identity.Alive.String()
	return root, item, command
}

func TestInventoryFindsOrphanTurnWithoutRunner(t *testing.T) {
	root, _, command := fixtureOrphanTurn(t)
	defer command.Wait()
	defer command.Process.Kill()
	items, err := Inventory(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Kind != ItemTurn || items[0].MissionID != "orphan" {
		t.Fatalf("orphan turn inventory: %#v", items)
	}
}

func TestStopTurnReportsSurvivorWhenSignalsDoNothing(t *testing.T) {
	_, item, command := fixtureOrphanTurn(t)
	defer command.Wait()
	defer command.Process.Kill()
	t.Setenv("METASYSTEM_FIXTURE_CAP_SCALE_MILLI", "1")
	original := stopSignal
	stopSignal = func(int, syscall.Signal) error { return nil }
	defer func() { stopSignal = original }()
	outcome, err := Stop(item, StopOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Result != "not-stopped" || outcome.Signal != TerminationKill {
		t.Fatalf("survival outcome: %#v", outcome)
	}
}

func TestRunLoopRefusesClosedFenceBeforeLease(t *testing.T) {
	root := t.TempDir()
	record := stopfence.Record{State: stopfence.StateClosed, Phase: stopfence.PhaseStopped,
		Generation: 4, ChangedAt: "2026-09-07T00:00:00Z", Checkout: root, NotStopped: []stopfence.Survivor{},
		By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 76}}}
	if err := stopfence.Write(root, record); err != nil {
		t.Fatal(err)
	}
	signalPath := filepath.Join(root, "start.json")
	if code := NewEngine(root, "closed").RunLoopAtGeneration("start", "tag", signalPath, 4, false); code != 3 {
		t.Fatalf("closed-fence exit: %d", code)
	}
	if pathExists(filepath.Join(root, "artifacts", "agents", "missions", "closed", "lease.d")) {
		t.Fatal("closed fence allowed a mission lease")
	}
}

func TestRunnerSecondFenceReadReleasesLease(t *testing.T) {
	root := t.TempDir()
	engine := NewEngine(root, "raced")
	engine.fenceGeneration = 3
	engine.fenceRead = func(string) (stopfence.Record, error) {
		return stopfence.Record{State: stopfence.StateClosed, Phase: stopfence.PhaseStopping, Generation: 4}, nil
	}
	signalPath := filepath.Join(root, "start.json")
	if code := engine.internalRunAtGeneration("start", "metasystem-mission-runner-raced-fixture", signalPath, 3, nil); code != 3 {
		t.Fatalf("raced loop exit: %d", code)
	}
	recordPath, _, _ := engine.runnerPaths()
	record, err := readJSONDoc(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	generation, _ := jsonInt(record["fenceGeneration"])
	if record["status"] != "stopped" || generation != 3 {
		t.Fatalf("raced runner record: %#v", record)
	}
	if pathExists(filepath.Join(engine.missionDir(), "lease.d")) || pathExists(filepath.Join(engine.missionDir(), "lease.json")) {
		t.Fatal("raced loop retained its mission lease")
	}
}

func TestRunnerSignalClosesTurnWithoutChangingMissionState(t *testing.T) {
	root := t.TempDir()
	engine := NewEngine(root, "stopping")
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Skip("own process identity is unavailable")
	}
	pgid, err := unix.Getpgid(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	engine.fenceGeneration = 8
	record := engine.runnerRecord(os.Getpid(), pgid, exact.StartedAt.Unix(), "runner-tag")
	recordPath, _, _ := engine.runnerPaths()
	if err := atomicWriteJSON(recordPath, record); err != nil {
		t.Fatal(err)
	}
	item, ok := itemFromRecord(ItemRunner, root, "stopping", "", recordPath, record)
	if !ok {
		t.Fatal("runner identity is invalid")
	}
	if err := atomicWriteJSON(filepath.Join(engine.missionDir(), "stop-intent.json"), map[string]any{
		"schemaVersion": 1, "missionId": "stopping", "runner": processFields(item),
	}); err != nil {
		t.Fatal(err)
	}
	turnPath := filepath.Join(engine.missionDir(), "turns", "stopping-t1", "turn.json")
	if err := atomicWriteJSON(turnPath, map[string]any{
		"missionId": "stopping", "turnId": "stopping-t1", "status": "pending", "outcome": nil,
	}); err != nil {
		t.Fatal(err)
	}
	engine.stopNotifications = make(chan os.Signal, 1)
	engine.stopNotifications <- syscall.SIGTERM
	err = engine.heartbeat("stopping-t1")
	var stopped *runnerStoppedError
	if !errors.As(err, &stopped) {
		t.Fatalf("signal result: %v", err)
	}
	turn, err := readJSONDoc(turnPath)
	if err != nil {
		t.Fatal(err)
	}
	if turn["status"] != "failed" || turn["error"] != "turn-lost" || turn["hostTermination"] != TerminationAlreadyGone {
		t.Fatalf("stopped turn: %#v", turn)
	}
	if pathExists(filepath.Join(engine.missionDir(), "state.json")) {
		t.Fatal("stop signal wrote mission state")
	}
}

func TestStopDeadRunnerReleasesLeaseAndClosesOrphanHost(t *testing.T) {
	root, turn, command := fixtureOrphanTurn(t)
	waited := make(chan struct{})
	go func() { _ = command.Wait(); close(waited) }()
	t.Cleanup(func() { _ = command.Process.Kill(); <-waited })
	t.Setenv("METASYSTEM_FIXTURE_CAP_SCALE_MILLI", "100")

	engine := NewEngine(root, "orphan")
	recordPath, _, _ := engine.runnerPaths()
	// A stale start time proves that this runner identity is gone even if
	// its PID has been reused. Stop must never signal that replacement.
	runner := Item{Kind: ItemRunner, Root: root, MissionID: "orphan", Runtime: "fake",
		RecordPath: recordPath, Pid: int64(os.Getpid()), PidStartedAt: 1,
		Pgid: int64(os.Getpid()), Tag: "stale-runner"}
	record := processFields(runner)
	record["status"] = "running"
	if err := atomicWriteJSON(recordPath, record); err != nil {
		t.Fatal(err)
	}
	leasePath := filepath.Join(engine.missionDir(), "lease.json")
	leaseDir := filepath.Join(engine.missionDir(), "lease.d")
	if err := os.MkdirAll(leaseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := atomicWriteJSON(leasePath, processFields(runner)); err != nil {
		t.Fatal(err)
	}
	intentPath := filepath.Join(engine.missionDir(), "stop-intent.json")
	if err := atomicWriteJSON(intentPath, map[string]any{"missionId": "orphan"}); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(engine.missionDir(), "state.json")
	state := []byte("{\"state\":\"active\"}\n")
	if err := os.WriteFile(statePath, state, 0o644); err != nil {
		t.Fatal(err)
	}

	outcome, err := Stop(runner, StopOptions{})
	if err != nil || outcome.Result != "already-gone" {
		t.Fatalf("dead runner stop: %#v, %v", outcome, err)
	}
	<-waited
	stoppedRunner, err := readJSONDoc(recordPath)
	if err != nil || stoppedRunner["status"] != "stopped" {
		t.Fatalf("runner conclusion: %#v, %v", stoppedRunner, err)
	}
	stoppedTurn, err := readJSONDoc(turn.RecordPath)
	if err != nil || stoppedTurn["status"] != "failed" || stoppedTurn["error"] != "turn-lost" || stoppedTurn["hostTermination"] != TerminationTerm {
		t.Fatalf("orphan host conclusion: %#v, %v", stoppedTurn, err)
	}
	for _, path := range []string{leasePath, leaseDir, intentPath} {
		if pathExists(path) {
			t.Fatalf("stop retained %s", path)
		}
	}
	if after, err := os.ReadFile(statePath); err != nil || string(after) != string(state) {
		t.Fatalf("stop changed mission state: %q, %v", after, err)
	}
}

func TestStopLiveRunnerSignalsOwnedGroup(t *testing.T) {
	root, runner, command := fixtureOrphanTurn(t)
	waited := make(chan struct{})
	go func() { _ = command.Wait(); close(waited) }()
	t.Cleanup(func() { _ = command.Process.Kill(); <-waited })
	t.Setenv("METASYSTEM_FIXTURE_CAP_SCALE_MILLI", "100")
	runner.Kind = ItemRunner
	engine := NewEngine(root, runner.MissionID)
	runner.RecordPath, _, _ = engine.runnerPaths()
	record := processFields(runner)
	record["status"] = "running"
	if err := atomicWriteJSON(runner.RecordPath, record); err != nil {
		t.Fatal(err)
	}

	outcome, err := Stop(runner, StopOptions{})
	if err != nil || outcome.Result != "stopped" || outcome.Signal != TerminationTerm {
		t.Fatalf("live runner stop: %#v, %v", outcome, err)
	}
	<-waited
	intent, err := readJSONDoc(filepath.Join(engine.missionDir(), "stop-intent.json"))
	if err != nil || intent["missionId"] != runner.MissionID || !intentMatchesRunner(intent, record) {
		t.Fatalf("stop intent did not bind the signalled runner: %#v, %v", intent, err)
	}
}
