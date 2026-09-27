package supervisor

// The external adapter contract's conformance cases (design 3.5): the fake
// runtime driven as a built-in and as an external executable (a Go test
// helper that speaks the contract), a partial override that changes one
// operation and delegates the rest with exit 64, and the refusal of an
// unconfigured executable with a built-in's name.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes/external"
)

// TestExternalAdapterHelperProcess is the adapter executable: argv after
// "--" is the operation, the request is the first stdin line.
// EXT_ADAPTER_MODE full answers every operation the fake needs; partial
// answers only config-identity and delegates the rest.
func TestExternalAdapterHelperProcess(t *testing.T) {
	mode := os.Getenv("EXT_ADAPTER_MODE")
	if mode == "" {
		t.Skip("subprocess helper")
	}
	operation := ""
	for index, arg := range os.Args {
		if arg == "--" && index+1 < len(os.Args) {
			operation = os.Args[index+1]
		}
	}
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadBytes('\n')
	var request map[string]any
	_ = json.Unmarshal(line, &request)
	reply := func(value map[string]any) {
		value["schemaVersion"] = external.SchemaVersion
		encoded, _ := json.Marshal(value)
		os.Stdout.Write(append(encoded, '\n'))
		os.Exit(0)
	}
	if mode == "partial" {
		if operation == "config-identity" {
			reply(map[string]any{"cliVersion": "override-9", "configHash": "override-hash", "configKeyHashes": map[string]any{}})
		}
		os.Exit(external.DelegateExit)
	}
	switch operation {
	case "signature":
		text, _ := runtimes.SignatureText("fake")
		var match, exclude []string
		for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
			verb, pattern, _ := strings.Cut(line, " ")
			if verb == "match" {
				match = append(match, pattern)
			} else {
				exclude = append(exclude, pattern)
			}
		}
		reply(map[string]any{"match": match, "exclude": exclude})
	case "config-identity":
		reply(map[string]any{"cliVersion": "fake-1", "configHash": "fake-config-v1", "configKeyHashes": map[string]any{}})
	case "local-config-paths":
		reply(map[string]any{"paths": []string{}})
	case "command":
		if request["declare"] == true {
			reply(map[string]any{})
		}
		roundDir, _ := request["roundDir"].(string)
		record, _ := request["record"].(string)
		prompt, _ := request["prompt"].(string)
		returnPath := filepath.Join(roundDir, "external-return.json")
		if err := adapter.WriteFakeReturn(record, prompt, returnPath); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		// One JSON value per output line: the CLI stand-in prints the
		// return compacted.
		data, _ := os.ReadFile(returnPath)
		var compact bytes.Buffer
		_ = json.Compact(&compact, data)
		_ = os.WriteFile(returnPath, compact.Bytes(), 0o644)
		session := "fake-session-" + filepath.Base(filepath.Dir(filepath.Dir(roundDir)))
		script := fmt.Sprintf("cat >/dev/null; printf '%%s\\n' '{\"event\":\"session\",\"id\":\"%s\"}'; cat '%s'; echo", session, returnPath)
		reply(map[string]any{"argv": []string{"/bin/sh", "-c", script}, "stdin": "prompt"})
	case "output-stream":
		output, _ := io.ReadAll(reader)
		scanner := bufio.NewScanner(bytes.NewReader(output))
		scanner.Buffer(make([]byte, 1<<20), 1<<24)
		for scanner.Scan() {
			var event map[string]any
			if json.Unmarshal(scanner.Bytes(), &event) != nil {
				continue
			}
			if event["event"] == "session" {
				encoded, _ := json.Marshal(map[string]any{"type": "session", "session": event["id"], "turn": "fake-turn-1", "model": "fake-model"})
				os.Stdout.Write(append(encoded, '\n'))
				continue
			}
			candidate := scanner.Text()
			usagePath := filepath.Join(os.TempDir(), fmt.Sprintf("ext-usage-%d.json", os.Getpid()))
			_ = adapter.WriteFakeUsage(usagePath)
			usage, _ := os.ReadFile(usagePath)
			os.Remove(usagePath)
			encoded, _ := json.Marshal(map[string]any{"type": "usage", "usage": json.RawMessage(bytes.TrimSpace(usage))})
			os.Stdout.Write(append(encoded, '\n'))
			encoded, _ = json.Marshal(map[string]any{"type": "result", "candidate": candidate})
			os.Stdout.Write(append(encoded, '\n'))
		}
		os.Exit(0)
	}
	os.Exit(external.DelegateExit)
}

