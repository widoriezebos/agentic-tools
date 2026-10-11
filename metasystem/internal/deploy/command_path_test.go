package deploy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeclaredAdapterReceivesRunningEnginePath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	runner := &Runner{Project: "fixture", trees: map[string]*landedTree{
		"fixture-commit": {source: dir, contract: Contract{Schema: 1, Adapter: Adapter{Argv: []string{
			"/bin/sh", "-c", `printf '{"outcome":"built","artifact":"%s"}' "$PATH"`,
		}}}},
	}}
	answer := runner.call("build", "fixture-commit", "", nil, filepath.Join(dir, "adapter.log"), nil)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	prefix := filepath.Dir(executable) + string(os.PathListSeparator)
	if !answer.done("built") || !strings.HasPrefix(answer.Artifact, prefix) {
		t.Fatalf("declared adapter environment: %+v; want PATH prefix %q", answer, prefix)
	}
}
