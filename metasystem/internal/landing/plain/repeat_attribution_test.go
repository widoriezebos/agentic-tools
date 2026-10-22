package plain

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
)

type attributionBed struct {
	*standingBed
	records      []FlakeRecord
	publications []FlakeRecord
	checks       []Result
	repeats      []string
	first        []FailedUnit
}

func newAttributionBed(t *testing.T, mode string) *attributionBed {
	t.Helper()
	b := &attributionBed{standingBed: newStandingBed(t, false), first: []FailedUnit{{Unit: "u/a", Tests: []string{"TestA1", "TestA2"}}, {Unit: "u/b", Tests: []string{"TestB1"}}}}
	if mode == "mixed-flake-first" {
		b.first[0].Tests = []string{"TestA2", "TestA1"}
	}
	if err := os.Remove(resultsPath(b.install)); err != nil {
		t.Fatal(err)
	}
	if mode == "scoped-mixed-with-inherited" {
		b.red.Failed = b.red.Failed[:1]
		b.incidents = b.incidents[:1]
		b.save(t, b.red)
	}
	exited := replayExitOne(t)
	var lost *os.ProcessState
	if mode == "lost-process" {
		child := exec.Command("sh", "-c", "kill -KILL $$")
		_ = child.Run()
		lost = child.ProcessState
		if lost == nil || lost.Exited() {
			t.Fatal("fixture process must be signalled")
		}
	}
	b.seams.Load = func(time.Time) hostload.Sample { return hostload.Sample{Available: true, Cores: 1, Load1m: 9} }
	b.seams.RecordMain = func(checks []Result) error {
		for _, check := range checks {
			if len(check.Failed) == 0 {
				t.Fatal("recorder received an empty check")
			}
		}
		b.checks = append(b.checks, checks...)
		return nil
	}
	b.seams.RecordFlake = func(f FlakeRecord) (FlakeRecorded, error) {
		b.publications = append(b.publications, f)
		if mode == "record-fails" && f.Unit == "u/b" {
			return FlakeRecorded{}, errors.New("publication refused")
		}
		b.records = append(b.records, f)
		return FlakeRecorded{Seen: 1}, nil
	}
	if mode == "nil-record" {
		b.seams.RecordFlake = nil
	}
	b.seams.Command = func(cmd *exec.Cmd) error {
		only := commandEnv(cmd, "LANDING_ONLY")
		if only == "" || strings.Contains(only, " ") {
			fmt.Fprintln(cmd.Stdout, "landing environment "+b.environment)
			for _, group := range b.contract.Groups {
				if commandEnv(cmd, "LANDING_PROOF_SCOPE") == "scoped" && !slices.Contains(strings.Fields(commandEnv(cmd, "LANDING_PROOF_GROUPS")), group.ID) {
					continue
				}
				fmt.Fprintf(cmd.Stdout, "landing group %s green 1\n", group.ID)
			}
			for _, unit := range b.first {
				fmt.Fprint(cmd.Stdout, plainTestEvents(unit.Unit, unit.Tests, "fail"))
			}
			attributionReport(cmd, b.first, true)
			return &exec.ExitError{ProcessState: exited}
		}
		b.repeats = append(b.repeats, only)
		index := slices.IndexFunc(b.first, func(f FailedUnit) bool { return f.Unit == only })
		if index < 0 {
			t.Fatalf("unexpected repeat unit %q", only)
		}
		if got := commandEnv(cmd, "LANDING_ONLY_TESTS"); got != strings.Join(b.first[index].Tests, " ") {
			t.Fatalf("repeat tests %q, want %v", got, b.first[index].Tests)
		}
		var failed []FailedUnit
		for _, name := range b.first[index].Tests {
			action := "pass"
			if name == "TestA1" {
				action = "fail"
			}
			if mode == "all-pass" || mode == "both-pass-in-unit" || mode == "other-failure-in-repeat" {
				action = "pass"
			}
			if mode == "all-fail" || mode == "both-pass-in-unit" && only == "u/b" {
				action = "fail"
			}
			if name == "TestA2" && mode == "skipped" {
				action = "skip"
			}
			if name == "TestA2" && mode == "missing-terminal" {
				action = "run"
			}
			// Each package has one terminal after all of its test events.
			event, _ := json.Marshal(map[string]string{"Action": action, "Package": only, "Test": name})
			fmt.Fprintln(cmd.Stdout, string(event))
			if action == "fail" || name == "TestA2" && mode == "listed-but-passed" {
				if len(failed) == 0 {
					failed = []FailedUnit{{Unit: only}}
				}
				failed[0].Tests = append(failed[0].Tests, name)
			}
		}
		if mode == "other-failure-in-repeat" && only == "u/a" {
			failed = []FailedUnit{{Unit: only}}
		}
		packageAction := "pass"
		if len(failed) > 0 {
			packageAction = "fail"
		}
		fmt.Fprint(cmd.Stdout, plainTestEvents(only, nil, packageAction))
		attributionReport(cmd, failed, mode != "incomplete-beside-pass" || only != "u/a")
		if only == "u/a" && lost != nil {
			return &exec.ExitError{ProcessState: lost}
		}
		if len(failed) > 0 {
			return &exec.ExitError{ProcessState: exited}
		}
		return nil
	}
	return b
}

