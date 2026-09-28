package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// The Release 1 bed: one temporary seat (a fake .git, no Git) with an
// enrolled person, a fixture-enrolled steward and the supervision hook stub.
// The helm verbs run through the public router, the Stop worker and the
// steward run in process, and a notification that reached the operator would
// land in the fixture notification log, never on the desktop.

type releaseCensus struct{}

func (releaseCensus) Workers(string) (steward.Workers, error) {
	return steward.Workers{Live: 1, CensusComplete: true}, nil
}

func newReleaseBed(t *testing.T) *helmBed {
	t.Helper()
	bed := newHelmBed(t, 20, true)
	top, err := filepath.EvalSymlinks(bed.inst)
	helmMust(t, err)
	hook := filepath.Join(bed.inst, "scripts", "agents", "supervision-hook.sh")
	helmMust(t, os.MkdirAll(filepath.Join(bed.inst, "plans"), 0o755),
		os.WriteFile(filepath.Join(bed.inst, "plans", "goals.md"), []byte("# Goals\n\n## Current goal: fix-it — Repair the thing\n- Origin: main\n- Next step: Repair it.\n"), 0o644),
		testexec.WriteFile(hook, []byte("#!/usr/bin/env bash\n"), 0o755),
		os.MkdirAll(filepath.Dir(steward.RepoIdentityPath(top)), 0o755))
	helmMust(t, steward.MintIdentity(steward.RepoIdentityPath(top), steward.InstallIdentity{RepoIdentity: top, Generation: 1,
		InstallPath: "/bin/true", MintedAt: "2026-09-28T09:00:00Z", Enrollment: steward.EnrollmentFixture}))
	bed.inst = top
	return bed
}

// stopWorker runs the Stop hook's worker path in this process.
func (b *helmBed) stopWorker() (int, string) {
	var out, errs bytes.Buffer
	env := map[string]string{"METASYSTEM_STOP_DEADLINE_PARENT": "777"}
	status := hooks.RunRuntimeHook(hooks.Invocation{
		Runtime: "claude", Event: "stop", Stdin: strings.NewReader(`{"session_id":"s-1"}`), Stdout: &out, Stderr: &errs,
		Lookup: func(name string) (string, bool) { value, ok := env[name]; return value, ok },
		Pid:    os.Getpid(), Ppid: 777, Script: filepath.Join(b.inst, "scripts", "agents", "supervision-hook.sh"),
		Now: func() time.Time { return helmNow }, Sleep: func(time.Duration) {}, Environ: os.Environ, TempDir: b.t.TempDir(),
	}, hookOwners{diagnostics: &errs})
	return status, out.String()
}

func tickAttempts(t *testing.T, root string) float64 {
	data, err := os.ReadFile(steward.ComponentEvidencePath(root, "steward-tick"))
	var record map[string]any
	if err != nil || json.Unmarshal(data, &record) != nil {
		return 0
	}
	seq, _ := record["attemptSeq"].(float64)
	return seq
}