// installExternalAdapter writes <root>/adapters/<name>, a wrapper that runs
// this test binary as the adapter in the given mode, and optionally the
// override setting.
func installExternalAdapter(t *testing.T, root, name, mode string, override bool) {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, external.Dir, name)
	mustWrite(t, path, fmt.Sprintf("#!/bin/sh\nEXT_ADAPTER_MODE=%s exec '%s' -test.run='^TestExternalAdapterHelperProcess$' -test.count=1 -- \"$@\"\n", mode, binary))
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(root, "metasystem.conf")
	existing, _ := os.ReadFile(conf)
	if override {
		existing = append(existing, []byte(external.OverrideKey(name)+"="+external.OverrideValue+"\n")...)
	}
	mustWrite(t, conf, string(existing))
}

func returnDigest(t *testing.T, path string) map[string]any {
	t.Helper()
	ret := readJSON(t, path)
	keep := map[string]any{}
	for _, key := range []string{"jobId", "round", "runtime", "sessionId", "mode", "schemaVersion", "whatWasDone", "evidence", "gaps"} {
		keep[key] = ret[key]
	}
	return keep
}

// TestExternalFakeConformsToTheBuiltin drives one fake round through the
// built-in and one through the external executable (overriding the built-in
// in full), and requires the same outcome: a completed record through one
// running→completed compare-and-swap, the same handshake session, turn and
// model, and the same return identity and content. The external round also
// registers custody and pre-fork evidence, as every non-fake runtime does.
func TestExternalFakeConformsToTheBuiltin(t *testing.T) {
	builtinRound := newFakeInstall(t, installOptions{})
	if code := builtinRound.run(); code != 0 {
		t.Fatalf("built-in fake exit %d: %s", code, builtinRound.stderr.String())
	}
	externalRound := newFakeInstall(t, installOptions{})
	installExternalAdapter(t, externalRound.root, "fake", "full", true)
	if code := externalRound.run(); code != 0 {
		t.Fatalf("external fake exit %d: %s\nlog: %s", code, externalRound.stderr.String(), externalRound.logText())
	}
	handshake := func(calls [][]string) []string {
		for _, call := range calls {
			if call[0] == "__handshake" {
				return []string{flagValue(call, "--session"), flagValue(call, "--turn"), flagValue(call, "--model")}
			}
		}
		return nil
	}
	terminal := func(calls [][]string) string {
		last := ""
		for _, call := range calls {
			if call[0] == "__record-cas" {
				last = flagValue(call, "--expect") + ">" + flagValue(call, "--status")
			}
		}
		return last
	}
	builtinCalls, externalCalls := builtinRound.dispatcher.calls(t), externalRound.dispatcher.calls(t)
	if fmt.Sprint(handshake(builtinCalls)) != fmt.Sprint(handshake(externalCalls)) {
		t.Fatalf("handshake built-in %v, external %v", handshake(builtinCalls), handshake(externalCalls))
	}
	if terminal(builtinCalls) != "running>completed" || terminal(externalCalls) != "running>completed" {
		t.Fatalf("terminal built-in %q, external %q (calls %v)", terminal(builtinCalls), terminal(externalCalls), externalCalls)
	}
	if !strings.Contains(fmt.Sprint(callNames(externalCalls)), "__register-custody") {
		t.Fatalf("the external round registered no custody: %v", callNames(externalCalls))
	}
	b, e := returnDigest(t, builtinRound.roundFile("return.json")), returnDigest(t, externalRound.roundFile("return.json"))
	if fmt.Sprint(b) != fmt.Sprint(e) {
		t.Fatalf("return built-in %v\nexternal %v", b, e)
	}
	if usage := readJSON(t, externalRound.roundFile("usage.json")); usage["inputTokens"] != float64(11) {
		t.Fatalf("external usage = %v", usage)
	}
}

