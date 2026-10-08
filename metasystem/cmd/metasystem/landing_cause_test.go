package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/designgate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func writeCauseProof(t *testing.T, install, file string, results ...plain.Result) {
	t.Helper()
	var data []byte
	for _, result := range results {
		line, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		data = append(append(data, line...), '\n')
	}
	if err := os.MkdirAll(plain.Dir(install), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plain.Dir(install), file), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLandingReturnRequiresDemonstratedOwnCause(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"own", "no cause", "invalid cause", "main", "other", "flake", "environment", "unclassified", "stale sha", "wrong goal", "request main", "no proof", "legacy red", "green", "newer main", "unrelated newest", "gate own", "gate main", "gate unreadable", "old gate", "person main", "person without proof", "unproven by"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newResolveVerbFixture(t)
			b.owners.landing.person = func(string) (string, error) { return "", errors.New("no enrolled person") }
			if _, _, err := plain.HandIn(b.install, plain.Line{Goal: "goal", SHA: "waiting"}); err != nil {
				t.Fatal(err)
			}
			cause := &plain.Cause{Kind: "own", Goal: "goal", SHA: "waiting", Tests: []string{"internal/steward TestTick"}, Evidence: "own.log"}
			proof := plain.Result{Result: plain.Red, At: "2026-10-06T10:00:00Z", Cause: cause, Goals: []plain.GoalSHA{{Goal: "goal", SHA: "waiting"}}}
			kind := "own"
			allowed := name == "own" || name == "unrelated newest" || name == "gate own" || name == "old gate" || strings.HasPrefix(name, "person")
			switch name {
			case "no cause":
				kind = ""
			case "invalid cause":
				kind = "guess"
			case "main", "other", "flake", "environment", "unclassified", "person main":
				cause.Kind = strings.TrimPrefix(name, "person ")
			case "stale sha":
				cause.SHA = "old"
			case "wrong goal":
				cause.Goal = "batch-mate"
			case "request main":
				kind = "main"
			case "legacy red":
				proof.Cause = nil
			case "green":
				proof.Result = plain.Green
			}
			writeCauseProof(t, b.install, "results.jsonl", proof)
			if name == "no proof" {
				if err := os.Remove(filepath.Join(plain.Dir(b.install), "results.jsonl")); err != nil {
					t.Fatal(err)
				}
			}
			if name == "newer main" || name == "unrelated newest" {
				next := proof
				next.At, next.Cause = "2026-10-06T10:01:00Z", &plain.Cause{Kind: "main"}
				if name == "unrelated newest" {
					next.Goals = []plain.GoalSHA{{Goal: "unrelated", SHA: "else"}}
				}
				writeCauseProof(t, b.install, "results.jsonl", proof, next)
			}
			if strings.Contains(name, "gate") && name != "gate unreadable" {
				gate := proof
				gate.At = "2026-10-06T10:01:00Z"
				if name != "gate own" {
					gate.Cause = &plain.Cause{Kind: "main"}
				}
				if name == "gate own" {
					writeCauseProof(t, b.install, "results.jsonl", plain.Result{Result: plain.Red, At: proof.At, Goals: proof.Goals, Cause: &plain.Cause{Kind: "main"}})
				}
				if name == "old gate" {
					gate.At = "2026-10-06T09:59:00Z"
				}
				writeCauseProof(t, b.install, "gates.jsonl", gate)
			}
			if name == "gate unreadable" {
				if err := os.Mkdir(filepath.Join(plain.Dir(b.install), "gates.jsonl"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if name == "person main" {
				kind = "main"
			}
			args := []string{"return", "goal"}
			if kind != "" {
				args = append(args, "--cause", kind)
			}
			if strings.HasPrefix(name, "person") || name == "unproven by" {
				args = append(args, "--by", "Wido")
				if name != "unproven by" {
					b.owners.resolver = stateroot.NewResolver(func(string) (string, error) { return b.root, nil }, os.Executable)
					b.owners.prove = enrolledPersonProver(t, b.install, laneTestNow)
					helmMust(t, os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("metasystem.template=true\ntesting.contract=testing.json\n"), 0600))
				}
				if name == "person main" {
					args = append(args, "--reason", "human decision")
				}
				if name == "person without proof" {
					if err := os.Remove(filepath.Join(plain.Dir(b.install), "results.jsonl")); err != nil {
						t.Fatal(err)
					}
				}
			}
			code, output := b.run(t, b.root, args...)
			entry, _, err := plain.Latest(b.install, "goal")
			if err != nil {
				t.Fatal(err)
			}
			if !allowed {
				if code == 0 || entry.State != plain.StateWaiting {
					t.Fatalf("unsafe return accepted: %d %s %+v", code, output, entry)
				}
				if name == "main" && !strings.Contains(output, "goal goal was not returned: the lane's last check found main itself is red (internal/steward TestTick)") {
					t.Fatalf("main refusal: %s", output)
				}
				return
			}
			if code != 0 || entry.State != plain.StateReturned || entry.Cause == nil || entry.Cause.Kind != kind {
				t.Fatalf("return: %d %s %+v", code, output, entry)
			}
			if !strings.HasPrefix(name, "person") && (!reflect.DeepEqual(entry.Cause, cause) || !strings.Contains(entry.Reason, "internal/steward TestTick") || !strings.Contains(entry.Reason, "own.log")) {
				t.Fatalf("proof lost: %+v", entry)
			}
			if name == "person main" && entry.Reason != "human decision" {
				t.Fatalf("reason lost: %+v", entry)
			}
			before := idemTreeDigest(t, plain.Dir(b.install))
			if code, output := b.run(t, b.root, args...); code != 0 {
				t.Fatalf("repeat: %d %s", code, output)
			}
			idemSameTree(t, "repeat return", before, idemTreeDigest(t, plain.Dir(b.install)))
		})
	}
}

