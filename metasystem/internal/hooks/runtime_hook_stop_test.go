package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Stop worker tests: the turn end from arming to emission, each captured
// fact, each infrastructure condition and its evidence. Ported from the
// supervision-hook bed's open-work, stop-failure, re-arm, repeated
// infrastructure and worker legs, and the nested bed's override and
// compiled-installation rows.

func stopOnce(t *testing.T, configure func(*fakeOps), payload string) (hookInstallation, *fakeOps, hookRun) {
	t.Helper()
	installation := newHookInstallation(t)
	ops := newFakeOps(t, installation)
	if configure != nil {
		configure(ops)
	}
	if payload == "" {
		payload = `{"session_id":"line-fixture","hook_event_name":"Stop"}`
	}
	return installation, ops, runHook(t, installation, ops, hookCall{runtime: "claude", event: "stop", payload: payload})
}

func (f *fakeOps) completion(t *testing.T) HookCompletion {
	t.Helper()
	if len(f.completions) != 1 {
		t.Fatalf("recorded %d completions, want 1: %s", len(f.completions), f.trace())
	}
	return f.completions[0]
}

func presentedFailures(t *testing.T, ops *fakeOps) string {
	t.Helper()
	if len(ops.stopInputs) != 1 {
		t.Fatalf("composed %d presentations: %s", len(ops.stopInputs), ops.trace())
	}
	return ops.captured[0].failures
}

// The healthy turn end: one presented payload, the elapsed response trail,
// the digest cursor advanced and the completion recorded with its report.
func TestStopEmitsThePresentedVerdict(t *testing.T) {
	t.Parallel()
	started := time.Now().Add(-3 * time.Second).Unix()
	installation := newHookInstallation(t)
	ops := newFakeOps(t, installation)
	ops.digestPending = func() (string, int) {
		return `{"cursor":42,"message":"NARRATOR DIGEST since last check-in\nA landing moved the repository storyline to commit abc123","prefixSha256":"` + strings.Repeat("d", 64) + `"}`, 0
	}
	run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "stop", payload: `{"session_id":"line-fixture","transcript_path":"/transcripts/t.jsonl"}`,
		env: map[string]string{stopDeadlineParentEnv: "777", stopDeadlineStartedEnv: fmt.Sprint(started)}})
	if run.status != 0 || run.stdout != `{"systemMessage":"presented: "}`+"\n" {
		t.Fatalf("stop = status %d stdout %q stderr %q", run.status, run.stdout, run.stderr)
	}
	completion := ops.completion(t)
	if completion.Result != "OK" || completion.Outcome != "EMITTED" || completion.Generation != "2" || completion.Attempt != "3" ||
		completion.HealthLine != "HEALTH healthy — fixture" || completion.ReportID != "report-id" || completion.ReportAlias != "a1" ||
		completion.Installation != installation.root || !completion.HasElapsed || completion.ElapsedSec < 3 {
		t.Fatalf("completion = %+v", completion)
	}
	if !ops.called("steward digest-advance cursor=42 prefix=" + strings.Repeat("d", 64)) {
		t.Fatalf("the delivered digest did not advance: %s", ops.trace())
	}
	if !regexp.MustCompile(`(?m) stop response decision=allow elapsed=[0-9]+s$`).MatchString(readHookLog(t, installation)) ||
		!strings.Contains(readHookLog(t, installation), " stop verdict block=false\n") {
		t.Fatalf("hook log = %q", readHookLog(t, installation))
	}
	verdict := ops.turnVerdicts[0]
	if verdict.Transcript != "/transcripts/t.jsonl" || verdict.Runtime != "claude" || verdict.MainID != "main-fixture" ||
		!filepath.IsAbs(verdict.FactsFile) || !filepath.IsAbs(verdict.CompletionFile) {
		t.Fatalf("turn verdict request = %+v", verdict)
	}
	input := ops.stopInputs[0]
	if input.Machine != "fixture-machine" || input.Lineage != "fixture-lineage" || input.ClaimEpoch != "7" || input.Attempt != strings.Repeat("ab", 16) || input.Advisor {
		t.Fatalf("presentation input coordinates = %+v", input)
	}
	// The worker's presentation files are its own and gone after the turn.
	if _, err := os.Stat(input.OutputFile); !os.IsNotExist(err) {
		t.Fatalf("the worker left its presentation staging: %v", err)
	}
}