func attributionReport(cmd *exec.Cmd, failed []FailedUnit, complete bool) {
	for _, f := range failed {
		fmt.Fprintf(cmd.Stdout, "LANDING-FAILED\t%s\t%s\n", f.Unit, strings.Join(f.Tests, " "))
	}
	if complete {
		fmt.Fprintf(cmd.Stdout, "LANDING-CHECKED\t%d\n", len(failed))
	}
}

func attributionIDs(checks []Result) []string {
	var ids []string
	for _, check := range checks {
		for _, unit := range check.Failed {
			if len(unit.Tests) == 0 {
				ids = append(ids, "red:"+unit.Unit)
			}
			for _, name := range unit.Tests {
				ids = append(ids, "red:"+unit.Unit+":"+name)
			}
		}
	}
	return ids
}

func (b *attributionBed) assert(t *testing.T, r Result, wantRed, wantFlakes []string, cause string) {
	t.Helper()
	if got := attributionIDs(b.checks); !slices.Equal(got, wantRed) {
		t.Fatalf("red identities %v, want %v", got, wantRed)
	}
	if r.Cause == nil || r.Cause.Kind != cause {
		t.Fatalf("cause %+v, want %s", r.Cause, cause)
	}
	if cause == "main" && len(wantRed) > 0 && r.Cause.Name != wantRed[0] {
		t.Fatalf("cause name %q, want %s", r.Cause.Name, wantRed[0])
	}
	var flakes []string
	for _, f := range b.records {
		if f.Where != "main" || f.Attempt != r.Attempt || f.Log != r.Log || f.Repeat != "alone" || f.RepeatAttempt == "" || f.RepeatLog == "" {
			t.Fatalf("flake provenance %+v", f)
		}
		if len(f.Outputs) != len(f.Tests) || len(f.RepeatOutputs) != len(f.Tests) {
			t.Fatalf("incomplete flake outputs %+v", f)
		}
		for _, test := range f.Tests {
			flakes = append(flakes, f.Unit+":"+test)
			first, second := f.Outputs[test], f.RepeatOutputs[test]
			if first.Outcome != "fail" || second.Outcome != "pass" || first.Digest == "" || second.Digest == "" {
				t.Fatalf("flake outcomes %+v %+v", first, second)
			}
			for _, path := range []string{first.Path, second.Path} {
				if _, err := os.ReadFile(path); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if !slices.Equal(flakes, wantFlakes) {
		t.Fatalf("published flakes %v, want %v", flakes, wantFlakes)
	}
	if !reflect.DeepEqual(r.Flakes, b.records) || r.FlakePublished != (len(b.records) > 0) {
		t.Fatalf("retained flakes %+v, publications %+v", r.Flakes, b.records)
	}
	for _, check := range b.checks {
		if check.Log != r.Log {
			t.Fatalf("red log %q, want first-pass log %q", check.Log, r.Log)
		}
	}
}

func TestTrunkRepeatAttributesEachTest(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"mixed", "both-pass-in-unit", "mixed-flake-first", "health", "missing-terminal", "skipped", "listed-but-passed", "incomplete-beside-pass", "lost-process", "record-fails", "nil-record", "other-failure-in-repeat", "all-fail", "all-pass", "old-red-row", "older-red-stays", "scoped-mixed-with-inherited"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			b := newAttributionBed(t, mode)
			r := b.detached(t)
			if mode == "all-pass" {
				if r.Result != Green || !r.RepeatComplete || !r.FlakePublished || len(b.records) != 2 || len(b.checks) != 0 {
					t.Fatalf("all-pass proof %+v, records %+v, checks %+v", r, b.records, b.checks)
				}
				got := normalizedAttribution(t, r, b.checkout)
				want, err := os.ReadFile("testdata/trunk-green-with-flakes-result.json")
				if err != nil {
					t.Fatalf("golden unavailable: %v\n%s", err, got)
				}
				if string(got) != string(want) {
					t.Fatalf("green result changed:\n%s", got)
				}
				return
			}
			if mode == "old-red-row" {
				old := r
				old.FlakePublished = false
				old.Flakes = nil
				for _, unit := range b.first {
					old.Flakes = append(old.Flakes, FlakeRecord{FailedUnit: unit, Attempt: r.Attempt})
				}
				b.save(t, old)
				b.checks, b.records = nil, nil
				b.seams.CommandForCommit = func(string) (string, error) { return "fixture-proof", nil }
				b.seams.Command = func(*exec.Cmd) error { t.Fatal("retained repeat ran again"); return nil }
				var err error
				r, err = ClassifyAttempt(b.install, b.checkout, r.Attempt, b.seams)
				if err != nil {
					t.Fatal(err)
				}
				if got := attributionIDs(b.checks); !slices.Equal(got, []string{"red:u/a:TestA1", "red:u/a:TestA2", "red:u/b:TestB1"}) {
					t.Fatalf("old row lost red identities: %v", got)
				}
				return
			}
			wantRed := []string{"red:u/a:TestA1"}
			wantFlakes := []string{"u/a:TestA2", "u/b:TestB1"}
			calls, cause := 2, "main"
			switch mode {
			case "both-pass-in-unit":
				wantRed = []string{"red:u/b:TestB1"}
				wantFlakes = []string{"u/a:TestA1", "u/a:TestA2"}
				calls = 1
			case "missing-terminal", "skipped", "listed-but-passed", "other-failure-in-repeat":
				wantRed = []string{"red:u/a:TestA1", "red:u/a:TestA2"}
				wantFlakes = []string{"u/b:TestB1"}
				calls = 1
			case "record-fails":
				wantRed = []string{"red:u/a:TestA1", "red:u/b:TestB1"}
				wantFlakes = []string{"u/a:TestA2"}
				calls = 1
			case "nil-record", "all-fail":
				wantRed = []string{"red:u/a:TestA1", "red:u/a:TestA2", "red:u/b:TestB1"}
				wantFlakes = nil
				calls = 0
			case "incomplete-beside-pass", "lost-process":
				wantRed = nil
				wantFlakes = []string{"u/b:TestB1"}
				calls = 1
				cause = "environment"
			}
			if r.Result != Red || r.RepeatComplete {
				t.Fatalf("proof changed verdict or repeat completion: %+v", r)
			}
			if len(b.records) != calls {
				t.Fatalf("flake calls %d, want %d", len(b.records), calls)
			}
			b.assert(t, r, wantRed, wantFlakes, cause)
			if mode == "record-fails" && len(b.publications) != 2 {
				t.Fatalf("publication attempts %d, want 2", len(b.publications))
			}
			if mode == "record-fails" && !strings.Contains(r.Reason, "u/b could not be recorded: publication refused") {
				t.Fatalf("missing publication failure: %s", r.Reason)
			}
			if !slices.Equal(b.repeats, []string{"u/a", "u/b"}) {
				t.Fatalf("repeated units %v", b.repeats)
			}
			first := slices.DeleteFunc(slices.Clone(r.Failed), func(f FailedUnit) bool { return f.Inherited })
			if !reflect.DeepEqual(first, b.first) {
				t.Fatalf("first-pass identities changed: %+v, want %+v", first, b.first)
			}
			last, ok, err := LastResult(b.install)
			if err != nil || !ok || !reflect.DeepEqual(last, r) {
				t.Fatalf("last row %+v differs from result %+v: %v", last, r, err)
			}
			if mode == "scoped-mixed-with-inherited" {
				if r.Scope != "scoped" || len(r.Failed) != 3 || !r.Failed[2].Inherited || r.Failed[2].Unit != "plans" {
					t.Fatalf("scoped proof lost inherited red: %+v", r)
				}
			}
			if mode == "health" || mode == "all-fail" {
				h := newStatusBed(t)
				h.lines("results.jsonl", r)
				s := healthSeams(bedNow)
				s.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) {
					return []goal.TrunkRedEntry{{ID: "a1", TestUnit: "u/a", TestName: "TestA1", Class: goal.TrunkRedClassTrunkRed, Sightings: []goal.TrunkRedSighting{{Attempt: r.Attempt, BaseCommit: r.Commit}}}}, nil
				}
				health := readHealthBed(t, h, s).Health
				if len(health.Incidents) != 1 || health.Incidents[0].Repeat != Red || healthRow(health, "confirmed-flake") != nil {
					t.Fatalf("red repeat health %+v; rows %+v", health.Incidents, health.Rows())
				}
			}
			if mode == "older-red-stays" {
				b.incidents = []goal.TrunkRedEntry{{ID: "older", Identity: "red:u/a:TestA2", Class: goal.TrunkRedClassTrunkRed, Sightings: []goal.TrunkRedSighting{{Attempt: "older", BaseCommit: r.Commit}}}}
				// The next tree changes only shared records, leaving the open failure standing.
				pkg := b.contract.Groups[0]
				pkg.ID = "u/a"
				pkg.Inputs = []string{"metasystem/records/a/**"}
				b.contract.Groups = append(b.contract.Groups, pkg)
				b.git.tree, b.git.commit = "next-tree", "next-commit"
				b.git.changed[[2]string{r.Tree, b.git.tree}] = "metasystem/records/shared/next.md"
				b.setContract(t)
				b.seams.NewID = func() string { return "next-attempt" }
				b.seams.Command = func(cmd *exec.Cmd) error {
					fmt.Fprintln(cmd.Stdout, "landing environment "+b.environment+"\nlanding group shared green 1\nLANDING-CHECKED\t0")
					return nil
				}
				b.checks = nil
				next := b.detached(t)
				if next.Scope != "scoped" || !slices.ContainsFunc(next.Failed, func(f FailedUnit) bool {
					return f.Inherited && f.Unit == "u/a" && slices.Equal(f.Tests, []string{"TestA2"})
				}) || len(b.checks) != 0 || b.incidents[0].Closed != nil {
					t.Fatalf("older red did not stay standing: %+v, checks %+v, incidents %+v", next, b.checks, b.incidents)
				}
			}
		})
	}
}

