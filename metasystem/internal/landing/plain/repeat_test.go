package plain

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

const failedReport = "printf 'LANDING-FAILED\tu/a\tTestA TestB\nLANDING-FAILED\tu/b\tTestC\nLANDING-LOAD\t2.75\nLANDING-CHECKED\t2\n'; exit 1"

// repeatBed keeps Git and time synthetic while the check uses a real shell.
type repeatBed struct {
	t                        *testing.T
	checkout, install, trace string
	seams                    ProveSeams
	records                  []FlakeRecord
}

func newRepeatBed(t *testing.T) *repeatBed {
	t.Helper()
	b := &repeatBed{t: t, checkout: t.TempDir()}
	var err error
	b.checkout, err = filepath.EvalSymlinks(b.checkout)
	if err != nil {
		t.Fatal(err)
	}
	b.install, b.trace = filepath.Join(b.checkout, "metasystem"), filepath.Join(b.checkout, "checks")
	g := stubGit{commit: "commit", tree: "tree"}
	id := 0
	b.seams = ProveSeams{Now: func() time.Time { return bedNow }, NewID: func() string { id++; return fmt.Sprintf("a%d", id) },
		Git: func(dir string, args ...string) (string, error) {
			switch strings.Join(args, " ") {
			case "rev-parse --verify --quiet refs/remotes/origin/main^{commit}":
				return "parent", nil
			case "rev-parse --verify commit^{tree}":
				return "tree", nil
			case "log --first-parent --merges --reverse --format=%H %P origin/main..commit":
				return "", nil
			}
			out, err := g.run(dir, args...)
			if err == nil && len(args) == 5 && args[0] == "worktree" && args[1] == "add" {
				err = os.MkdirAll(filepath.Join(args[3], "metasystem"), 0o755)
			}
			return out, err
		},
		Executable: func() (string, error) { return "engine", nil },
		Launch: func([]string, string, string) (int64, error) {
			t.Fatal("a refused or green tree launched a check")
			return 0, nil
		},
		Judge: func(checkout, commit string, failed []FailedUnit) (map[string]UnitJudgement, error) {
			if checkout != b.checkout || commit != "commit" || len(failed) != 2 || !reflect.DeepEqual(failed[0].Tests, []string{"TestA", "TestB"}) {
				t.Errorf("judge input: %s %s %+v", checkout, commit, failed)
			}
			return map[string]UnitJudgement{"u/a": {Known: true, Surfaces: []string{"surface-a"}}, "u/b": {Known: true, Surfaces: []string{"surface-b"}}}, nil
		},
		RecordFlake: func(r FlakeRecord) (FlakeRecorded, error) {
			b.records = append(b.records, r)
			return FlakeRecorded{Goal: "fix-flaky-" + strings.ReplaceAll(r.Unit, "/", "-"), Seen: 3}, nil
		},
	}
	return b
}

func (b *repeatBed) run(command string) Result {
	b.t.Helper()
	if err := os.MkdirAll(filepath.Join(Dir(b.install), "proofs"), 0o755); err != nil {
		b.t.Fatal(err)
	}
	log, err := os.Create(filepath.Join(Dir(b.install), "proofs", fmt.Sprintf("check-%d.log", len(b.lines()))))
	if err != nil {
		b.t.Fatal(err)
	}
	defer log.Close()
	command = fmt.Sprintf("printf '%%s|%%s|%%s|%%s\\n' \"$LANDING_ONLY\" \"$PWD\" \"$LANDING_TREE\" \"$LANDING_COMMIT\" >> %q; ", b.trace) + command
	result, err := Run(b.install, b.checkout, command, "", log, b.seams)
	if err != nil {
		b.t.Fatal(err)
	}
	return result
}

func (b *repeatBed) lines() []Result {
	b.t.Helper()
	lines, err := Results(b.install)
	if err != nil {
		b.t.Fatal(err)
	}
	return lines
}

