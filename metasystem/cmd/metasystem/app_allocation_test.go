package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/applaunch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot/stateroottest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
)

func TestAppStartAtRefRefusesMissingPortRange(t *testing.T) {
	t.Parallel()
	contract := appHTTPContract(appFixtureApp(t), appHeldPort(t).address)
	delete(contract, "portRange")
	bed := newAppBed(t, contract)
	code, out := bed.run("app", "start", "--at", "main")
	if code == 0 || !strings.Contains(out, "a run at another commit needs an address of its own, but the contract declares no portRange") {
		t.Fatalf("missing range was not refused: %d\n%s", code, out)
	}
	if record, err := applaunch.ReadRecord(bed.installation, applaunch.KeyFor("main")); err == nil {
		t.Fatalf("a refused allocation launched a run: %+v", record)
	}
}

func TestAppStartAtRefRefusesExhaustedPortRange(t *testing.T) {
	t.Parallel()
	ports := appAllocationPorts(t)
	contract := appHTTPContract(appFixtureApp(t), ports[0].address)
	contract["portRange"] = appAllocationRange(ports)
	bed := newAppBed(t, contract)
	code, out := bed.run("app", "start", "--at", "main")
	var low, high int
	fmt.Sscanf(appAllocationRange(ports), "%d-%d", &low, &high)
	want := fmt.Sprintf("every port from %d to %d is taken; free one or widen portRange", low, high)
	if code == 0 || !strings.Contains(out, want) {
		t.Fatalf("exhausted range was not refused: %d\n%s", code, out)
	}
	if record, err := applaunch.ReadRecord(bed.installation, applaunch.KeyFor("main")); err == nil {
		t.Fatalf("a refused allocation launched a run: %+v", record)
	}
}

func TestAppAllocationReusesRecordedAddressBeforeWalkingRange(t *testing.T) {
	t.Parallel()
	ports := appAllocationPorts(t)
	root := t.TempDir()
	run := appRun{
		roots:    lifecycle.Roots{Installation: stateroottest.Installation(t, root)},
		contract: applaunch.Contract{Address: ports[0].address, PortRange: appAllocationRange(ports)},
		ref:      "main", key: applaunch.KeyFor("main"),
	}
	recorded := appHeldPort(t)
	if err := applaunch.WriteRecord(root, applaunch.Record{Key: run.key, Address: recorded.address, Supervisor: appAllocationSupervisor(t)}); err != nil {
		t.Fatal(err)
	}
	recorded.listener.Close()
	if err := run.allocateAddress(); err != nil || run.address != recorded.address {
		t.Fatalf("the free recorded address must be reused even with the whole range occupied: address=%s err=%v", run.address, err)
	}
}

func TestAppAllocationWalkSkipsTakenRecordsAndOccupiedPorts(t *testing.T) {
	t.Parallel()
	ports := appAllocationPorts(t)
	root := t.TempDir()
	run := appRun{
		roots:    lifecycle.Roots{Installation: stateroottest.Installation(t, root)},
		contract: applaunch.Contract{Address: ports[0].address, PortRange: appAllocationRange(ports)},
		ref:      "main", key: applaunch.KeyFor("main"),
	}
	// This run's recorded address is occupied, so it must fall back to the range.
	supervisor := appAllocationSupervisor(t)
	if err := applaunch.WriteRecord(root, applaunch.Record{Key: run.key, Address: ports[0].address, Supervisor: supervisor}); err != nil {
		t.Fatal(err)
	}
	// A different run owns a currently free range port; that record still reserves it.
	if err := applaunch.WriteRecord(root, applaunch.Record{Key: applaunch.KeyFor("goal/g1"), Address: ports[2].address, Supervisor: supervisor}); err != nil {
		t.Fatal(err)
	}
	ports[2].listener.Close()
	ports[3].listener.Close()
	if err := run.allocateAddress(); err != nil || run.address != ports[3].address {
		t.Fatalf("the range walk must skip the occupied port and the other run's record: address=%s err=%v", run.address, err)
	}
}

func appAllocationSupervisor(t *testing.T) string {
	t.Helper()
	self, _, err := identity.KernelProber{}.Probe(int64(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := identity.EncodeRef(self.Ref())
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestAppBedPreservesContractStopWait(t *testing.T) {
	t.Parallel()
	gate, _ := appExitGate(t)
	bed := newAppBed(t, map[string]any{
		"start":  map[string]any{"argv": []string{appFixtureApp(t), "--no-listen", "--exit-fifo", gate}},
		"stopMs": 4000,
	})
	contract, err := applaunch.Load(filepath.Join(bed.installation, "launch.json"))
	if err != nil || contract.StopWaitMS() != 4000 {
		t.Fatalf("the fixture must preserve the exit contract's stop wait: stopMs=%d err=%v", contract.StopWaitMS(), err)
	}
}
