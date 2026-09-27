package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// SessionStart outcome tests, ported from the Bash 3.2 start-boundary matrix
// (internal/audit/hookstartexits_test.go) and the start legs of the hook
// fixture beds. The hook-start audit joins StartOutcomeNotices and
// StartIntentionalOutcomes to the cases below.

const startFallback = startLastResort

func noticeOf(key string) string { return StartOutcomeNotices[key].text + "\n" }

// finishDirect runs one declared outcome through the start owner, the way
// the shell matrix injected `start_finish` at the top of start_main.
func finishDirect(t *testing.T, setup func(*startRun), family, key string, stdout io.Writer) hookRun {
	t.Helper()
	var buffer, stderr bytes.Buffer
	if stdout == nil {
		stdout = &buffer
	}
	installation := newHookInstallation(t)
	s := &startRun{inv: Invocation{Runtime: "fake", Event: "start", Stdout: stdout, Stderr: &stderr, Script: installation.script}.withDefaults(),
		ops: newFakeOps(t, installation), contextKind: "none", contextBytes: 2048}
	if setup != nil {
		setup(s)
	}
	status := func() (status int) {
		defer func() {
			if exit, ok := recover().(hookExit); ok {
				status = exit.status
			}
		}()
		s.finish(family, key)
		return -1
	}()
	return hookRun{status: status, stdout: buffer.String(), stderr: stderr.String()}
}

// hook-start-case: every catalog key is published with its own status.
func TestHookStartDeclaredOutcomeMatrix(t *testing.T) {
	statuses := map[string]int{"invocation-invalid": 2, "runtime-unregistered": 2, "custody-unreadable": 1, "interrupted": 143}
	for key := range StartOutcomeNotices {
		t.Run(key, func(t *testing.T) {
			run := finishDirect(t, nil, "notice", key, nil)
			if run.status != statuses[key] || run.stdout != noticeOf(key) || run.stderr != "" {
				t.Fatalf("outcome = status %d stdout %q stderr %q", run.status, run.stdout, run.stderr)
			}
		})
	}
	intentional := []struct {
		key            string
		setup          func(*startRun)
		stdout, stderr string
	}{
		{"authenticated-delegate", nil, "", "Metasystem SessionStart intentionally skipped: authenticated delegate; its launcher owns context.\n"},
		{"foreign-runtime", nil, "", "Metasystem SessionStart intentionally skipped: another runtime owns this process.\n"},
		{"context-ready", channelContext("role packet"), `{"hookSpecificOutput":{"additionalContext":"role packet","hookEventName":"SessionStart"}}` + "\n", ""},
		{"screen-context-ready", func(s *startRun) { s.contextKind, s.notices = "screen", "screen context" }, `{"systemMessage":"screen context"}` + "\n", ""},
		{"notices-ready", func(s *startRun) { s.notices = "notice text" }, `{"systemMessage":"notice text"}` + "\n", ""},
		{"healthy-no-context", nil, "{}\n", ""},
	}
	if len(intentional) != len(StartIntentionalOutcomes) {
		t.Fatalf("intentional matrix has %d rows; the catalog has %d", len(intentional), len(StartIntentionalOutcomes))
	}
	for _, test := range intentional {
		t.Run(test.key, func(t *testing.T) {
			run := finishDirect(t, test.setup, "intentional", test.key, nil)
			if run.status != 0 || run.stdout != test.stdout || run.stderr != test.stderr {
				t.Fatalf("outcome = status %d stdout %q stderr %q", run.status, run.stdout, run.stderr)
			}
		})
	}
	t.Run("undeclared outcome", func(t *testing.T) {
		run := finishDirect(t, nil, "notice", "undeclared", nil)
		if run.status != 0 || run.stdout != startFallback+"\n" {
			t.Fatalf("undeclared outcome = status %d stdout %q", run.status, run.stdout)
		}
	})
}

func channelContext(payload string) func(*startRun) {
	return func(s *startRun) {
		s.contextKind, s.contextField, s.contextEvent, s.contextPayload = "channel", "hookSpecificOutput.additionalContext", "SessionStart", payload
		s.prepareContextObject()
	}
}

