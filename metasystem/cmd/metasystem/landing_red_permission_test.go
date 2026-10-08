package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/designgate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// Policy changes while the full command runs govern the next effect. Real
// Git is necessary here to attribute the selected merge against its parent.
func redPermissionBed(t *testing.T) (*batchVerbBed, plain.Result) {
	t.Helper()
	b := proofPermissionBed(t)
	b.owners.prove = enrolledPersonProver(t, b.installation, b.now)
	script := filepath.Join(filepath.Dir(b.checkout), "check")
	body := "#!/bin/sh\nprintf '%s|%s|%s\\n' \"$LANDING_COMMIT\" \"$LANDING_TREE\" \"$LANDING_PROOF_SCOPE\" >> " + shellCommand([]string{b.trace}) + "\nif [ \"$LANDING_COMMIT\" = " + shellCommand([]string{b.main}) + " ]; then printf 'LANDING-CHECKED\\t0\\n'; exit 0; fi\nprintf 'LANDING-FAILED\\tu/a\\tTestA\\nLANDING-CHECKED\\t1\\n'; exit 1\n"
	helmMust(t, testexec.WriteFile(script, []byte(body), 0755))
	b.owners.landing.plainProve.Judge = func(string, string, []plain.FailedUnit) (map[string]plain.UnitJudgement, error) { return nil, nil }
	b.owners.landing.plainProve.Command = func(cmd *exec.Cmd) error {
		if b.executions(t) == 0 {
			b.configuration += "landing.on-red=person\n"
			b.policy(t, "auto")
		}
		if b.executions(t) > 0 {
			observations, err := plain.Results(b.installation)
			saved := false
			for _, observation := range observations {
				saved = saved || observation.Result == plain.Red && len(observation.Failed) > 0
			}
			if err != nil || !saved {
				t.Fatal("attribution started without durable red")
			}
		}
		return cmd.Run()
	}
	code, text := b.run(t, "landing", "prove", "--wait")
	result, found, err := plain.LastResult(b.installation)
	if code == 0 || !strings.Contains(text, "LANE_RED_PERSON") || err != nil || !found || result.Result != plain.Red || !result.ClassificationPending || !result.CountedFull || b.executions(t) != 1 {
		t.Fatalf("red before continuation: %d %s %+v %v executions=%d", code, text, result, err, b.executions(t))
	}
	entry, _, err := plain.Latest(b.installation, "a")
	if err != nil || entry.State != plain.StateWaiting {
		t.Fatalf("red returned without its act: %+v %v", entry, err)
	}
	return b, result
}

func TestLandingClassifiedRedAgentProveDoesNotReopenRequest(t *testing.T) {
	t.Parallel()
	b, red := redPermissionBed(t)
	b.success(t, "landing", "prove", "--classify", red.Attempt)
	classified, found, err := plain.LastResult(b.installation)
	if err != nil || !found || classified.ClassificationPending || classified.ClassificationPerson == nil || classified.Repeat == "allowed" {
		t.Fatalf("classification did not finish: %+v %v", classified, err)
	}
	b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New("agent")
	}
	stopsPath := filepath.Join(plain.Dir(b.installation), "stops.jsonl")
	beforeStops, err := os.ReadFile(stopsPath)
	helmMust(t, err)
	beforeQuestions, unread := channel.WalkQuestions(b.installation)
	if len(unread) != 0 {
		t.Fatal(unread)
	}
	before, err := json.Marshal(beforeQuestions)
	helmMust(t, err)
	executions := b.executions(t)
	for range 2 {
		if code, text := b.run(t, "landing", "prove", "--wait"); code == 0 || !strings.Contains(text, "gets no other") {
			t.Fatalf("classified red must retain its repeat bound: %d %s", code, text)
		}
	}
	afterStops, err := os.ReadFile(stopsPath)
	helmMust(t, err)
	afterQuestions, unread := channel.WalkQuestions(b.installation)
	after, err := json.Marshal(afterQuestions)
	if err != nil || len(unread) != 0 || !bytes.Equal(beforeStops, afterStops) || !bytes.Equal(before, after) || b.executions(t) != executions {
		t.Fatalf("agent proof reopened classification or ran work: stops=%s questions=%s executions=%d err=%v unread=%v", afterStops, after, b.executions(t), err, unread)
	}
	if code, text := b.run(t, "landing", "return", "a", "--cause", "own"); code == 0 || !strings.Contains(text, "LANE_RETURN_PERSON") {
		t.Fatalf("classification supplied return authority: %d %s", code, text)
	}
}