func TestLandingReturnWritesReturnedCard(t *testing.T) {
	t.Parallel()
	b := newResolveVerbFixture(t)
	registry := t.TempDir()
	home := filepath.Join(registry, ".metasystem")
	b.owners.lookupEnv = func(key string) (string, bool) { return registry, key == "METASYSTEM_SUPERVISION_REGISTRY_HOME" }
	card := board.Card{Seat: board.Seat{Machine: "seat", Installation: "/seat/metasystem"}, Goal: "goal", Stage: board.StageJoined, Batch: "batch", Landed: 2, Writer: board.Writer{At: time.Now()}}
	if err := board.WriteAt(home, card); err != nil {
		t.Fatal(err)
	}
	if _, _, err := plain.HandIn(b.install, plain.Line{Goal: "goal", SHA: "waiting"}); err != nil {
		t.Fatal(err)
	}
	writeCauseProof(t, b.install, "results.jsonl", plain.Result{Result: plain.Red, Cause: &plain.Cause{Kind: "own", Goal: "goal", SHA: "waiting"}})
	if code, output := b.run(t, b.root, "return", "goal", "--cause", "own"); code != 0 {
		t.Fatalf("return: %d %s", code, output)
	}
	current, ok := board.LiveOrReturnedCard(home, "goal")
	if !ok || current.Stage != board.StageReturned || current.Landed != 2 || current.Batch != "" || current.Owner != nil || current.Job != nil || current.Proof != nil {
		t.Fatalf("card not returned: %+v", current)
	}
}

