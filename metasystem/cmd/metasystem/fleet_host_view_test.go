package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostcapacity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestFleetHostViewPublicMachineList(t *testing.T) {
	t.Parallel()
	b := newMachineBed(t)
	b.dead[557] = true
	store := launch.Store{Root: b.launchDir}
	for _, record := range []launch.Record{
		{ID: "reserved-build", Kind: "build", Goal: "g-2", State: launch.Starting, WorkingDirectory: b.other},
		{ID: "same-goal", Kind: "build", Goal: "g-1", State: launch.Running, WorkingDirectory: b.other, Supervisor: &identity.Ref{Pid: 556, StartedAtSec: 556}},
		{ID: "dead-supervisor", Kind: "build", Goal: "g-dead", State: launch.Running, WorkingDirectory: b.other, Supervisor: &identity.Ref{Pid: 557, StartedAtSec: 557}},
		{ID: "finished", Kind: "build", Goal: "g-3", State: launch.Completed},
		{ID: "reading", Kind: "read", Goal: "g-4", State: launch.Running},
	} {
		if err := store.Create(record); err != nil {
			t.Fatal(err)
		}
	}
	owners := b.owners()
	now := machineBedNow
	load := hostload.Sample{Available: true, Cores: 18, Load1m: 2, Load5m: 3, Load15m: 4}
	owners.landing.now = func() time.Time { return now }
	owners.machines.capacitySources.Load = func(at time.Time) hostload.Sample {
		sample := load
		sample.At = at.UTC().Format(time.RFC3339Nano)
		return sample
	}
	read := func(checkout string) hostcapacity.Snapshot {
		t.Helper()
		command, args, _ := resolveIntentArgv([]string{"machine", "list", "--json"})
		var out, stderr bytes.Buffer
		code := runIntentIn(command, args, &out, &stderr, checkout, owners)
		var result struct {
			Data struct {
				ThisComputer struct {
					Capacity hostcapacity.Snapshot `json:"capacity"`
				} `json:"thisComputer"`
			} `json:"data"`
		}
		if err := json.Unmarshal(out.Bytes(), &result); err != nil || code != 0 {
			t.Fatalf("machine list exit %d: %s %s (%v)", code, out.String(), stderr.String(), err)
		}
		return result.Data.ThisComputer.Capacity
	}
	fixedLoad := owners.machines.capacitySources.Load
	owners.machines.capacitySources.Load = nil
	physical := read(b.this)
	if len(physical.Builds) != 3 || !reflect.DeepEqual(physical.Goals, []string{"g-1", "g-2"}) {
		t.Fatalf("first machine list includes a build whose supervisor and child are dead: %+v", physical)
	}
	dead, err := store.Read("dead-supervisor")
	if err != nil || dead.State != launch.Failed || dead.Reason != "supervisor-lost" {
		t.Fatalf("dead build was not reconciled: %+v %v", dead, err)
	}
	if physical.Load.Cores < 1 || physical.Load.At != physical.At {
		t.Fatalf("the production host-load reader was not sampled at observation time: %+v", physical)
	}
	owners.machines.capacitySources.Load = fixedLoad
	first, second := read(b.this), read(b.other)
	if !reflect.DeepEqual(first, second) || !first.OwnerKnown || first.Owner.Root != b.landing || first.Owner.Install != b.landing || first.Owner.CustodyEpoch == 0 {
		t.Fatalf("seats disagree or lack the registered owner: %+v / %+v", first, second)
	}
	if !first.BuildsKnown || len(first.Builds) != 3 || !reflect.DeepEqual(first.Goals, []string{"g-1", "g-2"}) || first.Providers.Available || !strings.Contains(first.Providers.Detail, "unavailable") {
		t.Fatalf("active builds, distinct goals or provider coverage: %+v", first)
	}
	now = now.Add(time.Minute)
	load.Load1m = 9
	fresh := read(b.this)
	if fresh.Load.Load1m != 9 || fresh.At == first.At || fresh.Load.At != fresh.At {
		t.Fatalf("host reading was cached: %+v", fresh)
	}
	command, args, _ := resolveIntentArgv([]string{"machine", "list"})
	var out, stderr bytes.Buffer
	if code := runIntentIn(command, args, &out, &stderr, b.this, owners); code != 0 {
		t.Fatalf("text machine list exit %d: %s", code, stderr.String())
	}
	for _, visible := range []string{"Host capacity", "load 9.00", "18 cores", filepath.Base(b.landing), "reserved-build", "starting", "providers:", "unavailable"} {
		if !strings.Contains(out.String(), visible) {
			t.Fatalf("machine list hides %q: %s", visible, out.String())
		}
	}
	owners.machines.capacitySources.Load = func(at time.Time) hostload.Sample {
		registerLane(t, b.home, b.other, "Wido", at)
		sample := load
		sample.At = at.UTC().Format(time.RFC3339Nano)
		return sample
	}
	changed := read(b.this)
	if changed.OwnerKnown || !strings.Contains(strings.Join(changed.Errors, " "), "ownership changed") || len(changed.Builds) != 3 {
		t.Fatalf("changed ownership presented as known: %+v", changed)
	}
	owners.machines.capacitySources.Load = func(at time.Time) hostload.Sample {
		return hostload.Sample{At: at.UTC().Format(time.RFC3339Nano), Cores: 18, Detail: "kernel load source unreadable"}
	}
	if err := os.WriteFile(lane.RecordPath(b.home), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.launchDir, "reserved-build", "record.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	unknown := read(b.other)
	if unknown.OwnerKnown || unknown.Load.Available || unknown.BuildsKnown || len(unknown.Errors) < 3 || unknown.Load.Cores != 18 {
		t.Fatalf("unreadable sources became healthy: %+v", unknown)
	}
	out.Reset()
	stderr.Reset()
	if code := runIntentIn(command, args, &out, &stderr, b.other, owners); code != 0 {
		t.Fatalf("partial machine list exit %d: %s", code, stderr.String())
	}
	for _, visible := range []string{"load unknown", "18 cores", "registered owner unknown", "active builds unknown", "automatic capacity is unknown"} {
		if !strings.Contains(out.String(), visible) {
			t.Fatalf("partial machine list hides %q: %s", visible, out.String())
		}
	}
	if err := os.Remove(lane.RecordPath(b.home)); err != nil {
		t.Fatal(err)
	}
	missing := read(b.this)
	if missing.OwnerKnown || !strings.Contains(strings.Join(missing.Errors, " "), "metasystem landing set PATH") {
		t.Fatalf("missing owner lacks the existing remedy: %+v", missing)
	}
	if value := config.MustDefault("proof.admission.top-level-max"); value != "1" {
		t.Fatalf("compiled proof default = %q", value)
	}
	proveFleetDefaultProofSerialization(t)
}

type fleetProofReadyOutput struct {
	mu     sync.Mutex
	buffer bytes.Buffer
	ready  chan struct{}
	once   sync.Once
}

func (w *fleetProofReadyOutput) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.buffer.Write(data)
	if strings.Contains(w.buffer.String(), "fleet-proof-ready") {
		w.once.Do(func() { close(w.ready) })
	}
	return n, err
}