// A retained block reaches the runtime through the runtime's own mapper,
// once per outcome, and the verdict's watchdog and repeat state ride along.
func TestStopBlockReachesTheRuntimeMapper(t *testing.T) {
	t.Parallel()
	_, ops, run := stopOnce(t, func(ops *fakeOps) {
		ops.verdict = strings.Replace(ops.verdict, `"shouldBlock":false`, `"shouldBlock":true`, 1)
		ops.verdict = strings.Replace(ops.verdict, `"surfaceWatchdog":false`, `"surfaceWatchdog":true`, 1)
		ops.watchdog = func() (string, int) { return "SUPERVISION owner not running\n", 0 }
	}, `{"session_id":"stop-task-name-block","stop_hook_active":true}`)
	var block map[string]string
	if err := json.Unmarshal([]byte(run.stdout), &block); err != nil || block["decision"] != "block" ||
		!strings.Contains(block["reason"], "SUPERVISION owner not running") {
		t.Fatalf("block = %q (%v)", run.stdout, err)
	}
	if strings.Count(ops.trace(), "adapter stop-output runtime=claude") != 1 {
		t.Fatalf("the runtime mapper ran %d times: %s", strings.Count(ops.trace(), "adapter stop-output"), ops.trace())
	}
	verdict := ops.turnVerdicts[0]
	if !verdict.StopHookActive || verdict.Watchdog != sha256Text("SUPERVISION owner not running") {
		t.Fatalf("turn verdict request = %+v", verdict)
	}
}

// Captured pre-verdict failures and unreadable verdicts all allow the Stop
// with their degraded diagnostic; an early failure ends the worker for the
// deadline parent to convert.
func TestStopPreVerdictFailuresAllow(t *testing.T) {
	t.Parallel()
	bare := mustForm(t, "allowed", "bare") + "\n"
	t.Run("runtime registry", func(t *testing.T) {
		_, _, run := stopOnce(t, func(ops *fakeOps) { ops.runtimeNames = func() (string, int) { return "", 41 } }, "")
		if run.status != 41 || run.stdout != "" {
			t.Fatalf("registry failure = status %d stdout %q", run.status, run.stdout)
		}
	})
	t.Run("hook attempt", func(t *testing.T) {
		_, ops, run := stopOnce(t, func(ops *fakeOps) { ops.hookAttempt = func(int) (string, int) { return "", 42 } }, "")
		failures := presentedFailures(t, ops)
		if run.status != 0 || !strings.Contains(run.stdout, "attempt evidence could not be recorded") || failures != "attempt evidence could not be recorded" {
			t.Fatalf("hook attempt failure = status %d stdout %q failures %q", run.status, run.stdout, failures)
		}
		notices := ops.captured[0].notices
		if !strings.Contains(notices, "HEALTH unknown — hook-freshness=unknown (attempt evidence could not be recorded)") {
			t.Fatalf("the evidence failure was not carried: %q", notices)
		}
	})
	t.Run("turn verdict", func(t *testing.T) {
		installation, ops, run := stopOnce(t, func(ops *fakeOps) {
			ops.turnVerdict = func(TurnVerdictRequest) (string, string, int) { return "", "fixture turn-verdict failure\n", 43 }
		}, "")
		if run.status != 0 || run.stdout != bare {
			t.Fatalf("verdict failure = status %d stdout %q", run.status, run.stdout)
		}
		if completion := ops.completion(t); completion.Result != "ERROR" || completion.Outcome != "PRESENTATION_UNAVAILABLE" {
			t.Fatalf("completion = %+v", completion)
		}
		log := readHookLog(t, installation)
		if !strings.Contains(log, "stop-condition infrastructure turn-verdict-unavailable verdict-state 2 ") || !strings.Contains(log, " stop verdict unavailable\n") {
			t.Fatalf("hook log = %q", log)
		}
	})
	t.Run("malformed verdict", func(t *testing.T) {
		_, _, run := stopOnce(t, func(ops *fakeOps) {
			ops.turnVerdict = func(TurnVerdictRequest) (string, string, int) { return `{"shouldBlock":false}`, "", 0 }
		}, "")
		if run.status != 0 || run.stdout != bare {
			t.Fatalf("malformed verdict = status %d stdout %q", run.status, run.stdout)
		}
	})
	t.Run("partial mapper output", func(t *testing.T) {
		_, _, run := stopOnce(t, func(ops *fakeOps) {
			ops.stopOutput = func(_, _, output string, _ io.Writer) int { return writeTestFile(output, `{"systemMessage":`) }
		}, "")
		// The worker publishes what the mapper wrote; the deadline parent
		// refuses it as unreadable (TestDeadlineParentRejectsUnreadableOutput).
		if run.status != 0 || run.stdout != `{"systemMessage":`+"\n" || validStopOutput(run.stdout, true) {
			t.Fatalf("partial output = status %d stdout %q", run.status, run.stdout)
		}
	})
	t.Run("presentation unavailable keeps the retained block", func(t *testing.T) {
		_, ops, run := stopOnce(t, func(ops *fakeOps) {
			ops.verdict = strings.Replace(ops.verdict, `"shouldBlock":false`, `"shouldBlock":true`, 1)
			ops.stopPresent = func(string, string, io.Writer) int { return 1 }
		}, "")
		if run.stdout != mustForm(t, "blocked", "bare")+"\n" || ops.completion(t).Outcome != "PRESENTATION_UNAVAILABLE" {
			t.Fatalf("retained block = stdout %q completion %+v", run.stdout, ops.completions)
		}
	})
	// Every presentation owner that fails leaves the retained allowance in
	// its bare degraded form, and an owner's diagnostics reach the hook log.
	for _, owner := range []struct {
		name      string
		configure func(*fakeOps)
		logged    string
	}{
		{"attempt token unavailable", func(ops *fakeOps) {
			ops.tokenHex = func() (string, error) { return "", fmt.Errorf("fixture entropy failure") }
		}, ""},
		{"attempt token malformed", func(ops *fakeOps) {
			ops.tokenHex = func() (string, error) { return "not-hex", nil }
		}, ""},
		{"presentation input refused", func(ops *fakeOps) {
			ops.stopInput = func(_ StopInputRequest, stderr io.Writer) int {
				_, _ = io.WriteString(stderr, "fixture stop-input diagnostic\n")
				return 1
			}
		}, "fixture stop-input diagnostic\n"},
		{"runtime mapper refused", func(ops *fakeOps) {
			ops.stopOutput = func(_, _, _ string, stderr io.Writer) int {
				_, _ = io.WriteString(stderr, "fixture stop-output diagnostic\n")
				return 1
			}
		}, "fixture stop-output diagnostic\n"},
	} {
		t.Run(owner.name, func(t *testing.T) {
			installation, ops, run := stopOnce(t, owner.configure, "")
			if run.status != 0 || run.stdout != bare {
				t.Fatalf("%s = status %d stdout %q", owner.name, run.status, run.stdout)
			}
			if completion := ops.completion(t); completion.Result != "ERROR" || completion.Outcome != "PRESENTATION_UNAVAILABLE" {
				t.Fatalf("completion = %+v", completion)
			}
			if !strings.Contains(readHookLog(t, installation), owner.logged) {
				t.Fatalf("the owner's diagnostic did not reach the hook log: %q", readHookLog(t, installation))
			}
		})
	}
}

