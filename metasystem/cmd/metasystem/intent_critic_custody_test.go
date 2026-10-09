package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

type criticCustodyReader struct{ dead bool }

func (r *criticCustodyReader) ReadStart(pid int64) (identity.Exact, identity.Liveness, error) {
	if r.dead || pid == 21 {
		return identity.Exact{}, identity.Dead, nil
	}
	exact := identity.Exact{Pid: pid, StartedAt: time.Unix(400, 1_000)}
	if runtime.GOOS == "linux" {
		exact.StartTicks, exact.BootID = 400, "fixture-boot"
	}
	return exact, identity.Alive, nil
}
func (*criticCustodyReader) ReadArgv(int64) ([]string, bool) { return []string{"fixture-critic"}, true }

func criticCustodyBed(t *testing.T) (*workBed, intentOwners, string, *criticCustodyReader) {
	t.Helper()
	b, owners, run, _ := unitCarryIntentBed(t, false)
	b.head = "replayed-tip"
	reader := &criticCustodyReader{}
	dependencies := dispatchcore.CustodyDeathDependencies{Reader: reader, Processes: identity.FixedProcessTable{}, MatchesTag: func([]string, string) bool { return true }, TaggedScan: func(string) census.TaggedProcessCensus { return census.TaggedProcessCensus{} }}
	b.workOwnersHook = func(o *intentWorkOwners) { o.criticDeath = dependencies }
	owners.work.criticDeath = dependencies
	now, err := b.commandNow(b.root())
	if err != nil {
		t.Fatal(err)
	}
	owners.prove = enrolledPersonProver(t, b.root(), now)
	return b, owners, run, reader
}

