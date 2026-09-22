package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

type waitRegisterFixtureProber struct {
	exact identity.Exact
	state identity.Liveness
	err   error
}

func (p waitRegisterFixtureProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if p.exact.Pid != 0 && pid != p.exact.Pid {
		return identity.Exact{}, identity.Dead, nil
	}
	return p.exact, p.state, p.err
}

func waitRegisterCommandFixture(t *testing.T) (string, int64, string, time.Time) {
	t.Helper()
	root := t.TempDir()
	now := time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(goalNowEnvironment, now.Format(time.RFC3339Nano))
	t.Setenv(goalBootIDEnvironment, "system-boot")
	t.Setenv(goalBootNanosEnvironment, fmt.Sprint((2 * time.Hour).Nanoseconds()))
	self := int64(os.Getpid())
	exact, state, err := (identity.KernelProber{}).Probe(self)
	if err != nil || state != identity.Alive {
		t.Fatalf("fixture process identity: state=%s err=%v", state, err)
	}
	announcement, err := lease.AnnounceWithPair(root, "runtime-register", self, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "wait-register", "fake", "lineage-register")
	if err != nil {
		t.Fatal(err)
	}
	mainID := announcedMainID(t, announcement)
	originalPID, originalSignature := waitCallerPID, waitOpenWorkSignature
	originalProber := waitRegisterProber
	waitCallerPID = func() int64 { return self }
	waitOpenWorkSignature = func(_ context.Context, _ string) (string, error) { return strings.Repeat("a", 64), nil }
	t.Cleanup(func() {
		waitCallerPID, waitOpenWorkSignature = originalPID, originalSignature
		waitRegisterProber = originalProber
	})
	return root, self, mainID, now
}

func registeredRows(t *testing.T, root string) []metarun.Waiter {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(metarun.WaitersDir(root), "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]metarun.Waiter, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var row metarun.Waiter
		if err := json.Unmarshal(data, &row); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row)
	}
	return rows
}

func TestWaitRegisterLocalRecordsTheTrackedProcess(t *testing.T) {
	root, _, mainID, now := waitRegisterCommandFixture(t)
	tracked := int64(8123)
	waitRegisterProber = waitRegisterFixtureProber{
		exact: identity.Exact{Pid: tracked, StartedAt: time.Unix(5000, 0), StartTicks: 77, BootID: "process-boot"},
		state: identity.Alive,
	}
	code, output, problem := captureCommandOutput(t, true, true, func() int {
		return runWait([]string{"register", "--root", root, "--pid", fmt.Sprint(tracked), "--label", "compile release", "--job", "job-a", "--json"})
	})
	var row metarun.Waiter
	if err := json.Unmarshal([]byte(output), &row); err != nil || code != 0 || problem != "" {
		t.Fatalf("registration code=%d stdout=%q stderr=%q row=%+v err=%v", code, output, problem, row, err)
	}
	if row.Kind != "local" || row.Delivery != "harness" || row.State != metarun.WaiterStatePending ||
		row.Pid != tracked || row.PidStartedAt != 5000 || row.PidStartTicks != 77 || row.BootID != "process-boot" ||
		row.RegisteredBootID != "system-boot" || row.Label != "compile release" || row.JobID != "job-a" ||
		row.MainId != mainID || row.OwnerDigest != metarun.OwnerDigest(mainID) || row.ClaimEpoch == nil ||
		row.Session != "runtime-register" || row.RuntimeSession != "runtime-register" ||
		row.Deadline != now.Add(4*time.Hour).Format(time.RFC3339Nano) {
		t.Fatalf("registered row=%+v", row)
	}

	before := len(registeredRows(t, root))
	for _, test := range []struct {
		name   string
		prober identity.Prober
		args   []string
		want   string
	}{
		{name: "dead", prober: waitRegisterFixtureProber{exact: identity.Exact{Pid: tracked}, state: identity.Dead}, want: "not alive"},
		{name: "uncertain", prober: waitRegisterFixtureProber{exact: identity.Exact{Pid: tracked}, state: identity.Unknown, err: errors.New("unreadable")}, want: "uncertain"},
		{name: "weak", prober: waitRegisterFixtureProber{exact: identity.Exact{Pid: tracked}, state: identity.Alive}, want: "not exact"},
		{name: "over maximum", prober: waitRegisterFixtureProber{exact: identity.Exact{Pid: tracked}, state: identity.Alive}, args: []string{"--timeout", "25h"}, want: "no longer than 24 hours"},
	} {
		t.Run(test.name, func(t *testing.T) {
			waitRegisterProber = test.prober
			args := []string{"register", "--root", root, "--pid", fmt.Sprint(tracked), "--label", "refused"}
			args = append(args, test.args...)
			code, _, problem := captureCommandOutput(t, true, true, func() int { return runWait(args) })
			if code == 0 || !strings.Contains(problem, test.want) || len(registeredRows(t, root)) != before {
				t.Fatalf("code=%d stderr=%q rows=%d want rows=%d", code, problem, len(registeredRows(t, root)), before)
			}
		})
	}
}

func TestWaitRegisterHumanNeedsAQuestionAndADeadline(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "question", args: []string{"register", "--human", "--timeout", "1h"}, want: "requires --question"},
		{name: "deadline", args: []string{"register", "--human", "--question", "Proceed?"}, want: "requires --timeout"},
		{name: "maximum", args: []string{"register", "--human", "--question", "Proceed?", "--timeout", "25h"}, want: "no longer than 24 hours"},
	} {
		t.Run(test.name, func(t *testing.T) {
			code, _, problem := captureCommandOutput(t, true, true, func() int { return runWait(test.args) })
			if code != metarun.ExitInvalidWait || !strings.Contains(problem, test.want) {
				t.Fatalf("code=%d stderr=%q", code, problem)
			}
		})
	}
	root, _, mainID, now := waitRegisterCommandFixture(t)
	code, output, problem := captureCommandOutput(t, true, true, func() int {
		return runWait([]string{"register", "--root", root, "--human", "--question", "Proceed with release?", "--timeout", "2h", "--json"})
	})
	var row metarun.Waiter
	if err := json.Unmarshal([]byte(output), &row); err != nil || code != 0 || problem != "" ||
		row.Kind != "human" || row.Delivery != "human" || row.Question != "Proceed with release?" ||
		row.Pid != 0 || row.MainId != mainID || row.Deadline != now.Add(2*time.Hour).Format(time.RFC3339Nano) {
		t.Fatalf("human registration code=%d stdout=%q stderr=%q row=%+v err=%v", code, output, problem, row, err)
	}
}