// A re-arm notice is keyed from up's aggregate line; a later failure cannot
// erase the re-arm, and absence invents none.
func TestStopRearmNotices(t *testing.T) {
	t.Parallel()
	notices := func(t *testing.T, ops *fakeOps) string {
		t.Helper()
		return ops.captured[0].notices
	}
	_, ops, _ := stopOnce(t, func(ops *fakeOps) {
		ops.up = func(_ UpRequest, stdout, _ io.Writer) int {
			fmt.Fprintln(stdout, `up outcome=armed authority=writer re-armed="generation=9 previous=8 engine=abc1234 landed=def5678"`)
			return 0
		}
	}, "")
	if !strings.Contains(notices(t, ops), `Metasystem re-armed the rebuilt engine: up outcome=armed authority=writer re-armed="generation=9`) {
		t.Fatalf("successful re-arm notice = %q", notices(t, ops))
	}
	_, ops, _ = stopOnce(t, func(ops *fakeOps) {
		ops.up = func(_ UpRequest, stdout, _ io.Writer) int {
			fmt.Fprintln(stdout, `up outcome=failed re-armed="generation=9 previous=8 engine=abc1234 landed=def5678" component=steward-runner`)
			return 44
		}
	}, "")
	if !strings.Contains(notices(t, ops), "Metasystem re-armed the rebuilt engine:") || presentedFailures(t, ops) != "supervision arming failed" {
		t.Fatalf("failed re-arm = notices %q failures %q", notices(t, ops), presentedFailures(t, ops))
	}
	_, ops, _ = stopOnce(t, nil, "")
	if strings.Contains(notices(t, ops), "re-armed") {
		t.Fatalf("a run without the aggregate key invented a re-arm notice: %q", notices(t, ops))
	}
	for _, test := range []struct{ component, want string }{
		{"stop incomplete supervision-owner", "Metasystem stop incomplete supervision-owner; run: metasystem system stop."},
		{"owner standing", "Metasystem is stopped for this checkout; start again with metasystem system start."},
	} {
		_, ops, _ = stopOnce(t, func(ops *fakeOps) {
			ops.up = func(_ UpRequest, stdout, _ io.Writer) int {
				remedy := "metasystem system start"
				if strings.HasPrefix(test.component, "stop incomplete") {
					remedy = "metasystem system stop"
				}
				fmt.Fprintf(stdout, "component=stopped outcome=standing detail=%q\nup outcome=stopped remedy=%q\n", test.component, remedy)
				return 0
			}
		}, "")
		if !strings.Contains(notices(t, ops), test.want) {
			t.Fatalf("stopped notice = %q, want %q", notices(t, ops), test.want)
		}
	}
}