// A spent repeat holds the goals; a red alone cannot authorize their return.
func (b *repeatBed) refused() {
	b.t.Helper()
	entries := []func() error{
		func() error { _, err := Run(b.install, b.checkout, "exit 0", "", io.Discard, b.seams); return err },
		func() error { _, _, err := Start(b.install, b.checkout, b.seams); return err },
		func() error { _, _, err := Settled(b.install, b.checkout, b.seams); return err },
	}
	for i, entry := range entries {
		err := entry()
		var refused *NoRepeat
		if !errors.As(err, &refused) || err.Error() != "this code failed its check and gets no other; the waiting goals hold" {
			b.t.Errorf("entry %d: want typed repeat refusal, got %v", i, err)
		}
	}
}

func TestRepeatMissingEngineIsEnvironment(t *testing.T) {
	t.Parallel()
	b := newRepeatBed(t)
	for i := range 2 {
		red := b.run(filepath.Join(b.checkout, "missing-engine"))
		if red.Result != Red || red.Cause == nil || red.Cause.Kind != "environment" || red.CountedFull || red.Reason != "the proving command exited 127" {
			t.Fatalf("missing engine consumed a full check or lost its environment cause: %+v", red)
		}
		if (red.Repeat == "allowed") != (i == 0) {
			t.Fatalf("missing engine has the wrong repeat allowance on attempt %d: %+v", i+1, red)
		}
	}
}

func TestRepeatNoTestsRan(t *testing.T) {
	t.Parallel()
	b := newRepeatBed(t)
	red := b.run("printf 'LANDING-NOT-RUN\tdisk full\n'; exit 2")
	if red.Result != Red || red.Repeat != "allowed" || len(red.Failed) != 0 {
		t.Fatalf("no tests ran: %+v", red)
	}
	// A detached start must still consume the same allowance in Run.
	b.seams.Launch = func([]string, string, string) (int64, error) { return int64(os.Getpid()), nil }
	b.seams.Alive = func(Running) bool { return true }
	running, _, err := Start(b.install, b.checkout, b.seams)
	if err != nil {
		t.Fatal(err)
	}
	green, err := Run(b.install, b.checkout, "exit 0", running.Attempt, io.Discard, b.seams)
	if err != nil || green.Result != Green || len(b.records) != 0 {
		t.Fatalf("whole repeat: %+v %v records %+v", green, err, b.records)
	}
	if lines := b.lines(); len(lines) != 4 || !lines[0].ClassificationPending || lines[2].Repeat != "started" {
		t.Fatalf("allowance was not consumed: %+v", lines)
	}
}

func TestRepeatDeadCheck(t *testing.T) {
	t.Parallel()
	for _, entry := range []string{"run", "start", "settled"} {
		t.Run(entry, func(t *testing.T) {
			t.Parallel()
			b := newRepeatBed(t)
			b.seams.Alive = func(Running) bool { return false }
			if err := withLock(b.install, func() error { return writeRunning(b.install, Running{Attempt: "dead", Tree: "tree", Commit: "commit"}) }); err != nil {
				t.Fatal(err)
			}
			if entry == "start" {
				b.seams.Launch = func([]string, string, string) (int64, error) { return int64(os.Getpid()), nil }
				if _, _, err := Start(b.install, b.checkout, b.seams); err != nil {
					t.Fatal(err)
				}
				b.seams.Alive = func(Running) bool { return true }
				if green, err := Run(b.install, b.checkout, "exit 0", "a1", io.Discard, b.seams); err != nil || green.Result != Green {
					t.Fatalf("detached repeat: %+v %v", green, err)
				}
			} else {
				if entry == "settled" {
					if _, ok, err := Settled(b.install, b.checkout, b.seams); err != nil || ok {
						t.Fatalf("settled: %v %v", ok, err)
					}
				}
				if green := b.run("exit 0"); green.Result != Green {
					t.Fatal(green)
				}
			}
			lines := b.lines()
			if len(lines) != 3 || lines[0].Reason != "the lane's check stopped before it ended" || lines[0].Repeat != "allowed" || len(lines[0].Failed) != 0 || lines[1].Repeat != "started" || len(b.records) != 0 {
				t.Fatalf("dead check: %+v records %+v", lines, b.records)
			}
			if _, err := os.Stat(runningPath(b.install)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("running record remains: %v", err)
			}
		})
	}
}

