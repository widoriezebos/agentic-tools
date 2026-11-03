package loopstop

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestARedIsClassifiedByOneTable(t *testing.T) {
	t.Parallel()
	red := []RedUnit{{Unit: "internal/foo", Tests: []string{"TestX"}}}
	unnamed := []RedUnit{{Unit: "section/audits"}}
	selectors := []string{"LANDING_ONLY=internal/foo", "LANDING_ONLY_TESTS=TestX"}
	green := RedRun{Attempted: true, Exited: true, Complete: true, Passed: true, Selectors: selectors}
	failed := green
	failed.Passed, failed.Failed = false, red
	lost := RedRun{Attempted: true, Selectors: selectors}
	partial := RedRun{Attempted: true, Exited: true, Selectors: selectors}
	different := failed
	different.Selectors = []string{"LANDING_ONLY=internal/foo=TestX,TestY", "LANDING_ONLY_TESTS=TestX TestY"}
	wholeGreen := RedRun{Attempted: true, Exited: true, Complete: true, Passed: true, Whole: true}
	wrapperGreen := RedRun{Attempted: true, Exited: true, Passed: true, Whole: true, Wrapper: true}
	wrapperRed := wrapperGreen
	wrapperRed.Passed = false
	base := func(run RedRun) []RedReplay { return []RedReplay{{Role: "base", Named: run}} }
	input := func(alone RedRun, replays []RedReplay) RedEvidence {
		return RedEvidence{Goal: "subject", Failed: red, Alone: alone, Replays: replays}
	}
	withUnnamed := func(alone RedRun, replays []RedReplay) RedEvidence {
		e := input(alone, replays)
		e.Failed = slices.Concat(red, unnamed)
		return e
	}
	blocked := input(RedRun{}, base(RedRun{}))
	blocked.CannotRun = "the check evidence is unreadable"
	mixed := input(failed, base(green))
	mixed.Failed = []RedUnit{{Unit: "internal/foo", Tests: []string{"TestX", "TestY"}}}
	mixed.Alone.Selectors = []string{"LANDING_ONLY=internal/foo", "LANDING_ONLY_TESTS=TestX TestY"}
	mixed.Replays[0].Named.Selectors = mixed.Alone.Selectors
	multiple := input(RedRun{}, base(RedRun{}))
	multiple.Failed = append(slices.Clone(red), RedUnit{Unit: "internal/bar", Tests: []string{"TestY", "TestZ"}})
	registered := input(failed, base(green))
	registered.Failed = []RedUnit{{Unit: "internal/foo", Tests: []string{"TestX"}, Registered: []string{"flake:6373c392"}}}
	greenUnreadable := input(green, base(green))
	greenUnreadable.CannotRun = "the check evidence is unreadable"
	environmentFirst := blocked
	environmentFirst.Alone = lost
	greenIncomplete := green
	greenIncomplete.Complete = false
	unrelated := failed
	unrelated.Failed = []RedUnit{{Unit: "internal/foo", Tests: []string{"TestOther"}}}
	packageFailure := failed
	packageFailure.Failed = []RedUnit{{Unit: "internal/foo"}}
	unreportedTest := failed
	unreportedTest.Failed = []RedUnit{{Unit: "internal/foo", Tests: []string{"TestX(did not report)"}}}
	for _, row := range []struct {
		name                            string
		in                              RedEvidence
		next, kind, goal, sha, incident string
		selectors, tests, flaked        []string
		whole                           bool
		replay                          int
	}{
		{name: "rerun did not exit", in: input(lost, base(RedRun{})), kind: "environment", tests: []string{"internal/foo TestX"}},
		{name: "rerun report incomplete", in: input(partial, base(RedRun{})), kind: "environment", tests: []string{"internal/foo TestX"}},
		{name: "cannot read before rerun", in: blocked, kind: "unclassified", tests: []string{"internal/foo TestX"}},
		{name: "cannot read before flake", in: greenUnreadable, kind: "unclassified", flaked: []string{"internal/foo TestX"}},
		{name: "environment before unreadable", in: environmentFirst, kind: "environment", tests: []string{"internal/foo TestX"}},
		{name: "one unit rerun", in: input(RedRun{}, base(RedRun{})), next: "alone", selectors: selectors},
		{name: "several units rerun", in: multiple, next: "alone", selectors: []string{"LANDING_ONLY=internal/foo=TestX internal/bar=TestY,TestZ", "LANDING_ONLY_TESTS=TestX TestY TestZ"}},
		{name: "green rerun", in: input(green, nil), kind: "flake", tests: []string{"internal/foo TestX"}, flaked: []string{"internal/foo TestX"}},
		{name: "audits keep green rerun red", in: withUnnamed(green, base(RedRun{Whole: true})), next: "replay", flaked: []string{"internal/foo TestX"}},
		{name: "audits own after green base", in: withUnnamed(green, base(wrapperGreen)), kind: "own", goal: "subject", tests: []string{"section/audits (package)"}, flaked: []string{"internal/foo TestX"}},
		{name: "no replay declared", in: input(failed, nil), kind: "unclassified", tests: []string{"internal/foo TestX"}},
		{name: "incomplete green rerun cannot prove own", in: input(greenIncomplete, base(wrapperGreen)), kind: "unclassified", tests: []string{"internal/foo TestX"}},
		{name: "base not attempted", in: input(failed, base(RedRun{Selectors: selectors})), next: "replay", selectors: selectors},
		{name: "next replay after green", in: input(failed, []RedReplay{{Role: "main", Named: green}, {Role: "base", Named: RedRun{Selectors: selectors}}}), next: "replay", selectors: selectors, replay: 1},
		{name: "base did not exit", in: input(failed, base(lost)), kind: "environment", tests: []string{"internal/foo TestX"}},
		{name: "like for like incomplete", in: input(failed, base(partial)), kind: "environment", tests: []string{"internal/foo TestX"}},
		{name: "like for like green", in: input(failed, base(green)), kind: "own", goal: "subject", tests: []string{"internal/foo TestX"}},
		{name: "package failure rerun green base", in: input(packageFailure, base(green)), kind: "own", goal: "subject", tests: []string{"internal/foo TestX"}},
		{name: "unreported test rerun green base", in: input(unreportedTest, base(green)), kind: "own", goal: "subject", tests: []string{"internal/foo TestX"}},
		{name: "package failure rerun red main", in: input(packageFailure, []RedReplay{{Role: "main", Named: failed}}), kind: "main", incident: "red:internal/foo:TestX", tests: []string{"internal/foo TestX"}},
		{name: "registered red twice", in: registered, kind: "own", goal: "subject", incident: "flake:6373c392", tests: []string{"internal/foo TestX"}},
		{name: "green needs whole fallback", in: input(different, base(green)), next: "replay", whole: true},
		{name: "incomplete needs whole fallback", in: input(different, base(partial)), next: "replay", whole: true},
		{name: "lost named needs whole fallback", in: input(different, base(lost)), next: "replay", whole: true},
		{name: "whole fallback green", in: input(different, []RedReplay{{Role: "base", Named: green, Whole: wholeGreen}}), kind: "own", goal: "subject", tests: []string{"internal/foo TestX"}},
		{name: "whole fallback red on main", in: input(different, []RedReplay{{Role: "main", Named: green, Whole: RedRun{Attempted: true, Exited: true, Complete: true}}}), kind: "main", incident: "red:internal/foo:TestX", tests: []string{"internal/foo TestX"}},
		{name: "whole fallback cannot exit", in: input(different, []RedReplay{{Role: "base", Named: green, Whole: RedRun{Attempted: true, Whole: true}}}), kind: "environment", tests: []string{"internal/foo TestX"}},
		{name: "main reproduces", in: input(failed, []RedReplay{{Role: "main", Named: failed}}), kind: "main", incident: "red:internal/foo:TestX", tests: []string{"internal/foo TestX"}},
		{name: "first reproducing tree wins", in: input(failed, []RedReplay{{Role: "main", Named: failed}, {Role: "goal", Goal: "merge", SHA: "abc", Named: failed}}), kind: "main", incident: "red:internal/foo:TestX", tests: []string{"internal/foo TestX"}},
		{name: "goal merge reproduces", in: input(failed, []RedReplay{{Role: "goal", Goal: "merge", SHA: "abc", Named: failed}}), kind: "own", goal: "merge", sha: "abc", tests: []string{"internal/foo TestX"}},
		{name: "non main base reproduces", in: input(failed, base(failed)), kind: "other", tests: []string{"internal/foo TestX"}},
		{name: "unrelated named red cannot prove own", in: input(failed, base(unrelated)), kind: "unclassified", tests: []string{"internal/foo TestX"}},
		{name: "later tree reproduces after unrelated red", in: input(failed, []RedReplay{{Role: "main", Named: unrelated}, {Role: "goal", Goal: "merge", SHA: "abc", Named: failed}}), kind: "own", goal: "merge", sha: "abc", tests: []string{"internal/foo TestX"}},
		{name: "unnamed green base", in: RedEvidence{Goal: "subject", Failed: unnamed, Replays: base(wholeGreen)}, kind: "own", goal: "subject", tests: []string{"section/audits (package)"}},
		{name: "unnamed red main", in: RedEvidence{Failed: unnamed, Replays: []RedReplay{{Role: "main", Named: wrapperRed}}}, kind: "main", incident: "red:section/audits", tests: []string{"section/audits (package)"}},
		{name: "wrapper green without report", in: RedEvidence{Goal: "subject", Failed: unnamed, Replays: base(wrapperGreen)}, kind: "own", goal: "subject", tests: []string{"section/audits (package)"}},
		{name: "wrapper red without report", in: RedEvidence{Failed: unnamed, Replays: base(wrapperRed)}, kind: "other", tests: []string{"section/audits (package)"}},
		{name: "mixed rerun", in: mixed, kind: "own", goal: "subject", tests: []string{"internal/foo TestX"}, flaked: []string{"internal/foo TestY"}},
		{name: "unnamed no replay", in: RedEvidence{Failed: unnamed}, kind: "unclassified", tests: []string{"section/audits (package)"}},
		{name: "green missing report cannot prove own", in: input(failed, base(RedRun{Attempted: true, Exited: true, Passed: true, Selectors: selectors})), kind: "unclassified", tests: []string{"internal/foo TestX"}},
		{name: "wrapper rerun red needs base", in: input(wrapperRed, base(RedRun{Whole: true})), next: "replay"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			before, err := json.Marshal(row.in)
			if err != nil {
				t.Fatal(err)
			}
			got := ClassifyRed(row.in)
			if got.Cause != nil && got.Cause.Kind == "own" && got.Cause.Goal == row.in.Goal && slices.ContainsFunc(got.Cause.Tests, func(test string) bool { return !strings.HasSuffix(test, " (package)") }) {
				if !row.in.Alone.Attempted || !row.in.Alone.Exited || row.in.Alone.Passed || len(row.in.Replays) == 0 {
					t.Fatalf("own named red lacks an exited red rerun and a replay: %+v", row.in)
				}
				for _, replay := range row.in.Replays {
					if !(replay.Named.Exited && replay.Named.Complete && replay.Named.Passed || replay.Whole.Exited && replay.Whole.Complete && replay.Whole.Passed) {
						t.Fatalf("own named red lacks a completed green replay: %+v", replay)
					}
				}
			}
			after, err := json.Marshal(row.in)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) || !reflect.DeepEqual(got, ClassifyRed(row.in)) {
				t.Fatalf("classification changed its input or answer: %+v", got)
			}
			if got.Next != row.next || !slices.Equal(got.Selectors, row.selectors) || got.Whole != row.whole || got.Replay != row.replay || !slices.Equal(got.Flaked, row.flaked) {
				t.Fatalf("verdict=%+v; want next=%q selectors=%v whole=%v flaked=%v", got, row.next, row.selectors, row.whole, row.flaked)
			}
			if row.kind == "" {
				if got.Cause != nil {
					t.Fatalf("next run also has cause: %+v", got.Cause)
				}
				return
			}
			if got.Cause == nil || got.Cause.Kind != row.kind || got.Cause.Goal != row.goal || got.Cause.SHA != row.sha || got.Cause.Name != row.incident || !slices.Equal(got.Cause.Tests, row.tests) {
				t.Fatalf("cause=%+v; want kind=%s goal=%s sha=%s incident=%s tests=%v", got.Cause, row.kind, row.goal, row.sha, row.incident, row.tests)
			}
			if row.kind == "flake" && got.Words != "checks passed after a flake rerun: TestX (internal/foo) failed in the check and passed alone on this tree" {
				t.Fatalf("flake words: %q", got.Words)
			}
		})
	}
}