// Infrastructure failures allow on every occurrence and keep their stable
// cause, count, component detail, remedy and hook-log line.
func TestStopRepeatedInfrastructureAllows(t *testing.T) {
	t.Parallel()
	installation := newHookInstallation(t)
	armingFailure := func(ops *fakeOps) {
		ops.up = func(_ UpRequest, stdout, stderr io.Writer) int {
			fmt.Fprintln(stdout, `component=steward-runner outcome=failed detail="ENROLLMENT_DRIFT" remedy="run metasystem internal steward restart from an agent-free terminal"`)
			fmt.Fprintln(stdout, `up outcome=failed component=steward-runner remedy="ENROLLMENT_DRIFT: run 'metasystem internal steward restart' from an agent-free terminal"`)
			fmt.Fprintln(stderr, "arming diagnostic")
			return 44
		}
	}
	var last *fakeOps
	for occurrence := 1; occurrence <= 2; occurrence++ {
		ops := newFakeOps(t, installation)
		armingFailure(ops)
		run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "stop", payload: `{"session_id":"external-failure"}`})
		if run.status != 0 || strings.Contains(run.stdout, `"decision":"block"`) {
			t.Fatalf("occurrence %d = status %d stdout %q", occurrence, run.status, run.stdout)
		}
		last = ops
	}
	refusal := last.stopBlocks[0]
	wantResult := `component=steward-runner outcome=failed detail="ENROLLMENT_DRIFT" remedy="run metasystem internal steward restart from an agent-free terminal"` + "\n" +
		`up outcome=failed component=steward-runner remedy="ENROLLMENT_DRIFT: run 'metasystem internal steward restart' from an agent-free terminal"` + "\narming diagnostic"
	if refusal.Cause != "supervision arming failed" || refusal.Remedy != wantResult || refusal.ArmingResult != wantResult ||
		refusal.RefusalRecord != filepath.Join(installation.root, "artifacts", "agents", "supervision", "stop-refusals", "external-failure.json") {
		t.Fatalf("recorded refusal = %+v", refusal)
	}
	record, err := os.ReadFile(refusal.RefusalRecord)
	if err != nil || !strings.Contains(string(record), `"count": 2`) || !strings.Contains(string(record), `"sessionId": "external-failure"`) {
		t.Fatalf("refusal record = %s (%v)", record, err)
	}
	notices := last.captured[0].notices
	if !strings.Contains(notices, "occurrence 2") {
		t.Fatalf("the repeated-failure notice did not reach the presentation: %q", notices)
	}
	if !strings.Contains(readHookLog(t, installation), "stop-condition infrastructure supervision-arming-failed supervision-arming 2 ") {
		t.Fatalf("hook log = %q", readHookLog(t, installation))
	}
	arming := last.captured[0].armingStderr
	if arming != "arming diagnostic\n" || last.stopInputs[0].ArmingExit != 44 {
		t.Fatalf("arming capture = %q exit %d", arming, last.stopInputs[0].ArmingExit)
	}
}

// A dynamic diagnostic rides the notices while the occurrence identity stays
// the stable cause.
func TestStopMissingFactsKeepsTheStableOccurrence(t *testing.T) {
	t.Parallel()
	installation := newHookInstallation(t)
	for occurrence := 1; occurrence <= 2; occurrence++ {
		ops := newFakeOps(t, installation)
		ops.turnVerdict = func(request TurnVerdictRequest) (string, string, int) {
			_ = os.WriteFile(request.CompletionFile, []byte("{}\n"), 0o600)
			return ops.verdict, fmt.Sprintf("fixture removed frozen facts at %s\n", request.FactsFile), 0
		}
		runHook(t, installation, ops, hookCall{runtime: "claude", event: "stop", payload: `{"session_id":"missing-facts"}`})
		notices := ops.captured[0].notices
		if !strings.Contains(notices, "fixture removed frozen facts at ") {
			t.Fatalf("occurrence %d dropped its dynamic diagnostic: %q", occurrence, notices)
		}
	}
	record, _ := os.ReadFile(filepath.Join(installation.root, "artifacts", "agents", "supervision", "stop-refusals", "missing-facts.json"))
	if strings.Count(string(record), `"cause": "the frozen judgment facts were unavailable"`) != 1 || !strings.Contains(string(record), `"count": 2`) {
		t.Fatalf("dynamic diagnostics split the stable occurrence: %s", record)
	}
}

