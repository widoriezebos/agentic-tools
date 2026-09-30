package laneengine

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// startGroupLeader starts a real process leading its own process group and
// returns its exact identity; cleanup ends it by its literal pid.
func startGroupLeader(t *testing.T) (*exec.Cmd, identity.Ref) {
	t.Helper()
	command := exec.Command("sleep", "120")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	})
	exact, state, err := (identity.KernelProber{}).Probe(int64(command.Process.Pid))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe the started process: %v %v", state, err)
	}
	return command, exact.Ref()
}

// writeCustodyRecord writes one custody record into the host lane state as
// the kernel records a launched execution: its launcher, its child and the
// child's process group.
func writeCustodyRecord(t *testing.T, home, id, kind string, issuer, child identity.Ref) {
	t.Helper()
	encode := func(ref identity.Ref) string {
		if ref.Pid == 0 {
			return ""
		}
		encoded, err := identity.EncodeRef(ref)
		if err != nil {
			t.Fatal(err)
		}
		return encoded
	}
	record := map[string]any{"schema": 1, "id": id, "kind": kind, "subject": "validation cadence-run-1",
		"openedAt": "2026-09-30T18:00:00Z", "issuer": encode(issuer)}
	if child.Pid != 0 {
		record["child"], record["groups"] = encode(child), []int64{child.Pid}
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(lane.HostDir(home), "landing-custody")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// K9: a validation the kernel launched is custody like a proof. While its
// process lives the engine does not move, although no proof holds the
// host's proving lock; once it has ended and its group is empty, the
// advance goes through.
func TestValidationCustodyBlocksAdvance(t *testing.T) {
	t.Parallel()
	bed := newAdvanceBed(t)
	bed.enrollOld(t)
	command, child := startGroupLeader(t)
	self, _, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	writeCustodyRecord(t, bed.home, "validate-0123456789abcdef", "validate", self.Ref(), child)
	_, err = bed.advance(t)
	bed.untouched(t, err, CodeAdvanceCustodyLive)
	_ = command.Process.Kill()
	_ = command.Wait()
	if outcome, err := bed.advance(t); err != nil || !outcome.Changed {
		t.Fatalf("advance after the validation ended = %+v %v", outcome, err)
	}
}

// Custody that can't be read (here: a launcher that died before it
// recorded what it started) holds the engine until a person goes past it.
func TestUnknownCustodyNeedsAPersonToAdvance(t *testing.T) {
	t.Parallel()
	bed := newAdvanceBed(t)
	bed.enrollOld(t)
	command, launcher := startGroupLeader(t)
	_ = command.Process.Kill()
	_ = command.Wait()
	writeCustodyRecord(t, bed.home, "prove-0123456789abcdef", "prove", launcher, identity.Ref{})
	_, err := bed.advance(t)
	bed.untouched(t, err, CodeAdvanceCustodyUnknown)
	request := bed.request(t)
	request.Force, request.By = true, "Wido"
	if outcome, err := Advance(request, ProductionConditions(bed.home, bed.checkout), bed.steps(t)); err != nil || !outcome.Changed {
		t.Fatalf("a person's forced advance = %+v %v", outcome, err)
	}
	// The person's word is recorded: the record no longer holds the lane.
	data, err := os.ReadFile(filepath.Join(lane.HostDir(bed.home), "landing-custody", "prove-0123456789abcdef.json"))
	if err != nil || !strings.Contains(string(data), `"forced": "Wido"`) {
		t.Fatalf("the forced record = %s %v; want it settled in Wido's name", data, err)
	}
}
