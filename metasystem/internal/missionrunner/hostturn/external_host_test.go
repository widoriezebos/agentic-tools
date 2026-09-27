package hostturn

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// The mission host role of an external runtime (design verbs-object-action
// 3.5, VOA-25): an adapter executable installed after this engine was built
// serves a host turn and its resume through the same operations, with no Go
// registration and no host script. The executable is a wrapper around this
// test binary; TestMain enters runExternalHostFixture before the test
// environment starts.

type externalHostRequest struct {
	Operation string `json:"operation"`
	Usage     string `json:"usage"`
	Turn      struct {
		Role, Dir, Prompt, ResumeSession, Tag string
	} `json:"turn"`
	Private struct {
		Stdout string `json:"stdout"`
	} `json:"private"`
}

func runExternalHostFixture() int {
	operation := os.Args[len(os.Args)-1]
	var request externalHostRequest
	input, _ := io.ReadAll(os.Stdin)
	if err := json.Unmarshal(input, &request); err != nil || request.Operation != operation {
		fmt.Fprintf(os.Stderr, "bad request for %q: %v\n", operation, err)
		return 90
	}
	answer := func(value map[string]any) int {
		value["schemaVersion"] = 1
		data, _ := json.Marshal(value)
		fmt.Println(string(data))
		return 0
	}
	turn := request.Turn
	switch operation {
	case "describe":
		return answer(map[string]any{"name": "newhost",
			"capabilities": map[string]any{"resume": true, "followUp": true, "host": true, "usage": "native"},
			"match":        []string{`^([^[:space:]]*/)?newhost([[:space:]]|$)`},
			"positive":     "newhost --task x", "lookalike": "newhost-helper",
			"enforcement": map[string]string{"writeRoots": "mapped", "readRoots": "mapped", "network": "mapped"}})
	case "prepare":
		if turn.Role != "host" {
			fmt.Fprintln(os.Stderr, "the host fixture serves host turns only")
			return 1
		}
		session := "newhost-session-1"
		if turn.ResumeSession != "" {
			session = turn.ResumeSession
		}
		stdout := filepath.Join(turn.Dir, "newhost.out")
		return answer(map[string]any{
			"argv":  []string{"/bin/sh", "-c", `cat >/dev/null; printf '%s\n' "$1"`, "newhost", session, "--tag", turn.Tag},
			"argv0": "newhost", "stdin": turn.Prompt, "stdout": stdout, "private": map[string]any{"stdout": stdout}})
	case "finalize":
		data, _ := os.ReadFile(request.Private.Stdout)
		session := strings.TrimSpace(string(data))
		raw, ret, usage := filepath.Join(turn.Dir, "raw.out"), filepath.Join(turn.Dir, "return.json"), filepath.Join(turn.Dir, "usage.json")
		for path, body := range map[string]string{raw: `{"reply":"ok"}`, ret: `{"reply":"ok"}`,
			usage: `{"availability":"native","inputTokens":1,"cachedInputTokens":0,"outputTokens":1,"reasoningTokens":null,"cost":null,"providerUnits":null}`} {
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
		}
		return answer(map[string]any{"hostSession": session, "hostRaw": raw, "hostReturn": ret, "usage": usage})
	}
	return 64
}

// TestExternalRuntimeServesHostTurnAndResume: a previously unknown runtime
// completes a mission's host turn, then resumes its session in the next.
func TestExternalRuntimeServesHostTurnAndResume(t *testing.T) {
	t.Parallel()
	b := newHostBed(t, "unused-cli", "exit 1\n")
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	adapterPath := filepath.Join(b.root, "adapters", "newhost")
	if err := os.MkdirAll(filepath.Dir(adapterPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(adapterPath, []byte(fmt.Sprintf("#!/bin/sh\nEXTERNAL_HOST_FIXTURE=1 exec '%s' \"$@\"\n", self)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.root, "metasystem.conf"), []byte("adapters.newhost.use=external\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := b.run("newhost"); code != 0 {
		t.Fatalf("external host turn = %d: %s", code, b.stderr.String())
	}
	if result := b.result(); result["outcome"] != "completed" || result["sessionId"] != "newhost-session-1" {
		t.Fatalf("host result = %v", result)
	}
	if err := os.Remove(filepath.Join(b.turn, "result.json")); err != nil {
		t.Fatal(err)
	}
	if code := b.run("newhost", "--resume-session", "newhost-session-1"); code != 0 {
		t.Fatalf("external host resume = %d: %s", code, b.stderr.String())
	}
	if result := b.result(); result["outcome"] != "completed" || result["sessionId"] != "newhost-session-1" {
		t.Fatalf("resumed host result = %v", result)
	}
}
