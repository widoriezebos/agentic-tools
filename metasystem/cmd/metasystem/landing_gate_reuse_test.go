package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestLandingGateBaselineGreenNeverAnswersForMerge(t *testing.T) {
	t.Parallel()
	b := newMergeGateBed(t)
	var baseline plain.Result
	var running plain.Running
	b.fail = func(cmd *exec.Cmd, _ string) (string, error) {
		if commandEnv(cmd, "LANDING_COMMIT") == b.head {
			var err error
			baseline, _, err = plain.LastGate(b.install)
			if err != nil || baseline.Result != plain.Green || baseline.Requested != "merge-a" {
				t.Errorf("parent green belongs to parent: %+v err=%v", baseline, err)
			}
			if status := gateStatus(t, b); status.Result == plain.Green {
				t.Errorf("unfinished merge reads green: %+v", status)
			}
			running, _, _, err = plain.ReadRunning(b.install, b.owners.landing.plainProve)
			if err != nil {
				t.Fatal(err)
			}
		}
		return "LANDING-CHECKED\t0\n", nil
	}
	gateResult(t, b, 0)
	// A crash leaves the completed parent check and the merge's running record.
	data, err := json.Marshal(baseline)
	if err != nil {
		t.Fatal(err)
	}
	helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.install), "gates.jsonl"), append(data, '\n'), 0600))
	data, err = json.Marshal(running)
	if err != nil {
		t.Fatal(err)
	}
	helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.install), "running.json"), data, 0600))
	b.owners.landing.plainProve.Alive = func(plain.Running) bool { return false }
	if _, _, err := plain.Settled(b.install, b.root, b.owners.landing.plainProve); err != nil {
		t.Fatal(err)
	}
	status := gateStatus(t, b)
	if status.Result != plain.Red || status.Requested != b.head || status.Attributed != b.head || status.Cause == nil || status.Cause.Kind != "environment" || status.Cause.Name != "lost-process" {
		t.Fatalf("dead merge gate: %+v", status)
	}
}

func TestLandingGateStaticRedUsesParentAttribution(t *testing.T) {
	t.Parallel()
	for _, parent := range []string{plain.Green, plain.Red, "lost process", "not run"} {
		t.Run(parent, func(t *testing.T) {
			t.Parallel()
			b := newMergeGateBed(t)
			contract, err := testpolicy.Load(filepath.Join("..", "..", "testing.json"))
			if err != nil {
				t.Fatal(err)
			}
			goTool, err := exec.LookPath("go")
			if err != nil {
				t.Fatal(err)
			}
			for i, group := range contract.Groups {
				if group.ID == "fast-static-build" {
					contract.Groups[i] = testpolicy.Group{ID: group.ID, Kind: "build", Adapter: "command", CWD: ".", Phase: "acceptance", EnvironmentMode: "inherit",
						Inputs: []string{"*.go"}, Platforms: []string{"any"}, TargetMS: 1000, Argv: []string{goTool, "vet", "."}, Format: "exit-status"}
				}
			}
			contractData, err := json.Marshal(contract)
			if err != nil {
				t.Fatal(err)
			}
			command := b.owners.landing.plainProve.Command
			var statics []string
			b.owners.landing.plainProve.Command = func(cmd *exec.Cmd) error {
				if strings.HasSuffix(cmd.Args[len(cmd.Args)-1], " test groups fast-static-build") {
					commit := commandEnv(cmd, "LANDING_COMMIT")
					statics = append(statics, commit)
					if parent == "lost process" {
						return &exec.Error{Name: "static", Err: errors.New("tool unavailable")}
					}
					if parent == "not run" {
						fmt.Fprint(cmd.Stdout, "LANDING-NOT-RUN\tbusy\n")
						return nil
					}
					source := "package fixture\nimport \"fmt\"\nfunc broken() { fmt.Printf(\"%d\", \"bad\") }\n"
					if commit != b.head && parent == plain.Green {
						source = "package fixture\n"
					}
					for name, data := range map[string][]byte{"metasystem.conf": []byte("testing.contract=static.json\n"), "static.json": contractData, "go.mod": []byte("module fixture\n"), "fixture.go": []byte(source)} {
						helmMust(t, os.WriteFile(filepath.Join(cmd.Dir, name), data, 0600))
					}
					if code := runTestGroupsWithEnvironment([]string{"fast-static-build", "--root", cmd.Dir}, cmd.Env, cmd.Stdout, cmd.Stderr); code != 0 {
						return exec.Command("/usr/bin/false").Run()
					}
					return nil
				}
				return command(cmd)
			}
			red := gateResult(t, b, 1)
			want := "own"
			if parent == plain.Red {
				want = "main"
			} else if parent != plain.Green {
				want = "environment"
			}
			if red.Cause == nil || red.Cause.Kind != want || red.Static != plain.Red || red.Requested != b.head || len(b.runs) != 0 {
				t.Fatalf("static attribution: %+v statics=%v tests=%v", red, statics, b.runs)
			}
			if want == "environment" {
				if red.Repeat != "allowed" || len(statics) != 1 {
					t.Fatalf("static environment allowance: %+v statics=%v", red, statics)
				}
			} else if len(statics) != 2 || statics[0] != b.head || statics[1] != "merge-a" || want == "own" && (red.Cause.Goal != "b" || red.Cause.SHA != "sha-b") {
				t.Fatalf("static parent replay: %+v statics=%v", red, statics)
			}
			if want != "environment" {
				log, err := os.ReadFile(red.Log)
				if err != nil || !strings.Contains(string(log), "fmt.Printf") || !strings.Contains(string(log), "landing group fast-static-build red") {
					t.Fatalf("missing real vet failure: %s err=%v", log, err)
				}
			}
			t.Logf("merge static red; parent %s; cause %s; repeat %q", parent, red.Cause.Kind, red.Repeat)
		})
	}
}

