package main

import (
	"encoding/json"
	"errors"
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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

func freshProofEnv(cmd *exec.Cmd, key string) string {
	for _, item := range cmd.Env {
		if value, ok := strings.CutPrefix(item, key+"="); ok {
			return value
		}
	}
	return ""
}

func TestLandingTrunkAlwaysRunsFreshFullAndClearsIncidents(t *testing.T) {
	t.Parallel()
	b := newResolveVerbFixture(t)
	register := newIntentBed(t, false, nil)
	register.repo.commits["main"] = register.repo.commit(register.repo.canonical)
	owners := incidentRecorderFixture(t, register, "finder")
	b.owners.landing.mainEndpoint, b.owners.landing.machine = owners.mainEndpoint, owners.machine
	b.owners.landing.now = owners.now
	now := laneTestNow
	calls, fetches, worktrees, id := 0, 0, 0, 0
	red := false
	b.owners.landing.plainProve = plain.ProveSeams{
		Now: func() time.Time { return now }, NewID: func() string { id++; return fmt.Sprintf("trunk-%d", id) },
		Judge: func(string, string, []plain.FailedUnit) (map[string]plain.UnitJudgement, error) { return nil, nil },
		Git: func(_ string, args ...string) (string, error) {
			switch strings.Join(args, " ") {
			case "fetch origin main":
				fetches++
				return "", nil
			case "rev-parse --verify HEAD^{commit}":
				return "batch", nil
			case "rev-parse --verify HEAD^{tree}":
				return "batch-tree", nil
			case "rev-parse --verify origin/main^{commit}", "rev-parse --verify --quiet refs/remotes/origin/main^{commit}":
				return "main", nil
			case "rev-parse --verify main^{tree}":
				return "main-tree", nil
			case "show main:metasystem/metasystem.conf":
				return "proof.full=main-command\n", nil
			case "worktree list --porcelain", "worktree prune":
				return "", nil
			}
			if len(args) == 5 && args[0] == "worktree" && args[1] == "add" {
				if args[4] != "main" {
					t.Fatalf("proved %s instead of main", args[4])
				}
				worktrees++
				return "", os.MkdirAll(filepath.Join(args[3], "metasystem"), 0o755)
			}
			if len(args) == 4 && args[0] == "worktree" && args[1] == "remove" {
				return "", os.RemoveAll(args[3])
			}
			t.Fatalf("unstubbed Git: %v", args)
			return "", errors.New("unstubbed Git")
		},
		Command: func(cmd *exec.Cmd) error {
			calls++
			if cmd.Args[len(cmd.Args)-1] != "main-command" || freshProofEnv(cmd, "LANDING_COMMIT") != "main" || freshProofEnv(cmd, "LANDING_PROOF_SCOPE") != "full" || freshProofEnv(cmd, "LANDING_PROOF_BASE") != "" || freshProofEnv(cmd, "LANDING_PROOF_GROUPS") != "" {
				t.Fatalf("wrong command or scope: %v %v", cmd.Args, cmd.Env)
			}
			if red {
				fmt.Fprint(cmd.Stdout, replayFailure)
				return errors.New("red")
			}
			fmt.Fprint(cmd.Stdout, "LANDING-CHECKED\t0\n")
			return nil
		},
	}
	b.owners.landing.view = func(home string) lane.View {
		return lane.BuildView(lane.ViewSources{Home: home, Now: now, Owner: b.owners.landing.probe, Ready: b.owners.landing.ready, Wake: lane.WakeSources{Reasons: func(string) ([]string, error) { return plain.WakeReasons(b.install, b.root, time.Time{}, now) }}})
	}
	if err := os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("proof.trunk-every=2h\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	status := func(due bool) {
		t.Helper()
		code, out := b.run(t, b.root, "status", "--json")
		var data struct{ Data plain.Status }
		if err := json.Unmarshal([]byte(out), &data); err != nil || code != 0 || data.Data.Wake == nil || slices.Contains(data.Data.Wake.Reasons, plain.WakeFullDue) != due {
			t.Fatalf("status due=%v: %d %s %v", due, code, out, err)
		}
	}
	// Same tree, previous green and then previous red: neither authorizes reuse or refuses this check.
	for _, previous := range []string{plain.Green, plain.Red} {
		writeCauseProof(t, b.install, "results.jsonl", plain.Result{Tree: "main-tree", Commit: "main", Result: previous, Scope: "full", FullTree: "main-tree", At: now.Add(-3 * time.Hour).Format(time.RFC3339), FullAt: now.Add(-3 * time.Hour).Format(time.RFC3339), Trunk: true})
		status(true)
		before := calls
		red = true
		code, out := b.run(t, b.root, "prove", "--trunk", "--wait", "--json")
		var data struct{ Data plain.Result }
		if err := json.Unmarshal([]byte(out), &data); err != nil || code != 1 || !data.Data.Trunk || data.Data.Scope != "full" || data.Data.Cause == nil || data.Data.Cause.Kind != "main" || calls != before+2 {
			t.Fatalf("fresh red: %d %s calls=%d %v", code, out, calls, err)
		}
		status(false)
		register.repo.commits["main"] = register.repo.commit(register.repo.canonical)
		red = false
		before = calls
		code, out = b.run(t, b.root, "prove", "--trunk", "--wait", "--json")
		if err := json.Unmarshal([]byte(out), &data); err != nil || code != 0 || !data.Data.Trunk || data.Data.FullTree != data.Data.Tree || data.Data.FullAt != data.Data.At || data.Data.Scope != "full" || calls != before+1 {
			t.Fatalf("fresh green: %d %s calls=%d %v", code, out, calls, err)
		}
		_, listed := register.runJSON(register.owners(), "incident", "list")
		encoded, _ := json.Marshal(listed.Data)
		var incidents struct{ Incidents []goal.TrunkRedEntry }
		if err := json.Unmarshal(encoded, &incidents); err != nil || len(incidents.Incidents) != 0 {
			t.Fatalf("incidents not cleared: %s %v", encoded, err)
		}
		register.repo.commits["main"] = register.repo.commit(register.repo.canonical)
		// Another check of the green tree must execute again.
		before = calls
		code, out = b.run(t, b.root, "prove", "--trunk", "--wait", "--json")
		if code != 0 || calls != before+1 {
			t.Fatalf("green reused: %d %s calls=%d", code, out, calls)
		}
		now = now.Add(2 * time.Hour)
		status(true)
	}
	if fetches != 6 || worktrees != 8 {
		t.Fatalf("fetches=%d worktrees=%d", fetches, worktrees)
	}
}

func TestLandingMovedScriptIsFullAndMovedRecordIsScoped(t *testing.T) {
	t.Parallel()
	for _, script := range []bool{true, false} {
		t.Run(fmt.Sprintf("script=%v", script), func(t *testing.T) {
			t.Parallel()
			b := newReplayVerbBed(t)
			path := "metasystem/plans/page.md"
			if script {
				path = "scripts/check.sh"
			}
			contract := testpolicy.Contract{SchemaVersion: testpolicy.ExecutionContractSchemaVersion, ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"}, Surfaces: []testpolicy.Surface{{ID: "app", Paths: []string{"metasystem/**", "scripts/**"}, Standard: []string{"cover"}}}, Unknown: []string{"cover"}}
			for _, group := range []string{"cover", "other"} {
				inputs := []string{path}
				if group == "other" {
					inputs = []string{"other/**"}
				}
				contract.Groups = append(contract.Groups, testpolicy.Group{ID: group, Kind: "unit", Adapter: "command", CWD: ".", Phase: "acceptance", EnvironmentMode: "inherit", Inputs: inputs, Platforms: []string{"any"}, TargetMS: 1, Argv: []string{"tests"}, Format: "exit-status"})
			}
			encoded, err := json.Marshal(contract)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := testpolicy.Decode(encoded); err != nil {
				t.Fatal(err)
			}
			git := b.owners.landing.plainProve.Git
			moved := false
			b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
				if args[0] == "show" && strings.HasSuffix(args[1], ":metasystem/metasystem.conf") {
					return "proof.full=fixture\n", nil
				}
				if args[0] == "show" && strings.HasSuffix(args[1], ":metasystem/testing.json") {
					return string(encoded), nil
				}
				if args[0] == "rev-list" {
					return "sha-a\nsha-b", nil
				}
				if args[0] == "diff" && moved {
					return path, nil
				}
				return git(dir, args...)
			}
			b.owners.landing.plainProve.Closure = func(string, string, string) (adapter.Closure, error) { return adapter.Closure{}, nil }
			scopes, groups := []string{}, []string{}
			b.fail = func(cmd *exec.Cmd, _ string) (string, error) {
				scopes = append(scopes, freshProofEnv(cmd, "LANDING_PROOF_SCOPE"))
				groups = append(groups, freshProofEnv(cmd, "LANDING_PROOF_GROUPS"))
				return "landing environment fixture\nlanding group cover passed 1\nLANDING-CHECKED\t0\n", nil
			}
			for i := 0; i < 2; i++ {
				code, out := b.run(t, b.root, "prove", "--wait", "--json")
				var data struct{ Data plain.Result }
				if err := json.Unmarshal([]byte(out), &data); err != nil || code != 0 {
					t.Fatalf("prove: %d %s %v", code, out, err)
				}
				if i == 1 {
					want := "scoped"
					if script {
						want = "full"
					}
					if data.Data.Scope != want || script && !strings.Contains(data.Data.ScopeReason, "an executable input changed: "+path) {
						t.Fatalf("scope: %+v", data.Data)
					}
				}
				moved = true
				b.head = "merge-b-moved"
			}
			if len(scopes) != 2 || scopes[0] != "full" || script && (scopes[1] != "full" || groups[1] != "") || !script && (scopes[1] != "scoped" || groups[1] != "cover") {
				t.Fatalf("scopes=%v groups=%v", scopes, groups)
			}
		})
	}
}

func TestLandingGateAndTrunkTogetherAreRefusedBeforeEffects(t *testing.T) {
	t.Parallel()
	b := newMergeGateBed(t)
	b.owners.landing.plainProve.Git = func(string, ...string) (string, error) {
		t.Fatal("conflicting proof modes reached Git")
		return "", errors.New("unexpected Git")
	}
	code, out := b.run(t, b.root, "prove", "--gate", "--trunk", "--wait", "--json")
	if code != 1 || !strings.Contains(out, "--gate and --trunk cannot be used together") || len(b.runs) != 0 {
		t.Fatalf("conflicting proof modes: %d %s runs=%v", code, out, b.runs)
	}
}
