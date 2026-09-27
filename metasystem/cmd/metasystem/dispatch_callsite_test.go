package main

import (
	"os"
	"strings"
	"testing"
)

func TestAllAdapterLaunchRoutesConsumeTheOneShotCapability(t *testing.T) {
	common, err := os.ReadFile("../../scripts/agents/adapters/runtime-common.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"--launch-capability", "job launch-capability-consume", `--adapter-verb "$adapter_verb"`, `--supervisor-pid "$$"`} {
		if !strings.Contains(string(common), token) {
			t.Fatalf("real-adapter common path lacks %q", token)
		}
	}
	for _, runtime := range []string{"codex", "claude", "devin"} {
		data, readErr := os.ReadFile("../../scripts/agents/adapters/" + runtime + ".sh")
		if readErr != nil {
			t.Fatal(readErr)
		}
		text := string(data)
		if !strings.Contains(text, `runtime-common.sh"`) || !strings.Contains(text, `dispatch|follow-up) supervise "$command_name" "$@"`) {
			t.Fatalf("%s does not route both launch verbs through common supervision", runtime)
		}
	}
	fake, err := os.ReadFile("../../scripts/agents/adapters/fake.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"--launch-capability", "job launch-capability-consume", `dispatch|follow-up) supervise "$command" "$@"`} {
		if !strings.Contains(string(fake), token) {
			t.Fatalf("fake adapter path lacks %q", token)
		}
	}
}
