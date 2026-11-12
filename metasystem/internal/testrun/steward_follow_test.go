package testrun

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func followTestSeams(t *testing.T, acts *[]string) DeployedFollow {
	t.Helper()
	step := func(name string) error { *acts = append(*acts, name); return nil }
	return DeployedFollow{
		Now: func() time.Time { return time.Date(2026, 11, 11, 11, 0, 0, 0, time.UTC) },
		Deployed: func() (steward.DeployedEngine, error) {
			return steward.DeployedEngine{Commit: "deployed", Artifact: "/fixture/engine", Digest: "new"}, nil
		},
		EnrolledDigest: func() (string, error) { return "old", nil },
		Landed:         func(string) error { return step("landed") },
		Ready:          func() (bool, string) { _ = step("ready"); return true, "" },
		Lock:           func() (func(), error) { _ = step("lock"); return func() { _ = step("unlock") }, nil },
		Forward:        func(string) error { return step("forward") },
		Install:        func(steward.DeployedEngine) error { return step("install") },
		Start:          func() error { return step("start") },
	}
}

func TestFollowDeployedEngineAtBoundary(t *testing.T) {
	t.Parallel()
	var acts []string
	f := followTestSeams(t, &acts)
	if replaced, refusal, err := FollowDeployedEngineAtBoundary(t.TempDir(), f); err != nil || !replaced || refusal != nil {
		t.Fatalf("follow: replaced=%v refusal=%+v err=%v", replaced, refusal, err)
	}
	want := []string{"landed", "ready", "lock", "ready", "forward", "install", "start", "unlock"}
	if !reflect.DeepEqual(acts, want) {
		t.Fatalf("follow calls=%v want=%v", acts, want)
	}
}

func TestFollowDeployedEngineRechecksReadyUnderLock(t *testing.T) {
	t.Parallel()
	var acts []string
	f := followTestSeams(t, &acts)
	locked := false
	f.Ready = func() (bool, string) { return !locked, "a job arrived under the lock" }
	f.Lock = func() (func(), error) { locked = true; return func() { locked = false }, nil }
	if replaced, refusal, err := FollowDeployedEngineAtBoundary(t.TempDir(), f); err != nil || replaced || refusal == nil || refusal.Waiting {
		t.Fatalf("follow=%v refusal=%+v err=%v", replaced, refusal, err)
	}
	if locked || !reflect.DeepEqual(acts, []string{"landed"}) {
		t.Fatalf("mutated after readiness changed: locked=%v acts=%v", locked, acts)
	}
	f.Ready = func() (bool, string) { return !locked, followWaitingForBoundary }
	if _, refusal, err := FollowDeployedEngineAtBoundary(t.TempDir(), f); err != nil || refusal == nil || !refusal.Waiting {
		t.Fatalf("waiting under lock: refusal=%+v err=%v", refusal, err)
	}
}

func TestFollowDeployedEngineEqualDigestStopsBeforeGuards(t *testing.T) {
	t.Parallel()
	var acts []string
	f := followTestSeams(t, &acts)
	f.EnrolledDigest = func() (string, error) { return "new", nil }
	if replaced, refusal, err := FollowDeployedEngineAtBoundary(t.TempDir(), f); err != nil || replaced || refusal != nil || len(acts) != 0 {
		t.Fatalf("equal: replaced=%v err=%v acts=%v", replaced, err, acts)
	}
}

