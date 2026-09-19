package main

import (
	"bytes"
	"encoding/json"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testimpact"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestImpactedCLI(t *testing.T) {
	t.Parallel()
	code, stdout, _ := captureCommandOutput(t, true, true, func() int { return runTestImpacted([]string{"--bad", "--json"}) })
	var usage testimpact.Envelope
	cliRequire(t, json.Unmarshal([]byte(stdout), &usage) == nil && code == 2 && usage.Error != nil && usage.Error.Code == testimpact.CodeUsage, "usage code=%d output=%s", code, stdout)
	root := filepath.Clean(filepath.Join("..", ".."))
	runtimeSource, runtimeErr := os.ReadFile(filepath.Join(root, "cmd", "metasystem", "runtime_setup.go"))
	proofGo, goErr := os.ReadFile(filepath.Join(root, "internal", "proofrun", "test_go.go"))
	executorSource, executorErr := os.ReadFile(filepath.Join(root, "internal", "testimpact", "executor.go"))
	cliRequire(t, runtimeErr == nil && goErr == nil && executorErr == nil && bytes.Contains(runtimeSource, []byte("testpolicy.LoadAt(contractPath, result.Layout.GitRoot)")) && bytes.Contains(executorSource, []byte("proofrun.GroupArguments(ctx, group, root, cwd, environment, func(command *exec.Cmd) error")) && bytes.Contains(proofGo, []byte("command.Cancel, command.Stdout, command.Stderr = nil")) && bytes.Contains(executorSource, []byte("Setpgid: true")) && bytes.Contains(executorSource, []byte("syscall.Kill(-group, syscall.SIGTERM)")), "readiness or fallback argv discovery bypasses the required root and process lifecycle")
	code, stdout, _ = captureCommandOutput(t, true, true, func() int { return runTestImpacted([]string{"--list"}) })
	cliRequire(t, code == 0 && strings.Contains(stdout, "IMPACTED TESTS mode=list") && strings.Contains(stdout, "fallback=true reason=implementation-undeclared") && strings.Contains(stdout, "scope="), "summary: %s", stdout)
	binary := filepath.Join(t.TempDir(), "metasystem")
	command := exec.Command("go", "build", "-o", binary, "./cmd/metasystem")
	command.Dir = root
	output, err := command.CombinedOutput()
	cliRequire(t, err == nil, "build: %v: %s", err, output)
	for name, argv := range map[string][]string{
		"built":  {binary, "test", "impacted", "--list", "--json"},
		"go run": {"go", "run", "./cmd/metasystem", "test", "impacted", "--list", "--json"},
	} {
		command := exec.Command(argv[0], argv[1:]...)
		command.Dir = root
		var out, stderr bytes.Buffer
		command.Stdout, command.Stderr = &out, &stderr
		err := command.Run()
		cliRequire(t, err == nil, "%s: %v: %s", name, err, stderr.String())
		var envelope testimpact.Envelope
		cliRequire(t, json.Unmarshal(out.Bytes(), &envelope) == nil && envelope.Fallback && envelope.FallbackReason == "implementation-undeclared" && envelope.Result.Status == testimpact.StatusListed && strings.Contains(stderr.String(), "FULL TEST FALLBACK"), "%s envelope: %s stderr=%s", name, out.String(), stderr.String())
	}
	fixture := cliGitRepository(t)
	provider := filepath.Join(fixture, "provider.sh")
	err = testexec.WriteFile(provider, []byte("#!/bin/sh\nrequest=$(cat)\nbinding=$(printf '%s' \"$request\" | sed -n 's/.*\"binding\":\"\\([0-9a-f]*\\)\".*/\\1/p')\nprintf '{\"schemaVersion\":1,\"mode\":\"run\",\"binding\":\"%s\",\"status\":\"incomplete\",\"selections\":[{\"label\":\"blocked\",\"cwd\":\".\",\"argv\":[\"test\"],\"scope\":\"check\",\"tests\":[],\"reasons\":[{\"test\":\"*\",\"because\":\"offline dependency\"}],\"result\":\"incomplete\",\"seconds\":0}],\"uncertainty\":[]}\\n' \"$binding\"\n"), 0o755)
	cliRequire(t, err == nil, "%v", err)
	contract := `{"schemaVersion":1,"impacted":{"cwd":".","argv":["./provider.sh"]},"projectRisk":{"severity":1,"exposure":1,"reversibility":"revert","detection":"immediate","recovery":"bounded"},"surfaces":[{"id":"app","paths":["**"],"dependsOn":[],"standard":["unit"],"deep":[],"critical":[]}],"groups":[{"id":"unit","kind":"unit","adapter":"section","cwd":".","inputs":["seed"],"outputs":[],"tools":[],"obligations":[],"platforms":["any"],"targetMs":1000,"section":"unit"}],"always":{"canary":[],"standard":[]},"unknown":["unit"],"cadence":[]}`
	cliRequire(t, os.MkdirAll(filepath.Join(fixture, "metasystem"), 0o755) == nil, "mkdir contract")
	cliRequire(t, testexec.WriteFile(filepath.Join(fixture, "metasystem", "testing.json"), []byte(contract), 0o644) == nil, "write contract")
	exit, output := cliImpact(t, binary, fixture)
	cliRequire(t, exit == 1 && bytes.Contains(output, []byte(`"status":"incomplete"`)), "incomplete exit %d: %s", exit, output)
	exit, output = cliImpact(t, binary, fixture, "--base", "missing")
	cliRequire(t, exit == 2 && bytes.Contains(output, []byte(testimpact.CodeInputInvalid)), "input exit %d: %s", exit, output)
	cliRequire(t, testexec.WriteFile(filepath.Join(fixture, "metasystem", "testing.json"), []byte(`{}`), 0o644) == nil, "write invalid contract")
	exit, output = cliImpact(t, binary, fixture)
	cliRequire(t, exit == 2 && bytes.Contains(output, []byte(testimpact.CodeConfigInvalid)), "config exit %d: %s", exit, output)
	cliRequire(t, testexec.WriteFile(filepath.Join(fixture, "metasystem", "testing.json"), []byte(contract), 0o644) == nil, "restore contract")
	for _, failure := range []struct{ body, code string }{{"#!/bin/sh\nprintf '{}\\n'\n", testimpact.CodeResultInvalid}, {"#!/bin/sh\nexit 7\n", testimpact.CodeExecutionFailed}} {
		cliRequire(t, testexec.WriteFile(provider, []byte(failure.body), 0o755) == nil, "write failing provider")
		exit, output = cliImpact(t, binary, fixture)
		cliRequire(t, exit == 2 && bytes.Contains(output, []byte(failure.code)), "%s exit %d: %s", failure.code, exit, output)
	}
	for _, name := range []string{"changes.go", "executor.go", "protocol.go", "run.go"} {
		data, err := os.ReadFile(filepath.Join(root, "internal", "testimpact", name))
		cliRequire(t, err == nil, "%v", err)
		for _, forbidden := range []string{"internal/engine", "internal/state", "census", "process list", `"net"`} {
			cliRequire(t, !strings.Contains(string(data), forbidden), "%s depends on %q", name, forbidden)
		}
	}
}
func cliGitRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.name", "Fixture"}, {"config", "user.email", "fixture@example.invalid"}} {
		command := exec.Command("git", append([]string{"-C", root}, args...)...)
		output, err := command.CombinedOutput()
		cliRequire(t, err == nil, "git %s: %v: %s", args, err, output)
	}
	cliRequire(t, testexec.WriteFile(filepath.Join(root, "seed"), []byte("seed\n"), 0o644) == nil, "write seed")
	for _, args := range [][]string{{"add", "seed"}, {"commit", "-qm", "seed"}} {
		command := exec.Command("git", append([]string{"-C", root}, args...)...)
		output, err := command.CombinedOutput()
		cliRequire(t, err == nil, "git %s: %v: %s", args, err, output)
	}
	return root
}
func cliRequire(t *testing.T, condition bool, format string, arguments ...any) {
	if condition {
		return
	}
	t.Fatalf(format, arguments...)
}
func cliImpact(t *testing.T, binary, root string, arguments ...string) (int, []byte) {
	t.Helper()
	arguments = append([]string{"test", "impacted", "--json"}, arguments...)
	command := exec.Command(binary, arguments...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err == nil {
		return 0, output
	}
	return command.ProcessState.ExitCode(), output
}