func TestLandingRedPermissionAuthorityMatrix(t *testing.T) {
	t.Parallel()
	for _, act := range []string{"classification", "return"} {
		t.Run(act, func(t *testing.T) {
			t.Parallel()
			b, red := redPermissionBed(t)
			command := "metasystem landing prove --classify " + red.Attempt
			if act == "return" {
				b.success(t, "landing", "prove", "--classify", red.Attempt)
				b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
					return humanauthority.Proof{}, errors.New("agent")
				}
				if code, text := b.run(t, "landing", "return", "a", "--cause", "own"); code == 0 || !strings.Contains(text, "LANE_RETURN_PERSON") {
					t.Fatalf("automatic return: %d %s", code, text)
				}
				command = "metasystem landing return a --cause own --reason TEXT"
			}
			q := proofQuestion(t, b, command)
			words := strings.Fields(command)[1:]
			if act == "return" {
				words[len(words)-1] = "the person accepts this return"
			}
			calling := filepath.Join(t.TempDir(), "caller")
			b.git(t, filepath.Dir(calling), "clone", "--quiet", b.origin, calling)
			calling = realpath.Resolve(calling)
			root := filepath.Join(calling, "metasystem")
			b.cwd = calling
			_, err := humanauthority.Enroll(root, 20, person(), "Wido", b.now)
			helmMust(t, err)
			_, err = helm.Write(calling, helm.Record{By: "Wido", At: b.now.Format(time.RFC3339), Checkout: calling, Reason: "manual"})
			helmMust(t, err)
			fake := newFakeHelm(calling, filepath.Join(calling, ".git"), "DELEGATE")
			useHelmAdmitter(root, fake.admitter)
			t.Cleanup(func() { helmAdmitters.Delete(resolvedHelmRoot(root)) })
			pid := int64(80)
			var observed humanauthority.Proof
			var proofErr error
			b.owners.prove = func(at string, _ int64, _ humanauthority.Reader, _, _ string, now time.Time) (humanauthority.Proof, error) {
				if at != root {
					t.Errorf("borrowed enrollment at %s", at)
				}
				observed, proofErr = humanauthority.Prove(root, pid, person(), now)
				return observed, proofErr
			}
			before := b.executions(t)
			history, err := os.ReadFile(filepath.Join(plain.Dir(b.installation), "queue.jsonl"))
			helmMust(t, err)
			fake.machinery = true
			if code, text := b.run(t, words...); code == 0 || proofErr == nil {
				t.Fatalf("machinery act: %d %s", code, text)
			}
			fake.machinery, pid = false, 60
			if direct, err := humanauthority.Prove(root, pid, person(), b.now); err != nil || direct.Helm == nil {
				t.Fatalf("fallback not reached: %+v %v", direct, err)
			}
			if code, text := b.run(t, words...); code == 0 || proofErr != nil || observed.Helm == nil {
				t.Fatalf("helm act: %d %s %+v %v", code, text, observed, proofErr)
			}
			b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
				return humanauthority.HelmProof(root, humanauthority.HelmGrant{By: "Wido", Grant: "attorney"}, b.now)
			}
			if code, text := b.run(t, words...); code == 0 {
				t.Fatalf("attorney act: %s", text)
			}
			retained, err := os.ReadFile(filepath.Join(plain.Dir(b.installation), "queue.jsonl"))
			helmMust(t, err)
			current, err := channel.ReadQuestion(b.installation, q.ID)
			if err != nil || current.State == "closed" || !bytes.Equal(history, retained) || b.executions(t) != before {
				t.Fatalf("refused act had effects: %+v %v", current, err)
			}
			b.owners.prove = enrolledPersonProver(t, root, b.now)
			_, err = lane.SetPause(b.home, "Wido", b.now)
			helmMust(t, err)
			b.success(t, "landing", "run", "--goals", "a")
			b.owners.prove = enrolledPersonProver(t, root, b.now)
			if act == "return" {
				if code, text := b.run(t, append(words, "--by", "someone else")...); code == 0 {
					t.Fatalf("wrong by admitted: %s", text)
				}
				words = append(words, "--by", "human:Wido")
			}
			b.success(t, words...)
			current, err = channel.ReadQuestion(b.installation, q.ID)
			if err != nil || current.State != "closed" {
				t.Fatalf("effect question stayed open: %+v %v", current, err)
			}
			if act == "classification" {
				result, _, err := plain.LastResult(b.installation)
				if err != nil || result.ClassificationPending || result.ClassificationPerson == nil || result.ClassificationPerson.Root != root || result.Cause.Kind != "own" || result.Cause.Goal != "a" || b.executions(t) != before+2 {
					t.Fatalf("classified subject: %+v %v count=%d", result, err, b.executions(t))
				}
				entry, _, _ := plain.Latest(b.installation, "a")
				if entry.State != plain.StateWaiting {
					t.Fatal("classification authorized return")
				}
				// A restart can repeat attribution, but cannot use the full admission.
				b.success(t, words...)
				if b.executions(t) != before+4 {
					t.Fatal("classification replay did not remain isolated")
				}
				b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
					return humanauthority.Proof{}, errors.New("agent")
				}
				if code, text := b.run(t, "landing", "prove", "--wait"); code == 0 || !strings.Contains(text, "gets no other") || b.executions(t) != before+4 {
					t.Fatalf("classification lent full permission: %d %s", code, text)
				}
			} else {
				entry, _, err := plain.Latest(b.installation, "a")
				if err != nil || entry.State != plain.StateReturned || entry.SHA != red.Goals[0].SHA || entry.Reason != "the person accepts this return" || entry.Cause.Kind != "own" || entry.ReturnOrigin == nil || entry.ReturnOrigin.Person == nil || entry.ReturnOrigin.Person.Person != "Wido" || entry.ReturnOrigin.Person.Root != root {
					t.Fatalf("returned subject: %+v %v", entry, err)
				}
			}
			if !helm.Active(calling).Active {
				t.Fatal("act removed helm")
			}
			if _, paused := lane.ReadPause(b.home); !paused {
				t.Fatal("act removed pause")
			}
		})
	}
}