func TestHookStartContextOutcomeShapes(t *testing.T) {
	tests := []struct {
		name, outcome string
		setup         func(*startRun)
		expected      string
	}{
		{"context-ready", "context-ready", channelContext("role packet"), `{"hookSpecificOutput":{"additionalContext":"role packet","hookEventName":"SessionStart"}}` + "\n"},
		{"screen-context-ready", "screen-context-ready", func(s *startRun) {
			s.contextKind, s.notices = "screen", screenOnlyNotice+"\nrole packet"
		}, `{"systemMessage":"this runtime has no session-context channel; the packet reached the screen only\nrole packet"}` + "\n"},
		{"context-ready-with-notice", "context-ready", func(s *startRun) {
			channelContext("role packet")(s)
			s.notices = "pending notice"
		}, `{"systemMessage":"pending notice","hookSpecificOutput":{"additionalContext":"role packet","hookEventName":"SessionStart"}}` + "\n"},
		{"context-ready-without-event", "context-ready", func(s *startRun) {
			s.contextKind, s.contextField, s.contextPayload = "channel", "hookSpecificOutput.additionalContext", "role packet"
			s.preparedContext = `{"hookSpecificOutput":{"additionalContext":"role packet"}}`
		}, startFallback + "\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			run := finishDirect(t, test.setup, "intentional", test.outcome, nil)
			if run.status != 0 || run.stdout != test.expected || run.stderr != "" || strings.Count(run.stdout, "\n") != 1 {
				t.Fatalf("context outcome = status %d stdout %q stderr %q", run.status, run.stdout, run.stderr)
			}
		})
	}
}

func TestHookStartJSONEncoder(t *testing.T) {
	message := "quote\" slash\\ newline\n tab\t carriage\r back\b form\f utf8 héllo \x01"
	run := finishDirect(t, func(s *startRun) { s.notices = message }, "intentional", "notices-ready", nil)
	encoded, err := json.Marshal(map[string]string{"systemMessage": message})
	if err != nil {
		t.Fatal(err)
	}
	if run.status != 0 || run.stdout != string(encoded)+"\n" {
		t.Fatalf("JSON encoding = status %d stdout %q", run.status, run.stdout)
	}
}

