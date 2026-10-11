package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func TestLandingClassifyRecoversSavedLogWithoutFailedSet(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"complete", "no cause", "environment", "incomplete", "unreadable", "recorded set"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newResolveVerbFixture(t)
			helmMust(t, os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("metasystem.template=true\ntesting.contract=testing.json\n"), 0600))
			b.owners.resolver = stateroot.NewResolver(func(string) (string, error) { return b.root, nil }, os.Executable)
			b.owners.prove = enrolledPersonProver(t, b.install, laneTestNow)
			red := plain.Result{Attempt: "saved-red", Result: plain.Red, Commit: "main", Tree: "tree", Scope: "full",
				Log: filepath.Join(t.TempDir(), "saved.log"), At: laneTestNow.Format("2006-01-02T15:04:05Z07:00"),
				Cause: &plain.Cause{Kind: "unclassified", Evidence: "original evidence"}}
			log := "LANDING-FAILED\tu/a\tTestA\nLANDING-CHECKED\t1\nlanding prove: failed\n✗ commit is proven red\n→ metasystem landing status\n"
			wantFailed := []plain.FailedUnit{{Unit: "u/a", Tests: []string{"TestA"}}}
			wantTests := []string{"u/a TestA"}
			switch name {
			case "no cause":
				red.Cause = nil
			case "environment":
				red.Cause.Kind, red.Cause.Name, red.Cause.Tests = "environment", "lost-process", []string{"old TestOld"}
			case "incomplete":
				log = strings.Replace(log, "LANDING-CHECKED\t1", "LANDING-CHECKED\t2", 1)
				wantFailed, wantTests = nil, nil
			case "unreadable":
				wantFailed, wantTests = nil, nil
			case "recorded set":
				wantFailed = []plain.FailedUnit{{Unit: "u/b", Tests: []string{"TestB"}}}
				wantTests = []string{"u/b TestB"}
				red.Failed, red.Cause.Tests = wantFailed, wantTests
			}
			if name != "unreadable" {
				helmMust(t, os.WriteFile(red.Log, []byte(log), 0600))
			}
			writeCauseProof(t, b.install, "results.jsonl", red)
			path := filepath.Join(plain.Dir(b.install), "results.jsonl")
			original, err := os.ReadFile(path)
			helmMust(t, err)
			if name != "recorded set" && bytes.Contains(original, []byte(`"failed"`)) {
				t.Fatal("saved record already contains a failed set")
			}
			judged := false
			b.owners.landing.plainProve = plain.ProveSeams{
				Git: func(_ string, args ...string) (string, error) {
					switch strings.Join(args, " ") {
					case "show main:metasystem/metasystem.conf":
						return "proof.full=fixture-check\n", nil
					case "fetch --quiet origin +refs/heads/main:refs/remotes/origin/main":
						return "", nil
					case "rev-parse --verify --quiet refs/remotes/origin/main^{commit}":
						return "main", nil
					}
					if len(args) >= 2 && args[0] == "worktree" && (args[1] == "add" || args[1] == "remove") {
						return "", nil
					}
					return "", fmt.Errorf("unexpected Git command: %v", args)
				},
				Command: func(*exec.Cmd) error {
					t.Fatal("classification of main must use its saved red without executing a check")
					return nil
				},
				Judge: func(checkout, commit string, failed []plain.FailedUnit) (map[string]plain.UnitJudgement, error) {
					judged = true
					if checkout != b.root || commit != red.Commit || !reflect.DeepEqual(failed, wantFailed) {
						t.Fatalf("classification input: %s %s %+v", checkout, commit, failed)
					}
					return nil, nil
				},
			}
			code, output := b.run(t, b.root, "prove", "--classify", red.Attempt)
			if code != 0 {
				t.Fatalf("classify saved red: exit %d: %s", code, output)
			}
			results, err := plain.Results(b.install)
			helmMust(t, err)
			if len(results) != 2 {
				t.Fatalf("classification must append one result: %+v", results)
			}
			got := results[1]
			if judged != (len(wantFailed) > 0 && name != "environment") || !reflect.DeepEqual(got.Failed, wantFailed) || got.Cause == nil ||
				!reflect.DeepEqual(got.Cause.Tests, wantTests) || got.ClassificationPending || got.ClassificationPerson == nil {
				t.Fatalf("classification lost the saved failures or person: %+v judged=%t", got, judged)
			}
			if name == "environment" {
				if got.Cause.Kind != red.Cause.Kind || got.Cause.Name != red.Cause.Name || got.Cause.Evidence != red.Cause.Evidence {
					t.Fatalf("recovery replaced the recorded environment cause: %+v", got.Cause)
				}
			} else if len(wantFailed) > 0 {
				if got.Cause.Kind != "main" || got.Cause.Evidence != red.Log {
					t.Fatalf("saved red was not attributed to main: %+v", got.Cause)
				}
			} else if !reflect.DeepEqual(got.Cause, red.Cause) {
				t.Fatalf("unusable log changed the recorded cause: %+v", got.Cause)
			}
			stops, err := plain.OpenStops(b.install)
			helmMust(t, err)
			if len(stops) != 1 || stops[0].Measure.Name != "red set" || !reflect.DeepEqual(stops[0].Measure.Now, wantTests) ||
				len(wantTests) > 0 && stops[0].Class != strings.Join(wantTests, ", ") {
				t.Fatalf("stop lost the failure identities: %+v", stops)
			}
			after, err := os.ReadFile(path)
			helmMust(t, err)
			if !bytes.HasPrefix(after, original) {
				t.Fatal("classification rewrote the original red record")
			}
		})
	}
}
