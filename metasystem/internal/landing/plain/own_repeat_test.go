package plain

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestFinalMergeRepeatsEachFailingTestAlone(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"mixed", "all pass", "report incomplete", "report red", "pass then not run", "pass then command error", "absent", "tree guard spent", "retained", "start only", "worktree fails", "no names"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			b := newRepeatBed(t)
			b.seams.Judge = func(string, string, []FailedUnit) (map[string]UnitJudgement, error) {
				return nil, nil
			}
			proofs := filepath.Join(Dir(b.install), "proofs")
			if err := os.MkdirAll(proofs, 0755); err != nil {
				t.Fatal(err)
			}
			failed := []FailedUnit{{Unit: "u/p", Tests: []string{"TestA", "TestA/sub", "TestB"}}, {Unit: "u/r"}}
			if scenario == "all pass" {
				failed = []FailedUnit{{Unit: "u/p", Tests: []string{"TestA", "TestB"}}}
			} else if scenario == "no names" {
				failed = failed[1:]
			}
			log := filepath.Join(proofs, "final.log")
			if err := os.WriteFile(log, []byte(plainTestEvents("u/p", failed[0].Tests, "fail")), 0600); err != nil {
				t.Fatal(err)
			}
			red := Result{Attempt: "final", Commit: "tip", Tree: "tip-tree", Log: log, Result: Red, Failed: failed,
				Cause: &Cause{Kind: "unclassified", Tests: failingTests(failed), Evidence: log}}
			if err := appendLine(resultsPath(b.install), red); err != nil {
				t.Fatal(err)
			}
			if scenario == "tree guard spent" {
				if err := appendLine(resultsPath(b.install), Result{Tree: red.Tree, Repeat: "started"}); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "start only" {
				start := red
				start.Repeat, start.Cause = "own", &Cause{Kind: "flake"}
				start.Failed = []FailedUnit{{Unit: "u/p", Tests: []string{"TestA", "TestA/sub"}}, {Unit: "u/p", Tests: []string{"TestB"}}}
				for i, identity := range start.Failed {
					repeat := Running{Attempt: fmt.Sprintf("final-own-%d", i+1)}
					repeat.Log = filepath.Join(proofs, repeat.Attempt+".log")
					if err := os.WriteFile(repeat.Log, []byte(plainTestEvents(identity.Unit, identity.Tests, "pass")+"LANDING-CHECKED\t0\n"), 0600); err != nil {
						t.Fatal(err)
					}
					start.FlakeRepeats = append(start.FlakeRepeats, repeat)
				}
				if err := appendLine(resultsPath(b.install), start); err != nil {
					t.Fatal(err)
				}
			}
			adds, removes := 0, 0
			git := b.seams.Git
			b.seams.Git = func(dir string, args ...string) (string, error) {
				if len(args) == 5 && args[0] == "worktree" && args[1] == "add" && args[4] == "tip" {
					adds++
					if scenario == "worktree fails" {
						return "", errors.New("cannot create tip")
					}
				}
				if len(args) == 4 && args[0] == "worktree" && args[1] == "remove" && filepath.Base(args[3]) == "final-replay-2" {
					removes++
				}
				return git(dir, args...)
			}
			var selections []string
			b.seams.Command = func(cmd *exec.Cmd) error {
				commit := commandEnv(cmd, "LANDING_COMMIT")
				if commit == "commit" {
					fmt.Fprint(cmd.Stdout, "LANDING-CHECKED\t0\n")
					return nil
				}
				if commit != "tip" || commandEnv(cmd, "LANDING_ONLY") != "u/p" {
					t.Fatalf("unexpected repeat: commit=%s unit=%s", commit, commandEnv(cmd, "LANDING_ONLY"))
				}
				name := commandEnv(cmd, "LANDING_ONLY_TESTS")
				selections = append(selections, name)
				if name != "TestA" && name != "TestB" {
					t.Fatalf("repeat must select one top-level test: %q", name)
				}
				if name == "TestA" && scenario != "all pass" {
					fmt.Fprint(cmd.Stdout, plainTestEvents("u/p", []string{"TestA", "TestA/sub"}, "fail"), "LANDING-FAILED\tu/p\tTestA TestA/sub\nLANDING-CHECKED\t1\n")
					return errors.New("red")
				}
				if scenario != "absent" {
					fmt.Fprint(cmd.Stdout, plainTestEvents("u/p", []string{name}, "pass"))
				}
				switch scenario {
				case "report incomplete":
					return nil
				case "report red":
					fmt.Fprint(cmd.Stdout, "LANDING-FAILED\tu/p\tTestB\nLANDING-CHECKED\t1\n")
					return nil
				case "pass then not run":
					fmt.Fprint(cmd.Stdout, "LANDING-NOT-RUN\tenvironment\n")
					return nil
				}
				fmt.Fprint(cmd.Stdout, "LANDING-CHECKED\t0\n")
				if scenario == "pass then command error" {
					return errors.New("command lost")
				}
				return nil
			}
			trees := []replayTree{{Running: Running{Commit: "commit"}, Main: true}, {Running: Running{Commit: "tip"}, Goal: GoalSHA{Goal: "g", SHA: "sha-g"}}}
			running := Running{Attempt: red.Attempt, Commit: red.Commit, Tree: red.Tree}
			classify := func() Result {
				input := red
				cause := *red.Cause
				input.Cause = &cause
				return classifyReplay(b.seams, b.install, b.checkout, "fixture", running, input, Result{}, trees)
			}
			got := classify()
			wantRuns := 2
			if scenario == "start only" || scenario == "worktree fails" || scenario == "no names" {
				wantRuns = 0
			}
			if len(selections) != wantRuns || wantRuns > 0 && !reflect.DeepEqual(selections, []string{"TestA", "TestB"}) {
				t.Fatalf("own selections=%v", selections)
			}
			if scenario == "no names" && adds != 0 || scenario != "no names" && adds != 1 || scenario != "no names" && scenario != "worktree fails" && removes != 1 {
				t.Fatalf("own worktrees: added=%d removed=%d", adds, removes)
			}
			wantPassed := []FailedUnit(nil)
			if slices.Contains([]string{"mixed", "tree guard spent", "retained"}, scenario) {
				wantPassed = []FailedUnit{{Unit: "u/p", Tests: []string{"TestB"}}}
			} else if scenario == "all pass" {
				wantPassed = []FailedUnit{{Unit: "u/p", Tests: []string{"TestA"}}, {Unit: "u/p", Tests: []string{"TestB"}}}
			}
			wantFailed := failingTests(failed)
			if len(wantPassed) == 1 {
				wantFailed = []string{"u/p TestA", "u/p TestA/sub", "u/r (package)"}
			} else if scenario == "all pass" {
				wantFailed = nil
			}
			c := got.Cause
			if c == nil || c.Kind != "own" || c.Goal != "g" || c.SHA != "sha-g" || c.Name != "" || c.Evidence != log || !reflect.DeepEqual(c.Tests, red.Cause.Tests) || !reflect.DeepEqual(c.PassedAlone, failingTests(wantPassed)) || got.Result != Red || got.Repeat != "" {
				t.Fatalf("classification=%+v cause=%+v", got, c)
			}
			if scenario != "no names" && scenario != "worktree fails" && !reflect.DeepEqual(c.FailedOn, wantFailed) {
				t.Fatalf("failed on=%v want=%v", c.FailedOn, wantFailed)
			}
			if scenario == "worktree fails" && !strings.Contains(got.Reason, "the merge's tests could not be repeated alone: cannot create tip") {
				t.Fatalf("missing worktree failure: %s", got.Reason)
			}
			if len(wantPassed) == 1 {
				want := "TestA (u/p), u/r failed on the merge of g (sha-g) with main; u/p, u/r passed on main at commit; TestB (u/p) failed in the suite and passed alone on the merge"
				if words := returnWords(*c); words != want {
					t.Fatalf("words=%q want=%q", words, want)
				}
			} else if scenario == "all pass" {
				want := "TestA (u/p), TestB (u/p) failed in the suite on the merge of g (sha-g) with main and passed alone there; u/p passed on main at commit"
				if words := returnWords(*c); words != want {
					t.Fatalf("words=%q want=%q", words, want)
				}
				bound, found, _ := b.seams.checkBound(b.install, red.Tree)
				if !found || bound.Result != Red {
					t.Fatalf("passing alone turned the proof green: %+v found=%v", bound, found)
				}
			}
			var own []Result
			for _, row := range b.lines() {
				if row.Repeat == "own" {
					own = append(own, row)
					if row.RepeatComplete || row.FlakePublished {
						t.Fatalf("own episode completed or published a flake repeat: %+v", row)
					}
				}
			}
			if wantRuns > 0 {
				if len(own) != 2 || !reflect.DeepEqual(own[1].RepeatPassed, wantPassed) {
					t.Fatalf("own rows=%+v want passed=%+v", own, wantPassed)
				}
				for i, repeat := range own[1].FlakeRepeats {
					want := fmt.Sprintf("final-own-%d", i+1)
					if repeat.Attempt != want || repeat.Log != filepath.Join(proofs, want+".log") {
						t.Fatalf("own execution=%+v", repeat)
					}
				}
			}
			if scenario == "retained" || scenario == "start only" {
				before := len(selections)
				again := classify()
				if len(selections) != before || !reflect.DeepEqual(again.Cause, got.Cause) || returnWords(*again.Cause) != returnWords(*got.Cause) {
					t.Fatalf("retained episode ran or changed: before=%+v after=%+v runs=%v", got.Cause, again.Cause, selections)
				}
			}
			if len(b.records) != 0 {
				t.Fatalf("own repeats published flakes: %+v", b.records)
			}
		})
	}
}