func TestRepeatDetachedUnreadableIdentityKeepsAllowance(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"full", "gate", "trunk"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			for _, zero := range []bool{false, true} {
				t.Run(fmt.Sprintf("zero pid %v", zero), func(t *testing.T) {
					t.Parallel()
					b := newRepeatBed(t)
					b.seams.Gate, b.seams.Trunk = mode == "gate", mode == "trunk"
					git := b.seams.Git
					b.seams.Git = func(dir string, args ...string) (string, error) {
						if strings.Join(args, " ") == "rev-parse --verify origin/main^{commit}" {
							return "commit", nil
						}
						return git(dir, args...)
					}
					// An environment failure has left the one repeat available.
					before := Result{Tree: "tree", Commit: "commit", Result: Red, Repeat: "allowed", Attempt: "first"}
					if err := withLock(b.install, func() error { return appendLine(b.seams.resultsPath(b.install), before) }); err != nil {
						t.Fatal(err)
					}
					pid := int64(0)
					if !zero {
						child := exec.Command("/usr/bin/true")
						if err := child.Run(); err != nil {
							t.Fatal(err)
						}
						pid = int64(child.Process.Pid)
						if processRef(pid) != "" {
							t.Fatal("exited child still has a readable identity")
						}
					}
					b.seams.Launch = func([]string, string, string) (int64, error) { return pid, nil }
					running, already, err := Start(b.install, b.checkout, b.seams)
					retry := "metasystem landing prove"
					if mode != "full" {
						retry += " --" + mode
					}
					if err == nil || !strings.Contains(err.Error(), "environment") || !strings.HasSuffix(err.Error(), "retry: "+retry) || already || running.Admission == nil || running.Admission.State != "failed" {
						t.Errorf("unreadable launch was not refused with its retry: %+v already=%v err=%v", running, already, err)
					}
					failed, recorded, alive, readErr := ReadRunning(b.install, b.seams)
					if readErr != nil || !recorded || alive || failed.Admission == nil || failed.Admission.State != "failed" {
						t.Errorf("unreadable launch lost its failed admission: %+v %v", failed, readErr)
					}
					lines, err := readLines[Result](b.seams.resultsPath(b.install))
					if err != nil || !reflect.DeepEqual(lines, []Result{before}) {
						t.Fatalf("launch failure changed the repeat allowance: %+v err=%v", lines, err)
					}
				})
			}
		})
	}
}

func TestRepeatDetachedOwnAttemptKeepsAllowance(t *testing.T) {
	t.Parallel()
	b := newRepeatBed(t)
	b.seams.Launch = func([]string, string, string) (int64, error) { return int64(os.Getpid()), nil }
	b.seams.Alive = func(Running) bool { return false }
	running, _, err := Start(b.install, b.checkout, b.seams)
	if err != nil {
		t.Fatal(err)
	}
	if running.Process == "" || running.Pid != int64(os.Getpid()) {
		t.Fatalf("expected this process's detached attempt: %+v", running)
	}
	command := fmt.Sprintf("printf 'check\\n' >> %q; printf 'LANDING-NOT-RUN\\tdisk full\\n'; exit 2", b.trace)
	red, err := Run(b.install, b.checkout, command, running.Attempt, io.Discard, b.seams)
	if err != nil {
		t.Fatal(err)
	}
	trace, err := os.ReadFile(b.trace)
	if err != nil || string(trace) != "check\n" {
		t.Fatalf("detached check did not run once: %q %v", trace, err)
	}
	if lines := b.lines(); len(lines) != 2 || !lines[0].ClassificationPending || !reflect.DeepEqual(lines[1], red) || red.Attempt != running.Attempt || red.Result != Red || red.Repeat != "allowed" || red.Reason != "the proving command exited 2" {
		t.Fatalf("own attempt consumed its allowance: %+v", lines)
	}
	if green := b.run("exit 0"); green.Result != Green || len(b.records) != 0 {
		t.Fatalf("whole repeat: %+v records %+v", green, b.records)
	}
	if lines := b.lines(); len(lines) != 4 || !lines[0].ClassificationPending || lines[2].Repeat != "started" {
		t.Fatalf("whole repeat did not consume the allowance: %+v", lines)
	}
}