func writeCustodyCritic(t *testing.T, b *workBed, job, status string) string {
	t.Helper()
	path := filepath.Join(b.worktree, "artifacts", "agents", "jobs", job+".json")
	record := map[string]any{"jobId": job, "goalId": b.id, "role": "code-critic", "reviews": "commit:corrected-commit", "status": status, "round": 1, "instanceTag": "metasystem-job-" + job + "-nonce", "pid": int64(20), "pidStartedAt": int64(400), "pidStartedAtExactMicro": int64(400_000_001), "pidStartTicks": int64(400), "bootId": "fixture-boot", "pgid": int64(20), "custodyProcesses": []any{}}
	if runtime.GOOS == "darwin" {
		delete(record, "pidStartTicks")
		delete(record, "bootId")
	} else {
		delete(record, "pidStartedAtExactMicro")
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func closeCustodyRun(t *testing.T, b *workBed, run string) {
	t.Helper()
	record, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
	if err != nil {
		t.Fatal(err)
	}
	record.Rounds[len(record.Rounds)-1].Transferred = true
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.unitRoot, run, "run.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestIntentCommittedCriticCustodyAcrossCommands(t *testing.T) {
	t.Parallel()
	for _, interrupted := range []bool{false, true} {
		name := "returned-command"
		if interrupted {
			name = "interrupted-dispatch"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b, owners, run, reader := criticCustodyBed(t)
			crash := new(int)
			owners.delivery.branchRead = func([]string) (branch.BranchReadResult, int, error) {
				writeCustodyCritic(t, b, "critic-first", "running")
				if interrupted {
					panic(crash)
				}
				return branch.BranchReadResult{State: "dispatched", RootJob: "critic-first"}, 0, nil
			}
			if interrupted {
				func() {
					defer func() {
						if caught := recover(); caught != crash {
							t.Fatalf("dispatch interruption: %v", caught)
						}
					}()
					b.runJSON(owners, "work", "review", "run:"+run)
				}()
			} else {
				code, result := b.runJSON(owners, "work", "review", "run:"+run)
				if code != 1 || result.Outcome != intentInProgress {
					t.Fatalf("review: %d %+v", code, result)
				}
			}
			// A completed later round does not hide a live earlier round, even when
			// that earlier round's record itself says completed.
			writeCustodyCritic(t, b, "critic-first", "completed")
			later := writeCustodyCritic(t, b, "critic-later", "completed")
			data, err := dispatchcore.ReadRecordObject(later)
			if err != nil {
				t.Fatal(err)
			}
			data["pid"], data["pgid"] = 21, 21
			body, err := json.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(later, body, 0600); err != nil {
				t.Fatal(err)
			}
			closeCustodyRun(t, b, run)
			code, waiting := treeBuild(t, b, "v", false)
			if code != 3 || waiting.Next == nil || waiting.Next.Argv[3] != "run:"+run {
				t.Fatalf("live critic released: %d %+v", code, waiting)
			}
			writeCustodyCritic(t, b, "critic-first", "running")
			var cancelled []string
			owners.work.cancelCritic = func(root, job string) (map[string]any, int, error) {
				if root != b.worktree {
					t.Fatalf("wrong critic store: %s", root)
				}
				cancelled = append(cancelled, job)
				if job == "critic-first" {
					writeCustodyCritic(t, b, job, "cancelled")
				}
				return map[string]any{"outcome": "CANCELLED", "jobId": job}, 0, nil
			}
			code, pending := b.runJSON(owners, "work", "stop", "run:"+run)
			if code != 1 || !strings.Contains(resultWords(pending), "remains reserved") || !slices.Contains(cancelled, "critic-first") {
				t.Fatalf("stop trusted terminal status: %d %+v %v", code, pending, cancelled)
			}
			code, waiting = treeBuild(t, b, "v", false)
			if code != 3 {
				t.Fatalf("pending stop released live critic: %d %+v", code, waiting)
			}
			reader.dead = true
			for _, job := range []string{"critic-first", "critic-later"} {
				r, err := dispatchcore.ReadRecordObject(filepath.Join(b.worktree, "artifacts", "agents", "jobs", job+".json"))
				if err == nil {
					death := dispatchcore.ProveCustodyDeath(b.worktree, r, owners.work.criticDeath)
					if death.Outcome != dispatchcore.CustodyDeathProven {
						t.Fatalf("fixture custody %s: %+v", job, death)
					}
				}
			}
			code, stopped := b.runJSON(owners, "work", "stop", "run:"+run)
			if code != 0 {
				t.Fatalf("exact custody stop: %d %+v", code, stopped)
			}
			code, next := treeBuild(t, b, "v", false)
			if code != 0 {
				t.Fatalf("next writer: %d %+v", code, next)
			}
		})
	}
}

func TestIntentPersonRecoversDamagedTreeOwnership(t *testing.T) {
	t.Parallel()
	for _, damage := range []string{"malformed", "wrong-owner", "unreadable"} {
		t.Run(damage, func(t *testing.T) {
			t.Parallel()
			b, owners, run, reader := criticCustodyBed(t)
			writeCustodyCritic(t, b, "critic", "running")
			paths, err := filepath.Glob(filepath.Join(b.unitRoot, ".trees", "*.json"))
			if err != nil || len(paths) != 1 {
				t.Fatalf("owners: %v %v", paths, err)
			}
			path := paths[0]
			content := "{broken"
			if damage == "wrong-owner" {
				content = `{"worktree":"another-tree","run":"other-run"}`
			}
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			if damage == "unreadable" {
				if err := os.Chmod(path, 0000); err != nil {
					t.Fatal(err)
				}
			}
			cancelled := 0
			owners.work.cancelCritic = func(root, job string) (map[string]any, int, error) {
				return cancelDispatchJobWith(func(request delegateRequest, stdout, stderr io.Writer) int {
					if request.rootOverride != b.worktree || !slices.Equal(request.args, []string{"--cancel", "critic"}) {
						t.Fatalf("cancel request escaped its exact store: %+v", request)
					}
					if root != b.worktree || job != "critic" {
						t.Fatalf("wrong cancellation: %s %s", root, job)
					}
					cancelled++
					writeCustodyCritic(t, b, job, "cancelled")
					if err := json.NewEncoder(stdout).Encode(map[string]any{"outcome": "CANCELLED", "jobId": job}); err != nil {
						t.Fatal(err)
					}
					return 0
				}, root, job)
			}
			agent := owners
			agent.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
				return humanauthority.Proof{}, errors.New("agent invocation")
			}
			code, refused := b.runJSON(agent, "work", "stop", "run:"+run)
			if code != 1 || refused.Outcome != intentRefused {
				t.Fatalf("agent got recovery power: %d %+v", code, refused)
			}
			code, pending := b.runJSON(owners, "work", "stop", "run:"+run)
			if code != 1 || cancelled != 1 {
				t.Fatalf("person recovery did not quiesce critic: %d %+v calls=%d", code, pending, cancelled)
			}
			var recovered struct{ Run string }
			body, err := os.ReadFile(path)
			if err != nil || json.Unmarshal(body, &recovered) != nil || recovered.Run != run {
				t.Fatalf("damaged record not replaced: %s %v", body, err)
			}
			code, waiting := treeBuild(t, b, "v", false)
			if code != 3 {
				t.Fatalf("recovery allowed two live owners: %d %+v", code, waiting)
			}
			reader.dead = true
			code, stopped := b.runJSON(owners, "work", "stop", "run:"+run)
			if code != 0 {
				t.Fatalf("recovery: %d %+v", code, stopped)
			}
			code, next := treeBuild(t, b, "v", false)
			if code != 0 {
				t.Fatalf("recovered tree cannot proceed: %d %+v", code, next)
			}
		})
	}
}