func TestRedSelectorsOmitUnnamedUnits(t *testing.T) {
	t.Parallel()
	for _, units := range [][]RedUnit{nil, {{Unit: "section/audits"}}} {
		if got := RedSelectors(units); len(got) != 0 {
			t.Fatalf("unnamed selectors: %v", got)
		}
	}
	got := RedSelectors([]RedUnit{{Unit: "section/audits"}, {Unit: "internal/foo", Tests: []string{"TestX", "TestY"}}})
	if !slices.Equal(got, []string{"LANDING_ONLY=internal/foo", "LANDING_ONLY_TESTS=TestX TestY"}) {
		t.Fatalf("mixed selectors: %v", got)
	}
}

func TestRedWordsNameTheEvidence(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"own", "registered", "main", "other", "environment", "unclassified", "unnamed", "several"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			units := []RedUnit{{Unit: "internal/foo", Tests: []string{"TestX"}}}
			selectors := RedSelectors(units)
			run := RedRun{Attempted: true, Exited: true, Complete: true, Selectors: selectors, Failed: units}
			e := RedEvidence{Goal: "subject", Failed: units, Alone: run, Replays: []RedReplay{{Role: "base", Named: run}}}
			e.Replays[0].Named.Passed = true
			e.Replays[0].Named.Failed = nil
			want := "TestX in internal/foo failed in the check, failed again alone on this tree, and passed on the unit's base: the change causes it"
			switch kind {
			case "registered":
				e.Failed[0].Registered = []string{"flake:6373c392abcdef00"}
				want = strings.Replace(want, "internal/foo failed", "internal/foo, registered as flake:6373c392, failed", 1)
			case "main":
				e.Replays[0].Role, e.Replays[0].Named.Passed = "main", false
				e.Replays[0].Named.Failed = units
				want = "TestX in internal/foo fails on main too (main's incident red:internal/foo:TestX); this change is not the cause"
			case "other":
				e.Replays[0].Named.Passed = false
				e.Replays[0].Named.Failed = units
				want = "TestX in internal/foo failed in the check and on the unit's base, which is not main"
			case "environment":
				e.Replays[0].Named.Exited = false
				want = "TestX in internal/foo: the rerun or the base run could not finish (no completed exit evidence or complete report)"
			case "unclassified":
				e.CannotRun = "the base cannot be read"
				want = "the red could not be classified: the base cannot be read"
			case "unnamed":
				e.Failed, e.Alone = []RedUnit{{Unit: "section/audits"}}, RedRun{}
				e.Replays[0].Named = RedRun{Attempted: true, Exited: true, Passed: true, Wrapper: true}
				want = "section/audits failed in the check and passed on the unit's base; no rerun alone is possible for a failure that names no test"
			case "several":
				e.Failed = append(e.Failed, RedUnit{Unit: "internal/bar", Tests: []string{"TestY"}})
				e.Alone.Failed = e.Failed
				e.Alone.Selectors = RedSelectors(e.Failed)
				e.Replays[0].Named.Selectors = e.Alone.Selectors
				want += "; " + strings.ReplaceAll(strings.ReplaceAll(want, "TestX", "TestY"), "internal/foo", "internal/bar")
			}
			v := ClassifyRed(e)
			if v.Words != want || kind == "registered" && v.Cause.Name != "flake:6373c392abcdef00" {
				t.Fatalf("words=%q cause=%+v; want %q", v.Words, v.Cause, want)
			}
		})
	}
}
