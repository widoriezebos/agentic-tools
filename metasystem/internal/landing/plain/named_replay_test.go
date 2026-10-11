package plain

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestReplayParsesRealRedAndUsesItsNamedSelection(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("testdata/named-replay-red.json")
	if err != nil {
		t.Fatal(err)
	}
	var red Result
	if err := json.Unmarshal(data, &red); err != nil {
		t.Fatal(err)
	}
	if red.Attempt != "20261011T111952.309251000Z" || len(red.Failed) != 1 || !reflect.DeepEqual(red.Failed[0].Tests, []string{"TestReadStatusTrunkRedWaitsForRunningTrunkProof"}) {
		t.Fatalf("real red=%+v", red)
	}
	// The red and goal snapshots preserve the installation's persisted shapes.
	ledger, err := os.ReadFile("testdata/named-replay-goal.md")
	if err != nil {
		t.Fatal(err)
	}
	record, problems := goal.ParseFile(ledger)
	if len(problems) != 0 || record.Id != red.BatchMembers[0].Goal {
		t.Fatalf("real goal=%+v problems=%v", record, problems)
	}
	b := newRepeatBed(t)
	if err := os.MkdirAll(filepath.Join(Dir(b.install), "proofs"), 0755); err != nil {
		t.Fatal(err)
	}
	reporter := filepath.Join(t.TempDir(), "reporter")
	build := testenv.Go("build", "-buildvcs=false", "-o", reporter, "../../../proof")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build reporter: %v\n%s", err, output)
	}
	contract, err := os.ReadFile("testdata/testing.json")
	if err != nil {
		t.Fatal(err)
	}
	git := b.seams.Git
	b.seams.Git = func(dir string, args ...string) (string, error) {
		out, err := git(dir, args...)
		if err == nil && len(args) == 5 && args[0] == "worktree" && args[1] == "add" {
			module := filepath.Join(args[3], "metasystem")
			files := map[string][]byte{
				"testing.json":                        contract,
				"plans/goals/" + record.Id + ".md":    ledger,
				"go.mod":                              []byte("module example.com/replay\n\ngo 1.23\n"),
				"internal/landing/plain/base_test.go": []byte("package plain\nimport \"testing\"\nfunc TestBase(t *testing.T) { t.Parallel() }\n"),
			}
			for name, data := range files {
				path := filepath.Join(module, name)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
		}
		return out, err
	}
	calls := 0
	b.seams.Command = func(cmd *exec.Cmd) error {
		calls++
		want := red.Failed[0].Tests[0]
		if calls == 2 {
			want = ""
		}
		if calls > 2 || commandEnv(cmd, "LANDING_ONLY") != red.Failed[0].Unit || commandEnv(cmd, "LANDING_ONLY_TESTS") != want {
			t.Fatalf("replay call %d unit=%q names=%q", calls, commandEnv(cmd, "LANDING_ONLY"), commandEnv(cmd, "LANDING_ONLY_TESTS"))
		}
		child := exec.Command(reporter, "--host")
		child.Dir, child.Stdout, child.Stderr = cmd.Dir, cmd.Stdout, cmd.Stderr
		child.Env = proofrun.TestingEnvironment(cmd.Env, map[string]string{"GOWORK": "off", "GOTOOLCHAIN": "local", "GOPROXY": "off", "GOSUMDB": "off", "METASYSTEM_TESTING_WORKERS": "9"})
		return child.Run()
	}
	result := classifyReplay(b.seams, b.install, b.checkout, "fixture", Running{Attempt: "named", Trunk: true, Commit: red.Commit}, red, Result{}, []replayTree{{Running: Running{Commit: "commit"}, Main: true}, {Running: Running{Commit: red.Commit}, Goal: red.BatchMembers[0]}})
	if calls != 2 || result.Replay != "named replay green on main: whole-unit replay ran; replay: whole unit metasystem/internal/landing/plain" || result.Cause.Kind != "own" || result.Cause.Goal != record.Id {
		t.Fatalf("real red classification=%+v cause=%+v calls=%d", result, result.Cause, calls)
	}
	log, err := os.ReadFile(filepath.Join(Dir(b.install), "proofs/named-replay-1.log"))
	if err != nil || !strings.Contains(string(log), "landing planned 1\n") || !strings.Contains(string(log), "landing dropped "+red.Failed[0].Tests[0]) || !strings.Contains(string(log), `"Test":"TestBase"`) || strings.Contains(string(log), "LANDING-FAILED") {
		t.Fatalf("real red plan=%s err=%v", log, err)
	}

	t.Logf("real red cause=%s; reporter plan and output:\n%s", result.Cause.Kind, log)
}

