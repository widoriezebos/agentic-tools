package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginepin"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot/stateroottest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/up"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/up/testdata/startfixture"
)

func generationArmEffects(t *testing.T, home string, executions func(string) map[string]enginepin.Execution) processArmEffects {
	t.Helper()
	return processArmEffects{
		seed:   func(string) (stewardLandingRefSeed, error) { return stewardLandingRefSeed{}, nil },
		arm:    func(string, string, processArmAuthority) (string, error) { return "", nil },
		runner: func(string) (int64, bool) { return 44, true },
		up: func(options up.Options) up.Result {
			options.Executions = executions
			options.LaneHome = func() (string, error) { return home, nil }
			options.ProofAlive = func(plain.Running) bool { return true }
			options.RepairStewardRunner = func(string) (steward.RunnerRepairOutcome, error) {
				return steward.RunnerRepairOutcome{Status: "CURRENT", Generation: 148}, nil
			}
			return up.Run(options)
		},
	}
}

func generationVerified(supervise.EnsureOptions) (supervise.EnsureResult, error) {
	return supervise.EnsureResult{Action: "verified"}, nil
}

func assertStartupGeneration(t *testing.T, root, target, verdict string, observed map[string]enginepin.Execution, count int) {
	t.Helper()
	record, found, err := state.Read(plain.Dir(root))
	if err != nil || !found {
		t.Fatalf("generation record missing: %t %v", found, err)
	}
	events, skipped, err := state.ReadEvents(plain.Dir(root))
	if err != nil || skipped != 0 {
		t.Fatalf("generation events: %v skipped=%d", err, skipped)
	}
	var generations []state.Event
	for _, event := range events {
		if event.Kind == "generation" {
			generations = append(generations, event)
		}
	}
	if len(generations) != count {
		t.Fatalf("generation event count=%d want=%d: %+v", len(generations), count, events)
	}
	want := &state.Generation{Target: target, Verdict: verdict}
	for _, role := range []string{"owner", "watcher", "reaper", "runner"} {
		e := observed[role]
		want.Roles = append(want.Roles, state.GenerationRole{Role: role, Digest: e.Digest, Generation: e.Generation, Source: e.Source, Cause: e.Cause})
	}
	if !reflect.DeepEqual(record.Generation, want) || !reflect.DeepEqual(generations[count-1].Generation, want) || generations[count-1].To != verdict {
		t.Fatalf("generation provenance: record=%+v event=%+v want=%+v", record.Generation, generations[count-1], want)
	}
}

func TestLandingStartupHalfFailsOnAStaleProcessAfterSuccess(t *testing.T) {
	t.Parallel()
	root := realpath.Resolve(t.TempDir())
	home := t.TempDir()
	binary, digest := startfixture.Stage(t, root)
	startfixture.Register(t, home, root)
	read := startfixture.Executions(t, root, binary, map[string]string{"reaper": "obsolete"})
	effects := generationArmEffects(t, home, read)
	scope := processScope{Checkout: root, Binary: binary, Installation: stateroottest.Installation(t, root), Root: stateroottest.State(t, root)}
	_, err := restartLandingSupervisor(scope, effects, generationVerified)
	if err == nil || !strings.Contains(err.Error(), "reaper") || !strings.Contains(err.Error(), read(root)["reaper"].Digest) {
		t.Fatalf("stale startup: %v", err)
	}
	assertStartupGeneration(t, root, digest, "mismatch", read(root), 1)
	current := startfixture.Executions(t, root, binary, nil)
	effects = generationArmEffects(t, home, current)
	first, err := restartLandingSupervisor(scope, effects, generationVerified)
	if err != nil {
		t.Fatalf("current startup: %+v %v", first, err)
	}
	assertStartupGeneration(t, root, digest, "one generation", current(root), 2)
	second, err := restartLandingSupervisor(scope, effects, generationVerified)
	if err != nil || !second.unchanged {
		t.Fatalf("current repeat: %+v %v", second, err)
	}
	assertStartupGeneration(t, root, digest, "one generation", current(root), 3)
}

func TestSystemRestartFailsOnAStaleOrUnverifiableProcess(t *testing.T) {
	t.Parallel()
	for _, fault := range []string{"obsolete", "unknown"} {
		t.Run(fault, func(t *testing.T) {
			t.Parallel()
			b := newProcessBed(t)
			root := b.root()
			binary, digest := startfixture.Stage(t, root)
			// Public process scope resolves bin/metasystem; enroll those same bytes there.
			enrolled, err := steward.OpenEnrolledBinary(root)
			if err != nil {
				t.Fatal(err)
			}
			install := enrolled.Install
			_ = enrolled.Close()
			install.InstallPath = filepath.Join(root, "bin", "metasystem")
			if err := steward.MintIdentity(steward.RepoIdentityPath(root), install); err != nil {
				t.Fatal(err)
			}
			startfixture.Register(t, b.home, root)
			read := startfixture.Executions(t, root, binary, map[string]string{"reaper": fault})
			effects := generationArmEffects(t, b.home, read)
			realUp := effects.up
			effects.up = func(options up.Options) up.Result { options.EnsureArmed = generationVerified; return realUp(options) }
			owners := b.owners()
			owners.processes.process.armSteps = effects.steps
			code, result := b.runJSON(owners, "system", "restart")
			if code == 0 || result.Outcome != intentPartial || !strings.Contains(result.Summary, "reaper") {
				t.Fatalf("restart accepted stale callback: %d %+v", code, result)
			}
			want := read(root)["reaper"].Digest
			if fault == "unknown" {
				want = "unverifiable"
				if !strings.Contains(result.Summary, "executable identity is unknown") {
					t.Fatalf("unknown cause lost: %+v", result)
				}
			}
			if !strings.Contains(result.Summary, want) {
				t.Fatalf("restart cause lost: %+v", result)
			}
			verdict := "mismatch"
			if fault == "unknown" {
				verdict = "unverifiable"
			}
			assertStartupGeneration(t, root, digest, verdict, read(root), 1)
		})
	}
}

func TestLandingDeployNeverRestartsThroughADeferredStart(t *testing.T) {
	t.Parallel()
	b, owners, _ := landingDeployBed(t)
	root := realpath.Resolve(filepath.Join(b.landingA, "metasystem"))
	binary, _ := startfixture.Stage(t, root)
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.template=true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	read := startfixture.Executions(t, root, binary, nil)
	effects := generationArmEffects(t, b.home, read)
	owners.landing.restartSupervisor = func(string) error {
		scope := processScope{Checkout: b.landingA, Binary: binary, Installation: stateroottest.Installation(t, root), Root: stateroottest.State(t, root)}
		_, err := restartLandingSupervisor(scope, effects, func(supervise.EnsureOptions) (supervise.EnsureResult, error) {
			return supervise.EnsureResult{Action: "deferred", Attempt: "proof-148"}, nil
		})
		return err
	}
	code, result := runLandingDeploy(t, b, owners, "--now", "--by", "Wido")
	if code == 0 || result.Outcome == intentConfirmed || !strings.Contains(result.Summary, "deferred") || !strings.Contains(result.Summary, "proof-148") {
		t.Fatalf("deferred deploy accepted: %d %+v", code, result)
	}
	data, err := os.ReadFile(filepath.Join(plain.Dir(root), "deploys.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var row plain.DeployRun
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatal(err)
		}
		if row.SupervisorRestarted {
			t.Fatalf("deferred deploy recorded a restart: %+v", row)
		}
	}
}