// fullPathOps configures the fake owners the way the shell matrix's
// full-path fixture engine answered each mode.
func fullPathOps(t *testing.T, installation hookInstallation, mode string) *fakeOps {
	ops := newFakeOps(t, installation)
	contextModes := map[string]bool{"context": true, "context-trailing-newline": true, "context-arming-failure": true,
		"context-arming-rearmed": true, "context-pending": true, "context-holder-read": true, "context-process-identity": true,
		"context-wait-failure": true, "context-wait-rows": true, "context-digest": true, "context-corrupt": true,
		"acknowledgment-failure": true, "signal-after-publication": true}
	declaredModes := map[string]bool{"screen": true, "screen-holder-read": true, "screen-arming-failure": true}
	for key := range contextModes {
		declaredModes[key] = true
	}
	ops.identityPid = 1
	ops.findAncestor = func(string, int, string, bool) (string, int) {
		switch mode {
		case "process-identity", "context-process-identity":
			return "", 7
		case "holder-read", "context-holder-read", "screen-holder-read":
			return "", 0
		case "foreign":
			return `{"runtime":"fake","pid":1,"pidStartedAt":1}` + "\n", 0
		}
		return `{"runtime":"claude","pid":1,"pidStartedAt":1}` + "\n", 0
	}
	ops.classify = func(string, string, int) (string, int) { return "", 7 }
	ops.hookDelegate = func(string, string, string, int) (string, int) {
		if mode == "delegate" {
			return `{"delegate":true,"jobId":"fixture","matchedPid":1,"comparisonMode":"legacy-seconds"}` + "\n", 0
		}
		return "", 3
	}
	ops.startContext = func(string) (string, int) {
		if mode == "context-contract" {
			return "malformed\n", 0
		}
		if contextModes[mode] {
			return "field=hookSpecificOutput.additionalContext event=SessionStart bytes=2048 sources=startup,compact\n", 0
		}
		return "", 1
	}
	ops.stewardPending = func(string) (string, int) {
		switch mode {
		case "pending-read":
			return "", 7
		case "notices", "context-pending":
			return "incident\n", 0
		}
		return "", 0
	}
	ops.brainBoot = func(ctx context.Context) (string, string, int) {
		switch mode {
		case "brain-boot":
			return "", "", 7
		case "brain-timeout":
			<-ctx.Done()
			return "", "", 0
		case "context-trailing-newline":
			return `{"declared":true,"state":"declared","payload":"role packet\n","bytes":12,"sections":{"asks":"complete","held":"complete","fleet":"complete","digest":"complete"},"digestEmitted":false,"digestCursor":0,"digestPrefixSha256":"","declarationSha256":"` + strings.Repeat("a", 64) + `"}`, "", 0
		case "context-digest":
			return `{"declared":true,"state":"declared","payload":"role packet\nNARRATOR DIGEST warning","bytes":35,"sections":{"asks":"complete","held":"complete","fleet":"complete","digest":"complete"},"digestEmitted":true,"digestCursor":7,"digestPrefixSha256":"` + strings.Repeat("b", 64) + `","declarationSha256":"` + strings.Repeat("a", 64) + `"}`, "", 0
		case "context-corrupt":
			return `{"declared":true,"state":"corrupt","payload":"BRAIN SEAT warning\nrole packet","bytes":30,"sections":{"asks":"error","held":"skipped","fleet":"skipped","digest":"skipped"},"digestEmitted":false,"digestCursor":0,"digestPrefixSha256":"","declarationSha256":""}`, "", 0
		}
		if declaredModes[mode] {
			return `{"declared":true,"state":"declared","payload":"role packet","bytes":11,"sections":{"asks":"complete","held":"complete","fleet":"complete","digest":"complete"},"digestEmitted":false,"digestCursor":0,"digestPrefixSha256":"","declarationSha256":"` + strings.Repeat("a", 64) + `"}`, "", 0
		}
		return `{"declared":false}` + "\n", "", 0
	}
	ops.up = func(_ UpRequest, stdout, stderr io.Writer) int {
		switch mode {
		case "rearm-success":
			fmt.Fprintln(stdout, `up outcome=armed authority=writer re-armed="generation=9 previous=8"`)
			return 0
		case "arming-rearmed", "context-arming-rearmed":
			fmt.Fprintln(stdout, `up outcome=failed re-armed="generation=9 previous=8" component=steward-runner`)
			return 7
		case "arming":
			fmt.Fprintln(stdout, `up outcome=failed component=accepted-engine remedy="restart fixture"`)
			return 7
		case "context-arming-failure":
			fmt.Fprintln(stdout, `up outcome=ENROLLMENT_DRIFT component=accepted-engine remedy="restart fixture"`)
			return 7
		case "screen-arming-failure":
			fmt.Fprintln(stderr, `up outcome=failed component=accepted-engine`)
			return 7
		}
		return 0
	}
	ops.sessionStart = func(stdout, stderr io.Writer) int {
		switch mode {
		case "wait-recovery", "context-wait-failure":
			fmt.Fprintln(stderr, "first failure")
			fmt.Fprintln(stderr, "second failure")
			return 7
		case "context-wait-rows", "context-process-identity", "context-arming-failure", "brain-boot-wait", "brain-timeout-wait":
			fmt.Fprintln(stdout, "WAIT RECOVERY fixture row")
			return 0
		}
		return 64
	}
	ops.brainDelivered = func(string, string, string) int {
		if mode == "acknowledgment-failure" {
			return 7
		}
		return 0
	}
	switch mode {
	case "runtime-registry":
		ops.runtimeNames = func() (string, int) { return "", 7 }
	case "runtime-unregistered":
		ops.runtimeNames = func() (string, int) { return "fake\n", 0 }
	case "installation-validation":
		ops.stateRoot = func(string) (string, int) { return "", 1 }
	case "resolved-directory":
		ops.stateRoot = func(installation string) (string, int) { return installation + "/disappeared\n", 0 }
	}
	return ops
}