func TestReplayFallsBackBeforeAttributingNamedResults(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, unit, test, form, whole string
		calls                         int
	}{
		{"incomplete", "metasystem/internal/p", "TestP", "named replay incomplete: whole unit", "red", 2},
		{"green main", "metasystem/internal/p", "TestP", "named replay green on main: whole-unit replay ran", "red", 2},
		{"green goal", "metasystem/internal/p", "TestP", "named replay green on prefix: whole-unit replay ran", "red", 2},
		{"missing token", "metasystem/internal/p", "TestP(did not report)", "named replay invalid test identity: whole unit", "red", 1},
		{"subtest", "metasystem/internal/p", "TestP/subtest", "named replay invalid test identity: whole unit", "red", 1},
		{"command group", "fast-static-build", "TestP", "replay: whole unit fast-static-build", "red", 1},
		{"go group", "go-batchtest", "TestP", "replay: whole unit go-batchtest", "red", 1},
		{"section group", "section/go-engine-gate", "TestP", "replay: whole unit section/go-engine-gate", "red", 1},
		{"whole incomplete", "metasystem/internal/p", "TestP", "named replay incomplete: whole unit", "incomplete", 2},
		{"whole green", "metasystem/internal/p", "TestP", "named replay green on main: whole-unit replay ran", "green", 2},
		{"named red", "metasystem/internal/p", "TestP", "replay: 1 named tests of metasystem/internal/p", "red", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newRepeatBed(t)
			if err := os.MkdirAll(filepath.Join(Dir(b.install), "proofs"), 0755); err != nil {
				t.Fatal(err)
			}
			calls, removals := 0, 0
			git := b.seams.Git
			b.seams.Git = func(dir string, args ...string) (string, error) {
				if len(args) > 1 && args[0] == "worktree" && args[1] == "remove" {
					removals++
				}
				return git(dir, args...)
			}
			b.seams.Command = func(cmd *exec.Cmd) error {
				calls++
				if commandEnv(cmd, "LANDING_ONLY") != tc.unit {
					t.Fatalf("unit=%q", commandEnv(cmd, "LANDING_ONLY"))
				}
				named := commandEnv(cmd, "LANDING_ONLY_TESTS") != ""
				if named {
					if calls != 1 || tc.calls == 1 && tc.name != "named red" {
						t.Fatalf("invalid named replay")
					}
					if tc.name == "incomplete" || tc.name == "whole incomplete" {
						fmt.Fprint(cmd.Stdout, "LANDING-NOT-RUN\tenvironment\n")
						return fmt.Errorf("lost")
					}
					if tc.name != "named red" {
						fmt.Fprint(cmd.Stdout, "LANDING-CHECKED\t0\n")
						return nil
					}
				} else if tc.calls == 2 && calls != 2 {
					t.Fatalf("whole-unit fallback skipped named replay")
				}
				if tc.whole == "incomplete" {
					fmt.Fprint(cmd.Stdout, "LANDING-NOT-RUN\tenvironment\n")
					return fmt.Errorf("lost")
				}
				if tc.whole == "green" {
					fmt.Fprint(cmd.Stdout, "LANDING-CHECKED\t0\n")
					return nil
				}
				fmt.Fprintf(cmd.Stdout, "LANDING-FAILED\t%s\tTestOrder\nLANDING-CHECKED\t1\n", tc.unit)
				return fmt.Errorf("red")
			}
			var incidents []Result
			b.seams.RecordMain = func(checks []Result) error { incidents = append(incidents, checks...); return nil }
			red := Result{Result: Red, Failed: []FailedUnit{{Unit: tc.unit, Tests: []string{tc.test}}}, Cause: &Cause{Kind: "unclassified"}}
			prefix := replayTree{Running: Running{Commit: "commit"}, Main: tc.name != "green goal", Goal: GoalSHA{Goal: "g", SHA: "sha-g"}}
			result := classifyReplay(b.seams, b.install, b.checkout, "fixture", Running{Attempt: "fallback", Trunk: true}, red, Result{}, []replayTree{prefix})
			want := "main"
			if tc.name == "green goal" {
				want = "unclassified"
			}
			if tc.whole == "incomplete" || tc.whole == "green" {
				want = "unclassified"
			}
			if calls != tc.calls || removals != 1 || !strings.Contains(result.Replay, tc.form) || result.Cause.Kind != want {
				t.Fatalf("result=%+v cause=%+v calls=%d removals=%d", result, result.Cause, calls, removals)
			}
			if want == "main" && (len(incidents) != 1 || incidents[0].Failed[0].Tests[0] != "TestOrder") {
				t.Fatalf("main incident=%+v", incidents)
			}
			if tc.whole == "incomplete" && result.Repeat != "" {
				t.Fatalf("incomplete fallback allowed repeat: %+v", result)
			}
		})
	}
}

