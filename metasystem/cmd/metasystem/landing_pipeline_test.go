package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// Real merges and queue verbs prove the one-member policy and the return seen
// by a seat. Proof seams control the clock and package reports, never ancestry.
func TestOneGoalPerBatchLandsReturnsEveryRedAndReturnsAConflict(t *testing.T) {
	t.Parallel()
	for _, cause := range []string{"own", "main"} {
		t.Run(cause, func(t *testing.T) {
			t.Parallel()
			b := newBatchVerbBed(t, "1")
			b.now = time.Date(2026, 10, 1, 20, 0, 0, 0, time.UTC)
			b.owners.landing.now = func() time.Time { return b.now }
			script := filepath.Join(filepath.Dir(b.checkout), "impact-report")
			helmMust(t, testexec.WriteFile(script, []byte("#!/bin/sh\nprintf 'LANDING-CHECKED\\t0\\n'\n"), 0755))
			generator := filepath.Join(filepath.Dir(b.checkout), "generator")
			trace := generator + ".ran"
			helmMust(t, testexec.WriteFile(generator, []byte("#!/bin/sh\nprintf 'ran\\n' >> "+shellCommand([]string{trace})+"\n"), 0755))
			contract := testpolicy.Contract{SchemaVersion: testpolicy.ExecutionContractSchemaVersion,
				ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
				Surfaces:    []testpolicy.Surface{{ID: "fixture", Paths: []string{"metasystem/**"}, Standard: []string{"fixture"}}}, Unknown: []string{"fixture"},
				Groups:    []testpolicy.Group{{ID: "fixture", Kind: "unit", Adapter: "command", CWD: ".", Phase: "acceptance", EnvironmentMode: "inherit", Inputs: []string{"metasystem/**"}, Platforms: []string{"any"}, TargetMS: 1, Argv: []string{script}, Format: "exit-status"}},
				Generated: []testpolicy.Generated{{Paths: []string{"out/**"}, Command: []string{generator}}},
			}
			data, err := json.Marshal(contract)
			helmMust(t, err)
			helmMust(t, os.MkdirAll(filepath.Join(b.installation, "out"), 0755))
			helmMust(t, os.WriteFile(filepath.Join(b.installation, "testing.json"), data, 0644), os.WriteFile(filepath.Join(b.installation, "out/bundle"), []byte("base\n"), 0644))
			b.configuration += "testing.contract=testing.json\n"
			b.policy(t, "1")
			b.git(t, b.checkout, "add", "metasystem")
			b.git(t, b.checkout, "commit", "--quiet", "-m", "declare generated bundle")
			b.git(t, b.checkout, "push", "--quiet", "origin", "main")
			b.main = b.git(t, b.checkout, "rev-parse", "HEAD")
			b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
				if strings.Join(args, " ") == "show -s --format=%cI HEAD" {
					return b.now.UTC().Format(time.RFC3339), nil
				}
				return plain.Git(dir, args...)
			}
			b.owners.landing.plainProve.Judge = func(string, string, []plain.FailedUnit) (map[string]plain.UnitJudgement, error) { return nil, nil }
			b.success(t, "landing", "prove", "--trunk", "--wait")
			a, second := b.seat(t, "a"), b.seat(t, "b")
			b.success(t, "landing", "run")
			if got := b.batch(t).Members; !reflect.DeepEqual(got, []plain.GoalSHA{{Goal: "a", SHA: a}}) {
				t.Fatalf("first selection=%v", got)
			}
			b.assemble(t, a)
			b.now = b.now.Add(time.Minute)
			b.success(t, "landing", "prove", "--gate", "--wait")
			b.now = b.now.Add(time.Minute)
			b.success(t, "landing", "prove", "--wait")
			b.now = b.now.Add(time.Minute)
			b.success(t, "landing", "push")
			status := b.status(t)
			push := status["last_push"].(map[string]any)
			clock := plain.PushedClock(b.installation, plain.GoalSHA{Goal: "a", SHA: a})
			if clock == nil || clock.TotalMinutes == nil || *clock.TotalMinutes != 3 || clock.HandInAt["a"] != "2026-10-01T20:00:00Z" || len(clock.Proofs) == 0 || clock.Proofs[0].Result.Result != plain.Green || clock.PushAt != b.now.UTC().Format(time.RFC3339) {
				t.Fatalf("A landing clock=%+v", clock)
			}
			if push["clock"] == nil || queueStates(status)["a"] != plain.StateLanded || queueStates(status)["b"] != plain.StateWaiting {
				t.Fatalf("push clock/queue=%v", status)
			}
			b.success(t, "landing", "run")
			if got := b.batch(t).Members; !reflect.DeepEqual(got, []plain.GoalSHA{{Goal: "b", SHA: second}}) {
				t.Fatalf("second selection=%v", got)
			}
			b.assemble(t, second)
			parent := b.git(t, b.checkout, "rev-parse", "HEAD^1")
			merge := b.git(t, b.checkout, "rev-parse", "HEAD")
			var replays []string
			b.owners.landing.plainProve.Command = func(cmd *exec.Cmd) error {
				env := func(key string) string {
					for _, v := range cmd.Env {
						if value, ok := strings.CutPrefix(v, key+"="); ok {
							return value
						}
					}
					return ""
				}
				if only := env("LANDING_ONLY"); only != "" {
					if got := strings.Fields(only); !reflect.DeepEqual(got, []string{"u/first", "u/second"}) {
						t.Fatalf("replay units=%v; want both failed units", got)
					}
					replays = append(replays, env("LANDING_COMMIT"))
				}
				if env("LANDING_COMMIT") == merge {
					fmt.Fprint(cmd.Stdout, "LANDING-FAILED\tu/first\tTestFirst\nLANDING-FAILED\tu/second\tTestSecond\nLANDING-CHECKED\t2\n")
					return exec.Command("/usr/bin/false").Run()
				}
				if cause == "main" && (env("LANDING_ONLY") == "" || slices.Contains(strings.Fields(env("LANDING_ONLY")), "u/first")) {
					fmt.Fprint(cmd.Stdout, "LANDING-FAILED\tu/first\tTestFirst\nLANDING-CHECKED\t1\n")
					return exec.Command("/usr/bin/false").Run()
				}
				fmt.Fprint(cmd.Stdout, "LANDING-CHECKED\t0\n")
				return nil
			}
			code, text := b.run(t, "landing", "prove", "--wait")
			proof, ok, err := plain.LastResult(b.installation)
			if code != 1 || err != nil || !ok || proof.Cause == nil || proof.Cause.Kind != cause {
				t.Fatalf("proof=%+v replays=%v exit=%d %s err=%v", proof, replays, code, text, err)
			}
			// Return is an agent act: a person can intentionally return other causes.
			b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
				return humanauthority.Proof{}, fmt.Errorf("agent has no enrolled terminal")
			}
			if cause == "main" {
				before := idemTreeDigest(t, plain.Dir(b.installation))
				code, text = b.run(t, "landing", "return", "b", "--cause", "own")
				entry, _, err := plain.Latest(b.installation, "b")
				if code == 0 || err != nil || entry.State != plain.StateWaiting {
					t.Fatalf("main returned=%d %s entry=%+v err=%v", code, text, entry, err)
				}
				idemSameTree(t, "main return refused", before, idemTreeDigest(t, plain.Dir(b.installation)))
				b.cwd = b.checkout
				code, text = b.run(t, "landing", "status")
				if code != 0 || !strings.Contains(oneSpaced(text), "u/first") {
					t.Fatalf("main unit missing=%d %s", code, text)
				}
				t.Run("parentReplay", func(t *testing.T) {
					t.Parallel()
					if !reflect.DeepEqual(replays, []string{parent}) {
						t.Fatalf("replay runs=%v; want once on parent %s", replays, parent)
					}
				})
				return
			}
			b.success(t, "landing", "return", "b", "--cause", "own")
			entry, _, err := plain.Latest(b.installation, "b")
			for _, name := range []string{"u/first", "TestFirst", "u/second", "TestSecond"} {
				if err != nil || !strings.Contains(entry.Reason, name) {
					t.Fatalf("incomplete return=%+v err=%v", entry, err)
				}
			}
			pipelineSeatSeesReturn(t, b, "b", second, []string{"u/first", "TestFirst", "u/second", "TestSecond"})
			b.git(t, b.checkout, "checkout", "--quiet", "--detach", "origin/main")
			b.seat(t, "c")
			writer := filepath.Join(filepath.Dir(b.checkout), "seat-c")
			helmMust(t, os.WriteFile(filepath.Join(writer, "metasystem/out/bundle"), []byte("goal\n"), 0644))
			b.git(t, writer, "commit", "--quiet", "-am", "change generated bundle")
			b.git(t, writer, "push", "--quiet", "origin", "goal/c")
			third := b.git(t, writer, "rev-parse", "HEAD")
			_, _, err = plain.HandIn(b.installation, plain.Line{Goal: "c", SHA: third, Branch: "goal/c"})
			helmMust(t, err)
			b.advanceMain(t, "metasystem/out/bundle", "main\n")
			b.git(t, b.checkout, "checkout", "--quiet", "--detach", "origin/main")
			begun := filepath.Join(plain.Dir(b.installation), "resolve-begun.json")
			helmMust(t, os.WriteFile(begun, []byte("obsolete record"), 0600))
			if _, err := plain.Git(b.checkout, "merge", "--no-ff", "--no-edit", third); err == nil {
				t.Fatal("bundle merge did not conflict")
			}
			b.owners.landing.plainResolve = plain.ResolveSeams{Git: plain.Git, Now: func() time.Time { return b.now }}
			b.success(t, "landing", "resolve")
			entry, _, err = plain.Latest(b.installation, "c")
			if err != nil || entry.State != plain.StateReturned || !strings.Contains(entry.Reason, "metasystem/out/bundle (generated)") || !strings.Contains(entry.Reason, "work rebase c") {
				t.Fatalf("conflict return=%+v err=%v", entry, err)
			}
			for _, path := range []string{trace, filepath.Join(plain.Dir(b.installation), "regenerate.jsonl"), filepath.Join(b.checkout, ".git/MERGE_HEAD")} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("unexpected %s: %v", path, err)
				}
			}
			if got := b.git(t, b.checkout, "status", "--porcelain"); got != "" {
				t.Fatalf("checkout dirty: %s", got)
			}
			code, text = b.plainVerbBed.run(t, "landing", "status")
			if code != 0 || !strings.Contains(oneSpaced(text), "Stale resolve-begun.json: ignored") {
				t.Fatalf("stale record status=%d %s", code, text)
			}
			pipelineSeatSeesReturn(t, b, "c", third, []string{"metasystem/out/bundle", "work rebase c"})
			before := idemTreeDigest(t, plain.Dir(b.installation))
			code, text = b.run(t, "landing", "resolve")
			if code != 0 || !strings.Contains(text, "nothing was changed") {
				t.Fatalf("repeat=%d %s", code, text)
			}
			idemSameTree(t, "clean resolve repeat", before, idemTreeDigest(t, plain.Dir(b.installation)))
			t.Run("parentReplay", func(t *testing.T) {
				t.Parallel()
				if !reflect.DeepEqual(replays, []string{parent}) {
					t.Fatalf("replay runs=%v; want once on parent %s", replays, parent)
				}
			})
		})
	}
}

func pipelineSeatSeesReturn(t *testing.T, lane *batchVerbBed, id, sha string, names []string) {
	t.Helper()
	seat, state, _ := plainLaneBedWith(t, true, "critic-root", "critic-root")
	file := seat.goalFile("standing-validation")
	file.Id = id
	seat.addGoal(file)
	design := filepath.Join(seat.root(), "plans/designs/landing-work.md")
	data, err := os.ReadFile(design)
	helmMust(t, err)
	seat.writeFile(design, strings.ReplaceAll(string(data), "standing-validation", id))
	state.status.BranchTip = sha
	seat.owners.laneInstall = func(string) (string, error) { return lane.installation, nil }
	seat.laneInputs = func(owners *intentOwners) {
		owners.landing = lane.owners.landing
		owners.policies = lane.owners.policies
	}
	code, result := seat.do("work", "land", id)
	if code != 1 || result.Outcome != intentRefused {
		t.Fatalf("seat return=%d %+v", code, result)
	}
	for _, name := range names {
		if !strings.Contains(result.Summary, name) {
			t.Fatalf("seat missing %s: %+v", name, result)
		}
	}
}