// hook-start-case: the full start path publishes each intentional shape.
func TestHookStartIntentionalFullPathFixtures(t *testing.T) {
	tests := []struct {
		mode, stdout, stderr  string
		context, acknowledged bool
		skipsArming           bool
	}{
		{mode: "healthy", stdout: "{}\n"},
		{mode: "notices", stdout: `{"systemMessage":"Steward incidents pending: incident"}` + "\n"},
		{mode: "context", stdout: `{"hookSpecificOutput":{"additionalContext":"role packet","hookEventName":"SessionStart"}}` + "\n", context: true, acknowledged: true},
		{mode: "context-trailing-newline", stdout: `{"hookSpecificOutput":{"additionalContext":"role packet\n","hookEventName":"SessionStart"}}` + "\n", context: true, acknowledged: true},
		{mode: "context-arming-failure", stdout: `{"systemMessage":"Metasystem supervision arming failed: up outcome=ENROLLMENT_DRIFT component=accepted-engine remedy=\"restart fixture\"\nWAIT RECOVERY fixture row","hookSpecificOutput":{"additionalContext":"role packet","hookEventName":"SessionStart"}}` + "\n", context: true, acknowledged: true},
		{mode: "context-arming-rearmed", stdout: `{"systemMessage":"Metasystem supervision arming failed: up outcome=failed re-armed=\"generation=9 previous=8\" component=steward-runner\nMetasystem re-armed the rebuilt engine: up outcome=failed re-armed=\"generation=9 previous=8\" component=steward-runner","hookSpecificOutput":{"additionalContext":"role packet","hookEventName":"SessionStart"}}` + "\n", context: true, acknowledged: true},
		{mode: "context-pending", stdout: `{"systemMessage":"Steward incidents pending: incident","hookSpecificOutput":{"additionalContext":"role packet","hookEventName":"SessionStart"}}` + "\n", context: true, acknowledged: true},
		{mode: "context-holder-read", stdout: `{"systemMessage":"Metasystem supervision could not identify the immediate claude agent process; arming was refused.","hookSpecificOutput":{"additionalContext":"role packet","hookEventName":"SessionStart"}}` + "\n", context: true, acknowledged: true, skipsArming: true},
		{mode: "context-process-identity", stdout: `{"systemMessage":"Metasystem supervision could not identify the immediate claude agent process; arming was refused.\nWAIT RECOVERY fixture row","hookSpecificOutput":{"additionalContext":"role packet","hookEventName":"SessionStart"}}` + "\n", context: true, acknowledged: true, skipsArming: true},
		{mode: "context-wait-failure", stdout: `{"systemMessage":"Metasystem could not read durable wait recovery rows for this session: first failure; second failure","hookSpecificOutput":{"additionalContext":"role packet","hookEventName":"SessionStart"}}` + "\n", context: true, acknowledged: true},
		{mode: "context-wait-rows", stdout: `{"systemMessage":"WAIT RECOVERY fixture row","hookSpecificOutput":{"additionalContext":"role packet","hookEventName":"SessionStart"}}` + "\n", context: true, acknowledged: true},
		{mode: "context-digest", stdout: `{"hookSpecificOutput":{"additionalContext":"role packet\nNARRATOR DIGEST warning","hookEventName":"SessionStart"}}` + "\n", context: true, acknowledged: true},
		{mode: "context-corrupt", stdout: `{"hookSpecificOutput":{"additionalContext":"BRAIN SEAT warning\nrole packet","hookEventName":"SessionStart"}}` + "\n", context: true},
		{mode: "screen", stdout: `{"systemMessage":"this runtime has no session-context channel; the packet reached the screen only\nrole packet"}` + "\n"},
		{mode: "screen-holder-read", stdout: `{"systemMessage":"this runtime has no session-context channel; the packet reached the screen only\nrole packet\nMetasystem supervision could not identify the immediate claude agent process; arming was refused."}` + "\n", acknowledged: true, skipsArming: true},
		{mode: "screen-arming-failure", stdout: `{"systemMessage":"this runtime has no session-context channel; the packet reached the screen only\nrole packet\nMetasystem supervision arming failed: up outcome=failed component=accepted-engine"}` + "\n"},
		{mode: "rearm-success", stdout: `{"systemMessage":"Metasystem re-armed the rebuilt engine: up outcome=armed authority=writer re-armed=\"generation=9 previous=8\""}` + "\n"},
		{mode: "delegate", stderr: "Metasystem SessionStart intentionally skipped: authenticated delegate; its launcher owns context.\n"},
		{mode: "foreign", stderr: "Metasystem SessionStart intentionally skipped: another runtime owns this process.\n"},
	}
	for _, test := range tests {
		t.Run(test.mode, func(t *testing.T) {
			installation := newHookInstallation(t)
			ops := fullPathOps(t, installation, test.mode)
			run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "start", payload: `{"session_id":"full-path","source":"startup"}` + "\n"})
			if run.status != 0 || run.stdout != test.stdout || run.stderr != test.stderr {
				t.Fatalf("full path = status %d stdout %q stderr %q", run.status, run.stdout, run.stderr)
			}
			trace := ops.trace()
			switch {
			case test.mode == "delegate" || test.mode == "foreign":
				if ops.called("up ") || ops.called("brain boot") {
					t.Fatalf("%s skip continued into preparation: %s", test.mode, trace)
				}
			case test.skipsArming:
				if ops.called("up ") || !ops.called("session start ") {
					t.Fatalf("identity failure did not skip arming and preserve wait recovery: %s", trace)
				}
			default:
				if !ops.called("up ") || !ops.called("session start ") {
					t.Fatalf("successful completion skipped arming or wait recovery: %s", trace)
				}
			}
			if test.acknowledged && !ops.called("brain start-delivered ") {
				t.Fatalf("published declared context was not acknowledged: %s", trace)
			}
			if test.mode == "context-corrupt" && ops.called("brain start-delivered ") {
				t.Fatalf("a corrupt declaration was acknowledged: %s", trace)
			}
			if test.mode == "context-digest" && !ops.called("brain start-delivered sha="+strings.Repeat("a", 64)+" cursor=7 prefix="+strings.Repeat("b", 64)) {
				t.Fatalf("digest delivery did not acknowledge the emitted cursor: %s", trace)
			}
			if test.context {
				assertFullPathContextObject(t, run.stdout)
			}
		})
	}
}