// Each captured fact's failure is a named condition with its own line.
func TestStopCapturedFactFailures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		configure func(*fakeOps)
		failure   string
		logLine   string
		notice    string
	}{
		{"health", func(ops *fakeOps) { ops.health = func() (string, int) { return "", 45 } },
			"the health engine returned no verdict", "stop-condition infrastructure health-engine-returned-no-verdict health ", ""},
		{"narrator", func(ops *fakeOps) {
			ops.digestPending = func() (string, int) { return "steward digest-pending: fixture narrator read failure\n", 46 }
		}, "the narrator digest could not be read", "stop-condition infrastructure narrator-digest-could-not-be-read narrator ", ""},
		{"narrator state", func(ops *fakeOps) { ops.digestPending = func() (string, int) { return `{"message":"x"}`, 0 } },
			"the narrator digest state was unreadable", "stop-condition infrastructure narrator-digest-state-was-unreadable narrator ", ""},
		{"protocol", func(ops *fakeOps) { ops.protocolGrowth = func() (string, int) { return "", 1 } },
			"the holder protocol state could not be read", "stop-condition infrastructure holder-protocol-state-could-not-be-read holder-protocol ", ""},
		{"lease renewal", func(ops *fakeOps) { ops.renew = func() int { return 1 } },
			"the checkout holder lease could not be renewed", "stop-condition infrastructure checkout-holder-lease-could-not-be-renewed holder-lease ", ""},
		{"watchdog", func(ops *fakeOps) { ops.watchdog = func() (string, int) { return "", 1 } },
			"the supervision watchdog state could not be read", "stop-condition infrastructure supervision-watchdog-state-could-not-be-read watchdog ", ""},
		{"evidence", func(ops *fakeOps) { ops.evidenceGC = func(io.Writer) int { return 3 } },
			"the hook evidence state could not be maintained", "stop-condition infrastructure hook-evidence-state-could-not-be-maintained hook-evidence ", ""},
		{"holder classification", func(ops *fakeOps) {
			ops.classify = func(string, string, int) (string, int) { return `{"class":"MAIN","holder":"maybe"}`, 0 }
		}, "the checkout holder classification was unreadable", "stop-condition infrastructure checkout-holder-classification-was-unreadable checkout-holder ", ""},
		{"brain status", func(ops *fakeOps) {
			ops.verdict = strings.Replace(ops.verdict, `"brainStatusDue":false`, `"brainStatusDue":true`, 1)
			ops.channelPost = func(_, stderr io.Writer) int { fmt.Fprintln(stderr, "channel refused"); return 1 }
		}, "", "", "the brain's status line was not published: channel refused"},
		{"brain status without provider", func(ops *fakeOps) {
			ops.verdict = strings.Replace(ops.verdict, `"brainStatusDue":false`, `"brainStatusDue":true`, 1)
		}, "", "", "the brain's status line was not published: no channel provider is configured"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			installation, ops, run := stopOnce(t, test.configure, `{"session_id":"external-failure"}`)
			if run.status != 0 || strings.Contains(run.stdout, `"decision":"block"`) {
				t.Fatalf("%s = status %d stdout %q", test.name, run.status, run.stdout)
			}
			if test.failure != "" && !strings.HasPrefix(presentedFailures(t, ops), test.failure) {
				t.Fatalf("%s failures = %q", test.name, presentedFailures(t, ops))
			}
			if test.logLine != "" && !strings.Contains(readHookLog(t, installation), test.logLine) {
				t.Fatalf("%s hook log = %q", test.name, readHookLog(t, installation))
			}
			if test.notice != "" {
				notices := ops.captured[0].notices
				if !strings.Contains(notices, test.notice) {
					t.Fatalf("%s notices = %q", test.name, notices)
				}
			}
		})
	}
}

// An unreadable refusal record never recreates a blocking loop.
func TestStopUnreadableRefusalRecordAllows(t *testing.T) {
	t.Parallel()
	installation := newHookInstallation(t)
	record := filepath.Join(installation.root, "artifacts", "agents", "supervision", "stop-refusals", "broken-refusal-record.json")
	if err := os.MkdirAll(filepath.Dir(record), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(record, []byte("{broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, block := range []bool{false, true} {
		ops := newFakeOps(t, installation)
		ops.up = func(_ UpRequest, stdout io.Writer, _ io.Writer) int {
			fmt.Fprintln(stdout, "up outcome=failed")
			return 44
		}
		if block {
			ops.verdict = strings.Replace(ops.verdict, `"shouldBlock":false`, `"shouldBlock":true`, 1)
		}
		run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "stop", payload: `{"session_id":"broken-refusal-record"}`})
		notices := ops.captured[0].notices
		if run.status != 0 || !strings.Contains(notices, "Metasystem stop-refusal record failure: the record could not be read or atomically updated.") {
			t.Fatalf("block=%t: status %d stdout %q notices %q", block, run.status, run.stdout, notices)
		}
		if !block && strings.Contains(run.stdout, `"decision":"block"`) {
			t.Fatalf("an unreadable refusal record recreated a blocking loop: %q", run.stdout)
		}
	}
}

