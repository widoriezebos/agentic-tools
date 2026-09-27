package supervise

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

// TestSupBOrderlyShutdownEndsTheOwnerAndReportsItsComponentsGoneByTheOwner
// ports the supervision half of the stop-everything bed (supervision-fixtures
// part B): an orderly owner exits reason=shutdown on TERM and takes its
// watcher, reaper and landing owner with it, so the stop reports each
// component "already gone (by the owner)", releases the owner lock and
// leaves one exited row with reason shutdown in the registry.
func TestSupBOrderlyShutdownEndsTheOwnerAndReportsItsComponentsGoneByTheOwner(t *testing.T) {
	root := t.TempDir()
	registryPath := isolatedArmingRegistry(t)
	if err := os.MkdirAll(ownerLockDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	owner := ArmingOwner{Pid: 41, PidStartedAt: 100, InstanceTag: "metasystem-supervision-owner-test-bed-b"}
	if err := WriteArmingOwner(root, owner); err != nil {
		t.Fatal(err)
	}
	appendPreviousOwnerRelaunched(t, registryPath, root, owner, 1)
	components := map[string]stateComponent{}
	for index, component := range []Component{Watcher, Reaper, LandingOwner} {
		components[string(component)] = stateComponent{
			Pid: int64(71 + index), PidStartedAt: int64(200 + index),
			InstanceTag: owner.InstanceTag + "-" + string(component) + "-1",
		}
	}
	document := stateDocument{
		Owner:      stateIdentity{Pid: owner.Pid, PidStartedAt: owner.PidStartedAt, InstanceTag: owner.InstanceTag},
		Generation: 1, TeardownCeilingSec: 1, Components: components,
	}
	if err := writeArmingHelperJSON(filepath.Join(SupervisionDir(root), "state.json"), document); err != nil {
		t.Fatal(err)
	}

	ownerDead := false
	var signals []syscall.Signal
	priorLiveness, priorSignal := armingOwnerLiveness, armingOwnerSignal
	priorControl, priorEnumeration := takeoverComponentControl, enumerateTakeoverProcesses
	armingOwnerLiveness = func(ArmingOwner) identity.Liveness {
		if ownerDead {
			return identity.Dead
		}
		return identity.Alive
	}
	ledger := previousOwnerLedger(t, registryPath, root, owner)
	armingOwnerSignal = func(_ int64, signal syscall.Signal) error {
		signals = append(signals, signal)
		if signal == syscall.SIGTERM {
			// The orderly owner ends its own components and records why it left.
			ledger.AppendExited("shutdown", "signal-induced exit", true)
			ownerDead = true
		}
		return nil
	}
	componentSignals := 0
	takeoverComponentControl = func() recordedComponentControl {
		return recordedComponentControl{
			prober: &armingComponentProbe{state: identity.Dead},
			groupAbsent: func(int64) (bool, error) {
				return true, nil
			},
			signalGroup: func(int64, syscall.Signal) error {
				componentSignals++
				return nil
			},
		}
	}
	enumerateTakeoverProcesses = func(string) ([]census.Process, error) { return nil, nil }
	t.Cleanup(func() {
		armingOwnerLiveness, armingOwnerSignal = priorLiveness, priorSignal
		takeoverComponentControl, enumerateTakeoverProcesses = priorControl, priorEnumeration
	})

	report, err := ShutdownAt(root, root, root, "metasystem-supervision-owner-test-", 1)
	if err != nil || !report.Complete() {
		t.Fatalf("orderly shutdown = %+v err=%v", report, err)
	}
	if len(signals) != 1 || signals[0] != syscall.SIGTERM || componentSignals != 0 {
		t.Fatalf("owner signals = %v, component signals = %d; want one TERM to the owner only", signals, componentSignals)
	}
	want := strings.Join([]string{
		"supervision-owner pid 41 tag " + owner.InstanceTag + " generation 1: stopped (TERM, exited reason=shutdown)",
		"repo-watcher pid 71: already gone (by the owner)",
		"job-reaper pid 72: already gone (by the owner)",
		"landing-batch-owner pid 73: already gone (by the owner)",
	}, "\n")
	if got := strings.Join(report.Lines(), "\n"); got != want {
		t.Fatalf("orderly shutdown lines:\n%s\nwant:\n%s", got, want)
	}
	if _, statErr := os.Stat(ownerLockDir(root)); !os.IsNotExist(statErr) {
		t.Fatalf("orderly shutdown left the supervision owner lock: %v", statErr)
	}
	frames, err := registry.ReadFrames(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	exited := 0
	for _, frame := range frames {
		record, parseErr := registry.ParseRecord(frame.Record)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		if record.Event == registry.EventExited {
			exited++
			if record.OwnerTag != owner.InstanceTag || record.Reason != "shutdown" {
				t.Fatalf("exited row = %+v, want owner %s reason shutdown", record, owner.InstanceTag)
			}
		}
	}
	if exited != 1 {
		t.Fatalf("registry carries %d exited rows, want the owner's one orderly exit", exited)
	}
}