func TestClassifyAttemptSkipsOwnEpisodeRows(t *testing.T) {
	t.Parallel()
	b := newRepeatBed(t)
	if err := os.MkdirAll(Dir(b.install), 0755); err != nil {
		t.Fatal(err)
	}
	b.seams.Person = &ActProvenance{Person: "Wido", Root: b.install}
	b.seams.CommandForCommit = func(string) (string, error) { return "fixture", nil }
	b.seams.Judge = func(string, string, []FailedUnit) (map[string]UnitJudgement, error) { return nil, nil }
	git := b.seams.Git
	b.seams.Git = func(dir string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "rev-parse --verify --quiet refs/remotes/origin/main^{commit}":
			return "commit", nil
		case "show -s --format=%P tip":
			return "commit sha-g", nil
		case "log --first-parent --merges --reverse --format=%H %P commit..tip":
			return "tip commit sha-g", nil
		}
		return git(dir, args...)
	}
	b.seams.Command = func(cmd *exec.Cmd) error {
		if commandEnv(cmd, "LANDING_COMMIT") != "commit" {
			t.Fatalf("retained own episode ran: %v", cmd.Env)
		}
		fmt.Fprint(cmd.Stdout, "LANDING-CHECKED\t0\n")
		return nil
	}
	red := Result{Attempt: "saved", Commit: "tip", Tree: "tip-tree", Result: Red, Goals: []GoalSHA{{Goal: "g", SHA: "sha-g"}},
		Failed: []FailedUnit{{Unit: "u/p", Tests: []string{"TestA"}}, {Unit: "u/r"}}, Cause: &Cause{Kind: "unclassified"}}
	red.Cause.Tests = failingTests(red.Failed)
	own := red
	own.Repeat, own.Cause = "own", &Cause{Kind: "flake"}
	own.Failed = red.Failed[:1]
	own.RepeatPassed = own.Failed
	for _, row := range []Result{red, own} {
		if err := appendLine(resultsPath(b.install), row); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ClassifyAttempt(b.install, b.checkout, red.Attempt, b.seams)
	if err != nil || !reflect.DeepEqual(got.Failed, red.Failed) || got.Cause == nil || got.Cause.Kind != "own" || !reflect.DeepEqual(got.Cause.FailedOn, []string{"u/r (package)"}) {
		t.Fatalf("saved red lost package failure: %+v cause=%+v err=%v", got, got.Cause, err)
	}
}

