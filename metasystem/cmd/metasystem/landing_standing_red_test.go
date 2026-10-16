package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

func TestLandingKeeperStandingRedReachesDetachedRun(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"identical tree", "affected records", "changed environment", "hourly debt"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			b := newSelectionBed(t)
			helmMust(t, os.MkdirAll(filepath.Join(b.lane, "bin"), 0700))
			helmMust(t, os.Symlink("/usr/bin/true", filepath.Join(b.lane, "bin", "metasystem")))
			helmMust(t, os.Remove(filepath.Join(plain.Dir(b.lane), "queue.jsonl")))
			helmMust(t, os.WriteFile(filepath.Join(b.lane, "metasystem.conf"), []byte("testing.contract=testing.json\nproof.trunk-every=4h\n"), 0600))
			contract := testpolicy.Contract{SchemaVersion: 2, ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"}, Surfaces: []testpolicy.Surface{{ID: "fixture", Paths: []string{"records/**", "src/**"}, Standard: []string{"broken"}}}, Unknown: []string{"broken"}}
			for _, id := range []string{"broken", "shared", "unrelated"} {
				contract.Groups = append(contract.Groups, testpolicy.Group{ID: id, Kind: "unit", Adapter: "command", CWD: ".", Phase: "acceptance", EnvironmentMode: "inherit", Inputs: []string{"records/" + id + "/**"}, Platforms: []string{"any"}, TargetMS: 1, Argv: []string{"fixture"}, Format: "exit-status"})
			}
			contract.Groups = append(contract.Groups, testpolicy.Group{ID: "packages", Kind: "unit", Adapter: "go", CWD: ".", Phase: "acceptance", EnvironmentMode: "inherit", Inputs: []string{"src/**"}, Platforms: []string{"any"}, TargetMS: 1, PackageSelection: "changed-and-consumers", Tests: json.RawMessage(`"all"`)})
			encoded, err := json.Marshal(contract)
			helmMust(t, err)
			_, err = testpolicy.Decode(encoded)
			helmMust(t, err)
			at := b.now.Add(-10 * time.Minute).Format(time.RFC3339)
			red := plain.Result{Trunk: true, Attempt: "original-red", Commit: "old-main", Tree: "old-tree", Result: plain.Red, Scope: "full", CountedFull: true, At: at, Environment: "fixture-toolchain", Cause: &plain.Cause{Kind: "main"}, Failed: []plain.FailedUnit{{Unit: "broken"}, {Unit: "metasystem/cmd/metasystem", Tests: []string{"TestX"}}}}
			writeCauseProof(t, b.lane, "results.jsonl", red)
			seams := b.owners.landing.plainProve
			seams.Trunk = true
			seams.Policy = func(string) (plain.PolicyValue, error) { return plain.PolicyValue{Value: "auto"}, nil }
			fallback := seams.Git
			tree := "old-tree"
			if mode == "affected records" {
				tree = "records-tree"
			}
			fetches, launches, calls, sightings := 0, 0, 0, 0
			seams.Git = func(dir string, args ...string) (string, error) {
				switch args[0] {
				case "worktree":
					if args[1] == "add" {
						return "", os.MkdirAll(args[3], 0700)
					}
					if args[1] == "remove" {
						return "", os.RemoveAll(args[3])
					}
					return "", nil
				case "fetch":
					fetches++
					return "", nil
				case "rev-parse":
					if strings.Contains(args[len(args)-1], "origin/main^{commit}") {
						return "remote-records", nil
					}
					if args[len(args)-1] == "remote-records^{tree}" {
						return tree, nil
					}
				case "show":
					if args[1] == tree+":testing.json" {
						return string(encoded), nil
					}
					if args[1] == "remote-records:metasystem.conf" {
						return "proof.full=fixture-proof\n", nil
					}
				case "ls-tree":
					return "metasystem/cmd/metasystem/main.go", nil
				case "merge-base":
					if args[2] != "candidate" {
						return "", nil
					}
				case "diff":
					if mode == "affected records" {
						return "records/shared/change.md", nil
					}
					return "", nil
				}
				return fallback(dir, args...)
			}
			incidents := []goal.TrunkRedEntry{{ID: "broken", Identity: "red:broken", Sightings: []goal.TrunkRedSighting{{BaseCommit: red.Commit}}}, {ID: "native", Identity: "red:metasystem/cmd/metasystem:TestX", Sightings: []goal.TrunkRedSighting{{BaseCommit: red.Commit}}}}
			seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) { return incidents, nil }
			seams.RecordMain = func(checks []plain.Result) error {
				sightings++
				for _, check := range checks {
					for _, failure := range check.Failed {
						if failure.Inherited || failure.Unit != "shared" {
							t.Fatalf("carried failure recorded %+v", failure)
						}
					}
				}
				incidents = append(incidents, goal.TrunkRedEntry{ID: "shared", Identity: "red:shared", Sightings: []goal.TrunkRedSighting{{BaseCommit: "remote-records"}}})
				return nil
			}
			seams.Closure = func(string, string, string) (adapter.Closure, error) { return adapter.Closure{}, nil }
			seams.Executable = func() (string, error) { return "fixture-engine", nil }
			seams.Launch = func([]string, string, string) (int64, error) { launches++; return int64(os.Getpid()), nil }
			seams.Alive = func(r plain.Running) bool { return !r.Checkpoint }
			seams.Command = func(cmd *exec.Cmd) error {
				calls++
				environment := "fixture-toolchain"
				if mode == "changed environment" {
					environment = "different-toolchain"
				}
				fmt.Fprintln(cmd.Stdout, "landing environment "+environment)
				for _, id := range strings.Fields(commandEnv(cmd, "LANDING_PROOF_GROUPS")) {
					fmt.Fprintf(cmd.Stdout, "landing group %s green 1\n", id)
				}
				if mode == "hourly debt" && commandEnv(cmd, "LANDING_PROOF_SCOPE") == "full" {
					fmt.Fprintln(cmd.Stdout, "LANDING-FAILED\tshared\t\nLANDING-CHECKED\t1")
					return exec.Command("/usr/bin/false").Run()
				}
				fmt.Fprintln(cmd.Stdout, "LANDING-CHECKED\t0")
				return nil
			}
			keeper := newLandingAgentKeeper(b.lane, b.home, newTestLandingAgent(func(a *landingAgent) {
				a.proofEffects = seams
				a.now = func() time.Time { return b.now }
				a.machine = func(string) (string, error) { return "fixture", nil }
				a.settings = func(string) (launch.Settings, error) { return launch.DefaultSettings(), nil }
			}))
			helmMust(t, keeper.Observe(b.record))
			running, recorded, _, err := plain.ReadRunning(b.lane, seams)
			if err != nil || !recorded || running.Admission.Scope != "scoped" || launches != 1 || fetches != 1 {
				t.Fatalf("observation %+v %v launches=%d fetches=%d", running, err, launches, fetches)
			}
			log, err := os.Create(running.Log)
			helmMust(t, err)
			defer log.Close()
			r, err := plain.Run(b.lane, b.lane, "fixture-proof", running.Attempt, log, seams)
			helmMust(t, err)
			if mode == "changed environment" {
				if r.Scope != "full" || calls != 2 || !r.CountedFull {
					t.Fatalf("empty selection did not observe environment %+v calls=%d", r, calls)
				}
				return
			}
			if r.Result != plain.Red || r.Scope != "scoped" || r.CountedFull || r.FullTree != "" || r.FullAt != "" || r.OwedSince.Format(time.RFC3339) != at || len(r.Failed) != 2 || sightings != 0 {
				t.Fatalf("inherited result %+v sightings=%d", r, sightings)
			}
			for _, failure := range r.Failed {
				if !failure.Inherited || failure.Attempt != red.Attempt {
					t.Fatalf("failure %+v", failure)
				}
			}
			var scope struct {
				Groups []struct{ ID, State, From string }
			}
			data, err := os.ReadFile(filepath.Join(plain.Dir(b.lane), "proofs", r.Attempt+".scope.json"))
			helmMust(t, err, json.Unmarshal(data, &scope))
			want := map[string]string{"broken": "standing", "shared": "unproved", "unrelated": "unproved", "unit/cmd/metasystem": "standing"}
			if mode == "affected records" {
				want["shared"] = "ran"
			}
			if len(scope.Groups) != len(want) {
				t.Fatalf("scope %+v", scope)
			}
			for _, group := range scope.Groups {
				if group.State != want[group.ID] || group.State == "standing" && group.From != red.Tree {
					t.Fatalf("provenance %+v", group)
				}
			}
			if len(r.Ran) != map[bool]int{true: 1}[mode == "affected records"] {
				t.Fatalf("executed groups %+v", r.Ran)
			}
			helmMust(t, keeper.Observe(b.record))
			if launches != 1 || calls != 1 {
				t.Fatalf("exact red repeated launches=%d calls=%d", launches, calls)
			}
			if mode == "hourly debt" {
				b.now = b.now.Add(time.Hour)
				hold, err := keeper.Clock(b.record, func(lane.Record) error { return nil })
				helmMust(t, err)
				if !strings.Contains(hold, "hourly debt") {
					t.Fatalf("keeper clock did not request its hourly check: %s", hold)
				}
				next, _, _, err := plain.ReadRunning(b.lane, seams)
				helmMust(t, err)
				if next.Admission.Scope != "full" || !next.Fresh {
					t.Fatalf("hourly clock admission %+v", next)
				}
				fullLog, err := os.Create(next.Log)
				helmMust(t, err)
				defer fullLog.Close()
				paid, err := plain.Run(b.lane, b.lane, "fixture-proof", next.Attempt, fullLog, seams)
				helmMust(t, err)
				if paid.Result != plain.Red || !paid.CountedFull || sightings != 1 {
					t.Fatalf("hourly full result %+v sightings=%d", paid, sightings)
				}
				helmMust(t, keeper.Observe(b.record))
				_, err = keeper.Clock(b.record, func(lane.Record) error { return nil })
				helmMust(t, err)
				if launches != 2 || calls != 2 {
					t.Fatalf("counted full red repeated launches=%d calls=%d", launches, calls)
				}
			}

		})
	}
}
