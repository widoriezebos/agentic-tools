package testimpact

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractmerge"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestImpactedFallback(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.invalid/fallback\n\ngo 1.22\n", 0o644)
	writeFile(t, filepath.Join(root, "fallback_test.go"), `package fallback
import ("os"; "testing")
func TestOffline(t *testing.T) { for key, want := range map[string]string{"GOPROXY":"off", "GOSUMDB":"off", "GOTOOLCHAIN":"local"} { if os.Getenv(key) != want { t.Fatalf("%s=%q", key, os.Getenv(key)) } } }
`, 0o644)
	marker := filepath.Join(root, "order")
	writeExecutable(t, filepath.Join(root, "sub", "command.sh"), "#!/bin/sh\ntest \"$(basename \"$(pwd -P)\")\" = sub && test \"$FALLBACK_MARKER\" = exact || exit 8\nprintf 'command\\n' >>'"+marker+"'\nexit 7\n")
	writeExecutable(t, filepath.Join(root, "scripts", "agents", "validate-section-selector.sh"), "#!/bin/sh\ntest -f '"+marker+"' || exit 9\nprintf 'section\\n' >>'"+marker+"'\n")
	groups := []testpolicy.Group{
		fallbackGroup("command", "command", json.RawMessage(nil), []string{"./command.sh"}),
		fallbackGroup("section", "section", json.RawMessage(nil), nil),
		fallbackGroup("go", "go", json.RawMessage(`"all"`), nil),
	}
	groups[0].CWD, groups[0].Env, groups[1].Section, groups[2].Packages = "sub", map[string]string{"FALLBACK_MARKER": "exact"}, "fixture", []string{"."}
	contract := testpolicy.Contract{Groups: groups}
	listed := RunFallback(context.Background(), root, ModeList, contract, Executor{}, &bytes.Buffer{})
	require(t, listed.Status == StatusListed && len(listed.Groups) == 3, "list: %#v", listed)
	for i, item := range listed.Groups {
		cwd := filepath.Join(root, filepath.FromSlash(item.Group.CWD))
		env := proofrun.TestingEnvironment(os.Environ(), item.Group.Env)
		if item.Group.Adapter == "go" {
			env = proofrun.TestingEnvironment(env, map[string]string{"GOPROXY": "off", "GOSUMDB": "off", "GOTOOLCHAIN": "local"})
		}
		want, err := proofrun.GroupArguments(context.Background(), item.Group, root, cwd, env, nil)
		require(t, err == nil && reflect.DeepEqual(item.Argv, want), "argv %d: got=%q want=%q err=%v", i, item.Argv, want, err)
	}
	run := RunFallback(context.Background(), root, ModeRun, contract, Executor{}, &bytes.Buffer{})
	order, err := os.ReadFile(marker)
	require(t, err == nil && run.Status == StatusFailed && run.Groups[0].Status == StatusFailed && run.Groups[2].Status == StatusPassed && string(order) == "command\nsection\n", "run: %#v order=%q", run, order)
	blocked := RunFallback(context.Background(), root, ModeRun, testpolicy.Contract{Groups: []testpolicy.Group{fallbackGroup("missing", "command", nil, []string{"./missing"})}}, Executor{}, &bytes.Buffer{})
	require(t, blocked.Status == StatusIncomplete && blocked.Groups[0].Status == StatusIncomplete, "unavailable prerequisite: %#v", blocked)
	missingGo := fallbackGroup("missing-go", "go", json.RawMessage(`"all"`), nil)
	missingGo.Packages = []string{"."}
	unavailable := RunFallback(context.Background(), t.TempDir(), ModeRun, testpolicy.Contract{Groups: []testpolicy.Group{missingGo}}, Executor{}, &bytes.Buffer{})
	request := protocolRequest(t.TempDir(), ModeRun)
	require(t, unavailable.Status == StatusIncomplete && len(unavailable.Groups[0].Argv) > 0 && ValidateResult(request, fallbackResult(request, unavailable)) == nil, "discovery failure: %#v", unavailable)
	unavailable = RunFallback(context.Background(), t.TempDir(), ModeList, testpolicy.Contract{Groups: []testpolicy.Group{missingGo}}, Executor{}, &bytes.Buffer{})
	require(t, unavailable.Status == StatusListed && unavailable.Groups[0].Status == StatusNotRun, "list discovery failure: %#v", unavailable)
	repo := gitRepository(t)
	var fallbackProgress bytes.Buffer
	envelope, code := Run(context.Background(), Options{Start: repo, Mode: ModeList, Stderr: &fallbackProgress})
	require(t, code == 0 && envelope.Fallback && envelope.FallbackReason == "no-tests-declared" && envelope.Result.Status == StatusNotRun && strings.Contains(fallbackProgress.String(), "FULL TEST FALLBACK: NO TESTS DECLARED"), "missing contract fallback: code=%d %#v", code, envelope)
	contract = validContract()
	contract.Impacted = nil
	writeContract(t, repo, contract)
	envelope, code = Run(context.Background(), Options{Start: repo, Mode: ModeList, Stderr: &bytes.Buffer{}})
	require(t, code == 0 && envelope.FallbackReason == "implementation-undeclared" && len(envelope.Result.Selections) == 1, "undeclared: code=%d %#v", code, envelope)
	contract.TailoringRequired = true
	writeContract(t, repo, contract)
	envelope, code = Run(context.Background(), Options{Start: repo, Mode: ModeList, Stderr: &bytes.Buffer{}})
	require(t, code == 0 && envelope.FallbackReason == "tailoring-required", "template: code=%d %#v", code, envelope)
	contract.Groups[0].CWD = "../escape"
	writeContract(t, repo, contract)
	_, code = Run(context.Background(), Options{Start: repo, Mode: ModeList, Stderr: &bytes.Buffer{}})
	require(t, code == 2, "template accepted an invalid executable group: %d", code)
}
func TestImpactedChanges(t *testing.T) {
	t.Parallel()
	root := gitRepository(t)
	writeFile(t, filepath.Join(root, ".gitignore"), "ignored\n", 0o644)
	writeFile(t, filepath.Join(root, "old.txt"), "old\n", 0o644)
	writeFile(t, filepath.Join(root, "staged.txt"), "base\n", 0o644)
	gitRun(t, root, "add", ".")
	gitRun(t, root, "commit", "-qm", "paths")
	base := strings.TrimSpace(gitRun(t, root, "rev-parse", "HEAD"))
	writeFile(t, filepath.Join(root, "staged.txt"), "staged\n", 0o644)
	gitRun(t, root, "add", "staged.txt")
	require(t, os.Rename(filepath.Join(root, "old.txt"), filepath.Join(root, "new name.txt")) == nil, "rename failed")
	writeFile(t, filepath.Join(root, "untracked space.txt"), "one\n", 0o644)
	writeFile(t, filepath.Join(root, "ignored"), "ignored\n", 0o644)
	request, err := BuildRequest(context.Background(), root, "HEAD", ModeList)
	require(t, err == nil, "%v", err)
	want := []string{"new name.txt", "old.txt", "staged.txt", "untracked space.txt"}
	require(t, request.Base == base && reflect.DeepEqual(request.ChangedPaths, want), "base/paths: %s %#v", request.Base, request.ChangedPaths)
	first := request.Binding
	otherBase, _ := bindChanges(root, strings.Repeat("b", 40), request.ChangedPaths)
	require(t, first != otherBase, "base was not bound")
	var left, right bytes.Buffer
	writeBound(left.Write, []byte("ab"))
	writeBound(left.Write, []byte("c"))
	writeBound(right.Write, []byte("a"))
	writeBound(right.Write, []byte("bc"))
	require(t, !bytes.Equal(left.Bytes(), right.Bytes()), "binding fields were not length-prefixed")
	writeFile(t, filepath.Join(root, "untracked space.txt"), "two\n", 0o644)
	request, err = BuildRequest(context.Background(), root, "HEAD", ModeList)
	require(t, err == nil && request.Binding != first, "content was not bound: %s %s %v", first, request.Binding, err)
	contentBinding := request.Binding
	require(t, os.Chmod(filepath.Join(root, "untracked space.txt"), 0o755) == nil, "chmod failed")
	request, _ = BuildRequest(context.Background(), root, "HEAD", ModeList)
	require(t, request.Binding != contentBinding, "executable mode was not bound")
	require(t, os.Symlink("one", filepath.Join(root, "link")) == nil, "symlink failed")
	request, _ = BuildRequest(context.Background(), root, "HEAD", ModeList)
	linkBinding := request.Binding
	require(t, os.Remove(filepath.Join(root, "link")) == nil, "remove symlink failed")
	require(t, os.Symlink("two", filepath.Join(root, "link")) == nil, "replace symlink failed")
	request, _ = BuildRequest(context.Background(), root, "HEAD", ModeList)
	require(t, request.Binding != linkBinding, "symlink target was not bound")
	_, err = changedPaths([]byte("../escape\x00"))
	require(t, err != nil, "escaping path accepted")
	_, err = changedPaths([]byte{'x', 0xff, 0})
	require(t, err != nil, "non-UTF-8 path accepted")
	conflictFile := filepath.Join(root, "conflict.txt")
	writeFile(t, conflictFile, "one", 0o644)
	one := strings.TrimSpace(gitRun(t, root, "hash-object", "-w", conflictFile))
	writeFile(t, conflictFile, "two", 0o644)
	two := strings.TrimSpace(gitRun(t, root, "hash-object", "-w", conflictFile))
	index := "100644 " + one + " 1\tconflict.txt\n100644 " + one + " 2\tconflict.txt\n100644 " + two + " 3\tconflict.txt\n"
	command := exec.Command("git", "-C", root, "update-index", "--index-info")
	command.Stdin = strings.NewReader(index)
	output, err := command.CombinedOutput()
	require(t, err == nil, "make conflict: %v: %s", err, output)
	_, err = BuildRequest(context.Background(), root, "HEAD", ModeList)
	require(t, err != nil, "unresolved conflict accepted")
}
func TestImpactedConfig(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	contract := validContract()
	contract.Impacted = &testpolicy.Impacted{CWD: ".", Argv: []string{"./provider", "--flag"}}
	data, err := json.Marshal(contract)
	require(t, err == nil, "%v", err)
	_, err = testpolicy.Decode(data)
	require(t, err == nil, "%v", err)
	rendered, err := contractmerge.Render(contract)
	require(t, err == nil, "%v", err)
	roundTrip, err := testpolicy.Decode(rendered)
	require(t, err == nil && reflect.DeepEqual(roundTrip.Impacted, contract.Impacted), "round trip: %#v %v", roundTrip.Impacted, err)
	for name, replacement := range map[string]string{
		"null": `"impacted":null`, "unknown": `"impacted":{"cwd":".","argv":["x"],"extra":1}`,
		"missing cwd": `"impacted":{"argv":["x"]}`, "empty argv": `"impacted":{"cwd":".","argv":[]}`,
		"blanket success": `"impacted":{"cwd":".","argv":["true"]}`, "escaping cwd": `"impacted":{"cwd":"../x","argv":["x"]}`, "NUL argv": `"impacted":{"cwd":".","argv":["x","\u0000"]}`,
	} {
		mutated := replaceImpacted(data, replacement)
		_, err := testpolicy.Decode(mutated)
		require(t, err != nil, "%s accepted", name)
	}
	base, ours, theirs := contract, contract, contract
	ours.Impacted = &testpolicy.Impacted{CWD: ".", Argv: []string{"provider", "ours"}}
	merged, err := contractmerge.Merge(base, ours, theirs)
	require(t, err == nil && reflect.DeepEqual(merged.Impacted, ours.Impacted), "independent merge: %#v %v", merged.Impacted, err)
	theirs.Impacted = &testpolicy.Impacted{CWD: "provider", Argv: append([]string(nil), base.Impacted.Argv...)}
	merged, err = contractmerge.Merge(base, ours, theirs)
	require(t, err == nil && merged.Impacted.CWD == "provider" && reflect.DeepEqual(merged.Impacted.Argv, ours.Impacted.Argv), "field merge: %#v %v", merged.Impacted, err)
	theirs.Impacted = &testpolicy.Impacted{CWD: ".", Argv: []string{"provider", "theirs"}}
	_, err = contractmerge.Merge(base, ours, theirs)
	require(t, err != nil, "conflicting ordered argv edits merged")
	protected := testpolicy.ProtectedContract(contract, ours)
	require(t, reflect.DeepEqual(protected.Impacted, contract.Impacted), "protected contract replaced the provider")
	writeContract(t, root, contract)
	_, _, err = loadContract(root)
	require(t, err == nil, "%v", err)
	contract.Impacted.CWD, contract.TailoringRequired = "missing", true
	writeContract(t, root, contract)
	_, err = testpolicy.LoadAt(filepath.Join(root, "metasystem", "testing.json"), root)
	require(t, err != nil && strings.Contains(err.Error(), "existing directory inside"), "missing cwd accepted: %v", err)
	outside := t.TempDir()
	require(t, os.Symlink(outside, filepath.Join(root, "escape")) == nil, "escape symlink failed")
	contract.Impacted.CWD = "escape"
	writeContract(t, root, contract)
	_, err = testpolicy.LoadAt(filepath.Join(root, "metasystem", "testing.json"), root)
	require(t, err != nil && strings.Contains(err.Error(), "existing directory inside"), "symlink cwd escape accepted: %v", err)
}
func fallbackGroup(id, adapter string, tests json.RawMessage, argv []string) testpolicy.Group {
	return testpolicy.Group{ID: id, Kind: "static", Adapter: adapter, CWD: ".", Inputs: []string{"go.mod"}, Outputs: []string{}, Tools: []testpolicy.Tool{}, Obligations: []string{}, Platforms: []string{"any"}, TargetMS: 1000, Tests: tests, Argv: argv, Format: "exit-status"}
}
func validContract() testpolicy.Contract {
	group := fallbackGroup("unit", "section", nil, nil)
	group.Kind, group.Section = "unit", "unit"
	return testpolicy.Contract{SchemaVersion: 1, ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces: []testpolicy.Surface{{ID: "app", Paths: []string{"**"}, Standard: []string{"unit"}, Deep: []string{}, Critical: []string{}}}, Groups: []testpolicy.Group{group},
		Always: testpolicy.Always{Canary: []string{}, Standard: []string{}}, Unknown: []string{"unit"}, Cadence: []string{}}
}
func gitRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	writeFile(t, filepath.Join(root, "seed"), "seed\n", 0o644)
	gitRun(t, root, "add", "seed")
	gitRun(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "seed")
	return root
}
func gitRun(t *testing.T, root string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, arguments...)...)
	output, err := command.CombinedOutput()
	require(t, err == nil, "git %s: %v: %s", strings.Join(arguments, " "), err, output)
	return string(output)
}
func writeFile(t *testing.T, path, body string, mode os.FileMode) {
	require(t, os.MkdirAll(filepath.Dir(path), 0o755) == nil, "mkdir %s", path)
	require(t, testexec.WriteFile(path, []byte(body), mode) == nil, "write %s", path)
}
func writeContract(t *testing.T, root string, contract testpolicy.Contract) {
	data, err := json.Marshal(contract)
	require(t, err == nil, "%v", err)
	writeFile(t, filepath.Join(root, "metasystem", "testing.json"), string(data), 0o644)
}
func replaceImpacted(data []byte, replacement string) []byte {
	start := bytes.Index(data, []byte(`"impacted":`))
	end := start + bytes.Index(data[start:], []byte(`,"projectRisk"`))
	return append(append(append([]byte{}, data[:start]...), replacement...), data[end:]...)
}
