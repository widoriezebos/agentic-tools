package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processchange"
)

// inverseGit replaces only the branch owner's Git facts and effects.
type inverseGit struct {
	t                                           *testing.T
	b                                           *workBed
	head, content, scratchContent, unit, commit string
	scratchHead                                 string
	commits                                     []branch.Commit
	staged                                      []string
	paths                                       []string
	publications                                int
	fail, interrupted                           bool
}

func (f *inverseGit) inputs() (branch.CommitFacts, branch.CommitEffects) {
	fail := func() { f.t.Helper(); f.t.Fatal("unexpected Git operation") }
	connectionGit(f.t, f.b.worktree, "init", "-q")
	facts := branch.CommitFacts{
		Tip: func(_, ref string) (string, bool, error) {
			if ref == "refs/heads/goal/"+f.b.id {
				return f.head, true, nil
			}
			return "", false, nil
		},
		Head: func(dir string) (string, error) {
			if dir == f.b.worktree {
				return f.head, nil
			}
			if f.commit == f.head {
				return f.scratchHead, nil
			}
			return f.commit, nil
		},
		Tree: func(string) (string, error) { return "tree", nil }, Index: func(dir string) (string, error) {
			if dir == f.b.worktree {
				return "index-with-unrelated-staging", nil
			}
			return connectionGit(f.t, dir, "write-tree"), nil
		}, HeadRef: func(string) string { return "refs/heads/goal/" + f.b.id },
		Staged: func(dir string) ([]string, error) {
			if dir == f.b.worktree {
				return slices.Clone(f.staged), nil
			}
			return f.paths, nil
		}, Unstaged: func(string) ([]string, error) { return nil, nil },
		Patch: func(string) ([]byte, error) { fail(); return nil, nil }, Range: func(string, string, string, string) ([]branch.Commit, error) { return slices.Clone(f.commits), nil },
		Ancestor: func(string, string, string) (bool, error) { return true, nil }, Suffix: func(string, string, string) ([]string, error) { fail(); return nil, nil }, Kind: func(string, string, string) (branch.KindInfo, error) { fail(); return branch.KindInfo{}, nil }, Entries: func(string, string) ([]branch.Entry, error) { fail(); return nil, nil }, Changes: func(string, string, string) ([]string, error) { return f.paths, nil }, Worktrees: func(string) ([]branch.CommitWorktree, error) { fail(); return nil, nil },
	}
	effects := branch.CommitEffects{
		KeepTip: func(string, string, string) error { return nil }, ClearFetch: func(string, string) error { return nil },
		Open: func(_ string, base string, amend bool) (string, func(), error) {
			if base != f.head || amend {
				f.t.Fatal("wrong base or amend")
			}
			f.commit = f.head
			dir, err := filepath.EvalSymlinks(f.t.TempDir())
			if err != nil {
				f.t.Fatal(err)
			}
			connectionGit(f.t, dir, "init", "-q")
			connectionGit(f.t, dir, "config", "user.name", "fixture")
			connectionGit(f.t, dir, "config", "user.email", "fixture@example.invalid")
			writeUnitCarryFile(f.t, filepath.Join(dir, "metasystem.conf"), f.content)
			connectionGit(f.t, dir, "add", "metasystem.conf")
			connectionGit(f.t, dir, "commit", "-qm", "candidate parent")
			writeUnitCarryFile(f.t, filepath.Join(dir, "code.go"), f.content)
			connectionGit(f.t, dir, "add", "code.go")
			connectionGit(f.t, dir, "commit", "-qm", "candidate code")
			f.scratchHead = connectionGit(f.t, dir, "rev-parse", "HEAD")
			return dir, func() {}, nil
		},
		Apply: func(dir string, patch []byte) error {
			if f.fail {
				return errors.New("patch conflict")
			}
			command := exec.Command("git", "apply", "--index", "-")
			command.Dir, command.Env, command.Stdin = dir, gittree.ScrubbedEnviron(), strings.NewReader(string(patch))
			if output, err := command.CombinedOutput(); err != nil {
				return fmt.Errorf("git apply: %s: %w", output, err)
			}
			f.paths = strings.Fields(connectionGit(f.t, dir, "diff", "--cached", "--name-only"))
			content, err := os.ReadFile(filepath.Join(dir, "metasystem.conf"))
			f.scratchContent = string(content)
			return err
		},
		Commit: func(_, _, trailer string, amend bool) error {
			if amend {
				f.t.Fatal("amended history")
			}
			f.unit = strings.TrimPrefix(trailer, "Goal-Unit: "+f.b.id+"/")
			f.commit = fmt.Sprintf("%040d", f.publications+10)
			return nil
		},
		Replay: func(string, string) error { fail(); return nil }, WithoutPaths: func(string, string, []string) (string, error) { fail(); return "", nil }, Checkout: func(string, string, string) error { fail(); return nil }, Attach: func(string, string) error { fail(); return nil }, Restore: func(string, string, string) error { fail(); return nil },
		PatchCheckout: func(_, base, index, next string) error {
			if base != f.head || index != "index-with-unrelated-staging" || next != f.commit {
				f.t.Fatal("staged index was replaced")
			}
			return nil
		},
		Publish: func(_, goal, old, next, _ string) error {
			if old != f.head || goal != f.b.id {
				f.t.Fatal("wrong publication target")
			}
			f.publications++
			f.head, f.b.head, f.content = next, next, f.scratchContent
			f.commits = append(f.commits, branch.Commit{ID: next, Kind: branch.Unit, Unit: f.unit})
			if f.interrupted {
				f.interrupted = false
				return errors.New("publication result lost")
			}
			return nil
		},
	}
	return facts, effects
}
func TestDeclarationInversePublicRevert(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"delta-replay", "conflict", "missing", "wrong-target", "wrong-act-branch", "wrong-checkout", "wrong-current-branch", "explicit-conflict", "corrupt-baseline", "empty-restored", "no-final-newline", "inherited", "staged-overlap", "invalid-path", "write-failure", "push-failure", "inherited-mixed", "code-patch", "gate-failure", "cheap-failure"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			b, owners, _ := rebaseIntentBed(t)
			before := "# keep comment\nproof.cheap=true\nproof.audits=true\nproof.deadline=15\nproof.full=printf full\n"
			after := strings.Replace(before, "proof.cheap=true", "proof.cheap=printf cheap", 1)
			latest := after + "# later independent content\nother=kept\n"
			f := &inverseGit{t: t, b: b, head: b.head, content: latest, staged: []string{"records/unrelated.md"}, paths: []string{"metasystem.conf"}}
			original := processchange.ProcessAct{ID: "original", Goal: b.id, Checkout: b.root(), Class: "declaration", Status: "applied", BeforeDeclaration: &processchange.Declaration{Values: [4]string{"true", "true", "15", "printf full"}, Content: before, Commit: "before"}, AfterDeclaration: &processchange.Declaration{Values: [4]string{"printf cheap", "true", "15", "printf full"}, Content: after, Branch: "goal/" + b.id, Commit: b.head}}
			originalPath := filepath.Join(b.root(), "process", "acts", "original.json")
			write := func(path string, v any) {
				t.Helper()
				data, err := json.Marshal(v)
				if err != nil {
					t.Fatal(err)
				}
				if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "wrong-target" {
				original.Goal = "another"
			}
			if scenario == "wrong-act-branch" {
				original.AfterDeclaration.Branch = "goal/another"
			}
			if scenario == "wrong-checkout" {
				original.Checkout = t.TempDir()
			}
			if scenario == "conflict" || scenario == "explicit-conflict" {
				f.content = strings.Replace(latest, "proof.cheap=printf cheap", "proof.cheap=printf third", 1)
			}
			if scenario == "inherited-mixed" {
				f.content = strings.Replace(latest, "proof.audits=true", "proof.audits=false", 1)
				original.AfterDeclaration.Values[1] = "false"
				original.AfterDeclaration.Content = strings.Replace(after, "proof.audits=true", "proof.audits=false", 1)
				original.InheritedDeclaration = &processchange.Declaration{Values: [4]string{"true", "false", "15", "true"}}
			}
			if scenario == "inherited" {
				original.InheritedDeclaration = original.AfterDeclaration
			}
			if scenario == "staged-overlap" {
				f.staged = append(f.staged, "metasystem.conf")
			}
			if original.InheritedDeclaration == nil {
				original.InheritedDeclaration = original.BeforeDeclaration
			}
			if scenario == "no-final-newline" {
				f.content = strings.TrimSuffix(latest, "\n")
			}
			if scenario == "empty-restored" {
				f.content = "proof.cheap=printf cheap\n"
				original.BeforeDeclaration.Content = ""
				original.AfterDeclaration.Content = f.content
				for i, key := range []string{"proof.cheap", "proof.audits", "proof.deadline", "proof.full"} {
					original.BeforeDeclaration.Values[i], _, _ = config.CommittedContentLookup("", key)
					original.AfterDeclaration.Values[i], _, _ = config.CommittedContentLookup(f.content, key)
				}
			}
			if scenario == "cheap-failure" {
				f.content = strings.Replace(f.content, "proof.cheap=printf cheap", "proof.cheap=false", 1)
				original.AfterDeclaration.Values[0] = "false"
			}
			write(originalPath, original)
			if scenario == "missing" || scenario == "invalid-path" || scenario == "code-patch" {
				if err := os.WriteFile(originalPath, []byte("damaged evidence"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			baseline := filepath.Join(b.root(), "process", fmt.Sprintf("declaration-%x.json", sha256.Sum256([]byte(b.id))))
			write(baseline, map[string]any{"Goal": b.id, "Seed": "seed", "Snapshot": original.AfterDeclaration, "Act": "original"})
			if scenario == "corrupt-baseline" {
				if err := os.WriteFile(baseline, []byte("damaged reference"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			otherBaseline := filepath.Join(b.root(), "process", fmt.Sprintf("declaration-%x.json", sha256.Sum256([]byte("another"))))
			write(otherBaseline, map[string]any{"Goal": "another", "Seed": "different-seed", "Snapshot": processchange.Declaration{Values: [4]string{"other", "false", "20", "full-b"}}})
			otherBefore, _ := os.ReadFile(otherBaseline)
			q, _, err := channel.AskOrFind(channel.AskRequest{RepoRoot: b.root(), Goal: b.id, Kind: "other", Facts: []string{"inverse needed"}, Wants: "restore delta", Now: b.manager.Now()})
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"original", "other"} {
				if name == "other" {
					q, _, err = channel.AskOrFind(channel.AskRequest{RepoRoot: b.root(), Goal: b.id, Kind: "other", Facts: []string{"another stop"}, Wants: "keep this stop", Now: b.manager.Now()})
					if err != nil {
						t.Fatal(err)
					}
				}
				write(filepath.Join(b.root(), "process", "episodes", name, "stops", name+".json"), processchange.DriftStop{ID: name, Goal: b.id, Episode: "episode", Question: q.ID, Stop: loopstop.Stop{Loop: "process", Decision: "stop", Subject: b.id + "/u/round", Cause: &loopstop.Cause{Kind: "process-change", Name: name}}})
			}
			owners.work.git = func(root string, args ...string) ([]byte, error) {
				switch {
				case slices.Equal(args, []string{"symbolic-ref", "--short", "HEAD"}):
					if scenario == "wrong-current-branch" {
						return []byte("goal/another"), nil
					}
					return []byte("goal/" + b.id), nil
				case slices.Equal(args, []string{"rev-parse", "HEAD"}):
					return []byte(f.head), nil
				case args[0] == "show":
					return []byte(f.content), nil
				default:
					return b.workOwners().work.git(root, args...)
				}
			}
			remote := ""
			owners.connection.transport = inverseTransport{remote: &remote, fail: scenario == "push-failure"}
			facts, effects := f.inputs()
			owners.connection.push = func(req branch.PushRequest) (branch.PushResult, error) {
				return branch.PushWithInputs(req, branch.PushInputs{
					TxnRefs: func(string) ([]string, error) { return nil, nil }, Txn: func(string, string) (branch.PushTransaction, error) {
						t.Fatal("unexpected txn read")
						return branch.PushTransaction{}, nil
					}, Tip: facts.Tip, Ancestor: facts.Ancestor, Range: facts.Range, HeadRef: facts.HeadRef, TrackedClean: func(string, bool) (bool, error) { return false, nil }, WriteTxn: func(string, string, branch.PushTransaction) error { return nil }, ClearRef: func(string, string) error { return nil }, RecordOrigin: func(string, string, string) error { return nil }, Detach: func(string, string) error { t.Fatal("unexpected detach"); return nil }, RestoreHead: func(string, string) error { t.Fatal("unexpected restore"); return nil }, SwitchGoal: func(string, string) error { t.Fatal("unexpected switch"); return nil }, MoveRefs: effects.Publish,
				})
			}

			gateCalls := 0
			owners.connection.rebaseGate = func(string) (string, error) {
				gateCalls++
				if scenario == "gate-failure" {
					return "", errors.New("static check failed")
				}
				return "static green", nil
			}
			owners.connection.commitPatch = func(req branch.CommitRequest) (string, error) {
				return branch.CommitFrozenPatchWithInputs(req, facts, effects)
			}
			args := []string{"work", "revert", b.id, "--act", "original", "--reason", "Remove excess checks"}
			if code, result := b.runJSON(owners, args...); code != 1 || f.publications != 0 {
				t.Fatalf("agent authority: %d %+v", code, result)
			}
			owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
			var oldPlanPath string
			var oldPlanBytes []byte
			if scenario == "delta-replay" {
				aGoal, aTree, aContent := b.id, b.worktree, f.content
				b.id = "another"
				checkNextWorktree(t, b)
				f.content = "proof.cheap=other\nproof.audits=false\nproof.deadline=20\nproof.full=full-b\n"
				if code, wrong := b.runJSON(owners, "work", "revert", b.id, "--act", "original", "--reason", "Use A act on B"); code != 1 || !strings.Contains(wrong.Summary, "different goal, branch or target") || f.publications != 0 {
					t.Fatalf("A act accepted by B: %d %s", code, wrong.Summary)
				}
				b.id, b.worktree, f.content = aGoal, aTree, aContent
				brief := b.brief("before-inverse.md", "Build the old round.\n")
				_, built := checkActBuild(t, b, owners, "work", "build", b.id, "--work", "before-inverse", "--brief", brief, "--lines", "5")
				if code, stopped := b.runJSON(owners, "work", "stop", "run:"+resultData(t, built)["run"].(string)); code != 0 {
					t.Fatalf("release old failed round: %d %s", code, stopped.Summary)
				}
				oldPlanPath = resultData(t, built)["plan"].(string)
				oldPlanBytes, _ = os.ReadFile(oldPlanPath)
				plan, err := launch.ReadUnitPlan(oldPlanPath)
				if err != nil || plan.Check == nil || plan.Check.Cheap != "printf cheap" {
					t.Fatalf("old admitted round: %v", err)
				}
			}
			launchesBefore := len(b.starter.launched())
			if scenario == "delta-replay" {
				effects.Publish = func(repo, goal, old, next, origin string) error {
					f.publications++
					f.head, b.head, f.content = next, next, f.scratchContent
					f.commits = append(f.commits, branch.Commit{ID: next, Kind: branch.Unit, Unit: f.unit})
					return os.Chmod(filepath.Join(b.root(), "process", "acts"), 0500)
				}
			}
			if scenario == "write-failure" {
				f.fail = true
			}
			code, result := b.runJSON(owners, args...)
			if scenario == "delta-replay" {
				if code != 1 || f.publications != 1 {
					t.Fatalf("interruption: %d %+v", code, result)
				}
				state, _ := processchange.ReadState(b.root(), b.id)
				if len(state.Stops) != 2 {
					t.Fatal("stops closed before act completion")
				}
				if err := os.Chmod(filepath.Join(b.root(), "process", "acts"), 0700); err != nil {
					t.Fatal(err)
				}
				code, result = b.runJSON(owners, args...)
			}
			if scenario == "corrupt-baseline" {
				if code != 1 || f.publications != 1 || result.Next == nil || !slices.Contains(result.Next.Argv, "--patch") {
					t.Fatalf("reference repair remedy: %d %s", code, result.Summary)
				}
				state, err := processchange.ReadState(b.root(), b.id)
				if err != nil || len(state.Stops) != 2 {
					t.Fatal("unconfirmed completion cleared a hold")
				}
				var patch []byte
				for _, inverse := range state.Acts {
					if inverse.Undo == "original" {
						if inverse.Status != "pending" {
							t.Fatal("unconfirmed completion marked applied")
						}
						patch = inverse.Patch
					}
				}
				patchFile := filepath.Join(t.TempDir(), "retained.patch")
				if err := os.WriteFile(patchFile, patch, 0600); err != nil {
					t.Fatal(err)
				}
				args = append([]string(nil), result.Next.Argv[1:]...)
				args[slices.Index(args, "--patch")+1] = patchFile
				code, result = b.runJSON(owners, args...)
				data, _ := os.ReadFile(baseline)
				if string(data) != "damaged reference" {
					t.Fatal("damaged reference overwritten")
				}
			}
			if scenario == "missing" || scenario == "invalid-path" || scenario == "explicit-conflict" || scenario == "code-patch" {
				if scenario != "invalid-path" && scenario != "code-patch" && (code != 1 || result.Next == nil || !slices.Contains(result.Next.Argv, "--patch")) {
					t.Fatalf("missing remedy: %d %+v", code, result)
				}
				path := "metasystem.conf"
				if scenario == "invalid-path" {
					path = "metasystem/records/forbidden.md"
				}
				if scenario == "code-patch" {
					path = "code.go"
				}
				patch := fmt.Sprintf("--- a/%s\n+++ b/%s\n@@ -1,7 +1,7 @@\n", path, path)
				for _, side := range []struct{ prefix, text string }{{"-", f.content}, {"+", strings.Replace(latest, "proof.cheap=printf cheap", "proof.cheap=true", 1)}} {
					for _, line := range strings.SplitAfter(side.text, "\n") {
						if line != "" {
							patch += side.prefix + line
						}
					}
				}
				patchFile := filepath.Join(t.TempDir(), "repair.patch")
				if err := os.WriteFile(patchFile, []byte(patch), 0600); err != nil {
					t.Fatal(err)
				}
				if scenario != "invalid-path" && scenario != "code-patch" {
					args = append([]string(nil), result.Next.Argv[1:]...)
					args[slices.Index(args, "--patch")+1] = patchFile
				} else {
					args = append(args, "--patch", patchFile)
				}
				code, result = b.runJSON(owners, args...)
			}
			if scenario == "empty-restored" {
				if code != 0 || f.content != "" {
					t.Fatalf("empty declaration content: %d %q", code, f.content)
				}
			} else if scenario == "no-final-newline" {
				if code != 0 || f.content != strings.TrimSuffix(strings.Replace(latest, "proof.cheap=printf cheap", "proof.cheap=true", 1), "\n") {
					t.Fatalf("final newline changed: %d %q", code, f.content)
				}
			} else if scenario == "inherited-mixed" {
				if code != 0 || f.content != strings.Replace(strings.Replace(latest, "proof.cheap=printf cheap", "proof.cheap=true", 1), "proof.audits=true", "proof.audits=false", 1) {
					t.Fatalf("inherited declaration reversed: %d %q", code, f.content)
				}
			} else if scenario == "delta-replay" || scenario == "missing" || scenario == "explicit-conflict" || scenario == "corrupt-baseline" {
				if code != 0 || f.publications != 1 || f.content != strings.Replace(latest, "proof.cheap=printf cheap", "proof.cheap=true", 1) || !slices.Equal(f.staged, []string{"records/unrelated.md"}) {
					t.Fatalf("inverse: %d %+v content=%q", code, result, f.content)
				}
				act := processAct(t, result)
				if act.Undo != "original" || act.Status != "applied" || act.PublishedCommit != f.head {
					t.Fatalf("act: %+v", act)
				}
				if (scenario == "explicit-conflict" || scenario == "corrupt-baseline") && (act.AfterDeclaration != nil || !strings.Contains(act.Citation, "unavailable")) {
					t.Fatal("explicit repair fabricated a declaration baseline")
				}
				if scenario == "delta-replay" {
					var reference struct {
						Goal, Seed, Act string
						Snapshot        processchange.Declaration
					}
					data, err := os.ReadFile(baseline)
					if err != nil || json.Unmarshal(data, &reference) != nil || reference.Goal != b.id || reference.Seed != "seed" || reference.Act != "" || reference.Snapshot.Values != original.BeforeDeclaration.Values || reference.Snapshot.Commit != f.head || reference.Snapshot.Content != f.content {
						t.Fatalf("next admission kept the reversed declaration: %s (%v)", data, err)
					}
				}
				if code, result = b.runJSON(owners, args...); code != 0 || f.publications != 1 {
					t.Fatalf("replay: %d %+v", code, result)
				}
				state, _ := processchange.ReadState(b.root(), b.id)
				if len(state.Stops) != 1 || state.Stops[0].ID != "other" {
					t.Fatalf("other stop cleared: %+v", state)
				}
				if scenario == "missing" {
					body, _ := os.ReadFile(originalPath)
					if string(body) != "damaged evidence" {
						t.Fatal("damaged history overwritten")
					}
				}
			} else if code == 0 {
				t.Fatalf("unsafe inverse accepted: %+v", result)
			} else {
				state, _ := processchange.ReadState(b.root(), b.id)
				if len(state.Stops) != 2 {
					t.Fatal("failed publication cleared a hold")
				}
				for _, act := range state.Acts {
					if act.Undo == "original" && act.Status != "pending" {
						t.Fatalf("failed effect marked applied: %+v", act)
					}
				}
			}
			if scenario == "code-patch" && !strings.Contains(result.Summary, "only metasystem.conf") {
				t.Fatalf("code repair refusal: %s", result.Summary)
			}
			if scenario == "gate-failure" && (gateCalls != 1 || !strings.Contains(result.Summary, "static check failed")) {
				t.Fatalf("static gate bypassed: %d %s", gateCalls, result.Summary)
			}
			if scenario == "cheap-failure" && (gateCalls != 1 || !strings.Contains(result.Summary, "cheap check failed")) {
				t.Fatalf("cheap check bypassed: %d %s", gateCalls, result.Summary)
			}
			if code == 0 && gateCalls != 1 {
				t.Fatalf("successful inverse bypassed static gate: %d", gateCalls)
			}
			otherQuestion, _ := channel.ReadQuestion(b.root(), q.ID)
			if otherQuestion.State != "open" {
				t.Fatal("unrelated ask closed")
			}
			otherAfter, _ := os.ReadFile(otherBaseline)
			if string(otherBefore) != string(otherAfter) {
				t.Fatal("another goal changed")
			}
			if len(b.starter.launched()) != launchesBefore {
				t.Fatal("reversal launched work")
			}
			if scenario == "delta-replay" {
				brief := b.brief("after-inverse.md", "Build with the restored declarations.\n")
				owners.prove = fixedFixtureGoalAuthority
				code, built := checkActBuild(t, b, owners, "work", "build", b.id, "--work", "after-inverse", "--brief", brief, "--lines", "5")
				if code != 0 {
					t.Fatalf("next build required repeat approval: %d %s", code, built.Summary)
				}
				plan, err := launch.ReadUnitPlan(resultData(t, built)["plan"].(string))
				oldAfter, _ := os.ReadFile(oldPlanPath)
				if err != nil || plan.Check.Cheap != "true" || string(oldAfter) != string(oldPlanBytes) {
					t.Fatalf("round declarations: %v", err)
				}
			}
		})
	}
}

type inverseTransport struct {
	remote *string
	fail   bool
}

func (x inverseTransport) RemoteTip(string, string, string) (string, bool, error) {
	return *x.remote, *x.remote != "", nil
}
func (inverseTransport) Fetch(string, string, string, string) error { return nil }
func (x inverseTransport) Push(_, _, _, _, tip string) (branch.CASOutcome, error) {
	if x.fail {
		return branch.CASRefused, errors.New("push lease moved")
	}
	*x.remote = tip
	return branch.CASLanded, nil
}
