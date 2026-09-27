package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

// TestSecondSessionAsksEveryAdapterRuntime pins the runtime list
// scripts/agents/second-session.sh walks for its local-config-paths manifest
// to the registry's adapter-bearing runtimes: a new runtime that declares a
// local file cannot quietly miss every second session.
func TestSecondSessionAsksEveryAdapterRuntime(t *testing.T) {
	t.Parallel()
	script, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "second-session.sh"))
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`(?m)^for runtime in ([a-z -]+); do\n  "\$ms" delegate-supervisor "\$runtime" local-config-paths --root "\$harness_root" >>"\$paths"\n`).FindSubmatch(script)
	if match == nil {
		t.Fatal("second-session.sh no longer asks each runtime's delegate-supervisor entry for its local-config-paths")
	}
	listed := strings.Fields(string(match[1]))
	declared := runtimes.WithAdapter()
	if strings.Join(sortedCopy(listed), " ") != strings.Join(sortedCopy(declared), " ") {
		t.Fatalf("second-session.sh walks %v; the registry declares %v", listed, declared)
	}
}

func sortedCopy(values []string) []string {
	out := append([]string(nil), values...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
