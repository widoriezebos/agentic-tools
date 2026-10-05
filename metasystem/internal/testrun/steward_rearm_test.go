package testrun

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func TestStewardBoundaryRefreshFailureIsNotReplaced(t *testing.T) {
	t.Parallel()
	for _, reason := range []string{"facts", "local edits"} {
		t.Run(reason, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			deps := stewardRearmDependencies{
				now: func() time.Time { return time.Date(2026, 10, 4, 8, 0, 0, 0, time.UTC) },
				facts: func() (landedRearmFacts, error) {
					if reason == "facts" {
						return landedRearmFacts{}, errors.New("landing tip is unreadable\nfetch stalled")
					}
					return landedRearmFacts{Head: "old", Tip: "main", Source: "old", HeadIsAncestor: true, DirtyEnginePaths: []string{"cmd/metasystem/main.go"}}, nil
				},
				boundary: func() (bool, error) { return true, nil },
				lock: func() (func(), error) {
					t.Fatal("a refused refresh tried to mutate the engine")
					return func() {}, nil
				},
			}
			for range 2 {
				if replaced, err := rearmStewardWithDependencies(root, deps); replaced || err != nil {
					t.Fatalf("refused refresh: replaced=%t err=%v", replaced, err)
				}
			}
			log, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "steward", "runner.log"))
			if err != nil || strings.Count(string(log), "engine refresh:") != 1 {
				t.Fatalf("refresh repeated its reason: log=%q err=%v", log, err)
			}
			if reason == "local edits" && !strings.Contains(string(log), "has local edits") {
				t.Fatalf("refresh did not log the refusal: %q", log)
			}
			deps.facts = func() (landedRearmFacts, error) { return landedRearmFacts{}, errors.New("a different refresh failure") }
			for range 2 {
				if replaced, err := rearmStewardWithDependencies(root, deps); replaced || err != nil {
					t.Fatalf("second reason: replaced=%t err=%v", replaced, err)
				}
			}
			log, err = os.ReadFile(filepath.Join(root, "artifacts", "agents", "steward", "runner.log"))
			if err != nil || strings.Count(string(log), "engine refresh:") != 2 || strings.Count(string(log), "a different refresh failure") != 1 {
				t.Fatalf("distinct reason was hidden or repeated: log=%q err=%v", log, err)
			}
		})
	}
}

func TestStewardBoundaryRefreshSkipsNonSourceInstallation(t *testing.T) {
	t.Parallel()
	changed, err := RearmStewardAtBoundary(t.TempDir(), func() (bool, error) {
		t.Fatal("a non-source installation reached the unit boundary check")
		return true, nil
	})
	if changed || err != nil {
		t.Fatalf("non-source refresh: changed=%t err=%v", changed, err)
	}
}

func TestStewardFetchRebuildAndRearmAtUnitBoundary(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	now := time.Date(2026, 10, 4, 8, 0, 0, 0, time.UTC)
	boundary, head, engine := false, "old", "old"
	var acts []string
	deps := stewardRearmDependencies{
		now: func() time.Time { return now },
		facts: func() (landedRearmFacts, error) {
			acts = append(acts, "fetch")
			return landedRearmFacts{Source: engine, Head: head, Tip: "main", HeadIsAncestor: true, SourceOwnsTip: engine == "main"}, nil
		},
		boundary: func() (bool, error) { return boundary, nil },
		lock:     func() (func(), error) { return func() {}, nil },
		attempts: func() ([]proofrun.Attempt, error) { return nil, nil },
		forward:  func(tip string) error { acts = append(acts, "forward"); head = tip; return nil },
		rebuild:  func() error { acts = append(acts, "rebuild"); engine = head; return nil },
		start: func() error {
			acts = append(acts, "re-arm")
			if engine != head {
				t.Fatal("re-armed before checkout caught up")
			}
			return nil
		},
	}
	for range 2 {
		if changed, err := rearmStewardWithDependencies(root, deps); err != nil || changed {
			t.Fatalf("busy tick: %t %v", changed, err)
		}
		now = now.Add(time.Minute)
	}
	if head != "old" || engine != "old" || steward.RearmDeferredLine(root) != "engine main, checkout old, re-arm deferred" {
		t.Fatal("busy seat changed or deferral was hidden")
	}
	boundary = true
	acts = nil
	if changed, err := rearmStewardWithDependencies(root, deps); err != nil || !changed || !reflect.DeepEqual(acts, []string{"fetch", "forward", "rebuild", "re-arm"}) || head != "main" || engine != "main" {
		t.Fatalf("next tick at %s: %t %v acts=%v head=%s engine=%s", now, changed, err, acts, head, engine)
	}
	if changed, err := rearmStewardWithDependencies(root, deps); err != nil || changed || steward.RearmDeferredLine(root) != "" {
		t.Fatalf("current engine retained its deferral: changed=%t err=%v line=%q", changed, err, steward.RearmDeferredLine(root))
	}
}