func normalizedAttribution(t *testing.T, r Result, root string) []byte {
	t.Helper()
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	var normalize func(any) any
	normalize = func(v any) any {
		switch x := v.(type) {
		case map[string]any:
			for key, item := range x {
				switch key {
				case "at", "started_at", "since", "fullAt", "owedSince":
					x[key] = "<time>"
				case "minutes", "load", "Load", "pid":
					x[key] = 0
				case "load_start", "load_end":
					x[key] = map[string]any{}
				default:
					x[key] = normalize(item)
				}
			}
		case []any:
			for i := range x {
				x[i] = normalize(x[i])
			}
		case string:
			if strings.HasPrefix(x, "pid=") {
				return "<process>"
			}
			return strings.ReplaceAll(x, root, "<root>")
		}
		return v
	}
	data, err = json.MarshalIndent(normalize(value), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(data, '\n')
}

func TestMainPrefixRepeatAttributesEachTest(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"batch", "gate"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			b := newAttributionBed(t, "mixed")
			if err := os.MkdirAll(filepath.Join(Dir(b.install), "proofs"), 0755); err != nil {
				t.Fatal(err)
			}
			git := b.seams.Git
			b.seams.Git = func(dir string, args ...string) (string, error) {
				switch strings.Join(args, " ") {
				case "rev-parse --verify --quiet refs/remotes/origin/main^{commit}":
					return "prefix", nil
				case "rev-parse --verify prefix^{tree}":
					return "prefix-tree", nil
				case "show -s --format=%P batch-tip":
					return "prefix goal-tip", nil
				}
				return git(dir, args...)
			}
			running := Running{Attempt: "batch", Commit: "batch-tip", Tree: "batch-tree"}
			red := Result{Result: Red, Commit: running.Commit, Tree: running.Tree, Attempt: running.Attempt, Failed: b.first, Cause: &Cause{Kind: "unclassified"}, Goals: []GoalSHA{{Goal: "g", SHA: "goal-tip"}}}
			var r Result
			if mode == "gate" {
				r = replayGate(b.seams, b.install, b.checkout, "fixture-proof", running, red, Result{})
			} else {
				r = classifyReplay(b.seams, b.install, b.checkout, "fixture-proof", running, red, Result{}, []replayTree{{Running: Running{Commit: "prefix"}, Main: true}})
			}
			if r.Cause == nil || r.Cause.Kind != "main" || r.Cause.Name != "red:u/a:TestA1" || r.Result != Red {
				t.Fatalf("prefix attribution %+v", r)
			}
			if got := attributionIDs(b.checks); !slices.Equal(got, []string{"red:u/a:TestA1"}) {
				t.Fatalf("prefix red identities %v", got)
			}
			if len(b.checks) != 1 || b.checks[0].Tree != "prefix-tree" || b.checks[0].Commit != "prefix" {
				t.Fatalf("prefix coordinates %+v", b.checks)
			}
			check := b.checks[0]
			b.assert(t, check, []string{"red:u/a:TestA1"}, []string{"u/a:TestA2", "u/b:TestB1"}, "unclassified")
			if len(b.records) != 2 {
				t.Fatalf("prefix flake calls %d", len(b.records))
			}
			if !slices.Equal(b.repeats, []string{"u/a", "u/b"}) {
				t.Fatalf("prefix repeats %v", b.repeats)
			}
		})
	}
}