func TestRepeatCompletedDeadCheckKeepsGreen(t *testing.T) {
	t.Parallel()
	for _, entry := range []string{"run", "start", "settled"} {
		t.Run(entry, func(t *testing.T) {
			t.Parallel()
			b := newRepeatBed(t)
			b.seams.Alive = func(Running) bool { return false }
			green := Result{Tree: "tree", Commit: "commit", Result: Green, Attempt: "done", Log: "check.log", At: bedNow.Format(time.RFC3339)}
			other := Result{Tree: "other", Commit: "other", Result: Green, Attempt: "other"}
			if err := withLock(b.install, func() error {
				for _, result := range []Result{green, other} {
					if err := appendLine(resultsPath(b.install), result); err != nil {
						return err
					}
				}
				return writeRunning(b.install, Running{Tree: green.Tree, Commit: green.Commit, Attempt: green.Attempt})
			}); err != nil {
				t.Fatal(err)
			}
			switch entry {
			case "run":
				got, err := Run(b.install, b.checkout, "exit 7", "", io.Discard, b.seams)
				if err != nil || !reflect.DeepEqual(got, green) {
					t.Fatalf("completed run: %+v %v", got, err)
				}
			case "start":
				got, already, err := Start(b.install, b.checkout, b.seams)
				if err != nil || !already || got.Attempt != green.Attempt || got.Tree != green.Tree || got.Commit != green.Commit || got.Log != green.Log || got.Since != green.At {
					t.Fatalf("completed start: %+v %v %v", got, already, err)
				}
			case "settled":
				got, found, err := Settled(b.install, b.checkout, b.seams)
				if err != nil || !found || !reflect.DeepEqual(got, green) {
					t.Fatalf("completed settled: %+v %v %v", got, found, err)
				}
			}
			if lines := b.lines(); !reflect.DeepEqual(lines, []Result{green, other}) {
				t.Fatalf("completed attempt appended a result: %+v", lines)
			}
			if _, err := os.Stat(runningPath(b.install)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("completed running record remains: %v", err)
			}
		})
	}
}

func TestRepeatIncompleteReportRefusesEveryEntry(t *testing.T) {
	t.Parallel()
	for name, report := range map[string]string{"absent": "exit 1", "count": "printf 'LANDING-FAILED\tu/a\tTestA\nLANDING-CHECKED\t2\n'; exit 1", "not last": "printf 'LANDING-CHECKED\t0\nmore output\n'; exit 1"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newRepeatBed(t)
			b.seams.Judge = func(string, string, []FailedUnit) (map[string]UnitJudgement, error) {
				t.Error("incomplete report judged")
				return nil, nil
			}
			if red := b.run(report); red.Result != Red || red.Repeat != "" {
				t.Fatalf("incomplete: %+v", red)
			}
			b.refused()
		})
	}
}

func TestRepeatCannotJudgeRefuses(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"affected", "error", "nil", "missing"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newRepeatBed(t)
			b.seams.Judge = func(string, string, []FailedUnit) (map[string]UnitJudgement, error) {
				if name == "error" {
					return nil, errors.New("cannot read contract")
				}
				if name == "missing" {
					return nil, nil
				}
				return map[string]UnitJudgement{"u/a": {Affected: true}, "u/b": {}}, nil
			}
			if name == "nil" {
				b.seams.Judge = nil
			}
			if red := b.run(failedReport); red.Result != Red || red.Repeat != "" {
				t.Fatalf("cannot judge: %+v", red)
			}
			b.refused()
		})
	}
}

