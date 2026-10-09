package steward

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

func TestFleetRearmPreservesContinuation(t *testing.T) {
	t.Parallel()
	root := os.Getenv("FLEET_RECOVERY_ROOT")
	if root == "" {
		root, environment := newProcessGitFixtureEnvironment(t)
		configPath := filepath.Join(root, "git-replies.json")
		var config processGitConfig
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &config); err != nil {
			t.Fatal(err)
		}
		cwd, _ := os.Getwd()
		for _, reply := range []processGitReply{
			{Args: []string{"config", "--local", "--no-includes", "--get", landingRefConfigKey}, Stdout: []byte("refs/remotes/origin/main\n")},
			{Args: []string{"config", "--get", rearmResolveSecondsConfig}, Exit: 1},
			{Args: []string{"rev-parse", "--path-format=absolute", "--git-path", "hooks"}, Stdout: []byte(filepath.Join(root, ".git", "hooks"))},
			{Args: []string{"rev-parse", "--verify", "--quiet", "refs/remotes/origin/main^{commit}"}, Stdout: []byte(processGitHead)},
			{Args: []string{"rev-parse", "--verify", "--quiet", processGitHead + "^{commit}"}, Stdout: []byte(processGitHead)},
			{Args: []string{"rev-parse", "--verify", "HEAD^{commit}"}, Stdout: []byte(processGitHead)},
			{Args: []string{"merge-base", "--is-ancestor", processGitHead, "refs/remotes/origin/main"}},
		} {
			reply.Cwd, reply.Args = cwd, append([]string{"-C", root}, reply.Args...)
			config.Replies = append(config.Replies, reply)
		}
		data, _ = json.Marshal(config)
		if err := os.WriteFile(configPath, data, 0o600); err != nil {
			t.Fatal(err)
		}
		command := exec.Command(os.Args[0], "-test.run=^TestFleetRearmPreservesContinuation$", "-test.timeout=30m")
		for _, entry := range os.Environ() {
			name, _, _ := strings.Cut(entry, "=")
			if name != "PATH" && name != processGitConfigEnv && name != "METASYSTEM_SUPERVISION_REGISTRY_HOME" && name != "METASYSTEM_TESTENV_REGISTRY_NONCE" {
				command.Env = append(command.Env, entry)
			}
		}
		command.Env = append(command.Env, environment...)
		command.Env = append(command.Env, "FLEET_RECOVERY_ROOT="+root)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("public recovery: %v\n%s", err, output)
		}
		return
	}
	writeRecord := func(path string, value any) {
		t.Helper()
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var record map[string]any
		if err := json.Unmarshal(data, &record); err != nil {
			t.Fatal(err)
		}
		writeStewardRecord(t, path, record)
	}
	now := time.Date(2026, 10, 8, 8, 0, 0, 0, time.UTC)
	runnerNow = func() time.Time { return now }
	runnerSleep = func(d time.Duration) { now = now.Add(d); runtime.Gosched() }
	home, err := HomeStateRoot()
	if err != nil {
		t.Fatal(err)
	}
	providerHome := testprovider.Register(t, root)
	engine := filepath.Join(root, "engine")
	built := filepath.Join(t.TempDir(), "engine")
	buildFakeRunner(t, built, processGitHead)
	bytes, err := os.ReadFile(built)
	if err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(engine, bytes, 0o700); err != nil {
		t.Fatal(err)
	}
	digest, err := installDigest(engine)
	if err != nil {
		t.Fatal(err)
	}
	installed := InstallIdentity{RepoIdentity: root, Generation: 1, InstallPath: engine, InstallDigest: digest,
		MintedAt: now.Format(time.RFC3339), MintedBy: "human-terminal", HumanWitnessedGeneration: 1, HumanWitnessedAt: now.Format(time.RFC3339)}
	if err := MintIdentity(RepoIdentityPath(root), installed); err != nil {
		t.Fatal(err)
	}
	reapStewardRunnerFixture(t, root)
	stage := func(nonce string) Intent {
		t.Helper()
		it, err := StageIntent(root, nonce, "fix-it", "steward-"+nonce, "fake", "fixture", "seatIdle")
		if err != nil {
			t.Fatal(err)
		}
		if err := PrepareIntent(root, filepath.Join(root, "receipts.log"), it); err != nil {
			t.Fatal(err)
		}
		it.FenceAtMint, err = ReadEnrollmentFence(root)
		if err != nil {
			t.Fatal(err)
		}
		return it
	}
	it := stage("continuing")
	// Retained restart history must survive engine refreshes and a planned continuation.
	before := Evidence{Marks: Marks{HeadOid: processGitHead, OpidDigest: "no-ledger"}, TicksSinceAdvance: 7, DryRevivals: 1, AbnormalCount: 1}
	before.Abnormal[0] = AbnormalRestart{At: now.Add(-time.Minute), Class: "failed", Nonce: "previous", Pending: true}
	if err := SaveEvidence(root, EvidencePath(root), before); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := testexec.WriteFile(engine, append(append([]byte(nil), bytes...), byte(i)), 0o700); err != nil {
			t.Fatal(err)
		}
		outcome, err := ReArmRebuiltEngine(root, root, engine)
		if err != nil || outcome.Generation != i+2 {
			t.Fatalf("public re-arm %d: %+v %v", i, outcome, err)
		}
		after, err := LoadEvidence(EvidencePath(root))
		if err != nil || !reflect.DeepEqual(after, before) {
			t.Fatalf("re-arm erased restart history: %+v %v", after, err)
		}
	}
	handoff := writeStagedHandoffFixture(t, root, "abababababababab")
	handoffIntent, err := StageHandoffIntent(root, "abababababababab", "fix-it", "steward-witness", "fake", "fixture", handoff.binding)
	if err != nil || handoffIntent.HumanWitnessedGeneration != it.HumanWitnessedGeneration || handoffIntent.HumanWitnessedAt != it.HumanWitnessedAt {
		t.Fatalf("handoff lost enrollment witness: %+v %v", handoffIntent, err)
	}
	if err := os.Remove(handoff.liveSource); err != nil {
		t.Fatal(err)
	}
	cfg := TickConfig{Now: now, WorkStateRoot: home, ProviderHome: providerHome}
	calls := 0
	continuation := filepath.Join(t.TempDir(), "continue")
	launches := filepath.Join(root, "continuation-launches")
	if err := testexec.WriteFile(continuation, []byte("#!/bin/sh\nprintf '%s\\n' \"$2\" >> \"$1\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	run := func(it Intent) error {
		if lock, err := TryAcquireArbitration(root); !errors.Is(err, ErrArbitrationHeld) {
			if lock != nil {
				lock.Release()
			}
			t.Fatalf("launch is outside arbitration: %v", err)
		}
		authorization, err := AuthorizeDispatch(root, it.Nonce)
		if err != nil {
			return err
		}
		if output, err := exec.Command(continuation, launches, authorization.JobId).CombinedOutput(); err != nil {
			return fmt.Errorf("continuation process: %w: %s", err, output)
		}
		calls++
		return nil
	}
	outcome, err := CompleteRevival(root, cfg, deadCensus(), it.Nonce, run, nil)
	if err != nil || !outcome.Launched || calls != 1 {
		t.Fatalf("continuation after two re-arms: %+v calls=%d %v", outcome, calls, err)
	}
	again, err := CompleteRevival(root, cfg, deadCensus(), it.Nonce, run, nil)
	if err != nil || again.Launched || calls != 1 {
		t.Fatalf("continuation replay: %+v calls=%d %v", again, calls, err)
	}
	if _, err := AuthorizeDispatch(root, it.Nonce); err == nil {
		t.Fatal("stamped authorization replayed")
	}
	after, err := LoadEvidence(EvidencePath(root))
	if err != nil || after.Abnormal != before.Abnormal || after.AbnormalCount != before.AbnormalCount || after.DryRevivals != before.DryRevivals {
		t.Fatalf("planned continuation spent restart allowance: %+v %v", after, err)
	}
	// Check publication at the identity owner's final repository observation.
	if err := testexec.WriteFile(engine, append(append([]byte(nil), bytes...), 3), 0o700); err != nil {
		t.Fatal(err)
	}
	deps := rearmBed{root: root, second: processGitHead}.successDeps(t)
	readHead := deps.checkoutHead
	deps.checkoutHead = func(root string) (string, error) {
		lock, err := TryAcquireArbitration(root)
		if lock != nil {
			lock.Release()
		}
		if !errors.Is(err, ErrArbitrationHeld) {
			t.Fatalf("identity publication is outside arbitration: %v", err)
		}
		return readHead(root)
	}
	if _, err := reArmRebuiltEngineWithDeps(deps, root, root, engine); err != nil {
		t.Fatal(err)
	}
	for _, change := range []string{"human", "worker", "brief", "legacy", "crash"} {
		intent := stage(change)
		switch change {
		case "human":
			id, _ := VerifyIdentity(RepoIdentityPath(root), root)
			id.Generation++
			id.HumanWitnessedGeneration = id.Generation
			id.HumanWitnessedAt = now.Add(time.Hour).Format(time.RFC3339)
			id.MintedBy = "human-terminal"
			if err := MintIdentity(RepoIdentityPath(root), id); err != nil {
				t.Fatal(err)
			}
			id.Generation++
			id.MintedBy = "machine-rebuild"
			if err := MintIdentity(RepoIdentityPath(root), id); err != nil {
				t.Fatal(err)
			}
		case "worker":
			if err := BumpEnrollmentFence(root); err != nil {
				t.Fatal(err)
			}
		case "brief":
			if err := os.WriteFile(BriefPath(root, intent.Nonce), []byte("changed"), 0o600); err != nil {
				t.Fatal(err)
			}
		case "legacy":
			intent.HumanWitnessedGeneration, intent.HumanWitnessedAt = 0, ""
			writeRecord(filepath.Join(intentsDir(root), intent.Nonce+".json"), intent)
			id, _ := VerifyIdentity(RepoIdentityPath(root), root)
			id.Generation++
			id.MintedBy = "machine-rebuild"
			if err := MintIdentity(RepoIdentityPath(root), id); err != nil {
				t.Fatal(err)
			}
		}
		previous := calls
		if change == "worker" {
			result, err := CompleteRevival(root, cfg, deadCensus(), intent.Nonce, run, nil)
			if err != nil || result.Launched || calls != previous || !strings.Contains(result.Reason, "worker enrolled") {
				t.Fatalf("worker fence bypassed: %+v %v", result, err)
			}
			continue
		}
		if change == "crash" {
			if _, err := ConsumeIntent(root, intent.Nonce); err != nil {
				t.Fatal(err)
			}
			result, err := CompleteRevival(root, cfg, deadCensus(), intent.Nonce, run, nil)
			if err != nil || result.Launched || calls != previous {
				t.Fatalf("consumed intent relaunched: %+v %v", result, err)
			}
			after, _ := LoadEvidence(EvidencePath(root))
			if after.Abnormal != before.Abnormal || after.AbnormalCount != before.AbnormalCount {
				t.Fatal("crash lost restart history")
			}
			if err := StampLaunch(root, intent.Nonce); err != nil {
				t.Fatal(err)
			}
		} else {
			result, err := CompleteRevival(root, cfg, deadCensus(), intent.Nonce, run, nil)
			if err != nil || result.Launched || !result.Escalate || calls != previous || change == "brief" && !strings.Contains(result.Reason, "staged brief drifted") {
				t.Fatalf("changed %s authorized: %+v calls=%d %v", change, result, calls, err)
			}
			if err := StampLaunch(root, intent.Nonce); err != nil {
				t.Fatal(err)
			}
		}
	}
	launchedBytes, err := os.ReadFile(launches)
	if err != nil || string(launchedBytes) != "steward-continuing\n" {
		t.Fatalf("continuation launch effects: %q %v", launchedBytes, err)
	}
	// Read current process identity rather than trusting a running status or pid.
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatal("cannot observe fixture process")
	}
	ref := exact.Ref()
	budgetBed := &seatBed{t: t, root: root, goals: map[string]*goal.GoalFile{"fix-it": seatClaimedGoal("fix-it", SeatLineage)}}
	work, err := goal.ClaimableWorkFromProjection(budgetBed.projection(now), seatBedMachine, seatBusyDeadProber{})
	if err != nil {
		t.Fatal(err)
	}
	file, ok := work.OwnedClaim("fix-it")
	if !ok || file.Budget == nil {
		t.Fatal("held fixture has no elapsed allowance")
	}
	limit, valid := goal.ParseWorkingDuration(file.Budget.ElapsedLimit)
	if !valid || limit >= 48*time.Hour {
		t.Fatalf("fixture step is not beyond its allowance: %s valid=%t", limit, valid)
	}
	units := filepath.Join(home, "unit")
	runPath := filepath.Join(units, "run-1", "run.json")
	record := launch.UnitRunRecord{ID: "run-1", Goal: "fix-it", State: "running", Rounds: []launch.UnitRound{{Steps: []launch.UnitStep{{Name: "build", State: launch.StepStarting, StartedAt: now.Add(-48 * time.Hour).Format(time.RFC3339)}}}}}
	reservations := 0
	assertBusy := func(want bool, unknown bool) {
		t.Helper()
		busy, reason, _ := SeatBusyAt(root, units, work, SeatBusyOptions{Now: now})
		if busy != want || unknown && !strings.Contains(reason, "unknown") {
			t.Fatalf("dependency observation busy=%t reason=%q want=%t unknown=%t", busy, reason, want, unknown)
		}
		ready, err := SeatAtUnitBoundary(root, home, now, work)
		if err != nil || ready == want {
			t.Fatalf("dependency boundary: ready=%t wantBusy=%t %v", ready, want, err)
		}
		if stopReason, err := SeatBusy(root, work); err != nil || (stopReason != "") != want {
			t.Fatalf("public Stop observation: %q %v", stopReason, err)
		}
		if !want {
			return
		}
		if !unknown {
			reservations++
			reserved := stage(fmt.Sprintf("dependent-%d", reservations))
			previous := calls
			result, err := CompleteRevival(root, cfg, deadCensus(), reserved.Nonce, run, nil)
			if err != nil || result.Launched || calls != previous || !strings.Contains(result.Reason, reason) {
				t.Fatalf("revival missed its dependency: %+v %v", result, err)
			}
		}
		result, err := RunTick(root, cfg, deadCensus())
		if result.Evidence.DryRevivals != before.DryRevivals || result.Evidence.Abnormal != before.Abnormal || result.Evidence.AbnormalCount != before.AbnormalCount {
			t.Fatalf("waiting on a dependency reset restart history: %+v", result.Evidence)
		}
		if err != nil || want && !unknown && (result.Decision.Action != ActNone || result.Decision.Verdict != VerdictHealthy) || unknown && result.Decision.Action == ActRevive {
			t.Fatalf("public tick declared dependency idle: %+v %v", result.Decision, err)
		}
	}
	writeRecord(runPath, record)
	assertBusy(true, false)
	record.Rounds[0].Steps[0].State, record.Rounds[0].Steps[0].LaunchID = launch.StepRunning, "dependent"
	writeRecord(runPath, record)
	store := launch.Store{Root: filepath.Join(home, "launch")}
	if err := store.Create(launch.Record{ID: "dependent", State: launch.Running, Supervisor: &ref}); err != nil {
		t.Fatal(err)
	}
	assertBusy(true, false)
	if _, err := store.Update("dependent", func(r *launch.Record) error { r.State = launch.Completed; return nil }); err != nil {
		t.Fatal(err)
	}
	assertBusy(false, false)
	if err := os.Remove(runPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Dir(runPath)); err != nil {
		t.Fatal(err)
	}
	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	parentPath, childPath := filepath.Join(jobs, "build-root.json"), filepath.Join(jobs, "build-child.json")
	writeStewardRecord(t, parentPath, map[string]any{"goalId": "fix-it", "status": "completed"})
	job := map[string]any{"goalId": "", "parentJob": "build-root", "status": "running", "pid": ref.Pid, "pidStartedAt": ref.StartedAtSec, "pidStartedAtExactMicro": ref.StartedAtUnixMicro, "pidStartTicks": ref.StartTicks, "bootId": ref.BootID}
	writeStewardRecord(t, childPath, job)
	assertBusy(true, false)
	job["pidStartedAt"], job["pidStartedAtExactMicro"] = ref.StartedAtSec-1, ref.StartedAtUnixMicro-1000000
	if ref.StartTicks > 0 {
		job["pidStartTicks"] = ref.StartTicks + 1
	}
	writeStewardRecord(t, childPath, job)
	assertBusy(false, false)
	job["status"] = "completed"
	writeStewardRecord(t, childPath, job)
	assertBusy(false, false)
	job["goalId"], job["status"], job["pid"] = "unrelated", "running", ref.Pid
	writeStewardRecord(t, childPath, job)
	assertBusy(false, false)
	if err := os.WriteFile(childPath, []byte(`{"goalId":"fix-it","status":broken}`), 0o600); err != nil {
		t.Fatal(err)
	}
	assertBusy(true, true)
	if err := os.Remove(childPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(parentPath); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(runPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runPath, []byte(`{"goal":"fix-it","rounds":broken}`), 0o600); err != nil {
		t.Fatal(err)
	}
	assertBusy(true, true)
	fmt.Println("verified public re-arm, single continuation, fences, restart history and dependent-work tick")
}
