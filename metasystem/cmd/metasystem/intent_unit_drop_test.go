package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

type dropFixture struct {
	dirty, proofWrites, wrote bool
	transferScenarioFixture
	t                                        *testing.T
	base, v, inverse, tree, scratch, trailer string
	inversions, commits, checks, cleanups    int
	conflict, move, person, shared, lose     bool
}

type dropTransport struct{}

func (dropTransport) RemoteTip(string, string, string) (string, bool, error) { return "", false, nil }
func (dropTransport) Fetch(string, string, string, string) error {
	return errors.New("unexpected fetch")
}
func (dropTransport) Push(string, string, string, string, string) (branch.CASOutcome, error) {
	return "", errors.New("unexpected push")
}

type dropSnapshots struct{ f *dropFixture }

func (g dropSnapshots) Run(dir string, env []string, args ...string) ([]byte, error) {
	f := g.f
	switch strings.Join(args, " ") {
	case "rev-parse --show-toplevel":
		return []byte(dir), nil
	case "rev-parse HEAD":
		if dir == f.scratch {
			if f.commits > 0 {
				return []byte(f.inverse), nil
			}
			return []byte(f.v), nil
		}
		return []byte(f.bed.head), nil
	case "write-tree":
		return []byte(f.tree), nil
	case "diff --cached --raw -z --no-abbrev HEAD -- .":
		if dir == f.scratch {
			if f.wrote {
				return branchRawEntry("metasystem/proof-wrote.go"), nil
			}
			return branchRawEntry("metasystem/unit.go"), nil
		}
		if f.dirty {
			return branchRawEntry("metasystem/unrelated.go"), nil
		}
		return nil, nil
	}
	return (workGit{f.bed}).Run(dir, env, args...)
}

type dropProofStarter struct{ f *dropFixture }

func (s dropProofStarter) StartSupervisor(id, state string) (identity.Ref, error) {
	f := s.f
	r, err := f.bed.manager.Store.Read(id)
	if err != nil {
		return identity.Ref{}, err
	}
	if r.Kind == "proof" {
		f.checks++
		if r.WorkingDirectory != f.scratch {
			f.t.Fatalf("proof ran on %s, not candidate %s", r.WorkingDirectory, f.scratch)
		}
		body, err := os.ReadFile(filepath.Join(f.scratch, "unit.go"))
		if err != nil || string(body) != "base\n" {
			f.t.Fatalf("proof did not see the inverse: %q %v", body, err)
		}
		plan, err := os.ReadFile(r.Inputs[0].Path)
		if err != nil || !bytes.Contains(plan, []byte(`"30m"`)) {
			f.t.Fatalf("declared proof not retained: %s %v", plan, err)
		}
	}
	_, err = f.bed.starter.StartSupervisor(id, state)
	if f.proofWrites && r.Kind == "proof" {
		f.wrote = true
		if err := os.WriteFile(filepath.Join(f.scratch, "proof-wrote.go"), []byte("unexpected proof output"), 0600); err != nil {
			f.t.Fatal(err)
		}
	}
	if f.move && r.Kind == "proof" {
		f.bed.head = branchRawID("e")
	}
	return workProcessRef(99), err
}