func TestRepeatKnownUnitsAloneAndRecordsBeforeGreen(t *testing.T) {
	t.Parallel()
	b := newRepeatBed(t)
	record := b.seams.RecordFlake
	b.seams.RecordFlake = func(r FlakeRecord) (FlakeRecorded, error) {
		recorded, err := record(r)
		if r.Unit == "u/a" {
			recorded.Seen = 1
		}
		return recorded, err
	}
	command := fmt.Sprintf("if [ -z \"$LANDING_ONLY\" ]; then %s; fi; /usr/bin/grep -q '\"repeat\":\"started\"' %q", failedEvidenceReport(), resultsPath(b.install))
	command += "; " + passingEvidenceReport()
	green := b.run(command)
	if green.Result != Green || len(b.records) != 2 {
		t.Fatalf("unit repeats: %+v records %+v", green, b.records)
	}
	trace, _ := os.ReadFile(b.trace)
	worktree := filepath.Join(proofTrees(b.install), "a1", "metasystem")
	want := ""
	for _, unit := range []string{"", "u/a", "u/b"} {
		want += unit + "|" + worktree + "|tree|commit\n"
	}
	if string(trace) != want {
		t.Fatalf("checks: %s, want %s", trace, want)
	}
	if _, err := os.Stat(worktree); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("worktree remains: %v", err)
	}
	for i, r := range b.records {
		if len(r.Outputs) != len(r.Tests) || len(r.RepeatOutputs) != len(r.Tests) || r.Repeat != "alone" || r.Load != 2.75 || r.Attempt != "a1" || r.Tree != "tree" || r.Commit != "commit" || r.Log != green.Log || r.RepeatAttempt == r.Attempt || r.RepeatLog == r.Log || !reflect.DeepEqual(r.Surfaces, []string{fmt.Sprintf("surface-%c", 'a'+i)}) {
			t.Fatalf("sighting: %+v", r)
		}
		if _, err := os.Stat(r.RepeatLog); err != nil {
			t.Fatalf("repeat log: %v", err)
		}
		clause := r.Unit + " failed once and passed when run again alone; seen " + []string{"once", "3 times"}[i] + "; goal fix-flaky-" + strings.ReplaceAll(r.Unit, "/", "-") + " fixes it"
		if !strings.Contains(green.Reason, clause) {
			t.Fatalf("reason: %s", green.Reason)
		}
	}
	if lines := b.lines(); len(lines) != 4 || !lines[0].ClassificationPending || lines[0].Result != Red || lines[1].Repeat != "started" {
		t.Fatalf("results: %+v", lines)
	}
}

func TestRepeatNewTestAllowsOneWholeCheck(t *testing.T) {
	t.Parallel()
	b := newRepeatBed(t)
	judge := b.seams.Judge
	b.seams.Judge = func(c, s string, f []FailedUnit) (map[string]UnitJudgement, error) {
		m, e := judge(c, s, f)
		j := m["u/a"]
		j.Known = false
		m["u/a"] = j
		return m, e
	}
	// A whole repeat requires isolated greens on every replay tree.
	red := b.run("if [ -n \"$LANDING_ONLY\" ]; then printf 'LANDING-CHECKED\\t0\\n'; exit 0; fi; " + failedEvidenceReport())
	if red.Result != Red || red.Repeat != "allowed" || len(b.records) != 0 {
		t.Fatalf("new test: %+v records %+v", red, b.records)
	}
	green := b.run(fmt.Sprintf("/usr/bin/grep -q '\"repeat\":\"started\"' %q; %s", resultsPath(b.install), passingEvidenceReport()))
	if green.Result != Green || len(b.records) != 2 {
		t.Fatalf("whole repeat: %+v records %+v", green, b.records)
	}
	for _, r := range b.records {
		if len(r.Outputs) != len(r.Tests) || len(r.RepeatOutputs) != len(r.Tests) || r.Repeat != "whole" || r.Attempt != red.Attempt || r.RepeatAttempt != green.Attempt || r.Log != red.Log || len(r.Surfaces) != 1 || r.Load != 2.75 || !strings.Contains(green.Reason, "again in a whole check") {
			t.Fatalf("whole sighting: %+v reason %s", r, green.Reason)
		}
	}
	if lines := b.lines(); len(lines) != 4 || !lines[0].ClassificationPending || lines[2].Repeat != "started" {
		t.Fatalf("results: %+v", lines)
	}
}

