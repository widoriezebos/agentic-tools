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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

func TestLandingClockReachesStatusAndTheGoal(t *testing.T) {
	t.Parallel()
	seat, state, _ := plainLaneBedWith(t, true, "critic-root", "critic-root")
	seat.lineage = "m1"
	lane := newPlainVerbBed(t)
	lane.cwd = lane.checkout
	seat.owners.laneInstall = func(string) (string, error) { return lane.installation, nil }
	seat.owners.laneLatest = nil
	seat.owners.recordLanded = nil
	start := laneTestNow
	now := start
	seat.owners.now = func() time.Time { return now }
	lane.owners.landing.now = func() time.Time { return now }
	lane.owners.landing.plainProve.Now = func() time.Time { return now }
	lane.owners.landing.plainProve.Closure = func(string, string, string) (adapter.Closure, error) {
		return adapter.Closure{Unowned: []string{"metasystem/plans/note.md"}}, nil
	}
	// The contract covers the record-only fixed branch's remaining change.
	contract := testpolicy.Contract{SchemaVersion: testpolicy.ExecutionContractSchemaVersion,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces:    []testpolicy.Surface{{ID: "notes", Paths: []string{"metasystem/plans/**"}, Standard: []string{"notes"}}}, Unknown: []string{"notes"},
		Groups: []testpolicy.Group{{ID: "notes", Kind: "unit", Adapter: "command", CWD: ".", Phase: "acceptance", EnvironmentMode: "inherit", Inputs: []string{"metasystem/plans/**"}, Platforms: []string{"any"}, TargetMS: 1, Argv: []string{"fixture-tests"}, Format: "exit-status"}}}
	data, err := json.Marshal(contract)
	helmMust(t, err)
	helmMust(t, os.WriteFile(filepath.Join(lane.installation, "testing.json"), data, 0644))
	helmMust(t, os.WriteFile(filepath.Join(lane.installation, "metasystem.conf"), []byte("metasystem.template=true\ntesting.contract=testing.json\nproof.full=fixture\n"), 0644))
	lane.git(t, lane.checkout, "add", "metasystem/metasystem.conf", "metasystem/testing.json")
	lane.git(t, lane.checkout, "commit", "--quiet", "-m", "declare proof fixture")
	lane.git(t, lane.checkout, "push", "--quiet", "origin", "main")
	lane.main = lane.git(t, lane.checkout, "rev-parse", "HEAD")
	git := plain.Git
	mergedAt := start.Add(4 * time.Minute)
	lane.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
		if strings.Join(args, " ") == "show -s --format=%cI HEAD" {
			return mergedAt.UTC().Format(time.RFC3339), nil
		}
		return git(dir, args...)
	}
	var scopes []string
	red := true
	lane.owners.landing.plainProve.Judge = func(string, string, []plain.FailedUnit) (map[string]plain.UnitJudgement, error) { return nil, nil }
	lane.owners.landing.plainProve.Command = func(cmd *exec.Cmd) error {
		env := func(key string) string {
			for _, e := range cmd.Env {
				if v, ok := strings.CutPrefix(e, key+"="); ok {
					return v
				}
			}
			return ""
		}
		commit, only, scope := env("LANDING_COMMIT"), env("LANDING_ONLY"), env("LANDING_PROOF_SCOPE")
		fmt.Fprint(cmd.Stdout, "landing environment fixture\n")
		if only == "" && commit != lane.main {
			scopes = append(scopes, scope)
			minutes := 20
			if !red {
				minutes = 25
			}
			if scope == "scoped" {
				minutes = 6
			}
			now = now.Add(time.Duration(minutes) * time.Minute)
			fmt.Fprint(cmd.Stdout, "landing group fast-static-build green 60000\nlanding package metasystem/cmd/metasystem 1 ok 180000\nlanding package metasystem/cmd/metasystem 2 ok 240000\n")
		}
		if red && commit != lane.main {
			fmt.Fprint(cmd.Stdout, "LANDING-FAILED\tmetasystem/cmd/metasystem\tTestBroken\nLANDING-CHECKED\t1\n")
			// A real shell exit supplies the same process state as production.
			return exec.Command("/usr/bin/false").Run()
		}
		fmt.Fprint(cmd.Stdout, "LANDING-CHECKED\t0\n")
		return nil
	}
	must := func(args ...string) {
		t.Helper()
		if code, text := lane.run(t, args...); code != 0 {
			t.Fatalf("%v exit %d: %s", args, code, text)
		}
	}
	must("landing", "prove", "--trunk", "--wait")
	writer := filepath.Join(t.TempDir(), "seat")
	lane.git(t, filepath.Dir(writer), "clone", "--quiet", lane.origin, writer)
	lane.git(t, writer, "checkout", "--quiet", "-b", "goal/"+bedGoal)
	helmMust(t, os.WriteFile(filepath.Join(writer, "metasystem", "app.txt"), []byte("broken"), 0644))
	lane.git(t, writer, "add", "metasystem/app.txt")
	lane.git(t, writer, "commit", "--quiet", "-m", "candidate")
	lane.git(t, writer, "push", "--quiet", "origin", "HEAD")
	state.status.BranchTip = lane.git(t, writer, "rev-parse", "HEAD")
	code, result := seat.do("work", "land", bedGoal)
	expectOutcome(t, "hand in", code, result, intentConfirmed)
	lane.merge(t, state.status.BranchTip)
	now = mergedAt
	code, text := lane.run(t, "landing", "prove", "--wait")
	if code != 1 {
		t.Fatalf("red proof exit %d: %s", code, text)
	}
	now = start.Add(24 * time.Minute)
	must("landing", "return", bedGoal, "--cause", "own")
	helmMust(t, os.Remove(filepath.Join(writer, "metasystem", "app.txt")))
	helmMust(t, os.MkdirAll(filepath.Join(writer, "metasystem", "plans"), 0755))
	helmMust(t, os.WriteFile(filepath.Join(writer, "metasystem", "plans", "note.md"), []byte("fixed"), 0644))
	lane.git(t, writer, "add", "metasystem/app.txt", "metasystem/plans/note.md")
	lane.git(t, writer, "commit", "--quiet", "-m", "fix")
	lane.git(t, writer, "push", "--quiet", "origin", "HEAD")
	state.status.BranchTip = lane.git(t, writer, "rev-parse", "HEAD")
	now = start.Add(42 * time.Minute)
	code, result = seat.do("work", "land", bedGoal)
	expectOutcome(t, "fixed hand in", code, result, intentConfirmed)
	lane.merge(t, state.status.BranchTip)
	red = false
	now = start.Add(44 * time.Minute)
	must("landing", "prove", "--wait")
	if r, ok, err := plain.LastResult(lane.installation); err != nil || !ok || r.Scope != "scoped" {
		t.Fatalf("scoped proof: scope=%s reason=%s %v", r.Scope, r.ScopeReason, err)
	}
	// Expiry asks the existing rule for a full proof of the same fixed tree.
	now = start.Add(64 * time.Minute)
	must("landing", "prove", "--wait")
	now = start.Add(90 * time.Minute)
	must("landing", "push")
	if strings.Join(scopes, ",") != "full,scoped,full" {
		t.Fatalf("proof scopes: %v", scopes)
	}
	status := lane.status(t)
	push := status["last_push"].(map[string]any)
	raw, err := json.Marshal(push["clock"])
	helmMust(t, err)
	var clock plain.Clock
	helmMust(t, json.Unmarshal(raw, &clock))
	if clock.TotalMinutes == nil || *clock.TotalMinutes != 90 || clock.HandInAt[bedGoal] != start.UTC().Format(time.RFC3339) || len(clock.FixRounds) != 1 || clock.FixRounds[0].Minutes == nil || *clock.FixRounds[0].Minutes != 18 || len(clock.Proofs) != 3 {
		t.Fatalf("episode clock: %s", raw)
	}
	for i, p := range clock.Proofs {
		want := []float64{20, 6, 25}[i]
		if p.Minutes == nil || *p.Minutes != want || p.StartedAt == "" || p.Shards == nil || *p.Shards != 2 || p.Longest == nil || p.Longest.Shard != 2 || p.Longest.MS != 240000 || p.Steps["fast-static-build"] != 60000 {
			t.Fatalf("proof %d: %+v", i, p)
		}
	}
	words := clock.Words()
	for _, part := range []string{"landing took 90.0 min", "hand-in to merge 4.0", "static 1.0", "cheap tier not measured until the pipeline's first tier", "2 shards", "fix round 18.0", "push 1.0"} {
		if !strings.Contains(words, part) {
			t.Fatalf("clock lacks %q: %s", part, words)
		}
	}
	if code, text := lane.run(t, "landing", "status"); code != 0 || !strings.Contains(oneSpaced(text), oneSpaced(words)) {
		t.Fatalf("text status exit %d: %s", code, text)
	}
	state.status.EndpointTip = state.status.BranchTip
	for i := 0; i < 2; i++ {
		code, result = seat.do("work", "land", bedGoal)
		expectOutcome(t, "landed", code, result, intentUnchanged)
		if msg := resultData(t, result)["landedLine"]; msg != nil {
			t.Fatalf("landed record: %v", msg)
		}
	}
	file := seat.goalFile(bedGoal)
	count := 0
	for _, h := range file.History {
		if h.Verb == goal.LandedVerb {
			count++
			if !strings.HasSuffix(h.Reason, "; "+words) {
				t.Fatalf("landed reason: %s", h.Reason)
			}
		}
	}
	if count != 1 {
		t.Fatalf("landed lines: %d", count)
	}
	owners := seat.intentBed.owners()
	owners.delivery = seat.owners
	if code, out, stderr := seat.run(owners, "status", bedGoal); code != 0 || !strings.Contains(oneSpaced(out), "last landing:") || !strings.Contains(oneSpaced(out), oneSpaced(words)) {
		t.Fatalf("goal status exit %d: %s %s", code, out, stderr)
	}
}