func TestLandingGatePlanErrorAllowsOneEnvironmentRepeat(t *testing.T) {
	t.Parallel()
	b := newMergeGateBed(t)
	command := b.owners.landing.plainProve.Command
	b.owners.landing.plainProve.Command = func(cmd *exec.Cmd) error {
		if len(cmd.Args) > 1 && cmd.Args[1] == "test" {
			return errors.New("plan unavailable")
		}
		return command(cmd)
	}
	for _, repeat := range []string{"allowed", ""} {
		red := gateResult(t, b, 1)
		if red.Cause == nil || red.Cause.Kind != "environment" || red.Repeat != repeat || !strings.Contains(red.Reason, "plan unavailable") {
			t.Fatalf("plan error: %+v want repeat=%s", red, repeat)
		}
	}
	if code, out := b.run(t, b.root, "prove", "--gate", "--wait"); code != 1 || !strings.Contains(out, "gets no other") {
		t.Fatalf("third plan failure: exit=%d %s", code, out)
	}
}

func gateStatus(t *testing.T, b *replayVerbBed) plain.Result {
	t.Helper()
	b.owners.landing.view = func(string) lane.View { return lane.View{Root: &b.root, Owner: lane.OwnerView{State: lane.OwnerIdle}} }
	code, out := b.run(t, b.root, "status", "--json")
	var status struct{ Data plain.Status }
	if code != 0 || json.Unmarshal([]byte(out), &status) != nil || status.Data.LastGate == nil {
		t.Fatalf("status: exit=%d %s", code, out)
	}
	data, _ := json.Marshal(status.Data.LastGate)
	t.Logf("last_gate JSON: %s", data)
	return *status.Data.LastGate
}