func TestRepeatKnownRedWholeCheckFailureReplaysOnMain(t *testing.T) {
	t.Parallel()
	b := newRepeatBed(t)
	failed := []FailedUnit{{Unit: "u/a", Tests: []string{"TestA"}}}
	previous := Result{Tree: "tree", Commit: "commit", Result: Red, Repeat: "allowed", Attempt: "first", Failed: failed,
		Cause: &Cause{Kind: "unclassified", Tests: failingTests(failed), Evidence: "first.log"}}
	if err := withLock(b.install, func() error { return appendLine(resultsPath(b.install), previous) }); err != nil {
		t.Fatal(err)
	}
	judgements := 0
	b.seams.Judge = func(checkout, commit string, units []FailedUnit) (map[string]UnitJudgement, error) {
		judgements++
		if checkout != b.checkout || commit != "commit" || !reflect.DeepEqual(units, failed) {
			t.Fatalf("judge input: %s %s %+v", checkout, commit, units)
		}
		return map[string]UnitJudgement{"u/a": {Known: true}}, nil
	}
	var calls []*exec.Cmd
	b.seams.Command = func(cmd *exec.Cmd) error {
		calls = append(calls, cmd)
		fmt.Fprint(cmd.Stdout, plainTestEvents("u/a", []string{"TestA"}, "fail"), "LANDING-FAILED\tu/a\tTestA\nLANDING-CHECKED\t1\n")
		return errors.New("red")
	}
	red := b.run("fixture")
	if red.Result != Red || red.Cause == nil || red.Cause.Kind != "main" || red.Cause.Name != "red:u/a:TestA" || red.Repeat != "started" || len(red.FlakeRepeats) != 0 || len(b.records) != 0 {
		t.Fatalf("whole repeat lost main attribution: %+v cause=%+v records=%+v", red, red.Cause, b.records)
	}
	if judgements != 1 || len(calls) != 2 {
		t.Fatalf("want one judgement, one whole repeat and one main replay: judgements=%d calls=%d", judgements, len(calls))
	}
	for i, only := range []string{"", "u/a"} {
		commit := "commit"
		if i == 1 {
			commit = "parent"
		}
		attempt := "a1"
		if i == 1 {
			attempt += "-replay-1"
		}
		wantDir := filepath.Join(proofTrees(b.install), attempt, "metasystem")
		if commandEnv(calls[i], "LANDING_ONLY") != only || commandEnv(calls[i], "LANDING_COMMIT") != commit || calls[i].Dir != wantDir {
			t.Fatalf("check %d: only=%q commit=%q dir=%q; want only=%q commit=commit dir=%q", i, commandEnv(calls[i], "LANDING_ONLY"), commandEnv(calls[i], "LANDING_COMMIT"), calls[i].Dir, only, wantDir)
		}
	}
	lines := b.lines()
	if len(lines) != 4 || lines[1].Repeat != "started" || !reflect.DeepEqual(lines[len(lines)-1], red) {
		t.Fatalf("whole repeat and its attributed result were not recorded: %+v", lines)
	}
	b.refused()
}

func TestFlakeRepeatHistoryRefusalPreservesOrdinaryCause(t *testing.T) {
	t.Parallel()
	b := newRepeatBed(t)
	prior := Result{Tree: "tree", Commit: "commit", Result: Red, Repeat: "started", Attempt: "first"}
	if err := withLock(b.install, func() error { return appendLine(resultsPath(b.install), prior) }); err != nil {
		t.Fatal(err)
	}
	failed := []FailedUnit{{Unit: "u/a", Tests: []string{"TestA"}}}
	ordinary := Cause{Kind: "unclassified", Tests: failingTests(failed), Evidence: "red.log"}
	red := Result{Tree: "tree", Commit: "commit", Result: Red, Attempt: "second", Failed: failed, Cause: &Cause{Kind: ordinary.Kind, Tests: ordinary.Tests, Evidence: ordinary.Evidence}}
	b.seams.Judge = func(string, string, []FailedUnit) (map[string]UnitJudgement, error) {
		return map[string]UnitJudgement{"u/a": {Known: true}}, nil
	}
	b.seams.Command = func(*exec.Cmd) error {
		t.Fatal("a spent repeat executed another check")
		return nil
	}
	replays := 0
	result := continueRed(b.seams, b.install, b.checkout, "fixture", b.install,
		Running{Attempt: red.Attempt, Tree: red.Tree, Commit: red.Commit}, scopeDecision{}, io.Discard, red, Result{},
		func(result, previous Result) Result {
			replays++
			if result.Cause == nil || !reflect.DeepEqual(*result.Cause, ordinary) || result.Repeat != "" || len(result.FlakeRepeats) != 0 || previous.Result != "" {
				t.Fatalf("spent repeat reached replay with a flake cause or allowance: %+v cause=%+v previous=%+v", result, result.Cause, previous)
			}
			return result
		})
	if replays != 1 || result.Result != Red || len(b.records) != 0 || !reflect.DeepEqual(b.lines(), []Result{prior}) {
		t.Fatalf("spent repeat changed history or skipped replay: %+v replays=%d records=%+v history=%+v", result, replays, b.records, b.lines())
	}
}

