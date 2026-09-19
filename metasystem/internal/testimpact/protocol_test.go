package testimpact

import (
	"bytes"
	"context"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func protocolRequest(root, mode string) Request {
	return Request{SchemaVersion: 1, Mode: mode, Base: strings.Repeat("a", 40), RepositoryRoot: root,
		ChangedPaths: []string{"src/a.java"}, Binding: strings.Repeat("b", 64)}
}
func protocolResult(request Request) Result {
	return Result{SchemaVersion: 1, Mode: request.Mode, Binding: request.Binding, Status: StatusPassed,
		Selections: []Selection{{Label: "unit", CWD: ".", Argv: []string{"test"}, Scope: ScopeNamed,
			Tests: []string{"T"}, Reasons: []Reason{{Test: "T", Because: "changed"}}, Result: StatusPassed, Seconds: 1}},
		Uncertainty: []Uncertainty{}}
}
func TestImpactProtocol(t *testing.T) {
	t.Parallel()
	request := protocolRequest(t.TempDir(), ModeRun)
	requestJSON, _ := Encode(request)
	resultJSON, _ := Encode(protocolResult(request))
	bad := map[string][]byte{
		"request missing":      bytes.Replace(requestJSON, []byte(`"base":"`+request.Base+`",`), nil, 1),
		"request unknown":      bytes.Replace(requestJSON, []byte(`{"schemaVersion"`), []byte(`{"extra":1,"schemaVersion"`), 1),
		"request duplicate":    bytes.Replace(requestJSON, []byte(`{"schemaVersion":1`), []byte(`{"schemaVersion":1,"schemaVersion":1`), 1),
		"request null":         bytes.Replace(requestJSON, []byte(`"changedPaths":["src/a.java"]`), []byte(`"changedPaths":null`), 1),
		"request wrong type":   bytes.Replace(requestJSON, []byte(`"mode":"run"`), []byte(`"mode":1`), 1),
		"request trailing":     append(requestJSON, []byte(` {}`)...),
		"request invalid utf8": append(requestJSON[:len(requestJSON)-1], 0xff),
	}
	for name, document := range bad {
		_, err := DecodeRequest(document)
		require(t, err != nil, "%s accepted", name)
	}
	badResult := map[string][]byte{
		"result missing":   bytes.Replace(resultJSON, []byte(`"status":"passed",`), nil, 1),
		"nested unknown":   bytes.Replace(resultJSON, []byte(`"label":"unit"`), []byte(`"extra":1,"label":"unit"`), 1),
		"nested duplicate": bytes.Replace(resultJSON, []byte(`"test":"T"`), []byte(`"test":"T","test":"T"`), 1),
		"result trailing":  append(resultJSON, []byte(`false`)...),
	}
	for name, document := range badResult {
		_, err := DecodeResult(document, request)
		require(t, err != nil, "%s accepted", name)
	}
	valid := map[string]Result{"passed": protocolResult(request), "failed": outcome(request, StatusFailed), "incomplete": outcome(request, StatusIncomplete),
		"nothing": {SchemaVersion: 1, Mode: ModeRun, Binding: request.Binding, Status: StatusNothingSelected, Selections: []Selection{}, Uncertainty: []Uncertainty{}}}
	for name, result := range valid {
		err := ValidateResult(request, result)
		require(t, err == nil, "%s: %v", name, err)
	}
	listed := request
	listed.Mode = ModeList
	listResult := protocolResult(listed)
	listResult.Status, listResult.Selections[0].Result, listResult.Selections[0].Seconds = StatusListed, StatusNotRun, 0
	require(t, ValidateResult(listed, listResult) == nil, "listed rejected")
	invalid := map[string]Result{
		"passed empty":            {SchemaVersion: 1, Mode: ModeRun, Binding: request.Binding, Status: StatusPassed, Selections: []Selection{}, Uncertainty: []Uncertainty{}},
		"failed without failure":  func() Result { r := protocolResult(request); r.Status = StatusFailed; return r }(),
		"incomplete with failure": func() Result { r := outcome(request, StatusFailed); r.Status = StatusIncomplete; return r }(),
		"binding mismatch":        func() Result { r := protocolResult(request); r.Binding = strings.Repeat("c", 64); return r }(),
		"duplicate label": func() Result {
			r := protocolResult(request)
			r.Selections = append(r.Selections, r.Selections[0])
			return r
		}(),
		"named reason missing":   func() Result { r := protocolResult(request); r.Selections[0].Reasons = []Reason{}; return r }(),
		"broad wildcard missing": func() Result { r := protocolResult(request); r.Selections[0].Scope = ScopePackage; return r }(),
		"negative seconds":       func() Result { r := protocolResult(request); r.Selections[0].Seconds = -1; return r }(),
	}
	for name, result := range invalid {
		err := ValidateResult(request, result)
		require(t, err != nil, "%s accepted", name)
	}
	require(t, resultExit(outcome(request, StatusFailed)) == 1 && resultExit(protocolResult(request)) == 0, "result exit mapping changed")
}
func outcome(request Request, status string) Result {
	result := protocolResult(request)
	result.Status, result.Selections[0].Result = status, status
	return result
}

type signalWriter chan string

func (writer signalWriter) Write(data []byte) (int, error) {
	writer <- string(data)
	return len(data), nil
}
func TestExternalProvider(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "testing-impacted-protocol.md"))
	require(t, err == nil, "%v", err)
	example := strings.Split(strings.Split(string(doc), "<!-- BEGIN PROVIDER EXAMPLE -->")[1], "<!-- END PROVIDER EXAMPLE -->")[0]
	example = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(example), "```sh"), "```"))
	writeExecutable(t, filepath.Join(root, "provider.sh"), example)
	writeExecutable(t, filepath.Join(root, "gradlew"), "#!/bin/sh\nprintf 'gradle-log\\n' >&2\nexit 0\n")
	request := protocolRequest(root, ModeRun)
	var progress bytes.Buffer
	provider, err := RunProvider(context.Background(), testpolicy.Impacted{CWD: ".", Argv: []string{"./provider.sh"}}, root, request, Executor{}, &progress)
	require(t, err == nil && provider.Result.Status == StatusPassed && strings.Contains(progress.String(), "gradle-log"), "documented provider: status=%q stderr=%q err=%v", provider.Result.Status, progress.String(), err)
	input, _ := Encode(request)
	for name, malformed := range map[string]string{"missing": `{}`, "unknown": strings.Replace(string(input), `{"schemaVersion"`, `{"extra":1,"schemaVersion"`, 1), "duplicate": strings.Replace(string(input), `{"schemaVersion":1`, `{"schemaVersion":1,"schemaVersion":1`, 1), "trailing": string(input) + `{}`} {
		command := exec.Command(filepath.Join(root, "provider.sh"))
		command.Dir, command.Stdin = root, strings.NewReader(malformed)
		require(t, command.Run() != nil, "documented provider accepted %s request", name)
	}
	writeExecutable(t, filepath.Join(root, "gradlew"), "#!/bin/sh\nprintf 'Could not resolve dependency in offline mode\\n' >&2\nexit 1\n")
	provider, err = RunProvider(context.Background(), testpolicy.Impacted{CWD: ".", Argv: []string{"./provider.sh"}}, root, request, Executor{}, &progress)
	require(t, err == nil && provider.Result.Status == StatusIncomplete, "offline dependency: status=%q err=%v", provider.Result.Status, err)
	validJSON, err := Encode(protocolResult(request))
	require(t, err == nil, "%v", err)
	writeExecutable(t, filepath.Join(root, "bad.sh"), "#!/bin/sh\nprintf '%s\\n' '"+strings.TrimSpace(string(validJSON))+"'\nexit 7\n")
	bad, err := RunProvider(context.Background(), testpolicy.Impacted{CWD: ".", Argv: []string{"./bad.sh"}}, root, request, Executor{}, &progress)
	require(t, err != nil && bad.Run.ExitCode == 7, "provider nonzero did not override stdout: %#v, %v", bad, err)
	self, _ := os.Executable()
	resolved, _ := RunProvider(context.Background(), testpolicy.Impacted{CWD: ".", Argv: []string{"metasystem", "-test.run=^$"}}, root, request, Executor{}, &progress)
	require(t, resolved.Argv[0] == self, "metasystem resolved to %q, want %q", resolved.Argv[0], self)
	writeExecutable(t, filepath.Join(root, "cancel.sh"), "#!/bin/sh\ntrap 'printf term >&2' TERM\nprintf ready >&2\nwhile :; do :; done\n")
	called, fire := make(chan time.Duration), make(chan time.Time, 1)
	clock := func(duration time.Duration) <-chan time.Time { called <- duration; return fire }
	writer := make(signalWriter, 2)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := RunProvider(ctx, testpolicy.Impacted{CWD: ".", Argv: []string{"./cancel.sh"}}, root, request, Executor{Clock: clock}, writer)
		done <- err
	}()
	require(t, strings.Contains(<-writer, "ready"), "provider did not become ready")
	cancel()
	grace := <-called
	require(t, grace == 2*time.Second, "grace=%s", grace)
	require(t, strings.Contains(<-writer, "term"), "child did not receive SIGTERM")
	fire <- time.Unix(0, 0)
	err = <-done
	require(t, err != nil && strings.Contains(err.Error(), "cancelled"), "cancellation: %v", err)
}
func writeExecutable(t *testing.T, path, body string) {
	require(t, os.MkdirAll(filepath.Dir(path), 0o755) == nil, "mkdir %s", path)
	require(t, testexec.WriteFile(path, []byte(body), 0o755) == nil, "write %s", path)
}
func require(t *testing.T, condition bool, format string, arguments ...any) {
	if condition {
		return
	}
	t.Fatalf(format, arguments...)
}