func TestFinalMergePublishesWhatPassedAlone(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"same package", "reaches", "judge nil", "judge error", "judge missing unit", "all flake", "all flake after a proof", "retained", "recorder nil", "recorder fails", "suite evidence missing", "seen once"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			b := newRepeatBed(t)
			allFlake := strings.HasPrefix(scenario, "all flake")
			failed := []FailedUnit{{Unit: "u/p", Tests: []string{"TestA", "TestB"}}}
			passed := []FailedUnit{{Unit: "u/p", Tests: []string{"TestB"}}}
			if allFlake {
				passed = append([]FailedUnit{{Unit: "u/p", Tests: []string{"TestA"}}}, passed...)
			}
			proofs := filepath.Join(Dir(b.install), "proofs")
			if err := os.MkdirAll(proofs, 0755); err != nil {
				t.Fatal(err)
			}
			log := filepath.Join(proofs, "final.log")
			events := plainTestEvents("u/p", failed[0].Tests, "fail")
			if scenario == "suite evidence missing" {
				events = ""
			}
			if err := os.WriteFile(log, []byte(events), 0600); err != nil {
				t.Fatal(err)
			}
			red := Result{Attempt: "final", Commit: "tip", Tree: "tip-tree", Log: log, Result: Red, Failed: failed, Load: 2.75, Reason: "suite red",
				Cause: &Cause{Kind: "unclassified", Tests: failingTests(failed), Evidence: log}}
			if err := appendLine(resultsPath(b.install), red); err != nil {
				t.Fatal(err)
			}
			judgeCalls := 0
			b.seams.Judge = func(checkout, commit string, units []FailedUnit) (map[string]UnitJudgement, error) {
				judgeCalls++
				if checkout != b.checkout || commit != "tip" || !reflect.DeepEqual(units, passed) {
					t.Fatalf("judge input: %s %s %+v", checkout, commit, units)
				}
				if scenario == "judge missing unit" {
					return map[string]UnitJudgement{"u/other": {}}, nil
				}
				judgements := map[string]UnitJudgement{"u/p": {Affected: scenario == "reaches", Surfaces: []string{"surface-p"}}}
				if scenario == "judge error" {
					return judgements, errors.New("judge unavailable")
				}
				return judgements, nil
			}
			if scenario == "judge nil" {
				b.seams.Judge = nil
			}
			if scenario == "recorder nil" {
				b.seams.RecordFlake = nil
			} else if scenario == "recorder fails" {
				b.seams.RecordFlake = func(FlakeRecord) (FlakeRecorded, error) { return FlakeRecorded{}, errors.New("register unavailable") }
			} else if scenario == "seen once" {
				b.seams.RecordFlake = func(f FlakeRecord) (FlakeRecorded, error) {
					b.records = append(b.records, f)
					return FlakeRecorded{Goal: "fix-flaky-u-p", Seen: 1}, nil
				}
			}
			var selections []string
			b.seams.Command = func(cmd *exec.Cmd) error {
				if commandEnv(cmd, "LANDING_COMMIT") == "commit" {
					fmt.Fprint(cmd.Stdout, "LANDING-CHECKED\t0\n")
					return nil
				}
				name := commandEnv(cmd, "LANDING_ONLY_TESTS")
				if commandEnv(cmd, "LANDING_COMMIT") != "tip" || commandEnv(cmd, "LANDING_ONLY") != "u/p" || name != "TestA" && name != "TestB" {
					t.Fatalf("unexpected repeat: %v", cmd.Env)
				}
				selections = append(selections, name)
				if name == "TestA" && !allFlake {
					fmt.Fprint(cmd.Stdout, plainTestEvents("u/p", []string{name}, "fail"), "LANDING-FAILED\tu/p\tTestA\nLANDING-CHECKED\t1\n")
					return errors.New("red")
				}
				fmt.Fprint(cmd.Stdout, plainTestEvents("u/p", []string{name}, "pass"), "LANDING-CHECKED\t0\n")
				return nil
			}
			trees := []replayTree{{Running: Running{Commit: "commit"}, Main: true}, {Running: Running{Commit: "tip"}, Goal: GoalSHA{Goal: "g", SHA: "sha-g"}}}
			running := Running{Attempt: red.Attempt, Commit: red.Commit, Tree: red.Tree}
			previous := Result{}
			if scenario == "all flake after a proof" {
				previous.Result = Red
			}
			classify := func(input Result) Result {
				cause := *input.Cause
				input.Cause = &cause
				return classifyReplay(b.seams, b.install, b.checkout, "fixture", running, input, previous, trees)
			}
			got := classify(red)
			if !reflect.DeepEqual(selections, []string{"TestA", "TestB"}) || !reflect.DeepEqual(got.Failed, failed) || got.Result != Red || got.Cause == nil || !reflect.DeepEqual(got.Cause.Tests, red.Cause.Tests) || got.Cause.Evidence != log {
				t.Fatalf("proof or evidence changed: %+v cause=%+v runs=%v", got, got.Cause, selections)
			}
			wantJudgeCalls := 1
			if scenario == "judge nil" {
				wantJudgeCalls = 0
			}
			if judgeCalls != wantJudgeCalls {
				t.Fatalf("judge calls=%d want=%d", judgeCalls, wantJudgeCalls)
			}
			wantRecords := 1
			if slices.Contains([]string{"reaches", "judge nil", "judge error", "judge missing unit", "recorder nil", "recorder fails", "suite evidence missing"}, scenario) {
				wantRecords = 0
			} else if allFlake {
				wantRecords = 2
			}
			if len(b.records) != wantRecords || !reflect.DeepEqual(got.Flakes, b.records) {
				t.Fatalf("records=%+v flakes=%+v want=%d", b.records, got.Flakes, wantRecords)
			}
			for i, f := range b.records {
				ordinal := 2
				if allFlake {
					ordinal = i + 1
				}
				attempt := fmt.Sprintf("final-own-%d", ordinal)
				if f.Unit != "u/p" || !reflect.DeepEqual(f.Tests, passed[i].Tests) || f.Where != "tip" || f.Repeat != "alone" || f.RepeatAttempt != attempt || f.OpKey != attempt || f.RepeatLog != filepath.Join(proofs, attempt+".log") || f.Attempt != red.Attempt || f.Commit != red.Commit || f.Tree != red.Tree || f.Log != red.Log || f.Load != red.Load || !reflect.DeepEqual(f.Surfaces, []string{"surface-p"}) {
					t.Fatalf("record provenance=%+v", f)
				}
				if len(f.Outputs) != 1 || len(f.RepeatOutputs) != 1 || f.Outputs[f.Tests[0]].Outcome != "fail" || f.RepeatOutputs[f.Tests[0]].Outcome != "pass" {
					t.Fatalf("record did not narrow the full suite evidence: %+v", f)
				}
				seen := "3 times"
				if scenario == "seen once" {
					seen = "once"
				}
				clause := fmt.Sprintf("; %s (u/p) failed in the suite, passed alone on the merge; seen %s; goal fix-flaky-u-p fixes it", f.Tests[0], seen)
				if !strings.Contains(got.Reason, clause) {
					t.Fatalf("missing registration reason %q: %s", clause, got.Reason)
				}
			}
			if allFlake {
				wantRepeat := "allowed"
				if previous.Result != "" {
					wantRepeat = ""
				}
				if got.Cause.Kind != "flake" || got.Cause.Goal != "" || got.Cause.SHA != "" || got.Cause.Name != strings.Join(red.Cause.Tests, ", ") || got.Repeat != wantRepeat || len(got.Cause.FailedOn) != 0 || len(got.Cause.PassedAlone) != 0 || b.records[0].OpKey == b.records[1].OpKey {
					t.Fatalf("all flake attribution=%+v repeat=%s", got.Cause, got.Repeat)
				}
			} else {
				wantPassed := failingTests(passed)
				if wantRecords > 0 {
					wantPassed = nil
				}
				if got.Cause.Kind != "own" || got.Cause.Goal != "g" || got.Cause.SHA != "sha-g" || got.Cause.Name != "" || !reflect.DeepEqual(got.Cause.FailedOn, []string{"u/p TestA"}) || !reflect.DeepEqual(got.Cause.PassedAlone, wantPassed) || got.Repeat != "" {
					t.Fatalf("own attribution=%+v repeat=%s", got.Cause, got.Repeat)
				}
				if wantRecords > 0 && returnWords(*got.Cause) != "TestA (u/p) failed on the merge of g (sha-g) with main; u/p passed on main at commit" {
					t.Fatalf("return words=%s", returnWords(*got.Cause))
				}
			}
			if slices.Contains([]string{"recorder nil", "recorder fails", "suite evidence missing"}, scenario) && !strings.Contains(got.Reason, "; the flaky tests of u/p could not be recorded:") {
				t.Fatalf("missing publication failure: %s", got.Reason)
			}
			if scenario == "recorder fails" && !strings.Contains(got.Reason, "register unavailable") {
				t.Fatalf("recorder error was lost: %s", got.Reason)
			}
			var own []Result
			for _, row := range b.lines() {
				if row.Repeat == "own" {
					own = append(own, row)
					if row.RepeatComplete {
						t.Fatalf("own episode spent the full repeat: %+v", row)
					}
				}
			}
			last := own[len(own)-1]
			if !reflect.DeepEqual(last.RepeatPassed, passed) || wantRecords > 0 && (!last.FlakePublished || !reflect.DeepEqual(last.Flakes, got.Flakes)) {
				t.Fatalf("saved publication=%+v", last)
			}
			if scenario == "retained" {
				beforeRows := len(own)
				again := classify(got)
				if len(selections) != 2 || len(b.records) != wantRecords || judgeCalls != 1 || !reflect.DeepEqual(again.Cause, got.Cause) || !reflect.DeepEqual(again.Flakes, got.Flakes) || again.Reason != got.Reason {
					t.Fatalf("retained publication ran or changed: %+v cause=%+v", again, again.Cause)
				}
				ownRows := 0
				for _, row := range b.lines() {
					if row.Repeat == "own" {
						ownRows++
					}
				}
				if ownRows != beforeRows {
					t.Fatalf("retained publication added rows: %d -> %d", beforeRows, ownRows)
				}
			}
			bound, found, _ := b.seams.checkBound(b.install, red.Tree)
			if !found || bound.Result != Red {
				t.Fatalf("flake publication turned the proof green: %+v found=%v", bound, found)
			}
		})
	}
}