func TestLandingGateNotCheapStillRunsStatic(t *testing.T) {
	t.Parallel()
	for _, static := range []string{plain.Green, plain.Red} {
		t.Run(static, func(t *testing.T) {
			t.Parallel()
			b := newMergeGateBed(t)
			b.owners.landing.plainProve.ImpactCost = func(string, string) (int, bool, error) { return 97, false, nil }
			command := b.owners.landing.plainProve.Command
			statics := 0
			b.owners.landing.plainProve.Command = func(cmd *exec.Cmd) error {
				if strings.HasSuffix(cmd.Args[len(cmd.Args)-1], " test groups fast-static-build") {
					statics++
					fmt.Fprintf(cmd.Stdout, "landing environment fixture toolchain\nlanding group fast-static-build %s 1\n", static)
					if static == plain.Red {
						return exec.Command("/usr/bin/false").Run()
					}
					return nil
				}
				return command(cmd)
			}
			wantCode, wantState := 0, plain.Skipped
			if static == plain.Red {
				wantCode, wantState = 1, plain.Red
			}
			result := gateResult(t, b, wantCode)
			status := gateStatus(t, b)
			wantStatics := 1
			if static == plain.Red {
				wantStatics = 2
				if result.Cause == nil || result.Cause.Kind != "main" {
					t.Fatalf("static parent failure: %+v", result)
				}
			}
			if statics != wantStatics || len(b.runs) != 0 || result.Result != wantState || result.Static != static || result.Requested != b.head || result.Attributed != b.head || status.Result != wantState || status.Requested != b.head {
				t.Fatalf("result=%+v status=%+v static runs=%d test runs=%v", result, status, statics, b.runs)
			}
			if static == plain.Green && result.Reason != "impact covers 97%; the batch check follows" {
				t.Fatalf("skip reason: %+v", result)
			}
		})
	}
}

func TestLandingStatusGateAbsentStaleAndBaselineRed(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"absent", "stale", "baseline"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			b := newMergeGateBed(t)
			if state == "stale" {
				writeCauseProof(t, b.install, "gates.jsonl", plain.Result{Requested: "merge-a", Attributed: "merge-a", Commit: "merge-a", Result: plain.Red})
			}
			if state == "baseline" {
				b.fail = func(*exec.Cmd, string) (string, error) { return replayFailure, exec.Command("/usr/bin/false").Run() }
				gateResult(t, b, 1)
			}
			status := gateStatus(t, b)
			if status.Requested != b.head {
				t.Fatalf("request: %+v", status)
			}
			if state == "baseline" {
				if status.Result != plain.Red || status.Attributed != "merge-a" || status.Cause == nil {
					t.Fatalf("baseline disappeared: %+v", status)
				}
			} else if status.Result != "none" || status.Attributed != "" {
				t.Fatalf("historical gate visible: %+v", status)
			}
		})
	}
}

func TestLandingStatusGateLastSection(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"none", plain.Skipped, plain.Red} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			b := newMergeGateBed(t)
			b.head = strings.Repeat("1", 40)
			b.owners.landing.view = func(string) lane.View {
				return lane.View{Root: &b.root, Owner: lane.OwnerView{State: lane.OwnerIdle}}
			}
			gate := plain.Result{Requested: b.head, Commit: strings.Repeat("2", 40), Tree: strings.Repeat("3", 40), Result: state}
			want := "none for 111111111111"
			switch state {
			case "none":
				writeCauseProof(t, b.install, "gates.jsonl", plain.Result{Requested: "old-head", Commit: "old-head", Tree: "old-tree", Result: plain.Green})
			case plain.Skipped:
				gate.Reason = "impact covers 97%; the batch check follows"
				want = "skipped for 222222222222 (tree 333333333333) (" + gate.Reason + ")"
				writeCauseProof(t, b.install, "gates.jsonl", gate)
			case plain.Red:
				gate.Reason = "static check failed"
				gate.Cause = &plain.Cause{Kind: "main"}
				want = "red (static check failed) for 222222222222 (tree 333333333333); cause: main"
				writeCauseProof(t, b.install, "gates.jsonl", gate)
			}
			code, out := b.run(t, b.root, "status")
			_, last, found := strings.Cut(out, "\nLast\n")
			if code != 0 || !found || strings.TrimSpace(last) != "gate   "+want {
				t.Fatalf("gate last section: exit=%d\n%s\nwant: %s", code, out, want)
			}
			t.Logf("rendered Last section:\nLast\n%s", last)
			code, out = b.run(t, b.root, "status", "--json")
			var result struct {
				Data struct {
					LastGate map[string]json.RawMessage `json:"last_gate"`
				}
			}
			if err := json.Unmarshal([]byte(out), &result); err != nil || code != 0 {
				t.Fatalf("gate JSON: exit=%d %s (%v)", code, out, err)
			}
			if string(result.Data.LastGate["requested"]) != `"`+b.head+`"` || string(result.Data.LastGate["result"]) != `"`+state+`"` {
				t.Fatalf("gate lost requested HEAD or result: %s", out)
			}
			if _, present := result.Data.LastGate["classification-policy"]; present {
				t.Fatalf("zero classification policy emitted: %s", out)
			}
		})
	}
}