func assertFullPathContextObject(t *testing.T, stdout string) {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSuffix(stdout, "\n")), &object); err != nil {
		t.Fatalf("full start path did not publish one JSON object: %v: %q", err, stdout)
	}
	var context struct {
		AdditionalContext string `json:"additionalContext"`
		HookEventName     string `json:"hookEventName"`
	}
	if err := json.Unmarshal(object["hookSpecificOutput"], &context); err != nil {
		t.Fatalf("full start path omitted hookSpecificOutput: %v: %q", err, stdout)
	}
	if context.HookEventName != "SessionStart" || context.AdditionalContext == "" {
		t.Fatalf("full start path context shape = event %q context %q", context.HookEventName, context.AdditionalContext)
	}
}

func appendSystemMessage(object, line string) string {
	object = strings.TrimSuffix(object, "\n")
	object = strings.TrimSuffix(object, `"}`)
	return object + `\n` + line + `"}` + "\n"
}

// hook-start-case: each reachable failure branch publishes its notice.
func TestHookStartFailureBranchFixtures(t *testing.T) {
	tests := []struct {
		key, mode, runtime string
		payload            string
		env                map[string]string
		mutate             func(hookInstallation, *fakeOps) hookInstallation
		stdout             string
		status             int
	}{
		{key: "engine-missing", mode: "healthy", mutate: func(i hookInstallation, _ *fakeOps) hookInstallation {
			_ = os.Remove(filepath.Join(i.root, "bin", "metasystem"))
			return i
		}},
		{key: "installation-directory", mode: "healthy", mutate: func(i hookInstallation, _ *fakeOps) hookInstallation {
			i.script = "/metasystem-fixture-directory-does-not-exist/supervision-hook.sh"
			return i
		}},
		{key: "checkout-identification", mode: "healthy", mutate: func(i hookInstallation, ops *fakeOps) hookInstallation {
			ops.git = func(...string) (string, error) { return "", errors.New("exit status 67") }
			return i
		}},
		{key: "resolved-directory", mode: "resolved-directory"},
		{key: "installation-validation", mode: "installation-validation"},
		{key: "payload-read", mode: "healthy", payload: "not json\n"},
		{key: "pending-read", mode: "pending-read"},
		{key: "holder-read", mode: "holder-read", stdout: `{"systemMessage":"Metasystem supervision could not identify the immediate claude agent process; arming was refused."}` + "\n"},
		{key: "invocation-invalid", mode: "healthy", runtime: "Bad"},
		{key: "runtime-unregistered", mode: "runtime-unregistered"},
		{key: "runtime-registry", mode: "runtime-registry"},
		{key: "context-contract", mode: "context-contract"},
		{key: "custody-unreadable", mode: "healthy", env: map[string]string{"METASYSTEM_HOOK_DELEGATE_STATE_ROOT": "incomplete"}},
		{key: "process-identity", mode: "process-identity", stdout: `{"systemMessage":"Metasystem supervision could not identify the immediate claude agent process; arming was refused."}` + "\n"},
		{key: "brain-boot", mode: "brain-boot"},
		{key: "brain-timeout", mode: "brain-timeout", env: map[string]string{"METASYSTEM_BRAIN_BOOT_DEADLINE_MS": "1"}},
		{key: "arming", mode: "arming", stdout: `{"systemMessage":"Metasystem supervision arming failed: up outcome=failed component=accepted-engine remedy=\"restart fixture\""}` + "\n"},
		{key: "arming", mode: "arming-rearmed", stdout: `{"systemMessage":"Metasystem supervision arming failed: up outcome=failed re-armed=\"generation=9 previous=8\" component=steward-runner\nMetasystem re-armed the rebuilt engine: up outcome=failed re-armed=\"generation=9 previous=8\" component=steward-runner"}` + "\n"},
		{key: "wait-recovery", mode: "wait-recovery", stdout: `{"systemMessage":"Metasystem could not read durable wait recovery rows for this session: first failure; second failure"}` + "\n"},
	}
	for _, test := range tests {
		t.Run(test.key+"/"+test.mode, func(t *testing.T) {
			installation := newHookInstallation(t)
			ops := fullPathOps(t, installation, test.mode)
			if test.mutate != nil {
				installation = test.mutate(installation, ops)
			}
			runtime := test.runtime
			if runtime == "" {
				runtime = "claude"
			}
			payload := test.payload
			if payload == "" {
				payload = "{}\n"
			}
			call := hookCall{runtime: runtime, event: "start", payload: payload, env: test.env}
			if test.key == "brain-timeout" {
				call.after = func(time.Duration) <-chan time.Time { return time.After(20 * time.Millisecond) }
			}
			run := runHook(t, installation, ops, call)
			want, wantStatus := test.stdout, StartOutcomeNotices[test.key].status
			if want == "" {
				want = noticeOf(test.key)
			}
			if test.key == "brain-boot" || test.key == "brain-timeout" {
				want = appendSystemMessage(want, "Supervision may have been partly initialized.")
				if !ops.called("up ") || !ops.called("session start ") {
					t.Fatalf("brain failure skipped arming or wait recovery: %s", ops.trace())
				}
			}
			if run.status != wantStatus || run.stdout != want || run.stderr != "" {
				t.Fatalf("branch = status %d stdout %q stderr %q; want status %d stdout %q", run.status, run.stdout, run.stderr, wantStatus, want)
			}
		})
	}
	t.Run("payload-storage", func(t *testing.T) {
		installation := newHookInstallation(t)
		ops := fullPathOps(t, installation, "healthy")
		var stdout, stderr bytes.Buffer
		status := RunRuntimeHook(Invocation{Runtime: "claude", Event: "start", Stdin: failingReader{}, Stdout: &stdout, Stderr: &stderr,
			Script: installation.script, Ppid: 1}, ops)
		if status != 0 || stdout.String() != noticeOf("payload-storage") {
			t.Fatalf("unreadable input = status %d stdout %q", status, stdout.String())
		}
	})
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("input closed") }