// A verdict whose own state is unreadable is published as the degraded
// infrastructure display, never an all-clear.
func TestStopInfrastructureVerdictClass(t *testing.T) {
	t.Parallel()
	installation, ops, run := stopOnce(t, func(ops *fakeOps) {
		ops.verdict = `{"schemaVersion":1,"class":"infrastructure","causeCode":"turn-verdict-state","component":"verdict-state","shouldBlock":false,"display":"turn verdict state unreadable","surfaceWatchdog":false,"idleRefusal":false,"brainStatusDue":false}`
	}, "")
	verdict := ops.captured[0].verdict
	if run.status != 0 || !strings.Contains(verdict, `"class":"infrastructure"`) {
		t.Fatalf("infrastructure verdict = status %d verdict %q", run.status, verdict)
	}
	if !strings.Contains(readHookLog(t, installation), "stop-condition infrastructure turn-verdict-state verdict-state 2 ") {
		t.Fatalf("hook log = %q", readHookLog(t, installation))
	}
	// An idle-backlog refusal whose count was not spent is logged as its own
	// uncounted refusal, not an infrastructure condition.
	installation, _, run = stopOnce(t, func(ops *fakeOps) {
		ops.verdict = `{"schemaVersion":1,"class":"idle-with-backlog","shouldBlock":true,"countSpent":false,"display":"IDLE WITH BACKLOG","surfaceWatchdog":false,"idleRefusal":true,"brainStatusDue":false}`
	}, "")
	if !strings.Contains(run.stdout, `"decision":"block"`) ||
		!regexp.MustCompile(`(?m)^stop-condition idle-with-backlog idle-refusal-count-not-spent verdict-state 2 [0-9]+ refused-uncounted$`).MatchString(readHookLog(t, installation)) {
		t.Fatalf("idle refusal = stdout %q log %q", run.stdout, readHookLog(t, installation))
	}
}

// A read-only advisor skips the seat judgment but still publishes its
// pointer and report before the delivery cursors advance.
func TestStopAdvisorDelivery(t *testing.T) {
	t.Parallel()
	_, ops, run := stopOnce(t, func(ops *fakeOps) {
		ops.classify = func(string, string, int) (string, int) {
			return `{"class":"MAIN","holder":false,"mainId":"advisor-main","announcement":{"ownerLineage":"advisor-lineage"}}`, 0
		}
		ops.protocolGrowth = func() (string, int) { return `{"message":"advisor protocol guidance","counts":{"unreported":1}}`, 0 }
		ops.digestPending = func() (string, int) {
			return `{"cursor":9,"message":"advisor delivery digest line","prefixSha256":"` + strings.Repeat("e", 64) + `"}`, 0
		}
		ops.up = func(_ UpRequest, stdout io.Writer, _ io.Writer) int {
			fmt.Fprintln(stdout, "component=checkout-lease outcome=advisor\nup outcome=advisor authority=read-only")
			return 0
		}
	}, `{"session_id":"advisor-delivery"}`)
	if run.status != 0 || !strings.Contains(run.stdout, "OWNED-ELSEWHERE") || !strings.Contains(run.stdout, "advisor protocol guidance") {
		t.Fatalf("advisor = status %d stdout %q", run.status, run.stdout)
	}
	if !ops.stopInputs[0].Advisor || ops.stopInputs[0].VerdictFile != "" || ops.called("report turn-verdict") || ops.called("lease renew") {
		t.Fatalf("advisor ran the seat judgment: %s", ops.trace())
	}
	if !ops.called("steward digest-advance cursor=9") || !ops.called(`lease protocol-advance main=advisor-main caller=4321 counts={"unreported":1}`) {
		t.Fatalf("advisor delivery did not advance its cursors: %s", ops.trace())
	}
	if completion := ops.completion(t); completion.Result != "OK" || completion.Outcome != "EMITTED" {
		t.Fatalf("advisor completion = %+v", completion)
	}
}

// An older completion owner that refuses the elapsed measurement is retried
// once without it.
func TestStopCompletionRetriesWithoutElapsedOnce(t *testing.T) {
	t.Parallel()
	_, ops, _ := stopOnce(t, func(ops *fakeOps) {
		ops.hookComplete = func(request HookCompletion) int {
			if request.HasElapsed {
				return 2
			}
			return 0
		}
	}, "")
	if len(ops.completions) != 2 || !ops.completions[0].HasElapsed || ops.completions[1].HasElapsed || ops.completions[1].Outcome != "EMITTED" {
		t.Fatalf("completions = %+v", ops.completions)
	}
}

