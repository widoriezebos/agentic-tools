package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

// Each child has a private executable lookup; unknown Git operations fail.
func injectedRunnerChild(t *testing.T) bool {
	t.Helper()
	if os.Getenv("INJECTED_RUNNER_CHILD") != "" {
		return true
	}
	tools := t.TempDir()
	if err := testexec.WriteFile(filepath.Join(tools, "git"), []byte(`#!/bin/sh
root=$(pwd -P)
if [ "$1" = -C ]; then root=$2; shift 2; fi
case "$*" in
 'rev-parse --show-toplevel') printf '%s\n' "$root" ;;
 'rev-parse --verify HEAD^{commit}') printf '%s\n' 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' ;;
 'rev-parse --show-prefix'|'ls-tree -d --name-only aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa') exit 0 ;;
 *) exit 1 ;;
esac
`), 0700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+t.Name()+"$", "-test.count=1", "-test.timeout=30m", "-test.v")
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if name != "METASYSTEM_CENSUS_PROCESS_FILE" && name != "PATH" && name != "METASYSTEM_SUPERVISION_REGISTRY_HOME" && name != "METASYSTEM_TESTENV_REGISTRY_NONCE" {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	injectedRunnerFile(t, filepath.Join(tools, "processes.json"), []byte("[]"))
	cmd.Env = append(cmd.Env, "METASYSTEM_CENSUS_PROCESS_FILE="+filepath.Join(tools, "processes.json"), "INJECTED_RUNNER_CHILD=1", "PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	t.Log(string(out))
	if err != nil {
		t.Fatalf("isolated runner: %v", err)
	}
	return false
}

type runnerProbeTable struct {
	t     *testing.T
	exact map[int64]identity.Exact
	dead  map[int64]bool
	calls map[int64]int
}

func (p *runnerProbeTable) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	p.calls[pid]++
	exact, ok := p.exact[pid]
	if pid >= 41 && pid <= 43 {
		exact, ok = identity.Exact{Pid: pid, StartedAt: time.Unix(100, 0), ArgvKnown: true, Argv: []string{"supervision"}}, true
	}
	if !ok {
		p.t.Fatalf("unexpected process probe %d", pid)
	}
	if p.dead[pid] {
		return identity.Exact{}, identity.Dead, nil
	}
	return exact, identity.Alive, nil
}

func injectedRunnerWrite(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	injectedRunnerFile(t, path, data)
}
func injectedRunnerFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func injectedHandoff(t *testing.T, root, goalID string, predecessor identity.Exact) steward.Intent {
	t.Helper()
	now := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	home, err := board.Home()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(home, "metasystem-test-registry-") {
		t.Fatalf("provider fixture escaped the isolated child: %s", home)
	}
	if _, _, err := lane.Register(home, lane.Layout{Checkout: lane.CheckoutRoot(root), Install: lane.InstallRoot(root)}, "fixture", now); err != nil {
		t.Fatal(err)
	}
	injectedRunnerFile(t, filepath.Join(root, "bin", "metasystem"), []byte("fixture engine"))
	injectedRunnerWrite(t, filepath.Join(root, "artifacts", "agents", "supervision", "state.json"), map[string]any{"generation": 1, "owner": map[string]any{"pid": 41, "pidStartedAt": 100, "instanceTag": "supervision"}, "components": map[string]any{"watcher": map[string]any{"pid": 42, "pidStartedAt": 100, "instanceTag": "supervision"}, "reaper": map[string]any{"pid": 43, "pidStartedAt": 100, "instanceTag": "supervision"}}})
	const nonce, session, main = "1234567890abcdef", "injected-session", "main-1-100-abcdef"
	state := steward.HandoffState{SchemaVersion: 1, WrittenAt: now,
		Seat:     steward.HandoffSeat{Machine: "fixture", Runtime: "fake", Session: session, NormalizedSession: session, MainID: main, Identity: predecessor.Ref()},
		HeldGoal: steward.HandoffHeldGoal{ID: goalID, State: "claimed", AcceptedRevision: 1, Claimant: steward.HandoffClaimant{Machine: "fixture", Lineage: steward.SeatLineage, At: now.Format(time.RFC3339), Revision: 1}},
		NextStep: steward.HandoffNextStep{Text: "Continue the unit."}, Disposable: steward.HandoffDisposable}
	path := filepath.Join(steward.HandoffDir(root, nonce), "state.json")
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	injectedRunnerFile(t, path, data)
	sum := sha256.Sum256(data)
	binding := &steward.HandoffBinding{StatePath: path, StateDigest: hex.EncodeToString(sum[:]), Runtime: "fake", Session: session, MainId: main, Predecessor: predecessor.Ref(), RecordedAt: now}
	if err := steward.MintIdentity(steward.RepoIdentityPath(root), steward.InstallIdentity{RepoIdentity: root, Generation: 1, InstallPath: "/bin/true", MintedAt: now.Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	it := steward.Intent{Nonce: nonce, RepoIdentity: root, InstallGen: 1, Goal: goalID, JobId: "steward-" + nonce, Runtime: "fake", Reason: "seatHandoff", Handoff: binding}
	if err := steward.MintIntent(root, it); err != nil {
		t.Fatal(err)
	}
	injectedRunnerFile(t, steward.BriefPath(root, nonce), []byte("# Continue\n"))
	return it
}

func TestRevivalUsesTheInjectedProber(t *testing.T) {
	t.Parallel()
	if !injectedRunnerChild(t) {
		return
	}
	root := t.TempDir()
	testprovider.Register(t, root)
	injectedRunnerFile(t, filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\nlaunch.build.model.fake=fixture\n"))
	injectedRunnerFile(t, filepath.Join(root, "plans", "goals.md"), []byte("# Goals\n\n## Current goal: held — Finish the unit\n- Origin: main\n- Next step: Continue.\n"))
	predecessor := identity.Exact{Pid: int64(os.Getpid()), StartedAt: time.Unix(100, 0)}
	it := injectedHandoff(t, root, "held", predecessor)
	injectedRunnerWrite(t, filepath.Join(root, "artifacts", "agents", "mains", "main-99-100-abcdef.json"), lease.Announcement{SessionId: "stale", MainId: "main-99-100-abcdef", Pid: 99, PidStartedAt: 100, Runtime: "fake", InstanceTag: "fixture", CommandHash: strings.Repeat("a", 64), AnnouncedAt: "2026-09-01T09:00:00Z"})
	table := &runnerProbeTable{t: t, exact: map[int64]identity.Exact{predecessor.Pid: predecessor, 99: {Pid: 99, StartedAt: time.Unix(100, 0)}}, dead: map[int64]bool{99: true}, calls: map[int64]int{}}
	now := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	start := now
	held := false
	sleeps, launches, signals := 0, 0, 0
	clock := &steward.HandoffClock{Now: func() time.Time { return now }, Sleep: func(d time.Duration) {
		now = now.Add(d)
		sleeps++
		var record steward.RunnerRecord
		data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "steward", "runner.json"))
		if err != nil || json.Unmarshal(data, &record) != nil || record.PidStartedAt != predecessor.StartedAt.Unix() {
			t.Fatalf("runner self did not use the table: %+v %v", record, err)
		}
		if !held {
			if table.calls[99] < 2 {
				t.Fatalf("the tick and revival goal reads bypassed the table: %d probes, want at least 2", table.calls[99])
			}
			held = true
			live, err := steward.LiveIntents(root)
			if err != nil || len(live) != 1 || live[0].Nonce != it.Nonce || launches != 0 || signals != 0 {
				t.Fatalf("live predecessor was not held: live=%+v launches=%d signals=%d error=%v", live, launches, signals, err)
			}
			pending, _ := filepath.Glob(filepath.Join(root, "artifacts", "agents", "steward", "pending", "*.json"))
			if len(pending) != 0 {
				t.Fatalf("hold notice was not delivered through the production transport: %v", pending)
			}
			episodes, err := steward.AlertEpisodes(root)
			if err != nil || len(episodes) != 1 {
				t.Fatalf("production hold transport retained %d health episodes: %v", len(episodes), err)
			}
			// The resident runner remains alive; only its captured predecessor ended.
			table.exact[predecessor.Pid] = identity.Exact{Pid: predecessor.Pid, StartedAt: time.Unix(200, 0)}
		}
		if !now.Before(start.Add(2 * time.Second)) {
			injectedRunnerFile(t, filepath.Join(root, "artifacts", "agents", "steward", "stop"), nil)
		}
	}}
	var problem bytes.Buffer
	code := runStewardRunWithDependencies([]string{"--repo", root}, io.Discard, &problem,
		func(root string, _ steward.WorkerCensus, revive func() error, interval time.Duration, cfg steward.TickConfig) error {
			cfg.Now, cfg.WorkStateRoot = now, t.TempDir()
			cfg.CompletedBoundary = nil
			return steward.RunLoop(root, fleetHandoffCensus{}, revive, interval, cfg)
		}, nil, clock, func(string) int { return 1 }, func() (bool, error) { return false, nil }, stewardRunDependencies{Prober: table, Signal: func(int, syscall.Signal) error { signals++; return nil }, ReviveLaunch: func(got steward.Intent) error {
			launches++
			if got.Nonce != it.Nonce {
				t.Fatalf("wrong launch: %+v", got)
			}
			return nil
		}})
	if code != 0 || launches != 1 || signals != 0 || !held {
		w, _ := stewardCensusFor(root, table).Workers(root)
		t.Logf("census=%+v calls=%v", w, table.calls)
		t.Fatalf("runner revival: exit=%d launches=%d signals=%d sleeps=%d stderr=%s", code, launches, signals, sleeps, &problem)
	}
}

func TestBoundaryPassUsesTheInjectedProber(t *testing.T) {
	t.Parallel()
	if !injectedRunnerChild(t) {
		return
	}
	bed := newWorkBed(t)
	testprovider.Register(t, bed.root())
	now := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	predecessor := identity.Exact{Pid: 987654321, StartedAt: now.Add(-time.Hour)}
	self := identity.Exact{Pid: int64(os.Getpid()), StartedAt: time.Unix(100, 0)}
	it := injectedHandoff(t, bed.root(), bed.id, predecessor)
	mains := filepath.Join(bed.root(), "artifacts", "agents", "mains")
	injectedRunnerWrite(t, filepath.Join(mains, "worktree-lease.json"), lease.Lease{HolderMainId: it.Handoff.MainId, OwnerLineage: steward.SeatLineage, Pid: predecessor.Pid, PidStartedAt: predecessor.StartedAt.Unix(), Revision: 1, ClaimEpoch: 1})
	injectedRunnerWrite(t, filepath.Join(mains, "main-1-100-abcdef.json"), lease.Announcement{SessionId: it.Handoff.Session, MainId: it.Handoff.MainId, Pid: predecessor.Pid, PidStartedAt: predecessor.StartedAt.Unix(), Runtime: "fake", CommandHash: strings.Repeat("a", 64), InstanceTag: "fixture", OwnerLineage: steward.SeatLineage, AnnouncedAt: now.Format(time.RFC3339)})

	brief := bed.brief("boundary.md", "Build the unit.\n")
	bed.declaredCheap = "go test -timeout 30m -count=1 -run 'TestA|TestB' ./..."
	code, result, _ := bed.work("work", "build", bed.id, "finished", "--brief", brief, "--lines", "5", "--read-tool-calls", "12")
	if code != 0 {
		t.Fatalf("public build: %d %+v", code, result)
	}
	runID := resultData(t, result)["run"].(string)
	runner := &launch.UnitRunner{Manager: bed.manager, Git: workGit{bed}, Root: bed.unitRoot}
	if err := runner.ReviewSubject(runID, func(review launch.UnitReview, retain func(launch.UnitSubject) error) error {
		return retain(launch.UnitSubject{Round: review.Round.Number, Commit: strings.Repeat("a", 40), ResultDigest: launch.UnitResultDigest(review.Result), DiffDigest: review.DiffDigest})
	}); err != nil {
		t.Fatal(err)
	}
	owners := bed.workOwners()
	owners.work.inspectRead = func(string, string, string) (branch.BranchReadResult, error) {
		return branch.BranchReadResult{State: "collected", Published: true, AttestationCommit: strings.Repeat("b", 40)}, nil
	}

	table := &runnerProbeTable{t: t, exact: map[int64]identity.Exact{predecessor.Pid: predecessor, self.Pid: self}, dead: map[int64]bool{}, calls: map[int64]int{}}
	signals, launches := 0, 0
	clock := &steward.HandoffClock{Now: func() time.Time { return now }, Sleep: func(d time.Duration) {
		now = now.Add(d)
		injectedRunnerFile(t, filepath.Join(bed.root(), "artifacts", "agents", "steward", "stop"), nil)
	}}
	var problem bytes.Buffer
	code = runStewardRunWithDependencies([]string{"--repo", bed.root()}, io.Discard, &problem,
		func(root string, _ steward.WorkerCensus, revive func() error, interval time.Duration, cfg steward.TickConfig) error {
			cfg.Now, cfg.WorkStateRoot = now, filepath.Dir(bed.unitRoot)
			wireStewardSeat(&cfg, owners)
			observe := cfg.CompletedBoundary
			cfg.CompletedBoundary = func(root string, at time.Time) error {
				before := table.calls[predecessor.Pid]
				err := observe(root, at)
				if got := table.calls[predecessor.Pid] - before; got != 3 {
					t.Fatalf("boundary did not inject the projection, predecessor and signal checks: %d observations, want 3", got)
				}
				return err
			}
			return steward.RunLoop(root, fleetHandoffCensus{}, revive, interval, cfg)
		}, nil, clock, func(string) int { return 1 }, func() (bool, error) { return false, nil }, stewardRunDependencies{Prober: table, Signal: func(pid int, sig syscall.Signal) error {
			signals++
			events, err := steward.ReadUnitBoundaries(bed.root())
			if err != nil || len(events) != 1 || events[0].Handoff != it.Nonce || pid != int(predecessor.Pid) || sig != syscall.SIGTERM {
				t.Fatalf("signal preceded durable binding: %+v %v pid=%d signal=%v", events, err, pid, sig)
			}
			return nil
		}, ReviveLaunch: func(steward.Intent) error { launches++; return nil }})
	events, err := steward.ReadUnitBoundaries(bed.root())
	if code != 0 || err != nil || signals != 1 || launches != 0 || len(events) != 1 || !events[0].Signalled || events[0].Unit != runID+"/1" {
		t.Fatalf("boundary pass: exit=%d signals=%d launches=%d events=%+v error=%v stderr=%s", code, signals, launches, events, err, &problem)
	}
	episodes, err := steward.AlertEpisodes(bed.root())
	if err != nil {
		t.Fatal(err)
	}
	pending, err := steward.PendingNotifications(bed.root())
	if err != nil || len(pending) != 0 || len(episodes) != 1 {
		t.Fatalf("production transport: pending=%d episodes=%d error=%v", len(pending), len(episodes), err)
	}
}