func (f *dropFixture) raw(dir string, args ...string) ([]byte, error) {
	joined := strings.Join(args, " ")
	switch {
	case joined == "merge-base "+f.base+" "+f.bed.head:
		return []byte(f.base), nil
	case strings.HasPrefix(joined, "merge-base "):
		return []byte(f.base), nil
	case strings.HasPrefix(joined, "rev-list --first-parent --reverse --parents "):
		out := f.commit + " " + f.base + "\n" + f.v + " " + f.commit + "\n"
		if strings.HasSuffix(joined, f.inverse) {
			out += f.inverse + " " + f.v + "\n"
		}
		return []byte(out), nil
	case strings.HasPrefix(joined, "rev-list --first-parent "):
		return []byte(f.v + "\n" + f.commit + "\n"), nil
	case strings.HasPrefix(joined, "rev-list --parents -n 1 "):
		id := args[len(args)-1]
		parent := f.base
		if id == f.v {
			parent = f.commit
		}
		if id == f.inverse {
			parent = f.v
		}
		return []byte(id + " " + parent), nil
	case strings.HasPrefix(joined, "show -s --format=%(trailers:only,unfold=true) "):
		id := args[len(args)-1]
		if id == f.inverse {
			return []byte(f.trailer), nil
		}
		name := "stopped"
		if id == f.v {
			name = "V"
		}
		if f.shared && id == f.commit {
			name += "+other"
		}
		return []byte("Goal-Unit: " + f.bed.id + "/" + name), nil
	case strings.HasPrefix(joined, "diff-tree "):
		name := "metasystem/unit.go"
		if args[len(args)-1] == f.v {
			name = "metasystem/other.go"
		}
		return branchRawEntry(name), nil
	case strings.HasPrefix(joined, "log --format=%H %P %T "):
		if f.commits > 0 {
			return []byte(f.inverse + " " + f.v + " " + f.tree), nil
		}
		return nil, nil
	case joined == "revert --no-commit "+f.commit:
		f.inversions++
		if f.person {
			paths, _ := filepath.Glob(filepath.Join(f.bed.root(), "artifacts", "agents", "channel", "unit-stop-overrides", "*.json"))
			if len(paths) != 1 {
				f.t.Fatalf("inverse ran before person impact: %v", paths)
			}
		}
		if f.conflict {
			return nil, errors.New("unit.go: inverse conflict")
		}
		return nil, os.WriteFile(filepath.Join(dir, "unit.go"), []byte("base\n"), 0600)
	case joined == "diff --name-only --diff-filter=U":
		if f.conflict {
			return []byte("unit.go\n"), nil
		}
		return nil, nil
	case joined == "write-tree":
		return []byte(f.tree), nil
	}
	return f.ownersWorkGit(dir, args...)
}
func (f *dropFixture) ownersWorkGit(dir string, args ...string) ([]byte, error) {
	// Only the standing worktree observations reach this existing adapter.
	return f.bed.workOwners().work.git(dir, args...)
}

