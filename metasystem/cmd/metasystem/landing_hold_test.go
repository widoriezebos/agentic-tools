package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

const holdIncidentID = "tr-fast-hold000001"

func holdIncidentFixture(t *testing.T, b *intentBed) []goal.TrunkRedEntry {
	t.Helper()
	seedIncident(t, b, holdIncidentID)
	entries, problems := goal.ParseTrunkRed(b.repo.commit(b.repo.accepted).files["plans/goals/trunk-red.json"])
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	return entries
}

func TestWorkLandTrunkRedRefusesUntilIncidentClaim(t *testing.T) {
	t.Parallel()
	b, _, install := plainLaneBedWith(t, true, "critic-root", "critic-root")
	holdIncidentFixture(t, b.intentBed)
	code, result := b.do("work", "land", bedGoal)
	expectOutcome(t, "main red", code, result, intentRefused)
	if !strings.Contains(result.Summary, holdIncidentID) || !strings.Contains(result.Summary, "Nothing was handed in") || resultData(t, result)["code"] != goal.LandTrunkRedCode || result.Next == nil || strings.Join(result.Next.Argv, " ") != "metasystem incident list" {
		t.Fatalf("incident refusal: %+v", result)
	}
	if entries, err := plain.Entries(install); err != nil || len(entries) != 0 {
		t.Fatalf("refusal queued: %+v %v", entries, err)
	}
	code, result = b.do("incident", "claim", holdIncidentID, "--goal", bedGoal, "--by", "Wido")
	expectOutcome(t, "claim fix", code, result, intentConfirmed)
	code, result = b.do("work", "land", bedGoal)
	expectOutcome(t, "fix hand-in", code, result, intentConfirmed)
	entries, err := plain.Entries(install)
	if err != nil || len(entries) != 1 || entries[0].Exception != nil {
		t.Fatalf("fix hand-in: %+v %v", entries, err)
	}
}

func TestExceptionWorkLandTrunkRedBelongsToOneQueueLine(t *testing.T) {
	t.Parallel()
	b, state, install := plainLaneBedWith(t, true, "critic-root", "critic-root")
	holdIncidentFixture(t, b.intentBed)
	args := []string{"work", "land", bedGoal, "--exception", goal.LandTrunkRedCode, "--reason", "Ship the independent fix", "--by", "Wido"}
	code, result := b.do(args...)
	expectOutcome(t, "exception hand-in", code, result, intentConfirmed)
	entries, err := plain.Entries(install)
	if err != nil || len(entries) != 1 || entries[0].Exception == nil || *entries[0].Exception != (plain.Exception{Code: goal.LandTrunkRedCode, Reason: "Ship the independent fix", By: "Wido"}) || len(state.pushes) != 0 || state.candidates != 0 {
		t.Fatalf("exception did not hand in: %+v %v %+v", entries, err, result)
	}
	queue := filepath.Join(plain.Dir(install), "queue.jsonl")
	before, err := os.ReadFile(queue)
	if err != nil {
		t.Fatal(err)
	}
	code, result = b.do(args...)
	expectOutcome(t, "repeat exception", code, result, intentUnchanged)
	after, err := os.ReadFile(queue)
	if err != nil || string(before) != string(after) {
		t.Fatalf("repeat wrote queue: %v", err)
	}
	if _, _, err := plain.Return(install, bedGoal, "own failure", time.Now()); err != nil {
		t.Fatal(err)
	}
	code, result = b.do("work", "land", bedGoal, "--again")
	expectOutcome(t, "returned exception expired", code, result, intentRefused)
	if !strings.Contains(result.Summary, holdIncidentID) {
		t.Fatalf("return kept exception: %+v", result)
	}
	code, result = b.do(append(args, "--again")...)
	expectOutcome(t, "new exception", code, result, intentConfirmed)
	state.status.BranchTip = strings.Repeat("3", 40)
	code, result = b.do("work", "land", bedGoal)
	expectOutcome(t, "new tip has no exception", code, result, intentRefused)
	projection, err := goal.Project(goal.Endpoint{Root: b.root(), Remote: "local", Branch: goal.LocalLedgerBranch, Repository: b.repo}, false, syncRequestTestNow)
	if err != nil || len(projection.Tree.TrunkRed) != 1 || projection.Tree.TrunkRed[0].Closed != nil {
		t.Fatalf("exception closed incident: %+v %v", projection.Tree.TrunkRed, err)
	}
}