// hook-start-case: a brain failure still arms and publishes wait rows.
func TestHookStartBrainFailureKeepsWaitLine(t *testing.T) {
	for _, mode := range []string{"brain-boot", "brain-timeout"} {
		t.Run(mode, func(t *testing.T) {
			installation := newHookInstallation(t)
			ops := fullPathOps(t, installation, mode)
			ops.sessionStart = func(stdout, _ io.Writer) int {
				fmt.Fprintln(stdout, "WAIT RECOVERY fixture row")
				return 0
			}
			call := hookCall{runtime: "claude", event: "start", payload: `{"session_id":"brain-wait","source":"startup"}` + "\n"}
			if mode == "brain-timeout" {
				call.after = func(time.Duration) <-chan time.Time { return time.After(20 * time.Millisecond) }
			}
			run := runHook(t, installation, ops, call)
			want := appendSystemMessage(appendSystemMessage(noticeOf(mode), "WAIT RECOVERY fixture row"), "Supervision may have been partly initialized.")
			if run.status != 0 || run.stdout != want {
				t.Fatalf("brain failure = status %d stdout %q; want %q", run.status, run.stdout, want)
			}
			trace := ops.trace()
			brainAt, upAt, waitAt := strings.Index(trace, "brain boot"), strings.Index(trace, "\nup "), strings.Index(trace, "\nsession start ")
			if brainAt < 0 || upAt < brainAt || waitAt < upAt {
				t.Fatalf("brain failure did not reach arming and wait recovery in order: %s", trace)
			}
		})
	}
}