// A payload that cannot be emitted is recorded as such.
func TestStopEmissionFailureIsRecorded(t *testing.T) {
	t.Parallel()
	installation := newHookInstallation(t)
	ops := newFakeOps(t, installation)
	runHook(t, installation, ops, hookCall{runtime: "claude", event: "stop", payload: `{"session_id":"s"}`, stdout: closedWriter{}})
	if completion := ops.completion(t); completion.Result != "ERROR" || completion.Outcome != "EMISSION_FAILED" || completion.PayloadFile == "" {
		t.Fatalf("completion = %+v", completion)
	}
	if ops.called("steward digest-advance") {
		t.Fatalf("an unemitted payload advanced the digest: %s", ops.trace())
	}
}

// A Stop with no identified runtime runs only the restricted recovery and
// never classifies a holder.
func TestStopWithoutIdentityRecoversOnly(t *testing.T) {
	t.Parallel()
	_, ops, run := stopOnce(t, func(ops *fakeOps) {
		ops.findAncestor = func(string, int, string, bool) (string, int) { return "", 1 }
		ops.classify = func(string, string, int) (string, int) { return "", 1 }
	}, "")
	up := ops.upRequests[0]
	if !up.RecoverOnly || !up.IfDown || up.Pid != "" || run.status != 0 {
		t.Fatalf("unidentified stop up = %+v", up)
	}
	if !strings.HasPrefix(presentedFailures(t, ops), "the fallback runtime identity could not be classified") {
		t.Fatalf("failures = %q", presentedFailures(t, ops))
	}
}

// A holder view without its optional presentation projection is healthy.
func TestStopHolderWithoutOptionalProjection(t *testing.T) {
	t.Parallel()
	installation, ops, _ := stopOnce(t, func(ops *fakeOps) {
		ops.classify = func(string, string, int) (string, int) {
			return `{"class":"HOLDER","holder":true,"mainId":"main-fixture"}`, 0
		}
		ops.git = func(args ...string) (string, error) {
			if strings.Contains(strings.Join(args, " "), "metasystem.goal.machine") {
				return "", fmt.Errorf("unset")
			}
			return ops.ordinaryGit(args...)
		}
	}, "")
	if presentedFailures(t, ops) != "" || strings.Contains(readHookLog(t, installation), "stop-condition") {
		t.Fatalf("an optional projection requested repair: failures %q log %q", presentedFailures(t, ops), readHookLog(t, installation))
	}
	if input := ops.stopInputs[0]; input.ClaimEpoch != "0" || input.Lineage != "main-fixture" || input.Machine != "" {
		t.Fatalf("presentation coordinates = %+v", input)
	}
}

// An identity the stop cannot read is a recorded failure, never a guess; an
// announced main recorded by the classifier stands in for a missing ancestor.
func TestStopIdentityEvidenceIsReadOrRecorded(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		configure func(*fakeOps)
		failure   string
		pid       string
	}{
		{"ancestor without runtime", func(ops *fakeOps) {
			ops.findAncestor = func(string, int, string, bool) (string, int) { return `{"pid":4321,"pidStartedAt":1700000000}`, 0 }
		}, "the runtime identity was unreadable", ""},
		{"ancestor without a process start", func(ops *fakeOps) {
			ops.findAncestor = func(string, int, string, bool) (string, int) { return `{"runtime":"claude","pid":4321}`, 0 }
		}, "the runtime identity was unreadable", ""},
		{"announced main without runtime", func(ops *fakeOps) {
			ops.findAncestor = func(string, int, string, bool) (string, int) { return "", 1 }
			ops.classify = func(string, string, int) (string, int) { return `{"class":"MAIN","announcement":{"pid":4321}}`, 0 }
		}, "the fallback runtime identity was unreadable", ""},
		{"announced main stands in", func(ops *fakeOps) {
			ops.findAncestor = func(string, int, string, bool) (string, int) { return "", 1 }
		}, "", "4321"},
		{"holder unclassified", func(ops *fakeOps) {
			ops.classify = func(string, string, int) (string, int) { return "", 1 }
		}, "the checkout holder could not be classified", "4321"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, ops, run := stopOnce(t, test.configure, "")
			if run.status != 0 {
				t.Fatalf("stop = status %d stderr %q", run.status, run.stderr)
			}
			failures := presentedFailures(t, ops)
			if (test.failure == "" && failures != "") || !strings.HasPrefix(failures, test.failure) {
				t.Fatalf("failures = %q, want %q", failures, test.failure)
			}
			if up := ops.upRequests[0]; up.Pid != test.pid {
				t.Fatalf("up identity = %q, want %q (%+v)", up.Pid, test.pid, up)
			}
		})
	}
}

