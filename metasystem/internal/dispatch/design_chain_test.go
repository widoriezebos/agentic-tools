package dispatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestReadDesignCritiqueChains(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"none", "malformed", "unreadable"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			design := filepath.Join(repo, "page.md")
			jobs := filepath.Join(repo, "artifacts", "agents", "jobs")
			if chains, err := ReadDesignCritiqueChains(repo, "g", design); err != nil || len(chains) != 0 {
				t.Fatalf("no local jobs: chains=%+v err=%v", chains, err)
			}
			writeJSONFile(t, jobs, "closed.json", map[string]any{"jobId": "closed", "role": "design-critic", "design": design, "goalId": "g", "round": 1, "chainClosed": true})
			writeJSONFile(t, jobs, "closed-r2.json", map[string]any{"jobId": "closed-r2", "parentJob": "closed", "round": 2})
			writeJSONFile(t, jobs, "open.json", map[string]any{"jobId": "open", "role": "design-critic", "design": design, "goalId": "g", "round": 3})
			writeJSONFile(t, jobs, "other-goal.json", map[string]any{"jobId": "other-goal", "role": "design-critic", "design": design, "goalId": "other"})
			writeJSONFile(t, jobs, "other-page.json", map[string]any{"jobId": "other-page", "role": "design-critic", "design": filepath.Join(repo, "other.md"), "goalId": "g"})
			writeJSONFile(t, jobs, "mismatched.json", map[string]any{"jobId": "wrong-name", "role": "design-critic", "design": design, "goalId": "g"})
			path := filepath.Join(jobs, "broken.json")
			switch failure {
			case "malformed":
				if err := os.WriteFile(path, []byte(`{"jobId":`), 0o600); err != nil {
					t.Fatal(err)
				}
			case "unreadable":
				if err := os.Symlink(filepath.Join(repo, "missing.json"), path); err != nil {
					t.Fatal(err)
				}
			}
			chains, err := ReadDesignCritiqueChains(repo, "g", design)
			if failure == "none" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), path) {
				t.Fatalf("job read failure must name %s: %v", path, err)
			}
			if want := DesignCritiqueChains(repo, "g", design); !reflect.DeepEqual(chains, want) {
				t.Fatalf("chains=%+v; want the existing reader's chains=%+v", chains, want)
			}
			if len(chains) != 2 || chains[0].Root != "closed" || !chains[0].Closed || chains[0].NewestJob != "closed-r2" || chains[0].NewestRound != 2 || chains[1].Root != "open" || chains[1].Closed || chains[1].NewestRound != 3 {
				t.Fatalf("readable job records changed: %+v", chains)
			}
		})
	}
}

func TestReadDesignCritiqueChainsReportsUnreadableJobsFolder(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"not a directory", "permission denied"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			jobs := filepath.Join(repo, "artifacts", "agents", "jobs")
			if err := os.MkdirAll(filepath.Dir(jobs), 0o700); err != nil {
				t.Fatal(err)
			}
			if failure == "not a directory" {
				if err := os.WriteFile(jobs, nil, 0o600); err != nil {
					t.Fatal(err)
				}
			} else {
				if os.Geteuid() == 0 {
					t.Skip("root can read folders regardless of their permission bits")
				}
				if err := os.Mkdir(jobs, 0); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(jobs, 0o700) })
			}
			if chains, err := ReadDesignCritiqueChains(repo, "g", filepath.Join(repo, "page.md")); err == nil || !strings.Contains(err.Error(), jobs) || len(chains) != 0 {
				t.Fatalf("unreadable folder: chains=%+v err=%v", chains, err)
			}
			if chains := DesignCritiqueChains(repo, "g", filepath.Join(repo, "page.md")); len(chains) != 0 {
				t.Fatalf("unchecked reader behavior changed: %+v", chains)
			}
		})
	}
}
