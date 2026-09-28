package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// These tests port the host-adapter and fixture-control scenarios of
// scripts/agents/mission-fixtures.sh. The subjects are the host turns of the
// engine's delegate supervisor (the host scripts retired with the adapters'
// port to Go, batch 2); each runs against a private installation root with
// this test binary as its engine, and only the paid claude CLI is replaced.

// missionBedHostRoot copies the orchestrator schema and the given
// configuration into a private installation root.
func missionBedHostRoot(t *testing.T, conf string) string {
	t.Helper()
	root := t.TempDir()
	for _, relative := range []string{
		filepath.Join("scripts", "agents", "schemas", "orchestrator.schema.json"),
	} {
		data, err := os.ReadFile(filepath.Join("..", "..", relative))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, relative)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := testexec.WriteFile(filepath.Join(root, relative), data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

// missionBedHostTurn lays out one host turn: its record and its prompt.
func missionBedHostTurn(t *testing.T, root, mission, turnID string) (turn, prompt string) {
	t.Helper()
	turn = filepath.Join(root, "turns", turnID)
	if err := os.MkdirAll(turn, 0o700); err != nil {
		t.Fatal(err)
	}
	record := `{"missionId":"` + mission + `","turnId":"` + turnID + `","cycle":1,"model":"fixture-model","hostSession":"announced-session"}` + "\n"
	if err := os.WriteFile(filepath.Join(turn, "turn.json"), []byte(record), 0o600); err != nil {
		t.Fatal(err)
	}
	prompt = filepath.Join(turn, "prompt.md")
	if err := os.WriteFile(prompt, []byte("host adapter session fixture prompt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return turn, prompt
}

// missionBedFakeClaude installs the replacement for the paid CLI: it answers
// with the session FAKE_CLAUDE_SESSION names, or with none.
func missionBedFakeClaude(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	script := `#!/usr/bin/env bash
set -euo pipefail
cat >/dev/null
if [[ ${FAKE_CLAUDE_SESSION:-} == none ]]; then
  printf '{"result":"{}","usage":{"input_tokens":1,"output_tokens":1}}\n'
else
  printf '{"session_id":"%s","result":"{}","usage":{"input_tokens":1,"output_tokens":1}}\n' "${FAKE_CLAUDE_SESSION:?}"
fi
`
	if err := testexec.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

// runMissionBedHost runs one host verb of the named runtime through the
// engine's delegate supervisor against the installation root.
func runMissionBedHost(t *testing.T, root, runtime string, environment []string, args ...string) (int, string) {
	t.Helper()
	command := (proofBinaryFixture{t: t}).command(
		fixtureCommandEnvironment(t, append([]string{"METASYSTEM_BIN=" + commandTestExecutable(t)}, environment...)...),
		commandTestExecutable(t), append([]string{"delegate-supervisor", runtime, args[0], "--root", root}, args[1:]...)...)
	output, err := command.CombinedOutput()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), string(output)
	}
	if err != nil {
		t.Fatalf("run %s host %s: %v\n%s", runtime, args[0], err, output)
	}
	return 0, string(output)
}

func readMissionBedResult(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("result envelope %s: %v\n%s", path, err, data)
	}
	return result
}

// host-session: the claude host adapter is a witness, not a judge. A rotated
// session is reported in the result envelope with outcome completed; only a
// missing session keeps the adapter's exit-6 fault, reported unresumable
// with a null session.
func TestMissionBedClaudeHostReportsRotatedAndMissingSessions(t *testing.T) {
	t.Parallel()
	conf, err := os.ReadFile(filepath.Join("..", "..", "metasystem.conf"))
	if err != nil {
		t.Fatal(err)
	}
	root := missionBedHostRoot(t, string(conf))
	turn, prompt := missionBedHostTurn(t, root, "host-session", "host-session-t1-aaaa")
	path := "PATH=" + missionBedFakeClaude(t) + string(os.PathListSeparator) + os.Getenv("PATH")
	start := func(session, result string) (int, string) {
		return runMissionBedHost(t, root, "claude", []string{path, "FAKE_CLAUDE_SESSION=" + session},
			"start-turn", "--mission", "host-session", "--turn-id", "host-session-t1-aaaa",
			"--prompt", prompt, "--result", result, "--instance-tag", "fixture-host-session-tag",
			"--resume-session", "announced-session")
	}

	rotated := filepath.Join(turn, "result.json")
	if code, output := start("rotated-session", rotated); code != 0 {
		t.Fatalf("a rotated session failed the turn: exit %d\n%s", code, output)
	}
	if result := readMissionBedResult(t, rotated); result["outcome"] != "completed" || result["sessionId"] != "rotated-session" {
		t.Fatalf("a rotated session was not reported as completed under its new id: %v", result)
	}

	missing := filepath.Join(turn, "result-missing.json")
	if code, output := start("none", missing); code != 6 {
		t.Fatalf("a missing host session did not keep exit 6: exit %d\n%s", code, output)
	}
	if result := readMissionBedResult(t, missing); result["outcome"] != "unresumable" || result["sessionId"] != nil {
		t.Fatalf("a missing session was not reported unresumable with a null session: %v", result)
	}
}

// host-start-gate: a start gate that is never released fails the launch
// with exit 3, names the refusal, and writes no result envelope. The typed
// gate owner receives an authenticated fixture expiry event, so no wall
// time passes.
func TestMissionBedClaudeHostRefusesAnUnreleasedStartGate(t *testing.T) {
	t.Parallel()
	root := missionBedHostRoot(t, "metasystem.runtimes=fake\n")
	turn, prompt := missionBedHostTurn(t, root, "host-session", "host-session-t1-aaaa")
	expired := filepath.Join(t.TempDir(), "gate-expired")
	if err := os.WriteFile(expired, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	result := filepath.Join(turn, "result-gate.json")
	code, output := runMissionBedHost(t, root, "claude",
		[]string{
			"PATH=" + missionBedFakeClaude(t) + string(os.PathListSeparator) + os.Getenv("PATH"),
			"FAKE_CLAUDE_SESSION=rotated-session",
			"METASYSTEM_HOST_START_GATE=" + filepath.Join(t.TempDir(), "never-released"),
			"METASYSTEM_HOST_START_GATE_TIMEOUT_SEC=1",
			"METASYSTEM_START_GATE_EXPIRY_EVENT=" + expired,
			identity.FixtureOwnerEnv + "=" + syntheticFixtureOwnerKey(t, "mission-bed-start-gate"),
		},
		"start-turn", "--mission", "host-session", "--turn-id", "host-session-t1-aaaa",
		"--prompt", prompt, "--result", result, "--instance-tag", "fixture-host-session-tag")
	if code != 3 || !strings.Contains(output, "start gate was not released") {
		t.Fatalf("an unreleased start gate did not fail the launch by name: exit %d\n%s", code, output)
	}
	if _, err := os.Stat(result); !os.IsNotExist(err) {
		t.Fatalf("a launch that never passed the gate wrote a result envelope: %v", err)
	}
}

// fake-host controls: both fake-host hold controls are fixture-only. Outside
// a fake-runtime root the fake host refuses each of them by name.
func TestMissionBedFakeHostHoldControlsAreFixtureOnly(t *testing.T) {
	t.Parallel()
	root := missionBedHostRoot(t, "metasystem.runtimes=claude\n")
	turn, prompt := missionBedHostTurn(t, root, "outside", "outside-t1")
	for _, control := range []string{"METASYSTEM_FAKE_HOST_HOLD", "METASYSTEM_FAKE_HOST_IGNORE_TERM"} {
		t.Run(control, func(t *testing.T) {
			code, output := runMissionBedHost(t, root, "fake",
				[]string{control + "=1"},
				"start-turn", "--mission", "outside", "--turn-id", "outside-t1",
				"--prompt", prompt, "--result", filepath.Join(turn, "result.json"), "--instance-tag", "outside-fixture-control")
			if code == 0 || !strings.Contains(output, "available only in a fixture-mode root") {
				t.Fatalf("%s was not refused outside fixture mode: exit %d\n%s", control, code, output)
			}
		})
	}
}