func TestRepeatFailureAndUnconfirmedRecordStayRed(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"unit fails", "record error", "nil record", "whole fails", "whole record error"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newRepeatBed(t)
			whole := strings.HasPrefix(name, "whole")
			if whole {
				b.seams.Judge = func(string, string, []FailedUnit) (map[string]UnitJudgement, error) {
					return map[string]UnitJudgement{"u/a": {}, "u/b": {}}, nil
				}
				// The repeat rule requires isolated greens before the whole retry.
				b.run("if [ -n \"$LANDING_ONLY\" ]; then printf 'LANDING-CHECKED\\t0\\n'; exit 0; fi; " + failedEvidenceReport())
			}
			if strings.Contains(name, "record error") {
				b.seams.RecordFlake = func(FlakeRecord) (FlakeRecorded, error) { return FlakeRecorded{}, errors.New("record not confirmed") }
			}
			if name == "nil record" {
				b.seams.RecordFlake = nil
			}
			command := "if [ -z \"$LANDING_ONLY\" ]; then " + failedEvidenceReport() + "; fi; " + passingEvidenceReport()
			if name == "unit fails" {
				command = "if [ -z \"$LANDING_ONLY\" ]; then " + failedEvidenceReport() + "; fi; exit 2"
			}
			if whole {
				command = passingEvidenceReport()
				if name == "whole fails" {
					command = failedEvidenceReport()
				}
			}
			red := b.run(command)
			if red.Result != Red || red.Repeat == "allowed" || len(b.records) != 0 {
				t.Fatalf("%s: %+v records %+v", name, red, b.records)
			}
			if name == "unit fails" {
				trace, _ := os.ReadFile(b.trace)
				if strings.Count(string(trace), "|tree|commit\n") != 3 {
					t.Fatalf("did not repeat every unit exactly once: %s", trace)
				}
			}
			b.refused()
		})
	}
}

func TestRepeatCrashAfterStartedRefusesEveryEntry(t *testing.T) {
	t.Parallel()
	b := newRepeatBed(t)
	if err := withLock(b.install, func() error {
		if err := appendLine(resultsPath(b.install), Result{Tree: "tree", Commit: "commit", Result: Red, Repeat: "started"}); err != nil {
			return err
		}
		return writeRunning(b.install, Running{Tree: "tree", Commit: "commit", Attempt: "dead"})
	}); err != nil {
		t.Fatal(err)
	}
	b.seams.Alive = func(Running) bool { return false }
	b.refused()
	if lines := b.lines(); len(lines) != 2 || lines[1].Repeat != "" {
		t.Fatalf("crash regained allowance: %+v", lines)
	}
}

func TestRepeatExistingGreenDoesNotRun(t *testing.T) {
	t.Parallel()
	b := newRepeatBed(t)
	green := b.run("exit 0")
	again := b.run("exit 7")
	if !reflect.DeepEqual(again, green) || len(b.lines()) != 1 {
		t.Fatalf("green repeated: %+v then %+v", green, again)
	}
	trace, _ := os.ReadFile(b.trace)
	if strings.Count(string(trace), "|tree|commit\n") != 1 {
		t.Fatalf("green command repeated: %s", trace)
	}
	if _, already, err := Start(b.install, b.checkout, b.seams); err != nil || !already {
		t.Fatalf("green start: %v %v", already, err)
	}
}