// A Stop that cannot stage its presentation allows in the staging form.
func TestStopStagingFailureAllows(t *testing.T) {
	t.Parallel()
	installation := newHookInstallation(t)
	ops := newFakeOps(t, installation)
	run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "stop", payload: `{"session_id":"line-fixture"}`,
		tempDir: filepath.Join(t.TempDir(), "absent")})
	if run.status != 0 || run.stdout != mustForm(t, "allowed", "staging-failed")+"\n" {
		t.Fatalf("staging failure = status %d stdout %q", run.status, run.stdout)
	}
	if ops.called("lease classify") {
		t.Fatalf("an unstaged stop went on to classify: %s", ops.trace())
	}
}

// Witness for the Stop cost trace: a slow owner call is named in hooks.log
// with its own milliseconds, on an artificial clock the fake owners advance.
// Before the trace a 20-43 s Stop left only its total elapsed seconds.
func TestStopTracesEachOwnerCallsCost(t *testing.T) {
	t.Parallel()
	started := time.Now().Add(-3 * time.Second).Unix()
	installation := newHookInstallation(t)
	ops := newFakeOps(t, installation)
	var clock atomic.Int64
	advance := func(d time.Duration) { clock.Add(int64(d)) }
	ops.turnVerdict = func(request TurnVerdictRequest) (string, string, int) {
		advance(7 * time.Second)
		_ = os.WriteFile(request.FactsFile, []byte("{}\n"), 0o600)
		_ = os.WriteFile(request.CompletionFile, []byte("{}\n"), 0o600)
		return ops.verdict + "\n", "", 0
	}
	ops.evidenceGC = func(io.Writer) int {
		advance(1500 * time.Millisecond)
		return 0
	}
	run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "stop", payload: `{"session_id":"trace-fixture","transcript_path":"/transcripts/t.jsonl"}`,
		env:       map[string]string{stopDeadlineParentEnv: "777", stopDeadlineStartedEnv: fmt.Sprint(started)},
		monotonic: func() time.Duration { return time.Duration(clock.Load()) }})
	if run.status != 0 {
		t.Fatalf("stop = status %d stdout %q stderr %q", run.status, run.stdout, run.stderr)
	}
	log := readHookLog(t, installation)
	line := regexp.MustCompile(`(?m) stop phases (.*)$`).FindStringSubmatch(log)
	if line == nil {
		t.Fatalf("the Stop left no cost trace: %q", log)
	}
	for _, want := range []string{"verdict=7000ms", "evidence-gc=1500ms", "up=0ms", "health=0ms", "attempt=0ms"} {
		if !strings.Contains(" "+line[1]+" ", " "+want+" ") {
			t.Fatalf("the cost trace %q does not name %s", line[1], want)
		}
	}
}

// Witness for the Stop's biggest avoidable cost: evidence collection (4-5 s
// of every Stop on seat m1e, grace 90 minutes) runs at most once per
// interval on an artificial wall clock, and a skipped Stop still logs.
func TestStopRunsEvidenceCollectionAtMostOncePerInterval(t *testing.T) {
	t.Parallel()
	started := time.Now().Add(-3 * time.Second).Unix()
	installation := newHookInstallation(t)
	ops := newFakeOps(t, installation)
	var collections atomic.Int32
	ops.evidenceGC = func(io.Writer) int {
		collections.Add(1)
		return 0
	}
	wall := time.Date(2026, 9, 27, 19, 0, 0, 0, time.UTC)
	stop := func(at time.Time) {
		t.Helper()
		run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "stop", payload: `{"session_id":"gc-cadence","transcript_path":"/transcripts/t.jsonl"}`,
			env: map[string]string{stopDeadlineParentEnv: "777", stopDeadlineStartedEnv: fmt.Sprint(started)},
			now: func() time.Time { return at }})
		if run.status != 0 {
			t.Fatalf("stop at %s = status %d stderr %q", at, run.status, run.stderr)
		}
	}
	stop(wall)
	stop(wall.Add(time.Minute))
	stop(wall.Add(14 * time.Minute))
	if got := collections.Load(); got != 1 {
		t.Fatalf("evidence collection ran %d times within one interval", got)
	}
	if !strings.Contains(readHookLog(t, installation), " evidence-gc skipped: last ran 2026-09-27T19:00:00Z; next after 2026-09-27T19:15:00Z\n") {
		t.Fatalf("a skipped collection left no line: %q", readHookLog(t, installation))
	}
	stop(wall.Add(15 * time.Minute))
	if got := collections.Load(); got != 2 {
		t.Fatalf("evidence collection did not run once the interval passed: %d", got)
	}
}
