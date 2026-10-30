package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

func seedCurrentTrunkClock(t *testing.T, install string, at time.Time) {
	t.Helper()
	if err := os.MkdirAll(plain.Dir(install), 0700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(plain.Result{Trunk: true, Scope: "full", Result: plain.Green, At: at.Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(filepath.Join(plain.Dir(install), "results.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = file.Write(append(data, '\n'))
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("seed trunk clock: %v %v", err, closeErr)
	}
}

// The proof is applicable to the fixture's main tree, independently of cadence.
func seedApplicableMainProof(t *testing.T, install, commit, tree string, at time.Time) {
	t.Helper()
	helmMust(t, os.MkdirAll(plain.Dir(install), 0700))
	data, err := json.Marshal(plain.Result{Trunk: true, Commit: commit, Tree: tree,
		Scope: "full", Result: plain.Green, At: at.Format(time.RFC3339),
		FullTree: tree, FullAt: at.Format(time.RFC3339), Environment: "fixture-toolchain"})
	helmMust(t, err)
	file, err := os.OpenFile(filepath.Join(plain.Dir(install), "results.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	helmMust(t, err)
	_, err = file.Write(append(data, '\n'))
	helmMust(t, err)
	helmMust(t, file.Close())
}

func TestLandingObservationUnprovenMainStartsOnce(t *testing.T) {
	t.Parallel()
	observe := func(root, home string, seams plain.ProveSeams) func(lane.Record) error {
		seams.Policy = func(string) (plain.PolicyValue, error) {
			return plain.PolicyValue{Value: "auto", Source: "fixture"}, nil
		}
		keeper := newLandingAgentKeeper(root, home, newTestLandingAgent(func(agent *landingAgent) {
			agent.proofEffects = seams
			agent.now = func() time.Time { return laneTestNow }
			agent.machine = func(string) (string, error) { return "fixture", nil }
		}))
		return keeper.Observe
	}
	t.Run("selection installation at root", func(t *testing.T) {
		t.Parallel()
		b := newSelectionBed(t)
		helmMust(t, os.Remove(filepath.Join(plain.Dir(b.lane), "results.jsonl")))
		b.wantTrunkLaunches = 1
		b.keeper.Observe = observe(b.lane, b.home, b.owners.landing.plainProve)
		for range 2 {
			helmMust(t, b.keeper.Observe(b.record))
		}
		if b.trunkLaunches != 1 || b.starts != 0 {
			t.Fatalf("trunk=%d agent=%d", b.trunkLaunches, b.starts)
		}
	})
	t.Run("drain installation under metasystem", func(t *testing.T) {
		t.Parallel()
		b := newDrainVerbBed(t)
		helmMust(t, os.Remove(filepath.Join(plain.Dir(b.install), "results.jsonl")))
		b.wantTrunkLaunches = 1
		b.keeper.Observe = observe(b.root, b.home, b.owners.landing.plainProve)
		record, present, err := lane.Read(b.home)
		if err != nil || !present {
			t.Fatalf("lane record: %v %v", present, err)
		}
		for range 2 {
			helmMust(t, b.keeper.Observe(record))
		}
		if b.trunkLaunches != 1 || b.launches != 0 {
			t.Fatalf("trunk=%d agent=%d", b.trunkLaunches, b.launches)
		}
	})
}

func TestLandingSelectDepthClassStatus(t *testing.T) {
	t.Parallel()
	b := newSelectionBed(t)
	helmMust(t, os.WriteFile(filepath.Join(b.lane, "metasystem.conf"), []byte("landing.batch=auto\nproof.trunk-every=1h\n"), 0600))
	b.owners.landing.plainProve.GoalTier = func(_, id string) (uint8, error) {
		if id == "a" {
			return 3, nil
		}
		return 1, nil
	}
	code, output := b.run(t, b.lane, "landing", "run")
	if code != 0 {
		t.Fatalf("landing run: exit %d, %s", code, output)
	}
	batch := b.batch(t)
	if batch.DepthClass != "full" || !slices.Equal(batch.Members, []plain.GoalSHA{{Goal: "a", SHA: "sha-a"}, {Goal: "b", SHA: "sha-b"}}) {
		t.Fatalf("mixed selection: %+v", batch)
	}
	code, output = b.run(t, b.lane, "landing", "status", "--json")
	if code != 0 {
		t.Fatalf("landing status: exit %d, %s", code, output)
	}
	var status struct {
		Data struct {
			plain.Status
			Batch *plain.Batch `json:"batch"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &status); err != nil {
		t.Fatal(err)
	}
	if status.Data.Batch == nil || status.Data.Batch.DepthClass != "full" || !slices.Equal(status.Data.Batch.Members, batch.Members) {
		t.Fatalf("status lost depth or reason: %s", output)
	}
	t.Logf("observed status: %s", status.Data.Summary)
}

// Real Git proves that the selected tree is the tree published on main.
func TestLandingFullDueBatchAdapterProofPushClock(t *testing.T) {
	t.Parallel()
	b := newBatchVerbBed(t, "auto")
	sha := b.seat(t, "low")
	b.owners.landing.plainProve.GoalTier = func(string, string) (uint8, error) { return 1, nil }
	b.owners.landing.plainProve.BatchDepth = func(string, string, plain.Running) (string, string) { return "impact", "low tier" }
	b.success(t, "landing", "run")
	batch := b.batch(t)
	if batch.DepthClass != "cheap" {
		t.Fatalf("cheap selection: %+v", batch)
	}
	b.assemble(t, sha)
	b.success(t, "landing", "prove", "--wait")
	proof, ok, err := plain.LastResult(b.installation)
	want := "full proof overdue since " + b.now.Local().Format("15:04")
	if err != nil || !ok || proof.Scope != "full" || proof.ScopeReason != want || proof.FullTree != proof.Tree || proof.FullAt != proof.At {
		t.Fatalf("overdue proof: %+v %v", proof, err)
	}
	b.success(t, "landing", "push")
	push, ok, err := plain.LastPush(b.installation)
	if err != nil || !ok || push.Tree != proof.Tree || push.BatchID != batch.ID {
		t.Fatalf("push: %+v %v", push, err)
	}
	reasons, err := plain.WakeReasons(b.installation, b.checkout, time.Time{}, b.now, b.owners.landing.plainProve)
	if err != nil {
		t.Fatal(err)
	}
	for _, reason := range reasons {
		if reason == plain.WakeFullDue {
			t.Fatalf("pushed full proof did not reset clock: %v", reasons)
		}
	}
	t.Logf("observed depth: %s; reason: %s; full-due cleared after push", proof.Scope, proof.ScopeReason)
}

// Real Git proves an unpublished batch green leaves the trunk clock overdue.
func TestLandingFullDueLedgerRefreshAdapterRunsFullAndPaysClock(t *testing.T) {
	t.Parallel()
	for _, wait := range []bool{false, true} {
		name := "background attempt executes full"
		if wait {
			name = "wait executes full"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newBatchVerbBed(t, "auto")
			// The detached child is claimed in this process through its saved attempt.
			b.owners.landing.plainProve.Launch = func([]string, string, string) (int64, error) { b.launches++; return int64(os.Getpid()), nil }
			b.owners.landing.now = func() time.Time { return b.now }
			b.owners.landing.plainProve.Alive = func(r plain.Running) bool { return r.Pid == int64(os.Getpid()) || r.Pid == int64(os.Getppid()) }
			sha := b.seat(t, "low")
			b.owners.landing.plainProve.GoalTier = func(string, string) (uint8, error) { return 1, nil }
			if b.owners.landing.proveSeams(b.installation).BatchDepth == nil {
				t.Fatal("production proof has no batch depth policy")
			}
			b.success(t, "landing", "run")
			b.assemble(t, sha)
			b.success(t, "landing", "prove", "--wait")
			full, ok, err := plain.LastResult(b.installation)
			if err != nil || !ok || full.Scope != "full" || b.executions(t) != 1 {
				t.Fatalf("initial full proof: %+v %v", full, err)
			}
			b.now = b.now.Add(time.Minute)
			writer := filepath.Join(t.TempDir(), "main-writer")
			b.git(t, filepath.Dir(b.checkout), "clone", "--quiet", b.origin, writer)
			ledger := filepath.Join(writer, "metasystem", "plans", "goals.md")
			if err := os.MkdirAll(filepath.Dir(ledger), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(ledger, []byte("ledger refresh\n"), 0644); err != nil {
				t.Fatal(err)
			}
			b.git(t, writer, "add", "metasystem/plans/goals.md")
			b.git(t, writer, "commit", "--quiet", "-m", "advance goal ledger")
			b.git(t, writer, "push", "--quiet", "origin", "HEAD:main")
			b.git(t, b.checkout, "fetch", "--quiet", "origin")
			b.git(t, b.checkout, "merge", "--quiet", "--no-ff", "--no-edit", "origin/main")
			words := []string{"landing", "prove"}
			if wait {
				words = append(words, "--wait")
			}
			b.success(t, words...)
			if !wait {
				running, recorded, _, err := plain.ReadRunning(b.installation, b.owners.landing.plainProve)
				if err != nil || !recorded {
					t.Fatalf("refresh admission: %+v %v", running, err)
				}
				b.success(t, "landing", "prove", "--wait", "--attempt", running.Attempt)
			}
			refreshed, ok, err := plain.LastResult(b.installation)
			if err != nil || !ok || refreshed.Result != plain.Green || refreshed.Scope != "full" || refreshed.FullTree != refreshed.Tree || refreshed.FullAt != refreshed.At || refreshed.Tree == full.Tree || refreshed.ScopeReason != "full proof overdue since "+laneTestNow.Local().Format("15:04") || b.executions(t) != 2 || b.launches != 2 {
				t.Fatalf("overdue refresh did not execute full: %+v %v; executions=%d launches=%d", refreshed, err, b.executions(t), b.launches)
			}
			reasons, err := plain.WakeReasons(b.installation, b.checkout, time.Time{}, b.now, b.owners.landing.plainProve)
			if err != nil || !slices.Contains(reasons, plain.WakeFullDue) {
				t.Fatalf("unpublished refreshed proof paid clock: %v %v", reasons, err)
			}
			b.success(t, "landing", "push")
			push, ok, err := plain.LastPush(b.installation)
			if err != nil || !ok || push.Tree != refreshed.Tree || push.BatchID != full.BatchID {
				t.Fatalf("refreshed tree not pushed: %+v %v", push, err)
			}
			reasons, err = plain.WakeReasons(b.installation, b.checkout, time.Time{}, b.now, b.owners.landing.plainProve)
			if err != nil || slices.Contains(reasons, plain.WakeFullDue) {
				t.Fatalf("pushed refreshed full did not pay clock: %v %v", reasons, err)
			}
			t.Logf("%s: overdue ledger refresh ran full; publication cleared full-due", name)
		})
	}
}

// Real Git pins the contract and ledger change; native group execution proves
// prerequisite closure and the two admission routes retain full provenance.
func TestLandingScopedLedgerRefreshAdapterRunsAffectedChecksAndKeepsClock(t *testing.T) {
	t.Parallel()
	for _, route := range []string{"existing batch through landing prove", "records baseline through keeper observation"} {
		t.Run(route, func(t *testing.T) {
			t.Parallel()
			b := newBatchVerbBed(t, "auto")
			// The detached child is claimed in this process through its saved attempt.
			b.owners.landing.plainProve.Launch = func([]string, string, string) (int64, error) { b.launches++; return int64(os.Getpid()), nil }
			b.owners.landing.now = func() time.Time { return b.now }
			originAt := b.now
			groupsTrace := filepath.Join(t.TempDir(), "groups")
			contract := testpolicy.Contract{SchemaVersion: testpolicy.ExecutionContractSchemaVersion,
				ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
				Surfaces:    []testpolicy.Surface{{ID: "fixture", Paths: []string{"metasystem/**"}, Standard: []string{"ledger"}}}, Unknown: []string{"ledger"}}
			for _, id := range []string{"prerequisite", "ledger", "unaffected"} {
				input := "metasystem/internal/**"
				var requires []string
				if id == "ledger" {
					input, requires = "metasystem/plans/**", []string{"prerequisite"}
				}
				contract.Groups = append(contract.Groups, testpolicy.Group{ID: id, Kind: "unit", Adapter: "command", CWD: ".", Phase: "acceptance", EnvironmentMode: "inherit", Inputs: []string{input}, Platforms: []string{"any"}, TargetMS: 1, Requires: requires,
					Argv: []string{"/bin/sh", "-c", `printf '%s\n' "$0" >> "$LANDING_GROUP_TRACE"`, id}, Format: "exit-status"})
			}
			encoded, err := json.Marshal(contract)
			helmMust(t, err)
			writeTestingFixtureFile(t, filepath.Join(b.installation, "testing.json"), encoded, 0600)
			b.configuration += "testing.contract=testing.json\n"
			b.policy(t, "auto")
			b.git(t, b.checkout, "add", "metasystem/testing.json", "metasystem/metasystem.conf")
			b.git(t, b.checkout, "commit", "-qm", "declare synthetic ledger checks")
			b.git(t, b.checkout, "push", "--quiet", "origin", "main")
			b.main = b.git(t, b.checkout, "rev-parse", "HEAD")
			seedCurrentTrunkClock(t, b.installation, originAt)
			b.owners.landing.plainProve.GoalTier = func(string, string) (uint8, error) { return 3, nil }
			b.owners.landing.plainProve.Closure = func(string, string, string) (adapter.Closure, error) { return adapter.Closure{}, nil }
			contractReads := 0
			b.owners.landing.plainProve.Git = func(root string, args ...string) (string, error) {
				if len(args) == 2 && args[0] == "show" && strings.HasSuffix(args[1], ":metasystem/testing.json") {
					contractReads++
				}
				return plain.Git(root, args...)
			}
			previous := b.owners.landing.plainProve.Command
			b.owners.landing.plainProve.Command = func(cmd *exec.Cmd) error {
				if err := previous(cmd); err != nil {
					return err
				}
				committed, err := testpolicy.Load(filepath.Join(cmd.Dir, "testing.json"))
				if err != nil {
					return err
				}
				ids := strings.Fields(commandEnv(cmd, "LANDING_PROOF_GROUPS"))
				if commandEnv(cmd, "LANDING_PROOF_SCOPE") == "full" {
					ids = []string{"ledger", "unaffected"}
				}
				results, err := proofrun.RunNamedGroups(context.Background(), cmd.Dir, committed, ids, append(cmd.Env, "LANDING_GROUP_TRACE="+groupsTrace))
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.Stdout, "landing environment fixture-toolchain")
				for _, result := range results {
					fmt.Fprintf(cmd.Stdout, "landing group %s %s %d\n", result.ID, result.Status, result.DurationMS)
					if result.Status != "green" {
						return fmt.Errorf("group %s: %v %s", result.ID, result.Reasons, result.Output)
					}
				}
				return nil
			}
			sha := b.seat(t, "low")
			b.success(t, "landing", "run")
			b.assemble(t, sha)
			b.success(t, "landing", "prove", "--wait")
			full, ok, err := plain.LastResult(b.installation)
			if err != nil || !ok || full.Scope != "full" || full.Environment != "fixture-toolchain" || b.executions(t) != 1 {
				t.Fatalf("baseline: %+v %v", full, err)
			}
			keeperRoute := strings.HasPrefix(route, "records baseline")
			if keeperRoute {
				b.success(t, "landing", "push")
			}
			b.now = b.now.Add(10 * time.Minute)
			writer := filepath.Join(t.TempDir(), "main-writer")
			b.git(t, filepath.Dir(b.checkout), "clone", "--quiet", b.origin, writer)
			path := "metasystem/plans/goals/fixture.md"
			writeTestingFixtureFile(t, filepath.Join(writer, path), []byte("# fixture\n\n- State: open\n- Intent: check ledger inputs\n"), 0600)
			b.git(t, writer, "add", path)
			b.git(t, writer, "commit", "-qm", "advance synthetic goal ledger")
			b.git(t, writer, "push", "--quiet", "origin", "HEAD:main")
			wantReason := "the batch's full proof is under an hour old; only groups whose inputs cover changed paths run"
			if keeperRoute {
				seams := b.owners.landing.plainProve
				seams.Alive = func(r plain.Running) bool { return r.Pid == int64(os.Getpid()) || r.Pid == int64(os.Getppid()) }
				b.owners.landing.plainProve.Alive = seams.Alive
				seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil }
				keeper := newLandingAgentKeeper(b.checkout, b.home, newTestLandingAgent(func(agent *landingAgent) {
					agent.proofEffects = seams
					agent.now = func() time.Time { return b.now }
					agent.machine = func(string) (string, error) { return "fixture", nil }
					agent.settings = func(string) (launch.Settings, error) { return launch.DefaultSettings(), nil }
				}))
				record, present, err := lane.Read(b.home)
				if err != nil || !present {
					t.Fatalf("lane: %v %v", present, err)
				}
				helmMust(t, keeper.Observe(record))
				running, recorded, _, err := plain.ReadRunning(b.installation, seams)
				if err != nil || !recorded || !running.Trunk || running.Admission.Scope != "scoped" {
					t.Fatalf("observation admission: %+v %v", running, err)
				}
				b.success(t, "landing", "prove", "--trunk", "--wait", "--attempt", running.Attempt)
				wantReason = "main moved by records only; 2 checks run, the rest holds from the " + originAt.Local().Format("15:04") + " check"
			} else {
				b.git(t, b.checkout, "fetch", "--quiet", "origin")
				b.git(t, b.checkout, "merge", "--quiet", "--no-ff", "--no-edit", "origin/main")
				b.success(t, "landing", "prove", "--wait")
			}
			if contractReads != 1 {
				t.Fatalf("one admission read the committed testing contract %d times, want 1", contractReads)
			}
			scoped, ok, err := plain.LastResult(b.installation)
			if err != nil || !ok || scoped.Result != plain.Green || scoped.Scope != "scoped" || scoped.ScopeReason != wantReason || scoped.FullTree != full.FullTree || scoped.FullAt != full.FullAt || scoped.Tree == full.Tree || b.executions(t) != 2 || b.launches != 2 || !slices.Equal(scoped.Ran, []string{"prerequisite", "ledger"}) {
				t.Fatalf("scoped refresh: %+v %v; executions=%d launches=%d", scoped, err, b.executions(t), b.launches)
			}
			data, err := os.ReadFile(groupsTrace)
			helmMust(t, err)
			if got := strings.Fields(string(data)); !slices.Equal(got, []string{"prerequisite", "ledger", "unaffected", "prerequisite", "ledger"}) {
				t.Fatalf("executed groups: %v", got)
			}
			if !keeperRoute {
				b.success(t, "landing", "push")
			}
			assertDue := func(at time.Time, want bool) {
				t.Helper()
				reasons, err := plain.WakeReasons(b.installation, b.checkout, time.Time{}, at, b.owners.landing.plainProve)
				if err != nil || slices.Contains(reasons, plain.WakeFullDue) != want {
					t.Fatalf("full due at %s: %v %v, want %v", at, reasons, err, want)
				}
			}
			// A short interval distinguishes the original credit from a scoped publication's time.
			helmMust(t, os.WriteFile(filepath.Join(b.installation, "metasystem.conf"), []byte(strings.ReplaceAll(b.configuration, "proof.trunk-every=10h", "proof.trunk-every=30m")), 0600))
			assertDue(originAt.Add(29*time.Minute), false)
			assertDue(originAt.Add(31*time.Minute), true)
			b.policy(t, "auto")
			assertDue(originAt.Add(59*time.Minute), false)
			// Main's own records check keeps its full green; a scoped push owes.
			assertDue(originAt.Add(time.Hour+time.Second), !keeperRoute)
			t.Logf("%s; executed prerequisite and ledger; unaffected retained; original full tree/time and clock retained", scoped.ScopeReason)
		})
	}
}

// Real Git proves the candidate includes a superseded hand-in shared by two waiting goals.
func TestLandingSelectAdapterSupersededAncestryKeepsFullBatch(t *testing.T) {
	t.Parallel()
	b := newBatchVerbBed(t, "auto")
	oldA := b.seat(t, "a")
	seatA := filepath.Join(filepath.Dir(b.checkout), "seat-a")
	if err := os.WriteFile(filepath.Join(seatA, "a.txt"), []byte("a fixed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	b.git(t, seatA, "add", "a.txt")
	b.git(t, seatA, "commit", "--quiet", "-m", "fix a")
	newA := b.git(t, seatA, "rev-parse", "HEAD")
	b.git(t, seatA, "push", "--quiet", "origin", "goal/a")
	if _, _, err := plain.HandIn(b.installation, plain.Line{Goal: "a", SHA: newA}); err != nil {
		t.Fatal(err)
	}
	seatB := filepath.Join(t.TempDir(), "seat-b")
	b.git(t, filepath.Dir(b.checkout), "clone", "--quiet", b.origin, seatB)
	b.git(t, seatB, "checkout", "--quiet", "-b", "goal/b", oldA)
	if err := os.WriteFile(filepath.Join(seatB, "b.txt"), []byte("b\n"), 0644); err != nil {
		t.Fatal(err)
	}
	b.git(t, seatB, "add", "b.txt")
	b.git(t, seatB, "commit", "--quiet", "-m", "b stacked on old a")
	shaB := b.git(t, seatB, "rev-parse", "HEAD")
	b.git(t, seatB, "push", "--quiet", "origin", "goal/b")
	if _, _, err := plain.HandIn(b.installation, plain.Line{Goal: "b", SHA: shaB}); err != nil {
		t.Fatal(err)
	}
	entries, err := plain.Entries(b.installation)
	if err != nil || len(entries) != 3 || entries[0].State != plain.StateSuperseded {
		t.Fatalf("superseded fixture: %+v %v", entries, err)
	}
	b.owners.landing.plainProve.GoalTier = func(_, id string) (uint8, error) {
		if id == "a" {
			return 3, nil
		}
		return 1, nil
	}
	b.success(t, "landing", "run")
	batch := b.batch(t)
	if batch.DepthClass != "full" || !slices.Equal(batch.Members, []plain.GoalSHA{{Goal: "a", SHA: newA}, {Goal: "b", SHA: shaB}}) {
		t.Fatalf("shared superseded ancestry split: %+v", batch)
	}
	b.assemble(t, newA, shaB)
	b.success(t, "landing", "prove", "--wait")
	proof, ok, err := plain.LastResult(b.installation)
	if err != nil || !ok || proof.Result != plain.Green || proof.Scope != "full" || !slices.Equal(proof.BatchMembers, batch.Members) {
		t.Fatalf("shared ancestry failed proof admission: %+v %v", proof, err)
	}
	t.Log("selected both waiting goals at full depth; candidate containing superseded a passed admission")
}

// Real Git proves both waiting tips can share already-landed history.
func TestLandingSelectAdapterLandedAncestryAdmitsBothWaitingGoals(t *testing.T) {
	t.Parallel()
	b := newBatchVerbBed(t, "auto")
	b.owners.landing.plainProve.GoalTier = func(_, id string) (uint8, error) {
		if id == "a" {
			return 3, nil
		}
		return 1, nil
	}
	oldA := b.seat(t, "a")
	b.success(t, "landing", "run")
	b.assemble(t, oldA)
	b.success(t, "landing", "prove", "--wait")
	b.success(t, "landing", "push")

	seatA := filepath.Join(filepath.Dir(b.checkout), "seat-a")
	b.git(t, seatA, "fetch", "--quiet", "origin")
	b.git(t, seatA, "merge", "--quiet", "--no-edit", "origin/main")
	if err := os.WriteFile(filepath.Join(seatA, "a.txt"), []byte("a follow-up\n"), 0644); err != nil {
		t.Fatal(err)
	}
	b.git(t, seatA, "add", "a.txt")
	b.git(t, seatA, "commit", "--quiet", "-m", "a follow-up on main")
	newA := b.git(t, seatA, "rev-parse", "HEAD")
	b.git(t, seatA, "push", "--quiet", "origin", "goal/a")
	if _, _, err := plain.HandIn(b.installation, plain.Line{Goal: "a", SHA: newA, At: b.now.Format(time.RFC3339Nano)}); err != nil {
		t.Fatal(err)
	}
	shaB := b.seat(t, "b")
	entries, err := plain.Entries(b.installation)
	if err != nil || len(entries) != 3 || entries[0].State != plain.StateSuperseded {
		t.Fatalf("landed superseded fixture: %+v %v", entries, err)
	}
	if inside, err := plain.IsAncestor(b.checkout, oldA, "origin/main"); err != nil || !inside {
		t.Fatalf("earlier a hand-in is not on main: %v %v", inside, err)
	}

	b.success(t, "landing", "run")
	batch := b.batch(t)
	if batch.DepthClass != "full" || !slices.Equal(batch.Members, []plain.GoalSHA{{Goal: "a", SHA: newA}, {Goal: "b", SHA: shaB}}) {
		t.Fatalf("mixed waiting goals were split despite shared landed history: %+v", batch)
	}
	b.assemble(t, newA, shaB)
	b.success(t, "landing", "prove", "--wait")
	b.success(t, "landing", "push")
	for _, sha := range []string{newA, shaB} {
		if inside, err := plain.IsAncestor(b.checkout, sha, "origin/main"); err != nil || !inside {
			t.Fatalf("selected member was not published: %s %v %v", sha, inside, err)
		}
	}
	b.success(t, "landing", "run")
	if closed := b.batch(t); closed.State != plain.BatchClosed || closed.ID != batch.ID {
		t.Fatalf("landed members started another batch: %+v", closed)
	}
	t.Log("landed a1; selected and pushed a2 and b together at full depth")
}
