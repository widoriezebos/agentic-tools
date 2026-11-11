package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func TestIntentUnitCheckDirectoriesAcrossRounds(t *testing.T) {
	t.Parallel()
	for _, installation := range []string{".", "metasystem"} {
		for _, manual := range []bool{false, true} {
			name := installation + "/declared"
			if manual {
				name = installation + "/manual"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				b := newWorkBed(t)
				top := b.root()
				if installation != "." {
					top = realpath.ResolveExisting(t.TempDir())
					root := filepath.Join(top, installation)
					if err := os.Rename(b.root(), root); err != nil {
						t.Fatal(err)
					}
					b.facts.root, b.primary = root, top
					conf := filepath.Join(root, "metasystem.conf")
					writeTestingFixtureFile(t, conf, append(mustRead(t, conf), []byte("metasystem.template=true\n")...), 0600)
					writeFixtureEnrollment(t, root, "Wido")
				}
				runnerDirectory := filepath.Join(b.worktree, installation)
				if err := os.MkdirAll(runnerDirectory, 0700); err != nil {
					t.Fatal(err)
				}
				b.declaredCheap, b.declaredAudits, b.declaredDeadline = "pwd", "pwd", "15"
				owners := b.workOwners()
				owners.resolver = stateroot.NewResolver(func(path string) (string, error) {
					if withinPath(path, b.worktree) {
						return b.worktree, nil
					}
					return fakeTop(top)(path)
				}, noExecutable)
				owners.prove = enrolledPersonProver(t, b.root(), b.manager.Now())
				owners.lookupEnv = func(key string) (string, bool) { return "proof", key == launch.KindEnv }
				git := owners.work.git
				var baseline string
				owners.work.git = func(directory string, args ...string) ([]byte, error) {
					if slices.Equal(args, []string{"rev-parse", "--show-toplevel"}) {
						for _, root := range []string{b.worktree, top, baseline} {
							if root != "" && withinPath(directory, root) {
								return []byte(root), nil
							}
						}
					}
					if len(args) == 2 && args[0] == "show" {
						return []byte("proof.cheap=pwd\nproof.audits=pwd\nproof.deadline=15\nproof.full=true # full\n"), nil
					}
					return git(directory, args...)
				}
				run := func(directory string, args ...string) intentResult {
					t.Helper()
					command, rest, _ := resolveIntentArgv(args)
					var out, stderr bytes.Buffer
					code := runIntentIn(command, append([]string{"--repo", b.root(), "--json"}, rest...), &out, &stderr, directory, owners)
					var result intentResult
					if err := json.Unmarshal(out.Bytes(), &result); err != nil || code != 0 {
						t.Fatalf("%v: code=%d result=%s stderr=%s error=%v", args, code, &out, &stderr, err)
					}
					return result
				}
				brief := filepath.Join(b.root(), b.brief("directories.md", "Build the unit.\n"))
				args := []string{"work", "build", b.id, "directories", "--brief", brief, "--lines", "5"}
				if manual {
					args = append(args, "--reason", "Check from my folder", "--by", "Wido", "--check", "pwd")
				}
				built := run(top, args...)
				id := resultData(t, built)["run"].(string)
				for round := 1; round <= 2; round++ {
					plan, err := launch.ReadUnitPlan(resultData(t, built)["plan"].(string))
					manualDirectory := ""
					if manual && round == 1 {
						manualDirectory = b.worktree
					}
					if err != nil || plan.Check == nil || plan.Check.Directory != runnerDirectory || plan.Check.ManualDirectory != manualDirectory || len(plan.Proof) != 1 || plan.Proof[0].Dir != runnerDirectory {
						t.Fatalf("round %d: check=%+v proof=%+v error=%v", round, plan.Check, plan.Proof, err)
					}
					baseline = filepath.Join(filepath.Dir(plan.Path), "base-1")
					if err := os.MkdirAll(filepath.Join(baseline, installation), 0700); err != nil {
						t.Fatal(err)
					}
					for _, tree := range []string{b.worktree, baseline} {
						directory := filepath.Join(tree, installation)
						checked := run(directory, plan.Proof[0].Argv[1:]...)
						execution := resultData(t, checked)["execution"].(string)
						var retained struct {
							Directory string `json:"runDirectory"`
							Exits     []launch.CheckExit
						}
						if err := json.Unmarshal(mustRead(t, filepath.Join(execution, "result.json")), &retained); err != nil || retained.Directory != directory || len(retained.Exits) != 2 {
							t.Fatalf("execution in %s: %+v %v", directory, retained, err)
						}
						cheapDirectory := directory
						if manualDirectory != "" {
							cheapDirectory = tree
						}
						if strings.TrimSpace(retained.Exits[0].Output) != realpath.ResolveExisting(cheapDirectory) || (!manual || round == 2) && strings.TrimSpace(retained.Exits[1].Output) != realpath.ResolveExisting(directory) {
							t.Fatalf("commands ran in the wrong folders: %+v", retained.Exits)
						}
					}
					if round == 1 {
						if manual {
							t.Run("round-2", func(t *testing.T) {
								t.Skip("U6 (D6): round-scoped admission act; finding 576")
							})
							break
						}
						built = run(top, "work", "revise", "run:"+id, "--brief", brief, "--reason", "Exercise the next round", "--by", "Wido")
					}
				}
			})
		}
	}
}