// holdLaneFixture drives status and push through their public commands. Main's
// register and ancestry are per-test Git facts; no real Git is used.
func holdLaneFixture(t *testing.T, entries []goal.TrunkRedEntry) (*resolveVerbFixture, *int) {
	t.Helper()
	b := newResolveVerbFixture(t)
	b.owners.landing.contained = func(_ string, main string) func(string) (bool, error) {
		return func(sha string) (bool, error) { return main == "head" && sha == "sha-goal", nil }
	}
	b.owners.landing.plainProve.Git = func(_ string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "fetch --quiet origin +refs/heads/main:refs/remotes/origin/main":
			return "", nil
		case "rev-parse --verify --quiet refs/remotes/origin/main^{commit}":
			return "main", nil
		case "ls-tree --name-only main -- metasystem/plans/goals/trunk-red.json":
			return "metasystem/plans/goals/trunk-red.json", nil
		case "show main:metasystem/plans/goals/trunk-red.json":
			return string(goal.RenderTrunkRed(entries)), nil
		}
		if args[0] == "cat-file" {
			return "", os.ErrNotExist
		}
		t.Fatalf("unstubbed Git: %v", args)
		return "", nil
	}
	b.owners.landing.wake = func(string) lane.WakeSources {
		return lane.WakeSources{Reasons: func(string) ([]string, error) {
			return plain.WakeReasons(b.install, b.root, time.Time{}, laneTestNow, b.owners.landing.plainProve)
		}}
	}
	pushes := 0
	b.owners.landing.push = func(_, _ string, _ time.Time, before func(string, string) error) (plain.PushOutcome, error) {
		out := plain.PushOutcome{Old: "main", Commit: "head"}
		if err := before(out.Old, out.Commit); err != nil {
			return out, err
		}
		pushes++
		return out, nil
	}
	return b, &pushes
}

func TestLandingStatusTrunkRedHoldsAndWake(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"red", "fix", "closed", "flake", "hang", "exception", "conflict", "bad register"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			register := holdIncidentFixture(t, newIntentBed(t, false, nil))
			switch name {
			case "fix":
				register[0].FixGoal = "goal"
			case "closed":
				register[0].Closed = &goal.TrunkRedClosure{At: register[0].Opened, How: "hand", By: "Wido", Why: "fixed", Opid: register[0].Sightings[0].Opid}
			case "flake":
				register[0].Class = goal.TrunkRedClassPendingFlake
			case "hang":
				register[0].Class = goal.TrunkRedClassHang
			}
			b, _ := holdLaneFixture(t, register)
			line := plain.Line{Goal: "goal", SHA: "sha-goal"}
			if name == "exception" {
				line.Exception = &plain.Exception{Code: goal.LandTrunkRedCode, By: "Wido", Reason: "Ship"}
			}
			if _, _, err := plain.HandIn(b.install, line); err != nil {
				t.Fatal(err)
			}
			if name == "conflict" {
				writeHoldLines(t, b.install, line, plain.Line{Goal: "before", SHA: "sha-before"}, plain.Line{Goal: "goal", SHA: line.SHA, Outcome: plain.StateWaiting, After: []plain.GoalSHA{{Goal: "before", SHA: "sha-before"}}, Held: true, Reason: "waits for before"})
			}
			if name == "bad register" {
				git := b.owners.landing.plainProve.Git
				b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
					if args[0] == "show" {
						return "bad JSON", nil
					}
					return git(dir, args...)
				}
			}
			code, text := b.run(t, b.root, "status", "--json")
			var result struct{ Data plain.Status }
			if code != 0 || json.Unmarshal([]byte(text), &result) != nil {
				t.Fatalf("status: %d %s", code, text)
			}
			if name == "bad register" {
				if len(result.Data.Problems) == 0 || result.Data.Wake == nil || len(result.Data.Wake.Unread) == 0 || slices.Contains(result.Data.Wake.Reasons, plain.WakeQueued) {
					t.Fatalf("unreadable register wakes: %+v", result.Data)
				}
				return
			}
			if len(result.Data.Problems) != 0 || result.Data.Wake != nil && len(result.Data.Wake.Unread) != 0 {
				t.Fatalf("healthy register unreadable: %s", text)
			}
			held := name == "red" || name == "conflict"
			if len(result.Data.Queue) == 0 || result.Data.Queue[0].Held != held || result.Data.Wake == nil || slices.Contains(result.Data.Wake.Reasons, plain.WakeQueued) == held {
				t.Fatalf("status holds/wake: %s", text)
			}
			if held && !strings.Contains(result.Data.Queue[0].Reason, holdIncidentID) {
				t.Fatalf("hold lacks incident: %s", text)
			}
			if held {
				code, words := b.run(t, b.root, "status")
				if code != 0 || !strings.Contains(oneSpaced(words), "waiting (held)") || !strings.Contains(words, holdIncidentID) {
					t.Fatalf("held line invisible: %d %s", code, words)
				}
			}
		})
	}
}