// Return progress is read across every queue line, including older tips,
// a human return, a new batch identity, and a return to the same commit.
func TestLandingReturnHistoryAcrossRoutes(t *testing.T) {
	t.Parallel()
	for _, route := range []string{"public", "source", "regeneration", "design"} {
		t.Run(route, func(t *testing.T) {
			t.Parallel()
			b := newResolveVerbFixture(t)
			helmMust(t, os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("metasystem.template=true\ntesting.contract=testing.json\n"), 0600))
			b.owners.landing.machine = func(string) (string, error) { return "fixture", nil }
			b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
				return humanauthority.Proof{}, errors.New("agent")
			}
			b.owners.resolver = stateroot.NewResolver(func(string) (string, error) { return b.root, nil }, os.Executable)
			sha := "goal-sha"
			invoke := func(tests []string) (int, string) {
				t.Helper()
				proof := plain.Result{Result: plain.Red, Attempt: "proof-" + sha, BatchID: "batch-" + sha, At: laneTestNow.Format(time.RFC3339), Cause: &plain.Cause{Kind: "own", Goal: "goal", SHA: sha, Tests: tests, Evidence: "failure.log"}, Goals: []plain.GoalSHA{{Goal: "goal", SHA: sha}}}
				writeCauseProof(t, b.install, "results.jsonl", proof)
				if route == "source" {
					b.sourceConflictGit(t, func() {})
					original := b.owners.landing.plainResolve.Git
					b.owners.landing.plainResolve.Git = func(dir string, args ...string) (string, error) {
						if strings.Join(args, " ") == "rev-parse --verify MERGE_HEAD^{commit}" {
							return sha, nil
						}
						return original(dir, args...)
					}
					return b.run(t, b.root, "resolve", "--json")
				}
				if route == "regeneration" {
					runs := 0
					b.owners.landing.plainResolve = plain.ResolveSeams{
						Git: func(_ string, args ...string) (string, error) {
							switch strings.Join(args, " ") {
							case "show HEAD:metasystem/testing.json":
								return b.contract, nil
							case "diff --name-only --diff-filter=U -z", "ls-files -z", "ls-tree -r --name-only -z HEAD -- metasystem/out/result":
								return "metasystem/out/result\x00", nil
							case "rev-parse --verify HEAD^{commit}":
								return "main-sha", nil
							case "rev-parse --verify MERGE_HEAD^{commit}":
								return sha, nil
							case "diff --name-only -z AUTO_MERGE --", "ls-files --others --exclude-standard -z", "restore --source=HEAD --staged --worktree -- metasystem/out/result", "restore --source=AUTO_MERGE --worktree -- metasystem/out/result", "merge --abort":
								return "", nil
							default:
								t.Fatalf("unexpected regeneration Git %v", args)
								return "", nil
							}
						},
						Run: func(_ []string, _ string, log *os.File, _ func(int64) error) error {
							runs++
							if runs > 1 {
								return nil
							}
							return exec.Command("/usr/bin/false").Run()
						},
					}
					return b.run(t, b.root, "resolve", "--json")
				}
				if route == "design" {
					b.owners.landing.plainProve.Git = func(string, ...string) (string, error) { return "", nil }
					b.owners.landing.contained = func(_, ref string) func(string) (bool, error) {
						return func(string) (bool, error) { return ref == "head", nil }
					}
					b.owners.landing.push = func(_, _ string, _ time.Time, before func(string, string) error) (plain.PushOutcome, error) {
						return plain.PushOutcome{}, before("old", "head")
					}
					command, _ := findIntentAction("landing", "push")
					command = laneCommand(command, func(inv *intentInvocation, admitted laneAdmitted) int {
						return runIntentLandingPushWithOwners(inv, admitted, landingPushOwners{facts: func(id string) landing.DesignFacts {
							return landing.DesignFacts{Facts: designgate.Facts{Goal: id, Tier: 2, Mode: "refuse"}}
						}, notify: func(plain.PushOutcome) error { return nil }})
					})
					var out bytes.Buffer
					code := runIntentIn(command, []string{"--json"}, &out, &out, b.root, b.owners)
					return code, out.String()
				}
				return b.run(t, b.root, "return", "goal", "--cause", "own", "--json")
			}
			helmMust(t, func() error { _, _, err := plain.HandIn(b.install, plain.Line{Goal: "goal", SHA: sha}); return err }())
			code, text := invoke([]string{"u/a TestA", "u/a TestB"})
			entry, _, err := plain.Latest(b.install, "goal")
			if err != nil || entry.State != plain.StateReturned {
				t.Fatalf("first %s return: %d %s %+v %v", route, code, text, entry, err)
			}
			sha = "next-sha"
			_, _, err = plain.HandIn(b.install, plain.Line{Goal: "goal", SHA: sha})
			helmMust(t, err)
			code, text = invoke([]string{"u/a TestA"})
			entry, _, err = plain.Latest(b.install, "goal")
			if code == 0 || err != nil || entry.State != plain.StateWaiting || !strings.Contains(text, "LANE_RETURN_PERSON") {
				t.Fatalf("repeated %s escaped: %d %s %+v %v", route, code, text, entry, err)
			}
			stops, err := plain.OpenStops(b.install)
			helmMust(t, err)
			if len(stops) == 0 {
				t.Fatal("return hold has no subject command")
			}
			// Human return changes neither the retained automatic history nor its budget.
			b.owners.prove = enrolledPersonProver(t, b.install, laneTestNow)
			if code, text = b.run(t, b.root, "return", "goal", "--cause", "unclassified", "--reason", "person chooses"); code != 0 {
				t.Fatalf("person remedy: %s", text)
			}
			b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
				return humanauthority.Proof{}, errors.New("agent")
			}
			sha = "third-sha"
			_, _, err = plain.HandIn(b.install, plain.Line{Goal: "goal", SHA: sha})
			helmMust(t, err)
			if route != "public" {
				code, text = invoke(nil)
				entry, _, _ = plain.Latest(b.install, "goal")
				if code == 0 || entry.State != plain.StateWaiting {
					t.Fatalf("human reset %s history: %d %s", route, code, text)
				}
				return
			}
			code, text = invoke([]string{"u/b TestNew"})
			entry, _, _ = plain.Latest(b.install, "goal")
			if code != 0 || entry.State != plain.StateReturned {
				t.Fatalf("shrinking different failures refused: %d %s", code, text)
			}
			sha = "fourth-sha"
			_, _, err = plain.HandIn(b.install, plain.Line{Goal: "goal", SHA: sha})
			helmMust(t, err)
			code, text = invoke([]string{"u/c TestNewer"})
			entry, _, _ = plain.Latest(b.install, "goal")
			if code == 0 || entry.State != plain.StateWaiting || !strings.Contains(text, "two automatic returns") {
				t.Fatalf("third automatic return: %d %s", code, text)
			}
		})
	}
}

func TestLandingRedPermissionFailedWritesAndCorruptAdvice(t *testing.T) {
	t.Parallel()
	b, red := redPermissionBed(t)
	command := "metasystem landing prove --classify " + red.Attempt
	q := proofQuestion(t, b, command)
	before := b.executions(t)
	b.owners.prove = enrolledPersonProver(t, b.installation, b.now)
	path := filepath.Join(plain.Dir(b.installation), "running.json.tmp")
	helmMust(t, os.Mkdir(path, 0755))
	if code, text := b.run(t, "landing", "prove", "--classify", red.Attempt); code == 0 || b.executions(t) != before {
		t.Fatalf("failed classify record: %d %s", code, text)
	}
	current, err := channel.ReadQuestion(b.installation, q.ID)
	if err != nil || current.State == "closed" {
		t.Fatalf("failed effect closed: %+v %v", current, err)
	}
	helmMust(t, os.Remove(path))
	b.owners.policies.Registry = func(string) (config.PolicyRegistry, error) {
		return config.PolicyRegistry{}, errors.New("unreadable policy")
	}
	b.success(t, "landing", "prove", "--classify", red.Attempt)
	b.success(t, "landing", "return", "a", "--cause", "environment", "--reason", "explicit supplied cause", "--by", "Wido")
	entry, _, err := plain.Latest(b.installation, "a")
	if err != nil || entry.State != plain.StateReturned || entry.Cause.Kind != "environment" || entry.ReturnOrigin.Warning == "" {
		t.Fatalf("advice vetoed person: %+v %v", entry, err)
	}
	// A nonexistent attempt is never inferred from HEAD or the latest proof.
	if code, text := b.run(t, "landing", "prove", "--classify", "missing"); code == 0 || !strings.Contains(text, "no recorded red") {
		t.Fatalf("missing result: %d %s", code, text)
	}
}