func TestReleaseOneWithRunningSteward(t *testing.T) {
	bed := newReleaseBed(t)
	root := bed.inst
	bed.wantTake(nil, 0, "proven", "Wido")

	if status, out := bed.stopWorker(); status != 0 || !strings.Contains(out, "HUMAN AT THE HELM") || !strings.Contains(out, "Stop allowed.") {
		t.Fatalf("HM-6: the Stop worker under the helm answered %d %q", status, out)
	}

	tick, err := steward.RunTick(root, steward.TickConfig{Now: helmNow}, releaseCensus{})
	if err != nil || tick.Decision.Verdict != steward.VerdictHelm || tick.Decision.Action != steward.ActNone {
		t.Fatalf("HM-7: the tick under the helm = %+v %v", tick.Decision, err)
	}

	// A pending notice and a broken evidence store: one loop iteration of
	// the running steward delivers nothing and revives nothing.
	helmMust(t, steward.QueueNotification(root, steward.PendingNotification{Nonce: "held-notice", Message: "steward: held"}),
		os.WriteFile(steward.EvidencePath(root), []byte("{torn"), 0o644))
	before := tickAttempts(t, root)
	var revived atomic.Int32
	done := make(chan error, 1)
	go func() {
		done <- steward.RunLoop(root, releaseCensus{}, func() error { revived.Add(1); return nil }, time.Hour, steward.TickConfig{Now: helmNow})
	}()
	// Wait for the loop's first tick by its own record, on no clock: the
	// outcome asserted below does not depend on how long the tick took.
	for tickAttempts(t, root) <= before {
		select {
		case err := <-done:
			t.Fatalf("the runner ended before its first tick: %v", err)
		default:
			runtime.Gosched()
		}
	}
	helmMust(t, os.WriteFile(filepath.Join(root, "artifacts", "agents", "steward", "stop"), []byte("stop\n"), 0o644))
	helmMust(t, <-done)
	if _, err := os.Stat(filepath.Join(root, "artifacts", "agents", "steward", "notifications.log")); !os.IsNotExist(err) || revived.Load() != 0 {
		t.Fatalf("HM-7: the running steward delivered (%v) or revived (%d) under the helm", err, revived.Load())
	}
	if pending, err := steward.PendingNotifications(root); err != nil || len(pending) != 1 {
		t.Fatalf("HM-7: the held notice is no longer pending: %+v %v", pending, err)
	}

	if code, out := bed.run("helm", "return"); code != 0 {
		t.Fatalf("helm return: %d\n%s", code, out)
	}
	ordinary, err := steward.RunTick(root, steward.TickConfig{Now: helmNow.Add(time.Minute)}, releaseCensus{})
	if err != nil || ordinary.Decision.Verdict == steward.VerdictHelm {
		t.Fatalf("after return the tick must decide again: %+v %v", ordinary.Decision, err)
	}
	if _, err := os.Stat(steward.ComponentEvidencePath(root, "ledger-attention")); err != nil {
		t.Fatalf("after return the tick must run its ordinary passes: %v", err)
	}
}

func TestWatcherAndReaperRunUnderHelm(t *testing.T) {
	bed := newReleaseBed(t)
	root := bed.inst
	bed.wantTake(nil, 0, "proven", "Wido")
	now := helmNow

	supervision := filepath.Join(root, "artifacts", "agents", "supervision")
	watcher := supervise.WatcherConfig{SupervisionDir: supervision, Interval: 60, IntervalMS: 60000, BudgetPercent: 100,
		Fingerprint: func() (string, error) { return "fp-helm", nil },
		Census:      func(fp string, at time.Time) (census.Verdict, error) { return census.Verdict{Fingerprint: fp}, nil },
		Now:         func() time.Time { return now }}
	if err := watcher.WatcherPass(); err != nil {
		t.Fatalf("HM-11: the watcher pass under the helm: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(supervision, "last-census.json")); err != nil || !strings.Contains(string(data), "fp-helm") {
		t.Fatalf("HM-11: the watcher published no verdict under the helm: %v %s", err, data)
	}

	jobs := supervise.JobsDir(root)
	helmMust(t, os.MkdirAll(jobs, 0o755), os.WriteFile(filepath.Join(jobs, "lost.json"), []byte(`{"jobId":"lost","status":"running","pid":333,"pidStartedAt":7,"instanceTag":"job-lost","startedAt":"`+now.Add(-time.Minute).Format(time.RFC3339)+`","capMin":60}`+"\n"), 0o644))
	var applied []string
	reaper := supervise.ReaperConfig{Repo: root, JobsDir: jobs, Now: func() time.Time { return now },
		Custodian: func(int64, int64, string) identity.Liveness { return identity.Dead },
		Survivors: func(string, int64, int64) (bool, bool) { return false, true },
		Apply: func(job, expect, target string, _ map[string]any) (bool, error) {
			applied = append(applied, job+":"+expect+"->"+target)
			return true, nil
		}, Emit: func(string) {}}
	if err := reaper.ReaperPass(); err != nil || len(applied) != 1 {
		t.Fatalf("HM-11: the reaper did not reap a lost job under the helm: %v %v", applied, err)
	}
}
