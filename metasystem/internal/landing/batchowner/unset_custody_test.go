package batchowner

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/custody"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// startOrphanedGroup starts a shell leading its own process group that
// leaves a sleeping child in the group and exits: the group outlives its
// leader. It returns the dead leader's identity; cleanup ends the group by
// its literal id.
func startOrphanedGroup(t *testing.T) identity.Ref {
	t.Helper()
	command := exec.Command("sh", "-c", "sleep 120 & echo started")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	group := command.Process.Pid
	t.Cleanup(func() { _ = syscall.Kill(-group, syscall.SIGKILL) })
	exact, state, err := (identity.KernelProber{}).Probe(int64(group))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe the group leader: %v %v", state, err)
	}
	buffer := make([]byte, 16)
	if n, _ := stdout.Read(buffer); !strings.HasPrefix(string(buffer[:n]), "started") {
		t.Fatalf("the group leader said %q", buffer[:n])
	}
	if err := command.Wait(); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Kill(-group, 0); err != nil {
		t.Fatalf("the group emptied with its leader: %v", err)
	}
	return exact.Ref()
}

// K9: unset's settle step waits on the custody store, not only on the host
// proving lock. A validation whose process has ended while its process
// group still runs (what a run store would already call terminal) is live.
func TestUnsetWaitsForAValidationGroupThatOutlivesItsLeader(t *testing.T) {
	t.Parallel()
	bed := newUnsetBed(t)
	leader := startOrphanedGroup(t)
	encoded, err := identity.EncodeRef(leader)
	if err != nil {
		t.Fatal(err)
	}
	record, err := json.Marshal(map[string]any{"schema": 1, "id": "validate-00000000000000aa", "kind": "validate", "subject": "validation cadence-run-1",
		"openedAt": unsetNow.Format(time.RFC3339), "issuer": encoded, "child": encoded, "groups": []map[string]any{{"id": leader.Pid, "leader": encoded}}})
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(lane.HostDir(bed.home), "landing-custody")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "validate-00000000000000aa.json"), record, 0o600); err != nil {
		t.Fatal(err)
	}
	steps := UnsetLane{Home: bed.home, By: "Wido", Now: func() time.Time { return unsetNow }}
	settlement, err := steps.settle(bed.layout)
	if err != nil || settlement.Settled(true) || !strings.Contains(strings.Join(settlement.Live, "\n"), "process group "+strconv.Itoa(int(leader.Pid))) {
		t.Fatalf("settle with a validation group that outlives its leader = %+v %v; want it live", settlement, err)
	}
}

// A person's landing unset --force past custody that can't be read records
// whom it went past: the record is settled in the person's name.
func TestUnsetForceRecordsThePersonPastUnknownCustody(t *testing.T) {
	t.Parallel()
	bed := newUnsetBed(t)
	gone := exec.Command("sleep", "120")
	if err := gone.Start(); err != nil {
		t.Fatal(err)
	}
	exact, _, err := (identity.KernelProber{}).Probe(int64(gone.Process.Pid))
	_ = gone.Process.Kill()
	_ = gone.Wait()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := identity.EncodeRef(exact.Ref())
	if err != nil {
		t.Fatal(err)
	}
	// Opened by a launcher that died before it recorded what it started.
	record, err := json.Marshal(map[string]any{"schema": 1, "id": "prove-00000000000000bb", "kind": "prove", "subject": "batch b1",
		"openedAt": unsetNow.Format(time.RFC3339), "issuer": encoded})
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(lane.HostDir(bed.home), "landing-custody")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "prove-00000000000000bb.json")
	if err := os.WriteFile(path, record, 0o600); err != nil {
		t.Fatal(err)
	}
	steps := UnsetLane{Home: bed.home, By: "Wido", Now: func() time.Time { return unsetNow },
		Calls: LaneCallSet{
			Handover: func(ownercall.Invocation, ownercall.HandoverRequest) error { return nil },
			EditNext: func(ownercall.Invocation, string, string, string) error { return nil },
			Release:  func(ownercall.Invocation, string, string) error { return nil },
		},
		Custody: func(lane.Layout) custody.Probes {
			return custody.Probes{}
		}}
	report, err := lane.Unset(bed.home, "Wido", unsetNow, false, steps.Seams())
	if err != nil || report.Stopped != lane.StepSettled || len(report.Settlement.Unknown) != 1 {
		t.Fatalf("unset without force = %+v %v; want it stopped at the unknown custody", report, err)
	}
	if _, err := lane.Unset(bed.home, "Wido", unsetNow, true, steps.Seams()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), `"forced": "Wido"`) {
		t.Fatalf("the record the forced unset went past = %s %v; want it settled in Wido's name", data, err)
	}
}