func TestLandingReturnNewFailureClassAndNonShrinkingSet(t *testing.T) {
	t.Parallel()
	for _, first := range []string{"conflict", "tests"} {
		t.Run(first, func(t *testing.T) {
			t.Parallel()
			b := newResolveVerbFixture(t)
			helmMust(t, os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("metasystem.template=true\ntesting.contract=testing.json\n"), 0600))
			b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
				return humanauthority.Proof{}, errors.New("agent")
			}
			b.owners.landing.machine = func(string) (string, error) { return "fixture", nil }
			_, _, err := plain.HandIn(b.install, plain.Line{Goal: "goal", SHA: "goal-sha"})
			helmMust(t, err)
			if first == "conflict" {
				b.sourceConflictGit(t, func() {})
				if code, text := b.run(t, b.root, "resolve"); code != 0 {
					t.Fatalf("first source conflict: %d %s", code, text)
				}
			} else {
				writeCauseProof(t, b.install, "results.jsonl", plain.Result{Result: plain.Red, Cause: &plain.Cause{Kind: "own", Goal: "goal", SHA: "goal-sha", Tests: []string{"u/a TestOld"}, Evidence: "old.log"}})
				if code, text := b.run(t, b.root, "return", "goal", "--cause", "own"); code != 0 {
					t.Fatalf("first test return: %d %s", code, text)
				}
			}
			_, _, err = plain.HandIn(b.install, plain.Line{Goal: "goal", SHA: "fixed"})
			helmMust(t, err)
			writeCauseProof(t, b.install, "results.jsonl", plain.Result{Result: plain.Red, BatchID: "renamed", Attempt: "new-proof", Cause: &plain.Cause{Kind: "own", Goal: "goal", SHA: "fixed", Tests: []string{"u/b TestNew"}, Evidence: "new.log"}})
			code, text := b.run(t, b.root, "return", "goal", "--cause", "own")
			entry, _, err := plain.Latest(b.install, "goal")
			if first == "conflict" {
				if code != 0 || err != nil || entry.State != plain.StateReturned {
					t.Fatalf("new failure class held: %d %s %+v %v", code, text, entry, err)
				}
				_, _, err = plain.HandIn(b.install, plain.Line{Goal: "goal", SHA: "another"})
				helmMust(t, err)
				writeCauseProof(t, b.install, "results.jsonl", plain.Result{Result: plain.Red, Cause: &plain.Cause{Kind: "own", Goal: "goal", SHA: "another", Tests: []string{"u/c TestOther"}, Evidence: "third.log"}})
				code, text = b.run(t, b.root, "return", "goal", "--cause", "own")
				entry, _, _ = plain.Latest(b.install, "goal")
				if code == 0 || entry.State != plain.StateWaiting || !strings.Contains(text, "two automatic returns") {
					t.Fatalf("third after new class: %d %s", code, text)
				}
			} else if code == 0 || err != nil || entry.State != plain.StateWaiting || !strings.Contains(text, "strictly shrunk") {
				t.Fatalf("equal-sized disjoint red set escaped: %d %s", code, text)
			}
		})
	}
}

func TestLandingClassifyRetainsFullBudget(t *testing.T) {
	t.Parallel()
	b, red := redPermissionBed(t)
	script := filepath.Join(filepath.Dir(b.checkout), "check")
	body := "#!/bin/sh\nprintf '%s|%s|%s\\n' \"$LANDING_COMMIT\" \"$LANDING_TREE\" \"$LANDING_PROOF_SCOPE\" >> " + shellCommand([]string{b.trace}) + "\nif [ -n \"$LANDING_ONLY\" ]; then printf 'LANDING-CHECKED\\t0\\n'; exit 0; fi\nprintf 'LANDING-FAILED\\tu/a\\tTestA\\nLANDING-CHECKED\\t1\\n'; exit 1\n"
	helmMust(t, testexec.WriteFile(script, []byte(body), 0755))
	b.success(t, "landing", "prove", "--classify", red.Attempt)
	classified, _, err := plain.LastResult(b.installation)
	if err != nil || !classified.ClassificationPending || b.executions(t) != 4 || len(classified.Executions) != 1 || classified.Person == nil {
		t.Fatalf("classification did not admit exactly its next full: %+v %v count=%d", classified, err, b.executions(t))
	}
	before := b.executions(t)
	b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New("agent")
	}
	if code, text := b.run(t, "landing", "prove", "--wait"); code == 0 || b.executions(t) != before {
		t.Fatalf("classifier lent another full admission: %d %s", code, text)
	}
	b.owners.prove = enrolledPersonProver(t, b.installation, b.now)
	next, _, err := plain.LastResult(b.installation)
	helmMust(t, err)
	b.success(t, "landing", "prove", "--classify", next.Attempt)
	// A renamed batch and another classification cannot restore spent allowance.
	b.configuration = strings.ReplaceAll(b.configuration, "landing.proof=person", "landing.proof=auto")
	b.configuration = strings.ReplaceAll(b.configuration, "landing.on-red=person", "landing.on-red=auto")
	b.policy(t, "auto")
	b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New("agent")
	}
	before = b.executions(t)
	if code, text := b.run(t, "landing", "prove", "--wait"); code == 0 || !strings.Contains(text, "LANE_PROOF_BUDGET") || b.executions(t) != before {
		t.Fatalf("classification reset spent budget: %d %s", code, text)
	}
	results, err := plain.Results(b.installation)
	helmMust(t, err)
	attempts := map[string]bool{}
	for _, result := range results {
		if result.CountedFull {
			attempts[result.Attempt] = true
		}
		if result.LoopClosed {
			t.Fatal("classification closed the budget loop")
		}
	}
	if len(attempts) != 2 {
		t.Fatalf("full history count=%d", len(attempts))
	}
}

func TestLandingReturnSameTipKeepsCurrentRequest(t *testing.T) {
	t.Parallel()
	b := newResolveVerbFixture(t)
	helmMust(t, os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("metasystem.template=true\n"), 0600))
	b.owners.resolver = stateroot.NewResolver(func(string) (string, error) { return b.root, nil }, os.Executable)
	b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New("agent")
	}
	machine := func(string) (string, error) { return "fixture", nil }
	b.owners.landing.machine = machine
	_, _, err := plain.HandIn(b.install, plain.Line{Goal: "goal", SHA: "same-sha"})
	helmMust(t, err)
	writeCauseProof(t, b.install, "results.jsonl", plain.Result{Result: plain.Red, Cause: &plain.Cause{Kind: "own", Goal: "goal", SHA: "same-sha", Tests: []string{"u/a TestA"}, Evidence: "own.log"}})
	if code, text := b.run(t, b.root, "return", "goal", "--cause", "own"); code != 0 {
		t.Fatalf("first return: %d %s", code, text)
	}
	_, _, err = plain.HandIn(b.install, plain.Line{Goal: "goal", SHA: "same-sha", Again: true})
	helmMust(t, err)
	if code, text := b.run(t, b.root, "return", "goal", "--cause", "own"); code == 0 || !strings.Contains(text, "known failures") {
		t.Fatalf("same tip reset history: %d %s", code, text)
	}
	helmMust(t, plain.SyncPolicyQuestion(b.install, machine, laneTestNow))
	open, unread := channel.WalkOpenQuestions(b.install)
	if len(open) != 1 || len(unread) != 0 || channel.LaneStopCommand(open[0]) != "metasystem landing return goal --cause own --reason TEXT" {
		t.Fatalf("older effect hid current request: %+v %v", open, unread)
	}
	unrelated := plain.Stop{Loop: "lane-proof", Subject: "main", Trunk: true, Scope: "full", Decision: "stop", Handoff: "ask lane", Required: []string{"metasystem", "landing", "prove", "--trunk"}, At: laneTestNow.Format(time.RFC3339)}
	raw, err := json.Marshal(unrelated)
	helmMust(t, err)
	file, err := os.OpenFile(filepath.Join(plain.Dir(b.install), "stops.jsonl"), os.O_APPEND|os.O_WRONLY, 0600)
	helmMust(t, err)
	_, err = file.Write(append(raw, '\n'))
	helmMust(t, err, file.Close())
	if code, text := b.run(t, b.root, "status", "--verbose"); code != 0 || !strings.Contains(text, "metasystem landing return goal --cause own --reason TEXT") {
		t.Fatalf("newer unrelated stop hid the current return: %d %s", code, text)
	}
	b.owners.prove = enrolledPersonProver(t, b.install, laneTestNow)
	if code, text := b.run(t, b.root, "return", "goal", "--cause", "main", "--reason", "the supplied cause wins", "--by", "Wido"); code != 0 {
		t.Fatalf("person current act: %d %s", code, text)
	}
	current, err := channel.ReadQuestion(b.install, open[0].ID)
	if err != nil || current.State != "closed" {
		t.Fatalf("current effect did not close: %+v %v", current, err)
	}
	remaining, err := plain.OpenStops(b.install)
	helmMust(t, err)
	if len(remaining) != 1 || remaining[0].Subject != "main" {
		t.Fatalf("return closed another subject: %+v", remaining)
	}
	entry, _, err := plain.Latest(b.install, "goal")
	if err != nil || entry.Cause.Kind != "main" || entry.SHA != "same-sha" || entry.ReturnOrigin.Person == nil {
		t.Fatalf("current supplied subject lost: %+v %v", entry, err)
	}
}