func TestHookStartForgedDelegateHintRefuses(t *testing.T) {
	installation := newHookInstallation(t)
	ops := fullPathOps(t, installation, "healthy")
	run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "start", payload: "{}\n", env: map[string]string{
		"METASYSTEM_HOOK_DELEGATE_STATE_ROOT":        installation.root,
		"METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT": installation.root,
		"METASYSTEM_HOOK_DELEGATE_JOB":               "job-forged",
	}})
	if run.status != 1 || run.stdout != noticeOf("custody-unreadable") || run.stderr != "" {
		t.Fatalf("forged delegate hint = status %d stdout %q stderr %q", run.status, run.stdout, run.stderr)
	}
	if strings.Count(ops.trace(), "lease hook-delegate ") != 1 || ops.called("runtime list") || ops.called("up ") || ops.called("session start ") {
		t.Fatalf("forged delegate hint continued after custody refusal: %s", ops.trace())
	}
}

// hook-start-case: the engine-skew notice for an engine whose state-root
// answer is refused or malformed.
func TestEngineSkewStartFixture(t *testing.T) {
	for _, test := range []struct {
		name   string
		answer string
		status int
	}{
		{"refusal", "", 2}, {"empty-success", "", 0}, {"lf-success", "ROOT\nextra\n", 0}, {"cr-success", "ROOT\r\n", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			installation := newHookInstallation(t)
			ops := newFakeOps(t, installation)
			ops.stateRoot = func(root string) (string, int) {
				return strings.ReplaceAll(test.answer, "ROOT", root), test.status
			}
			run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "start", payload: `{"session_id":"engine-skew","source":"startup"}` + "\n"})
			if run.status != 0 || run.stdout != noticeOf("engine-skew") {
				t.Fatalf("engine skew = status %d stdout %q", run.status, run.stdout)
			}
			if trace := ops.trace(); !strings.HasSuffix(trace, "runtime list\npath state-root "+installation.root+"\n") {
				t.Fatalf("resolver trace = %q", trace)
			}
		})
	}
}

// hook-start-case: after publication, failed bookkeeping or a signal is
// reported on stderr with status 1; a signal before publication interrupts.
func TestHookStartPostPreparationFixtures(t *testing.T) {
	bookkeeping := startBookkeepingNotice + "\n"
	context := `{"hookSpecificOutput":{"additionalContext":"role packet","hookEventName":"SessionStart"}}` + "\n"
	t.Run("acknowledgment-failure", func(t *testing.T) {
		installation := newHookInstallation(t)
		run := runHook(t, installation, fullPathOps(t, installation, "acknowledgment-failure"), hookCall{runtime: "claude", event: "start", payload: "{}\n"})
		if run.status != 1 || run.stdout != context || run.stderr != bookkeeping {
			t.Fatalf("acknowledgment failure = status %d stdout %q stderr %q", run.status, run.stdout, run.stderr)
		}
	})
	t.Run("signal-after-publication", func(t *testing.T) {
		installation := newHookInstallation(t)
		ops := fullPathOps(t, installation, "signal-after-publication")
		signals := make(chan os.Signal, 1)
		ops.brainDelivered = func(string, string, string) int {
			signals <- syscall.SIGTERM
			return 0
		}
		run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "start", payload: "{}\n", signals: signals})
		if run.status != 1 || run.stdout != context || run.stderr != bookkeeping {
			t.Fatalf("signal after publication = status %d stdout %q stderr %q", run.status, run.stdout, run.stderr)
		}
	})
	t.Run("signal-after-arming", func(t *testing.T) {
		installation := newHookInstallation(t)
		ops := fullPathOps(t, installation, "healthy")
		signals := make(chan os.Signal, 1)
		ops.up = func(UpRequest, io.Writer, io.Writer) int {
			signals <- syscall.SIGTERM
			return 0
		}
		run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "start", payload: "{}\n", signals: signals})
		want := appendSystemMessage(noticeOf("interrupted"), "Supervision may have been partly initialized.")
		if run.status != 143 || run.stdout != want || ops.called("session start ") {
			t.Fatalf("signal after arming = status %d stdout %q trace %s", run.status, run.stdout, ops.trace())
		}
	})
}