func TestRepeatInheritedGreenIncludesFlakeReason(t *testing.T) {
	t.Parallel()
	for _, entry := range []string{"run", "settled"} {
		t.Run(entry, func(t *testing.T) {
			t.Parallel()
			b := newRepeatBed(t)
			source := Result{Tree: "old", Commit: "old", Result: Green, Reason: "u/a failed once and passed when run again alone; seen 3 times; goal fix-flaky-u-a fixes it"}
			if err := withLock(b.install, func() error { return appendLine(resultsPath(b.install), source) }); err != nil {
				t.Fatal(err)
			}
			b.seams.Git = stubGit{commit: "commit", tree: "tree", changed: map[[2]string]string{{"old", "tree"}: "metasystem/plans/goals/fix.md"}}.run
			var green Result
			if entry == "run" {
				green = b.run("exit 1")
			} else {
				var ok bool
				var err error
				green, ok, err = Settled(b.install, b.checkout, b.seams)
				if err != nil || !ok {
					t.Fatalf("settled: %v %v", ok, err)
				}
			}
			if green.Result != Green || !strings.HasSuffix(green.Reason, "; "+source.Reason) {
				t.Fatalf("inherited reason: %+v", green)
			}
		})
	}
}

func TestRepeatRedCannotBorrowInheritedGreenPermission(t *testing.T) {
	t.Parallel()
	for _, entry := range []string{"run", "start"} {
		t.Run(entry, func(t *testing.T) {
			t.Parallel()
			b := newRepeatBed(t)
			green := Result{Tree: "old", Commit: "old", Result: Green}
			red := Result{Tree: "tree", Commit: "commit", Result: Red, Repeat: "allowed", Cause: &Cause{Kind: "environment"}}
			if err := withLock(b.install, func() error {
				if err := appendLine(resultsPath(b.install), green); err != nil {
					return err
				}
				return appendLine(resultsPath(b.install), red)
			}); err != nil {
				t.Fatal(err)
			}
			git := b.seams.Git
			b.seams.Git = func(dir string, args ...string) (string, error) {
				if strings.Join(args, " ") == "diff --name-only --no-renames old tree" {
					return "metasystem/plans/goals/fix.md", nil
				}
				return git(dir, args...)
			}
			b.seams.Policy = func(key string) (PolicyValue, error) {
				if key == "landing.proof" {
					return PolicyValue{Value: "person"}, nil
				}
				return PolicyValue{Value: "auto"}, nil
			}
			b.seams.Command = func(*exec.Cmd) error { t.Error("a red tree borrowed permission from an older green"); return nil }
			b.seams.Launch = func([]string, string, string) (int64, error) {
				t.Error("a red tree launched without fresh full-check permission")
				return int64(os.Getpid()), nil
			}
			var err error
			if entry == "run" {
				_, err = Run(b.install, b.checkout, "exit 0", "", io.Discard, b.seams)
			} else {
				_, _, err = Start(b.install, b.checkout, b.seams)
			}
			var refusal *Refusal
			if !errors.As(err, &refusal) || refusal.Code != "LANE_PROOF_PERSON" || len(b.lines()) != 2 {
				t.Fatalf("repeat bypassed full-check permission or changed history: %v %+v", err, b.lines())
			}
		})
	}
}

func plainTestEvents(unit string, tests []string, action string) string {
	var out strings.Builder
	for _, test := range tests {
		event, _ := json.Marshal(map[string]string{"Action": action, "Package": unit, "Test": test})
		out.Write(event)
		out.WriteByte('\n')
	}
	event, _ := json.Marshal(map[string]string{"Action": action, "Package": unit})
	out.Write(event)
	out.WriteByte('\n')
	return out.String()
}

func failedEvidenceReport() string {
	events := plainTestEvents("u/a", []string{"TestA", "TestB"}, "fail") + plainTestEvents("u/b", []string{"TestC"}, "fail")
	return "printf '%s' '" + events + "'; " + failedReport
}

func passingEvidenceReport() string {
	events := plainTestEvents("u/a", []string{"TestA", "TestB"}, "pass") + plainTestEvents("u/b", []string{"TestC"}, "pass")
	return "printf '%s' '" + events + "'; printf 'LANDING-CHECKED\\t0\\n'"
}