func TestLandingReturnClosesMatchingProofOrGateStop(t *testing.T) {
	t.Parallel()
	for _, loop := range []string{"lane-proof", "lane-gate"} {
		for _, kind := range []string{"own", "unclassified"} {
			t.Run(loop+"/"+kind, func(t *testing.T) {
				t.Parallel()
				b := newResolveVerbFixture(t)
				helmMust(t, os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("metasystem.template=true\n"), 0600))
				b.owners.resolver = stateroot.NewResolver(func(string) (string, error) { return b.root, nil }, os.Executable)
				b.owners.prove = enrolledPersonProver(t, b.install, laneTestNow)
				b.owners.landing.machine = func(string) (string, error) { return "fixture", nil }
				_, _, err := plain.HandIn(b.install, plain.Line{Goal: "goal", SHA: "current-sha"})
				helmMust(t, err)
				result := plain.Result{Result: plain.Red, Attempt: "current", Tree: "current-tree", Goals: []plain.GoalSHA{{Goal: "goal", SHA: "current-sha"}}}
				register := "results.jsonl"
				if loop == "lane-gate" {
					register = "gates.jsonl"
				}
				writeCauseProof(t, b.install, register, result)
				stop := plain.Stop{Loop: loop, Subject: "goal", Tree: result.Tree, ProofAttempt: result.Attempt, Decision: "stop", Handoff: "ask lane", At: laneTestNow.Format(time.RFC3339), Cause: &plain.Cause{Kind: kind, Goal: "goal"}}
				otherGoal, otherTree, proofAct, barren := stop, stop, stop, stop
				otherGoal.Subject, otherGoal.Cause = "other", &plain.Cause{Kind: kind, Goal: "other"}
				otherTree.Tree, otherTree.ProofAttempt = "other-tree", "other-attempt"
				proofAct.Required = []string{"metasystem", "landing", "prove"}
				barren.Loop, barren.Subject, barren.Required = "lane-return", "lane", []string{"metasystem", "landing", "run", "--goals", "goal"}
				// A separate proof request has a separate opening identity.
				proofAct.At = laneTestNow.Add(time.Second).Format(time.RFC3339)
				var data []byte
				for _, one := range []plain.Stop{stop, otherGoal, otherTree, proofAct, barren} {
					raw, err := json.Marshal(one)
					helmMust(t, err)
					data = append(data, append(raw, '\n')...)
				}
				helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.install), "stops.jsonl"), data, 0600))
				if code, text := b.run(t, b.root, "return", "goal", "--cause", kind, "--reason", "the person chose this return"); code != 0 {
					t.Fatalf("return: %d %s", code, text)
				}
				open, err := plain.OpenStops(b.install)
				if err != nil || len(open) != 4 {
					t.Fatalf("return did not isolate its goal and proof tree: %+v %v", open, err)
				}
				for _, one := range open {
					if one.Loop == loop && one.Subject == "goal" && one.Tree == "current-tree" && len(one.Required) == 0 {
						t.Fatalf("followed return left its proof stop open: %+v", one)
					}
				}
			})
		}
	}
}

func TestLandingPushClosesGoalProofStopAndPreservesOtherTip(t *testing.T) {
	t.Parallel()
	b := newBatchVerbBed(t, "auto")
	sha := b.seat(t, "a")
	b.success(t, "landing", "run")
	b.assemble(t, sha)
	b.success(t, "landing", "prove", "--wait")
	green, found, err := plain.LastResult(b.installation)
	if err != nil || !found || green.Result != plain.Green {
		t.Fatalf("missing current green: %+v %v", green, err)
	}
	red := green
	red.Attempt, red.Result, red.Cause = "earlier-red", plain.Red, &plain.Cause{Kind: "own", Goal: "a", SHA: sha}
	writeCauseProof(t, b.installation, "results.jsonl", red, green)
	stop := plain.Stop{Loop: "lane-proof", Subject: "a", Tree: red.Tree, ProofAttempt: red.Attempt, Decision: "stop", Handoff: "return a own", At: b.now.Format(time.RFC3339), Cause: red.Cause}
	raw, err := json.Marshal(stop)
	helmMust(t, err)
	helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.installation), "stops.jsonl"), append(raw, '\n'), 0600))
	other := plain.Stop{Loop: "lane-return", Subject: "a", Tree: "other-sha", Decision: "stop", Handoff: "ask lane", At: b.now.Format(time.RFC3339), Cause: &plain.Cause{Kind: "own", Goal: "a", SHA: "other-sha"}}
	raw, err = json.Marshal(other)
	helmMust(t, err)
	file, err := os.OpenFile(filepath.Join(plain.Dir(b.installation), "stops.jsonl"), os.O_APPEND|os.O_WRONLY, 0600)
	helmMust(t, err)
	_, err = file.Write(append(raw, '\n'))
	helmMust(t, err, file.Close())
	b.success(t, "landing", "push")
	open, err := plain.OpenStops(b.installation)
	if err != nil || len(open) != 1 || open[0].Loop != "lane-return" || open[0].Tree != "other-sha" {
		t.Fatalf("push did not close only its goal's proof stop: %+v %v", open, err)
	}
}