func TestFollowDeployedEngineFailuresLogOnce(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"deployed", "no active line", "unreadable record", "no contract", "commit missing", "artifact missing", "enrolled", "landed", "lock", "forward", "install", "start"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			var acts []string
			f := followTestSeams(t, &acts)
			fail := func() error { return errors.New("fixture refusal") }
			switch stage {
			case "deployed", "no active line", "unreadable record", "no contract":
				f.Deployed = func() (steward.DeployedEngine, error) { return steward.DeployedEngine{}, fail() }
				if stage != "deployed" {
					fail = func() error { return errors.New("fixture refusal: " + stage) }
				}
			case "commit missing":
				f.Deployed = func() (steward.DeployedEngine, error) {
					return steward.DeployedEngine{Artifact: "/fixture/engine"}, fail()
				}
			case "artifact missing":
				f.Deployed = func() (steward.DeployedEngine, error) { return steward.DeployedEngine{Commit: "deployed"}, fail() }
			case "enrolled":
				f.EnrolledDigest = func() (string, error) { return "", fail() }
			case "landed":
				f.Landed = func(string) error { return fail() }
			case "lock":
				f.Lock = func() (func(), error) { return nil, fail() }
			case "forward":
				f.Forward = func(string) error { return fail() }
			case "install":
				f.Install = func(steward.DeployedEngine) error { return fail() }
			case "start":
				f.Start = fail
			}
			initialNow := f.Now()
			for pass := 0; pass < 10; pass++ {
				now := initialNow.Add(time.Duration(pass) * time.Minute)
				f.Now = func() time.Time { return now }
				replaced, refusal, err := FollowDeployedEngineAtBoundary(root, f)
				if replaced || err != nil || refusal == nil || refusal.Waiting || refusal.Reason != fail().Error() {
					t.Fatalf("replaced=%v refusal=%+v err=%v", replaced, refusal, err)
				}
				wantCommit, wantCommand := "deployed", "cd "+root+" && cp -p /fixture/engine bin/.metasystem.hand && mv bin/.metasystem.hand bin/metasystem && bin/metasystem system restart"
				if stage == "deployed" || strings.Contains(stage, " ") {
					wantCommit, wantCommand = steward.FollowDeploymentUnavailable, "cd "+root+" && bin/metasystem deploy status"
				}
				if refusal.Commit != wantCommit || refusal.Command != wantCommand {
					t.Fatalf("refusal=%+v", refusal)
				}
			}
			log := followTestLog(t, root)
			if strings.Count(log, "engine follow:") != 1 || !strings.Contains(log, "fixture refusal") {
				t.Fatalf("log=%s", log)
			}
			if stage == "deployed" && !strings.Contains(log, "cd "+root+" && bin/metasystem deploy status") {
				t.Fatalf("unavailable hand route=%s", log)
			}
			for _, act := range acts {
				if (stage == "forward" && (act == "install" || act == "start")) || (stage == "install" && act == "start") {
					t.Fatalf("continued after %s: %v", stage, acts)
				}
			}
		})
	}
}

func followTestLog(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "steward", "runner.log"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func followTestJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, path, data, 0o600)
}

type followTestProber struct{ live identity.Exact }

func (p followTestProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if pid == p.live.Pid {
		return p.live, identity.Alive, nil
	}
	return identity.Exact{}, identity.Dead, nil
}

