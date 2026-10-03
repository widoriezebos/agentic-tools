package dispatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestDesignChainsPassARootThatNeverRan: a design-critic root refused at
// setup never ran, so it is not the design's critique: the design's one
// chain is the root that did run, and a later review or close sees no second
// chain.
func TestDesignChainsPassARootThatNeverRan(t *testing.T) {
	t.Parallel()
	repo, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	page := "metasystem/plans/designs/page.md"
	design := filepath.Join(repo, filepath.FromSlash(page))
	jobs := filepath.Join(repo, "artifacts", "agents", "jobs")
	for _, dir := range []string{filepath.Dir(design), jobs} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(design, []byte("# Page\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	write := func(id string, record map[string]any) {
		t.Helper()
		record["jobId"], record["role"], record["design"], record["goalId"] = id, "design-critic", page, "g"
		data, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(jobs, id+".json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("design-critic-husk", map[string]any{"status": "failed", "phase": "setup", "refusalClass": "setup", "error": "dispatch-refused"})
	if chains := DesignCritiqueChains(repo, "g", design); len(chains) != 0 {
		t.Fatalf("a root refused at setup counts as the design's critique: %+v", chains)
	}
	write("design-critic-ran", map[string]any{"status": "completed", "round": 1})
	if chains := DesignCritiqueChains(repo, "g", design); len(chains) != 1 || chains[0].Root != "design-critic-ran" {
		t.Fatalf("the design's chains = %+v; want the root that ran alone", chains)
	}
}