func TestLandingSourceConflictPersonHoldsWaitingEntry(t *testing.T) {
	t.Parallel()
	b := newResolveVerbFixture(t)
	helmMust(t, os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("metasystem.template=true\ntesting.contract=testing.json\nlanding.on-red=person\n"), 0600))
	b.owners.landing.machine = func(string) (string, error) { return "fixture", nil }
	_, _, err := plain.HandIn(b.install, plain.Line{Goal: "goal", SHA: "goal-sha"})
	helmMust(t, err)
	aborts := 0
	b.sourceConflictGit(t, func() { aborts++ })
	if code, text := b.run(t, b.root, "resolve", "--json"); code == 0 || !strings.Contains(text, "LANE_RETURN_PERSON") {
		t.Fatalf("source conflict escaped person policy: %d %s", code, text)
	}
	entry, _, err := plain.Latest(b.install, "goal")
	if err != nil || entry.State != plain.StateWaiting || !entry.Held || entry.Cause == nil || entry.Cause.Kind != "own" || entry.Cause.SHA != "goal-sha" || aborts != 1 {
		t.Fatalf("source conflict hold was not recorded: %+v %v aborts=%d", entry, err, aborts)
	}
	if code, text := b.run(t, b.root, "resolve", "--json"); code != 0 || aborts != 1 {
		t.Fatalf("held source conflict ran again: %d %s aborts=%d", code, text, aborts)
	}
}

func TestLandingRegenerationPersonStopsBeforeReplay(t *testing.T) {
	t.Parallel()
	b := newResolveVerbFixture(t)
	helmMust(t, os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("metasystem.template=true\ntesting.contract=testing.json\nlanding.on-red=person\n"), 0600))
	b.owners.landing.machine = func(string) (string, error) { return "fixture", nil }
	_, _, err := plain.HandIn(b.install, plain.Line{Goal: "goal", SHA: "goal-sha"})
	helmMust(t, err)
	runs := 0
	b.owners.landing.plainResolve = plain.ResolveSeams{
		Git: func(_ string, args ...string) (string, error) {
			switch strings.Join(args, " ") {
			case "show HEAD:metasystem/testing.json":
				return b.contract, nil
			case "diff --name-only --diff-filter=U -z", "ls-files -z", "ls-tree -r --name-only -z HEAD -- metasystem/out/result":
				return "metasystem/out/result\x00", nil
			case "rev-parse --verify HEAD^{commit}":
				return "main-sha", nil
			case "rev-parse --verify MERGE_HEAD^{commit}":
				return "goal-sha", nil
			case "diff --name-only -z AUTO_MERGE --", "ls-files --others --exclude-standard -z", "restore --source=HEAD --staged --worktree -- metasystem/out/result", "restore --source=AUTO_MERGE --worktree -- metasystem/out/result", "merge --abort":
				return "", nil
			default:
				t.Fatalf("red person started regeneration replay: %v", args)
				return "", nil
			}
		},
		Run: func(_ []string, _ string, _ *os.File, _ func(int64) error) error {
			runs++
			return exec.Command("/usr/bin/false").Run()
		},
	}
	code, text := b.run(t, b.root, "resolve", "--json")
	entry, _, err := plain.Latest(b.install, "goal")
	if code == 0 || err != nil || entry.State != plain.StateWaiting || !entry.Held || entry.Cause == nil || entry.Cause.Kind != "unclassified" || runs != 1 || !strings.Contains(text, "LANE_RETURN_PERSON") {
		t.Fatalf("red regeneration continued: %d %s %+v %v runs=%d", code, text, entry, err, runs)
	}
	stops, err := plain.OpenStops(b.install)
	helmMust(t, err)
	found := false
	for _, stop := range stops {
		if stop.Command() == "metasystem landing return goal --cause unclassified --reason TEXT" {
			found = true
		}
	}
	if !found {
		t.Fatalf("unclassified regeneration lacks actual return remedy: %+v", stops)
	}
	if code, text := b.run(t, b.root, "resolve", "--json"); code != 0 || runs != 1 {
		t.Fatalf("held regeneration ran again: %d %s runs=%d", code, text, runs)
	}
}

func TestLandingRedSavedBeforeAutomaticReplay(t *testing.T) {
	t.Parallel()
	b, _ := redPermissionBed(t)
	b.configuration = strings.ReplaceAll(b.configuration, "landing.on-red=person", "landing.on-red=auto")
	b.policy(t, "auto")
	helmMust(t, os.Remove(filepath.Join(plain.Dir(b.installation), "results.jsonl")), os.Remove(b.trace))
	observed := 0
	b.owners.landing.plainProve.Command = func(cmd *exec.Cmd) error {
		if commandEnv(cmd, "LANDING_ONLY") != "" {
			saved, ok, err := plain.LastResult(b.installation)
			if err != nil || !ok || saved.Result != plain.Red || !saved.ClassificationPending || !saved.CountedFull || len(saved.Failed) == 0 || len(saved.Executions) != 1 {
				t.Fatalf("automatic replay has no durable red: %+v %v", saved, err)
			}
			observed++
		}
		return cmd.Run()
	}
	if code, text := b.run(t, "landing", "prove", "--wait"); code == 0 {
		t.Fatalf("own red unexpectedly passed: %s", text)
	}
	if observed != 2 || b.executions(t) != 3 {
		t.Fatalf("automatic attribution execution count=%d observations=%d", b.executions(t), observed)
	}
	result, _, err := plain.LastResult(b.installation)
	if err != nil || result.ClassificationPending || result.Cause.Kind != "own" {
		t.Fatalf("automatic attribution: %+v %v", result, err)
	}
}

func TestLandingClassifyCannotRepeatIncompleteFull(t *testing.T) {
	t.Parallel()
	b, red := redPermissionBed(t)
	script := filepath.Join(filepath.Dir(b.checkout), "check")
	body := "#!/bin/sh\nprintf '%s|%s|%s\\n' \"$LANDING_COMMIT\" \"$LANDING_TREE\" \"$LANDING_PROOF_SCOPE\" >> " + shellCommand([]string{b.trace}) + "\nif [ -n \"$LANDING_ONLY\" ]; then printf 'LANDING-CHECKED\\t0\\n'; exit 0; fi\nprintf 'LANDING-NOT-RUN\\tdisk full\\n'; exit 2\n"
	helmMust(t, testexec.WriteFile(script, []byte(body), 0755))
	b.success(t, "landing", "prove", "--classify", red.Attempt)
	next, _, err := plain.LastResult(b.installation)
	if err != nil || next.ClassificationOf != red.Attempt || next.CountedFull || b.executions(t) != 4 {
		t.Fatalf("incomplete full admission: %+v %v count=%d", next, err, b.executions(t))
	}
	b.success(t, "landing", "prove", "--classify", red.Attempt)
	if b.executions(t) != 6 {
		t.Fatalf("restart repeated the spent full admission: count=%d", b.executions(t))
	}
}