func TestLandingProveReusesGateOnlyAtBatchBaseWithStaticAndEnvironment(t *testing.T) {
	t.Parallel()
	for _, mutation := range []string{"one member", "two members", "static red", "static absent", "environment", "depth", "tree", "gate red"} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel()
			b := newMergeGateBed(t)
			if mutation != "two members" {
				b.head = "merge-a"
			}
			b.prepareBatch(t)
			seedCurrentTrunkClock(t, b.install, laneTestNow)
			if mutation != "two members" {
				batch, err := plain.ReadBatch(b.install)
				if err != nil {
					t.Fatal(err)
				}
				batch.Members = batch.Members[:1]
				data, err := json.Marshal(batch)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(plain.Dir(b.install), "batch.json"), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			// A single merge has the batch's fetched main as its comparison base.
			b.fail = func(*exec.Cmd, string) (string, error) { return "LANDING-CHECKED\t0\n", nil }
			gate := gateResult(t, b, 0)
			switch mutation {
			case "static red":
				gate.Static = plain.Red
			case "static absent":
				gate.Static = ""
			case "environment":
				gate.Environment = "other toolchain"
			case "depth":
				gate.Depth = "full"
			case "tree":
				gate.Tree = "other-tree"
			case "gate red":
				gate.Result = plain.Red
			}
			writeCauseProof(t, b.install, "gates.jsonl", gate)
			runsBefore := len(b.runs)
			// A fresh batch proof fails, so only valid gate reuse can return green.
			b.fail = func(*exec.Cmd, string) (string, error) {
				return "LANDING-FAILED\tfast-static-build\t\nLANDING-CHECKED\t1\nlanding environment fixture toolchain\n", exec.Command("/usr/bin/false").Run()
			}
			code, out := b.run(t, b.root, "prove", "--impact", "--wait", "--json")
			var proof struct{ Data plain.Result }
			if err := json.Unmarshal([]byte(out), &proof); err != nil {
				t.Fatalf("proof: %s %v", out, err)
			}
			if mutation == "one member" {
				if code != 0 || proof.Data.Result != plain.Green || proof.Data.Scope != "impact" || proof.Data.BaseCommit != "main" || proof.Data.Environment != gate.Environment || proof.Data.Reason != "proven by attempt "+gate.Attempt || len(b.runs) != runsBefore {
					t.Fatalf("not reused: exit=%d %+v runs=%v output=%s", code, proof.Data, b.runs, out)
				}
				git := b.owners.landing.plainProve.Git
				b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
					if args[0] == "push" {
						return "", nil
					}
					return git(dir, args...)
				}

				if code, out := b.run(t, b.root, "push", "--json"); code != 0 {
					t.Fatalf("reused impact proof refused push: exit=%d %s", code, out)
				}

			} else {
				if code != 1 || proof.Data.Result != plain.Red || len(b.runs) == runsBefore || strings.Contains(proof.Data.Reason, "proven by attempt") {
					t.Fatalf("invalid reuse: exit=%d %+v runs=%v output=%s", code, proof.Data, b.runs, out)
				}
				if code, out := b.run(t, b.root, "push", "--json"); code != 1 {
					t.Fatalf("red pushed: exit=%d %s", code, out)
				}
			}
		})
	}
}

func TestSkillLandingAgentGateStatesAndBatchDepth(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "skills", "landing-agent", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, row := range []struct{ marker, next string }{
		{"1. **One waiting", "2. **Several waiting"},
		{"2. **Several waiting", "3. **Red"},
		{"5. **Main moved", "6. **Lane paused"},
	} {
		marker := row.marker
		_, section, found := strings.Cut(text, marker)
		if !found {
			t.Fatalf("missing case %s", marker)
		}
		section, _, _ = strings.Cut(section, "\n"+row.next)
		for _, state := range []string{"green or skipped", "none means the merge gate has not run for this merge", "proof at the batch's depth is green"} {
			if !strings.Contains(section, state) {
				t.Errorf("%s omits %q", marker, state)
			}
		}
	}
}
