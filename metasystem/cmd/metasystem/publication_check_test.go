package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

type publicationIntentGit struct {
	base      launch.GitRunner
	raw, tree string
}

type publicationIntentStarter struct{ starter *workStarter }

func (s publicationIntentStarter) StartSupervisor(id, state string) (identity.Ref, error) {
	_, err := s.starter.StartSupervisor(id, state)
	return workProcessRef(30), err
}

func (g publicationIntentGit) Run(dir string, env []string, args ...string) ([]byte, error) {
	switch strings.Join(args, " ") {
	case "diff --cached --raw -z --no-abbrev HEAD -- .":
		return []byte(g.raw), nil
	case "write-tree":
		return []byte(g.tree), nil
	}
	return g.base.Run(dir, env, args...)
}

func TestPublicationCheckThroughWorkReview(t *testing.T) {
	t.Parallel()
	for _, mismatch := range []bool{false, true} {
		t.Run(map[bool]string{false: "reuse and restart", true: "actual parent differs"}[mismatch], func(t *testing.T) {
			t.Parallel()
			b, owners, run, runner := unitCarryIntentBed(t, false)
			b.manager.Supervisor = publicationIntentStarter{b.starter}
			owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
			holdUnitCarryCheckout(t, b, &owners)
			var round launch.UnitRound
			var head string
			if err := runner.ReviewSubject(run, func(review launch.UnitReview, retain func(launch.UnitSubject) error) error {
				round, head = review.Round, review.Head
				subject := *review.Subject
				subject.Commit, subject.Tip = "", ""
				return retain(subject)
			}); err != nil {
				t.Fatal(err)
			}
			parent, tree := "publication-parent", "publication-tree"
			// Both parents are commits in this injected repository, with the
			// candidate's exact content available to its snapshot reader.
			commits := map[string]string{head: tree, parent: "parent-tree"}
			b.head = head
			raw := ":100644 100644 " + strings.Repeat("a", 40) + " " + strings.Repeat("b", 40) + " M\x00unit.go\x00"
			path := filepath.Join(round.Directory, "proof-after.json")
			data, err := os.ReadFile(path)
			var snapshot map[string]any
			if err != nil || json.Unmarshal(data, &snapshot) != nil {
				t.Fatalf("snapshot: %s %v", data, err)
			}
			snapshot["tree"], snapshot["treeId"] = raw, tree
			transferWriteJSON(t, path, snapshot)
			planPath := filepath.Join(round.Directory, "plan.json")
			plan, err := launch.ReadUnitPlan(planPath)
			if err != nil {
				t.Fatal(err)
			}
			plan.Check.Declaration = &launch.UnitDeclaration{Values: [4]string{plan.Check.Cheap, "declared audits", "15", "full"}, SourceSHA256: "declaration"}
			plan.Check.Audits = "declared audits"
			transferWriteJSON(t, planPath, plan)
			current := launch.CheckSelection{Version: 1, Base: parent, ContractDigest: "contract", EngineDigest: "engine", GoVersion: "go fixture", GOOS: "fixture", GOARCH: "fixture", EnvironmentSHA256: "environment", Workers: 9, VerdictPolicyVersion: 1, Groups: []string{"fast-static-build", "prerequisite"}}
			evidence := launch.CheckEvidence{Version: 1, Selection: current, Declaration: plan.Check.Declaration, Commands: [2]string{plan.Check.Cheap, plan.Check.Audits}, Source: "check-source"}
			evidence.Content.Before, evidence.Content.After = tree, tree
			evidence.Strength.Complete, evidence.Strength.Cheap, evidence.Strength.Audited = true, true, true
			if mismatch {
				evidence.Selection.Base = head
			}
			transferWriteJSON(t, filepath.Join(round.Directory, "check-source", "evidence.json"), evidence)
			contextReads, commitsAttempted := 0, 0
			owners.work.units = func(stateroot.Layout) *launch.UnitRunner {
				copy := *runner
				copy.Git = publicationIntentGit{runner.Git, raw, tree}
				copy.PublicationCheck = func(dir, base string, _ launch.UnitCheck, _ launch.CheckSelection) (launch.CheckSelection, []string, error) {
					contextReads++
					if dir != b.worktree || base != parent {
						t.Fatalf("consumer lost actual parent: %s %s", dir, base)
					}
					return current, []string{"fast-static-build", "prerequisite"}, nil
				}
				return &copy
			}
			git := owners.work.git
			owners.work.git = func(dir string, args ...string) ([]byte, error) {
				joined := strings.Join(args, " ")
				switch {
				case joined == "diff --cached --name-only -z":
					return []byte("unit.go\x00"), nil
				case joined == "diff --cached --raw -z --no-abbrev HEAD -- .":
					return []byte(raw), nil
				case joined == "write-tree":
					return []byte(tree), nil
				case strings.HasPrefix(joined, "--literal-pathspecs add -A -- "):
					return nil, nil
				case joined == "rev-parse HEAD":
					if _, present := commits[b.head]; !present {
						t.Fatalf("unknown fixture commit %s", b.head)
					}
					return []byte(b.head), nil
				}
				return git(dir, args...)
			}
			owners.connection.commitToken = func(_ string, fn func() error) error { return fn() }
			owners.connection.commit = func(req branch.CommitRequest) (string, error) {
				commitsAttempted++
				if err := req.BeforeCommit(b.worktree, parent, tree); err != nil {
					return "", err
				}
				b.head = parent
				return "", errors.New("fixture interruption after publication proof")
			}
			launches := len(b.starter.launched())
			for repeat := 0; repeat < 2; repeat++ {
				b.head = head
				code, result, stderr := b.run(owners, "work", "review", "run:"+run)
				if code == 0 || !strings.Contains(result+stderr, "fixture interruption after publication proof") {
					t.Fatalf("review did not reach proof boundary: code=%d output=%s stderr=%s", code, result, stderr)
				}
				record, err := runner.Status(run)
				if err != nil {
					t.Fatal(err)
				}
				subject := record.Subjects[0]
				wantLaunches := launches
				if mismatch {
					wantLaunches++
				}
				if len(b.starter.launched()) != wantLaunches || (subject.GateReuse != nil) == mismatch || subject.GateRunID == "" {
					t.Fatalf("wrong publication result: %+v launches=%v stderr=%s", subject, b.starter.launched(), stderr)
				}
				if mismatch && !strings.Contains(stderr, "base differs") {
					t.Fatalf("rerun did not name base: %s", stderr)
				}
				if !mismatch && (subject.GateReuse.Source != "check-source" || subject.GateReuse.Why == "") {
					t.Fatalf("reuse lost source: %+v", subject.GateReuse)
				}
			}
			if contextReads < 2 || commitsAttempted != 2 {
				t.Fatalf("public review skipped decision: reads=%d attempts=%d", contextReads, commitsAttempted)
			}
		})
	}
}