// A killed full command has admission evidence but no result yet. Classification
// must recover that execution before replacing its slot or lending another one.
func TestLandingClassifyCannotRepeatLostFull(t *testing.T) {
	t.Parallel()
	b, red := redPermissionBed(t)
	path := filepath.Join(plain.Dir(b.installation), "results.jsonl")
	before, err := os.ReadFile(path)
	helmMust(t, err)
	script := filepath.Join(filepath.Dir(b.checkout), "check")
	body := "#!/bin/sh\nprintf '%s|%s|%s\\n' \"$LANDING_COMMIT\" \"$LANDING_TREE\" \"$LANDING_PROOF_SCOPE\" >> " + shellCommand([]string{b.trace}) + "\nif [ -n \"$LANDING_ONLY\" ]; then printf 'LANDING-CHECKED\\t0\\n'; exit 0; fi\nprintf 'LANDING-NOT-RUN\\tdisk full\\n'; exit 2\n"
	helmMust(t, testexec.WriteFile(script, []byte(body), 0755))
	var execution plain.Running
	b.owners.landing.plainProve.Command = func(cmd *exec.Cmd) error {
		if commandEnv(cmd, "LANDING_ONLY") == "" {
			var recorded bool
			execution, recorded, _, err = plain.ReadRunning(b.installation, b.owners.landing.plainProve)
			if err != nil || !recorded || execution.ClassificationOf != red.Attempt {
				t.Fatalf("next full has no pinned classification admission: %+v %v", execution, err)
			}
		}
		return cmd.Run()
	}
	b.success(t, "landing", "prove", "--classify", red.Attempt)
	// Reconstruct interruption after the command claimed admission but before
	// it could append its result; use the injected process observation.
	helmMust(t, os.WriteFile(path, before, 0600))
	raw, err := json.Marshal(execution)
	helmMust(t, err, os.WriteFile(filepath.Join(plain.Dir(b.installation), "running.json"), raw, 0600))
	b.owners.landing.plainProve.Alive = func(plain.Running) bool { return false }
	b.success(t, "landing", "prove", "--classify", red.Attempt)
	if b.executions(t) != 6 {
		t.Fatalf("restart repeated a lost full execution: count=%d", b.executions(t))
	}
	observations, err := plain.Results(b.installation)
	helmMust(t, err)
	recovered := false
	for _, observation := range observations {
		recovered = recovered || observation.Attempt == execution.Attempt && observation.ClassificationOf == red.Attempt && len(observation.Executions) == 1
	}
	if !recovered {
		t.Fatal("lost full admission vanished from retained history")
	}
}

func TestLandingClassifyHistoricalRedCannotAdmitReplacement(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"own", "all-green"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			b, red := redPermissionBed(t)
			replacement := b.seat(t, "replacement")
			b.git(t, b.checkout, "fetch", "--quiet", "origin", "goal/replacement")
			_, _, err := plain.HandIn(b.installation, plain.Line{Goal: "a", SHA: replacement})
			helmMust(t, err)
			b.git(t, b.checkout, "checkout", "--quiet", "--detach", replacement)
			b.policy(t, "auto")
			if mode == "all-green" {
				script := filepath.Join(filepath.Dir(b.checkout), "check")
				body := "#!/bin/sh\nprintf '%s|%s|%s\\n' \"$LANDING_COMMIT\" \"$LANDING_TREE\" \"$LANDING_PROOF_SCOPE\" >> " + shellCommand([]string{b.trace}) + "\nprintf 'LANDING-CHECKED\\t0\\n'\n"
				helmMust(t, testexec.WriteFile(script, []byte(body), 0755))
			}
			b.success(t, "landing", "prove", "--classify", red.Attempt)
			result, _, err := plain.LastResult(b.installation)
			if err != nil || result.Attempt != red.Attempt || result.Commit != red.Commit || result.Tree != red.Tree || result.ClassificationPerson == nil || b.executions(t) != 3 {
				t.Fatalf("historical red changed its subject or executed replacement: %+v %v count=%d", result, err, b.executions(t))
			}
			if mode == "own" && (result.Cause.Kind != "own" || result.Cause.SHA != red.Goals[0].SHA) {
				t.Fatalf("historical red was not attributed: %+v", result)
			}
			if mode == "all-green" && result.Repeat != "allowed" {
				t.Fatalf("historical attribution: %+v", result)
			}
			b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
				return humanauthority.Proof{}, errors.New("agent")
			}
			if code, text := b.run(t, "landing", "return", "a", "--cause", "own"); code == 0 {
				t.Fatalf("old attribution returned replacement: %s", text)
			}
			entry, _, err := plain.Latest(b.installation, "a")
			if err != nil || entry.State != plain.StateWaiting || entry.SHA != replacement || b.executions(t) != 3 {
				t.Fatalf("replacement changed: %+v %v", entry, err)
			}
		})
	}
}