func TestLandingProveRecordsCauseAndGoalCommits(t *testing.T) {
	t.Parallel()
	b := newResolveVerbFixture(t)
	if err := os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("proof.full=printf 'LANDING-FAILED\\tu/a\\tTestA\\nLANDING-CHECKED\\t1\\n'; exit 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := plain.HandIn(b.install, plain.Line{Goal: "goal", SHA: "waiting"}); err != nil {
		t.Fatal(err)
	}
	falseState := replayFalseState(t)
	b.owners.landing.plainProve = plain.ProveSeams{Now: func() time.Time { return laneTestNow }, Git: func(dir string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "rev-parse --verify HEAD^{commit}":
			return "head", nil
		case "rev-parse --verify HEAD^{tree}":
			return "tree", nil
		case "show head:metasystem/metasystem.conf":
			return "proof.full=printf 'LANDING-FAILED\\tu/a\\tTestA\\nLANDING-CHECKED\\t1\\n'; exit 1\n", nil
		case "fetch --quiet origin +refs/heads/main:refs/remotes/origin/main", "cat-file -e main^{commit}", "cat-file -e head^{commit}", "cat-file -e waiting^{commit}", "merge-base --is-ancestor waiting head", "merge-base --is-ancestor main head":
			return "", nil
		case "rev-parse --verify --quiet refs/remotes/origin/main^{commit}":
			return "main", nil
		case "rev-list --first-parent --reverse --parents main..head":
			return "head main waiting", nil
		case "merge-base --is-ancestor waiting main", "merge-base --is-ancestor head main":
			return "", &exec.ExitError{ProcessState: falseState}
		case "ls-tree --name-only main -- metasystem/plans/goals/trunk-red.json":
			return "", nil
		case "show origin/main:metasystem/testing.json", "show origin/main:metasystem/plans/goals/trunk-red.json":
			return "", errors.New("not declared")
		case "worktree prune":
			return "", nil
		}
		if len(args) == 5 && args[0] == "worktree" && args[1] == "add" {
			return "", os.MkdirAll(filepath.Join(args[3], "metasystem"), 0o755)
		}
		if len(args) == 4 && args[0] == "worktree" && args[1] == "remove" {
			return "", os.RemoveAll(args[3])
		}
		return "", fmt.Errorf("unexpected stub Git: %v", args)
	}}
	b.prepareBatch(t)
	code, output := b.run(t, b.root, "prove", "--wait", "--json")
	var result struct {
		Data plain.Result `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("%d %s: %v", code, output, err)
	}
	proof := result.Data
	if code != 1 || proof.Cause == nil || proof.Cause.Kind != "unclassified" || proof.Cause.Evidence != proof.Log || !reflect.DeepEqual(proof.Cause.Tests, []string{"u/a TestA"}) || !reflect.DeepEqual(proof.Goals, []plain.GoalSHA{{Goal: "goal", SHA: "waiting"}}) {
		t.Fatalf("proof omitted cause or hand-in: %d %+v\n%s", code, proof, output)
	}
	stored, _, err := plain.LastResult(b.install)
	if err != nil || !reflect.DeepEqual(stored, proof) {
		t.Fatalf("record differs: %+v %v", stored, err)
	}
}

func TestLandingStatusReadsCauseAndNamesOtherGoal(t *testing.T) {
	t.Parallel()
	b := newResolveVerbFixture(t)
	goals := []plain.GoalSHA{{Goal: "culprit", SHA: "culprit-sha"}, {Goal: "mate", SHA: "mate-sha"}}
	for _, goal := range goals {
		if _, _, err := plain.HandIn(b.install, plain.Line{Goal: goal.Goal, SHA: goal.SHA}); err != nil {
			t.Fatal(err)
		}
	}
	proof := plain.Result{Result: plain.Red, At: laneTestNow.Format(time.RFC3339), Goals: goals, Cause: &plain.Cause{Kind: "own", Goal: "culprit", SHA: "culprit-sha"}}
	writeCauseProof(t, b.install, "results.jsonl", proof)
	b.owners.landing.plainProve.Git = func(_ string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "rev-parse" {
			return "main", nil
		}
		if len(args) > 0 && (args[0] == "cat-file" || args[0] == "merge-base") {
			return "", nil
		}
		return "", fmt.Errorf("unexpected stub Git: %v", args)
	}
	if code, output := b.run(t, b.root, "status", "--verbose"); code != 0 || !strings.Contains(output, "cause: other culprit") {
		t.Fatalf("batch mate's cause: %d %s", code, output)
	}
	code, output := b.run(t, b.root, "status", "--json")
	var result struct {
		Data plain.Status `json:"data"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil || code != 0 || result.Data.LastProof == nil || !reflect.DeepEqual(result.Data.LastProof.Cause, proof.Cause) || !reflect.DeepEqual(result.Data.LastProof.Goals, goals) {
		t.Fatalf("status lost cause or commits: %d %s %v", code, output, err)
	}
}

func TestLandingReturnPersonWithoutBy(t *testing.T) {
	t.Parallel()
	b := newResolveVerbFixture(t)
	b.owners.resolver = stateroot.NewResolver(func(string) (string, error) { return b.root, nil }, os.Executable)
	b.owners.prove = enrolledPersonProver(t, b.install, laneTestNow)
	helmMust(t, os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("metasystem.template=true\ntesting.contract=testing.json\n"), 0600))
	if _, _, err := plain.HandIn(b.install, plain.Line{Goal: "goal", SHA: "waiting"}); err != nil {
		t.Fatal(err)
	}
	code, output := b.run(t, b.root, "return", "goal", "--cause", "main", "--reason", "main needs repair")
	entry, ok, err := plain.Latest(b.install, "goal")
	if code != 0 || err != nil || !ok || entry.State != plain.StateReturned || entry.Cause == nil || entry.Cause.Kind != "main" || entry.Cause.Name != "Wido" || entry.Reason != "main needs repair" {
		t.Fatalf("person's return without --by: code=%d output=%s entry=%+v ok=%v err=%v", code, output, entry, ok, err)
	}
	data, err := os.ReadFile(filepath.Join(plain.Dir(b.install), "queue.jsonl"))
	if err != nil || !strings.Contains(string(data), `"name":"Wido"`) {
		t.Fatalf("person's name absent from return line: %s err=%v", data, err)
	}
}