func newDropFixture(t *testing.T) *dropFixture {
	t.Helper()
	f := &dropFixture{transferScenarioFixture: newTransferScenarioFixture(t, false), t: t, base: branchRawID("b"), v: branchRawID("d"), inverse: branchRawID("f"), tree: branchRawID("c"), scratch: t.TempDir()}
	f.bed.head = f.commit
	transferWriteJSON(t, filepath.Join(f.bed.root(), "artifacts", "agents", "jobs", f.critic+".json"), f.admitted)
	for name, body := range map[string]string{"unit.go": "base plus original and correction\n", "other.go": "V remains unread\n"} {
		if err := os.WriteFile(filepath.Join(f.scratch, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	units := f.owners.work.units
	f.owners.work.units = func(layout stateroot.Layout) *launch.UnitRunner {
		r := units(layout)
		r.Git = dropSnapshots{f}
		return r
	}
	f.owners.work.git = f.raw
	f.bed.manager.Supervisor = dropProofStarter{f}
	f.owners.connection.endpointTip = func(string, goal.Endpoint) (string, error) { return f.base, nil }
	return f
}

func (f *dropFixture) connect() {
	f.owners.connection.commitToken = func(_ string, body func() error) error { return body() }
	f.owners.connection.transport = dropTransport{}
	facts := branch.CommitFacts{
		Tip: func(_, ref string) (string, bool, error) {
			if strings.HasPrefix(ref, "refs/heads/goal/") {
				return f.bed.head, true, nil
			}
			return "", false, nil
		},
		Head: func(dir string) (string, error) {
			if dir == f.scratch && f.commits > 0 {
				return f.inverse, nil
			}
			return f.bed.head, nil
		},
		Tree: func(string) (string, error) { return f.tree, nil }, Index: func(string) (string, error) { return f.tree, nil }, HeadRef: func(string) string { return "refs/heads/goal/" + f.bed.id },
		Staged: func(string) ([]string, error) { return nil, nil }, Unstaged: func(string) ([]string, error) { return nil, nil }, Patch: func(string) ([]byte, error) { return nil, nil },
		Range: func(repo, base, tip, id string) ([]branch.Commit, error) {
			return branch.ValidateRangeWithGit(repo, base, tip, id, f.raw)
		},
		Ancestor: func(string, string, string) (bool, error) { return false, errors.New("unexpected ancestry query") }, Suffix: func(string, string, string) ([]string, error) { return nil, errors.New("unexpected suffix query") },
		Kind: func(repo, commit, id string) (branch.KindInfo, error) {
			return branch.KindOfWithRaw(repo, commit, id, f.raw)
		}, Entries: func(repo, commit string) ([]branch.Entry, error) {
			return branch.RawEntriesWithRaw(repo, commit, f.raw)
		},
		Changes: func(string, string, string) ([]string, error) { return []string{"metasystem/unit.go"}, nil }, Worktrees: func(string) ([]branch.CommitWorktree, error) { return nil, errors.New("unexpected worktree query") },
	}
	effects := branch.CommitEffects{
		KeepTip: func(string, string, string) error { return nil }, ClearFetch: func(string, string) error { return nil }, Open: func(string, string, bool) (string, func(), error) { return f.scratch, func() { f.cleanups++ }, nil },
		Apply: func(string, []byte) error {
			return errors.New("a drop must prepare its inverse, not apply a staged patch")
		},
		Commit: func(dir, subject, trailer string, amend bool) error {
			if dir != f.scratch || subject != "goal "+f.bed.id+" drop stopped" || !strings.HasPrefix(trailer, "Goal-Drop: "+f.bed.id+"/stopped ") || strings.Contains(trailer, "Goal-Unit") || amend {
				return fmt.Errorf("wrong inverse identity: %q %q", subject, trailer)
			}
			f.trailer = trailer
			f.commits++
			return nil
		},
		Replay: func(string, string) error { return errors.New("unexpected replay") }, WithoutPaths: func(string, string, []string) (string, error) { return "", errors.New("unexpected path filtering") },
		Checkout: func(string, string, string) error {
			body, err := os.ReadFile(filepath.Join(f.scratch, "unit.go"))
			if err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(f.bed.worktree, "unit.go"), body, 0600)
		},
		Attach: func(string, string) error { return errors.New("unexpected attach") }, Restore: func(string, string, string) error { return errors.New("unexpected rollback") },
		Publish: func(_, _, old, next, _ string) error {
			if old != f.v || next != f.inverse {
				return fmt.Errorf("stale publication %s %s", old, next)
			}
			f.bed.head = next
			return nil
		},
	}
	f.owners.connection.commit = func(req branch.CommitRequest) (string, error) {
		id, err := branch.CommitStagedWithInputs(req, facts, effects)
		if f.lose && err == nil {
			return "", errors.New("commit response lost after installation")
		}
		return id, err
	}
}

func (f *dropFixture) review(t *testing.T, extra ...string) (int, intentResult) {
	t.Helper()
	return transferPublic(t, f.bed, f.owners, append([]string{"work", "review", f.bed.id, "--work", "stopped"}, extra...)...)
}
func (f *dropFixture) retained(t *testing.T) launch.UnitRunRecord {
	t.Helper()
	r, err := f.runner.Status(f.run)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestWorkReviewDropsOptionalCommittedUnit(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"inverse", "lost-response", "conflict", "red-proof", "moved-tree", "shared", "person", "stale-binding", "stale-design", "required", "dirty-tree", "proof-writes"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newDropFixture(t)
			f.connect()
			code, result := f.review(t)
			if code != 1 || result.Next == nil {
				t.Fatalf("prepared request: %d %+v", code, result)
			}
			qs, bad := channel.WalkOpenQuestions(f.bed.stateRoot())
			if len(bad) != 0 || len(qs) != 2 {
				t.Fatalf("initial asks: %+v %v", qs, bad)
			}
			for _, q := range qs {
				if q.UnitStop == nil || !strings.Contains(q.UnitStop.Needs, "--dispositions") || !slices.Contains(q.UnitStop.AcceptableActs, "work-drop") || !slices.Contains(q.UnitStop.AcceptableActs, "work-revise") {
					t.Fatalf("drop remedy was overwritten: %+v", q)
				}
			}
			path := filepath.Join(f.retained(t).Rounds[1].Directory, "stop-dispositions.md")
			f.bed.head = f.v
			f.conflict = name == "conflict"
			f.move = name == "moved-tree"
			f.shared = name == "shared"
			f.person = name == "person"
			f.lose = name == "lost-response"
			f.dirty = name == "dirty-tree"
			f.proofWrites = name == "proof-writes"
			if name == "required" {
				body, err := os.ReadFile(f.page)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(f.page, append(body, []byte("| stopped | Required behavior | 5 |\n")...), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if name == "red-proof" {
				f.bed.starter.fail["proof"] = true
			}
			if name == "stale-binding" {
				body, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				body = bytes.Replace(body, []byte("subject="+f.commit), []byte("subject="+f.v), 1)
				if err := os.WriteFile(path, body, 0600); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"--dispositions", path}
			if f.person {
				args = append(args, "--by", "Wido", "--reason", "Remove the extra behavior")
			}
			original, err := os.ReadFile(filepath.Join(f.retained(t).Rounds[1].Directory, "worktree.diff"))
			if err != nil {
				t.Fatal(err)
			}
			code, result = f.review(t, args...)
			r := f.retained(t)
			successful := name == "inverse" || name == "person" || name == "stale-design"
			if successful {
				if code != 1 || result.Outcome != intentPartial || r.Subjects[0].Drop == nil || r.Subjects[0].Drop.Phase != "tree-applied" || r.Subjects[0].Drop.Subject.Commit != f.inverse || r.Subjects[0].Drop.Subject.GateRunID == "" || f.inversions != 1 || f.commits != 1 || f.checks != 1 {
					t.Fatalf("inverse outcome: %d %+v retained=%+v inverse=%d commits=%d proof=%d", code, result, r.Subjects, f.inversions, f.commits, f.checks)
				}
				if !slices.Equal(r.Subjects[0].Drop.Covered, []string{f.commit}) || r.Subjects[0].Commit != f.commit || r.Rounds[1].Transferred || r.Rounds[1].Stop.Decision != "stop" {
					t.Fatalf("original subject/read erased: %+v", r)
				}
				body, err := os.ReadFile(filepath.Join(f.bed.worktree, "unit.go"))
				if err != nil || string(body) != "base\n" {
					t.Fatalf("owned inverse absent: %q %v", body, err)
				}
				kind, err := branch.KindOfWithRaw(f.bed.worktree, f.inverse, f.bed.id, f.raw)
				if err != nil || kind.Kind != branch.Drop || kind.Operation != r.Subjects[0].Drop.Subject.Operation {
					t.Fatalf("kind %v %v", kind, err)
				}
				if name == "stale-design" {
					body, err := os.ReadFile(f.page)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(f.page, append(body, []byte("\nChanged accepted requirements.\n")...), 0600); err != nil {
						t.Fatal(err)
					}
				}
				code, result = f.review(t, args...)
				if code != 1 || f.inversions != 1 || f.commits != 1 || f.checks != 1 {
					t.Fatalf("replay duplicated effect: %d %+v", code, result)
				}
				if name == "stale-design" && (!strings.Contains(result.Summary, "requirements changed") || result.Outcome != intentInProgress) {
					t.Fatalf("stale scope rejoined: %+v", result)
				}
			} else if name == "lost-response" {
				if f.commits != 1 || r.Subjects[0].Drop == nil || r.Subjects[0].Drop.Subject.Commit != "" {
					t.Fatalf("response was not lost: %+v %+v", result, r)
				}
				f.lose = false
				code, result = f.review(t, args...)
				if code != 1 || result.Outcome != intentPartial || f.inversions != 1 || f.commits != 1 || f.checks != 1 || f.retained(t).Subjects[0].Drop.Subject.Commit != f.inverse {
					t.Fatalf("response recovery: %d %+v", code, result)
				}
			} else {
				if code == 0 || f.commits != 0 {
					t.Fatalf("failure installed inverse: %d %+v", code, result)
				}
				if name == "conflict" || name == "red-proof" || name == "moved-tree" || name == "proof-writes" {
					if r.Subjects[0].Drop == nil || r.Subjects[0].Drop.Subject.GateWorktree != f.scratch || f.inversions != 1 {
						t.Fatalf("lost pending scratch: %+v", r)
					}
				}
				if name == "stale-binding" || name == "shared" || name == "required" || name == "dirty-tree" {
					if f.inversions != 0 || f.checks != 0 {
						t.Fatalf("unsafe input launched effects: %+v", result)
					}
				}
			}
			statusCode, status := transferPublic(t, f.bed, f.owners, "work", "status", f.bed.id, "--work", "stopped")
			if statusCode != 0 || resultData(t, status)["work"].([]any)[0].(map[string]any)["subjects"] == nil {
				t.Fatalf("current status hides retained subjects: %d %+v", statusCode, status)
			}
			if f.cleanups != 0 {
				t.Fatalf("only scratch evidence cleaned: %d", f.cleanups)
			}
			unchanged, err := os.ReadFile(filepath.Join(r.Rounds[1].Directory, "worktree.diff"))
			if err != nil || !bytes.Equal(original, unchanged) {
				t.Fatalf("source bytes lost: %v", err)
			}
			body, err := os.ReadFile(filepath.Join(f.scratch, "other.go"))
			if err != nil || string(body) != "V remains unread\n" {
				t.Fatalf("unrelated bytes lost: %q %v", body, err)
			}
			qs, bad = channel.WalkOpenQuestions(f.bed.stateRoot())
			if len(bad) != 0 || len(qs) != 2 {
				t.Fatalf("pending outcome closed asks: %+v %v", qs, bad)
			}
			// The retained source still owns the tree until publication and closure.
			code, waiting := treeBuild(t, f.bed, "another", false)
			if code != 3 || waiting.Next == nil {
				t.Fatalf("another writer entered pending drop: %d %+v", code, waiting)
			}
		})
	}
}