// hook-start-case: an unexpected failure inside the start still publishes
// the last-resort notice.
func TestHookStartUnexpectedTerminationPublishesTheLastResort(t *testing.T) {
	installation := newHookInstallation(t)
	ops := newFakeOps(t, installation)
	ops.stewardPending = func(string) (string, int) { panic("unexpected owner failure") }
	run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "start", payload: "{}\n"})
	if run.status != 0 || run.stdout != startFallback+"\n" {
		t.Fatalf("unexpected termination = status %d stdout %q", run.status, run.stdout)
	}
}

// hook-start-case: HUP, INT and TERM interrupt with their own statuses, also
// while the outcome is being finalized.
func TestHookStartSignalsUseOwnedStatuses(t *testing.T) {
	for _, test := range []struct {
		signal os.Signal
		status int
	}{{syscall.SIGHUP, 129}, {syscall.SIGINT, 130}, {syscall.SIGTERM, 143}} {
		t.Run(test.signal.String(), func(t *testing.T) {
			installation := newHookInstallation(t)
			signals := make(chan os.Signal, 1)
			signals <- test.signal
			run := runHook(t, installation, newFakeOps(t, installation), hookCall{runtime: "claude", event: "start", payload: "{}\n", signals: signals})
			if run.status != test.status || run.stdout != noticeOf("interrupted") {
				t.Fatalf("signal = status %d stdout %q", run.status, run.stdout)
			}
		})
	}
	t.Run("TERM while finalizing", func(t *testing.T) {
		installation := newHookInstallation(t)
		ops := newFakeOps(t, installation)
		signals := make(chan os.Signal, 1)
		ops.sessionStart = func(io.Writer, io.Writer) int {
			// Delivered during the last owner call; observed as the outcome
			// is decided.
			signals <- syscall.SIGTERM
			return 64
		}
		ops.startContext = func(string) (string, int) { return "", 1 }
		run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "start", payload: "{}\n", signals: signals})
		want := appendSystemMessage(noticeOf("interrupted"), "Supervision may have been partly initialized.")
		if run.status != 143 || run.stdout != want {
			t.Fatalf("deferred signal = status %d stdout %q", run.status, run.stdout)
		}
	})
}

type closedWriter struct{}

func (closedWriter) Write([]byte) (int, error) { return 0, syscall.EBADF }

// hook-start-case: with stdout closed the last resort goes to stderr, 74.
func TestHookStartLastResortUsesStderrWhenStdoutIsClosed(t *testing.T) {
	run := finishDirect(t, nil, "notice", "response-rendering", closedWriter{})
	if run.status != 74 || run.stderr != startFallback+"\n" {
		t.Fatalf("closed stdout = status %d stderr %q", run.status, run.stderr)
	}
}

func TestHookStartPublishesThroughOpenStdoutWithUnwritableModeBits(t *testing.T) {
	for _, test := range []struct{ name, key, expected string }{
		{"declared-notice", "installation-validation", noticeOf("installation-validation")},
		{"emergency-fallback", "undeclared", startFallback + "\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			stdout, err := os.OpenFile(filepath.Join(t.TempDir(), "stdout"), os.O_CREATE|os.O_RDWR, 0o600)
			if err != nil {
				t.Fatal(err)
			}
			defer stdout.Close()
			if err := stdout.Chmod(0); err != nil {
				t.Fatal(err)
			}
			run := finishDirect(t, nil, "notice", test.key, stdout)
			if _, err := stdout.Seek(0, 0); err != nil {
				t.Fatal(err)
			}
			output, err := io.ReadAll(stdout)
			if err != nil {
				t.Fatal(err)
			}
			if run.status != 0 || string(output) != test.expected {
				t.Fatalf("open stdout = status %d stdout %q", run.status, output)
			}
		})
	}
}