func TestAPushRecordedOnceAfterALostResponse(t *testing.T) {
	t.Parallel()
	b := newPlainVerbBed(t)
	b.cwd = b.checkout
	b.setCommand(t, "fixture")
	now := time.Date(2026, 10, 1, 20, 0, 0, 0, time.UTC)
	b.owners.landing.now = func() time.Time { return now }
	b.owners.landing.plainProve.Now = func() time.Time { return now }
	b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
		if strings.Join(args, " ") == "show -s --format=%cI HEAD" {
			return time.Date(2026, 10, 1, 20, 0, 0, 0, time.UTC).Format(time.RFC3339), nil
		}
		return plain.Git(dir, args...)
	}
	b.owners.landing.plainProve.Command = func(cmd *exec.Cmd) error {
		now = now.Add(2 * time.Minute)
		_, err := fmt.Fprint(cmd.Stdout, "landing environment fixture\nlanding package fixture/unit 1 ok 120000\nLANDING-CHECKED\t0\n")
		return err
	}
	sha := b.seat(t, "goal-a")
	head := b.merge(t, sha)
	if code, text := b.run(t, "landing", "prove", "--wait"); code != 0 {
		t.Fatalf("prove exit %d: %s", code, text)
	}
	batch, err := plain.ReadBatch(b.installation)
	helmMust(t, err)
	if batch == nil || batch.State != plain.BatchRunning {
		t.Fatalf("running batch: %+v", batch)
	}
	stop := plain.Stop{Loop: "lane-return", Subject: "goal-a", Tree: sha, At: now.UTC().Format(time.RFC3339), Decision: "stop"}
	data, err := json.Marshal(stop)
	helmMust(t, err)
	helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.installation), "stops.jsonl"), append(data, '\n'), 0644))
	// Main received the proven commit, but the push's follow-up never ran.
	b.git(t, b.checkout, "push", "--quiet", "origin", head+":main")
	now = now.Add(time.Minute)
	for i := 0; i < 2; i++ {
		if code, text := b.run(t, "landing", "push"); code != 0 || !strings.Contains(text, "already on main, recorded") {
			t.Fatalf("recover push %d exit %d: %s", i, code, text)
		}
	}
	data, err = os.ReadFile(filepath.Join(plain.Dir(b.installation), "pushes.jsonl"))
	helmMust(t, err)
	if strings.Count(string(data), "\n") != 1 {
		t.Fatalf("push records: %s", data)
	}
	var push plain.Pushed
	helmMust(t, json.Unmarshal(data, &push))
	if push.Commit != head || push.Old != batch.Base || push.Clock == nil || push.Clock.TotalMinutes == nil || len(push.Clock.Proofs) != 1 {
		t.Fatalf("reconstructed push: %+v", push)
	}
	batch, err = plain.ReadBatch(b.installation)
	helmMust(t, err)
	proof, ok, err := plain.LastResult(b.installation)
	helmMust(t, err)
	stops, err := plain.OpenStops(b.installation)
	helmMust(t, err)
	if batch.State != plain.BatchClosed || !ok || !proof.LoopClosed || len(stops) != 0 {
		t.Fatalf("completion: batch=%s proof=%+v stops=%+v", batch.State, proof, stops)
	}
	if got := b.status(t)["last_push"].(map[string]any); got["commit"] != head || got["clock"] == nil {
		t.Fatalf("status push: %v", got)
	}
}