func TestLandingPushReturnedCommitBoundaries(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"still in head", "already on main", "removed from head", "fix-forward hand-in", "same commit handed in again"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newResolveVerbFixture(t)
			// Push reads fresh main and incident inputs before checking returns (lane-reads-its-policies.md:145).
			notAncestor := replayFalseState(t)
			b.owners.landing.plainProve.Git = func(_ string, args ...string) (string, error) {
				if args[0] == "rev-parse" {
					return "old", nil
				}
				if args[0] == "merge-base" {
					return "", &exec.ExitError{ProcessState: notAncestor}
				}
				if args[0] == "cat-file" {
					return "", nil
				}
				if args[0] == "ls-tree" {
					return "", nil
				}
				t.Fatalf("unexpected Git: %v", args)
				return "", nil
			}
			if _, _, err := plain.HandIn(b.install, plain.Line{Goal: "goal", SHA: "returned-sha"}); err != nil {
				t.Fatal(err)
			}
			if _, _, err := plain.ReturnDesignRefused(b.install, plain.DesignCheck{Goal: "goal", Commit: "returned-sha", Reason: "the design changed"}, laneTestNow); err != nil {
				t.Fatal(err)
			}
			retrySHA := ""
			switch name {
			case "fix-forward hand-in":
				retrySHA = "fix-forward-sha"
			case "same commit handed in again":
				retrySHA = "returned-sha"
			}
			if retrySHA != "" {
				entry, added, err := plain.HandIn(b.install, plain.Line{Goal: "goal", SHA: retrySHA, Again: retrySHA == "returned-sha"})
				if err != nil || !added || entry.State != plain.StateWaiting {
					t.Fatalf("retry not queued: entry=%+v added=%v err=%v", entry, added, err)
				}
			}
			pushes := 0
			b.owners.landing.push = func(_, _ string, _ time.Time, before func(string, string) error) (plain.PushOutcome, error) {
				outcome := plain.PushOutcome{Old: "old", Commit: "head"}
				if err := before(outcome.Old, outcome.Commit); err != nil {
					return outcome, err
				}
				pushes++
				outcome.Changed = true
				return outcome, nil
			}
			b.owners.landing.contained = func(checkout, ref string) func(string) (bool, error) {
				if checkout != b.root || (ref != "head" && ref != "old") {
					t.Fatalf("containment checked %s at %s", checkout, ref)
				}
				return func(sha string) (bool, error) {
					if sha != "returned-sha" && (retrySHA == "" || sha != retrySHA) {
						t.Fatalf("checked unexpected commit %s", sha)
					}
					return name == "already on main" || (name == "still in head" || retrySHA != "") && ref == "head", nil
				}
			}
			command, ok := findIntentAction("landing", "push")
			if !ok {
				t.Fatal("landing push is not discoverable")
			}
			command = laneCommand(command, func(inv *intentInvocation, admitted laneAdmitted) int {
				return runIntentLandingPushWithOwners(inv, admitted, landingPushOwners{
					facts: func(id string) landing.DesignFacts {
						if id != "goal" {
							t.Fatalf("checked unexpected goal %s", id)
						}
						return landing.DesignFacts{Facts: designgate.Facts{Goal: id, Tier: 1}}
					},
					notify: func(plain.PushOutcome) error { return nil },
				})
			})
			var stdout, stderr bytes.Buffer
			code := runIntentIn(command, []string{"--json"}, &stdout, &stderr, b.root, b.owners)
			output := stdout.String()
			var result intentResult
			if err := json.Unmarshal([]byte(output), &result); err != nil {
				t.Fatal(err)
			}
			if name == "still in head" {
				if code != 1 || pushes != 0 || result.Outcome != intentRefused || !strings.Contains(result.Summary, "returned goal goal") {
					t.Fatalf("returned commit escaped: %d %s pushes=%d", code, output, pushes)
				}
			} else if code != 0 || pushes != 1 || result.Outcome != intentConfirmed {
				t.Fatalf("safe HEAD refused: %d %s pushes=%d", code, output, pushes)
			}
		})
	}
}