func TestLandingClassifyRetainsSavedScopedAdmission(t *testing.T) {
	t.Parallel()
	b, saved := redPermissionBed(t)
	saved.Attempt += "-scoped"
	saved.Scope, saved.CountedFull = "scoped", false
	saved.Base = b.git(t, b.checkout, "rev-parse", b.main+"^{tree}")
	saved.Executions[0].Attempt = saved.Attempt
	saved.Executions[0].Scope, saved.Executions[0].Base = saved.Scope, saved.Base
	saved.Executions[0].Groups = []string{"records", "rules"}
	raw, err := json.Marshal(saved)
	helmMust(t, err)
	file, err := os.OpenFile(filepath.Join(plain.Dir(b.installation), "results.jsonl"), os.O_APPEND|os.O_WRONLY, 0600)
	helmMust(t, err)
	_, err = file.Write(append(raw, '\n'))
	helmMust(t, err, file.Close())
	events := flakeTestEvents([]string{"TestA"}, "fail")
	rawLog, err := os.ReadFile(saved.Log)
	helmMust(t, err, os.WriteFile(saved.Log, append([]byte(events), rawLog...), 0600))
	script := filepath.Join(filepath.Dir(b.checkout), "check")
	body := "#!/bin/sh\nprintf '%s|%s|%s\\n' \"$LANDING_COMMIT\" \"$LANDING_TREE\" \"$LANDING_PROOF_SCOPE\" >> " + shellCommand([]string{b.trace}) + "\nprintf 'LANDING-CHECKED\\t0\\n'\n"
	helmMust(t, testexec.WriteFile(script, []byte(body), 0755))
	b.owners.landing.plainProve.Judge = func(string, string, []plain.FailedUnit) (map[string]plain.UnitJudgement, error) {
		return map[string]plain.UnitJudgement{"u/a": {Known: true}}, nil
	}
	b.owners.landing.plainProve.RecordFlake = func(plain.FlakeRecord) (plain.FlakeRecorded, error) {
		return plain.FlakeRecorded{Goal: "recorded-flake", Seen: 1}, nil
	}
	b.owners.landing.plainProve.Command = func(cmd *exec.Cmd) error {
		if commandEnv(cmd, "LANDING_ONLY") != "u/a" || commandEnv(cmd, "LANDING_COMMIT") != saved.Commit || commandEnv(cmd, "LANDING_PROOF_SCOPE") != "scoped" || commandEnv(cmd, "LANDING_PROOF_BASE") != saved.Base || commandEnv(cmd, "LANDING_PROOF_GROUPS") != "records rules" {
			t.Fatalf("classification changed saved scope: unit=%q commit=%q scope=%q base=%q groups=%q", commandEnv(cmd, "LANDING_ONLY"), commandEnv(cmd, "LANDING_COMMIT"), commandEnv(cmd, "LANDING_PROOF_SCOPE"), commandEnv(cmd, "LANDING_PROOF_BASE"), commandEnv(cmd, "LANDING_PROOF_GROUPS"))
		}
		fmt.Fprint(cmd.Stdout, flakeTestEvents([]string{"TestA"}, "pass"))
		return cmd.Run()
	}
	b.success(t, "landing", "prove", "--classify", saved.Attempt)
	result, _, err := plain.LastResult(b.installation)
	if err != nil || result.Attempt != saved.Attempt || result.Scope != saved.Scope || result.Result != plain.Green || b.executions(t) != 2 {
		t.Fatalf("saved scoped attribution: %+v %v count=%d", result, err, b.executions(t))
	}
}

func TestLandingClassifyTrunkRefreshCannotAdmitReplacement(t *testing.T) {
	t.Parallel()
	b, _ := redPermissionBed(t)
	script := filepath.Join(filepath.Dir(b.checkout), "check")
	body := "#!/bin/sh\nprintf '%s|%s|%s\\n' \"$LANDING_COMMIT\" \"$LANDING_TREE\" \"$LANDING_PROOF_SCOPE\" >> " + shellCommand([]string{b.trace}) + "\nif [ -n \"$LANDING_ONLY\" ]; then printf 'LANDING-CHECKED\\t0\\n'; exit 0; fi\nprintf 'LANDING-FAILED\\tu/a\\tTestA\\nLANDING-CHECKED\\t1\\n'; exit 1\n"
	helmMust(t, testexec.WriteFile(script, []byte(body), 0755))
	if code, text := b.run(t, "landing", "prove", "--trunk", "--wait"); code == 0 || !strings.Contains(text, "LANE_RED_PERSON") {
		t.Fatalf("trunk red: %d %s", code, text)
	}
	red, _, err := plain.LastResult(b.installation)
	if err != nil || !red.Trunk || red.Commit != b.main || b.executions(t) != 2 {
		t.Fatalf("saved trunk subject: %+v %v", red, err)
	}
	replacement := b.seat(t, "main-next")
	b.git(t, filepath.Dir(b.checkout), "--git-dir", b.origin, "update-ref", "refs/heads/main", replacement)
	if ref := b.git(t, b.checkout, "rev-parse", "origin/main"); ref != red.Commit {
		t.Fatalf("fixture fetched the replacement too early: %s", ref)
	}
	b.success(t, "landing", "prove", "--classify", red.Attempt)
	result, _, err := plain.LastResult(b.installation)
	if err != nil || result.Attempt != red.Attempt || result.Commit != red.Commit || !result.Trunk || result.ClassificationPerson == nil || b.executions(t) != 3 {
		t.Fatalf("classification reused a stale main reference or admitted its replacement: %+v %v count=%d", result, err, b.executions(t))
	}
	if ref := b.git(t, b.checkout, "rev-parse", "origin/main"); ref != replacement {
		t.Fatalf("next-full boundary did not refresh main: %s", ref)
	}
}

func TestLandingClassifyRechecksBudgetAtFullAdmission(t *testing.T) {
	t.Parallel()
	b, red := redPermissionBed(t)
	script := filepath.Join(filepath.Dir(b.checkout), "check")
	body := "#!/bin/sh\nprintf '%s|%s|%s\\n' \"$LANDING_COMMIT\" \"$LANDING_TREE\" \"$LANDING_PROOF_SCOPE\" >> " + shellCommand([]string{b.trace}) + "\nif [ -n \"$LANDING_ONLY\" ]; then printf 'LANDING-CHECKED\\t0\\n'; exit 0; fi\nprintf 'LANDING-FAILED\\tu/a\\tTestA\\nLANDING-CHECKED\\t1\\n'; exit 1\n"
	helmMust(t, testexec.WriteFile(script, []byte(body), 0755))
	intervened := false
	b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
		if !intervened && strings.Join(args, " ") == "rev-parse --verify HEAD^{commit}" && b.executions(t) == 3 {
			_, recorded, _, err := plain.ReadRunning(b.installation, b.owners.landing.plainProve)
			helmMust(t, err)
			if !recorded {
				// Another completed full becomes durable after classification released
				// its slot, before its next full owns admission. No timing wait is needed.
				other := red
				other.Attempt, other.Repeat, other.ClassificationPending = "intervening-full", "allowed", false
				other.Executions[0].Attempt = other.Attempt
				raw, err := json.Marshal(other)
				helmMust(t, err)
				file, err := os.OpenFile(filepath.Join(plain.Dir(b.installation), "results.jsonl"), os.O_APPEND|os.O_WRONLY, 0600)
				helmMust(t, err)
				_, err = file.Write(append(raw, '\n'))
				helmMust(t, err, file.Close())
				intervened = true
			}
		}
		return plain.Git(dir, args...)
	}
	b.success(t, "landing", "prove", "--classify", red.Attempt)
	if !intervened || b.executions(t) != 3 {
		t.Fatalf("classification bypassed budget at fresh admission: intervened=%v count=%d", intervened, b.executions(t))
	}
	stops, err := plain.OpenStops(b.installation)
	helmMust(t, err)
	needed := false
	for _, stop := range stops {
		needed = needed || stop.Loop == "lane-proof" && stop.Command() == "metasystem landing prove"
	}
	if !needed {
		t.Fatalf("budget change lost the explicit next-check act: %+v", stops)
	}
}