func TestLandingPushRefusesHeldGoal(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"red", "exception", "fix", "conflict", "records after code", "read error", "not in HEAD", "already on main"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			register := holdIncidentFixture(t, newIntentBed(t, false, nil))
			if name == "fix" || name == "conflict" {
				register[0].FixGoal = "goal"
			}
			b, pushes := holdLaneFixture(t, register)
			line := plain.Line{Goal: "goal", SHA: "sha-goal"}
			if name == "exception" {
				line.Exception = &plain.Exception{Code: goal.LandTrunkRedCode, By: "Wido", Reason: "Ship"}
			}
			if _, _, err := plain.HandIn(b.install, line); err != nil {
				t.Fatal(err)
			}
			if name == "records after code" {
				if _, _, err := plain.HandIn(b.install, plain.Line{Goal: line.Goal, SHA: "records", Records: true, Exception: &plain.Exception{Code: goal.LandTrunkRedCode, By: "Wido", Reason: "Publish records"}}); err != nil {
					t.Fatal(err)
				}
			}
			if name == "conflict" {
				writeHoldLines(t, b.install, line, plain.Line{Goal: "before", SHA: "sha-before"}, plain.Line{Goal: "goal", SHA: line.SHA, Outcome: plain.StateWaiting, After: []plain.GoalSHA{{Goal: "before", SHA: "sha-before"}}, Held: true, Reason: "waits for before"})
			}
			if name == "read error" {
				b.owners.landing.plainProve.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) {
					return nil, errors.New("main register unavailable")
				}
			}
			if name == "not in HEAD" || name == "already on main" {
				b.owners.landing.contained = func(_ string, main string) func(string) (bool, error) {
					return func(sha string) (bool, error) { return name == "already on main" && sha == line.SHA, nil }
				}
			}
			code, text := b.run(t, b.root, "push", "--json")
			refused := name == "red" || name == "conflict" || name == "records after code" || name == "read error"
			if (code != 0) != refused || *pushes != map[bool]int{true: 0, false: 1}[refused] {
				t.Fatalf("push crossed hold: %s: %d pushes=%d %s", name, code, *pushes, text)
			}
			if name == "red" || name == "conflict" {
				var result intentResult
				if json.Unmarshal([]byte(text), &result) != nil || !strings.Contains(result.Summary, "held goal goal") || result.Next == nil || !strings.Contains(strings.Join(result.Next.Argv, " "), "checkout --detach origin/main") {
					t.Fatalf("hold remedy: %s", text)
				}
			}
		})
	}
}

func TestExceptionWorkLandTrunkRedRequiresPerson(t *testing.T) {
	t.Parallel()
	b, _, install := plainLaneBedWith(t, true, "critic-root", "critic-root")
	b.lineage = "m1"
	owners := b.intentBed.owners()
	owners.delivery, owners.connection, owners.work = b.owners, b.connection, b.work
	owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, fmt.Errorf("agent has no person's terminal")
	}
	code, result := b.runJSON(owners, "work", "land", bedGoal, "--exception", goal.LandTrunkRedCode, "--reason", "Ship", "--by", "Wido")
	expectOutcome(t, "agent exception", code, result, intentRefused)
	if entries, err := plain.Entries(install); err != nil || len(entries) != 0 {
		t.Fatalf("agent wrote exception: %+v %v", entries, err)
	}
}

func TestExceptionWorkLandTrunkRedAdmitsWaitingLine(t *testing.T) {
	t.Parallel()
	b, _, install := plainLaneBedWith(t, true, "critic-root", "critic-root")
	code, result := b.do("work", "land", bedGoal)
	expectOutcome(t, "hand-in before main red", code, result, intentConfirmed)
	holdIncidentFixture(t, b.intentBed)
	args := []string{"work", "land", bedGoal, "--exception", goal.LandTrunkRedCode, "--reason", "Ship this line", "--by", "Wido"}
	code, result = b.do("work", "land", bedGoal)
	expectOutcome(t, "waiting line held by main red", code, result, intentRefused)
	code, result = b.do(args...)
	expectOutcome(t, "exception on waiting line", code, result, intentUnchanged)
	entries, err := plain.Entries(install)
	if err != nil || len(entries) != 1 || entries[0].Exception == nil {
		t.Fatalf("waiting line lost exception: %+v %v", entries, err)
	}
	queue := filepath.Join(plain.Dir(install), "queue.jsonl")
	before, err := os.ReadFile(queue)
	if err != nil {
		t.Fatal(err)
	}
	code, result = b.do(args...)
	expectOutcome(t, "repeat waiting exception", code, result, intentUnchanged)
	after, err := os.ReadFile(queue)
	if err != nil || string(before) != string(after) {
		t.Fatalf("repeat waiting exception wrote: %v", err)
	}
}

func writeHoldLines(t *testing.T, install string, lines ...plain.Line) {
	t.Helper()
	var data []byte
	for _, line := range lines {
		encoded, err := json.Marshal(line)
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, append(encoded, '\n')...)
	}
	if err := os.WriteFile(filepath.Join(plain.Dir(install), "queue.jsonl"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}