// TestPartialOverrideDelegatesWithExit64: an override that answers only
// config-identity changes that operation; every other operation, and the
// round itself, is the built-in's.
func TestPartialOverrideDelegatesWithExit64(t *testing.T) {
	f := newFakeInstall(t, installOptions{})
	installExternalAdapter(t, f.root, "fake", "partial", true)
	if code := f.run("fake", "identity", "--root", f.root); code != 0 || f.stdout.String() != "override-9 override-hash\n" {
		t.Fatalf("overridden identity = %d %q %s", code, f.stdout.String(), f.stderr.String())
	}
	f.stdout.buf.Reset()
	text, _ := runtimes.SignatureText("fake")
	if code := f.run("fake", "signature", "--root", f.root); code != 0 || f.stdout.String() != text {
		t.Fatalf("delegated signature = %d %q", code, f.stdout.String())
	}
	f.stdout.buf.Reset()
	if code := f.run("fake", "output-stream", "--root", f.root, "--round-dir", "/r"); code != 0 || f.stdout.String() != "/r/events.jsonl\n" {
		t.Fatalf("delegated output-stream = %d %q", code, f.stdout.String())
	}
	if code := f.run(); code != 0 {
		t.Fatalf("delegated round exit %d: %s", code, f.stderr.String())
	}
	if got := f.logText(); got != "fake supervisor started value=fake-tag-1\n" {
		t.Fatalf("the delegated round is not the built-in's: log %q", got)
	}
}

// TestUnconfiguredBuiltinNameIsRefused: an executable with a built-in's name
// and no override setting is refused, naming the setting, for the entry and
// for the recognizers' discovery.
func TestUnconfiguredBuiltinNameIsRefused(t *testing.T) {
	f := newFakeInstall(t, installOptions{})
	installExternalAdapter(t, f.root, "fake", "full", false)
	if code := f.run("fake", "identity", "--root", f.root); code != 2 || !strings.Contains(f.stderr.String(), "adapters.fake.override=external") {
		t.Fatalf("unconfigured override = %d %q", code, f.stderr.String())
	}
	_, refusals, err := external.Discover(f.root)
	if err != nil || len(refusals) != 1 || !strings.Contains(refusals[0].Reason, "adapters.fake.override=external") {
		t.Fatalf("discovery refusals = %v %v", refusals, err)
	}
}

// TestExternalRuntimeIsDiscoveredAndRecognized: a new runtime name is a
// registry member for the entry and for the census signatures.
func TestExternalRuntimeIsDiscoveredAndRecognized(t *testing.T) {
	f := newFakeInstall(t, installOptions{})
	installExternalAdapter(t, f.root, "newagent", "full", false)
	if code := f.run("newagent", "config-identity", "--root", f.root); code != 0 ||
		f.stdout.String() != `{"cliVersion":"fake-1","configHash":"fake-config-v1","configKeyHashes":{},"runtime":"newagent"}`+"\n" {
		t.Fatalf("external config-identity = %d %q %s", code, f.stdout.String(), f.stderr.String())
	}
	f.stdout.buf.Reset()
	if code := f.run("newagent", "output-stream", "--root", f.root, "--round-dir", "/r"); code != 0 || f.stdout.String() != "/r/"+ExternalOutputFile+"\n" {
		t.Fatalf("external output-stream = %d %q", code, f.stdout.String())
	}
	adapters, _, err := external.Discover(f.root)
	if err != nil || len(adapters) != 1 || adapters[0].Name != "newagent" || adapters[0].Overrides {
		t.Fatalf("discovered %v %v", adapters, err)
	}
}