func TestLandingPushLeavesReturnedMembersUnlanded(t *testing.T) {
	t.Parallel()
	for _, base := range []string{"lane push", "hand push with trunk proof", "mixed batch"} {
		t.Run(base, func(t *testing.T) {
			t.Parallel()
			b := newBatchVerbBed(t, "auto")
			if base == "lane push" {
				sha := b.seat(t, "landed")
				b.success(t, "landing", "run")
				b.assemble(t, sha)
				b.success(t, "landing", "prove", "--wait")
				b.success(t, "landing", "push")
			} else if base == "hand push with trunk proof" {
				b.advanceMain(t, "hand.txt", "hand-pushed base\n")
				b.git(t, b.checkout, "checkout", "--quiet", "--detach", "origin/main")
				b.success(t, "landing", "prove", "--trunk", "--wait")
			}
			b.seat(t, "returned")
			kept := ""
			if base == "mixed batch" {
				kept = b.seat(t, "kept")
			}
			b.git(t, b.checkout, "fetch", "--quiet", "origin")
			b.success(t, "landing", "run")
			selected, err := plain.ReadBatch(b.installation)
			helmMust(t, err)
			b.owners.prove = enrolledPersonProver(t, b.installation, b.now)
			b.success(t, "landing", "return", "returned", "--cause", "unclassified", "--by", "Wido", "--reason", "fix on the seat")
			if kept != "" {
				b.assemble(t, kept)
				b.success(t, "landing", "prove", "--wait")
			}
			// Retain a return-policy stop from an interrupted earlier act.
			stop := plain.Stop{Loop: "lane-return", Subject: "returned", Tree: selected.Members[0].SHA, At: b.now.UTC().Format(time.RFC3339Nano), Decision: "stop"}
			data, err := json.Marshal(stop)
			helmMust(t, err)
			helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.installation), "stops.jsonl"), append(data, '\n'), 0644))
			stops, err := plain.OpenStops(b.installation)
			helmMust(t, err)
			if len(stops) != 1 || stops[0].Loop != "lane-return" || stops[0].Subject != "returned" {
				t.Fatalf("return did not retain its stop: %+v", stops)
			}
			pushPath := filepath.Join(plain.Dir(b.installation), "pushes.jsonl")
			before, err := os.ReadFile(pushPath)
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			b.success(t, "landing", "push")
			after, err := os.ReadFile(pushPath)
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			batch, err := plain.ReadBatch(b.installation)
			helmMust(t, err)
			remaining, err := plain.OpenStops(b.installation)
			helmMust(t, err)
			entry, ok, err := plain.Latest(b.installation, "returned")
			helmMust(t, err)
			if kept != "" {
				push, found, err := plain.LastPush(b.installation)
				helmMust(t, err)
				if !found || len(push.BatchMembers) != 1 || push.BatchMembers[0].Goal != "kept" || push.Clock.HandInAt["returned"] != "" {
					t.Fatalf("push included an unlanded return: %+v", push)
				}
				// The design check also recovers legitimate return-policy closure.
				// Publication must never claim that it closed this member's stop.
				data, err := os.ReadFile(filepath.Join(plain.Dir(b.installation), "stops.jsonl"))
				helmMust(t, err)
				for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
					var recorded plain.Stop
					helmMust(t, json.Unmarshal([]byte(line), &recorded))
					if recorded.Decision == "close" && recorded.Subject == "returned" && recorded.Handoff == "push" {
						t.Fatalf("push closed a returned member's stop: %+v", recorded)
					}
				}
			} else if string(after) != string(before) || batch.State == plain.BatchClosed {
				t.Fatalf("an unlanded batch was completed: batch=%+v pushes=%s", batch, after)
			}
			if batch.ID != selected.ID || !ok || entry.State != plain.StateReturned || kept == "" && (len(remaining) != 1 || remaining[0].At != stops[0].At) {
				t.Fatalf("an unlanded return was completed: batch=%+v entry=%+v stops=%+v pushes=%s", batch, entry, remaining, after)
			}
		})
	}
}

func TestLaneLandingRefusalDoesNotRecordALandedLine(t *testing.T) {
	t.Parallel()
	b, state, install := plainLaneBedWith(t, true, "critic-root", "critic-root")
	state.status.EndpointTip = state.status.BranchTip
	state.status.Status.Units = nil
	state.status.Status.Commits = nil
	helmMust(t, os.Remove(filepath.Join(b.root(), "plans", "designs", "landing-work.md")))
	_, _, err := plain.HandIn(install, plain.Line{Goal: bedGoal, Branch: "goal/" + bedGoal, SHA: state.status.BranchTip, At: laneTestNow.UTC().Format(time.RFC3339)})
	helmMust(t, err)
	b.owners.recordLanded = func(*intentInvocation, string) error {
		t.Fatal("a refusal recorded a landed line")
		return nil
	}
	code, result := b.do("work", "land", bedGoal)
	expectOutcome(t, "no committed work", code, result, intentRefused)
	if result.Data != nil || !strings.Contains(result.Summary, "has no committed work to land") {
		t.Fatalf("wrong refusal after reading landed queue entry: %+v", result)
	}
}