func TestIntentTreeRecoveryIgnoresUnrelatedRuns(t *testing.T) {
	t.Parallel()
	for _, ownership := range []string{"damaged", "missing"} {
		t.Run(ownership, func(t *testing.T) {
			t.Parallel()
			b, owners, run, _ := criticCustodyBed(t)
			paths, err := filepath.Glob(filepath.Join(b.unitRoot, ".trees", "*.json"))
			if err != nil || len(paths) != 1 {
				t.Fatalf("owners: %v %v", paths, err)
			}
			gone := filepath.Join(t.TempDir(), "deleted-tree")
			if err := os.Mkdir(gone, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(gone); err != nil {
				t.Fatal(err)
			}
			writeUnitCarryFile(t, filepath.Join(b.unitRoot, "a-gone", "run.json"), `{"id":"a-gone","worktree":`+strconv.Quote(gone)+`}`)
			writeUnitCarryFile(t, filepath.Join(b.unitRoot, "a-corrupt", "run.json"), "{broken")
			// An unrelated tree's identity is outside this tree's ownership.
			writeUnitCarryFile(t, filepath.Join(b.unitRoot, "a-unrelated", "run.json"), `{"id":"wrong-id","worktree":`+strconv.Quote(t.TempDir())+`}`)
			if ownership == "missing" {
				if err := os.Remove(paths[0]); err != nil {
					t.Fatal(err)
				}
				// With no reservation, retained history cannot impose an owner.
				writeUnitCarryFile(t, filepath.Join(b.unitRoot, "a-retained", "run.json"), `{"id":"a-retained","state":"running","worktree":`+strconv.Quote(b.worktree)+`}`)
			} else {
				if err := os.WriteFile(paths[0], []byte("{broken"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			code, stopped := b.runJSON(owners, "work", "stop", "run:"+run)
			if code != 0 {
				t.Fatalf("person stop with %s ownership: %d %+v", ownership, code, stopped)
			}
			if _, err := os.Stat(paths[0]); !os.IsNotExist(err) {
				t.Fatalf("ownership not released: %v", err)
			}
			code, next := treeBuild(t, b, "v", false)
			if code != 0 {
				t.Fatalf("next writer: %d %+v", code, next)
			}
		})
	}
}

func TestIntentDamagedTreeOwnershipPrintsExecutableRecovery(t *testing.T) {
	t.Parallel()
	for _, known := range []bool{true, false} {
		name := "named-run"
		if !known {
			name = "unknown-run"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b, owners, run, _ := criticCustodyBed(t)
			paths, err := filepath.Glob(filepath.Join(b.unitRoot, ".trees", "*.json"))
			if err != nil || len(paths) != 1 {
				t.Fatalf("owners: %v %v", paths, err)
			}
			if err := os.WriteFile(paths[0], []byte("{broken"), 0600); err != nil {
				t.Fatal(err)
			}
			if known {
				// A closed older run cannot hide the current writer.
				writeUnitCarryFile(t, filepath.Join(b.unitRoot, "a-closed", "run.json"), `{"id":"a-closed","state":"cancelled","worktree":`+strconv.Quote(b.worktree)+`}`)
			} else {
				if err := os.WriteFile(filepath.Join(b.unitRoot, run, "run.json"), []byte("{broken"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			launched := len(b.starter.launched())
			code, refused := treeBuild(t, b, "v", false)
			if code != 1 || refused.Next == nil || len(b.starter.launched()) != launched {
				t.Fatalf("damaged ownership started work or omitted recovery: %d %+v", code, refused)
			}
			if !known {
				if len(refused.Next.Argv) < 3 || !slices.Equal(refused.Next.Argv[:3], []string{"metasystem", "question", "ask"}) || !strings.Contains(refused.Next.Reason, "person") {
					t.Fatalf("unknown owner has no person request: %+v", refused.Next)
				}
				return
			}
			want := []string{"metasystem", "work", "stop", "run:" + run}
			if !slices.Equal(refused.Next.Argv, want) || !strings.Contains(refused.Next.Reason, "person") {
				t.Fatalf("wrong recovery: %+v; want %v", refused, want)
			}
			args := append([]string{"work", "build", b.id, "--work", "v", "--brief", b.brief("v.md", "Build the declared requirement.\n"), "--lines", "5"}, workCheck...)
			code, stdout, stderr := b.run(b.workOwners(), args...)
			if code != 1 || !strings.Contains(stdout+stderr, strings.Join(want, " ")) || len(b.starter.launched()) != launched {
				t.Fatalf("text did not print the exact recovery command: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			code, stopped := b.runJSON(owners, refused.Next.Argv[1:]...)
			if code != 0 {
				t.Fatalf("printed person remedy failed: %d %+v", code, stopped)
			}
			code, next := treeBuild(t, b, "v", false)
			if code != 0 {
				t.Fatalf("writer could not proceed after printed recovery: %d %+v", code, next)
			}
		})
	}
}
