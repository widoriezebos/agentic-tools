package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// The fake tool runs as a child of the public executor; only the tool and
// clock are replaced. Selection, records and continuation are production code.
func TestBoundedCheckProductionRoute(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, script string
		outcome      launch.CheckOutcome
		cut          bool
		message      string
	}{
		{"green", "", launch.CheckPassed, false, ""},
		{"custody start failure", "", launch.CheckEnvironmentFailure, false, ""},
		{"untracked changed package", "", launch.CheckPassed, false, ""},
		{"nomination also changed", "", launch.CheckPassed, false, "RESULT: DONE\nNOMINATE: ./nominated, TestFirst, first\n"},
		{"prerequisites outside budget", "case $1 in static|audits) printf 1200 > \"$clock\";; esac", launch.CheckPassed, false, ""},
		{"malformed source", "test \"$1\" != static || exit 1", launch.CheckFailed, false, ""},
		{"unavailable selection", "", launch.CheckEnvironmentFailure, false, ""},
		{"static red", "test \"$1\" != static || exit 1", launch.CheckFailed, false, ""},
		{"audits red", "test \"$1\" != audits || exit 1", launch.CheckFailed, false, ""},
		{"nominated red", "test \"$1\" != TestFirst || exit 1", launch.CheckFailed, false, ""},
		{"changed red", "test \"$1\" != changed || exit 1", launch.CheckFailed, false, ""},
		{"bare signal", "test \"$1\" != TestFirst || kill -TERM $$", launch.CheckFailed, false, ""},
		{"exit 137", "test \"$1\" != TestFirst || exit 137", launch.CheckFailed, false, ""},
		{"oom", "if test \"$1\" = TestFirst; then printf 'fatal error: out of memory\\n'; kill -KILL $$; fi", launch.CheckEnvironmentFailure, false, ""},
		{"budget between", "test \"$1\" != TestFirst || printf 360 > \"$clock\"", launch.CheckBudgetExhausted, false, ""},
		{"custodian lost", "if test \"$1\" = TestSecond; then exec HELPER DIRECTORY custodian; fi", launch.CheckEnvironmentFailure, false, ""},
		{"early detached exit", "if test \"$1\" = TestSecond; then exec HELPER DIRECTORY exit; fi", launch.CheckFailed, false, ""},
		{"escaped descendant", "if test \"$1\" = TestSecond; then exec HELPER DIRECTORY; fi", launch.CheckBudgetExhausted, true, ""},
		{"deadline", "if test \"$1\" = TestSecond; then printf ready > \"$ready\"; read line < \"$hold\"; fi", launch.CheckEnvironmentFailure, true, ""},
		{"cancelled", "if test \"$1\" = TestSecond; then printf ready > \"$ready\"; read line < \"$hold\"; fi", launch.CheckCancelled, true, ""},
		{"budget during", "if test \"$1\" = TestSecond; then printf ready > \"$ready\"; read line < \"$hold\"; fi", launch.CheckBudgetExhausted, true, ""},
		{"failure before budget", "test \"$1\" != TestFirst || exit 1; if test \"$1\" = TestSecond; then printf ready > \"$ready\"; read line < \"$hold\"; fi", launch.CheckFailed, false, ""},
		{"framework red before cut", "if test \"$1\" = TestSecond; then printf '{\"Action\":\"fail\",\"Test\":\"TestSecond\"}\\n'; printf ready > \"$ready\"; read line < \"$hold\"; fi", launch.CheckFailed, true, ""},
		{"cancelled after observed failure", "if test \"$1\" = TestSecond; then printf '{\"Action\":\"fail\",\"Test\":\"TestSecond\"}\\n'; printf ready > \"$ready\"; read line < \"$hold\"; fi", launch.CheckFailed, true, ""},
		{"no nominations", "", launch.CheckPassed, false, "RESULT: DONE\n"},
		{"empty nominations", "", launch.CheckPassed, false, "RESULT: DONE\n\n"},
		{"malformed nominations", "", launch.CheckPassed, false, "RESULT: DONE\nNOMINATE: ./nominated, TestFirst, first\nNOMINATE: ./nominated, TestSecond, second\nNOMINATE: ./nominated, TestFirst\nNOMINATE: ./nominated, TestFirst, reason, extra\nNOMINATE: ./unknown, TestFirst, absent\nNOMINATE: ./nominated, [, malformed\nNOMINATE: ./nominated, TestNope, absent\nNOMINATE: ./nominated;touch bad, TestFirst, invalid\n"},
		{"all malformed nominations", "", launch.CheckPassed, false, "RESULT: DONE\nNOMINATE: ./nominated, TestNope, absent\n"},
		{"budget exact last", "test \"$1\" != changed || printf 360 > \"$clock\"", launch.CheckPassed, false, ""},
		{"budget late failure", "if test \"$1\" = changed; then printf 361 > \"$clock\"; printf '{\"Action\":\"fail\",\"Test\":\"TestChanged\"}\\n'; exit 1; fi", launch.CheckFailed, false, ""},
		{"budget late last", "test \"$1\" != changed || printf 361 > \"$clock\"", launch.CheckBudgetExhausted, false, ""},
		{"failure then oom", "test \"$1\" != TestFirst || exit 1; if test \"$1\" = TestSecond; then printf 'fatal error: out of memory\\n'; kill -KILL $$; fi", launch.CheckFailed, false, ""},
		{"oom then failure", "if test \"$1\" = TestFirst; then printf 'fatal error: out of memory\\n'; kill -KILL $$; fi; test \"$1\" != TestSecond || exit 1", launch.CheckEnvironmentFailure, false, ""},
		{"static oom", "if test \"$1\" = static; then printf 'fatal error: out of memory\\n'; kill -KILL $$; fi", launch.CheckEnvironmentFailure, false, ""},
		{"admission outside budget", "test \"$1\" != changed || printf 1500 > \"$clock\"", launch.CheckPassed, false, ""},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			b := declaredCheckBed(t, "proof.cheap=printf gate\\n\nproof.audits=audits\nproof.deadline=15\nproof.check-kind=bounded\nproof.check-budget=6\n")
			bin := t.TempDir()
			log := filepath.Join(bin, "order")
			clock := filepath.Join(bin, "clock")
			ready := filepath.Join(bin, "ready")
			hold := filepath.Join(bin, "hold")
			if err := syscall.Mkfifo(hold, 0600); err != nil {
				t.Fatal(err)
			}
			testenv.HoldFIFO(t, hold)
			if row.name == "admission outside budget" {
				if err := os.WriteFile(clock, []byte("1200"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if row.name == "escaped descendant" || row.name == "early detached exit" || row.name == "custodian lost" {
				helper := filepath.Join(bin, "child-helper")
				build := testenv.Go("build", "-p", "2", "-o", helper, "./testdata/bounded-child")
				if err := build.Run(); err != nil {
					t.Fatal(err)
				}
				row.script = strings.ReplaceAll(strings.ReplaceAll(row.script, "HELPER", shellCommand([]string{helper})), "DIRECTORY", shellCommand([]string{bin}))
				ready = filepath.Join(bin, "child")
			}
			script := fmt.Sprintf("#!/bin/sh\nclock='%s'\nready='%s'\nhold='%s'\nprintf '%%s\\n' \"$1\" >> '%s'\n%s\nprintf 'completed %%s\\n' \"$1\"\ncase $1 in TestFirst|TestSecond) printf '{\"Action\":\"pass\",\"Test\":\"%%s\"}\\n' \"$1\";; changed) printf '{\"Action\":\"pass\",\"Test\":\"TestChanged\"}\\n';; esac\n", clock, ready, hold, log, row.script)
			if err := testexec.WriteFile(filepath.Join(bin, "tool"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"audits", "metasystem", "go"} {
				body := "#!/bin/sh\nexec '" + filepath.Join(bin, "tool") + "' " + name + "\n"
				if name == "metasystem" {
					body = "#!/bin/sh\nexec '" + filepath.Join(bin, "tool") + "' static\n"
				}
				if name == "go" {
					body = "#!/bin/sh\ncase \"$*\" in *TestFirst*) name=TestFirst;; *TestSecond*) name=TestSecond;; *) name=changed;; esac\nexec '" + filepath.Join(bin, "tool") + "' \"$name\"\n"
				}
				if err := testexec.WriteFile(filepath.Join(bin, name), []byte(body), 0700); err != nil {
					t.Fatal(err)
				}
			}
			for _, pkg := range []string{"nominated", "changed", "dependent"} {
				dir := filepath.Join(b.worktree, pkg)
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
				names := []string{"TestChanged"}
				if pkg == "nominated" {
					names = []string{"TestFirst", "TestSecond"}
				}
				body := "package fixture\nimport \"testing\"\n"
				for _, name := range names {
					body += "func " + name + "(t *testing.T) {}\n"
				}
				if err := os.WriteFile(filepath.Join(dir, "fixture_test.go"), []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if row.name == "malformed source" || row.name == "unavailable selection" {
				if err := os.WriteFile(filepath.Join(b.worktree, "changed", "fixture_test.go"), []byte("package fixture\n}\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			hook := b.workOwnersHook
			b.workOwnersHook = func(o *intentWorkOwners) {
				hook(o)
				git := o.git
				o.git = func(dir string, args ...string) ([]byte, error) {
					if len(args) > 0 && args[0] == "ls-files" && slices.Contains(args, "--others") {
						if row.name == "untracked changed package" {
							return []byte("changed/fixture_test.go\x00"), nil
						}
						return nil, nil
					}
					if len(args) > 0 && args[0] == "diff" && slices.Contains(args, "--name-only") {
						if row.name == "untracked changed package" {
							return nil, nil
						}
						if row.name == "nomination also changed" {
							return []byte("nominated/fixture_test.go\x00"), nil
						}
						return []byte("changed/fixture_test.go\x00"), nil
					}
					return git(dir, args...)
				}
				o.checkContext = func(c *launch.CheckContext) {
					c.Context = candidateResourceCustodyContext(t)
					if row.name == "custody start failure" {
						c.Context = proofrun.WithResourceCustodyExecutable(c.Context, filepath.Join(bin, "missing-custodian"))
					}
					cancelled, cancel := context.WithCancel(c.Context)
					t.Cleanup(cancel)
					c.Context = cancelled
					first := true
					c.Now = func() time.Time {
						if first && row.name == "admission outside budget" {
							first = false
							return time.Unix(0, 0)
						}
						v, _ := os.ReadFile(clock)
						var seconds int
						fmt.Sscanf(string(v), "%d", &seconds)
						return time.Unix(int64(seconds), 0)
					}
					c.After = func(d time.Duration) <-chan time.Time {
						ch := make(chan time.Time, 1)
						if row.cut && ((row.name == "deadline" && d == 15*time.Minute) || (row.name != "deadline" && d == 6*time.Minute)) {
							go func() {
								testenv.Await(t, "second test started", func() bool { _, err := os.Stat(ready); return err == nil })
								if strings.Contains(row.name, "cancelled") {
									cancel()
								} else {
									ch <- time.Unix(360, 0)
								}
							}()
						}
						return ch
					}
				}
			}
			code, built, _ := b.work("work", "build", b.id, "bounded", "--brief", b.brief("bounded.md", "Build.\n"), "--lines", "100")
			if code != 0 {
				t.Fatalf("bounded admission: %s", built.Summary)
			}
			run := resultData(t, built)["run"].(string)
			plan, err := launch.ReadUnitPlan(resultData(t, built)["plan"].(string))
			if err != nil {
				t.Fatal(err)
			}
			// This is the same durable input judgeBuildResult writes after collection.
			nominations := launch.CheckNominations{Entries: []launch.CheckNomination{{Package: "./nominated", Pattern: "TestFirst", Reason: "first"}, {Package: "./nominated", Pattern: "TestSecond", Reason: "second"}}}
			if row.message != "" {
				nominations = launch.ParseCheckNominations(row.message)
			}
			data, _ := json.Marshal(nominations)
			if err := os.WriteFile(filepath.Join(filepath.Dir(plan.Path), "nominations.json"), data, 0600); err != nil {
				t.Fatal(err)
			}
			code, result := declaredCheckAtEngine(t, b, b.worktree, "proof", filepath.Join(bin, "metasystem"), "test", "run", "--unit-run", run)
			wantCode := row.outcome == launch.CheckPassed || row.outcome == launch.CheckBudgetExhausted
			if (code == 0) != wantCode || resultData(t, result)["checkOutcome"] != string(row.outcome) {
				t.Fatalf("code=%d result=%s data=%v", code, result.Summary, resultData(t, result)["checkOutcome"])
			}
			execution := resultData(t, result)["execution"].(string)
			retained, err := launch.ReadCheckResult(execution)
			if err != nil {
				t.Fatal(err)
			}
			if retained.Version != 2 || retained.Exits[0].Name == "cheap" {
				t.Fatal("an older reader could accept bounded feedback")
			}
			if len(retained.Selected) != len(retained.Ran)+len(retained.NotRun) {
				t.Fatal("selected test accounting diverged")
			}
			unique := map[string]bool{}
			for _, name := range retained.Selected {
				if unique[name] {
					t.Fatal("duplicate selected test")
				}
				unique[name] = true
			}
			order, _ := os.ReadFile(log)
			lines := strings.Fields(string(order))
			if row.name == "custody start failure" {
				if len(lines) != 0 {
					t.Fatal("a command ran without custody")
				}
				return
			}
			if len(lines) < 2 || !slices.Equal(lines[:2], []string{"static", "audits"}) {
				t.Fatalf("prerequisite order: %v", lines)
			}
			wantOrder := []string{"static", "audits", "TestFirst", "TestSecond", "changed"}
			if row.name == "nomination also changed" {
				wantOrder = []string{"static", "audits", "TestFirst", "TestSecond"}
			}
			if len(nominations.Entries) == 0 || row.name == "all malformed nominations" {
				wantOrder = []string{"static", "audits", "changed"}
			}
			if row.name == "admission outside budget" && (retained.AdmissionWaitSeconds != 1200 || retained.FeedbackSeconds != 300) {
				t.Fatal("admission wait was charged to execution")
			}
			if row.name == "prerequisites outside budget" && retained.PrerequisiteSeconds != 1200 {
				t.Fatal("prerequisite time was lost")
			}
			if row.name == "malformed nominations" && len(retained.Skipped) != 6 {
				t.Fatalf("lost skip diagnostics: %v", retained.Skipped)
			}
			if row.outcome == launch.CheckPassed && !slices.Equal(lines, wantOrder) {
				t.Fatalf("selection order: %v", lines)
			}
			if row.name == "malformed source" || row.name == "unavailable selection" {
				if !slices.Equal(lines, []string{"static", "audits"}) || len(retained.Skipped) == 0 {
					t.Fatal("selection failure lost prerequisite execution or its terminal record")
				}
			}
			if row.name == "failure before budget" || row.name == "failure then oom" || row.name == "nominated red" {
				if !slices.Equal(lines, []string{"static", "audits", "TestFirst"}) {
					t.Fatalf("feedback continued after red: %v", lines)
				}
			}
			if row.name == "escaped descendant" || row.name == "early detached exit" {
				body, _ := os.ReadFile(ready)
				pid, _ := strconv.ParseInt(string(body), 10, 64)
				exact, state, err := (identity.KernelProber{}).Probe(pid)
				if err != nil || state == identity.Alive && !exact.Zombie {
					t.Fatal("a detached child survived the result")
				}
			}
			_, evErr := os.Stat(filepath.Join(execution, "evidence.json"))
			if (evErr == nil) != (row.outcome == launch.CheckPassed) {
				t.Fatal("coverage does not agree with outcome")
			}
			if row.name == "oom" {
				repaired := strings.Replace(script, row.script, "", 1)
				if err := testexec.WriteFile(filepath.Join(bin, "tool"), []byte(repaired), 0700); err != nil {
					t.Fatal(err)
				}
				retryCode, retry := declaredCheckAtEngine(t, b, b.worktree, "proof", filepath.Join(bin, "metasystem"), "test", "run", "--unit-run", run)
				retried, readErr := launch.ReadCheckResult(resultData(t, retry)["execution"].(string))
				if retryCode != 0 || readErr != nil || retried.Outcome != launch.CheckPassed || !slices.Equal(retried.Selected, retained.Selected) || retried.ExecutionID == retained.ExecutionID {
					t.Fatal("environment retry did not start a fresh frozen selection")
				}
			}
			if row.outcome == launch.CheckBudgetExhausted {
				if len(retained.NotRun) == 0 || retained.RequirePassed() == nil {
					t.Fatal("exhaustion lost its incomplete coverage")
				}
				if !strings.Contains(strings.Join(retained.StatusLines(), "\n"), "candidate's complete check") {
					t.Fatal("status lost deferred coverage")
				}
			}
		})
	}
}

// boundedRoundStarter replaces only the model and child CLI boundary. The
// runner still collects RESULT, freezes nominations, checks, and starts reads.
type boundedRoundStarter struct {
	b               *workBed
	t               *testing.T
	engine, message string
	checked         intentResult
}

func (s *boundedRoundStarter) StartSupervisor(id, state string) (identity.Ref, error) {
	record, err := s.b.manager.Store.Read(id)
	if err != nil {
		return identity.Ref{}, err
	}
	if record.Kind == "build" {
		if err := os.WriteFile(filepath.Join(state, "last-message.txt"), []byte(s.message), 0600); err != nil {
			return identity.Ref{}, err
		}
	}
	if record.Kind == "proof" {
		command, err := (launch.PlainExec{}).Command(record, state)
		if err != nil {
			return identity.Ref{}, err
		}
		code, result := declaredCheckAtEngine(s.t, s.b, command.Directory, "proof", s.engine, command.Args...)
		s.checked = result
		s.b.starter.fail["proof"] = code != 0
	}
	return s.b.starter.StartSupervisor(id, state)
}

func TestBoundedRoundContinuesAndFreezesNominations(t *testing.T) {
	t.Parallel()
	b := declaredCheckBed(t, "proof.cheap=printf gate\nproof.audits=true\nproof.deadline=15\nproof.check-kind=bounded\n")
	bin := t.TempDir()
	clock := filepath.Join(bin, "clock")
	for name, body := range map[string]string{"metasystem": "#!/bin/sh\nexit 0\n", "go": "#!/bin/sh\nprintf 360 > '" + clock + "'\nprintf 'first completed\\n{\"Action\":\"pass\",\"Test\":\"TestFirst\"}\\n'\n"} {
		if err := testexec.WriteFile(filepath.Join(bin, name), []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(b.worktree, "fixture_test.go"), []byte("package fixture\nimport \"testing\"\nfunc TestFirst(t *testing.T){}\nfunc TestSecond(t *testing.T){}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	hook := b.workOwnersHook
	b.workOwnersHook = func(o *intentWorkOwners) {
		hook(o)
		git := o.git
		o.git = func(dir string, args ...string) ([]byte, error) {
			if len(args) > 0 && args[0] == "ls-files" && slices.Contains(args, "--others") {
				return nil, nil
			}
			if len(args) > 0 && args[0] == "diff" && slices.Contains(args, "--name-only") {
				return []byte("fixture_test.go\x00"), nil
			}
			return git(dir, args...)
		}
		o.checkContext = func(c *launch.CheckContext) {
			c.Context = candidateResourceCustodyContext(t)
			c.Now = func() time.Time {
				v, _ := os.ReadFile(clock)
				var n int
				fmt.Sscanf(string(v), "%d", &n)
				return time.Unix(int64(n), 0)
			}
			c.After = func(time.Duration) <-chan time.Time { return make(chan time.Time) }
		}
	}
	starter := &boundedRoundStarter{b: b, t: t, engine: filepath.Join(bin, "metasystem"), message: "RESULT: DONE\nNOMINATE: ., TestFirst, first feedback\nNOMINATE: ., TestSecond, second feedback\n"}
	b.manager.Supervisor = starter
	code, built, _ := b.work("work", "build", b.id, "bounded-round", "--brief", b.brief("bounded-round.md", "Read each round: yes\nBuild.\n"), "--lines", "100")
	if code != 0 || resultData(t, starter.checked)["checkOutcome"] != "budget-exhausted" || !slices.Contains(b.starter.launched(), "read") {
		t.Fatalf("exhaustion blocked read: code=%d outcome=%v launches=%v summary=%s", code, resultData(t, starter.checked)["checkOutcome"], b.starter.launched(), built.Summary)
	}
	if !strings.Contains(built.Summary, "budget-exhausted") || !strings.Contains(built.Summary, "1 of 2 selected tests not run") || !strings.Contains(built.Summary, "prerequisites passed") || built.Next == nil || !strings.Contains(built.Next.Reason, "review runs the complete check") || strings.Contains(built.Summary, "green") || strings.Contains(built.Next.Reason, "checks passed") {
		t.Fatalf("incomplete feedback was presented as a pass: summary=%s next=%+v", built.Summary, built.Next)
	}
	planPath := resultData(t, built)["plan"].(string)
	path := filepath.Join(filepath.Dir(planPath), "nominations.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var nominations launch.CheckNominations
	if json.Unmarshal(before, &nominations) != nil || len(nominations.Entries) != 2 {
		t.Fatal("builder nominations were not frozen")
	}
	record, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(resultData(t, built)["run"].(string))
	if err != nil {
		t.Fatal(err)
	}
	state, err := b.manager.Store.StateDir(record.Rounds[0].Steps[0].LaunchID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, "last-message.txt"), []byte("RESULT: DONE\nNOMINATE: ., TestOther, later\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(clock, []byte("0"), 0600); err != nil {
		t.Fatal(err)
	}
	_, again := declaredCheckAtEngine(t, b, b.worktree, "proof", starter.engine, "test", "run", "--unit-run", record.ID)
	after, _ := os.ReadFile(path)
	if !slices.Equal(before, after) || resultData(t, again)["checkOutcome"] != "budget-exhausted" {
		t.Fatal("resume reread the builder's mutable message")
	}
	command, rest, _ := resolveIntentArgv([]string{"work", "status", "run:" + record.ID})
	var stdout, stderr bytes.Buffer
	owners := b.workOwners()
	owners.processes.launches = func() *launch.Manager { return b.manager }
	if code := runIntentIn(command, rest, &stdout, &stderr, b.root(), owners); code != 0 || !strings.Contains(stdout.String(), "budget-exhausted") || !strings.Contains(stdout.String(), "candidate's complete check") {
		t.Fatalf("public status lost feedback: code=%d %s", code, stdout.String())
	}
	// A retained budget result must not replace a later failed round outcome.
	// Waiting reads the recorded round without executing its checks again.
	for _, outcome := range []string{"proof-wrote", "read-failed"} {
		t.Run(outcome, func(t *testing.T) {
			record.Rounds[0].Outcome = outcome
			data, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			writeTestingFixtureFile(t, filepath.Join(b.unitRoot, record.ID, "run.json"), data, 0600)
			code, stopped, _ := b.work("work", "wait", "run:"+record.ID)
			if code != 0 || resultData(t, stopped)["outcome"] != outcome {
				t.Fatalf("retained round was not read: exit=%d outcome=%v", code, resultData(t, stopped)["outcome"])
			}
			wantNext := "revise"
			if outcome == "read-failed" {
				wantNext = "review"
			}
			if !strings.Contains(stopped.Summary, outcome) || strings.Contains(stopped.Summary, "the round continues") || stopped.Next == nil || !slices.Contains(stopped.Next.Argv, wantNext) {
				t.Fatalf("stopped round presented as continuing: summary=%s next=%+v", stopped.Summary, stopped.Next)
			}
		})
	}
}

func TestBoundedManualRepairAdmission(t *testing.T) {
	t.Parallel()
	for _, person := range []bool{false, true} {
		t.Run(fmt.Sprint(person), func(t *testing.T) {
			t.Parallel()
			body := "proof.cheap=true\nproof.audits=true\nproof.deadline=15\n"
			b := declaredCheckBed(t, body)
			b.head = "candidate-commit"
			hook := b.workOwnersHook
			b.workOwnersHook = func(o *intentWorkOwners) {
				hook(o)
				git := o.git
				o.git = func(dir string, args ...string) ([]byte, error) {
					if len(args) == 2 && args[0] == "show" && (args[1] == "HEAD:metasystem.conf" || args[1] == "candidate-commit:metasystem.conf") {
						return []byte(body + "proof.full=printf full\nproof.check-kind=bounded\n"), nil
					}
					return git(dir, args...)
				}
			}
			owners := b.workOwners()
			args := []string{"work", "build", b.id, "manual", "--brief", b.brief("manual.md", "Build.\n"), "--lines", "100", "--reason", "Repair the command"}
			if person {
				owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
				args = append(args, "--by", "Wido")
			}
			args = append(args, "--check", "true")
			code, result := checkActBuild(t, b, owners, args...)
			if !person {
				if code == 0 || len(b.starter.launched()) != 0 {
					t.Fatal("agent manual check bypassed admission")
				}
				return
			}
			if code != 0 {
				t.Fatalf("person repair: %s", result.Summary)
			}
			plan, err := launch.ReadUnitPlan(resultData(t, result)["plan"].(string))
			if err != nil {
				t.Fatal(err)
			}
			if plan.Check.Declaration != nil || plan.Check.Kind() != "legacy" {
				t.Fatal("manual repair masqueraded as bounded coverage")
			}
			admitted := false
			for _, act := range checkActs(t, b) {
				admitted = admitted || act.Class == "declaration" && act.Status == "applied" && act.AfterDeclaration.EffectiveValues()[4] == "bounded"
			}
			if !admitted {
				t.Fatal("bounded tree passed manual repair without declaration admission")
			}
		})
	}
}