func proveFleetDefaultProofSerialization(t *testing.T) {
	t.Helper()
	engine := testenv.Link(t, testenv.Engine(t), filepath.Join(t.TempDir(), "metasystem"))
	admission := filepath.Join(t.TempDir(), "host-admission")
	firstRoot, secondRoot := t.TempDir(), t.TempDir()
	for _, root := range []string{firstRoot, secondRoot} {
		if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if cap, err := proofrun.ResolveAdmissionCap("", 18); err != nil || cap.Max != 1 {
		t.Fatalf("no configuration path: %+v %v", cap, err)
	}
	gate := filepath.Join(firstRoot, "release.fifo")
	if err := syscall.Mkfifo(gate, 0o600); err != nil {
		t.Fatal(err)
	}
	fixture := proofBinaryFixture{t: t}
	env := append(receiptCanaryEnvironment(), "METASYSTEM_PROOF_ADMISSION_TEST_DIR="+admission, "METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT="+firstRoot)
	first := fixture.command(env, engine, "proof-run", "launch", "--suite", "fleet-first", "--root", firstRoot,
		"--conf", filepath.Join(firstRoot, "metasystem.conf"), "--log", filepath.Join(firstRoot, "suite.log"), "--progress", filepath.Join(firstRoot, "progress.jsonl"), "--banner", "fleet first",
		"--", "bash", "-c", `printf 'fleet-proof-ready\n'; read -r release < "$1"`, "fixture", gate)
	ready := &fleetProofReadyOutput{ready: make(chan struct{})}
	first.Stdout, first.Stderr = ready, ready
	if err := first.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Process.Kill() })
	firstDone := make(chan error, 1)
	go func() { firstDone <- first.Wait() }()
	select {
	case <-ready.ready:
	case err := <-firstDone:
		t.Fatalf("first proof exited before holding its slot: %v\n%s", err, ready.buffer.String())
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	}
	child := filepath.Join(secondRoot, "child-ran")
	env = append(receiptCanaryEnvironment(), "METASYSTEM_PROOF_ADMISSION_TEST_DIR="+admission, "METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT="+secondRoot)
	second := fixture.command(env, engine, "proof-run", "launch", "--suite", "fleet-second", "--root", secondRoot,
		"--conf", filepath.Join(secondRoot, "metasystem.conf"), "--log", filepath.Join(secondRoot, "suite.log"), "--progress", filepath.Join(secondRoot, "progress.jsonl"), "--banner", "fleet second",
		"--", "bash", "-c", `printf 'ran\n' > "$1"`, "fixture", child)
	waiting := &publicCapacityWaitOutput{waiting: make(chan struct{})}
	second.Stdout, second.Stderr = waiting, waiting
	if err := second.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Process.Kill() })
	secondDone := make(chan error, 1)
	go func() { secondDone <- second.Wait() }()
	select {
	case <-waiting.waiting:
	case err := <-secondDone:
		t.Fatalf("second unconfigured checkout bypassed serialization: %v\n%s", err, waiting.String())
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	}
	if _, err := os.Stat(child); !os.IsNotExist(err) {
		t.Fatalf("second child ran while the first proof held the slot: %v", err)
	}
	writer, err := os.OpenFile(gate, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = writer.WriteString("release\n")
	_ = writer.Close()
	if err != nil {
		t.Fatal(err)
	}
	for _, done := range []chan error{firstDone, secondDone} {
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("serialized proof failed: %v\n%s", err, waiting.String())
			}
		case <-t.Context().Done():
			t.Fatal(t.Context().Err())
		}
	}
	if data, err := os.ReadFile(child); err != nil || string(data) != "ran\n" {
		t.Fatalf("second proof did not resume: %q %v", data, err)
	}
	assertHostAdmissionClean(t, admission, 1)
}
