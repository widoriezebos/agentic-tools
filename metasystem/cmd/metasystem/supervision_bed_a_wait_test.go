package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// supAWriteJob writes one delegate job record in the shape the wait source
// reads.
func supAWriteJob(t *testing.T, root, job, status string) {
	t.Helper()
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	record := `{"jobId":"` + job + `","operationId":"reserve-` + job + `","round":1,"status":"` + status + `","startedAt":"2026-09-13T12:00:00Z"}`
	if err := os.WriteFile(filepath.Join(jobs, job+".json"), []byte(record), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Ported from supervision-fixtures.sh wait-restart. The bed named
// TestWaitRestartRecoveryReplay, which de01a54b2 renamed to
// TestSuccessorRefusesWrongSessionRow, so the bed selected nothing in
// internal/run and its installed leg (gated on METASYSTEM_WAIT_BINARY, with
// a sleep poll for the pending row) had not run since. Here the package's
// TestMain always supplies the source-built candidate, and the waiter's own
// registration descriptor says when the row is pending: an installed waiter
// killed while pending leaves a durable row that an installed resume
// recovers to the source's terminal result.
func TestSupervisionBedAWaitRestartKilledWaiterResumesFromItsRow(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	self := int64(os.Getpid())
	exact, state, err := (identity.KernelProber{}).Probe(self)
	if err != nil || state != identity.Alive {
		t.Fatalf("current process identity=%+v state=%s err=%v", exact, state, err)
	}
	if _, err := lease.AnnounceWithPair(root, "wait-restart-session", self, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "wait-restart-test", "fake", "wait-restart-lineage"); err != nil {
		t.Fatal(err)
	}
	supAWriteJob(t, root, "job-restart", "running")

	readyRead, readyWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	fixture := testutil.Fixture(t)
	waiter := exec.Command(commandTestExecutable(t), waitHelperCommand, "--root", root, "--job", "job-restart", "--timeout", "1m", "--json")
	waiter.Env = fixture.Env(append(os.Environ(), waitRegisteredFDEnvironment+"=3", "GO_WANT_BATCH_E2E_COMMAND=1"))
	waiter.ExtraFiles = []*os.File{readyWrite}
	var waiterOutput strings.Builder
	waiter.Stdout, waiter.Stderr = &waiterOutput, &waiterOutput
	if err := waiter.Start(); err != nil {
		_ = readyRead.Close()
		_ = readyWrite.Close()
		t.Fatal(err)
	}
	_ = readyWrite.Close()
	fixture.Record(waiter.Process.Pid)
	done := make(chan error, 1)
	go func() { done <- waiter.Wait() }()
	joined := false
	t.Cleanup(func() {
		if !joined {
			_ = waiter.Process.Kill()
			<-done
		}
	})
	line, readErr := bufio.NewReader(readyRead).ReadString('\n')
	_ = readyRead.Close()
	waitID := strings.TrimSpace(line)
	if readErr != nil || !metarun.ValidWaitID(waitID) {
		_ = waiter.Process.Kill()
		joined = true
		waitErr := <-done
		t.Fatalf("installed waiter registration signal=%q err=%v exit=%v output=%s", line, readErr, waitErr, waiterOutput.String())
	}
	row, _, err := metarun.FindWaiterByID(root, waitID)
	if err != nil || row.State != "pending" || row.Kind != "job" || row.TargetID != "job-restart" || row.Pid != int64(waiter.Process.Pid) {
		t.Fatalf("registered row=%+v err=%v", row, err)
	}
	if err := waiter.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	joined = true
	var exit *exec.ExitError
	if waitErr := <-done; !errors.As(waitErr, &exit) {
		t.Fatalf("killed installed waiter did not end by its signal: %v output=%s", waitErr, waiterOutput.String())
	}
	if orphan, _, err := metarun.FindWaiterByID(root, waitID); err != nil || orphan.State != "pending" {
		t.Fatalf("the killed waiter's row did not stay pending for recovery: row=%+v err=%v", orphan, err)
	}

	supAWriteJob(t, root, "job-restart", "completed")
	resume := exec.Command(commandTestExecutable(t), waitHelperCommand, "--root", root, "--resume", waitID, "--json")
	resume.Env = fixture.Env(append(os.Environ(), "GO_WANT_BATCH_E2E_COMMAND=1"))
	resumed, err := resume.CombinedOutput()
	if err != nil {
		t.Fatalf("installed resume did not recover from the row: err=%v output=%s", err, resumed)
	}
	var result metarun.WaitResult
	if err := json.Unmarshal(resumed, &result); err != nil || result.ExitCode != metarun.ExitGreen || result.State != "ready" {
		t.Fatalf("installed resume result=%+v err=%v output=%s", result, err, resumed)
	}
	final, _, err := metarun.FindWaiterByID(root, result.WaitID)
	if err != nil || final.State == "pending" {
		t.Fatalf("the recovered wait stayed pending: row=%+v err=%v", final, err)
	}
}