func TestDeployedFollowReadyHoldsEveryLiveRecord(t *testing.T) {
	t.Parallel()
	for _, row := range []string{"foreign-pending-job", "dead-running-job", "unreadable-job", "unfinished-proof", "held-unit", "live-lane"} {
		t.Run(row, func(t *testing.T) {
			t.Parallel()
			root, home := t.TempDir(), t.TempDir()
			clock := newScheduledRearmClock()
			now := clock.Now()
			prober := followTestProber{live: identity.Exact{Pid: 41, StartedAt: now, StartTicks: 12, BootID: "fixture-boot"}}
			if runtime.GOOS == "darwin" {
				prober.live.StartTicks, prober.live.BootID = 0, ""
			}
			boundary := func() (bool, error) {
				if row != "held-unit" {
					return true, nil
				}
				return steward.SeatAtUnitBoundary(root, home, clock.Now(), prober, goal.ClaimableBudgetedWork{Claimed: []string{"held"}})
			}
			switch row {
			case "foreign-pending-job", "dead-running-job":
				status := "pending"
				if row == "dead-running-job" {
					status = "running"
				}
				followTestJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", "foreign.json"), map[string]any{"goalId": "not-held", "status": status, "pid": 999})
			case "unreadable-job":
				writeTestingFixtureFile(t, filepath.Join(root, "artifacts", "agents", "jobs", "broken.json"), []byte("{"), 0o600)
			case "unfinished-proof":
				hash := strings.Repeat("a", 64)
				proof := proofrun.BuildProofIdentityForContext(proofrun.ExecutionContext{ManifestDigest: hash, Configuration: hash, Platform: "fixture", Toolchain: hash}, "fixture", "proof", nil, 1)
				attempt := proofrun.Attempt{SchemaVersion: proofrun.LegacyAttemptSchemaVersion, AttemptID: "open", GoalID: "held", GoalRevision: 1, AccountingRevision: 1, ReservedMinutes: 1, StartedAt: now.Format(time.RFC3339), Deadline: now.Add(time.Minute).Format(time.RFC3339), ControlRoot: root, ExecutionRoot: root, ProofIdentity: proof, Launcher: proofrun.ProcessIdentity{Pid: 999, PidStartedAt: 100}}
				path, err := proofrun.AttemptPath(root, "open")
				if err != nil {
					t.Fatal(err)
				}
				followTestJSON(t, path, attempt)
				if _, err := proofrun.ReadAttempts(root); err != nil {
					t.Fatal(err)
				}
			case "held-unit":
				followTestJSON(t, filepath.Join(home, "unit", "run-1", "run.json"), map[string]any{"id": "run-1", "goal": "held", "state": "running", "rounds": []launch.UnitRound{{Steps: []launch.UnitStep{{Name: "build", State: launch.StepRunning, LaunchID: "launch-1", StartedAt: now.Format(time.RFC3339)}}}}})
				ref := prober.live.Ref()
				if err := (launch.Store{Root: filepath.Join(home, "launch")}).Create(launch.Record{ID: "launch-1", State: launch.Running, Supervisor: &ref}); err != nil {
					t.Fatal(err)
				}
			case "live-lane":
				ref, err := identity.EncodeRef(prober.live.Ref())
				if err != nil {
					t.Fatal(err)
				}
				followTestJSON(t, filepath.Join(plain.Dir(root), "running.json"), plain.Running{Attempt: "live", Pid: 41, Process: ref})
			}
			git := func(context.Context, string, ...string) ([]byte, error) { return []byte("20"), nil }
			installs, starts := 0, 0
			f := DeployedFollowDependencies(context.Background(), root, clock.Now, boundary, func() (steward.DeployedEngine, error) {
				return steward.DeployedEngine{Commit: "deployed", Artifact: "/fixture/artifact", Digest: "new"}, nil
			}, func(steward.DeployedEngine) error { installs++; return nil }, steward.RearmOperations{Clock: clock, Git: git, Prober: prober})
			f.EnrolledDigest = func() (string, error) { return "old", nil }
			f.Landed = func(string) error { return nil }
			f.Lock = func() (func(), error) { return func() {}, nil }
			f.Forward = func(string) error { return nil }
			f.Start = func() error { starts++; return nil }
			for range 10 {
				if replaced, refusal, err := FollowDeployedEngineAtBoundary(root, f); replaced || err != nil || refusal == nil || refusal.Waiting != (row == "held-unit") || refusal.Commit != "deployed" {
					t.Fatalf("busy follow=%v refusal=%+v err=%v", replaced, refusal, err)
				}
				clock.advance(time.Minute)
			}
			log := followTestLog(t, root)
			if installs != 0 || starts != 0 || strings.Count(log, "engine follow:") != 1 {
				t.Fatalf("installs=%d starts=%d log=%s", installs, starts, log)
			}
			if strings.Contains(log, "<nil>") && row == "held-unit" {
				t.Fatalf("boundary reason=%s", log)
			}
			if row == "held-unit" && !strings.Contains(log, "the seat is not at its unit boundary;") {
				t.Fatalf("boundary reason=%s", log)
			}
			if row == "live-lane" && !strings.Contains(log, "the lane proof is running;") {
				t.Fatalf("lane reason=%s", log)
			}
		})
	}
}