func TestReplayKeepsFallbackFormAcrossPrefixes(t *testing.T) {
	t.Parallel()
	b := newRepeatBed(t)
	if err := os.MkdirAll(filepath.Join(Dir(b.install), "proofs"), 0755); err != nil {
		t.Fatal(err)
	}
	git := b.seams.Git
	b.seams.Git = func(dir string, args ...string) (string, error) {
		if strings.Join(args, " ") == "rev-parse --verify merge-g^{tree}" {
			return "merge-tree", nil
		}
		return git(dir, args...)
	}
	calls := 0
	b.seams.Command = func(cmd *exec.Cmd) error {
		calls++
		if commandEnv(cmd, "LANDING_COMMIT") == "commit" {
			fmt.Fprint(cmd.Stdout, "LANDING-CHECKED\t0\n")
			return nil
		}
		if commandEnv(cmd, "LANDING_ONLY_TESTS") != "TestP" {
			t.Fatal("later prefix omitted named test")
		}
		fmt.Fprint(cmd.Stdout, "LANDING-FAILED\tmetasystem/internal/p\tTestP\nLANDING-CHECKED\t1\n")
		return fmt.Errorf("red")
	}
	red := Result{Result: Red, Failed: []FailedUnit{{Unit: "metasystem/internal/p", Tests: []string{"TestP"}}}, Cause: &Cause{Kind: "unclassified"}}
	trees := []replayTree{{Running: Running{Commit: "commit"}, Main: true}, {Running: Running{Commit: "merge-g"}, Goal: GoalSHA{Goal: "g", SHA: "sha-g"}}}
	result := classifyReplay(b.seams, b.install, b.checkout, "fixture", Running{Attempt: "prefixes", Trunk: true}, red, Result{}, trees)
	if calls != 3 || result.Cause.Kind != "own" || result.Cause.Goal != "g" || !strings.Contains(result.Replay, "named replay green on main: whole-unit replay ran") || !strings.Contains(result.Replay, "replay: 1 named tests of metasystem/internal/p") {
		t.Fatalf("result=%+v cause=%+v calls=%d", result, result.Cause, calls)
	}
}
