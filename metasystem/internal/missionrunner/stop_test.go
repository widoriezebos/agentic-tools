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
